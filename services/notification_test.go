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

	"github.com/ably/ably-go/ably"
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
	return NewNotificationService(repo, core.NewChannelFactory(), rt, nil)
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
	svc := NewNotificationService(repo, factory, rt, nil)

	res, apiErr := svc.SendNotification(context.Background(), api.SendNotificationRequest{UserID: uid, Type: "system", Title: "t", Channels: []string{"fake"}})
	require.Nil(t, apiErr)
	assert.NotNil(t, res)
	assert.Equal(t, 1, sent)
	assert.Len(t, rt.calls, 1)

	sent = 0
	_, apiErr = svc.SendNotification(context.Background(), api.SendNotificationRequest{UserID: uid, Type: "system", Title: "t", Channels: []string{"fake", "fake"}})
	require.Nil(t, apiErr)
	assert.Equal(t, 1, sent, "duplicate channels are delivered once")
}

func TestSendNotificationStoredWhenEmailChannelFails(t *testing.T) {
	repo := &mocks.NotificationRepository{CreateFn: createEcho()}
	factory := core.NewChannelFactory()
	factory.RegisterEmail(core.EmailConfig{APIKey: "key", Domain: "mg.example.com"}, func(context.Context, string) (string, error) {
		return "", errors.New("account down")
	})
	require.Contains(t, factory, core.ChannelEmail)

	res, apiErr := NewNotificationService(repo, factory, &fakeRealtime{}, nil).SendNotification(context.Background(), api.SendNotificationRequest{UserID: uid, Type: "system", Title: "t", Channels: []string{core.ChannelEmail}})
	require.Nil(t, apiErr)
	assert.Equal(t, nid, res.ID)
}

func TestSendNotificationOnlyUsesRequestedChannels(t *testing.T) {
	resolved := 0
	factory := core.NewChannelFactory()
	factory.RegisterEmail(core.EmailConfig{APIKey: "key", Domain: "mg.example.com"}, func(context.Context, string) (string, error) {
		resolved++
		return "", errors.New("stop")
	})
	svc := NewNotificationService(&mocks.NotificationRepository{CreateFn: createEcho()}, factory, nil, nil)

	_, apiErr := svc.SendNotification(context.Background(), api.SendNotificationRequest{UserID: uid, Type: "system", Title: "t"})
	require.Nil(t, apiErr)
	assert.Zero(t, resolved, "email is opt-in")

	_, apiErr = svc.SendNotification(context.Background(), api.SendNotificationRequest{UserID: uid, Type: "system", Title: "t", Channels: []string{"sms"}})
	require.NotNil(t, apiErr)
	assert.Equal(t, http.StatusBadRequest, apiErr.Code)
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

type fakeTokens struct {
	params *ably.TokenParams
	err    error
}

func (f *fakeTokens) CreateTokenRequest(params *ably.TokenParams, _ ...ably.AuthOption) (*ably.TokenRequest, error) {
	f.params = params
	if f.err != nil {
		return nil, f.err
	}
	return &ably.TokenRequest{TokenParams: *params, KeyName: "app.key"}, nil
}

func TestAblyToken(t *testing.T) {
	newTokenSvc := func(tokens core.TokenRequestCreator) NotificationService {
		return NewNotificationService(&mocks.NotificationRepository{}, core.NewChannelFactory(), nil, tokens)
	}

	t.Run("scoped to the caller channel, subscribe only", func(t *testing.T) {
		tokens := &fakeTokens{}
		res, apiErr := newTokenSvc(tokens).AblyToken(context.Background(), uid)
		require.Nil(t, apiErr)
		assert.JSONEq(t, `{"user:`+uid+`":["subscribe"]}`, res.Capability)
		assert.Equal(t, uid, tokens.params.ClientID)
		assert.Equal(t, (30 * time.Minute).Milliseconds(), tokens.params.TTL)
		assert.Equal(t, "app.key", res.KeyName)
	})

	t.Run("unavailable without ably", func(t *testing.T) {
		_, apiErr := newTokenSvc(nil).AblyToken(context.Background(), uid)
		require.NotNil(t, apiErr)
		assert.Equal(t, http.StatusServiceUnavailable, apiErr.Code)
		assert.EqualError(t, apiErr.Error, "realtime unavailable")
	})

	t.Run("invalid user id", func(t *testing.T) {
		_, apiErr := newTokenSvc(&fakeTokens{}).AblyToken(context.Background(), "x")
		require.NotNil(t, apiErr)
		assert.Equal(t, http.StatusBadRequest, apiErr.Code)
	})

	t.Run("creator error is a generic 500", func(t *testing.T) {
		_, apiErr := newTokenSvc(&fakeTokens{err: errors.New("key=secret")}).AblyToken(context.Background(), uid)
		require.NotNil(t, apiErr)
		assert.Equal(t, http.StatusInternalServerError, apiErr.Code)
		assert.EqualError(t, apiErr.Error, "internal server error")
	})
}

func TestHandleEvent(t *testing.T) {
	tests := []struct {
		event    string
		payload  map[string]any
		wantType string
	}{
		{core.EventEmailOpened, map[string]any{"count": 3}, "email_opened"},
		{core.EventEmailSent, map[string]any{"count": 3, "scheduled": true}, "scheduled_email_sent"},
		{core.EventEmailFailed, map[string]any{"count": 3, "scheduled": true}, "email_failed"},
		{core.EventJobsMatched, map[string]any{"count": 3}, "new_matching_jobs"},
		{core.EventJobsMatched, map[string]any{"count": float64(2)}, "new_matching_jobs"},
		{core.EventJobClosingSoon, map[string]any{"job_id": nid, "job_title": "Portail RH", "company": "Acme", "closes_on": "2026-12-03", "days_left": float64(2)}, "saved_job_closing"},
		{core.EventCVTips, map[string]any{"resume_id": nid, "count": float64(4), "rule": "summary"}, "cv_tip"},
		// Nothing to count, no offer or no day, no CV: not news.
		{core.EventJobsMatched, map[string]any{"count": float64(0)}, ""},
		{core.EventJobsMatched, map[string]any{}, ""},
		{core.EventJobClosingSoon, map[string]any{"job_id": nid, "closes_on": "soon"}, ""},
		{core.EventJobClosingSoon, map[string]any{"closes_on": "2026-12-03"}, ""},
		{core.EventCVTips, map[string]any{"resume_id": nid, "count": float64(0)}, ""},
		{core.EventCVTips, map[string]any{"count": float64(4)}, ""},
		// A send the student clicked shows its result on the screen: no notification.
		{core.EventEmailSent, map[string]any{"scheduled": false}, ""},
		{core.EventEmailSent, map[string]any{}, ""},
		{core.EventEmailFailed, map[string]any{"scheduled": false}, ""},
		{core.EventPaymentCompleted, map[string]any{}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.event, func(t *testing.T) {
			var got *db.Notification
			repo := &mocks.NotificationRepository{CreateFn: func(ctx context.Context, n *db.Notification) (*db.Notification, error) {
				got = n
				return createEcho()(ctx, n)
			}}
			payload := map[string]any{"user_id": uid, core.EventIDKey: "evt-1"}
			for k, v := range tt.payload {
				payload[k] = v
			}
			require.NoError(t, newSvc(repo, nil).HandleEvent(context.Background(), tt.event, payload))
			if tt.wantType == "" {
				assert.Nil(t, got)
				return
			}
			require.NotNil(t, got)
			assert.Equal(t, tt.wantType, got.Type)
			assert.Equal(t, uid, got.UserID)
			assert.Equal(t, tt.payload, got.Data, "neither the user id nor the event id is stored as data")
			require.NotNil(t, got.EventID)
			assert.Equal(t, "evt-1", *got.EventID)
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
	assert.NoError(t, newSvc(repo, nil).HandleEvent(context.Background(), core.EventEmailOpened, map[string]any{}), "an event nobody can receive is not delivered again")
	assert.False(t, called)
}

func TestHandleEventReplayAndFailure(t *testing.T) {
	t.Run("the same event again changes nothing", func(t *testing.T) {
		pushed := 0
		repo := &mocks.NotificationRepository{CreateFn: func(context.Context, *db.Notification) (*db.Notification, error) {
			return nil, repositories.ErrConflict
		}}
		rt := realtimeFunc(func(context.Context, string, any) error { pushed++; return nil })
		err := newSvc(repo, rt).HandleEvent(context.Background(), core.EventEmailOpened, map[string]any{"user_id": uid, core.EventIDKey: "evt-1"})
		assert.NoError(t, err)
		assert.Zero(t, pushed, "nothing is pushed a second time")
	})
	t.Run("a storage failure asks for one more delivery", func(t *testing.T) {
		repo := &mocks.NotificationRepository{CreateFn: func(context.Context, *db.Notification) (*db.Notification, error) {
			return nil, errors.New("database down")
		}}
		err := newSvc(repo, nil).HandleEvent(context.Background(), core.EventEmailOpened, map[string]any{"user_id": uid, core.EventIDKey: "evt-1"})
		assert.Error(t, err)
	})
	t.Run("an event without an id is stored without one", func(t *testing.T) {
		var got *db.Notification
		repo := &mocks.NotificationRepository{CreateFn: func(ctx context.Context, n *db.Notification) (*db.Notification, error) {
			got = n
			return createEcho()(ctx, n)
		}}
		require.NoError(t, newSvc(repo, nil).HandleEvent(context.Background(), core.EventEmailOpened, map[string]any{"user_id": uid}))
		require.NotNil(t, got)
		assert.Nil(t, got.EventID)
	})
}

type realtimeFunc func(ctx context.Context, userID string, payload any) error

func (f realtimeFunc) Publish(ctx context.Context, userID string, payload any) error {
	return f(ctx, userID, payload)
}
