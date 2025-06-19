package services

import (
	"errors"
	"gorm.io/gorm"
	"social/db"
	"social/models"
)

type InterestHandler struct {
	DB    *gorm.DB
	Model *models.Interest
}

func (i *InterestHandler) Get() (interest *models.Interest, err error) {
	err = db.ORM.Model(i.Model).Select("*").First(&interest).Error
	return interest, err
}

func (i *InterestHandler) Create() (err error) {
	var alreadyExists bool
	err = db.ORM.Where("name = ?", i.Model.Name).First(&alreadyExists).Error
	if alreadyExists {
		return errors.New("interest already exists")
	}
	return err
}

func (i *InterestHandler) SetToUser(userId int64) (err error) {
	var alreadyExist bool
	err = db.ORM.Model(&models.UserInterest{}).Where("user_id = ? AND interest_id = ?", userId, i.Model.ID).First(&alreadyExist).Error
	if err != nil {
		return err
	}
	if alreadyExist {
		return errors.New("interest already exists for user")
	}

	err = db.ORM.Model(models.UserInterest{}).Create(models.UserInterest{
		UserID:     userId,
		InterestID: i.Model.ID,
	}).Error

	return err
}

func (i *InterestHandler) GetByUser(userId int64) (interest *[]models.Interest, err error) {
	err = db.ORM.Model(&models.UserInterest{}).Where("user_id = ?", userId).Scan(&interest).Error
	if err != nil {
		return nil, err
	}
	return interest, nil
}
