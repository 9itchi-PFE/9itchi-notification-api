package services

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"9itchi-notification-api/core"
	"9itchi-notification-api/models/api"
	"9itchi-notification-api/models/db"
	"9itchi-notification-api/repositories"
	"9itchi-notification-api/services/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	uid = "11111111-1111-1111-1111-111111111111"
	nid = "22222222-2222-2222-2222-222222222222"
)

type fakeRealtime struct {
	calls []string
	err   error
}

func (f *fakeRealtime) Publish(_ context.Context, userID string, _ any) error {
	f.calls = append(f.calls, userID)
	return f.err
}

type fakeChannel struct{ sent *int }

func (f fakeChannel) Name() string { return "fake" }
func (f fakeChannel) Send(_ context.Context, _, _ string, _ map[string]any) error {
	*f.sent++
	return errors.New("boom")
}

func newSvc(repo *mocks.NotificationRepository, rt core.Realtime) NotificationService {
	return NewNotificationService(repo, core.NewChannelFactory(), rt)
}

func createEcho() func(context.Context, *db.Notification) (*db.Notification, error) {
	return func(_ context.Context, n *db.Notification) (*db.Notification, error) {
		return &db.Notification{ID: nid, UserID: n.UserID, Type: n.Type, Title: n.Title, Data: n.Data, CreatedAt: time.Now()}, nil
	}
}

func TestSendNotification(t *testing.T) {
	tests := []struct {
		name     string
		req      api.SendNotificationRequest
		repoErr  error
		wantCode int
		wantPub  int
	}{
		{"ok", api.SendNotificationRequest{UserID: uid, Type: "system", Title: "hi", Data: map[string]any{"a": 1}}, nil, 0, 1},
		{"bad type", api.SendNotificationRequest{UserID: uid, Type: "nope", Title: "hi"}, nil, http.StatusBadRequest, 0},
		{"bad user id", api.SendNotificationRequest{UserID: "x", Type: "system", Title: "hi"}, nil, http.StatusBadRequest, 0},
		{"empty title", api.SendNotificationRequest{UserID: uid, Type: "system"}, nil, http.StatusBadRequest, 0},
		{"repo error", api.SendNotificationRequest{UserID: uid, Type: "system", Title: "hi"}, errors.New("db down"), http.StatusInternalServerError, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mocks.NotificationRepository{CreateFn: func(ctx context.Context, n *db.Notification) (*db.Notification, error) {
				if tt.repoErr != nil {
					return nil, tt.repoErr
				}
				return createEcho()(ctx, n)
			}}
			rt := &fakeRealtime{}
			res, apiErr := newSvc(repo, rt).SendNotification(context.Background(), tt.req)
			if tt.wantCode != 0 {
				require.NotNil(t, apiErr)
				assert.Equal(t, tt.wantCode, apiErr.Code)
				assert.Nil(t, res)
			} else {
				require.Nil(t, apiErr)
				assert.Equal(t, nid, res.ID)
				assert.Equal(t, "system", res.Type)
			}
			assert.Len(t, rt.calls, tt.wantPub)
		})
	}
}

func TestSendNotificationDeliveryFailuresDoNotFail(t *testing.T) {
	sent := 0
	repo := &mocks.NotificationRepository{CreateFn: createEcho()}
	factory := core.ChannelFactory{"fake": func() core.Channel { return fakeChannel{sent: &sent} }}
	rt := &fakeRealtime{err: errors.New("ably down")}
	svc := NewNotificationService(repo, factory, rt)

	res, apiErr := svc.SendNotification(context.Background(), api.SendNotificationRequest{UserID: uid, Type: "system", Title: "t"})
	require.Nil(t, apiErr)
	assert.NotNil(t, res)
	assert.Equal(t, 1, sent)
	assert.Len(t, rt.calls, 1)
}

func TestListNotifications(t *testing.T) {
	var gotLimit, gotOffset int
	repo := &mocks.NotificationRepository{
		ListFn: func(_ context.Context, _ string, _ bool, limit, offset int) ([]db.Notification, error) {
			gotLimit, gotOffset = limit, offset
			return []db.Notification{{ID: nid, UserID: uid, Type: "system", Title: "t"}}, nil
		},
		CountFn: func(_ context.Context, _ string, unreadOnly bool) (int, error) {
			if unreadOnly {
				return 2, nil
			}
			return 5, nil
		},
	}
	res, apiErr := newSvc(repo, nil).ListNotifications(context.Background(), uid, api.ListNotificationsQuery{Page: 2, PageSize: 500})
	require.Nil(t, apiErr)
	assert.Equal(t, 100, gotLimit)
	assert.Equal(t, 100, gotOffset)
	assert.Equal(t, 5, res.Total)
	assert.Equal(t, 2, res.UnreadCount)
	assert.Len(t, res.Items, 1)
	assert.NotNil(t, res.Items[0].Data)

	res, apiErr = newSvc(repo, nil).ListNotifications(context.Background(), uid, api.ListNotificationsQuery{Unread: true})
	require.Nil(t, apiErr)
	assert.Equal(t, 20, gotLimit)
	assert.Equal(t, 2, res.Total)
	assert.Equal(t, 2, res.UnreadCount)
}

func TestListNotificationsError(t *testing.T) {
	repo := &mocks.NotificationRepository{
		ListFn: func(context.Context, string, bool, int, int) ([]db.Notification, error) { return nil, errors.New("x") },
	}
	_, apiErr := newSvc(repo, nil).ListNotifications(context.Background(), uid, api.ListNotificationsQuery{})
	require.NotNil(t, apiErr)
	assert.Equal(t, http.StatusInternalServerError, apiErr.Code)
	assert.Equal(t, "internal server error", apiErr.Error.Error())
}

func TestMarkRead(t *testing.T) {
	tests := []struct {
		name     string
		id       string
		repoErr  error
		wantCode int
	}{
		{"ok", nid, nil, 0},
		{"bad id", "x", nil, http.StatusBadRequest},
		{"not owner", nid, repositories.ErrNotFound, http.StatusNotFound},
		{"db error", nid, errors.New("x"), http.StatusInternalServerError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mocks.NotificationRepository{MarkReadFn: func(_ context.Context, userID, id string) (*db.Notification, error) {
				if tt.repoErr != nil {
					return nil, tt.repoErr
				}
				return &db.Notification{ID: id, UserID: userID, Read: true}, nil
			}}
			res, apiErr := newSvc(repo, nil).MarkRead(context.Background(), uid, tt.id)
			if tt.wantCode != 0 {
				require.NotNil(t, apiErr)
				assert.Equal(t, tt.wantCode, apiErr.Code)
				return
			}
			require.Nil(t, apiErr)
			assert.True(t, res.Read)
		})
	}
}

func TestMarkAllRead(t *testing.T) {
	repo := &mocks.NotificationRepository{MarkAllReadFn: func(context.Context, string) (int64, error) { return 3, nil }}
	res, apiErr := newSvc(repo, nil).MarkAllRead(context.Background(), uid)
	require.Nil(t, apiErr)
	assert.EqualValues(t, 3, res.Updated)

	repo.MarkAllReadFn = func(context.Context, string) (int64, error) { return 0, errors.New("x") }
	_, apiErr = newSvc(repo, nil).MarkAllRead(context.Background(), uid)
	require.NotNil(t, apiErr)
	assert.Equal(t, http.StatusInternalServerError, apiErr.Code)
}

func TestAblyTokenNotImplemented(t *testing.T) {
	_, apiErr := newSvc(&mocks.NotificationRepository{}, nil).AblyToken(context.Background(), uid)
	require.NotNil(t, apiErr)
	assert.Equal(t, http.StatusNotImplemented, apiErr.Code)
}

func TestHandleEvent(t *testing.T) {
	tests := []struct {
		event    string
		wantType string
	}{
		{core.EventEmailOpened, "email_opened"},
		{core.EventEmailSent, "scheduled_email_sent"},
		{core.EventEmailFailed, "email_failed"},
		{core.EventJobsMatched, "new_matching_jobs"},
		{core.EventPaymentCompleted, ""},
	}
	for _, tt := range tests {
		t.Run(tt.event, func(t *testing.T) {
			var got *db.Notification
			repo := &mocks.NotificationRepository{CreateFn: func(ctx context.Context, n *db.Notification) (*db.Notification, error) {
				got = n
				return createEcho()(ctx, n)
			}}
			newSvc(repo, nil).HandleEvent(context.Background(), tt.event, map[string]any{"user_id": uid, "count": 3})
			if tt.wantType == "" {
				assert.Nil(t, got)
				return
			}
			require.NotNil(t, got)
			assert.Equal(t, tt.wantType, got.Type)
			assert.Equal(t, uid, got.UserID)
			assert.Equal(t, map[string]any{"count": 3}, got.Data)
			assert.NotEmpty(t, got.Title)
		})
	}
}

func TestHandleEventMissingUserIsSkipped(t *testing.T) {
	called := false
	repo := &mocks.NotificationRepository{CreateFn: func(ctx context.Context, n *db.Notification) (*db.Notification, error) {
		called = true
		return createEcho()(ctx, n)
	}}
	newSvc(repo, nil).HandleEvent(context.Background(), core.EventEmailOpened, map[string]any{})
	assert.False(t, called)
}
