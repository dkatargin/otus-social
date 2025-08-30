package handlers

import (
	"net/http"
	"social/models"
	"social/services"
	"time"

	"github.com/gin-gonic/gin"
)

type LoginRequest struct {
	Nickname string `json:"nickname" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type LoginResponse struct {
	Status string `json:"status"`
	UserID string `json:"user_id"`
	Token  string `json:"token"`
}

type RegisterRequest struct {
	Nickname  string    `json:"nickname" binding:"required"`
	Password  string    `json:"password" binding:"required"`
	Firstname string    `json:"first_name"`
	Lastname  string    `json:"last_name"`
	Birthday  time.Time `json:"birthday"`
	Sex       string    `json:"sex" binding:"required"`
	Interests []string  `json:"interests"`
	City      string    `json:"city" binding:"required"`
}

type TokenRefreshRequest struct {
	Nickname string `json:"nickname" binding:"required"`
	Token    string `json:"token" binding:"required"`
}

type LogoutRequest struct {
	Nickname string `json:"nickname" binding:"required"`
	Token    string `json:"token" binding:"required"`
}

type LogoutResponse struct {
	Status string `json:"status"`
}

func Register(c *gin.Context) {
	var err error
	var registerRequest RegisterRequest
	if err = c.ShouldBindJSON(&registerRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request", "details": err.Error()})
		return
	}

	newUser := models.User{
		Nickname:  registerRequest.Nickname,
		FirstName: registerRequest.Firstname,
		LastName:  registerRequest.Lastname,
		Password:  registerRequest.Password,
		Sex:       registerRequest.Sex,
		City:      registerRequest.City,
	}

	if !registerRequest.Birthday.IsZero() {
		newUser.Birthday = registerRequest.Birthday
	}

	if registerRequest.Interests != nil {
		var newInterests []models.Interest
		for _, interest := range registerRequest.Interests {
			newInterests = append(newInterests, models.Interest{Name: interest})
		}
	}

	userHandler := services.UserHandler{
		Nickname: &registerRequest.Nickname,
		Password: &registerRequest.Password,
		DbModel:  &newUser,
	}

	userId, err := userHandler.Register()
	if err != nil {
		if err.Error() == "user already exists" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "dbModel already exists"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	if registerRequest.Interests != nil {
		interestHandler := services.InterestHandler{
			Model: &models.Interest{},
		}
		for _, interest := range registerRequest.Interests {
			interestHandler.Model.Name = interest
			if err := interestHandler.Create(); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create interest"})
				return
			}
			interestHandler.SetToUser(*userId)
		}
	}

}

func Login(c *gin.Context) {
	var loginRequest LoginRequest
	if err := c.ShouldBindJSON(&loginRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	userHandler := services.UserHandler{
		Nickname: &loginRequest.Nickname,
		Password: &loginRequest.Password,
	}

	token, err := userHandler.Login()
	if err != nil {
		if err.Error() == "invalid nickname" || err.Error() == "invalid password" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Login successful",
		"token":    token,
		"nickname": userHandler.Nickname})
}

func Logout(c *gin.Context) {
	var logoutRequest LogoutRequest
	if err := c.ShouldBindJSON(&logoutRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	userHandler := services.UserHandler{
		Nickname: &logoutRequest.Nickname,
		Token:    &logoutRequest.Token,
	}
	err := userHandler.Logout()
	if err != nil {
		if err.Error() == "token is empty" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Token is empty"})
			return
		}
		if err.Error() == "user not found" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Logout successful"})
}
