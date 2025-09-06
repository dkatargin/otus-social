package handlers

import (
	"social/db"
	"social/models"
	"social/services"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

type UserInfo struct {
	ID        int64  `json:"id"`
	Nickname  string `json:"nickname"`
	Firstname string `json:"first_name"`
	Lastname  string `json:"last_name"`
}

type UserRegisterRequest struct {
	Nickname  string `json:"nickname" binding:"required"`
	Password  string `json:"password" binding:"required"`
	FirstName string `json:"first_name" binding:"required"`
	LastName  string `json:"last_name" binding:"required"`
	Birthday  string `json:"birthday" binding:"required"`
	Sex       string `json:"sex" binding:"required"`
	City      string `json:"city" binding:"required"`
}

func UserSearch(c *gin.Context) {
	firstName := c.Query("first_name")
	lastName := c.Query("last_name")

	if firstName == "" && lastName == "" {
		c.JSON(400, gin.H{"error": "At least one search parameter (first_name or last_name) is required"})
		return
	}

	limit := 50
	offset := 0
	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 1000 {
			limit = l
		}
	}
	if offsetStr := c.Query("offset"); offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	users, err := services.SearchUsers(c.Request.Context(), firstName, lastName, limit, offset)
	if err != nil {
		c.JSON(500, gin.H{"error": "Internal server error"})
		return
	}

	if len(users) == 0 {
		c.JSON(404, gin.H{"error": "No users found"})
		return
	}

	var userInfos []UserInfo
	for _, user := range users {
		userInfos = append(userInfos, UserInfo{
			ID:        user.ID,
			Nickname:  user.Nickname,
			Firstname: user.FirstName,
			Lastname:  user.LastName,
		})
	}

	c.JSON(200, gin.H{"users": userInfos})
}

func UserGet(c *gin.Context) {
	idParam := c.Param("id")
	if idParam == "" {
		c.JSON(400, gin.H{"error": "User ID is required"})
		return
	}

	id, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid user ID"})
		return
	}

	user, err := services.GetUser(c.Request.Context(), id)
	if err != nil {
		c.JSON(404, gin.H{"error": "User not found"})
		return
	}

	userInfo := UserInfo{
		ID:        user.ID,
		Nickname:  user.Nickname,
		Firstname: user.FirstName,
		Lastname:  user.LastName,
	}

	c.JSON(200, gin.H{"user": userInfo})
}

func UserRegister(c *gin.Context) {
	var req UserRegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}

	// Парсим дату рождения
	birthday, err := time.Parse("2006-01-02", req.Birthday)
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid birthday format. Use YYYY-MM-DD"})
		return
	}

	// Проверяем пол
	if req.Sex != "male" && req.Sex != "female" {
		c.JSON(400, gin.H{"error": "Sex must be 'male' or 'female'"})
		return
	}

	// Создаем пользователя
	user := &models.User{
		Nickname:  req.Nickname,
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Password:  req.Password,
		Birthday:  birthday,
		Sex:       models.Sex(req.Sex),
		City:      req.City,
	}

	handler := &services.UserHandler{
		Nickname: &req.Nickname,
		Password: &req.Password,
		DbModel:  user,
	}

	userId, err := handler.Register()
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	// Записываем информацию о транзакции для тестирования
	writeTransaction := &models.WriteTransaction{
		TableName:   "users",
		Operation:   "INSERT",
		RecordID:    *userId,
		Timestamp:   time.Now(),
		TestSession: c.GetHeader("X-Test-Session"),
	}
	db.GetWriteDB(c.Request.Context()).Create(writeTransaction)

	c.JSON(201, gin.H{
		"user_id": userId,
		"message": "User registered successfully",
	})
}
