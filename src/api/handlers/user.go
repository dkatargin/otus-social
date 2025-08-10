package handlers

import (
	"social/db"

	"github.com/gin-gonic/gin"
)

type UserInfo struct {
	Nickname  string `json:"nickname"`
	Firstname string `json:"first_name"`
	Lastname  string `json:"last_name"`
}

func UserSearch(c *gin.Context) {
	query, hasQuery := c.GetQuery("query")
	if !hasQuery {
		c.JSON(400, gin.H{"error": "Search query is required"})
		return
	}

	var users []UserInfo

	db.ORM.Table("users").Select(
		"nickname, firstname, lastname").Where(
		"firstname ILIKE ? OR lastname ILIKE ?", query+"%", query+"%").Find(&users)
	if len(users) == 0 {
		c.JSON(404, gin.H{"error": "No users found"})
		return
	}
	c.JSON(200, gin.H{"users": users})
}

func UserGet(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(400, gin.H{"error": "User ID is required"})
		return
	}

	var user UserInfo

	result := db.ORM.Table("users").Select(
		"nickname, firstname, lastname").Where("id = ?", id).First(&user)
	if result.Error != nil {
		c.JSON(404, gin.H{"error": "User not found"})
		return
	}
	c.JSON(200, gin.H{"user": user})
}
