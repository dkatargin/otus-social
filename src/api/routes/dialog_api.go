package routes

import (
	"social/api/handlers"

	"github.com/gin-gonic/gin"
)

func DialogInternalApi(router *gin.Engine) *gin.RouterGroup {
	dialogInternalEndpoints := router.Group("/v1/")
	{
		dialogInternalEndpoints.POST("messages/send", handlers.SendMessageInternalHandler)
		dialogInternalEndpoints.POST("messages/list", handlers.ListDialogInternalHandler)
	}
	return dialogInternalEndpoints
}
