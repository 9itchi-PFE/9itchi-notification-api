package mapping

import (
	"9itchi-notification-api/models/api"
	"9itchi-notification-api/models/db"
)

// NotificationToResponse converts a row to its DTO (never nil data).
func NotificationToResponse(n *db.Notification) api.NotificationResponse {
	data := n.Data
	if data == nil {
		data = map[string]any{}
	}
	return api.NotificationResponse{
		ID:        n.ID,
		UserID:    n.UserID,
		Type:      n.Type,
		Title:     n.Title,
		Data:      data,
		Read:      n.Read,
		CreatedAt: n.CreatedAt,
	}
}
