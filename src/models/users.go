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
	Firstname string    `gorm:"size:255" json:"first_name"`
	Lastname  string    `gorm:"size:255" json:"last_name"`
	Password  string    `gorm:"size:255" json:"-"`
	Birthday  time.Time `json:"birthday"`
	Sex       string    `gorm:"type:sex"`
	City      string    `gorm:"size:255" json:"city"`
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

type Migration struct {
	ID        int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	Name      string    `gorm:"size:60;uniqueIndex" json:"name"`
	AppliedAt time.Time `gorm:"autoCreateTime" json:"applied_at"`
}
