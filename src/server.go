package main

import (
	"flag"
	"log"
	"social/api/routes"
	"social/config"
	"social/db"

	"github.com/gin-gonic/gin"
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

	err = db.ConnectDB()
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
