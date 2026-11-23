package mocks

import (
	"context"

	"9itchi-notification-api/models/db"
)

// NotificationRepository is a hand-written mock of repositories.NotificationRepository.
// Set the *Fn fields a test needs; unset ones panic (nil func call) to flag unexpected use.
type NotificationRepository struct {
	CreateFn      func(ctx context.Context, n *db.Notification) (*db.Notification, error)
	ListFn        func(ctx context.Context, userID string, unreadOnly bool, limit, offset int) ([]db.Notification, error)
	CountFn       func(ctx context.Context, userID string, unreadOnly bool) (int, error)
	MarkReadFn    func(ctx context.Context, userID, id string) (*db.Notification, error)
	MarkAllReadFn func(ctx context.Context, userID string) (int64, error)
}

func (m *NotificationRepository) Create(ctx context.Context, n *db.Notification) (*db.Notification, error) {
	return m.CreateFn(ctx, n)
}

func (m *NotificationRepository) List(ctx context.Context, userID string, unreadOnly bool, limit, offset int) ([]db.Notification, error) {
	return m.ListFn(ctx, userID, unreadOnly, limit, offset)
}

func (m *NotificationRepository) Count(ctx context.Context, userID string, unreadOnly bool) (int, error) {
	return m.CountFn(ctx, userID, unreadOnly)
}

func (m *NotificationRepository) MarkRead(ctx context.Context, userID, id string) (*db.Notification, error) {
	return m.MarkReadFn(ctx, userID, id)
}

func (m *NotificationRepository) MarkAllRead(ctx context.Context, userID string) (int64, error) {
	return m.MarkAllReadFn(ctx, userID)
}
