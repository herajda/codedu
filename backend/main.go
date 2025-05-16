// backend/main.go
package main

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

func main() {
	// 1️⃣ Initialize the DB (db.go)
	InitDB()

	// 2️⃣ Create the router
	r := gin.Default()

	// 3️⃣ Public routes
	r.POST("/register", Register) // students only
	r.POST("/login", Login)       // all roles

	// 4️⃣ Protected routes
	api := r.Group("/api")
	api.Use(JWTAuth()) // validate JWT and populate userID & role
	{
		api.GET("/ping", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"msg": "pong"})
		})

		api.GET("/me", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"id":   c.GetInt("userID"),
				"role": c.GetString("role"),
			})
		})

		api.POST("/assignments", RoleGuard("teacher", "admin"), createAssignment)
		api.DELETE("/users/:id", RoleGuard("admin"), deleteUser)
		api.GET("/my-submissions", RoleGuard("student"), listSubs)
	}

	log.Println("🚀 Server running on http://localhost:8080")
	log.Printf("▶️  Using DATABASE_URL=%s", dsn)
	if err := r.Run(":8080"); err != nil {
		log.Fatalf("could not start server: %v", err)
	}
}
