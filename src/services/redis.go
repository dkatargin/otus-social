package services

import (
	"context"
	"fmt"
	"social/config"

	"github.com/go-redis/redis/v8"
)

var RedisClient *redis.Client
var QueueServiceInstance *QueueService

func InitRedis() error {
	if config.AppConfig == nil {
		return fmt.Errorf("AppConfig is not loaded")
	}

	redisConfig := config.AppConfig.Redis
	RedisClient = redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", redisConfig.Host, redisConfig.Port),
		Password: redisConfig.Password,
		DB:       redisConfig.DB,
	})

	// Тест соединения
	_, err := RedisClient.Ping(context.Background()).Result()
	if err != nil {
		return fmt.Errorf("failed to connect to Redis: %w", err)
	}

	// Инициализируем сервис очереди
	QueueServiceInstance = NewQueueService()

	return nil
}

func CloseRedis() error {
	if RedisClient != nil {
		return RedisClient.Close()
	}
	return nil
}
