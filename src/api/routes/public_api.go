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

		// Друзья
		publicEndpoints.POST("friends/add", handlers.AddFriend)
		publicEndpoints.POST("friends/approve", handlers.ApproveFriend)
		publicEndpoints.POST("friends/delete", handlers.DeleteFriend)
		publicEndpoints.GET("friends/list", handlers.GetFriends)
		publicEndpoints.GET("friends/requests", handlers.GetPendingRequests)
	}
	return publicEndpoints
}
