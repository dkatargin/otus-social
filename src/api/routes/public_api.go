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
		publicEndpoints.GET("user/search", handlers.UserSearch)
		publicEndpoints.GET("user/get/:id", handlers.UserGet)
		publicEndpoints.POST("user/register", handlers.UserRegister)
	}
	return publicEndpoints
}
