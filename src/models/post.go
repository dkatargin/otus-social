package models

import "time"

// Post - модель поста пользователя
type Post struct {
	ID        int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID    int64     `gorm:"index" json:"user_id"`
	Content   string    `gorm:"type:text" json:"content"`
	CreatedAt time.Time `gorm:"index" json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (Post) TableName() string {
	return "posts"
}

// FeedPost - структура для ленты с дополнительной информацией о пользователе
type FeedPost struct {
	ID         int64     `json:"id"`
	UserID     int64     `json:"user_id"`
	UserName   string    `json:"user_name"`
	UserAvatar string    `json:"user_avatar,omitempty"`
	Content    string    `json:"content"`
	CreatedAt  time.Time `json:"created_at"`
}

// FeedResponse - ответ API для ленты
type FeedResponse struct {
	Posts   []FeedPost `json:"posts"`
	HasMore bool       `json:"has_more"`
	LastID  int64      `json:"last_id,omitempty"`
}
