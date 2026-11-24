package services

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"9itchi-notification-api/core"
	"9itchi-notification-api/models"
	"9itchi-notification-api/models/api"
	"9itchi-notification-api/models/db"
	"9itchi-notification-api/repositories"
	"9itchi-notification-api/utils"

	"go.uber.org/zap"
)

// Recipient is what the service needs to know to write to a user. Never store or log the address.
type Recipient struct {
	Email    string
	Name     string
	Language string
}

// EmailContext names what an application email was sent for.
type EmailContext struct {
	ApplicationID string
	JobID         string
	JobTitle      string
	Company       string
}

// AlertDeps is what the alerts need beyond the notifications themselves. Every field is optional: without Prefs
// each kind keeps its default, without Mailer or Users no email leaves, without EmailContext an email does not name
// the company.
type AlertDeps struct {
	Prefs  repositories.PreferenceRepository
	Mailer core.Mailer
	// Users returns the address, name and language of a user (account-api).
	Users func(ctx context.Context, userID string) (*Recipient, error)
	// EmailContext returns what an email was sent for, nil when it was sent for no application (email-api).
	EmailContext func(ctx context.Context, emailID string) (*EmailContext, error)
	// FrontendURL is where the links of the emails lead.
	FrontendURL string
}

// Option configures the service.
type Option func(*notificationService)

// WithAlerts gives the service the alert settings and the means to send emails.
func WithAlerts(deps AlertDeps) Option {
	return func(s *notificationService) { s.alerts = deps }
}

// scheduledOnly keeps the events of an email sent by the scheduler.
func scheduledOnly(payload map[string]any) bool {
	scheduled, _ := payload["scheduled"].(bool)
	return scheduled
}

// fromEvent builds an event handler: payload.user_id is the recipient, the rest of the payload is stored as data.
// keep, when set, decides whether the event is worth telling. The user's alert settings then decide where it goes:
// in 9itchi, by email, both or nowhere.
func (s *notificationService) fromEvent(notifType, title string, keep func(payload map[string]any) bool) func(ctx context.Context, payload map[string]any) error {
	return func(ctx context.Context, payload map[string]any) error {
		if keep != nil && !keep(payload) {
			return nil
		}
		userID, _ := payload["user_id"].(string)
		if !uuidRe.MatchString(userID) {
			// A payload that is refused today is refused tomorrow: delivering it again changes nothing.
			utils.Logger.Warn("event ignored: no valid user", zap.String("type", notifType))
			return nil
		}
		data := make(map[string]any, len(payload))
		for k, v := range payload {
			if k != "user_id" && k != core.EventIDKey {
				data[k] = v
			}
		}
		var eventID *string
		if id, _ := payload[core.EventIDKey].(string); id != "" {
			eventID = &id
		}
		s.nameTheOffer(ctx, data)

		choice, err := s.alertChoice(ctx, userID, core.AlertOfType(notifType))
		if err != nil {
			return fmt.Errorf("alert settings for %s not read", notifType)
		}
		if choice.InApp {
			res, apiErr := s.send(ctx, api.SendNotificationRequest{UserID: userID, Type: notifType, Title: title, Data: data}, eventID)
			switch {
			case apiErr != nil && apiErr.Code >= http.StatusInternalServerError:
				return fmt.Errorf("notification for %s not stored", notifType)
			case apiErr != nil:
				utils.Logger.Warn("event ignored", zap.String("type", notifType), zap.Int("status", apiErr.Code))
				return nil
			case res == nil:
				// The same event again: it was told the first time, by every way.
				return nil
			}
		}
		if choice.Email {
			company, _ := data["company"].(string)
			jobTitle, _ := data["job_title"].(string)
			s.mail(ctx, userID, notifType, emailDetails{Company: company, JobTitle: jobTitle})
		}
		return nil
	}
}

// nameTheOffer adds the company and the offer an application email was sent for, so that the notification can say
// « Acme a ouvert ta candidature ». Without them it is told without the name.
func (s *notificationService) nameTheOffer(ctx context.Context, data map[string]any) {
	emailID, _ := data["email_id"].(string)
	if emailID == "" || s.alerts.EmailContext == nil {
		return
	}
	c, err := s.alerts.EmailContext(ctx, emailID)
	if err != nil {
		utils.Logger.Warn("email context not read", zap.Error(err))
		return
	}
	if c == nil {
		return
	}
	data["application_id"], data["job_id"], data["job_title"], data["company"] = c.ApplicationID, c.JobID, c.JobTitle, c.Company
}

// alertChoice returns where a kind of alert goes for a user. kind "" is a notification that is always delivered.
func (s *notificationService) alertChoice(ctx context.Context, userID, kind string) (core.AlertChoice, error) {
	if kind == "" {
		return core.AlertChoice{Email: true, InApp: true}, nil
	}
	if s.alerts.Prefs == nil {
		return core.DefaultAlert(kind), nil
	}
	p, err := s.alerts.Prefs.Get(ctx, userID, kind)
	if errors.Is(err, repositories.ErrNotFound) {
		return core.DefaultAlert(kind), nil
	}
	if err != nil {
		utils.Logger.Error("read alert preference failed", zap.Error(err))
		return core.AlertChoice{}, err
	}
	return core.AlertChoice{Email: p.Email, InApp: p.InApp}, nil
}

// mail writes and sends one email to a user, in their language. A failure is logged and never undoes what was
// already told; the address is never logged.
func (s *notificationService) mail(ctx context.Context, userID, kind string, d emailDetails) {
	if s.alerts.Mailer == nil || s.alerts.Users == nil {
		return
	}
	u, err := s.alerts.Users(ctx, userID)
	if err != nil || u == nil || u.Email == "" {
		utils.Logger.Warn("email not sent: recipient not found", zap.String("kind", kind))
		return
	}
	if kind == emailWelcome {
		d.Name = firstName(u.Name)
	}
	content, ok := writeEmail(kind, u.Language, s.alerts.FrontendURL, d)
	if !ok {
		return
	}
	text, htmlBody := core.RenderEmail(content)
	if err := s.alerts.Mailer.Mail(ctx, u.Email, content.Subject, text, htmlBody); err != nil {
		utils.Logger.Warn("email not sent", zap.String("kind", kind))
	}
}

// receiptEmail emails the receipt of a completed payment. It is not an alert: it is sent whatever the settings say.
// A payment of nothing (a code that covered the whole price) has nothing to confirm.
func (s *notificationService) receiptEmail(ctx context.Context, payload map[string]any) error {
	userID, _ := payload["user_id"].(string)
	paymentID, _ := payload["payment_id"].(string)
	amount, _ := payload["amount_millimes"].(float64) // JSON numbers
	if !uuidRe.MatchString(userID) || !uuidRe.MatchString(paymentID) || amount <= 0 {
		return nil
	}
	no, _ := payload["receipt_no"].(string)
	s.mail(ctx, userID, emailReceipt, emailDetails{Millimes: int64(amount), ReceiptNo: no, PaymentID: paymentID})
	return nil
}

// welcomeEmail greets a new account. Like the receipt, it does not depend on the alert settings.
func (s *notificationService) welcomeEmail(ctx context.Context, payload map[string]any) error {
	userID, _ := payload["user_id"].(string)
	if !uuidRe.MatchString(userID) {
		return nil
	}
	s.mail(ctx, userID, emailWelcome, emailDetails{})
	return nil
}

func (s *notificationService) GetAlertPreferences(ctx context.Context, userID string) (*api.AlertPreferencesResponse, *models.ApiError) {
	if !uuidRe.MatchString(userID) {
		return nil, &models.ApiError{Code: http.StatusBadRequest, Error: errors.New("invalid user id")}
	}
	chosen := map[string]db.AlertPreference{}
	if s.alerts.Prefs != nil {
		rows, err := s.alerts.Prefs.ListByUser(ctx, userID)
		if err != nil {
			return nil, internalError("list alert preferences", err)
		}
		for _, r := range rows {
			chosen[r.Kind] = r
		}
	}
	res := &api.AlertPreferencesResponse{Items: make([]api.AlertPreferenceResponse, 0, len(core.AlertKinds))}
	for _, kind := range core.AlertKinds {
		choice := core.DefaultAlert(kind)
		if r, ok := chosen[kind]; ok {
			choice = core.AlertChoice{Email: r.Email, InApp: r.InApp}
		}
		res.Items = append(res.Items, api.AlertPreferenceResponse{Kind: kind, Email: choice.Email, InApp: choice.InApp})
	}
	return res, nil
}

func (s *notificationService) UpdateAlertPreferences(ctx context.Context, userID string, req api.UpdateAlertPreferencesRequest) (*api.AlertPreferencesResponse, *models.ApiError) {
	if !uuidRe.MatchString(userID) {
		return nil, &models.ApiError{Code: http.StatusBadRequest, Error: errors.New("invalid user id")}
	}
	if s.alerts.Prefs == nil {
		return nil, &models.ApiError{Code: http.StatusServiceUnavailable, Error: errors.New("alert settings unavailable")}
	}
	seen := map[string]bool{}
	prefs := make([]db.AlertPreference, 0, len(req.Items))
	for _, item := range req.Items {
		if !core.IsAlertKind(item.Kind) || item.Email == nil || item.InApp == nil {
			return nil, &models.ApiError{Code: http.StatusBadRequest, Error: errors.New("each item needs a known kind, email and in_app")}
		}
		if seen[item.Kind] {
			return nil, &models.ApiError{Code: http.StatusBadRequest, Error: errors.New("a kind is given twice")}
		}
		seen[item.Kind] = true
		prefs = append(prefs, db.AlertPreference{UserID: userID, Kind: item.Kind, Email: *item.Email, InApp: *item.InApp})
	}
	if len(prefs) == 0 {
		return nil, &models.ApiError{Code: http.StatusBadRequest, Error: errors.New("items is required")}
	}
	if err := s.alerts.Prefs.Upsert(ctx, userID, prefs); err != nil {
		return nil, internalError("save alert preferences", err)
	}
	return s.GetAlertPreferences(ctx, userID)
}
