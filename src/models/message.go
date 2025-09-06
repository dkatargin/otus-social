package models

import (
	"time"
)

// Message представляет сообщение в диалоге между пользователями
type Message struct {
	ID         int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	FromUserID int64     `gorm:"column:from_user_id;index" json:"from_id"`
	ToUserID   int64     `gorm:"column:to_user_id;index" json:"to_id"`
	Text       string    `gorm:"type:text;not null" json:"text"`
	CreatedAt  time.Time `gorm:"autoCreateTime" json:"created_at"`
	IsRead     bool      `gorm:"default:false" json:"is_read"`
}

// TableName возвращает имя таблицы для модели Message
func (Message) TableName() string {
	return "messages"
}
