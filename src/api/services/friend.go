package services

import (
	"errors"
	"social/db"
	"social/models"
	"time"

	"gorm.io/gorm"
)

// FriendService - сервис для работы с дружбой
// Здесь будут методы для добавления, подтверждения и удаления друзей

type FriendService struct{}

func NewFriendService() *FriendService {
	return &FriendService{}
}

// AddFriend - создать заявку в друзья
func (s *FriendService) AddFriend(userID, friendID int64) error {
	// Валидация входных данных
	if userID <= 0 || friendID <= 0 {
		return errors.New("invalid user ID")
	}

	// Проверка на самодобавление
	if userID == friendID {
		return errors.New("cannot add yourself as friend")
	}

	// Проверка на существование заявки в любом направлении
	var existing models.Friend
	err := db.ORM.Where(
		"(user_id = ? AND friend_id = ?) OR (user_id = ? AND friend_id = ?)",
		userID, friendID, friendID, userID,
	).First(&existing).Error

	if err == nil {
		if existing.Status == "approved" {
			return errors.New("users are already friends")
		}
		return errors.New("friend request already exists")
	}

	// Создание заявки
	friend := models.Friend{
		UserID:    userID,
		FriendID:  friendID,
		Status:    "pending",
		CreatedAt: time.Now(),
	}
	return db.ORM.Create(&friend).Error
}

// ApproveFriend - подтвердить заявку в друзья
func (s *FriendService) ApproveFriend(userID, friendID int64) error {
	// Валидация входных данных
	if userID <= 0 || friendID <= 0 {
		return errors.New("invalid user ID")
	}

	return db.ORM.Transaction(func(tx *gorm.DB) error {
		// Найти входящую заявку
		var friend models.Friend
		err := tx.Where("user_id = ? AND friend_id = ? AND status = ?", friendID, userID, "pending").First(&friend).Error
		if err != nil {
			return errors.New("friend request not found")
		}

		// Обновить статус заявки
		friend.Status = "approved"
		friend.ApprovedAt = time.Now()
		if err := tx.Save(&friend).Error; err != nil {
			return err
		}

		// Создать обратную связь для взаимной дружбы
		reverseFriend := models.Friend{
			UserID:     userID,
			FriendID:   friendID,
			Status:     "approved",
			CreatedAt:  time.Now(),
			ApprovedAt: time.Now(),
		}
		return tx.Create(&reverseFriend).Error
	})
}

// DeleteFriend - удалить друга или заявку
func (s *FriendService) DeleteFriend(userID, friendID int64) error {
	// Валидация входных данных
	if userID <= 0 || friendID <= 0 {
		return errors.New("invalid user ID")
	}

	return db.ORM.Transaction(func(tx *gorm.DB) error {
		// Удалить все связи между пользователями (в обоих направлениях)
		result := tx.Where(
			"(user_id = ? AND friend_id = ?) OR (user_id = ? AND friend_id = ?)",
			userID, friendID, friendID, userID,
		).Delete(&models.Friend{})

		if result.Error != nil {
			return result.Error
		}

		if result.RowsAffected == 0 {
			return errors.New("friendship not found")
		}

		return nil
	})
}

// GetFriends - получить список друзей пользователя
func (s *FriendService) GetFriends(userID int64) ([]models.Friend, error) {
	if userID <= 0 {
		return nil, errors.New("invalid user ID")
	}

	var friends []models.Friend
	err := db.ORM.Where("user_id = ? AND status = ?", userID, "approved").Find(&friends).Error
	return friends, err
}

// GetPendingRequests - получить входящие заявки в друзья
func (s *FriendService) GetPendingRequests(userID int64) ([]models.Friend, error) {
	if userID <= 0 {
		return nil, errors.New("invalid user ID")
	}

	var requests []models.Friend
	err := db.ORM.Where("friend_id = ? AND status = ?", userID, "pending").Find(&requests).Error
	return requests, err
}
