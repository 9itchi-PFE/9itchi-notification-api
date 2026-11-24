package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"9itchi-notification-api/models"
	"9itchi-notification-api/models/api"
	"9itchi-notification-api/services/mocks"

	"github.com/ably/ably-go/ably"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newRouter(svc *mocks.NotificationService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("user_id", "u1") })
	r.GET("/notifications", ListNotifications(svc))
	r.PUT("/notifications/:id/read", MarkRead(svc))
	r.PUT("/notifications/read-all", MarkAllRead(svc))
	r.GET("/notifications/ably-token", AblyToken(svc))
	r.POST("/internal/notifications", SendNotification(svc))
	r.GET("/notifications/preferences", GetAlertPreferences(svc))
	r.PUT("/notifications/preferences", UpdateAlertPreferences(svc))
	return r
}

func do(r *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}

func TestListNotificationsHandler(t *testing.T) {
	var gotQ api.ListNotificationsQuery
	var gotUser string
	svc := &mocks.NotificationService{ListFn: func(_ context.Context, userID string, q api.ListNotificationsQuery) (*api.NotificationListResponse, *models.ApiError) {
		gotUser, gotQ = userID, q
		return &api.NotificationListResponse{Items: []api.NotificationResponse{}, Total: 0, Page: 1, PageSize: 20, UnreadCount: 4}, nil
	}}
	w := do(newRouter(svc), http.MethodGet, "/notifications?unread=true&page=2&page_size=5", "")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "u1", gotUser)
	assert.Equal(t, api.ListNotificationsQuery{Page: 2, PageSize: 5, Unread: true}, gotQ)
	assert.JSONEq(t, `{"items":[],"total":0,"page":1,"page_size":20,"unread_count":4}`, w.Body.String())

	w = do(newRouter(svc), http.MethodGet, "/notifications?page_size=1000", "")
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), `"error"`)
}

func TestMarkReadHandler(t *testing.T) {
	svc := &mocks.NotificationService{ReadFn: func(_ context.Context, userID, id string) (*api.NotificationResponse, *models.ApiError) {
		if id == "missing" {
			return nil, &models.ApiError{Code: http.StatusNotFound, Error: errors.New("notification not found")}
		}
		return &api.NotificationResponse{ID: id, Read: true}, nil
	}}
	r := newRouter(svc)
	w := do(r, http.MethodPut, "/notifications/abc/read", "")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"read":true`)

	w = do(r, http.MethodPut, "/notifications/missing/read", "")
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "notification not found")
}

func TestMarkAllReadHandler(t *testing.T) {
	svc := &mocks.NotificationService{ReadAll: func(context.Context, string) (*api.MarkAllReadResponse, *models.ApiError) {
		return &api.MarkAllReadResponse{Updated: 2}, nil
	}}
	w := do(newRouter(svc), http.MethodPut, "/notifications/read-all", "")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"updated":2}`, w.Body.String())
}

func TestSendNotificationHandler(t *testing.T) {
	var got api.SendNotificationRequest
	svc := &mocks.NotificationService{SendFn: func(_ context.Context, req api.SendNotificationRequest) (*api.NotificationResponse, *models.ApiError) {
		got = req
		if req.Type == "bad" {
			return nil, &models.ApiError{Code: http.StatusBadRequest, Error: errors.New("invalid notification type")}
		}
		return &api.NotificationResponse{ID: "n1", Type: req.Type}, nil
	}}
	r := newRouter(svc)

	valid := `{"user_id":"11111111-1111-1111-1111-111111111111","type":"system","title":"hi","data":{"k":"v"}}`
	w := do(r, http.MethodPost, "/internal/notifications", valid)
	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Equal(t, "system", got.Type)
	assert.Equal(t, map[string]any{"k": "v"}, got.Data)

	w = do(r, http.MethodPost, "/internal/notifications", `{"user_id":"not-a-uuid","type":"system","title":"hi"}`)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	w = do(r, http.MethodPost, "/internal/notifications", `{"user_id":"11111111-1111-1111-1111-111111111111","type":"bad","title":"hi"}`)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "invalid notification type")
}

func TestAblyTokenHandler(t *testing.T) {
	svc := &mocks.NotificationService{AblyFunc: func(_ context.Context, userID string) (*api.AblyTokenResponse, *models.ApiError) {
		return &api.AblyTokenResponse{TokenParams: ably.TokenParams{Capability: `{"user:` + userID + `":["subscribe"]}`, ClientID: userID}, KeyName: "app.key"}, nil
	}}
	w := do(newRouter(svc), http.MethodGet, "/notifications/ably-token", "")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"capability":"{\"user:u1\":[\"subscribe\"]}","clientId":"u1","keyName":"app.key"}`, w.Body.String())

	svc.AblyFunc = func(context.Context, string) (*api.AblyTokenResponse, *models.ApiError) {
		return nil, &models.ApiError{Code: http.StatusServiceUnavailable, Error: errors.New("realtime unavailable")}
	}
	w = do(newRouter(svc), http.MethodGet, "/notifications/ably-token", "")
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.Contains(t, w.Body.String(), `"error":"realtime unavailable"`)
}

func TestAlertPreferencesHandlers(t *testing.T) {
	var got api.UpdateAlertPreferencesRequest
	svc := &mocks.NotificationService{
		PrefsFn: func(_ context.Context, userID string) (*api.AlertPreferencesResponse, *models.ApiError) {
			assert.Equal(t, "u1", userID)
			return &api.AlertPreferencesResponse{Items: []api.AlertPreferenceResponse{{Kind: "tips", Email: false, InApp: true}}}, nil
		},
		SetPrefs: func(_ context.Context, userID string, req api.UpdateAlertPreferencesRequest) (*api.AlertPreferencesResponse, *models.ApiError) {
			assert.Equal(t, "u1", userID)
			got = req
			return &api.AlertPreferencesResponse{Items: []api.AlertPreferenceResponse{{Kind: "tips", Email: true, InApp: false}}}, nil
		},
	}
	r := newRouter(svc)

	w := do(r, http.MethodGet, "/notifications/preferences", "")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"items":[{"kind":"tips","email":false,"in_app":true}]}`, w.Body.String())

	w = do(r, http.MethodPut, "/notifications/preferences", `{"items":[{"kind":"tips","email":true,"in_app":false}]}`)
	assert.Equal(t, http.StatusOK, w.Code)
	require.Len(t, got.Items, 1)
	assert.True(t, *got.Items[0].Email)
	assert.False(t, *got.Items[0].InApp, "false is a choice, not a missing field")

	for _, body := range []string{
		`{`, `{}`, `{"items":[]}`,
		`{"items":[{"kind":"payments","email":true,"in_app":true}]}`,
		`{"items":[{"kind":"tips","email":true}]}`,
		`{"items":[{"kind":"tips","in_app":true}]}`,
	} {
		w = do(r, http.MethodPut, "/notifications/preferences", body)
		assert.Equal(t, http.StatusBadRequest, w.Code, body)
		assert.Contains(t, w.Body.String(), `"error"`)
	}

	failing := newRouter(&mocks.NotificationService{PrefsFn: func(context.Context, string) (*api.AlertPreferencesResponse, *models.ApiError) {
		return nil, &models.ApiError{Code: http.StatusInternalServerError, Error: errors.New("internal server error")}
	}})
	w = do(failing, http.MethodGet, "/notifications/preferences", "")
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
