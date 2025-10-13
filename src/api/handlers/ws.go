package handlers

import (
	"log"
	"net/http"
	"social/services"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// WSFeedHandler - WebSocket endpoint для ленты
func WSFeedHandler(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println("WebSocket upgrade error:", err)
		return
	}
	defer conn.Close()

	services.GlobalWSConnManager.Add(userID.(int64), conn)
	defer services.GlobalWSConnManager.Remove(userID.(int64), conn)

	// Тестовое приветствие
	_ = conn.WriteMessage(websocket.TextMessage, []byte(`{"event":"connected","message":"WebSocket connected"}`))

	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			log.Println("WebSocket read error:", err)
			break
		}
		// Здесь можно реализовать обработку входящих сообщений, если потребуется
	}
}
