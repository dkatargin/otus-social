package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"social/db"
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
	// Обновляем кеш лент друзей
	qs.postService.updateFriendsFeeds(ctx, task.UserID, &task.Post)

	// Отправляем WebSocket уведомления через RabbitMQ
	qs.sendFeedNotifications(ctx, task.UserID, &task.Post)
}

// processDeletePost обрабатывает удаление поста
func (qs *QueueService) processDeletePost(ctx context.Context, task *FeedUpdateTask) {
	// Удаляем пост из лент всех друзей
	qs.removePostFromFriends(ctx, task.UserID, task.Post.ID)
}

// removePostFromFriends удаляет пост из лент друзей
func (qs *QueueService) removePostFromFriends(ctx context.Context, userID int64, postID int64) {
	// Получаем список друзей
	var friends []models.Friend
	err := db.GetReadOnlyDB(ctx).
		Where("(user_id = ? OR friend_id = ?) AND status = ?", userID, userID, "approved").
		Find(&friends).Error

	if err != nil {
		log.Printf("Error getting friends for user %d: %v", userID, err)
		return
	}

	// Удаляем пост из лент всех друзей
	for _, friend := range friends {
		var friendID int64
		if friend.UserID == userID {
			friendID = friend.FriendID
		} else {
			friendID = friend.UserID
		}

		qs.removePostFromUserFeed(ctx, friendID, postID)
	}

	// Удаляем из своей ленты тоже
	qs.removePostFromUserFeed(ctx, userID, postID)
}

// removePostFromUserFeed удаляет пост из ленты конкретного пользователя
func (qs *QueueService) removePostFromUserFeed(ctx context.Context, userID int64, postID int64) {
	feedKey := fmt.Sprintf("%s%d", FEED_KEY_PREFIX, userID)
	postKey := fmt.Sprintf("%s%d", POST_KEY_PREFIX, postID)

	pipe := RedisClient.Pipeline()

	// Удаляем из sorted set
	pipe.ZRem(ctx, feedKey, fmt.Sprintf("%d", postID))

	// Удаляем кешированные данные поста
	pipe.Del(ctx, postKey)

	pipe.Exec(ctx)
}

// EnqueueFeedUpdate добавляет задачу обновления ленты в очередь
func (qs *QueueService) EnqueueFeedUpdate(ctx context.Context, userID int64, post models.Post, action string) error {
	task := FeedUpdateTask{
		UserID: userID,
		Post:   post,
		Action: action,
	}

	taskData, err := json.Marshal(task)
	if err != nil {
		return fmt.Errorf("failed to marshal task: %w", err)
	}

	return RedisClient.RPush(ctx, FEED_UPDATE_QUEUE, taskData).Err()
}

// GetQueueStats возвращает статистику очереди
func (qs *QueueService) GetQueueStats(ctx context.Context) (int64, error) {
	return RedisClient.LLen(ctx, FEED_UPDATE_QUEUE).Result()
}

// sendFeedNotifications отправляет WebSocket уведомления друзьям через RabbitMQ
func (qs *QueueService) sendFeedNotifications(ctx context.Context, userID int64, post *models.Post) {
	// Получаем список друзей
	var friends []models.Friend
	err := db.GetReadOnlyDB(ctx).
		Where("(user_id = ? OR friend_id = ?) AND status = ?", userID, userID, "approved").
		Find(&friends).Error

	if err != nil {
		log.Printf("Error getting friends for user %d: %v", userID, err)
		return
	}

	// Проверяем, является ли пользователь celebrity
	if len(friends) >= CELEBRITY_THRESHOLD {
		log.Printf("User %d is celebrity (%d friends), using batch processing", userID, len(friends))
		qs.sendBatchNotifications(ctx, userID, post, friends)
		return
	}

	// Обычная обработка для пользователей с небольшим количеством друзей
	qs.sendDirectNotifications(ctx, userID, post, friends)
}

// sendDirectNotifications отправляет уведомления напрямую для обычных пользователей
func (qs *QueueService) sendDirectNotifications(ctx context.Context, userID int64, post *models.Post, friends []models.Friend) {
	for _, friend := range friends {
		var friendID int64
		if friend.UserID == userID {
			friendID = friend.FriendID
		} else {
			friendID = friend.UserID
		}

		// Создаем событие для WebSocket
		event := FeedEvent{
			UserID:    friendID, // Кому отправляем
			PostID:    post.ID,
			AuthorID:  userID, // Кто создал пост
			Content:   post.Content,
			CreatedAt: post.CreatedAt,
		}

		// Отправляем через RabbitMQ
		if err := PublishFeedEvent(ctx, event); err != nil {
			log.Printf("Failed to publish feed event for user %d: %v", friendID, err)
		}
	}
}

// sendBatchNotifications отправляет уведомления батчами для celebrity пользователей
func (qs *QueueService) sendBatchNotifications(ctx context.Context, userID int64, post *models.Post, friends []models.Friend) {
	for i := 0; i < len(friends); i += CELEBRITY_BATCH_SIZE {
		end := i + CELEBRITY_BATCH_SIZE
		if end > len(friends) {
			end = len(friends)
		}

		batch := friends[i:end]

		// Создаем отдельную горутину для каждого батча
		go func(batch []models.Friend) {
			qs.sendDirectNotifications(ctx, userID, post, batch)
		}(batch)

		// Небольшая задержка между батчами чтобы не перегружать систему
		if i+CELEBRITY_BATCH_SIZE < len(friends) {
			time.Sleep(10 * time.Millisecond)
		}
	}
}
