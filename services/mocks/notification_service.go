package mocks

import (
	"context"

	"9itchi-notification-api/models"
	"9itchi-notification-api/models/api"
)

// NotificationService is a hand-written mock of services.NotificationService for handler tests.
type NotificationService struct {
	ListFn   func(ctx context.Context, userID string, q api.ListNotificationsQuery) (*api.NotificationListResponse, *models.ApiError)
	ReadFn   func(ctx context.Context, userID, id string) (*api.NotificationResponse, *models.ApiError)
	ReadAll  func(ctx context.Context, userID string) (*api.MarkAllReadResponse, *models.ApiError)
	SendFn   func(ctx context.Context, req api.SendNotificationRequest) (*api.NotificationResponse, *models.ApiError)
	EventFn  func(ctx context.Context, event string, payload map[string]any)
	AblyFunc func(ctx context.Context, userID string) (*api.AblyTokenResponse, *models.ApiError)
}

func (m *NotificationService) ListNotifications(ctx context.Context, userID string, q api.ListNotificationsQuery) (*api.NotificationListResponse, *models.ApiError) {
	return m.ListFn(ctx, userID, q)
}

func (m *NotificationService) MarkRead(ctx context.Context, userID, id string) (*api.NotificationResponse, *models.ApiError) {
	return m.ReadFn(ctx, userID, id)
}

func (m *NotificationService) MarkAllRead(ctx context.Context, userID string) (*api.MarkAllReadResponse, *models.ApiError) {
	return m.ReadAll(ctx, userID)
}

func (m *NotificationService) AblyToken(ctx context.Context, userID string) (*api.AblyTokenResponse, *models.ApiError) {
	return m.AblyFunc(ctx, userID)
}

func (m *NotificationService) SendNotification(ctx context.Context, req api.SendNotificationRequest) (*api.NotificationResponse, *models.ApiError) {
	return m.SendFn(ctx, req)
}

func (m *NotificationService) HandleEvent(ctx context.Context, event string, payload map[string]any) {
	m.EventFn(ctx, event, payload)
}
