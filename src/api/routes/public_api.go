package routes

import (
	"social/api/handlers"

	"github.com/gin-gonic/gin"
)

func PublicApi(router *gin.Engine) *gin.RouterGroup {
	publicEndpoints := router.Group("/api/v1/")
	{
		publicEndpoints.POST("auth/register", handlers.Register)
		publicEndpoints.POST("auth/login", handlers.Login)
		publicEndpoints.POST("auth/logout", handlers.Logout)
		publicEndpoints.GET("users/search", handlers.UserSearch)
		publicEndpoints.GET("users/get/:id", handlers.UserGet)
	}

	return publicEndpoints
}
