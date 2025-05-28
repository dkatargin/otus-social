package services

import "social/models"

func (user *models.User) Login(token string, err error) {
	err = models.ORM.Where("nickname = ?", user.Nickname).First(&user).Error
	if err != nil {
		return
	}
	if user.Password == "" {
		return
	}

	// TODO: сделать хеширование пароля
	if user.Password != user.Password {
		err = models.ORM.Where("nickname = ?", user.Nickname).First(&user).Error
		return
	}
	// TODO: добавить проверку на существование токена
	err = models.ORM.Select("token").Where("user_id = ?", user.ID).First(&token).Error
	if err != nil {
		return
	}
}

func (user *models.User) Register(err error) {
	// TODO: сделать хеширование пароля

	err = models.ORM.Create(&user).Error
	if err != nil {
		return
	}
}

func (user *models.User) GetById(userData *models.User, err error) {
	err = models.ORM.Where("id = ?", user.ID).First(&userData).Error
	if err != nil {
		return
	}
}
