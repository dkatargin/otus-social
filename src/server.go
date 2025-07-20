package main

import (
	"flag"
	"fmt"
	"github.com/gin-gonic/gin"
	"log"
	"social/api/routes"
	"social/config"
	"social/db"
)

func main() {
	var configPath string
	flag.StringVar(&configPath, "config", "config.yaml", "Path to the configuration file")
	flag.Parse()

	err := config.LoadConfig(configPath)
	if err != nil {
		panic("Failed to load configuration: " + err.Error())
	}
	log.Println("Starting server...", config.AppConfig)

	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=5432 sslmode=disable TimeZone=UTC",
		config.AppConfig.Database.Host, config.AppConfig.Database.User, config.AppConfig.Database.Password,
		config.AppConfig.Database.Name)

	err = db.ConnectDB(dsn)
	if err != nil {
		panic("Failed to connect to the database: " + err.Error())
	}

	router := gin.Default()

	router.Use(gin.Logger())
	router.Use(gin.Recovery())

	routes.PublicApi(router)

	// Start the server
	if err := router.Run(":8080"); err != nil {
		panic(err)
	}
}
