package models

import "time"

// Friend - модель для хранения дружбы между пользователями
// Status: "pending" (ожидание), "approved" (подтверждена)
type Friend struct {
	ID         int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID     int64     `gorm:"index" json:"user_id"`
	FriendID   int64     `gorm:"index" json:"friend_id"`
	Status     string    `gorm:"type:varchar(20);default:pending" json:"status"`
	CreatedAt  time.Time `gorm:"autoCreateTime" json:"created_at"`
	ApprovedAt time.Time `json:"approved_at,omitempty"`
}

func (Friend) TableName() string {
	return "friends"
}
