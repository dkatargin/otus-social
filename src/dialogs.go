package main

import (
	"log"
	"social/api/middleware"
	"social/api/routes"
	"social/config"
	"social/services"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {

	// Загружаем конфигурацию
	if err := config.LoadConfig("config.yaml"); err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	router := gin.Default()

	router.Use(gin.Logger())
	router.Use(gin.Recovery())
	router.Use(middleware.PrometheusMiddleware("dialogs"))

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

	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	routes.DialogInternalApi(router)

	if err := router.Run(":8080"); err != nil {
		panic(err)
	}

}
