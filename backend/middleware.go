package main

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
)

func JWTAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		var tokenStr string
		auth := c.GetHeader("Authorization")
		if strings.HasPrefix(auth, "Bearer ") {
			tokenStr = strings.TrimPrefix(auth, "Bearer ")
		} else if cookie, err := c.Cookie("access_token"); err == nil {
			tokenStr = cookie
		} else {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing token"})
			return
		}
		token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
			return jwtSecret, nil
		})
		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}
		claims := token.Claims.(jwt.MapClaims)
		userIDStr := claims["sub"].(string)
		userID, err := uuid.Parse(userIDStr)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid user id"})
			return
		}
		c.Set("userID", userID)
		c.Set("role", claims["role"].(string))
		c.Next()
	}
}

// getUserID extracts the UUID from the gin context
func getUserID(c *gin.Context) uuid.UUID {
	userID, exists := c.Get("userID")
	if !exists {
		// This should never happen if middleware is working correctly
		return uuid.Nil
	}
	return userID.(uuid.UUID)
}

func RoleGuard(allowed ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		role := c.GetString("role")
		for _, a := range allowed {
			if role == a {
				c.Next()
				return
			}
		}
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden"})
	}
}
