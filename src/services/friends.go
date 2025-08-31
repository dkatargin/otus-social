package services

import (
	"context"
	"fmt"
	"social/db"
	"social/models"
	"time"
)

type FriendService struct{}

func NewFriendService() *FriendService {
	return &FriendService{}
}

// AddFriend добавляет запрос на дружбу
func (fs *FriendService) AddFriend(userID, friendID int64) error {
	if userID == friendID {
		return fmt.Errorf("cannot add yourself as friend")
	}

	// Проверяем, что пользователи существуют
	var userCount int64
	err := db.GetReadOnlyDB(context.Background()).Model(&models.User{}).Where("id IN (?)", []int64{userID, friendID}).Count(&userCount).Error
	if err != nil {
		return fmt.Errorf("error checking users: %w", err)
	}
	if userCount != 2 {
		return fmt.Errorf("one or both users do not exist")
	}

	// Проверяем, что дружба не существует
	var existingFriend models.Friend
	err = db.GetReadOnlyDB(context.Background()).Where(
		"((user_id = ? AND friend_id = ?) OR (user_id = ? AND friend_id = ?))",
		userID, friendID, friendID, userID,
	).First(&existingFriend).Error

	if err == nil {
		// Дружба уже существует
		if existingFriend.Status == "approved" {
			return fmt.Errorf("friendship already exists")
		} else {
			return fmt.Errorf("friend request already pending")
		}
	}

	// Создаем запрос на дружбу
	friendship := &models.Friend{
		UserID:    userID,
		FriendID:  friendID,
		Status:    "pending",
		CreatedAt: time.Now(),
	}

	err = db.GetWriteDB(context.Background()).Create(friendship).Error
	if err != nil {
		return fmt.Errorf("failed to create friend request: %w", err)
	}

	return nil
}

// ApproveFriend подтверждает дружбу
func (fs *FriendService) ApproveFriend(userID, requesterID int64) error {
	// Находим запрос на дружбу
	var friendship models.Friend
	err := db.GetWriteDB(context.Background()).Where(
		"user_id = ? AND friend_id = ? AND status = ?",
		requesterID, userID, "pending",
	).First(&friendship).Error

	if err != nil {
		return fmt.Errorf("friend request not found")
	}

	// Обновляем статус на approved
	friendship.Status = "approved"
	friendship.ApprovedAt = time.Now()

	err = db.GetWriteDB(context.Background()).Save(&friendship).Error
	if err != nil {
		return fmt.Errorf("failed to approve friendship: %w", err)
	}

	return nil
}

// DeleteFriend удаляет дружбу
func (fs *FriendService) DeleteFriend(userID, friendID int64) error {
	// Удаляем все записи дружбы между пользователями
	err := db.GetWriteDB(context.Background()).Where(
		"((user_id = ? AND friend_id = ?) OR (user_id = ? AND friend_id = ?))",
		userID, friendID, friendID, userID,
	).Delete(&models.Friend{}).Error

	if err != nil {
		return fmt.Errorf("failed to delete friendship: %w", err)
	}

	return nil
}

// GetFriends возвращает список друзей пользователя
func (fs *FriendService) GetFriends(userID int64) ([]models.User, error) {
	var friends []models.User

	// Получаем всех друзей (где дружба подтверждена)
	err := db.GetReadOnlyDB(context.Background()).
		Table("\"user\" u").
		Joins("JOIN friend f ON (f.user_id = u.id AND f.friend_id = ?) OR (f.friend_id = u.id AND f.user_id = ?)", userID, userID).
		Where("f.status = ? AND u.id != ?", "approved", userID).
		Select("u.id, u.nickname, u.first_name, u.last_name, u.city, u.created_at").
		Find(&friends).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get friends: %w", err)
	}

	return friends, nil
}

// GetPendingRequests возвращает входящие заявки в друзья
func (fs *FriendService) GetPendingRequests(userID int64) ([]models.User, error) {
	var requesters []models.User

	// Получаем пользователей, которые отправили заявку в друзья
	err := db.GetReadOnlyDB(context.Background()).
		Table("\"user\" u").
		Joins("JOIN friend f ON f.user_id = u.id").
		Where("f.friend_id = ? AND f.status = ?", userID, "pending").
		Select("u.id, u.nickname, u.first_name, u.last_name, u.city, u.created_at").
		Find(&requesters).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get pending requests: %w", err)
	}

	return requesters, nil
}
