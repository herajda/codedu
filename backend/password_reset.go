package main

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/smtp"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"golang.org/x/crypto/bcrypt"
)

const resetTokenTTL = time.Hour

var (
	mailer               *mailerConfig
	errInvalidResetToken = errors.New("invalid or expired reset token")
)

type mailerConfig struct {
	host         string
	port         int
	username     string
	password     string
	fromAddress  string
	fromHeader   string
	resetBaseURL string
	appBaseURL   string
}

func InitMailer() {
	_ = godotenv.Load()
	host := strings.TrimSpace(os.Getenv("SMTP_HOST"))
	from := strings.TrimSpace(os.Getenv("SMTP_FROM"))
	if host == "" || from == "" {
		log.Println("⚠️ SMTP not fully configured; password reset emails disabled")
		return
	}
	portStr := strings.TrimSpace(os.Getenv("SMTP_PORT"))
	if portStr == "" {
		portStr = "587"
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		log.Printf("invalid SMTP_PORT: %v", err)
		return
	}
	baseURL := strings.TrimSpace(os.Getenv("PASSWORD_RESET_BASE_URL"))
	if baseURL == "" {
		log.Println("⚠️ PASSWORD_RESET_BASE_URL not set; password reset emails disabled")
		return
	}
	username := os.Getenv("SMTP_USERNAME")
	password := os.Getenv("SMTP_PASSWORD")
	fromName := strings.TrimSpace(os.Getenv("SMTP_FROM_NAME"))
	fromHeader := from
	if fromName != "" {
		fromHeader = fmt.Sprintf("%s <%s>", fromName, from)
	}
	appBase := strings.TrimSpace(os.Getenv("APP_BASE_URL"))
	if appBase == "" {
		appBase = baseURL
	}
	mailer = &mailerConfig{
		host:         host,
		port:         port,
		username:     username,
		password:     password,
		fromAddress:  from,
		fromHeader:   fromHeader,
		resetBaseURL: baseURL,
		appBaseURL:   strings.TrimRight(appBase, "/"),
	}
	log.Printf("📧 SMTP mailer configured for %s:%d", host, port)
}

func (m *mailerConfig) resetURL(token string) string {
	base := strings.TrimRight(m.resetBaseURL, "/")
	if base == "" {
		return ""
	}
	return fmt.Sprintf("%s/reset-password?token=%s", base, url.QueryEscape(token))
}

func (m *mailerConfig) absoluteURL(path string) string {
	base := strings.TrimRight(m.appBaseURL, "/")
	if base == "" {
		return ""
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return base + path
}

func (m *mailerConfig) sendPasswordReset(to, token string) error {
	resetLink := m.resetURL(token)
	if resetLink == "" {
		return errors.New("reset URL not configured")
	}
	subject := "Password reset instructions"
	body := fmt.Sprintf("We received a request to reset your password.\n\nFollow this link to choose a new password (valid for 1 hour):\n%s\n\nIf you did not request a reset, you can safely ignore this email.", resetLink)
	return m.sendPlainText(to, subject, body)
}

func (m *mailerConfig) sendPlainText(to, subject, body string) error {
	msg := buildEmailMessage(m.fromHeader, to, subject, body)
	return m.sendMail([]string{to}, []byte(msg))
}

func (m *mailerConfig) sendMail(recipients []string, msg []byte) error {
	addr := fmt.Sprintf("%s:%d", m.host, m.port)
	var auth smtp.Auth
	if strings.TrimSpace(m.username) != "" {
		auth = smtp.PlainAuth("", m.username, m.password, m.host)
	}

	// Implicit TLS (e.g. port 465)
	if m.port == 465 {
		tlsCfg := &tls.Config{ServerName: m.host}
		conn, err := tls.Dial("tcp", addr, tlsCfg)
		if err != nil {
			return err
		}
		defer conn.Close()

		client, err := smtp.NewClient(conn, m.host)
		if err != nil {
			return err
		}
		defer client.Close()

		if auth != nil {
			if ok, _ := client.Extension("AUTH"); ok {
				if err := client.Auth(auth); err != nil {
					return err
				}
			}
		}

		if err := client.Mail(m.fromAddress); err != nil {
			return err
		}
		for _, rcpt := range recipients {
			if err := client.Rcpt(rcpt); err != nil {
				return err
			}
		}
		wc, err := client.Data()
		if err != nil {
			return err
		}
		if _, err := wc.Write(msg); err != nil {
			_ = wc.Close()
			return err
		}
		if err := wc.Close(); err != nil {
			return err
		}
		return client.Quit()
	}

	// Default (plain or STARTTLS negotiated by SendMail)
	return smtp.SendMail(addr, auth, m.fromAddress, recipients, msg)
}

func buildEmailMessage(from, to, subject, body string) string {
	normalizedBody := strings.ReplaceAll(body, "\n", "\r\n")
	headers := []string{
		fmt.Sprintf("From: %s", from),
		fmt.Sprintf("To: %s", to),
		fmt.Sprintf("Subject: %s", subject),
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
		"Content-Transfer-Encoding: 8bit",
		"",
		normalizedBody,
	}
	return strings.Join(headers, "\r\n") + "\r\n"
}

func requestPasswordReset(c *gin.Context) {
	if mailer == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "password reset email not configured"})
		return
	}
	var req struct {
		Email string `json:"email" binding:"required,email"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("password reset invalid payload: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	log.Printf("password reset request received for email=%s", req.Email)
	user, err := FindUserByEmail(req.Email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			log.Printf("password reset no account for email=%s", req.Email)
			// Hide enumeration details
			c.Status(http.StatusAccepted)
			return
		}
		log.Printf("password reset lookup failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db fail"})
		return
	}
	if user.BkUID != nil {
		log.Printf("password reset ignored for bakalari user id=%s", user.ID)
		c.Status(http.StatusAccepted)
		return
	}
	log.Printf("password reset requested for email=%s user=%s", user.Email, user.ID)
	token, err := createPasswordResetToken(user.ID)
	if err != nil {
		log.Printf("could not create reset token: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not initiate reset"})
		return
	}
	go func(email, token string) {
		if err := mailer.sendPasswordReset(email, token); err != nil {
			log.Printf("could not send reset email to %s: %v", email, err)
		} else {
			log.Printf("password reset email sent to %s", email)
		}
	}(user.Email, token)
	log.Printf("password reset response returning 202 for email=%s", user.Email)
	c.Status(http.StatusAccepted)
}

func completePasswordReset(c *gin.Context) {
	var req struct {
		Token    string `json:"token" binding:"required"`
		Password string `json:"password" binding:"required,min=6"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	userID, err := consumePasswordResetToken(req.Token)
	if err != nil {
		if errors.Is(err, errInvalidResetToken) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid or expired token"})
			return
		}
		log.Printf("could not consume reset token: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not reset password"})
		return
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err := UpdateUserPassword(userID, string(hash)); err != nil {
		log.Printf("could not update password: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db fail"})
		return
	}
	c.Status(http.StatusNoContent)
}

func createPasswordResetToken(userID uuid.UUID) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	hash := hashResetToken(token)
	expires := time.Now().Add(resetTokenTTL)

	tx, err := DB.Beginx()
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(`DELETE FROM password_reset_tokens WHERE user_id=$1 OR expires_at < now()`, userID); err != nil {
		return "", err
	}
	if _, err := tx.Exec(`INSERT INTO password_reset_tokens (token_hash, user_id, expires_at) VALUES ($1,$2,$3)`, hash, userID, expires); err != nil {
		return "", err
	}
	if err := tx.Commit(); err != nil {
		return "", err
	}
	return token, nil
}

func consumePasswordResetToken(token string) (uuid.UUID, error) {
	hash := hashResetToken(token)
	tx, err := DB.Beginx()
	if err != nil {
		return uuid.Nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var userID uuid.UUID
	var bkUID *string
	err = tx.QueryRow(`
        SELECT pr.user_id, u.bk_uid
          FROM password_reset_tokens pr
          JOIN users u ON u.id = pr.user_id
         WHERE pr.token_hash=$1 AND pr.used_at IS NULL AND pr.expires_at > now()
         FOR UPDATE`, hash).Scan(&userID, &bkUID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return uuid.Nil, errInvalidResetToken
		}
		return uuid.Nil, err
	}
	if bkUID != nil {
		return uuid.Nil, errInvalidResetToken
	}
	if _, err := tx.Exec(`UPDATE password_reset_tokens SET used_at=now() WHERE token_hash=$1`, hash); err != nil {
		return uuid.Nil, err
	}
	if err := tx.Commit(); err != nil {
		return uuid.Nil, err
	}
	return userID, nil
}

func hashResetToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
