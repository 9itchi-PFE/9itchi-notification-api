package api

import "time"

// SendNotificationRequest is the validated body of SendNotification.
type SendNotificationRequest struct {
	UserID string         `json:"user_id" binding:"required,uuid"`
	Type   string         `json:"type" binding:"required,max=50"`
	Title  string         `json:"title" binding:"required,max=200"`
	Data   map[string]any `json:"data"`
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

// AblyTokenResponse is the scoped Ably token request (not implemented yet).
type AblyTokenResponse struct {
	TokenRequest map[string]any `json:"token_request"`
}

// ListNotificationsQuery holds the query-string filters of ListNotifications.
type ListNotificationsQuery struct {
	Page     int  `form:"page" binding:"omitempty,min=1"`
	PageSize int  `form:"page_size" binding:"omitempty,min=1,max=100"`
	Unread   bool `form:"unread"`
}
