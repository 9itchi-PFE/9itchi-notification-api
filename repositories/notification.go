package repositories

import (
	"context"

	"9itchi-notification-api/models/db"

	"github.com/jackc/pgx/v5"
)

const notificationColumns = `id, user_id, type, title, data, read, event_id, created_at, updated_at`

// NotificationRepository is the only layer that touches PostgreSQL for notification data.
type NotificationRepository interface {
	// Create returns ErrConflict when a notification already exists for the same event id.
	Create(ctx context.Context, n *db.Notification) (*db.Notification, error)
	List(ctx context.Context, userID string, unreadOnly bool, limit, offset int) ([]db.Notification, error)
	Count(ctx context.Context, userID string, unreadOnly bool) (int, error)
	// MarkRead returns ErrNotFound when the row does not exist or belongs to another user.
	MarkRead(ctx context.Context, userID, id string) (*db.Notification, error)
	MarkAllRead(ctx context.Context, userID string) (int64, error)
}

type notificationRepository struct {
	store *Store
}

func NewNotificationRepository(store *Store) NotificationRepository {
	return &notificationRepository{store: store}
}

func (r *notificationRepository) Create(ctx context.Context, n *db.Notification) (*db.Notification, error) {
	ctx, cancel := QueryTimeout(ctx)
	defer cancel()
	data := n.Data
	if data == nil {
		data = map[string]any{}
	}
	rows, err := r.store.Conn(ctx).Query(ctx,
		`INSERT INTO notifications (user_id, type, title, data, event_id) VALUES ($1, $2, $3, $4, $5) RETURNING `+notificationColumns,
		n.UserID, n.Type, n.Title, data, n.EventID)
	if err != nil {
		return nil, MapError(err)
	}
	out, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[db.Notification])
	return out, MapError(err)
}

func (r *notificationRepository) List(ctx context.Context, userID string, unreadOnly bool, limit, offset int) ([]db.Notification, error) {
	ctx, cancel := QueryTimeout(ctx)
	defer cancel()
	rows, err := r.store.Conn(ctx).Query(ctx,
		`SELECT `+notificationColumns+` FROM notifications
		 WHERE user_id = $1 AND (NOT $2 OR read = false)
		 ORDER BY created_at DESC, id DESC LIMIT $3 OFFSET $4`,
		userID, unreadOnly, limit, offset)
	if err != nil {
		return nil, MapError(err)
	}
	items, err := pgx.CollectRows(rows, pgx.RowToStructByName[db.Notification])
	return items, MapError(err)
}

func (r *notificationRepository) Count(ctx context.Context, userID string, unreadOnly bool) (int, error) {
	ctx, cancel := QueryTimeout(ctx)
	defer cancel()
	var n int
	err := r.store.Conn(ctx).QueryRow(ctx,
		`SELECT count(*) FROM notifications WHERE user_id = $1 AND (NOT $2 OR read = false)`,
		userID, unreadOnly).Scan(&n)
	return n, MapError(err)
}

func (r *notificationRepository) MarkRead(ctx context.Context, userID, id string) (*db.Notification, error) {
	ctx, cancel := QueryTimeout(ctx)
	defer cancel()
	rows, err := r.store.Conn(ctx).Query(ctx,
		`UPDATE notifications SET read = true, updated_at = now() WHERE id = $1 AND user_id = $2 RETURNING `+notificationColumns,
		id, userID)
	if err != nil {
		return nil, MapError(err)
	}
	out, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[db.Notification])
	return out, MapError(err)
}

func (r *notificationRepository) MarkAllRead(ctx context.Context, userID string) (int64, error) {
	ctx, cancel := QueryTimeout(ctx)
	defer cancel()
	tag, err := r.store.Conn(ctx).Exec(ctx,
		`UPDATE notifications SET read = true, updated_at = now() WHERE user_id = $1 AND read = false`, userID)
	if err != nil {
		return 0, MapError(err)
	}
	return tag.RowsAffected(), nil
}
