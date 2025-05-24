// backend/main.go
package main

import (
	"log"
	"net/http"

	"golang.org/x/crypto/bcrypt"

	"github.com/gin-gonic/gin"
)

// seed a single admin so you’re never locked out
func ensureAdmin() {
	const (
		email    = "admin@example.com"
		password = "admin123"
	)
	var exists bool
	if err := DB.Get(&exists,
		`SELECT EXISTS(SELECT 1 FROM users WHERE email=$1)`, email); err != nil {
		log.Fatalf("admin check failed: %v", err)
	}
	if !exists {
		hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if _, err := DB.Exec(`
		    INSERT INTO users (email,password_hash,role)
		    VALUES ($1,$2,'admin')`, email, hash); err != nil {
			log.Fatalf("could not insert admin: %v", err)
		}
		log.Printf("👑  Admin seeded → %s / %s", email, password)
	}
}

func main() {
	// 1) Init DB
	InitDB()
	ensureAdmin()

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
		api.POST("/classes/:id/students", RoleGuard("teacher", "admin"), addStudents)
		api.GET("/classes/all", RoleGuard("admin"), listAllClasses) // new

		// ADMIN → add teacher
		api.POST("/teachers", RoleGuard("admin"), createTeacher)
		api.GET("/users", RoleGuard("admin"), listUsers)               // new
		api.PUT("/users/:id/role", RoleGuard("admin"), updateUserRole) // new

		// TEACHER only
		api.POST("/classes", RoleGuard("teacher"), createClass)

		// Assignments now tied to class
		api.POST("/classes/:id/assignments", RoleGuard("teacher"), createAssignment) // keep old handler but pass class id

		// User deletion (admin)
		api.DELETE("/users/:id", RoleGuard("admin"), deleteUser)
		// List my submissions (student)
		api.GET("/my-submissions", RoleGuard("student"), listSubs)
		api.DELETE("/classes/:id/students/:sid", RoleGuard("teacher", "admin"), removeStudent)

		api.GET("/students", RoleGuard("teacher", "admin"), listStudents)
		api.GET("/classes/:id", RoleGuard("teacher", "student", "admin"), getClass)

	}

	log.Println("🚀 Server running on http://localhost:8080")
	if err := r.Run(":8080"); err != nil {
		log.Fatalf("could not start server: %v", err)
	}
}
