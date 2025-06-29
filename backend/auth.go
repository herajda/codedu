package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
	"github.com/joho/godotenv"
	"golang.org/x/crypto/bcrypt"
)

var jwtSecret []byte
var bakalariBaseURL string

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
	bakalariBaseURL = os.Getenv("BAKALARI_BASE_URL")
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
func issueTokens(uid int, role string) (string, string, error) {
	access := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":  uid,
		"role": role,
		"exp":  time.Now().Add(accessTokenTTL).Unix(),
	})
	accessStr, err := access.SignedString(jwtSecret)
	if err != nil {
		return "", "", err
	}

	refresh := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":     uid,
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

func setAuthCookies(c *gin.Context, access, refresh string) {
	secure := c.Request.TLS != nil
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     "access_token",
		Value:    access,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int(accessTokenTTL.Seconds()),
	})
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     "refresh_token",
		Value:    refresh,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int(refreshTokenTTL.Seconds()),
	})
}

func clearAuthCookies(c *gin.Context) {
	secure := c.Request.TLS != nil
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     "access_token",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	})
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     "refresh_token",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	})
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
	c.Status(http.StatusNoContent)
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
		hash, _ := bcrypt.GenerateFromPassword([]byte(clientHash(req.Password)), bcrypt.DefaultCost)
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

	access, refresh, err := issueTokens(user.ID, user.Role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not generate token"})
		return
	}
	setAuthCookies(c, access, refresh)
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
	uid := int(claims["sub"].(float64))
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
	clearAuthCookies(c)
	c.Status(http.StatusNoContent)
}
