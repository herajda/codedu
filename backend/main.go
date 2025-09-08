// backend/main.go
package main

import (
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

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
	if password == "" || password == "admin123" {
		log.Fatal("ADMIN_PASSWORD must be set to a non-default value")
	}

	hashed := clientHash(password)
	hash, _ := bcrypt.GenerateFromPassword([]byte(hashed), bcrypt.DefaultCost)

	if _, err := DB.Exec(`
            INSERT INTO users (email, password_hash, role)
            VALUES ($1,$2,'admin')
            ON CONFLICT (email) DO UPDATE SET password_hash = EXCLUDED.password_hash, role='admin'`,
		email, hash); err != nil {
		log.Fatalf("could not ensure admin: %v", err)
	}
	log.Printf("👑  Admin ensured → %s", email)
}

// Default avatar catalog served by frontend under /avatars
var defaultAvatars = []string{
	"/avatars/a1.svg",
	"/avatars/a2.svg",
	"/avatars/a3.svg",
	"/avatars/a4.svg",
	"/avatars/a5.svg",
	"/avatars/a6.svg",
	"/avatars/a7.svg",
	"/avatars/a8.svg",
	"/avatars/a9.svg",
	"/avatars/a10.svg",
	"/avatars/a11.svg",
	"/avatars/a12.svg",
	"/avatars/a13.svg",
	"/avatars/a14.svg",
	"/avatars/a15.svg",
	"/avatars/a16.svg",
	"/avatars/a17.svg",
	"/avatars/a18.svg",
	"/avatars/a19.svg",
	"/avatars/a20.svg",
	"/avatars/a21.svg",
	"/avatars/a22.svg",
	"/avatars/a23.svg",
	"/avatars/a24.svg",
	"/avatars/a25.svg",
	"/avatars/a26.svg",
	"/avatars/a27.svg",
	"/avatars/a28.svg",
	"/avatars/a29.svg",
	"/avatars/a30.svg",
	"/avatars/a31.svg",
	"/avatars/a32.svg",
	"/avatars/a33.svg",
	"/avatars/a34.svg",
	"/avatars/a35.svg",
	"/avatars/a36.svg",
	"/avatars/a37.svg",
	"/avatars/a38.svg",
	"/avatars/a39.svg",
	"/avatars/a40.svg",
	"/avatars/a41.svg",
	"/avatars/a42.svg",
	"/avatars/a43.svg",
	"/avatars/a44.svg",
	"/avatars/a45.svg",
	"/avatars/a46.svg",
	"/avatars/a47.svg",
	"/avatars/a48.svg",
	"/avatars/a49.svg",
	"/avatars/a50.svg",
	"/avatars/a51.svg",
	"/avatars/a52.svg",
	"/avatars/a53.svg",
	"/avatars/a54.svg",
	"/avatars/a55.svg",
	"/avatars/a56.svg",
}

func main() {
    // 1) Init DB and auth
    InitDB()
    InitAuth()
    ensureAdmin()
    // Ensure Teachers' group exists as a special class
    EnsureTeacherGroupExists()
	// Ensure the shared execution root exists with permissive traversal
	ensureExecRoot(execRoot)
	StartWorker(2)
	// seed RNG for avatar assignment
	rand.Seed(time.Now().UnixNano())
	// one-time ensure avatars for existing users
	if err := AssignRandomAvatarsToUsersWithout(defaultAvatars); err != nil {
		log.Printf("could not assign default avatars: %v", err)
	}

	// 2) Router
	r := gin.Default()

	// 3) Public
	r.POST("/api/register", Register)
	r.POST("/api/login", Login)
	r.POST("/api/login-bakalari", LoginBakalari)
	r.POST("/api/refresh", Refresh)
	r.POST("/api/logout", Logout)

	// 4) Protected
	api := r.Group("/api")
	api.Use(JWTAuth())
	{
		// LLM interactive session API
		registerSessionRoutes(api)
		// health-check
		api.GET("/ping", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"msg": "pong"})
		})
		// who am I
		api.GET("/me", func(c *gin.Context) {
			u, err := GetUser(getUserID(c))
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "db fail"})
				return
			}
			if u.Avatar == nil {
				pick := defaultAvatars[rand.Intn(len(defaultAvatars))]
				// best-effort update; ignore error but reflect in response
				_ = UpdateUserProfile(u.ID, nil, &pick, nil)
				u.Avatar = &pick
			}
			c.JSON(http.StatusOK, gin.H{
				"id":     u.ID,
				"role":   u.Role,
				"name":   u.Name,
				"avatar": u.Avatar,
				"bk_uid": u.BkUID,
				"email":  u.Email,
				"theme":  u.Theme,
			})
		})
		// expose default avatars catalog to the frontend
		api.GET("/avatars", func(c *gin.Context) {
			c.JSON(http.StatusOK, defaultAvatars)
		})
		api.PUT("/me", updateProfile)
		api.PUT("/me/password", changePassword)
		api.POST("/me/link-local", linkLocalAccount)

		// Assignments
		api.GET("/assignments", RoleGuard("student", "teacher", "admin"), listAssignments)
		api.POST("/assignments", RoleGuard("teacher", "admin"), createAssignment)
		api.GET("/assignments/:id", RoleGuard("student", "teacher", "admin"), getAssignment)
		api.PUT("/assignments/:id", RoleGuard("teacher", "admin"), updateAssignment)
		api.DELETE("/assignments/:id", RoleGuard("teacher", "admin"), deleteAssignment)
		api.PUT("/assignments/:id/publish", RoleGuard("teacher", "admin"), publishAssignment)
		// allow optional trailing slash for template endpoints
		api.POST("/assignments/:id/template", RoleGuard("teacher", "admin"), uploadTemplate)
		api.POST("/assignments/:id/template/", RoleGuard("teacher", "admin"), uploadTemplate)
		api.GET("/assignments/:id/template", RoleGuard("student", "teacher", "admin"), getTemplate)
		api.GET("/assignments/:id/template/", RoleGuard("student", "teacher", "admin"), getTemplate)
		api.POST("/assignments/:id/tests", RoleGuard("teacher", "admin"), createTestCase)
		api.POST("/assignments/:id/tests/upload", RoleGuard("teacher", "admin"), uploadUnitTests)
		api.POST("/assignments/:id/tests/ai-generate", RoleGuard("teacher", "admin"), generateAITests)
		api.DELETE("/assignments/:id/tests", RoleGuard("teacher", "admin"), deleteAllTestCases)
		api.PUT("/tests/:id", RoleGuard("teacher", "admin"), updateTestCase)
		api.DELETE("/tests/:id", RoleGuard("teacher", "admin"), deleteTestCase)
		api.POST("/assignments/:id/solution-run", RoleGuard("teacher", "admin"), runTeacherSolution)
		api.POST("/assignments/:id/submissions", RoleGuard("student"), createSubmission)
		api.GET("/submissions/:id", RoleGuard("student", "teacher", "admin"), getSubmission)
		api.PUT("/submissions/:id/points", RoleGuard("teacher", "admin"), overrideSubmissionPoints)
		api.PUT("/submissions/:id/accept", RoleGuard("teacher", "admin"), acceptSubmission)
		api.PUT("/submissions/:id/undo-accept", RoleGuard("teacher", "admin"), undoManualAccept)
		// TEACHER / STUDENT / ADMIN common
		api.GET("/classes", RoleGuard("teacher", "student", "admin"), myClasses)
		api.POST("/classes/:id/students", RoleGuard("teacher", "admin"), addStudents)
		api.POST("/classes/:id/import-bakalari", RoleGuard("teacher"), importBakalariStudents)
		api.GET("/classes/all", RoleGuard("admin"), listAllClasses) // new
		// Admin class utilities
		api.POST("/admin/classes", RoleGuard("admin"), adminCreateClass)
		api.PUT("/admin/classes/:id/transfer", RoleGuard("admin"), adminTransferClass)

		// ADMIN → add teacher
		api.POST("/teachers", RoleGuard("admin"), createTeacher)
		api.GET("/users", RoleGuard("admin"), listUsers)               // new
		api.PUT("/users/:id/role", RoleGuard("admin"), updateUserRole) // new

		// TEACHER only
		api.POST("/classes", RoleGuard("teacher"), createClass)
		api.PUT("/classes/:id", RoleGuard("teacher", "admin"), updateClass)
		api.DELETE("/classes/:id", RoleGuard("teacher", "admin"), deleteClass)

        // Assignments now tied to class
        api.POST("/classes/:id/assignments", RoleGuard("teacher", "admin"), createAssignment)
        api.POST("/classes/:id/assignments/import", RoleGuard("teacher", "admin"), importAssignmentToClass)

		// User deletion (admin)
		api.DELETE("/users/:id", RoleGuard("admin"), deleteUser)
		// List my submissions (student)
		api.GET("/my-submissions", RoleGuard("student"), listSubs)
		api.GET("/events", RoleGuard("student", "teacher", "admin"), eventsHandler)
		// Interactive terminal for manual review sessions (teacher/admin only)
		api.GET("/submissions/:id/terminal", RoleGuard("teacher", "admin"), submissionTerminalWS)
		// Simplified run session WS for manual review
		api.GET("/submissions/:id/run", RoleGuard("teacher", "admin"), submissionRunWS)

		// GUI proxy (noVNC static + WebSocket) when a Tkinter GUI is detected
		api.GET("/submissions/:id/gui/*path", RoleGuard("teacher", "admin"), func(c *gin.Context) {
			// Look up active run session and reverse proxy to local noVNC on 127.0.0.1:GuiHostPort
			sidStr := c.Param("id")
			var sid int
			if _, err := fmt.Sscanf(sidStr, "%d", &sid); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
				return
			}
			sessionKey := fmt.Sprintf("sub-%d", sid)
			runSessionsMu.Lock()
			sess := runSessions[sessionKey]
			runSessionsMu.Unlock()
			if sess == nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "no active session"})
				return
			}
			sess.Mu.Lock()
			hostPort := sess.GuiHostPort
			enabled := sess.GuiEnabled
			sess.Mu.Unlock()
			if !enabled || hostPort == 0 {
				c.JSON(http.StatusNotFound, gin.H{"error": "gui not available"})
				return
			}
			// Build a proxy that points to container noVNC and explicitly set the path from the *path param
			target, _ := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", hostPort))
			proxy := httputil.NewSingleHostReverseProxy(target)
			proxy.Director = func(r *http.Request) {
				p := c.Param("path")
				if p == "" {
					p = "/"
				}
				if !strings.HasPrefix(p, "/") {
					p = "/" + p
				}
				r.URL.Scheme = "http"
				r.URL.Host = target.Host
				r.URL.Path = p
				r.URL.RawPath = p
				r.Host = target.Host
			}
			proxy.ErrorHandler = func(rw http.ResponseWriter, r *http.Request, e error) {
				rw.WriteHeader(http.StatusBadGateway)
				msg := "proxy error"
				if e != nil {
					msg += ": " + e.Error()
				}
				_, _ = rw.Write([]byte(msg))
			}
			proxy.ServeHTTP(c.Writer, c.Request)
		})
		api.DELETE("/classes/:id/students/:sid", RoleGuard("teacher", "admin"), removeStudent)

		api.GET("/students", RoleGuard("teacher", "admin"), listStudents)
		api.GET("/classes/:id/progress", RoleGuard("teacher", "admin"), getClassProgress)
		api.GET("/classes/:id", RoleGuard("teacher", "student", "admin"), getClass)

		api.GET("/users/:id", RoleGuard("student", "teacher", "admin"), getUserPublic)
		api.POST("/users/:id/block", RoleGuard("student", "teacher", "admin"), blockUser)
		api.DELETE("/users/:id/block", RoleGuard("student", "teacher", "admin"), unblockUser)
		api.GET("/blocked-users", RoleGuard("student", "teacher", "admin"), listBlockedUsers)

		// Messaging
		api.GET("/user-search", RoleGuard("student", "teacher", "admin"), searchUsers)
		api.GET("/messages", RoleGuard("student", "teacher", "admin"), listConversations)
		api.POST("/messages", RoleGuard("student", "teacher", "admin"), createMessage)
		api.GET("/messages/:id", RoleGuard("student", "teacher", "admin"), listMessages)
		api.PUT("/messages/:id/read", RoleGuard("student", "teacher", "admin"), markMessagesReadHandler)
		api.POST("/messages/:id/star", RoleGuard("student", "teacher", "admin"), starConversation)
		api.DELETE("/messages/:id/star", RoleGuard("student", "teacher", "admin"), unstarConversation)
		api.POST("/messages/:id/archive", RoleGuard("student", "teacher", "admin"), archiveConversation)
		api.DELETE("/messages/:id/archive", RoleGuard("student", "teacher", "admin"), unarchiveConversation)
		api.GET("/messages/events", RoleGuard("student", "teacher", "admin"), messageEventsHandler)
		api.GET("/messages/file/:id", RoleGuard("student", "teacher", "admin"), downloadMessageFile)

		// User presence
		api.POST("/presence", RoleGuard("student", "teacher", "admin"), presenceHandler)
		api.PUT("/presence", RoleGuard("student", "teacher", "admin"), presenceHandler)
		api.DELETE("/presence", RoleGuard("student", "teacher", "admin"), presenceHandler)
		api.GET("/online-users", RoleGuard("student", "teacher", "admin"), onlineUsersHandler)

        // Class forums
        api.GET("/classes/:id/forum", RoleGuard("teacher", "student", "admin"), listForumMessagesHandler)
        api.POST("/classes/:id/forum", RoleGuard("teacher", "student", "admin"), createForumMessageHandler)
        api.GET("/classes/:id/forum/events", RoleGuard("teacher", "student", "admin"), forumEventsHandler)

        // Class file system
        api.GET("/classes/:id/files", RoleGuard("teacher", "student", "admin"), listClassFiles)
        api.GET("/classes/:id/notebooks", RoleGuard("teacher", "student", "admin"), listClassNotebooks)
        api.POST("/classes/:id/files", RoleGuard("teacher", "admin"), uploadClassFile)
		api.GET("/files/:id", RoleGuard("teacher", "student", "admin"), downloadClassFile)
		api.PUT("/files/:id", RoleGuard("teacher", "admin"), renameClassFile)
		api.PUT("/files/:id/content", RoleGuard("teacher", "admin"), updateFileContent)
		api.DELETE("/files/:id", RoleGuard("teacher", "admin"), deleteClassFile)

	}

	// 5) Frontend
	buildPath := filepath.Join("..", "frontend", "build")

	// serve built assets without conflicting with /api routes
	r.Static("/_app", filepath.Join(buildPath, "_app"))
	r.StaticFile("/favicon.png", filepath.Join(buildPath, "favicon.png"))
	// serve avatars catalog
	r.Static("/avatars", filepath.Join(buildPath, "avatars"))

	// send index.html for all other routes so SvelteKit can handle routing
	r.NoRoute(func(c *gin.Context) {
		c.File(filepath.Join(buildPath, "index.html"))
	})

	// Honor PORT env (default 8080) so reverse proxy can route correctly
	port := getenvOr("PORT", "8080")
	log.Printf("🚀 Server running on http://0.0.0.0:%s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("could not start server: %v", err)
	}
}
