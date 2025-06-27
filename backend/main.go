// backend/main.go
package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"

	"golang.org/x/crypto/bcrypt"

	"github.com/gin-gonic/gin"
)

// seed a single admin so you’re never locked out
func ensureAdmin() {
	email := os.Getenv("ADMIN_EMAIL")
	if email == "" {
		email = "admin@example.com"
	}
	password := os.Getenv("ADMIN_PASSWORD")
	if password == "" {
		password = "admin123"
	}
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
	// 1) Init DB and auth
	InitDB()
	InitAuth()
	ensureAdmin()
	StartWorker(2)

	// 2) Router
	r := gin.Default()

	// 3) Public
	r.POST("/api/register", Register)
	r.POST("/api/login", Login)
	r.POST("/api/login-bakalari", LoginBakalari)

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
		api.PUT("/assignments/:id/publish", RoleGuard("teacher", "admin"), publishAssignment)
		api.POST("/assignments/:id/tests", RoleGuard("teacher", "admin"), createTestCase)
		api.DELETE("/tests/:id", RoleGuard("teacher", "admin"), deleteTestCase)
		api.POST("/assignments/:id/submissions", RoleGuard("student"), createSubmission)
		api.GET("/submissions/:id", RoleGuard("student", "teacher", "admin"), getSubmission)
		// TEACHER / STUDENT common
		api.GET("/classes", RoleGuard("teacher", "student"), myClasses)
		api.POST("/classes/:id/students", RoleGuard("teacher", "admin"), addStudents)
		api.POST("/bakalari/atoms", RoleGuard("teacher"), bakalariAtoms)
		api.POST("/classes/:id/import-bakalari", RoleGuard("teacher"), importBakalariStudents)
		api.GET("/classes/all", RoleGuard("admin"), listAllClasses) // new

		// ADMIN → add teacher
		api.POST("/teachers", RoleGuard("admin"), createTeacher)
		api.GET("/users", RoleGuard("admin"), listUsers)               // new
		api.PUT("/users/:id/role", RoleGuard("admin"), updateUserRole) // new

		// TEACHER only
		api.POST("/classes", RoleGuard("teacher"), createClass)

		// Assignments now tied to class
		api.POST("/classes/:id/assignments", RoleGuard("teacher", "admin"), createAssignment)

		// User deletion (admin)
		api.DELETE("/users/:id", RoleGuard("admin"), deleteUser)
		// List my submissions (student)
		api.GET("/my-submissions", RoleGuard("student"), listSubs)
		api.DELETE("/classes/:id/students/:sid", RoleGuard("teacher", "admin"), removeStudent)

		api.GET("/students", RoleGuard("teacher", "admin"), listStudents)
		api.GET("/classes/:id", RoleGuard("teacher", "student", "admin"), getClass)

	}

	// 5) Frontend
	buildPath := filepath.Join("..", "frontend", "build")

	// serve built assets without conflicting with /api routes
	r.Static("/_app", filepath.Join(buildPath, "_app"))
	r.StaticFile("/favicon.png", filepath.Join(buildPath, "favicon.png"))

	// send index.html for all other routes so SvelteKit can handle routing
	r.NoRoute(func(c *gin.Context) {
		c.File(filepath.Join(buildPath, "index.html"))
	})

	log.Println("🚀 Server running on http://localhost:22946")
	if err := r.Run(":22946"); err != nil {
		log.Fatalf("could not start server: %v", err)
	}
}
