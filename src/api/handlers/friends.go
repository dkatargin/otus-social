package handlers

import (
	"net/http"
	"social/services"

	"github.com/gin-gonic/gin"
)

var friendService = services.NewFriendService()

// AddFriend - обработчик для добавления друга
func AddFriend(c *gin.Context) {
	// Получаем user_id из контекста (устанавливается middleware)
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	type req struct {
		FriendID int64 `json:"friend_id"`
	}
	var r req
	if err := c.ShouldBindJSON(&r); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	if err := friendService.AddFriend(userID.(int64), r.FriendID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "friend request sent"})
}

// ApproveFriend - обработчик для подтверждения дружбы
func ApproveFriend(c *gin.Context) {
	// Получаем user_id из контекста (устанавливается middleware)
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	type req struct {
		FriendID int64 `json:"friend_id"`
	}
	var r req
	if err := c.ShouldBindJSON(&r); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	if err := friendService.ApproveFriend(userID.(int64), r.FriendID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "friendship approved"})
}

// DeleteFriend - обработчик для удаления друга
func DeleteFriend(c *gin.Context) {
	// Получаем user_id из контекста (устанавливается middleware)
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	type req struct {
		FriendID int64 `json:"friend_id"`
	}
	var r req
	if err := c.ShouldBindJSON(&r); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	if err := friendService.DeleteFriend(userID.(int64), r.FriendID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "friend deleted"})
}

// GetFriends - обработчик для получения списка друзей
func GetFriends(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	friends, err := friendService.GetFriends(userID.(int64))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"friends": friends})
}

// GetPendingRequests - обработчик для получения входящих заявок в друзья
func GetPendingRequests(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	requests, err := friendService.GetPendingRequests(userID.(int64))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"requests": requests})
}
