package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"social/db"
	"social/models"
	"strconv"
	"time"

	"github.com/go-redis/redis/v8"
)

const (
	FEED_CACHE_TTL  = 24 * time.Hour // TTL для кеша ленты
	MAX_FEED_SIZE   = 1000           // Максимальное количество постов в ленте
	FEED_KEY_PREFIX = "user_feed:"   // Префикс для ключей ленты в Redis
	POST_KEY_PREFIX = "post:"        // Префикс для кеша постов
)

type PostService struct{}

func NewPostService() *PostService {
	return &PostService{}
}

// CreatePost создает новый пост и обновляет ленты друзей
func (ps *PostService) CreatePost(ctx context.Context, userID int64, content string) (*models.Post, error) {
	log.Printf("DEBUG: CreatePost called for userID=%d, content=%s", userID, content)

	post := &models.Post{
		UserID:    userID,
		Content:   content,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Сохраняем пост в БД
	err := db.GetWriteDB(ctx).Create(post).Error
	if err != nil {
		log.Printf("ERROR: Failed to create post in DB: %v", err)
		return nil, fmt.Errorf("failed to create post: %w", err)
	}

	log.Printf("DEBUG: Post created in DB with ID=%d", post.ID)

	// Добавляем задачу обновления лент в очередь
	if QueueServiceInstance != nil && RedisClient != nil {
		log.Printf("DEBUG: Using QueueService path")
		go QueueServiceInstance.EnqueueFeedUpdate(context.Background(), userID, *post, "create")
	} else {
		log.Printf("DEBUG: Using fallback path - QueueServiceInstance=%v, RedisClient=%v", QueueServiceInstance != nil, RedisClient != nil)
		// Fallback - обновляем ленты синхронно, если очередь не инициализирована
		go ps.updateFriendsFeeds(context.Background(), userID, post)
	}

	return post, nil
}

// GetUserFeed получает ленту пользователя с пагинацией
func (ps *PostService) GetUserFeed(ctx context.Context, userID int64, lastID int64, limit int) (*models.FeedResponse, error) {
	if limit <= 0 || limit > 100 {
		limit = 20 // Дефолтный лимит
	}

	feedKey := fmt.Sprintf("%s%d", FEED_KEY_PREFIX, userID)

	// Пытаемся получить из кеша
	feedPosts, err := ps.getFeedFromCache(ctx, feedKey, lastID, limit)
	if err == nil && len(feedPosts) > 0 {
		return &models.FeedResponse{
			Posts:   feedPosts,
			HasMore: len(feedPosts) == limit,
			LastID:  getLastID(feedPosts),
		}, nil
	}

	// Если в кеше нет или ошибка, строим ленту из БД
	feedPosts, err = ps.buildFeedFromDB(ctx, userID, lastID, limit)
	if err != nil {
		return nil, err
	}

	// Кешируем результат
	go ps.cacheFeed(context.Background(), feedKey, feedPosts)

	return &models.FeedResponse{
		Posts:   feedPosts,
		HasMore: len(feedPosts) == limit,
		LastID:  getLastID(feedPosts),
	}, nil
}

// buildFeedFromDB строит ленту из базы данных
func (ps *PostService) buildFeedFromDB(ctx context.Context, userID int64, lastID int64, limit int) ([]models.FeedPost, error) {
	var friendIDs []int64

	// Получаем список друзей
	err := db.GetReadOnlyDB(ctx).
		Model(&models.Friend{}).
		Where("(user_id = ? OR friend_id = ?) AND status = ?", userID, userID, "approved").
		Select("CASE WHEN user_id = ? THEN friend_id ELSE user_id END", userID).
		Scan(&friendIDs).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get friends: %w", err)
	}

	if len(friendIDs) == 0 {
		return []models.FeedPost{}, nil
	}

	// Добавляем самого пользователя в список
	friendIDs = append(friendIDs, userID)

	// Строим запрос для получения постов
	query := db.GetReadOnlyDB(ctx).
		Table("posts p").
		Select("p.id, p.user_id, u.first_name || ' ' || u.last_name as user_name, p.content, p.created_at").
		Joins("JOIN \"users\" u ON p.user_id = u.id").
		Where("p.user_id IN ?", friendIDs).
		Order("p.created_at DESC, p.id DESC").
		Limit(limit)

	if lastID > 0 {
		query = query.Where("p.id < ?", lastID)
	}

	var feedPosts []models.FeedPost
	err = query.Scan(&feedPosts).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get feed posts: %w", err)
	}

	return feedPosts, nil
}

// getFeedFromCache получает ленту из Redis кеша
func (ps *PostService) getFeedFromCache(ctx context.Context, feedKey string, lastID int64, limit int) ([]models.FeedPost, error) {
	if RedisClient == nil {
		return nil, fmt.Errorf("redis not available")
	}

	// Используем Redis Sorted Set для хранения ленты (score = timestamp)
	var start, stop int64 = 0, int64(limit - 1)

	if lastID > 0 {
		// Находим позицию lastID в отсортированном множестве
		rank := RedisClient.ZRevRank(ctx, feedKey, strconv.FormatInt(lastID, 10)).Val()
		start = rank + 1
		stop = start + int64(limit) - 1
	}

	postIDs, err := RedisClient.ZRevRange(ctx, feedKey, start, stop).Result()
	if err != nil {
		return nil, err
	}

	if len(postIDs) == 0 {
		return []models.FeedPost{}, nil
	}

	// Получаем данные постов из кеша
	var feedPosts []models.FeedPost
	pipe := RedisClient.Pipeline()

	postKeys := make([]string, len(postIDs))
	for i, postID := range postIDs {
		postKeys[i] = fmt.Sprintf("%s%s", POST_KEY_PREFIX, postID)
	}

	cmds := make([]*redis.StringCmd, len(postKeys))
	for i, key := range postKeys {
		cmds[i] = pipe.Get(ctx, key)
	}

	_, err = pipe.Exec(ctx)
	if err != nil && err != redis.Nil {
		return nil, err
	}

	for _, cmd := range cmds {
		val, err := cmd.Result()
		if err == redis.Nil {
			continue
		}
		if err != nil {
			continue
		}

		var feedPost models.FeedPost
		if err := json.Unmarshal([]byte(val), &feedPost); err == nil {
			feedPosts = append(feedPosts, feedPost)
		}
	}

	return feedPosts, nil
}

// cacheFeed кеширует ленту в Redis
func (ps *PostService) cacheFeed(ctx context.Context, feedKey string, posts []models.FeedPost) {
	if len(posts) == 0 || RedisClient == nil {
		return
	}

	pipe := RedisClient.Pipeline()

	// Очищаем старую ленту
	pipe.Del(ctx, feedKey)

	// Добавляем посты в sorted set (score = unix timestamp)
	for _, post := range posts {
		score := float64(post.CreatedAt.Unix())
		pipe.ZAdd(ctx, feedKey, &redis.Z{
			Score:  score,
			Member: strconv.FormatInt(post.ID, 10),
		})

		// Кешируем сам пост
		postKey := fmt.Sprintf("%s%d", POST_KEY_PREFIX, post.ID)
		postData, _ := json.Marshal(post)
		pipe.Set(ctx, postKey, postData, FEED_CACHE_TTL)
	}

	// Ограничиваем размер ленты
	pipe.ZRemRangeByRank(ctx, feedKey, 0, -MAX_FEED_SIZE-1)

	// Устанавливаем TTL для ленты
	pipe.Expire(ctx, feedKey, FEED_CACHE_TTL)

	pipe.Exec(ctx)
}

// updateFriendsFeeds обновляет ленты друзей при создании нового поста
func (ps *PostService) updateFriendsFeeds(ctx context.Context, userID int64, post *models.Post) {
	log.Printf("DEBUG: updateFriendsFeeds called for userID=%d, postID=%d", userID, post.ID)

	// Получаем список друзей
	var friends []models.Friend
	err := db.GetReadOnlyDB(ctx).
		Where("(user_id = ? OR friend_id = ?) AND status = ?", userID, userID, "approved").
		Find(&friends).Error

	if err != nil {
		log.Printf("ERROR: Failed to get friends for userID=%d: %v", userID, err)
		return
	}

	log.Printf("DEBUG: Found %d friends for userID=%d", len(friends), userID)

	// Создаем FeedPost для кеширования
	var user models.User
	if err := db.GetReadOnlyDB(ctx).First(&user, userID).Error; err != nil {
		log.Printf("ERROR: Failed to get user data for userID=%d: %v", userID, err)
		return
	}

	feedPost := models.FeedPost{
		ID:        post.ID,
		UserID:    post.UserID,
		UserName:  user.FirstName + " " + user.LastName,
		Content:   post.Content,
		CreatedAt: post.CreatedAt,
	}

	// Обновляем ленты всех друзей
	for _, friend := range friends {
		var friendID int64
		if friend.UserID == userID {
			friendID = friend.FriendID
		} else {
			friendID = friend.UserID
		}

		log.Printf("DEBUG: Processing friend userID=%d", friendID)
		ps.addPostToUserFeed(ctx, friendID, feedPost)

		// Публикуем событие в RabbitMQ для push feed
		err := PublishFeedEvent(ctx, FeedEvent{
			UserID:    friendID,
			PostID:    post.ID,
			AuthorID:  post.UserID,
			Content:   post.Content,
			CreatedAt: post.CreatedAt,
		})

		// Fallback: если RabbitMQ недоступен, отправляем напрямую через WebSocket
		if err != nil {
			log.Printf("DEBUG: RabbitMQ error, using fallback for friendID=%d: %v", friendID, err)
			ps.sendDirectWSEvent(friendID, post.ID, post.UserID, post.Content, post.CreatedAt)
		} else {
			log.Printf("DEBUG: RabbitMQ event published successfully for friendID=%d", friendID)
		}
	}

	// Добавляем в свою ленту тоже
	ps.addPostToUserFeed(ctx, userID, feedPost)
	// Публикуем событие для самого автора
	err = PublishFeedEvent(ctx, FeedEvent{
		UserID:    userID,
		PostID:    post.ID,
		AuthorID:  post.UserID,
		Content:   post.Content,
		CreatedAt: post.CreatedAt,
	})

	// Fallback для автора
	if err != nil {
		log.Printf("DEBUG: RabbitMQ error for author, using fallback for userID=%d: %v", userID, err)
		ps.sendDirectWSEvent(userID, post.ID, post.UserID, post.Content, post.CreatedAt)
	} else {
		log.Printf("DEBUG: RabbitMQ event published successfully for author userID=%d", userID)
	}
}

// sendDirectWSEvent отправляет WebSocket событие напрямую (fallback для RabbitMQ)
func (ps *PostService) sendDirectWSEvent(userID, postID, authorID int64, content string, createdAt time.Time) {
	log.Printf("DEBUG: sendDirectWSEvent called for userID=%d, postID=%d, authorID=%d", userID, postID, authorID)

	pushMsg := struct {
		Event     string    `json:"event"`
		UserID    int64     `json:"user_id"`
		PostID    int64     `json:"post_id"`
		AuthorID  int64     `json:"author_id"`
		Content   string    `json:"content"`
		CreatedAt time.Time `json:"created_at"`
	}{
		Event:     "feed_posted",
		UserID:    userID,
		PostID:    postID,
		AuthorID:  authorID,
		Content:   content,
		CreatedAt: createdAt,
	}

	if pushData, err := json.Marshal(pushMsg); err == nil {
		log.Printf("DEBUG: Sending WebSocket message to userID=%d: %s", userID, string(pushData))
		GlobalWSConnManager.Send(userID, pushData)
		log.Printf("DEBUG: WebSocket message sent successfully")
	} else {
		log.Printf("ERROR: Failed to marshal push message: %v", err)
	}
}

// addPostToUserFeed добавляет пост в ленту конкретного пользователя
func (ps *PostService) addPostToUserFeed(ctx context.Context, userID int64, post models.FeedPost) {
	if RedisClient == nil {
		return
	}

	feedKey := fmt.Sprintf("%s%d", FEED_KEY_PREFIX, userID)
	postKey := fmt.Sprintf("%s%d", POST_KEY_PREFIX, post.ID)

	pipe := RedisClient.Pipeline()

	// Добавляем в sorted set
	score := float64(post.CreatedAt.Unix())
	pipe.ZAdd(ctx, feedKey, &redis.Z{
		Score:  score,
		Member: strconv.FormatInt(post.ID, 10),
	})

	// Кешируем данные поста
	postData, err := json.Marshal(post)
	if err != nil {
		fmt.Println("failed to marshal post for caching:", err)
		return
	}
	pipe.Set(ctx, postKey, postData, FEED_CACHE_TTL)

	// Ограничиваем размер ленты
	pipe.ZRemRangeByRank(ctx, feedKey, 0, -MAX_FEED_SIZE-1)

	// Обновляем TTL
	pipe.Expire(ctx, feedKey, FEED_CACHE_TTL)

	pipe.Exec(ctx)
}

// InvalidateUserFeed инвалидирует кеш ленты пользователя
func (ps *PostService) InvalidateUserFeed(ctx context.Context, userID int64) error {
	if RedisClient == nil {
		return fmt.Errorf("redis not available")
	}
	feedKey := fmt.Sprintf("%s%d", FEED_KEY_PREFIX, userID)
	return RedisClient.Del(ctx, feedKey).Err()
}

// RebuildUserFeedFromDB перестраивает кеш ленты пользователя из БД
func (ps *PostService) RebuildUserFeedFromDB(ctx context.Context, userID int64) error {
	if RedisClient == nil {
		return fmt.Errorf("redis not available")
	}

	feedKey := fmt.Sprintf("%s%d", FEED_KEY_PREFIX, userID)

	// Удаляем старый кеш
	RedisClient.Del(ctx, feedKey)

	// Строим новую ленту из БД
	feedPosts, err := ps.buildFeedFromDB(ctx, userID, 0, MAX_FEED_SIZE)
	if err != nil {
		return err
	}

	// Кешируем новую ленту
	ps.cacheFeed(ctx, feedKey, feedPosts)

	return nil
}

// RebuildAllFeeds перестраивает кеши всех лент из БД
func (ps *PostService) RebuildAllFeeds(ctx context.Context) error {
	// Получаем всех пользователей
	var userIDs []int64
	err := db.GetReadOnlyDB(ctx).Model(&models.User{}).Pluck("id", &userIDs).Error
	if err != nil {
		return err
	}

	// Перестраиваем ленты для всех пользователей
	for _, userID := range userIDs {
		if err := ps.RebuildUserFeedFromDB(ctx, userID); err != nil {
			// Логируем ошибку, но продолжаем
			continue
		}
	}

	return nil
}

// DeletePost удаляет пост и убирает его из лент друзей
func (ps *PostService) DeletePost(ctx context.Context, userID int64, postID int64) error {
	// Проверяем, что пост принадлежит пользователю
	var post models.Post
	err := db.GetWriteDB(ctx).Where("id = ? AND user_id = ?", postID, userID).First(&post).Error
	if err != nil {
		return fmt.Errorf("post not found or access denied: %w", err)
	}

	// Удаляем пост из БД
	err = db.GetWriteDB(ctx).Delete(&post).Error
	if err != nil {
		return fmt.Errorf("failed to delete post: %w", err)
	}

	// Добавляем задачу удаления поста из лент в очередь
	if QueueServiceInstance != nil && RedisClient != nil {
		go QueueServiceInstance.EnqueueFeedUpdate(context.Background(), userID, post, "delete")
	} else {
		// Fallback - удаляем из лент синхронно, если очередь не работает
		ps.removePostFromAllFeeds(context.Background(), userID, postID)
	}

	return nil
}

func getLastID(posts []models.FeedPost) int64 {
	if len(posts) == 0 {
		return 0
	}
	return posts[len(posts)-1].ID
}

// removePostFromAllFeeds удаляет пост из лент всех друзей (fallback метод)
func (ps *PostService) removePostFromAllFeeds(ctx context.Context, userID int64, postID int64) {
	if RedisClient == nil {
		return
	}

	// Получаем список друзей
	var friends []models.Friend
	err := db.GetReadOnlyDB(ctx).
		Where("(user_id = ? OR friend_id = ?) AND status = ?", userID, userID, "approved").
		Find(&friends).Error

	if err != nil {
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

		ps.removePostFromUserFeed(ctx, friendID, postID)
	}

	// Удаляем из своей ленты тоже
	ps.removePostFromUserFeed(ctx, userID, postID)
}

// removePostFromUserFeed удаляет пост из ленты конкретного пользователя
func (ps *PostService) removePostFromUserFeed(ctx context.Context, userID int64, postID int64) {
	if RedisClient == nil {
		return
	}

	feedKey := fmt.Sprintf("%s%d", FEED_KEY_PREFIX, userID)
	postKey := fmt.Sprintf("%s%d", POST_KEY_PREFIX, postID)

	pipe := RedisClient.Pipeline()

	// Удаляем из sorted set
	pipe.ZRem(ctx, feedKey, fmt.Sprintf("%d", postID))

	// Удаляем кешированные данные поста
	pipe.Del(ctx, postKey)

	pipe.Exec(ctx)
}
