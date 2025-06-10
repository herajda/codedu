package main

import (
	"encoding/json"
	"net/http"
	"net/url"
	"time"

	"log"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
	"github.com/joho/godotenv"
	"golang.org/x/crypto/bcrypt"
)

var jwtSecret []byte
var bakalariBaseURL string

func InitAuth() {
	_ = godotenv.Load()
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		log.Fatal("JWT_SECRET is not set")
	}
	jwtSecret = []byte(secret)
	bakalariBaseURL = os.Getenv("BAKALARI_BASE_URL")
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
	if err := CreateStudent(req.Email, string(hash), nil, nil, nil); err != nil {
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

// LoginBakalari verifies credentials using Bakaláři API v3. If the user does
// not exist locally, a new account is created with the provided role (student
// or teacher).
func LoginBakalari(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if bakalariBaseURL == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "bakalari base url not configured"})
		return
	}

	form := url.Values{}
	form.Set("client_id", "ANDR")
	form.Set("grant_type", "password")
	form.Set("username", req.Username)
	form.Set("password", req.Password)

	resp, err := http.PostForm(bakalariBaseURL+"/api/login", form)
	if err != nil {
		log.Printf("bakalari login request failed: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	var bkres struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&bkres); err != nil || bkres.AccessToken == "" {
		log.Printf("bakalari login decode error: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	// fetch user info to determine role and class name
	reqUser, _ := http.NewRequest("GET", bakalariBaseURL+"/api/3/user", nil)
	reqUser.Header.Set("Authorization", "Bearer "+bkres.AccessToken)
	userResp, err := http.DefaultClient.Do(reqUser)
	if err != nil {
		log.Printf("bakalari user request failed: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}
	defer userResp.Body.Close()
	if userResp.StatusCode != http.StatusOK {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}
	var userInfo struct {
		UserType string `json:"UserType"`
		UserUID  string `json:"UserUID"`
		Class    struct {
			Abbrev string `json:"Abbrev"`
		} `json:"Class"`
	}
	if err := json.NewDecoder(userResp.Body).Decode(&userInfo); err != nil {
		log.Printf("decode user info failed: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	role := "student"
	if userInfo.UserType == "teacher" {
		role = "teacher"
	}
	var bkClass *string
	if role == "student" && userInfo.Class.Abbrev != "" {
		bkClass = &userInfo.Class.Abbrev
	}
	var bkUID *string
	if uid := userInfo.UserUID; uid != "" {
		if len(uid) >= 5 {
			id5 := uid[len(uid)-5:]
			bkUID = &id5
		} else {
			bkUID = &uid
		}
	}

	user, err := FindUserByEmail(req.Username)
	if err != nil {
		// create new user with specified role
		hash, _ := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if role == "teacher" {
			err = CreateTeacher(req.Username, string(hash), bkUID)
		} else {
			err = CreateStudent(req.Username, string(hash), nil, bkClass, bkUID)
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not create user"})
			return
		}
		user, _ = FindUserByEmail(req.Username)
	} else {
		// update stored bakalari info when changed
		if role == "student" && bkClass != nil && (user.BkClass == nil || *user.BkClass != *bkClass) {
			_, _ = DB.Exec(`UPDATE users SET bk_class=$1 WHERE id=$2`, *bkClass, user.ID)
			user.BkClass = bkClass
		}
		if bkUID != nil && (user.BkUID == nil || *user.BkUID != *bkUID) {
			_, _ = DB.Exec(`UPDATE users SET bk_uid=$1 WHERE id=$2`, *bkUID, user.ID)
			user.BkUID = bkUID
		}
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":  user.ID,
		"role": user.Role,
		"exp":  time.Now().Add(24 * time.Hour).Unix(),
	})
	signed, err := token.SignedString(jwtSecret)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not generate token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"token": signed})
}
