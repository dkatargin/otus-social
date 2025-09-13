package models

import (
	"time"
)

type Sex string

const (
	MALE   Sex = "male"
	FEMALE Sex = "female"
)

type User struct {
	ID        int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	Nickname  string    `gorm:"size:60;uniqueIndex" json:"nickname"`
	FirstName string    `gorm:"size:255" json:"first_name"`
	LastName  string    `gorm:"size:255" json:"last_name"`
	Password  string    `gorm:"size:255" json:"-"`
	Birthday  time.Time `json:"birthday"`
	Sex       Sex       `gorm:"type:sex" json:"sex"`
	City      string    `gorm:"size:255" json:"city"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (User) TableName() string {
	return "users"
}

type Interest struct {
	ID   int64  `gorm:"primaryKey;autoIncrement" json:"id"`
	Name string `gorm:"size:60;uniqueIndex:interests_name_key" json:"name"`
}

type UserInterest struct {
	ID         int64 `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID     int64 `gorm:"index" json:"user_id"`
	InterestID int64 `gorm:"index" json:"interest_id"`
}

type UserTokens struct {
	ID     int64  `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID int64  `gorm:"index:user_token_idx,unique" json:"user_id"`
	Token  string `gorm:"size:255;index:user_token_idx,unique" json:"token"`
}

func (UserTokens) TableName() string {
	return "user_tokens"
}

// WriteTransaction для отслеживания записей во время нагрузочного тестирования
type WriteTransaction struct {
	ID          int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	TableName   string    `gorm:"size:100" json:"table_name"`
	Operation   string    `gorm:"size:20" json:"operation"` // INSERT, UPDATE, DELETE
	RecordID    int64     `json:"record_id"`
	Timestamp   time.Time `json:"timestamp"`
	TestSession string    `gorm:"size:100" json:"test_session"`
}

type Migration struct {
	ID        int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	Name      string    `gorm:"size:60;uniqueIndex" json:"name"`
	AppliedAt time.Time `gorm:"autoCreateTime" json:"applied_at"`
}
