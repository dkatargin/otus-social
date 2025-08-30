package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"golang.org/x/crypto/argon2"
	"social/db"
	"social/models"
	"strings"
	"time"
)

type UserExtra struct {
	Firstname *string    `json:"first_name"`
	Lastname  *string    `json:"last_name"`
	Birthday  *time.Time `json:"birthday"`
	Sex       *string    `json:"sex"`
	City      *string    `json:"city"`
}

type UserHandler struct {
	Nickname *string
	Password *string
	Token    *string
	Extra    *UserExtra

	DbModel *models.User
}

func (h *UserHandler) Register() (userId *int64, err error) {
	var alreadyExists int64
	if h.DbModel == nil || h.DbModel.Password == "" {
		return nil, errors.New("nickname is empty")
	}

	ctx := context.Background()
	// Проверяем, существует ли пользователь с таким никнеймом (read-only операция)
	err = db.GetReadOnlyDB(ctx).Model(&models.User{}).Where("nickname = ?", *h.Nickname).Count(&alreadyExists).Error
	if err != nil {
		return nil, err
	}
	if alreadyExists > 0 {
		return nil, errors.New("user already exists")
	}

	salt := make([]byte, 16)
	if _, err = rand.Read(salt); err != nil {
		return nil, err
	}

	hash := argon2.IDKey([]byte(h.DbModel.Password), salt, 1, 64*1024, 4, 32)
	passwordHash := hex.EncodeToString(salt) + "$" + hex.EncodeToString(hash)
	h.DbModel.Password = passwordHash

	// Запись в мастер
	trx := db.GetWriteDB(ctx).Model(&models.User{}).Create(&h.DbModel)
	if trx.Error != nil {
		return nil, trx.Error
	}
	return &h.DbModel.ID, err
}

func (h *UserHandler) CheckToken() (err error) {
	if h.Token == nil || *h.Token == "" {
		return errors.New("Token is empty")
	}

	ctx := context.Background()
	// Чтение токена - read-only операция
	err = db.GetReadOnlyDB(ctx).Model(&models.UserTokens{}).Where("Token = ? AND user_id = ?", *h.Token, h.DbModel.ID).First(&h.DbModel).Error
	if err != nil {
		return err
	}
	if h.DbModel.ID == 0 {
		return errors.New("user not found")
	}
	return nil
}

func (h *UserHandler) Login() (token string, err error) {
	ctx := context.Background()
	// Получаем пользователя из БД (read-only операция)
	var storedUser *models.User
	err = db.GetReadOnlyDB(ctx).Model(&models.User{}).Where("nickname = ?", h.Nickname).First(&storedUser).Error
	if err != nil {
		return "", errors.New("invalid nickname")
	}
	// Проверяем пароль
	parts := strings.Split(storedUser.Password, "$")
	if len(parts) != 2 {
		return "", errors.New("invalid password format")
	}
	storedSalt, err := hex.DecodeString(parts[0])
	if err != nil {
		return "", err
	}
	storedHash := parts[1]
	hash := argon2.IDKey([]byte(*h.Password), storedSalt, 1, 64*1024, 4, 32)
	if hex.EncodeToString(hash) != storedHash {
		return "", errors.New("invalid password")
	}

	// Удаляем старые токены (если они есть)
	_ = h.Logout()
	// Генерируем новый токен
	tokenBytes := make([]byte, 32)
	if _, err = rand.Read(tokenBytes); err != nil {
		return "", err
	}
	token = hex.EncodeToString(tokenBytes)
	// Запись токена в мастер
	err = db.GetWriteDB(ctx).Model(&models.UserTokens{}).Create(&models.UserTokens{
		UserID: storedUser.ID,
		Token:  token,
	}).Error
	return token, err
}

func (h *UserHandler) Logout() (err error) {
	ctx := context.Background()
	var userId int64
	// Чтение для п��лучения ID (read-only)
	db.GetReadOnlyDB(ctx).Model(&models.User{}).Select("id").Where("nickname = ?", h.Nickname).First(&userId)
	// Удаление токена (запись в мастер)
	err = db.GetWriteDB(ctx).Table("user_tokens").Where("user_id = ?", userId).Delete(&models.UserTokens{}).Error
	if err != nil {
		return err
	}
	return nil
}

// GetUser получает пользователя по ID (read-only операция)
func GetUser(ctx context.Context, userID int64) (*models.User, error) {
	var user models.User
	err := db.GetReadOnlyDB(ctx).Where("id = ?", userID).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// SearchUsers ищет пользователей по имени и фамилии (read-only операция)
func SearchUsers(ctx context.Context, firstName, lastName string, limit, offset int) ([]models.User, error) {
	var users []models.User
	query := db.GetReadOnlyDB(ctx).Model(&models.User{})

	if firstName != "" {
		query = query.Where("first_name ILIKE ?", firstName+"%")
	}
	if lastName != "" {
		query = query.Where("last_name ILIKE ?", lastName+"%")
	}

	err := query.Order("id").Limit(limit).Offset(offset).Find(&users).Error
	return users, err
}
