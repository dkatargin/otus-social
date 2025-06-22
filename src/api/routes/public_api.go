package routes

import (
	"github.com/gin-gonic/gin"
	"social/api/handlers"
)

func PublicApi(router *gin.Engine) *gin.RouterGroup {
	publicEndpoints := router.Group("/api/v1/")
	{
		publicEndpoints.POST("auth/register", handlers.Register)
		publicEndpoints.POST("auth/login", handlers.Login)
		publicEndpoints.POST("auth/logout", handlers.Logout)
		publicEndpoints.GET("user/search", handlers.UserSearch)
	}

	return publicEndpoints
}
