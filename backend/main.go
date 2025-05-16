// backend/main.go
package main

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

func main() {
	// 1) Init DB
	InitDB()

	// 2) Router
	r := gin.Default()

	// 3) Public
	r.POST("/register", Register)
	r.POST("/login", Login)

	// 4) Protected
	api := r.Group("/api")
	api.Use(JWTAuth())
	{
		// health-check
		api.GET("/ping", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"msg": "pong"})
		})
		// who am I
		api.GET("/me", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"id":   c.GetInt("userID"),
				"role": c.GetString("role"),
			})
		})

		// Assignments
		api.GET("/assignments", RoleGuard("student", "teacher", "admin"), listAssignments)
		api.POST("/assignments", RoleGuard("teacher", "admin"), createAssignment)
		api.GET("/assignments/:id", RoleGuard("student", "teacher", "admin"), getAssignment)
		api.PUT("/assignments/:id", RoleGuard("teacher", "admin"), updateAssignment)
		api.DELETE("/assignments/:id", RoleGuard("teacher", "admin"), deleteAssignment)
		// TEACHER / STUDENT common
		api.GET("/classes", RoleGuard("teacher", "student"), myClasses)

		// ADMIN → add teacher
		api.POST("/teachers", RoleGuard("admin"), createTeacher)

		// TEACHER only
		api.POST("/classes", RoleGuard("teacher"), createClass)
		api.POST("/classes/:id/students", RoleGuard("teacher"), addStudents)

		// Assignments now tied to class
		api.POST("/classes/:id/assignments", RoleGuard("teacher"), createAssignment) // keep old handler but pass class id

		// User deletion (admin)
		api.DELETE("/users/:id", RoleGuard("admin"), deleteUser)
		// List my submissions (student)
		api.GET("/my-submissions", RoleGuard("student"), listSubs)
	}

	log.Println("🚀 Server running on http://localhost:8080")
	if err := r.Run(":8080"); err != nil {
		log.Fatalf("could not start server: %v", err)
	}
}
