package main

import (
	"net/http"
	"time"

	"log"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
	"github.com/joho/godotenv"
	"golang.org/x/crypto/bcrypt"
)

var jwtSecret []byte

func InitAuth() {
	_ = godotenv.Load()
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		log.Fatal("JWT_SECRET is not set")
	}
	jwtSecret = []byte(secret)
}

type registerReq struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
}

func Register(c *gin.Context) {
	var req registerReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err := CreateStudent(req.Email, string(hash)); err != nil {
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
	log.Printf("[Login] attempt: email=%q password=%q", req.Email, req.Password)

	// 2️⃣ Look up the user by email
	user, err := FindUserByEmail(req.Email)
	if err != nil {
		log.Printf("[Login] user lookup error: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}
	log.Printf("[Login] found user id=%d hash=%q", user.ID, user.PasswordHash)

	// 3️⃣ Verify the password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		log.Printf("[Login] password compare error: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	log.Printf("[Login] password OK for user %d", user.ID)
	// 4️⃣ Generate the token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":  user.ID,
		"role": user.Role,
		"exp":  time.Now().Add(24 * time.Hour).Unix(),
	})
	signed, err := token.SignedString(jwtSecret)
	if err != nil {
		log.Printf("[Login] token sign error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not generate token"})
		return
	}
	log.Printf("[Login] issuing token for user %d", user.ID)

	c.JSON(http.StatusOK, gin.H{"token": signed})
}
