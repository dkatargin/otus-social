package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"social/models"
	"time"

	"github.com/go-redis/redis/v8"
)

const (
	FEED_UPDATE_QUEUE    = "feed_update_queue"
	QUEUE_WORKER_COUNT   = 5
	CELEBRITY_THRESHOLD  = 1000 // Порог друзей для celebrity пользователей
	CELEBRITY_BATCH_SIZE = 100  // Размер батча для celebrity
)

// FeedUpdateTask представляет задачи для обновления лент
type FeedUpdateTask struct {
	UserID int64       `json:"user_id"`
	Post   models.Post `json:"post"`
	Action string      `json:"action"` // "create", "delete"
}

type QueueService struct {
	postService *PostService
}

func NewQueueService() *QueueService {
	return &QueueService{
		postService: NewPostService(),
	}
}

// StartWorkers запускает воркеры для обработки очереди
func (qs *QueueService) StartWorkers(ctx context.Context) {
	for i := 0; i < QUEUE_WORKER_COUNT; i++ {
		go qs.worker(ctx, i)
	}
}

// worker обрабатывает задачи из очереди
func (qs *QueueService) worker(ctx context.Context, workerID int) {
	log.Printf("Feed update worker %d started", workerID)

	for {
		select {
		case <-ctx.Done():
			log.Printf("Feed update worker %d stopping", workerID)
			return
		default:
			// Получаем задачу из очереди (блокирующий вызов с таймаутом)
			result, err := RedisClient.BLPop(ctx, 5*time.Second, FEED_UPDATE_QUEUE).Result()
			if err != nil {
				if err == redis.Nil {
					// Таймаут - продолжаем
					continue
				}
				log.Printf("Worker %d error getting task: %v", workerID, err)
				time.Sleep(time.Second)
				continue
			}

			if len(result) < 2 {
				continue
			}

			// Десериализуем задачу
			var task FeedUpdateTask
			if err := json.Unmarshal([]byte(result[1]), &task); err != nil {
				log.Printf("Worker %d error unmarshaling task: %v", workerID, err)
				continue
			}

			// Обрабатываем задачу
			qs.processTask(ctx, &task, workerID)
		}
	}
}

// processTask обрабатывает конкретную задачу
func (qs *QueueService) processTask(ctx context.Context, task *FeedUpdateTask, workerID int) {
	log.Printf("Worker %d processing task for user %d, action: %s", workerID, task.UserID, task.Action)

	switch task.Action {
	case "create":
		qs.processCreatePost(ctx, task)
	case "delete":
		qs.processDeletePost(ctx, task)
	default:
		log.Printf("Worker %d unknown action: %s", workerID, task.Action)
	}
}

// processCreatePost обрабатывает создание поста
func (qs *QueueService) processCreatePost(ctx context.Context, task *FeedUpdateTask) {
	// Используем метод updateFriendsFeeds из PostService
	qs.postService.updateFriendsFeeds(ctx, task.UserID, &task.Post)
}

// processDeletePost обрабатывает удаление поста
func (qs *QueueService) processDeletePost(ctx context.Context, task *FeedUpdateTask) {
	// Удаляем пост из кешей лент
	qs.postService.removePostFromFeeds(ctx, task.UserID, task.Post.ID)
}

// EnqueueFeedUpdate добавляет задачу обновления ленты в очередь
func (qs *QueueService) EnqueueFeedUpdate(ctx context.Context, userID int64, post models.Post, action string) error {
	if RedisClient == nil {
		return fmt.Errorf("redis not available")
	}

	task := FeedUpdateTask{
		UserID: userID,
		Post:   post,
		Action: action,
	}

	taskData, err := json.Marshal(task)
	if err != nil {
		return fmt.Errorf("failed to marshal task: %w", err)
	}

	err = RedisClient.RPush(ctx, FEED_UPDATE_QUEUE, taskData).Err()
	if err != nil {
		return fmt.Errorf("failed to enqueue task: %w", err)
	}

	log.Printf("Enqueued feed update task for user %d, action: %s", userID, action)
	return nil
}

// GetStats возвращает статистику очереди
func (qs *QueueService) GetStats() map[string]interface{} {
	stats := make(map[string]interface{})

	if RedisClient != nil {
		ctx := context.Background()
		queueLength := RedisClient.LLen(ctx, FEED_UPDATE_QUEUE).Val()
		stats["queue_length"] = queueLength
		stats["worker_count"] = QUEUE_WORKER_COUNT
		stats["queue_name"] = FEED_UPDATE_QUEUE
	} else {
		stats["error"] = "Redis not available"
	}

	return stats
}

// QueueServiceInstance глобальный экземпляр сервиса очередей
var QueueServiceInstance *QueueService

// InitQueueService инициализирует сервис очередей
func InitQueueService() {
	QueueServiceInstance = NewQueueService()
}
