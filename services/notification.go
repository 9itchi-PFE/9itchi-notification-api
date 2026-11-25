package services

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"time"

	"9itchi-notification-api/core"
	"9itchi-notification-api/models"
	"9itchi-notification-api/models/api"
	"9itchi-notification-api/models/db"
	"9itchi-notification-api/models/mapping"
	"9itchi-notification-api/repositories"
	"9itchi-notification-api/utils"

	"github.com/ably/ably-go/ably"
	"go.uber.org/zap"
)

const (
	defaultPageSize = 20
	maxPageSize     = 100
	// ablyTokenTTL keeps tokens short; the client renews through the endpoint.
	ablyTokenTTL = 30 * time.Minute
)

var uuidRe = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// allowedTypes is the allow-list of notification types (mirrors the CHECK constraint).
var allowedTypes = map[string]bool{
	"email_opened":         true,
	"scheduled_email_sent": true,
	"email_failed":         true,
	"new_matching_jobs":    true,
	"saved_job_closing":    true,
	"cv_tip":               true,
	"month_gifted":         true,
	"admin_message":        true,
	"system":               true,
}

// NotificationService holds all business logic for notification. Handlers stay thin.
type NotificationService interface {
	// GET /notifications: own notifications, newest first, with unread_count
	ListNotifications(ctx context.Context, userID string, q api.ListNotificationsQuery) (*api.NotificationListResponse, *models.ApiError)
	// PUT /notifications/:id/read: mark one as read (404 if not the caller's)
	MarkRead(ctx context.Context, userID string, id string) (*api.NotificationResponse, *models.ApiError)
	// PUT /notifications/read-all: mark all as read
	MarkAllRead(ctx context.Context, userID string) (*api.MarkAllReadResponse, *models.ApiError)
	// GET /notifications/ably-token: subscribe-only Ably token request for the caller's user:{id} channel (503 when Ably is not configured)
	AblyToken(ctx context.Context, userID string) (*api.AblyTokenResponse, *models.ApiError)
	// POST /internal/notifications: store, deliver through channels, publish realtime
	SendNotification(ctx context.Context, req api.SendNotificationRequest) (*api.NotificationResponse, *models.ApiError)
	// GET /notifications/preferences: where each kind of alert goes for the caller (email, in 9itchi)
	GetAlertPreferences(ctx context.Context, userID string) (*api.AlertPreferencesResponse, *models.ApiError)
	// PUT /notifications/preferences: change the choice for the kinds given; the others keep theirs
	UpdateAlertPreferences(ctx context.Context, userID string, req api.UpdateAlertPreferencesRequest) (*api.AlertPreferencesResponse, *models.ApiError)
	// HandleEvent turns a domain event (core.Event*) into a notification. Unknown events are ignored. An error means
	// the event could not be stored and is worth one more delivery; an event that can never be handled returns nil.
	HandleEvent(ctx context.Context, event string, payload map[string]any) error
}

// EventsQueue is the durable queue the service reads its events from.
const EventsQueue = "notification-api.events"

// HandledEvents are the events the service acts on: the routing keys of its queue.
var HandledEvents = []string{
	core.EventEmailOpened, core.EventEmailSent, core.EventEmailFailed, core.EventJobsMatched,
	core.EventPaymentCompleted, core.EventUserRegistered, core.EventSubscriptionRenewalDue,
	core.EventJobClosingSoon, core.EventCVTips, core.EventSubscriptionGifted,
}

type notificationService struct {
	repo     repositories.NotificationRepository
	channels core.ChannelFactory
	realtime core.Realtime
	tokens   core.TokenRequestCreator // nil when Ably is not configured
	events   map[string]func(ctx context.Context, payload map[string]any) error
	alerts   AlertDeps
}

func NewNotificationService(repo repositories.NotificationRepository, channels core.ChannelFactory, realtime core.Realtime, tokens core.TokenRequestCreator, opts ...Option) NotificationService {
	s := &notificationService{repo: repo, channels: channels, realtime: realtime, tokens: tokens}
	for _, opt := range opts {
		opt(s)
	}
	s.events = map[string]func(ctx context.Context, payload map[string]any) error{
		core.EventEmailOpened: s.fromEvent("email_opened", "Your email was opened", nil),
		// A send the student clicked shows its result on the screen: only what the scheduler did is news.
		core.EventEmailSent:      s.fromEvent("scheduled_email_sent", "Your scheduled email was sent", scheduledOnly),
		core.EventEmailFailed:    s.fromEvent("email_failed", "Your email could not be sent", scheduledOnly),
		core.EventJobsMatched:    s.fromEvent("new_matching_jobs", "New jobs match your profile", hasCount),
		core.EventJobClosingSoon: s.fromEvent("saved_job_closing", "A saved offer closes soon", closesOnADay),
		core.EventCVTips:         s.fromEvent("cv_tip", "Your CV has points to correct", tipsACV),
		// A gift is not an alert to switch off: it is told in 9itchi and by email, whatever the settings say.
		core.EventSubscriptionGifted:     s.fromEvent("month_gifted", "The team gave you a month", endsOnADay),
		core.EventPaymentCompleted:       s.receiptEmail,
		core.EventUserRegistered:         s.welcomeEmail,
		core.EventSubscriptionRenewalDue: s.renewalEmail,
	}
	return s
}

func (s *notificationService) HandleEvent(ctx context.Context, event string, payload map[string]any) error {
	if handle, ok := s.events[event]; ok {
		return handle(ctx, payload)
	}
	return nil
}

func (s *notificationService) ListNotifications(ctx context.Context, userID string, q api.ListNotificationsQuery) (*api.NotificationListResponse, *models.ApiError) {
	page, size := q.Page, q.PageSize
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = defaultPageSize
	}
	if size > maxPageSize {
		size = maxPageSize
	}
	rows, err := s.repo.List(ctx, userID, q.Unread, size, (page-1)*size)
	if err != nil {
		return nil, internalError("list notifications", err)
	}
	total, err := s.repo.Count(ctx, userID, q.Unread)
	if err != nil {
		return nil, internalError("count notifications", err)
	}
	unread := total
	if !q.Unread {
		if unread, err = s.repo.Count(ctx, userID, true); err != nil {
			return nil, internalError("count unread notifications", err)
		}
	}
	items := make([]api.NotificationResponse, 0, len(rows))
	for i := range rows {
		items = append(items, mapping.NotificationToResponse(&rows[i]))
	}
	return &api.NotificationListResponse{Items: items, Total: total, Page: page, PageSize: size, UnreadCount: unread}, nil
}

func (s *notificationService) MarkRead(ctx context.Context, userID string, id string) (*api.NotificationResponse, *models.ApiError) {
	if !uuidRe.MatchString(id) {
		return nil, &models.ApiError{Code: http.StatusBadRequest, Error: errors.New("invalid notification id")}
	}
	n, err := s.repo.MarkRead(ctx, userID, id)
	if errors.Is(err, repositories.ErrNotFound) {
		return nil, &models.ApiError{Code: http.StatusNotFound, Error: errors.New("notification not found")}
	}
	if err != nil {
		return nil, internalError("mark notification read", err)
	}
	res := mapping.NotificationToResponse(n)
	return &res, nil
}

func (s *notificationService) MarkAllRead(ctx context.Context, userID string) (*api.MarkAllReadResponse, *models.ApiError) {
	updated, err := s.repo.MarkAllRead(ctx, userID)
	if err != nil {
		return nil, internalError("mark all notifications read", err)
	}
	return &api.MarkAllReadResponse{Updated: updated}, nil
}

func (s *notificationService) AblyToken(_ context.Context, userID string) (*api.AblyTokenResponse, *models.ApiError) {
	if s.tokens == nil {
		return nil, &models.ApiError{Code: http.StatusServiceUnavailable, Error: errors.New("realtime unavailable")}
	}
	if !uuidRe.MatchString(userID) {
		return nil, &models.ApiError{Code: http.StatusBadRequest, Error: errors.New("invalid user id")}
	}
	capability, err := json.Marshal(map[string][]string{core.UserChannel(userID): {"subscribe"}})
	if err != nil {
		return nil, internalError("build ably capability", err)
	}
	req, err := s.tokens.CreateTokenRequest(&ably.TokenParams{
		TTL:        ablyTokenTTL.Milliseconds(),
		Capability: string(capability),
		ClientID:   userID,
	})
	if err != nil {
		return nil, internalError("create ably token request", err)
	}
	return req, nil
}

func (s *notificationService) SendNotification(ctx context.Context, req api.SendNotificationRequest) (*api.NotificationResponse, *models.ApiError) {
	return s.send(ctx, req, nil)
}

// send stores and delivers one notification. eventID is set when it comes from an event: the same event delivered
// again finds its notification already there and does nothing (nil, nil).
func (s *notificationService) send(ctx context.Context, req api.SendNotificationRequest, eventID *string) (*api.NotificationResponse, *models.ApiError) {
	if !uuidRe.MatchString(req.UserID) {
		return nil, &models.ApiError{Code: http.StatusBadRequest, Error: errors.New("invalid user_id")}
	}
	if !allowedTypes[req.Type] {
		return nil, &models.ApiError{Code: http.StatusBadRequest, Error: errors.New("invalid notification type")}
	}
	if req.Title == "" || len(req.Title) > 200 {
		return nil, &models.ApiError{Code: http.StatusBadRequest, Error: errors.New("invalid title")}
	}
	names := []string{core.ChannelInApp}
	seen := map[string]bool{core.ChannelInApp: true}
	for _, name := range req.Channels {
		if _, ok := s.channels[name]; !ok {
			return nil, &models.ApiError{Code: http.StatusBadRequest, Error: errors.New("unsupported channel")}
		}
		if !seen[name] {
			seen[name] = true
			names = append(names, name)
		}
	}
	n, err := s.repo.Create(ctx, &db.Notification{UserID: req.UserID, Type: req.Type, Title: req.Title, Data: req.Data, EventID: eventID})
	if eventID != nil && errors.Is(err, repositories.ErrConflict) {
		return nil, nil
	}
	if err != nil {
		return nil, internalError("create notification", err)
	}
	res := mapping.NotificationToResponse(n)
	// The live push goes first so a slow channel (email) never delays it.
	if s.realtime != nil {
		if err := s.realtime.Publish(ctx, n.UserID, res); err != nil {
			utils.Logger.Warn("realtime publish failed")
		}
	}
	for _, name := range names {
		newChannel, ok := s.channels[name]
		if !ok {
			continue
		}
		if err := newChannel().Send(ctx, n.UserID, n.Title, res.Data); err != nil {
			utils.Logger.Warn("channel delivery failed", zap.String("channel", name))
		}
	}
	return &res, nil
}

// internalError logs the cause and returns a generic 500.
func internalError(op string, err error) *models.ApiError {
	utils.Logger.Error(op+" failed", zap.Error(err))
	return &models.ApiError{Code: http.StatusInternalServerError, Error: errors.New("internal server error")}
}
