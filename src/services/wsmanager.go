package services

import (
	"sync"

	"github.com/gorilla/websocket"
)

type WSConnManager struct {
	mu    sync.RWMutex
	users map[int64][]*websocket.Conn
}

func NewWSConnManager() *WSConnManager {
	return &WSConnManager{
		users: make(map[int64][]*websocket.Conn),
	}
}

func (m *WSConnManager) Add(userID int64, conn *websocket.Conn) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.users[userID] = append(m.users[userID], conn)
}

func (m *WSConnManager) Remove(userID int64, conn *websocket.Conn) {
	m.mu.Lock()
	defer m.mu.Unlock()
	conns := m.users[userID]
	for i, c := range conns {
		if c == conn {
			m.users[userID] = append(conns[:i], conns[i+1:]...)
			break
		}
	}
	if len(m.users[userID]) == 0 {
		delete(m.users, userID)
	}
}

func (m *WSConnManager) Send(userID int64, message []byte) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, conn := range m.users[userID] {
		_ = conn.WriteMessage(websocket.TextMessage, message)
	}
}

var GlobalWSConnManager = NewWSConnManager()
