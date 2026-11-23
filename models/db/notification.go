package db

import "time"

// Notification maps table notifications.
type Notification struct {
	ID        string         `db:"id" json:"id"`
	UserID    string         `db:"user_id" json:"user_id"`
	Type      string         `db:"type" json:"type"`
	Title     string         `db:"title" json:"title"`
	Data      map[string]any `db:"data" json:"-"`
	Read      bool           `db:"read" json:"read"`
	CreatedAt time.Time      `db:"created_at" json:"created_at"`
	UpdatedAt time.Time      `db:"updated_at" json:"updated_at"`
}
