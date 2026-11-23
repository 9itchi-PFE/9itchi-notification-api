package router

import (
	"9itchi-notification-api/handlers"
	"9itchi-notification-api/services"

	"github.com/gin-gonic/gin"
)

func InitializeRouter(services *services.Services) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())

	initializeMiddlewares(r)
	initializeRoutes(r, services)

	return r
}

func initializeRoutes(r *gin.Engine, services *services.Services) {
	// Health endpoints
	r.GET("/", handlers.GetHealth)
	r.GET("/health", handlers.GetHealth)

	// user endpoints
	userGroup := r.Group("/")
	userGroup.Use(AuthMiddleware())
	{
		userGroup.GET("/notifications", handlers.ListNotifications(services.Notification))
		userGroup.PUT("/notifications/:id/read", handlers.MarkRead(services.Notification))
		userGroup.PUT("/notifications/read-all", handlers.MarkAllRead(services.Notification))
		userGroup.GET("/notifications/ably-token", handlers.AblyToken(services.Notification))
	}

	// internal endpoints
	internalGroup := r.Group("/")
	internalGroup.Use(InternalAPIKeyMiddleware())
	{
		internalGroup.POST("/internal/notifications", handlers.SendNotification(services.Notification))
	}

	// Fallback
	r.NoRoute(handlers.NoRoute)
}
