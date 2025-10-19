package services

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"social/db"
	"social/models"
)

// SagaStep представляет шаг в SAGA
type SagaStep struct {
	Name         string
	Execute      func(ctx context.Context) error
	Compensate   func(ctx context.Context) error
	Executed     bool
	CompensateOn []string // На каких ошибках компенсиров��ть
}

// Saga представляет SAGA транзакцию
type Saga struct {
	ID            string
	Steps         []*SagaStep
	Compensations []*SagaCompensation
	ctx           context.Context
	mu            sync.Mutex
}

// CounterSagaService сервис для SAGA операций со счетчиками
type CounterSagaService struct {
	counterService     *CounterService
	redisDialogService *RedisDialogService
	ctx                context.Context
	mu                 sync.RWMutex

	// Активные SAGA транзакции
	activeSagas map[string]*Saga
}

var (
	sagaServiceInstance *CounterSagaService
	sagaServiceOnce     sync.Once
)

// GetCounterSagaService возвращает singleton инстанс CounterSagaService
func GetCounterSagaService() *CounterSagaService {
	sagaServiceOnce.Do(func() {
		sagaServiceInstance = NewCounterSagaService(GetCounterService())
	})
	return sagaServiceInstance
}

// NewCounterSagaService создает новый SAGA сервис
func NewCounterSagaService(counterService *CounterService) *CounterSagaService {
	service := &CounterSagaService{
		counterService: counterService,
		ctx:            context.Background(),
		activeSagas:    make(map[string]*Saga),
	}

	// Запускаем фоновые задачи
	go service.runReconciliationWorker()
	go service.runConsistencyChecker()

	log.Println("Counter SAGA service initialized")
	return service
}

// NewSaga создает новую SAGA транзакцию
func (s *CounterSagaService) NewSaga(id string) *Saga {
	saga := &Saga{
		ID:            id,
		Steps:         make([]*SagaStep, 0),
		Compensations: make([]*SagaCompensation, 0),
		ctx:           s.ctx,
	}

	s.mu.Lock()
	s.activeSagas[id] = saga
	s.mu.Unlock()

	return saga
}

// AddStep добавляет шаг в SAGA
func (saga *Saga) AddStep(name string, execute func(ctx context.Context) error, compensate func(ctx context.Context) error) *Saga {
	saga.Steps = append(saga.Steps, &SagaStep{
		Name:       name,
		Execute:    execute,
		Compensate: compensate,
		Executed:   false,
	})
	return saga
}

// Execute выполн��ет SAGA транзакцию
func (saga *Saga) Execute() error {
	saga.mu.Lock()
	defer saga.mu.Unlock()

	executedSteps := make([]*SagaStep, 0)

	// Выполняем шаги последовательно
	for _, step := range saga.Steps {
		if err := step.Execute(saga.ctx); err != nil {
			log.Printf("SAGA %s: Step %s failed: %v", saga.ID, step.Name, err)

			// Выполняем компенсирующие транзакции в обратном порядке
			for i := len(executedSteps) - 1; i >= 0; i-- {
				compensateStep := executedSteps[i]
				if compensateStep.Compensate != nil {
					if compErr := compensateStep.Compensate(saga.ctx); compErr != nil {
						log.Printf("SAGA %s: Compensation for %s failed: %v", saga.ID, compensateStep.Name, compErr)
					} else {
						log.Printf("SAGA %s: Compensation for %s completed", saga.ID, compensateStep.Name)
					}
				}
			}

			return fmt.Errorf("saga failed at step %s: %w", step.Name, err)
		}

		step.Executed = true
		executedSteps = append(executedSteps, step)
	}

	log.Printf("SAGA %s completed successfully", saga.ID)
	return nil
}

// HandleNewMessage обрабатывает новое сообщение с использованием SAGA
func (s *CounterSagaService) HandleNewMessage(fromUserID, toUserID int64, text string) error {
	sagaID := fmt.Sprintf("new_message_%d_%d_%d", fromUserID, toUserID, time.Now().UnixNano())
	saga := s.NewSaga(sagaID)

	var messageID int64
	var counterCompensation *SagaCompensation

	// Шаг 1: Увеличиваем счетчик непрочитанных сообщений
	saga.AddStep(
		"increment_unread_counter",
		func(ctx context.Context) error {
			err := s.counterService.IncrementCounterSync(toUserID, CounterTypeUnreadMessages, 1)
			if err == nil {
				counterCompensation = s.counterService.CreateCompensation(toUserID, CounterTypeUnreadMessages, 1)
			}
			return err
		},
		func(ctx context.Context) error {
			if counterCompensation != nil {
				return s.counterService.ExecuteCompensation(counterCompensation)
			}
			return nil
		},
	)

	// Шаг 2: Сохраняем сообщение в базу данных
	saga.AddStep(
		"save_message_to_db",
		func(ctx context.Context) error {
			message := &models.Message{
				FromUserID: fromUserID,
				ToUserID:   toUserID,
				Text:       text,
				IsRead:     false,
				CreatedAt:  time.Now(),
			}

			if err := db.GetDB().Create(message).Error; err != nil {
				return fmt.Errorf("failed to save message: %w", err)
			}

			messageID = message.ID
			return nil
		},
		func(ctx context.Context) error {
			if messageID > 0 {
				return db.GetDB().Delete(&models.Message{}, messageID).Error
			}
			return nil
		},
	)

	// Шаг 3: Отправляем уведомление через RabbitMQ (опционально)
	saga.AddStep(
		"send_notification",
		func(ctx context.Context) error {
			// Отправка уведомления о новом сообщении
			// З��есь может быть интеграция с системой уведомлений
			log.Printf("Notification sent: new message from %d to %d", fromUserID, toUserID)
			return nil
		},
		func(ctx context.Context) error {
			// Компенсация не требуется для уведомлений
			return nil
		},
	)

	return saga.Execute()
}

// HandleMarkAsRead обрабатывает отметку сообщений ��ак прочитанных с использованием SAGA
func (s *CounterSagaService) HandleMarkAsRead(userID, dialogPartnerID int64) error {
	sagaID := fmt.Sprintf("mark_read_%d_%d_%d", userID, dialogPartnerID, time.Now().UnixNano())
	saga := s.NewSaga(sagaID)

	var unreadCount int64
	var counterCompensation *SagaCompensation

	// Шаг 1: Получаем количество непрочитанных сообщений
	saga.AddStep(
		"count_unread_messages",
		func(ctx context.Context) error {
			var count int64
			err := db.ORM.Model(&models.Message{}).
				Where("to_user_id = ? AND from_user_id = ? AND is_read = false", userID, dialogPartnerID).
				Count(&count).Error

			if err != nil {
				return fmt.Errorf("failed to count unread messages: %w", err)
			}

			unreadCount = count
			return nil
		},
		nil, // Компенсация не требуется
	)

	// Шаг 2: Обновляем сообщения в БД
	saga.AddStep(
		"mark_messages_as_read",
		func(ctx context.Context) error {
			result := db.ORM.Model(&models.Message{}).
				Where("to_user_id = ? AND from_user_id = ? AND is_read = false", userID, dialogPartnerID).
				Update("is_read", true)

			return result.Error
		},
		func(ctx context.Context) error {
			// Откатываем изменения
			db.ORM.Model(&models.Message{}).
				Where("to_user_id = ? AND from_user_id = ?", userID, dialogPartnerID).
				Where("updated_at > ?", time.Now().Add(-1*time.Minute)).
				Update("is_read", false)
			return nil
		},
	)

	// Шаг 3: Обновляем счетчик
	saga.AddStep(
		"update_counter",
		func(ctx context.Context) error {
			if unreadCount > 0 {
				err := s.counterService.IncrementCounterSync(userID, CounterTypeUnreadMessages, -unreadCount)
				if err == nil {
					counterCompensation = s.counterService.CreateCompensation(userID, CounterTypeUnreadMessages, -unreadCount)
				}
				return err
			}
			return nil
		},
		func(ctx context.Context) error {
			if counterCompensation != nil {
				return s.counterService.ExecuteCompensation(counterCompensation)
			}
			return nil
		},
	)

	return saga.Execute()
}

// ReconcileCounter сверяет счетчик с реальными данными и исправляет расхождения
func (s *CounterSagaService) ReconcileCounter(userID int64, counterType CounterType) error {
	var actualCount int64
	var err error

	switch counterType {
	case CounterTypeUnreadMessages:
		err = db.GetDB().Model(&models.Message{}).
			Where("to_user_id = ? AND is_read = false", userID).
			Count(&actualCount).Error

	case CounterTypeUnreadDialogs:
		// Подсчитываем количество диалогов с непрочитанными сообщениями
		err = db.GetDB().Model(&models.Message{}).
			Select("COUNT(DISTINCT from_user_id)").
			Where("to_user_id = ? AND is_read = false", userID).
			Scan(&actualCount).Error

	case CounterTypeFriendRequests:
		// Подсчитываем количество запросов в друзья
		err = db.GetDB().Model(&models.Friend{}).
			Where("user_id = ? AND status = ?", userID, "pending").
			Count(&actualCount).Error

	default:
		return fmt.Errorf("unsupported counter type: %s", counterType)
	}

	if err != nil {
		return fmt.Errorf("failed to count actual value: %w", err)
	}

	// Получаем текущее значение счетчика
	currentCount, err := s.counterService.GetCounter(userID, counterType)
	if err != nil {
		return fmt.Errorf("failed to get counter: %w", err)
	}

	// Если есть расхождение, корректируем
	if currentCount != actualCount {
		log.Printf("Counter mismatch for user %d, type %s: cached=%d, actual=%d. Reconciling...",
			userID, counterType, currentCount, actualCount)

		if err := s.counterService.SetCounterValue(userID, counterType, actualCount); err != nil {
			return fmt.Errorf("failed to reconcile counter: %w", err)
		}

		log.Printf("Counter reconciled for user %d, type %s: %d -> %d",
			userID, counterType, currentCount, actualCount)
	}

	return nil
}

// runReconciliationWorker запускает воркер для периодической сверки счетчиков
func (s *CounterSagaService) runReconciliationWorker() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		s.performFullReconciliation()
	}
}

// performFullReconciliation выполняет полную сверку счетчиков
func (s *CounterSagaService) performFullReconciliation() {
	log.Println("Starting full counter reconciliation...")

	// Получаем список активных пользователей с непрочитанными сообщениями
	var userIDs []int64
	err := db.ORM.Model(&models.Message{}).
		Select("DISTINCT to_user_id").
		Where("is_read = false").
		Pluck("to_user_id", &userIDs).Error

	if err != nil {
		log.Printf("Failed to get active users for reconciliation: %v", err)
		return
	}

	reconciled := 0
	for _, userID := range userIDs {
		if err := s.ReconcileCounter(userID, CounterTypeUnreadMessages); err != nil {
			log.Printf("Failed to reconcile counter for user %d: %v", userID, err)
		} else {
			reconciled++
		}

		// Небольшая задержка, чтобы не перегружать систему
		time.Sleep(10 * time.Millisecond)
	}

	log.Printf("Reconciliation completed: %d users processed", reconciled)
}

// runConsistencyChecker запускает проверку консистентности
func (s *CounterSagaService) runConsistencyChecker() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		s.checkAndFixInconsistencies()
	}
}

// checkAndFixInconsistencies проверяет и исправляет несоответствия
func (s *CounterSagaService) checkAndFixInconsistencies() {
	// Находим пользователей с возможн��ми несоответствиями
	// Это упрощенная версия - в продакшене можно использовать более сложные эвристики

	var results []struct {
		UserID      int64
		ActualCount int64
	}

	err := db.GetDB().Model(&models.Message{}).
		Select("to_user_id as user_id, COUNT(*) as actual_count").
		Where("is_read = false").
		Where("created_at > ?", time.Now().Add(-24*time.Hour)).
		Group("to_user_id").
		Having("COUNT(*) > 0").
		Find(&results).Error

	if err != nil {
		log.Printf("Consistency check failed: %v", err)
		return
	}

	// Проверяем и исправляем несоответствия
	for _, result := range results {
		cachedCount, err := s.counterService.GetCounter(result.UserID, CounterTypeUnreadMessages)
		if err != nil {
			continue
		}

		// Если разница больше 10%, пересчитываем
		diff := abs(cachedCount - result.ActualCount)
		threshold := maxInt64(result.ActualCount/10, 5) // 10% или минимум 5

		if diff > threshold {
			log.Printf("Inconsistency detected for user %d: cached=%d, actual=%d",
				result.UserID, cachedCount, result.ActualCount)
			_ = s.ReconcileCounter(result.UserID, CounterTypeUnreadMessages)
		}
	}
}

// GetCounterStats возвращает статистику по счетчикам пользователя
func (s *CounterSagaService) GetCounterStats(userID int64) (map[string]interface{}, error) {
	counters, err := s.counterService.GetAllCounters(userID)
	if err != nil {
		return nil, err
	}

	stats := map[string]interface{}{
		"counters":  counters,
		"timestamp": time.Now().Unix(),
	}

	return stats, nil
}

// Helper functions
func abs(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
