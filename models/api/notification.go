package api

import (
	"time"

	"github.com/ably/ably-go/ably"
)

// SendNotificationRequest is the validated body of SendNotification.
type SendNotificationRequest struct {
	UserID string         `json:"user_id" binding:"required,uuid"`
	Type   string         `json:"type" binding:"required,max=50"`
	Title  string         `json:"title" binding:"required,max=200"`
	Data   map[string]any `json:"data"`
	// Channels lists extra delivery channels (e.g. "email"); in-app is always used.
	Channels []string `json:"channels" binding:"omitempty,max=5,dive,max=20"`
}

// NotificationResponse is one notification.
type NotificationResponse struct {
	ID        string         `json:"id"`
	UserID    string         `json:"user_id"`
	Type      string         `json:"type"`
	Title     string         `json:"title"`
	Data      map[string]any `json:"data"`
	Read      bool           `json:"read"`
	CreatedAt time.Time      `json:"created_at"`
}

// NotificationListResponse is a page of notifications plus the user's total unread count.
type NotificationListResponse struct {
	Items       []NotificationResponse `json:"items"`
	Total       int                    `json:"total"`
	Page        int                    `json:"page"`
	PageSize    int                    `json:"page_size"`
	UnreadCount int                    `json:"unread_count"`
}

// MarkAllReadResponse reports how many notifications changed.
type MarkAllReadResponse struct {
	Updated int64 `json:"updated"`
}

// AblyTokenResponse is the signed Ably TokenRequest, returned as-is so the Ably client can use it as authUrl output.
type AblyTokenResponse = ably.TokenRequest

// ListNotificationsQuery holds the query-string filters of ListNotifications.
type ListNotificationsQuery struct {
	Page     int  `form:"page" binding:"omitempty,min=1"`
	PageSize int  `form:"page_size" binding:"omitempty,min=1,max=100"`
	Unread   bool `form:"unread"`
}
