package routes

import (
	"social/api/handlers"
	"social/services"

	"github.com/gin-gonic/gin"
)

func mockAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetHeader("X-User-ID")
		if userID == "" {
			c.AbortWithStatusJSON(401, gin.H{"error": "Unauthorized"})
			return
		}
		c.Set("user_id", userID)
		c.Next()
	}
}

func DialogInternalApi(router *gin.Engine) *gin.RouterGroup {
	redisDialogService := services.GetRedisDialogService()
	redisHandlers := handlers.NewRedisDialogHandlers(redisDialogService)

	dialogInternalEndpoints := router.Group("/v1/")
	{
		dialogInternalEndpoints.POST("messages/send", handlers.SendMessageInternalHandler)
		dialogInternalEndpoints.POST("messages/list", handlers.ListDialogInternalHandler)
	}

	dialogGroup := router.Group("/dialog")
	dialogGroup.Use(mockAuthMiddleware())
	{
		dialogGroup.POST("/:user_id/send", redisHandlers.SendMessageHandler)
		dialogGroup.GET("/:user_id/list", redisHandlers.ListDialogHandler)
		dialogGroup.POST("/:user_id/read", redisHandlers.MarkAsReadHandler)
		dialogGroup.GET("/:user_id/stats", redisHandlers.GetDialogStatsHandler)
	}

	return dialogInternalEndpoints
}
