package models

import (
	"database/sql/driver"
	"time"
)

type sex string

const (
	MALE   sex = "male"
	FEMALE sex = "female"
)

func (ct *sex) Scan(value interface{}) error {
	*ct = sex(value.([]byte))
	return nil
}

func (ct sex) Value() (driver.Value, error) {
	return string(ct), nil
}

type User struct {
	ID        int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	Nickname  string    `gorm:"size:60;uniqueIndex" json:"nickname"`
	FirstName string    `gorm:"size:255" json:"first_name"`
	LastName  string    `gorm:"size:255" json:"last_name"`
	Password  string    `gorm:"size:255" json:"-"`
	Birthday  time.Time `json:"birthday"`
	Sex       string    `gorm:"type:sex"`
	City      string    `gorm:"size:255" json:"city"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
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
	UserID int64  `gorm:"index" json:"user_id"`
	Token  string `gorm:"size:255" json:"token"`
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
