package main

import (
	"fmt"
	"log"
	"social/api/handlers"
	"social/api/middleware"
	"social/config"
	"social/db"
	"social/services"

	"github.com/gin-gonic/gin"
)

func setupRedisDialogRoutes(r *gin.Engine, redisService *services.RedisDialogService) {
	// Создаем обработчики для Redis диалогов
	redisHandlers := handlers.NewRedisDialogHandlers(redisService)

	// API группа с аутентификацией
	api := r.Group("/api/v1")
	api.Use(middleware.AuthMiddleware())

	// Redis диалоги маршруты (с префиксом /redis для тестирования)
	redisDialogs := api.Group("/redis/dialog")
	{
		redisDialogs.POST("/:user_id/send", redisHandlers.SendMessageHandler)
		redisDialogs.GET("/:user_id/list", redisHandlers.ListDialogHandler)
		redisDialogs.GET("/:user_id/stats", redisHandlers.GetDialogStatsHandler)
		redisDialogs.POST("/:user_id/read", redisHandlers.MarkAsReadHandler)
	}
}

func initializeRedisDialogService() *services.RedisDialogService {
	if config.AppConfig == nil {
		log.Fatal("Config not loaded")
	}

	redisAddr := fmt.Sprintf("%s:%d",
		config.AppConfig.RedisDialogs.Host,
		config.AppConfig.RedisDialogs.Port)

	return services.NewRedisDialogService(
		redisAddr,
		config.AppConfig.RedisDialogs.Password,
		config.AppConfig.RedisDialogs.DB,
	)
}

// Обновленная функция main с поддержкой Redis диалогов
func mainWithRedisDialogs() {
	// Загружаем конфигурацию
	if err := config.LoadConfig("etc/app.yaml"); err != nil {
		log.Fatal("Failed to load config:", err)
	}

	// Инициализируем базу данных
	if err := db.InitDB(); err != nil {
		log.Fatal("Failed to initialize database:", err)
	}

	// Инициализируем Redis сервис для диалогов
	redisService := initializeRedisDialogService()
	defer redisService.Close()

	// Настраиваем Gin
	r := gin.Default()

	// Добавляем middleware
	r.Use(middleware.CORSMiddleware())
	r.Use(middleware.LoggingMiddleware())

	// Настраиваем существующие роуты
	setupRoutes(r)

	// Добавляем Redis диалоги роуты
	setupRedisDialogRoutes(r, redisService)

	// Запускаем сервер
	addr := fmt.Sprintf("%s:%d",
		config.AppConfig.Backend.Host,
		config.AppConfig.Backend.Port)

	log.Printf("Server starting on %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatal("Failed to start server:", err)
	}
}

// Функция для настройки существующих роутов (заглушка)
func setupRoutes(r *gin.Engine) {
	// Здесь должны быть существующие роуты
	// Эта функция должна быть реализована в основном server.go
}

// main - основная функция для запуска сервера с Redis диалогами
func main() {
	// Используем обновленную функцию с Redis диалогами
	mainWithRedisDialogs()
}
