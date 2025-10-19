package handlers

import (
	"net/http"
	"strconv"

	"social/services"

	"github.com/gin-gonic/gin"
)

// GetCounters возвращает все счетчики для текущего пользователя
// @Summary Get user counters
// @Description Get all counters (unread messages, dialogs, friend requests, etc.) for the authenticated user
// @Tags counters
// @Security Bearer
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /counters [get]
func GetCounters(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not authenticated"})
		return
	}

	uid, ok := userID.(int64)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user id"})
		return
	}

	counterService := services.GetCounterService()
	counters, err := counterService.GetAllCounters(uid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get counters"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user_id":  uid,
		"counters": counters,
	})
}

// GetCounterByType возвращает конкретный счетчик
// @Summary Get specific counter
// @Description Get a specific counter by type
// @Tags counters
// @Security Bearer
// @Produce json
// @Param type path string true "Counter type (unread_messages, unread_dialogs, friend_requests, notifications)"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /counters/{type} [get]
func GetCounterByType(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not authenticated"})
		return
	}

	uid, ok := userID.(int64)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user id"})
		return
	}

	counterType := services.CounterType(c.Param("type"))

	// Ва��идация типа счетчика
	validTypes := map[services.CounterType]bool{
		services.CounterTypeUnreadMessages: true,
		services.CounterTypeUnreadDialogs:  true,
		services.CounterTypeFriendRequests: true,
		services.CounterTypeNotifications:  true,
	}

	if !validTypes[counterType] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid counter type"})
		return
	}

	counterService := services.GetCounterService()
	count, err := counterService.GetCounter(uid, counterType)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get counter"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user_id": uid,
		"type":    counterType,
		"count":   count,
	})
}

// ResetCounter сбрасывает счетчик
// @Summary Reset counter
// @Description Reset a specific counter to zero
// @Tags counters
// @Security Bearer
// @Accept json
// @Produce json
// @Param type path string true "Counter type"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /counters/{type}/reset [post]
func ResetCounter(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not authenticated"})
		return
	}

	uid, ok := userID.(int64)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user id"})
		return
	}

	counterType := services.CounterType(c.Param("type"))

	counterService := services.GetCounterService()
	if err := counterService.ResetCounter(uid, counterType); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to reset counter"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "counter reset successfully",
		"type":    counterType,
	})
}

// ReconcileCounter принудительно сверяет счетчик с реальными данными
// @Summary Reconcile counter
// @Description Force reconciliation of counter with actual data
// @Tags counters
// @Security Bearer
// @Produce json
// @Param type path string true "Counter type"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /counters/{type}/reconcile [post]
func ReconcileCounter(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not authenticated"})
		return
	}

	uid, ok := userID.(int64)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user id"})
		return
	}

	counterType := services.CounterType(c.Param("type"))

	sagaService := services.GetCounterSagaService()
	if err := sagaService.ReconcileCounter(uid, counterType); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to reconcile counter"})
		return
	}

	// Получаем обновленное значение
	counterService := services.GetCounterService()
	count, _ := counterService.GetCounter(uid, counterType)

	c.JSON(http.StatusOK, gin.H{
		"message": "counter reconciled successfully",
		"type":    counterType,
		"count":   count,
	})
}

// GetCounterStats возвращает детальную статистику по счетчикам
// @Summary Get counter statistics
// @Description Get detailed statistics for user counters
// @Tags counters
// @Security Bearer
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /counters/stats [get]
func GetCounterStats(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not authenticated"})
		return
	}

	uid, ok := userID.(int64)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user id"})
		return
	}

	sagaService := services.GetCounterSagaService()
	stats, err := sagaService.GetCounterStats(uid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get stats"})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// IncrementCounter ручное увеличение счетчика (для тестирования/админки)
// @Summary Increment counter
// @Description Manually increment a counter (admin/testing)
// @Tags counters
// @Security Bearer
// @Accept json
// @Produce json
// @Param type path string true "Counter type"
// @Param body body map[string]int64 true "Delta value"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /counters/{type}/increment [post]
func IncrementCounter(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not authenticated"})
		return
	}

	uid, ok := userID.(int64)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user id"})
		return
	}

	counterType := services.CounterType(c.Param("type"))

	// Validate counterType
	if _, ok := services.ValidTypes[counterType]; !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid counter type"})
		return
	}
	var req struct {
		Delta int64 `json:"delta" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	counterService := services.GetCounterService()
	if err := counterService.IncrementCounterSync(uid, counterType, req.Delta); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to increment counter"})
		return
	}

	// Получаем обновленное значение
	count, _ := counterService.GetCounter(uid, counterType)

	c.JSON(http.StatusOK, gin.H{
		"message": "counter incremented successfully",
		"type":    counterType,
		"count":   count,
	})
}

// GetUnreadMessagesCount быстрое получение количества непрочитанных сообщений
// @Summary Get unread messages count
// @Description Quick endpoint to get only unread messages count
// @Tags counters
// @Security Bearer
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /counters/unread-messages [get]
func GetUnreadMessagesCount(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not authenticated"})
		return
	}

	uid, ok := userID.(int64)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user id"})
		return
	}

	counterService := services.GetCounterService()
	count, err := counterService.GetCounter(uid, services.CounterTypeUnreadMessages)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get counter"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"unread_count": count,
	})
}

// GetBatchCounters получает счетчики для нескольких пользователей (для админки)
// @Summary Get batch counters
// @Description Get counters for multiple users (admin only)
// @Tags counters
// @Security Bearer
// @Accept json
// @Produce json
// @Param body body map[string][]int64 true "User IDs"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /counters/batch [post]
func GetBatchCounters(c *gin.Context) {
	var req struct {
		UserIDs []int64 `json:"user_ids" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	if len(req.UserIDs) > 100 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "too many users requested (max 100)"})
		return
	}

	counterService := services.GetCounterService()
	results := make(map[int64]map[services.CounterType]int64)

	for _, uid := range req.UserIDs {
		counters, err := counterService.GetAllCounters(uid)
		if err != nil {
			continue
		}
		results[uid] = counters
	}

	c.JSON(http.StatusOK, gin.H{
		"results": results,
	})
}

// GetDialogCounters получает счетчики для конкретного диалога
// @Summary Get dialog counters
// @Description Get unread message count for a specific dialog
// @Tags counters
// @Security Bearer
// @Produce json
// @Param user_id path int true "Dialog partner user ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /counters/dialogs/{user_id} [get]
func GetDialogCounters(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not authenticated"})
		return
	}

	uid, ok := userID.(int64)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user id"})
		return
	}

	partnerIDStr := c.Param("user_id")
	partnerID, err := strconv.ParseInt(partnerIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid partner user id"})
		return
	}

	// Получаем статистику диалога из Redis
	dialogService := services.GetRedisDialogService()
	if dialogService == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "dialog service not available"})
		return
	}

	stats, err := dialogService.GetDialogStats(uid, partnerID, uid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get dialog stats"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user_id":      uid,
		"partner_id":   partnerID,
		"unread_count": stats.UnreadCount,
	})
}
