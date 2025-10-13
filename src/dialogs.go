package main

import (
	"social/api/routes"

	"github.com/gin-gonic/gin"
)

func main() {

	router := gin.Default()

	router.Use(gin.Logger())
	router.Use(gin.Recovery())

	routes.DialogInternalApi(router)

	if err := router.Run(":8080"); err != nil {
		panic(err)
	}

}
