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
			// WebSocket для ленты
			authenticated.GET("ws/feed", handlers.WSFeedHandler)
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

			// Диалоги
			authenticated.POST("dialog/:user_id/send", handlers.SendMessagePublicHandler)
			authenticated.GET("dialog/:user_id/list", handlers.ListDialogPublicHandler)
			authenticated.POST("dialog/:user_id/read", handlers.MarkDialogAsReadHandler)

			// Счетчики
			authenticated.GET("counters", handlers.GetCounters)
			authenticated.GET("counters/stats", handlers.GetCounterStats)
			authenticated.GET("counters/unread-messages", handlers.GetUnreadMessagesCount)
			authenticated.GET("counters/:type", handlers.GetCounterByType)
			authenticated.POST("counters/:type/reset", handlers.ResetCounter)
			authenticated.POST("counters/:type/reconcile", handlers.ReconcileCounter)
			authenticated.POST("counters/:type/increment", handlers.IncrementCounter)
			authenticated.GET("counters/dialogs/:user_id", handlers.GetDialogCounters)
			authenticated.POST("counters/batch", handlers.GetBatchCounters)
		}

		// Админские эндпоинты (без аутентификации для простоты)
		publicEndpoints.DELETE("admin/cache/feed/:user_id", handlers.InvalidateUserFeed)
		publicEndpoints.POST("admin/feed/rebuild/:user_id", handlers.RebuildUserFeed)
		publicEndpoints.POST("admin/feed/rebuild-all", handlers.RebuildAllFeeds)
		publicEndpoints.GET("admin/queue/stats", handlers.GetQueueStats)
	}
	return publicEndpoints
}
