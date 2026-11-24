package db

import "time"

// AlertPreference maps table alert_preferences: where one kind of alert goes for one user.
type AlertPreference struct {
	UserID    string    `db:"user_id" json:"user_id"`
	Kind      string    `db:"kind" json:"kind"`
	Email     bool      `db:"email" json:"email"`
	InApp     bool      `db:"in_app" json:"in_app"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}
