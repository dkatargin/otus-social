package services

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"golang.org/x/crypto/argon2"
	"log"
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
	log.Println("Registering user with nickname:", *h.Nickname, h.DbModel)
	if h.DbModel == nil || h.DbModel.Password == "" {
		return nil, errors.New("nickname is empty")
	}
	log.Println("User already exists:", *h.Nickname)
	// Проверяем, существует ли пользователь с таким никнеймом
	err = db.ORM.Model(&models.User{}).Where("nickname = ?", *h.Nickname).Count(&alreadyExists).Error
	if err != nil {
		log.Println("Error checking if user exists:", err)
		return nil, err
	}
	log.Println("User already exists:", *h.Nickname)
	if alreadyExists > 0 {
		return nil, errors.New("user already exists")
	}

	log.Println("Registering user:", *h.Nickname)
	salt := make([]byte, 16)
	if _, err = rand.Read(salt); err != nil {
		log.Println("Error generating salt:", err)
		return nil, err
	}

	log.Println("Generating salt:", hex.EncodeToString(salt))
	hash := argon2.IDKey([]byte(h.DbModel.Password), salt, 1, 64*1024, 4, 32)
	passwordHash := hex.EncodeToString(salt) + "$" + hex.EncodeToString(hash)
	h.DbModel.Password = passwordHash

	log.Println("Registering user:", h.DbModel.Nickname)
	trx := db.ORM.Model(&models.User{}).Create(&h.DbModel)
	if trx.Error != nil {
		log.Println(trx.Error)
		return nil, trx.Error
	}
	log.Println(err)
	return &h.DbModel.ID, err
}

func (h *UserHandler) CheckToken() (err error) {
	if h.Token == nil || *h.Token == "" {
		return errors.New("Token is empty")
	}
	err = db.ORM.Model(&models.UserTokens{}).Where("Token = ? AND user_id = ?", *h.Token, h.DbModel.ID).First(&h.DbModel).Error
	if err != nil {
		return err
	}
	if h.DbModel.ID == 0 {
		return errors.New("user not found")
	}
	return nil
}

func (h *UserHandler) Login() (token string, err error) {
	// Получаем пользователя из БД
	var storedUser *models.User
	err = db.ORM.Model(&models.User{}).Where("user_id = ?", h.DbModel.ID).First(&storedUser).Error
	if err != nil {
		return "", err
	}
	// Проверяем пароль
	parts := strings.Split(storedUser.Password, "$")
	if len(parts) != 2 {
		return "", errors.New("invalid Password format")
	}
	storedSalt, err := hex.DecodeString(parts[0])
	if err != nil {
		return "", err
	}
	storedHash := parts[1]
	hash := argon2.IDKey([]byte(h.DbModel.Password), storedSalt, 1, 64*1024, 4, 32)
	if hex.EncodeToString(hash) != storedHash {
		return "", errors.New("invalid Password")
	}

	// Удаляем старые токены (если они есть)
	_ = h.Logout()
	// Генерируем новый токен
	tokenBytes := make([]byte, 32)
	if _, err = rand.Read(tokenBytes); err != nil {
		return "", err
	}
	token = hex.EncodeToString(tokenBytes)

	err = db.ORM.Model(&models.UserTokens{}).Create(models.UserTokens{
		UserID: h.DbModel.ID,
		Token:  token,
	}).Error
	return "", err
}

func (h *UserHandler) Logout() (err error) {
	err = db.ORM.Model(&models.UserTokens{}).Where("user_id = ?", h.DbModel.ID).Delete(&models.UserTokens{}).Error
	if err != nil {
		return err
	}
	return nil
}
