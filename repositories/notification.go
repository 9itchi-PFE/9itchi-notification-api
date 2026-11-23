package repositories

// NotificationRepository is the only layer that touches PostgreSQL for notification data.
// Use store.Conn(ctx), a context from QueryTimeout, and return MapError(err).
type NotificationRepository interface {
}

type notificationRepository struct {
	store *Store
}

func NewNotificationRepository(store *Store) NotificationRepository {
	return &notificationRepository{store: store}
}
