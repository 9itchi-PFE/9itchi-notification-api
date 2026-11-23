package handlers

import (
	"net/http"

	"9itchi-notification-api/models"
	"9itchi-notification-api/models/api"
	"9itchi-notification-api/services"

	"github.com/gin-gonic/gin"
)

// ListNotifications: GET /notifications (user) own notifications.
func ListNotifications(svc services.NotificationService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var q api.ListNotificationsQuery
		if err := c.ShouldBindQuery(&q); err != nil {
			models.FormatError(c, &models.ApiError{Code: http.StatusBadRequest, Error: err})
			return
		}
		result, apiErr := svc.ListNotifications(c.Request.Context(), c.GetString("user_id"), q)
		if apiErr != nil {
			models.FormatError(c, apiErr)
			return
		}
		c.JSON(http.StatusOK, result)
	}
}

// MarkRead: PUT /notifications/:id/read (user) mark one as read.
func MarkRead(svc services.NotificationService) gin.HandlerFunc {
	return func(c *gin.Context) {
		result, apiErr := svc.MarkRead(c.Request.Context(), c.GetString("user_id"), c.Param("id"))
		if apiErr != nil {
			models.FormatError(c, apiErr)
			return
		}
		c.JSON(http.StatusOK, result)
	}
}

// MarkAllRead: PUT /notifications/read-all (user) mark all as read.
func MarkAllRead(svc services.NotificationService) gin.HandlerFunc {
	return func(c *gin.Context) {
		result, apiErr := svc.MarkAllRead(c.Request.Context(), c.GetString("user_id"))
		if apiErr != nil {
			models.FormatError(c, apiErr)
			return
		}
		c.JSON(http.StatusOK, result)
	}
}

// AblyToken: GET /notifications/ably-token (user) scoped Ably token request for user:{id} channels.
func AblyToken(svc services.NotificationService) gin.HandlerFunc {
	return func(c *gin.Context) {
		result, apiErr := svc.AblyToken(c.Request.Context(), c.GetString("user_id"))
		if apiErr != nil {
			models.FormatError(c, apiErr)
			return
		}
		c.JSON(http.StatusOK, result)
	}
}

// SendNotification: POST /internal/notifications (internal) create + push a notification.
func SendNotification(svc services.NotificationService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req api.SendNotificationRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			models.FormatError(c, &models.ApiError{Code: http.StatusBadRequest, Error: err})
			return
		}
		result, apiErr := svc.SendNotification(c.Request.Context(), req)
		if apiErr != nil {
			models.FormatError(c, apiErr)
			return
		}
		c.JSON(http.StatusCreated, result)
	}
}
