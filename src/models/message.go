package models

import "time"

// Message - сообщение между пользователями (используется для шардинга)
type Message struct {
	ID         int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	FromUserID int64     `gorm:"index" json:"from_user_id"`
	ToUserID   int64     `gorm:"index" json:"to_user_id"`
	Text       string    `gorm:"type:text" json:"text"`
	CreatedAt  time.Time `gorm:"index" json:"created_at"`
	IsRead     bool      `gorm:"index" json:"is_read"`
}
