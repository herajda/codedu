package main

import (
    "github.com/gin-gonic/gin"
)

func main() {
    r := gin.Default()

    // Basic route
    r.GET("/api/ping", func(c *gin.Context) {
        c.JSON(200, gin.H{"message": "pong"})
    })

    // Run server
    r.Run(":8080")
}
