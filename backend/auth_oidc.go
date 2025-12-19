package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"log"
	"net/http"
	"os"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/microsoft"
)

var (
	oauth2Config oauth2.Config
	oidcVerifier *oidc.IDTokenVerifier
)

func InitOIDC() {
	clientID := os.Getenv("MICROSOFT_CLIENT_ID")
	clientSecret := os.Getenv("MICROSOFT_CLIENT_SECRET")
	redirectURL := os.Getenv("MICROSOFT_REDIRECT_URL")
	tenantID := os.Getenv("MICROSOFT_TENANT_ID")
	if tenantID == "" {
		tenantID = "common"
	}

	if clientID == "" || clientSecret == "" || redirectURL == "" {
		log.Println("Microsoft OIDC not configured (missing vars)")
		return
	}

	// For standard OIDC, we usually hit the discovery endpoint.
	// Microsoft endpoint: https://login.microsoftonline.com/{tenant}/v2.0
	provider, err := oidc.NewProvider(context.Background(), "https://login.microsoftonline.com/"+tenantID+"/v2.0")
	if err != nil {
		log.Printf("Failed to query Microsoft OIDC provider: %v. Login will be unavailable.", err)
		return
	}

	oauth2Config = oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		// Endpoint:     provider.Endpoint(), // Use provider discovery
		Endpoint: microsoft.AzureADEndpoint(tenantID),
		Scopes:   []string{oidc.ScopeOpenID, "profile", "email", "User.Read"},
	}

	// If provider discovery worked, we can use a verifier.
	// Note: For "common" tenant, issuer validation often fails because the issuer in token is the specific tenant UUID.
	// We might need to handle that.
	oidcConfig := &oidc.Config{
		ClientID: clientID,
	}
	if tenantID == "common" {
		oidcConfig.SkipIssuerCheck = true
	}
	oidcVerifier = provider.Verifier(oidcConfig)

	log.Println("Microsoft OIDC initialized")
}

func LoginMicrosoft(c *gin.Context) {
	if oauth2Config.ClientID == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Microsoft login not configured"})
		return
	}

	// Generate state cookie
	b := make([]byte, 16)
	rand.Read(b)
	state := base64.URLEncoding.EncodeToString(b)

	// Set state cookie (short lived)
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     "oauth_state",
		Value:    state,
		MaxAge:   300,
		Path:     "/",
		HttpOnly: true,
		Secure:   isSecure(c),
	})

	http.Redirect(c.Writer, c.Request, oauth2Config.AuthCodeURL(state), http.StatusTemporaryRedirect)
}

func CallbackMicrosoft(c *gin.Context) {
	if oauth2Config.ClientID == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Microsoft login not configured"})
		return
	}

	// Verify state
	stateCookie, err := c.Cookie("oauth_state")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "state cookie missing"})
		return
	}
	if c.Query("state") != stateCookie {
		c.JSON(http.StatusBadRequest, gin.H{"error": "state mismatch"})
		return
	}
	// Delete state cookie
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     "oauth_state",
		Value:    "",
		MaxAge:   -1,
		Path:     "/",
		HttpOnly: true,
		Secure:   isSecure(c),
	})

	// Exchange code
	code := c.Query("code")
	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no code provided"})
		return
	}

	token, err := oauth2Config.Exchange(c.Request.Context(), code)
	if err != nil {
		log.Printf("OIDC exchange failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to exchange token"})
		return
	}

	// Extract ID Token
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "no id_token field in oauth2 token"})
		return
	}

	idToken, err := oidcVerifier.Verify(c.Request.Context(), rawIDToken)
	if err != nil {
		log.Printf("OIDC verify failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to verify ID token"})
		return
	}

	var claims struct {
		Email string `json:"email"`
		Name  string `json:"name"`
		Oid   string `json:"oid"` // Microsoft Object ID
	}
	if err := idToken.Claims(&claims); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to parse claims"})
		return
	}

	// fallback if email is empty (sometimes happens with different scopes)
	if claims.Email == "" {
		// Try to get from 'preferred_username' or similar if needed,
		// but for now let's error or fallback
		var claimsMap map[string]interface{}
		if err := idToken.Claims(&claimsMap); err == nil {
			if v, ok := claimsMap["preferred_username"].(string); ok {
				claims.Email = v
			}
		}
	}

	if claims.Email == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "email not provided by IDP"})
		return
	}

	// Check DB
	// We check by MS OID first
	user, err := FindUserByMsOID(claims.Oid)

	// Determine if user should be teacher
	shouldBeTeacher := false
	if whitelisted, err := IsEmailWhitelisted(claims.Email); err == nil && whitelisted {
		shouldBeTeacher = true
	}

	if err == nil && user != nil {
		// User exists
		// Promote if needed
		if shouldBeTeacher && user.Role != "teacher" && user.Role != "admin" {
			log.Printf("Promoting user %s to teacher (whitelist match)", user.Email)
			DB.Exec("UPDATE users SET role='teacher' WHERE id=$1", user.ID)
			user.Role = "teacher"
		}

		loginUser(c, user)
		return
	}

	// Check by email
	user, err = FindUserByEmail(claims.Email)
	if err == nil && user != nil {
		// Link account
		if err := LinkMsOID(user.ID, claims.Oid); err != nil {
			log.Printf("Failed to link MS OID: %v", err)
		}

		// Promote if needed
		if shouldBeTeacher && user.Role != "teacher" && user.Role != "admin" {
			log.Printf("Promoting user %s to teacher (whitelist match)", user.Email)
			DB.Exec("UPDATE users SET role='teacher' WHERE id=$1", user.ID)
			user.Role = "teacher"
		}

		loginUser(c, user)
		return
	}

	// Create new user
	// Generate random pass
	randBytes := make([]byte, 16)
	rand.Read(randBytes)
	tmpPass := hex.EncodeToString(randBytes)
	hash, _ := bcrypt.GenerateFromPassword([]byte(clientHash(tmpPass)), bcrypt.DefaultCost)

	initialRole := "student"
	if shouldBeTeacher {
		initialRole = "teacher"
	}

	newTaskUser := &User{
		Email:         claims.Email,
		PasswordHash:  string(hash),
		Name:          &claims.Name,
		Role:          initialRole,
		EmailVerified: true, // Trusted from MS
		MsOID:         &claims.Oid,
	}

	// We need a helper to create full user or reuse CreateStudent logic but adapted
	// CreateStudent takes pointers etc.
	// Let's modify models.go to have a proper CreateUser func, or do it raw here.
	// simpler to do raw insert here or create helper.

	err = CreateUserFromOIDC(newTaskUser)
	if err != nil {
		log.Printf("Failed to create OIDC user: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create user"})
		return
	}

	// Fetch again to get ID
	user, err = FindUserByMsOID(claims.Oid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch new user"})
		return
	}

	loginUser(c, user)
}

func loginUser(c *gin.Context, user *User) {
	access, refresh, err := issueTokens(user.ID, user.Role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not generate token"})
		return
	}
	setAuthCookies(c, access, refresh)
	MarkUserOnline(user.ID)
	// Redirect to dashboard or home
	http.Redirect(c.Writer, c.Request, "/dashboard", http.StatusTemporaryRedirect)
}
