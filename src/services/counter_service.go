package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
)

// CounterType тип счетчика
type CounterType string

const (
	CounterTypeUnreadMessages CounterType = "unread_messages"
	CounterTypeUnreadDialogs  CounterType = "unread_dialogs"
	CounterTypeFriendRequests CounterType = "friend_requests"
	CounterTypeNotifications  CounterType = "notifications"
)

// Counter представляет счетчик для пользователя
type Counter struct {
	UserID    int64       `json:"user_id"`
	Type      CounterType `json:"type"`
	Count     int64       `json:"count"`
	UpdatedAt time.Time   `json:"updated_at"`
	Version   int64       `json:"version"` // Для оптимистичных блокировок
}

// CounterService сервис для управления счетчиками
type CounterService struct {
	redisClient *redis.Client
	ctx         context.Context
	mu          sync.RWMutex

	// Кэш для батчинга операций
	updateQueue chan *CounterUpdate
	batchSize   int
	flushTicker *time.Ticker
}

// CounterUpdate представляет обновление счетчика
type CounterUpdate struct {
	UserID int64
	Type   CounterType
	Delta  int64
	Done   chan error
}

// SagaCompensation представляет компенсирующую транзакцию
type SagaCompensation struct {
	UserID int64
	Type   CounterType
	Delta  int64
}

var (
	counterServiceInstance *CounterService
	counterServiceOnce     sync.Once
)

// GetCounterService возвращает singleton инстанс CounterService
func GetCounterService() *CounterService {
	counterServiceOnce.Do(func() {
		counterServiceInstance = NewCounterService(RedisClient)
	})
	return counterServiceInstance
}

// NewCounterService создает новый сервис счетчиков
func NewCounterService(redisClient *redis.Client) *CounterService {
	service := &CounterService{
		redisClient: redisClient,
		ctx:         context.Background(),
		updateQueue: make(chan *CounterUpdate, 1000),
		batchSize:   100,
		flushTicker: time.NewTicker(100 * time.Millisecond),
	}

	// Загружаем Lua скрипты для атомарных операций
	service.loadLuaScripts()

	// Запускаем фоновые процессы
	go service.processBatchUpdates()
	go service.runSyncWorker()

	log.Println("Counter service initialized")
	return service
}

// Lua скрипты для атомарных операций
var (
	incrementCounterScript = `
		local key = KEYS[1]
		local delta = tonumber(ARGV[1])
		local timestamp = tonumber(ARGV[2])
		local version = tonumber(ARGV[3])
		
		local counter = redis.call('GET', key)
		local current_count = 0
		local current_version = 0
		
		if counter then
			local data = cjson.decode(counter)
			current_count = tonumber(data.count) or 0
			current_version = tonumber(data.version) or 0
		end
		
		-- Оптимистичная блокировка
		if version > 0 and current_version ~= version then
			return {err = "version_mismatch"}
		end
		
		local new_count = math.max(0, current_count + delta)
		local new_version = current_version + 1
		
		local new_data = {
			count = new_count,
			updated_at = timestamp,
			version = new_version
		}
		
		redis.call('SET', key, cjson.encode(new_data))
		redis.call('EXPIRE', key, 86400) -- 24 часа TTL
		
		return {new_count, new_version}
	`

	getCounterScript = `
		local key = KEYS[1]
		local counter = redis.call('GET', key)
		
		if not counter then
			return {0, 0, 0}
		end
		
		local data = cjson.decode(counter)
		return {data.count or 0, data.updated_at or 0, data.version or 0}
	`

	batchIncrementScript = `
		local results = {}
		local timestamp = tonumber(ARGV[1])
		
		for i = 2, #ARGV, 2 do
			local key = ARGV[i]
			local delta = tonumber(ARGV[i + 1])
			
			local counter = redis.call('GET', key)
			local current_count = 0
			local current_version = 0
			
			if counter then
				local data = cjson.decode(counter)
				current_count = tonumber(data.count) or 0
				current_version = tonumber(data.version) or 0
			end
			
			local new_count = math.max(0, current_count + delta)
			local new_version = current_version + 1
			
			local new_data = {
				count = new_count,
				updated_at = timestamp,
				version = new_version
			}
			
			redis.call('SET', key, cjson.encode(new_data))
			redis.call('EXPIRE', key, 86400)
			
			table.insert(results, new_count)
		end
		
		return results
	`
)

var (
	incrementCounterSHA string
	getCounterSHA       string
	batchIncrementSHA   string
)

// loadLuaScripts загружает Lua скрипты в Redis
func (s *CounterService) loadLuaScripts() {
	var err error

	incrementCounterSHA, err = s.redisClient.ScriptLoad(s.ctx, incrementCounterScript).Result()
	if err != nil {
		log.Printf("Warning: Failed to load incrementCounter script: %v", err)
	}

	getCounterSHA, err = s.redisClient.ScriptLoad(s.ctx, getCounterScript).Result()
	if err != nil {
		log.Printf("Warning: Failed to load getCounter script: %v", err)
	}

	batchIncrementSHA, err = s.redisClient.ScriptLoad(s.ctx, batchIncrementScript).Result()
	if err != nil {
		log.Printf("Warning: Failed to load batchIncrement script: %v", err)
	}

	log.Println("Counter Lua scripts loaded")
}

// getCounterKey возвращает ключ Redis для счетчика
func (s *CounterService) getCounterKey(userID int64, counterType CounterType) string {
	return fmt.Sprintf("counter:%d:%s", userID, counterType)
}

// GetCounter получает значение счетчика (оптимизировано для чтения)
func (s *CounterService) GetCounter(userID int64, counterType CounterType) (int64, error) {
	key := s.getCounterKey(userID, counterType)

	// Используем Lua скрипт для атомарного чтения
	result, err := s.redisClient.EvalSha(s.ctx, getCounterSHA, []string{key}).Result()
	if err != nil {
		// Fallback на обычное чтение
		data, err := s.redisClient.Get(s.ctx, key).Result()
		if err == redis.Nil {
			return 0, nil
		}
		if err != nil {
			return 0, fmt.Errorf("failed to get counter: %w", err)
		}

		var counter struct {
			Count int64 `json:"count"`
		}
		if err := json.Unmarshal([]byte(data), &counter); err != nil {
			return 0, fmt.Errorf("failed to unmarshal counter: %w", err)
		}
		return counter.Count, nil
	}

	if values, ok := result.([]interface{}); ok && len(values) > 0 {
		if count, ok := values[0].(int64); ok {
			return count, nil
		}
	}

	return 0, nil
}

// GetAllCounters получает все счетчики для пользователя (батчевое чтение)
func (s *CounterService) GetAllCounters(userID int64) (map[CounterType]int64, error) {
	counters := make(map[CounterType]int64)

	counterTypes := []CounterType{
		CounterTypeUnreadMessages,
		CounterTypeUnreadDialogs,
		CounterTypeFriendRequests,
		CounterTypeNotifications,
	}

	// Используем pipeline для батчевого чтения
	pipe := s.redisClient.Pipeline()
	cmds := make(map[CounterType]*redis.StringCmd)

	for _, ct := range counterTypes {
		key := s.getCounterKey(userID, ct)
		cmds[ct] = pipe.Get(s.ctx, key)
	}

	_, err := pipe.Exec(s.ctx)
	if err != nil && err != redis.Nil {
		log.Printf("Pipeline exec error (non-critical): %v", err)
	}

	for ct, cmd := range cmds {
		data, err := cmd.Result()
		if err == redis.Nil {
			counters[ct] = 0
			continue
		}
		if err != nil {
			log.Printf("Error reading counter %s: %v", ct, err)
			counters[ct] = 0
			continue
		}

		var counter struct {
			Count int64 `json:"count"`
		}
		if err := json.Unmarshal([]byte(data), &counter); err != nil {
			log.Printf("Error unmarshaling counter %s: %v", ct, err)
			counters[ct] = 0
			continue
		}
		counters[ct] = counter.Count
	}

	return counters, nil
}

// IncrementCounter увеличивает счетчик (асинхронно через очередь)
func (s *CounterService) IncrementCounter(userID int64, counterType CounterType, delta int64) error {
	update := &CounterUpdate{
		UserID: userID,
		Type:   counterType,
		Delta:  delta,
		Done:   make(chan error, 1),
	}

	select {
	case s.updateQueue <- update:
		// Для некритичных операций можем не ждать
		return nil
	case <-time.After(100 * time.Millisecond):
		// Если очередь переполнена, делаем синхронно
		return s.incrementCounterSync(userID, counterType, delta, 0)
	}
}

// IncrementCounterSync синхронно увеличивает счетчик
func (s *CounterService) IncrementCounterSync(userID int64, counterType CounterType, delta int64) error {
	return s.incrementCounterSync(userID, counterType, delta, 0)
}

// incrementCounterSync внутренний метод для синхронного обновления
func (s *CounterService) incrementCounterSync(userID int64, counterType CounterType, delta int64, version int64) error {
	key := s.getCounterKey(userID, counterType)
	timestamp := time.Now().Unix()

	result, err := s.redisClient.EvalSha(
		s.ctx,
		incrementCounterSHA,
		[]string{key},
		delta,
		timestamp,
		version,
	).Result()

	if err != nil {
		return fmt.Errorf("failed to increment counter: %w", err)
	}

	if values, ok := result.([]interface{}); ok {
		if len(values) == 1 {
			if errMsg, ok := values[0].(map[string]interface{}); ok {
				if _, exists := errMsg["err"]; exists {
					return fmt.Errorf("version mismatch")
				}
			}
		}
	}

	return nil
}

// processBatchUpdates обрабатывает обновления счетчиков батчами
func (s *CounterService) processBatchUpdates() {
	batch := make([]*CounterUpdate, 0, s.batchSize)

	for {
		select {
		case update := <-s.updateQueue:
			batch = append(batch, update)
			if len(batch) >= s.batchSize {
				s.flushBatch(batch)
				batch = make([]*CounterUpdate, 0, s.batchSize)
			}

		case <-s.flushTicker.C:
			if len(batch) > 0 {
				s.flushBatch(batch)
				batch = make([]*CounterUpdate, 0, s.batchSize)
			}
		}
	}
}

// flushBatch сбрасывает батч обновлений в Redis
func (s *CounterService) flushBatch(batch []*CounterUpdate) {
	if len(batch) == 0 {
		return
	}

	timestamp := time.Now().Unix()
	args := []interface{}{timestamp}

	for _, update := range batch {
		key := s.getCounterKey(update.UserID, update.Type)
		args = append(args, key, update.Delta)
	}

	_, err := s.redisClient.EvalSha(s.ctx, batchIncrementSHA, []string{}, args...).Result()
	if err != nil {
		log.Printf("Error flushing batch: %v", err)
		// Fallback на синхронные обновления
		for _, update := range batch {
			if err := s.incrementCounterSync(update.UserID, update.Type, update.Delta, 0); err != nil {
				log.Printf("Error in fallback increment: %v", err)
			}
		}
	}
}

// ResetCounter сбрасывает счетчик
func (s *CounterService) ResetCounter(userID int64, counterType CounterType) error {
	key := s.getCounterKey(userID, counterType)

	data := map[string]interface{}{
		"count":      0,
		"updated_at": time.Now().Unix(),
		"version":    1,
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal counter: %w", err)
	}

	return s.redisClient.Set(s.ctx, key, jsonData, 24*time.Hour).Err()
}

// runSyncWorker запускает воркер для периодической синхронизации счетчиков
func (s *CounterService) runSyncWorker() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		// Здесь будет логика сверки счетчиков с реальными данными
		// Это будет реализовано в saga_service.go
		log.Println("Counter sync worker tick")
	}
}

// CreateCompensation создает компенсирующую транзакцию
func (s *CounterService) CreateCompensation(userID int64, counterType CounterType, delta int64) *SagaCompensation {
	return &SagaCompensation{
		UserID: userID,
		Type:   counterType,
		Delta:  -delta, // Инвертируем дельту для компенсации
	}
}

// ExecuteCompensation выполняет компенсирующую транзакцию
func (s *CounterService) ExecuteCompensation(comp *SagaCompensation) error {
	return s.IncrementCounterSync(comp.UserID, comp.Type, comp.Delta)
}

// SetCounterValue устанавливает точное значение счетчика (для синхронизации)
func (s *CounterService) SetCounterValue(userID int64, counterType CounterType, value int64) error {
	key := s.getCounterKey(userID, counterType)

	data := map[string]interface{}{
		"count":      value,
		"updated_at": time.Now().Unix(),
		"version":    time.Now().UnixNano(), // Используем timestamp как версию
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal counter: %w", err)
	}

	return s.redisClient.Set(s.ctx, key, jsonData, 24*time.Hour).Err()
}

// GetCounterWithVersion получает счетчик с версией (для оптимистичных блокировок)
func (s *CounterService) GetCounterWithVersion(userID int64, counterType CounterType) (int64, int64, error) {
	key := s.getCounterKey(userID, counterType)

	result, err := s.redisClient.EvalSha(s.ctx, getCounterSHA, []string{key}).Result()
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get counter: %w", err)
	}

	if values, ok := result.([]interface{}); ok && len(values) >= 3 {
		count, _ := strconv.ParseInt(fmt.Sprint(values[0]), 10, 64)
		version, _ := strconv.ParseInt(fmt.Sprint(values[2]), 10, 64)
		return count, version, nil
	}

	return 0, 0, nil
}
