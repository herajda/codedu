package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"golang.org/x/crypto/bcrypt"
)

var jwtSecret []byte

const (
	accessTokenTTL  = 15 * time.Minute
	refreshTokenTTL = 30 * 24 * time.Hour
)

func InitAuth() {
	_ = godotenv.Load()
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		log.Fatal("JWT_SECRET is not set")
	}
	jwtSecret = []byte(secret)
}

// clientHash replicates the SHA-256 hashing performed by the frontend before
// submitting passwords. The resulting hex string is then hashed with bcrypt
// for storage.
func clientHash(pw string) string {
	sum := sha256.Sum256([]byte(pw))
	return hex.EncodeToString(sum[:])
}

// issueTokens creates a short lived access token and a long lived refresh token
// for the given user ID and role.
func issueTokens(uid uuid.UUID, role string) (string, string, error) {
	access := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":  uid.String(),
		"role": role,
		"exp":  time.Now().Add(accessTokenTTL).Unix(),
	})
	accessStr, err := access.SignedString(jwtSecret)
	if err != nil {
		return "", "", err
	}

	refresh := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":     uid.String(),
		"role":    role,
		"exp":     time.Now().Add(refreshTokenTTL).Unix(),
		"refresh": true,
	})
	refreshStr, err := refresh.SignedString(jwtSecret)
	if err != nil {
		return "", "", err
	}
	return accessStr, refreshStr, nil
}

// isSecure determines if we should set the Secure flag on cookies. When running
// behind a reverse proxy/terminating TLS, the request may not have TLS set, so
// honor X-Forwarded-Proto as well.
func isSecure(c *gin.Context) bool {
	if c.Request.TLS != nil {
		return true
	}
	if strings.EqualFold(c.Request.Header.Get("X-Forwarded-Proto"), "https") {
		return true
	}
	return false
}

func setAuthCookies(c *gin.Context, access, refresh string) {
	secure := isSecure(c)
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     "access_token",
		Value:    access,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(accessTokenTTL.Seconds()),
	})
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     "refresh_token",
		Value:    refresh,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(refreshTokenTTL.Seconds()),
	})
}

func clearAuthCookies(c *gin.Context) {
	secure := isSecure(c)
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     "access_token",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     "refresh_token",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

type registerReq struct {
	FirstName string `json:"firstName" binding:"required"`
	LastName  string `json:"lastName" binding:"required"`
	Email     string `json:"email" binding:"required,email"`
	Password  string `json:"password" binding:"required,min=6"`
}

func Register(c *gin.Context) {
	var req registerReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	first := strings.TrimSpace(req.FirstName)
	last := strings.TrimSpace(req.LastName)
	if first == "" || last == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "first and last name are required"})
		return
	}
	name := strings.TrimSpace(first + " " + last)
	email := strings.TrimSpace(req.Email)
	hash, _ := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err := CreateStudent(email, string(hash), &name, nil, nil); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not create user"})
		return
	}
	c.Status(http.StatusCreated)
}

type loginReq struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

func Login(c *gin.Context) {
	// 1️⃣ Parse & validate the incoming JSON
	log.Println("[Login] 🔑 handler invoked")
	var req loginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("[Login] JSON bind error: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	log.Printf("[Login] attempt: email=%q", req.Email)

	// 2️⃣ Look up the user by email
	user, err := FindUserByEmail(req.Email)
	if err != nil {
		log.Printf("[Login] user lookup error: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}
	log.Printf("[Login] found user id=%d", user.ID)

	// 3️⃣ Verify the password (the request already contains SHA-256 hash)
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		log.Printf("[Login] password compare error: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	log.Printf("[Login] password OK for user %d", user.ID)
	// 4️⃣ Issue tokens & cookies
	access, refresh, err := issueTokens(user.ID, user.Role)
	if err != nil {
		log.Printf("[Login] token sign error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not generate token"})
		return
	}
	setAuthCookies(c, access, refresh)

	// Mark user as online
	if err := MarkUserOnline(user.ID); err != nil {
		log.Printf("[Login] failed to mark user online: %v", err)
	}

	c.Status(http.StatusNoContent)
}

// LoginBakalari accepts user information obtained from the Bakaláři API on
// the client and creates or updates a local account without handling the
// user's Bakaláři credentials.
func LoginBakalari(c *gin.Context) {
	var req struct {
		UID   string  `json:"uid" binding:"required"`
		Role  string  `json:"role" binding:"required"`
		Class *string `json:"class"`
		Name  *string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// trim UID to last three characters, matching previous behaviour
	uid := req.UID
	if len(uid) > 3 {
		uid = uid[len(uid)-3:]
	}
	bkUID := uid

	role := "student"
	if req.Role == "teacher" {
		role = "teacher"
	}
	bkClass := req.Class

	var user *User
	var err error
	user, err = FindUserByBkUID(bkUID)
	if err != nil || user == nil {
		// create new user with random password placeholder
		randBytes := make([]byte, 16)
		if _, err = rand.Read(randBytes); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "random"})
			return
		}
		tmpPass := hex.EncodeToString(randBytes)
		hash, _ := bcrypt.GenerateFromPassword([]byte(clientHash(tmpPass)), bcrypt.DefaultCost)
		email := bkUID
		if role == "teacher" {
			// Set name if provided
			var nm *string
			if req.Name != nil && strings.TrimSpace(*req.Name) != "" {
				t := strings.TrimSpace(*req.Name)
				nm = &t
			}
			err = CreateTeacher(email, string(hash), nm, &bkUID)
		} else {
			// Set name if provided
			var nm *string
			if req.Name != nil && strings.TrimSpace(*req.Name) != "" {
				t := strings.TrimSpace(*req.Name)
				nm = &t
			}
			err = CreateStudent(email, string(hash), nm, bkClass, &bkUID)
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not create user"})
			return
		}
		user, _ = FindUserByBkUID(bkUID)
	} else if role == "student" {
		if bkClass != nil && (user.BkClass == nil || *user.BkClass != *bkClass) {
			_, _ = DB.Exec(`UPDATE users SET bk_class=$1 WHERE id=$2`, *bkClass, user.ID)
			user.BkClass = bkClass
		}
		// If name missing, update from Bakaláři
		if req.Name != nil {
			n := strings.TrimSpace(*req.Name)
			if n != "" && (user.Name == nil || *user.Name == "") {
				_, _ = DB.Exec(`UPDATE users SET name=$1 WHERE id=$2`, n, user.ID)
				user.Name = &n
			}
		}
	} else if role == "teacher" {
		// If name missing for teacher, update from Bakaláři
		if req.Name != nil {
			n := strings.TrimSpace(*req.Name)
			if n != "" && (user.Name == nil || *user.Name == "") {
				_, _ = DB.Exec(`UPDATE users SET name=$1 WHERE id=$2`, n, user.ID)
				user.Name = &n
			}
		}
	}

	access, refresh, err := issueTokens(user.ID, user.Role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not generate token"})
		return
	}
	setAuthCookies(c, access, refresh)

	// Mark user as online
	if err := MarkUserOnline(user.ID); err != nil {
		log.Printf("[LoginBakalari] failed to mark user online: %v", err)
	}

	c.Status(http.StatusNoContent)
}

// Refresh issues a new pair of tokens using the refresh token cookie.
func Refresh(c *gin.Context) {
	refreshStr, err := c.Cookie("refresh_token")
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing refresh token"})
		return
	}
	token, err := jwt.Parse(refreshStr, func(t *jwt.Token) (interface{}, error) { return jwtSecret, nil })
	if err != nil || !token.Valid {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
		return
	}
	claims := token.Claims.(jwt.MapClaims)
	if claims["refresh"] != true {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
		return
	}
	uid, err := uuid.Parse(claims["sub"].(string))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user id"})
		return
	}
	role := claims["role"].(string)
	access, refresh, err := issueTokens(uid, role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not generate token"})
		return
	}
	setAuthCookies(c, access, refresh)
	c.Status(http.StatusNoContent)
}

func Logout(c *gin.Context) {
	// Get user ID from context before clearing cookies
	userID := getUserID(c)

	clearAuthCookies(c)

	// Mark user as offline
	if userID != uuid.Nil {
		if err := MarkUserOffline(userID); err != nil {
			log.Printf("[Logout] failed to mark user offline: %v", err)
		}
	}

	c.Status(http.StatusNoContent)
}
