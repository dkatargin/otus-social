package db

import (
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"log"
	"social/models"
)

var ORM *gorm.DB

func ConnectDB(dsn string) (err error) {
	if ORM != nil {
		return nil // Already connected
	}
	log.Println("Connecting to the database: " + dsn)
	ORM, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})

	if err != nil {
		return err
	}
	err = ORM.AutoMigrate(&models.User{}, &models.Migration{})
	return err
}
