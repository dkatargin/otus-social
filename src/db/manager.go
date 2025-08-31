package db

import (
	"context"
	"fmt"
	"log"
	"social/config"
	"social/models"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
	"gorm.io/plugin/dbresolver"
)

var ORM *gorm.DB

func dsnFromConfig(dbConf config.DBConfig) string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		dbConf.Host, dbConf.Port, dbConf.User, dbConf.Password, dbConf.DBName,
	)
}

func ConnectDB() (err error) {
	if ORM != nil {
		log.Println("ORM is already initialized")
		return nil
	}

	if config.AppConfig == nil {
		return fmt.Errorf("AppConfig is not loaded")
	}

	if config.AppConfig.Databases.Master.Host == "" {
		return fmt.Errorf("Master database configuration is missing")
	}
	var conf = config.AppConfig
	if conf == nil {
		return fmt.Errorf("AppConfig is nil")
	}

	// Initialize the ORM with the master database
	masterDSN := dsnFromConfig(conf.Databases.Master)
	// Init replicas
	replicaDSNs := make([]gorm.Dialector, 0, len(conf.Databases.Replicas))
	for _, r := range conf.Databases.Replicas {
		replicaDSNs = append(replicaDSNs, postgres.Open(dsnFromConfig(r)))
	}

	db, err := gorm.Open(postgres.Open(masterDSN), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{
			SingularTable: true,
			NoLowerCase:   false,
		},
	})
	if err != nil {
		return err
	}

	if len(replicaDSNs) > 0 {
		err = db.Use(dbresolver.Register(dbresolver.Config{
			Replicas: replicaDSNs,
			Policy:   dbresolver.RandomPolicy{},
		}))
		if err != nil {
			return
		}
	}

	err = db.AutoMigrate(&models.User{}, &models.Migration{}, &models.UserTokens{}, &models.WriteTransaction{}, &models.Interest{}, &models.UserInterest{}, &models.Friend{}, &models.Post{})

	ORM = db
	return nil
}

// GetReadOnlyDB возвращает подключение для чтения (слейвы)
func GetReadOnlyDB(ctx context.Context) *gorm.DB {
	return ORM.WithContext(ctx).Clauses(dbresolver.Read)
}

// GetWriteDB возвращает подключение для записи (мастер)
func GetWriteDB(ctx context.Context) *gorm.DB {
	return ORM.WithContext(ctx).Clauses(dbresolver.Write)
}
