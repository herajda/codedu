package main

import (
	"fmt"
	htemplate "html/template"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func handleEmailUnsubscribe(c *gin.Context) {
	token := strings.TrimSpace(c.Query("token"))
	if token == "" {
		c.String(http.StatusBadRequest, "Missing unsubscribe token.")
		return
	}

	userID, scope, err := parseUnsubscribeToken(token)
	if err != nil {
		log.Printf("[unsubscribe] invalid token: %v", err)
		c.String(http.StatusBadRequest, "Invalid unsubscribe link.")
		return
	}

	if err := applyUnsubscribePreference(userID, scope); err != nil {
		log.Printf("[unsubscribe] preference update failed: %v", err)
		c.String(http.StatusInternalServerError, "Could not update notification preferences.")
		return
	}

	if c.Request.Method == http.MethodPost {
		c.Status(http.StatusNoContent)
		return
	}

	message := unsubscribeSuccessMessage(scope)
	html := fmt.Sprintf(`<!DOCTYPE html><html><body style="font-family:'Helvetica Neue',Arial,sans-serif;padding:32px 24px;line-height:1.6;background:#f5f7fb;">`+
		`<div style="max-width:520px;margin:0 auto;background:#ffffff;padding:32px 28px;border-radius:8px;box-shadow:0 10px 30px rgba(15,23,42,0.08);">`+
		`<h2 style="margin-top:0;color:#111827;">You're unsubscribed</h2>`+
		`<p style="margin-bottom:16px;color:#1f2933;">%s</p>`+
		`<p style="margin-bottom:0;color:#4b5563;">You can update your email preferences any time in your CodEdu settings.</p>`+
		`</div></body></html>`, htemplate.HTMLEscapeString(message))

	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
}

func applyUnsubscribePreference(userID uuid.UUID, scope string) error {
	switch scope {
	case unsubscribeScopeAlerts:
		_, err := DB.Exec(`UPDATE users SET email_notifications=FALSE WHERE id=$1`, userID)
		return err
	case unsubscribeScopeDigest:
		_, err := DB.Exec(`UPDATE users SET email_message_digest=FALSE WHERE id=$1`, userID)
		return err
	default:
		return fmt.Errorf("unknown unsubscribe scope: %s", scope)
	}
}

func unsubscribeSuccessMessage(scope string) string {
	switch scope {
	case unsubscribeScopeAlerts:
		return "You'll no longer receive assignment alerts by email."
	case unsubscribeScopeDigest:
		return "You'll no longer receive the daily message digest emails."
	default:
		return "Your email notification preferences have been updated."
	}
}
