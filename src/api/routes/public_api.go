package routes

import (
	"social/api/handlers"
	"social/api/middleware"

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

		// Эндпоинты, требующие аутентификации
		authenticated := publicEndpoints.Group("/")
		authenticated.Use(middleware.TestAuthMiddleware())
		{
			// Друзья
			authenticated.POST("friends/add", handlers.AddFriend)
			authenticated.POST("friends/approve", handlers.ApproveFriend)
			authenticated.POST("friends/delete", handlers.DeleteFriend)
			authenticated.GET("friends/list", handlers.GetFriends)
			authenticated.GET("friends/requests", handlers.GetPendingRequests)

			// Посты и лента
			authenticated.POST("posts/create", handlers.CreatePost)
			authenticated.DELETE("posts/:post_id", handlers.DeletePost)
			authenticated.GET("feed", handlers.GetFeed)
		}

		// Админские эндпоинты (без аутентификации для простоты)
		publicEndpoints.DELETE("admin/cache/feed/:user_id", handlers.InvalidateUserFeed)
		publicEndpoints.POST("admin/feed/rebuild/:user_id", handlers.RebuildUserFeed)
		publicEndpoints.POST("admin/feed/rebuild-all", handlers.RebuildAllFeeds)
		publicEndpoints.GET("admin/queue/stats", handlers.GetQueueStats)
	}
	return publicEndpoints
}
