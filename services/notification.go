package services

import (
	"context"

	"9itchi-notification-api/models"
	"9itchi-notification-api/models/api"
	"9itchi-notification-api/repositories"
)

// NotificationService holds all business logic for notification. Handlers stay thin.
type NotificationService interface {
	// GET /notifications: own notifications
	ListNotifications(ctx context.Context, userID string) (any, *models.ApiError)
	// PUT /notifications/:id/read: mark one as read
	MarkRead(ctx context.Context, userID string, id string) (any, *models.ApiError)
	// PUT /notifications/read-all: mark all as read
	MarkAllRead(ctx context.Context, userID string) (any, *models.ApiError)
	// GET /notifications/ably-token: scoped Ably token request for user:{id} channels
	AblyToken(ctx context.Context, userID string) (any, *models.ApiError)
	// POST /internal/notifications: create + push a notification
	SendNotification(ctx context.Context, req api.SendNotificationRequest) (any, *models.ApiError)
}

type notificationService struct {
	repo repositories.NotificationRepository
}

func NewNotificationService(repo repositories.NotificationRepository) NotificationService {
	return &notificationService{repo: repo}
}

func (s *notificationService) ListNotifications(_ context.Context, _ string) (any, *models.ApiError) {
	return nil, models.NotImplemented()
}

func (s *notificationService) MarkRead(_ context.Context, _ string, _ string) (any, *models.ApiError) {
	return nil, models.NotImplemented()
}

func (s *notificationService) MarkAllRead(_ context.Context, _ string) (any, *models.ApiError) {
	return nil, models.NotImplemented()
}

func (s *notificationService) AblyToken(_ context.Context, _ string) (any, *models.ApiError) {
	return nil, models.NotImplemented()
}

func (s *notificationService) SendNotification(_ context.Context, _ api.SendNotificationRequest) (any, *models.ApiError) {
	return nil, models.NotImplemented()
}
