package models

import "time"

// Friend - модель для хранения дружбы между пользователями
// Status: "pending" (ожидание), "approved" (подтверждена)
type Friend struct {
	ID         int64     `json:"id"`
	UserID     int64     `json:"user_id"`
	FriendID   int64     `json:"friend_id"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
	ApprovedAt time.Time `json:"approved_at,omitempty"`
}
