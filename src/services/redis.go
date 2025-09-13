package services

import (
	"context"
	"fmt"
	"log"
	"os"
	"social/config"

	"github.com/go-redis/redis/v8"
)

var RedisClient *redis.Client

func InitRedis() error {
	// Проверяем переменные окружения для тестового режима
	redisHost := os.Getenv("REDIS_HOST")
	redisPort := os.Getenv("REDIS_PORT")

	if redisHost == "" {
		if config.AppConfig != nil {
			redisHost = config.AppConfig.Redis.Host
		} else {
			redisHost = "localhost"
		}
	}

	if redisPort == "" {
		if config.AppConfig != nil {
			redisPort = fmt.Sprintf("%d", config.AppConfig.Redis.Port)
		} else {
			redisPort = "6380" // Тестовый порт
		}
	}

	RedisClient = redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", redisHost, redisPort),
		Password: "", // Пустой пароль для тестов
		DB:       0,
	})

	// Тест соединения
	_, err := RedisClient.Ping(context.Background()).Result()
	if err != nil {
		return fmt.Errorf("failed to connect to Redis at %s:%s: %w", redisHost, redisPort, err)
	}

	log.Printf("Redis connected successfully at %s:%s", redisHost, redisPort)
	return nil
}

func CloseRedis() error {
	if RedisClient != nil {
		return RedisClient.Close()
	}
	return nil
}
