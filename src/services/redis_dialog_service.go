package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/go-redis/redis/v8"
)

// FlexInt64 кастомный тип для работы с int64 который может приходить как строка или число
type FlexInt64 int64

func (fi *FlexInt64) UnmarshalJSON(data []byte) error {
	var num int64
	if err := json.Unmarshal(data, &num); err != nil {
		// Пробуем как строку, если не число
		var numStr string
		if err2 := json.Unmarshal(data, &numStr); err2 != nil {
			return err // возвращаем первую ошибку
		}
		var err3 error
		num, err3 = strconv.ParseInt(numStr, 10, 64)
		if err3 != nil {
			return err3
		}
	}
	*fi = FlexInt64(num)
	return nil
}

func (fi FlexInt64) MarshalJSON() ([]byte, error) {
	return json.Marshal(int64(fi))
}

func (fi FlexInt64) Int64() int64 {
	return int64(fi)
}

// UnixTime кастомный тип для работы с Unix timestamp в JSON
type UnixTime time.Time

func (ut *UnixTime) UnmarshalJSON(data []byte) error {
	var timestamp int64
	if err := json.Unmarshal(data, &timestamp); err != nil {
		// Пробуем как строку, если не число
		var timestampStr string
		if err2 := json.Unmarshal(data, &timestampStr); err2 != nil {
			return err // возвращаем первую ошибку
		}
		var err3 error
		timestamp, err3 = strconv.ParseInt(timestampStr, 10, 64)
		if err3 != nil {
			return err3
		}
	}
	*ut = UnixTime(time.Unix(timestamp, 0))
	return nil
}

func (ut UnixTime) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Time(ut).Unix())
}

func (ut UnixTime) Time() time.Time {
	return time.Time(ut)
}

// RedisDialogService предоставляет функциональность диалогов через Redis с UDF
type RedisDialogService struct {
	client *redis.Client
	ctx    context.Context
}

// Message структура сообщения для Redis
type RedisMessage struct {
	ID         string    `json:"id"`
	FromUserID FlexInt64 `json:"from_id"`
	ToUserID   FlexInt64 `json:"to_id"`
	Text       string    `json:"text"`
	CreatedAt  UnixTime  `json:"created_at"`
	IsRead     bool      `json:"is_read"`
}

// DialogStats статистика диалога
type DialogStats struct {
	TotalMessages int64 `json:"total_messages"`
	UnreadCount   int64 `json:"unread_count"`
	LastActivity  int64 `json:"last_activity"`
}

// NewRedisDialogService создает новый экземпляр Redis-��ервиса для диалогов
func NewRedisDialogService(addr, password string, db int) *RedisDialogService {
	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	ctx := context.Background()

	// Проверяем соединение
	_, err := rdb.Ping(ctx).Result()
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}

	service := &RedisDialogService{
		client: rdb,
		ctx:    ctx,
	}

	// Загружаем UDF скрипты
	service.loadLuaScripts()

	return service
}

// Lua скрипты для UDF
var (
	sendMessageScript = `
		local dialog_key = KEYS[1]
		local unread_key = KEYS[2]
		local stats_key = KEYS[3]
		local message_id = ARGV[1]
		local from_user_id = tonumber(ARGV[2])
		local to_user_id = tonumber(ARGV[3])
		local text = ARGV[4]
		local created_at = tonumber(ARGV[5])
		
		local message = {
			id = message_id,
			from_id = from_user_id,
			to_id = to_user_id,
			text = text,
			created_at = created_at,
			is_read = false
		}
		
		local message_json = cjson.encode(message)
		
		-- Добавляем сообщение в sorted set диалога (score = timestamp)
		redis.call('ZADD', dialog_key, created_at, message_json)
		
		-- Увеличиваем счетчик непрочитанных для получателя
		redis.call('HINCRBY', unread_key, to_user_id, 1)
		
		-- Обновляем статистику диалога
		redis.call('HSET', stats_key, 
			'total_messages', redis.call('ZCARD', dialog_key),
			'last_activity', created_at
		)
		
		-- Устанавливаем TTL для диалога (30 дней)
		redis.call('EXPIRE', dialog_key, 2592000)
		redis.call('EXPIRE', unread_key, 2592000)
		redis.call('EXPIRE', stats_key, 2592000)
		
		return message_id
	`

	getMessagesScript = `
		local dialog_key = KEYS[1]
		local offset = tonumber(ARGV[1])
		local limit = tonumber(ARGV[2])
		
		-- Получаем сообщения с пагинацией (от новых к старым)
		local messages = redis.call('ZREVRANGE', dialog_key, offset, offset + limit - 1)
		
		return messages
	`

	markAsReadScript = `
		local dialog_key = KEYS[1]
		local unread_key = KEYS[2]
		local user_id = tonumber(ARGV[1])
		local timestamp = tonumber(ARGV[2])
		
		-- Получаем все сообщения пользователя
		local messages = redis.call('ZREVRANGE', dialog_key, 0, -1)
		local updated_count = 0
		
		for i, message_json in ipairs(messages) do
			local message = cjson.decode(message_json)
			if message.to_id == user_id and not message.is_read then
				message.is_read = true
				local updated_json = cjson.encode(message)
				-- Обновляем сообщение в sorted set
				redis.call('ZREM', dialog_key, message_json)
				redis.call('ZADD', dialog_key, message.created_at, updated_json)
				updated_count = updated_count + 1
			end
		end
		
		-- Сбрасываем счетчик непрочитанных
		redis.call('HSET', unread_key, user_id, 0)
		
		return updated_count
	`

	getStatsScript = `
		local stats_key = KEYS[1]
		local unread_key = KEYS[2]
		local user_id = tonumber(ARGV[1])
		
		local total_messages = redis.call('HGET', stats_key, 'total_messages') or 0
		local last_activity = redis.call('HGET', stats_key, 'last_activity') or 0
		local unread_count = redis.call('HGET', unread_key, user_id) or 0
		
		return {total_messages, unread_count, last_activity}
	`
)

// SHA хеши для Lua ск��иптов (будут заполнены при загрузке)
var (
	sendMessageSHA string
	getMessagesSHA string
	markAsReadSHA  string
	getStatsSHA    string
)

// loadLuaScripts загружает Lua скрипты в Redis
func (s *RedisDialogService) loadLuaScripts() {
	var err error

	sendMessageSHA, err = s.client.ScriptLoad(s.ctx, sendMessageScript).Result()
	if err != nil {
		log.Fatalf("Failed to load sendMessage script: %v", err)
	}

	getMessagesSHA, err = s.client.ScriptLoad(s.ctx, getMessagesScript).Result()
	if err != nil {
		log.Fatalf("Failed to load getMessages script: %v", err)
	}

	markAsReadSHA, err = s.client.ScriptLoad(s.ctx, markAsReadScript).Result()
	if err != nil {
		log.Fatalf("Failed to load markAsRead script: %v", err)
	}

	getStatsSHA, err = s.client.ScriptLoad(s.ctx, getStatsScript).Result()
	if err != nil {
		log.Fatalf("Failed to load getStats script: %v", err)
	}

	log.Println("Lua UDF scripts loaded successfully")
}

// getDialogKey возвращает ключ для диалога между двумя по��ьзователями
func (s *RedisDialogService) getDialogKey(user1, user2 int64) string {
	// Обеспечиваем детерминированный порядок
	if user1 > user2 {
		user1, user2 = user2, user1
	}
	return fmt.Sprintf("dialog:%d:%d", user1, user2)
}

// getUnreadKey возвращает ключ для счетчиков непрочитанных сообщений
func (s *RedisDialogService) getUnreadKey(user1, user2 int64) string {
	if user1 > user2 {
		user1, user2 = user2, user1
	}
	return fmt.Sprintf("unread:%d:%d", user1, user2)
}

// getStatsKey возвращает ключ для статистики диалога
func (s *RedisDialogService) getStatsKey(user1, user2 int64) string {
	if user1 > user2 {
		user1, user2 = user2, user1
	}
	return fmt.Sprintf("stats:%d:%d", user1, user2)
}

// SendMessage отправляет сообщение с использованием UDF
func (s *RedisDialogService) SendMessage(fromUserID, toUserID int64, text string) (*RedisMessage, error) {
	dialogKey := s.getDialogKey(fromUserID, toUserID)
	unreadKey := s.getUnreadKey(fromUserID, toUserID)
	statsKey := s.getStatsKey(fromUserID, toUserID)

	now := time.Now()
	messageID := fmt.Sprintf("%d_%d", now.UnixNano(), fromUserID)

	result, err := s.client.EvalSha(s.ctx, sendMessageSHA, []string{dialogKey, unreadKey, statsKey},
		messageID, fromUserID, toUserID, text, now.Unix()).Result()

	if err != nil {
		return nil, fmt.Errorf("failed to send message: %v", err)
	}

	return &RedisMessage{
		ID:         result.(string),
		FromUserID: FlexInt64(fromUserID),
		ToUserID:   FlexInt64(toUserID),
		Text:       text,
		CreatedAt:  UnixTime(now),
		IsRead:     false,
	}, nil
}

// GetMessages получает сообщения диалога с пагинацией
func (s *RedisDialogService) GetMessages(user1, user2 int64, offset, limit int) ([]*RedisMessage, error) {
	dialogKey := s.getDialogKey(user1, user2)

	result, err := s.client.EvalSha(s.ctx, getMessagesSHA, []string{dialogKey}, offset, limit).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get messages: %v", err)
	}

	messagesData := result.([]interface{})
	messages := make([]*RedisMessage, 0, len(messagesData))

	for _, msgData := range messagesData {
		var msg RedisMessage
		if err := json.Unmarshal([]byte(msgData.(string)), &msg); err != nil {
			log.Printf("Failed to unmarshal message: %v", err)
			continue
		}
		messages = append(messages, &msg)
	}

	return messages, nil
}

// MarkAsRead отмечает сообщения как прочитанные
func (s *RedisDialogService) MarkAsRead(user1, user2, readerUserID int64) (int, error) {
	dialogKey := s.getDialogKey(user1, user2)
	unreadKey := s.getUnreadKey(user1, user2)

	result, err := s.client.EvalSha(s.ctx, markAsReadSHA, []string{dialogKey, unreadKey},
		readerUserID, time.Now().Unix()).Result()

	if err != nil {
		return 0, fmt.Errorf("failed to mark as read: %v", err)
	}

	count, _ := result.(int64)
	return int(count), nil
}

// GetDialogStats получает статистику диалога
func (s *RedisDialogService) GetDialogStats(user1, user2, forUserID int64) (*DialogStats, error) {
	statsKey := s.getStatsKey(user1, user2)
	unreadKey := s.getUnreadKey(user1, user2)

	result, err := s.client.EvalSha(s.ctx, getStatsSHA, []string{statsKey, unreadKey}, forUserID).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get stats: %v", err)
	}

	data := result.([]interface{})

	totalMessages, _ := strconv.ParseInt(data[0].(string), 10, 64)
	unreadCount, _ := strconv.ParseInt(data[1].(string), 10, 64)
	lastActivity, _ := strconv.ParseInt(data[2].(string), 10, 64)

	return &DialogStats{
		TotalMessages: totalMessages,
		UnreadCount:   unreadCount,
		LastActivity:  lastActivity,
	}, nil
}

// Close закрывает соединение с Redis
func (s *RedisDialogService) Close() error {
	return s.client.Close()
}

// GetClient возвращает Redis клиент для прямого доступа
func (s *RedisDialogService) GetClient() *redis.Client {
	return s.client
}
