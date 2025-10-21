package main

import (
	"log"
	"social/api/routes"
	"social/services"

	"github.com/gin-gonic/gin"
)

func main() {

	router := gin.Default()

	router.Use(gin.Logger())
	router.Use(gin.Recovery())

	// Инициализируем Redis
	err := services.InitRedis()
	if err != nil {
		panic("Failed to connect to Redis: " + err.Error())
	}
	defer services.CloseRedis()

	// Инициализируем сервис очередей
	services.InitQueueService()

	// Затем инициализируем DialogService
	if err := services.InitRedisDialogService(); err != nil {
		log.Fatalf("Failed to init RedisDialogService: %v", err)
	}

	routes.DialogInternalApi(router)

	if err := router.Run(":8080"); err != nil {
		panic(err)
	}

}
