package main

import (
	"bytes"
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/mail"
	"net/smtp"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/emersion/go-msgauth/dkim"
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
	host            string
	port            int
	username        string
	password        string
	fromAddress     string
	fromHeader      string
	resetBaseURL    string
	appBaseURL      string
	messageIDDomain string
	dkimDomain      string
	dkimSelector    string
	dkimSigner      crypto.Signer
	dkimHeaderKeys  []string
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
	messageDomain := domainFromAddress(from)
	if messageDomain == "" {
		messageDomain = host
	}
	m := &mailerConfig{
		host:            host,
		port:            port,
		username:        username,
		password:        password,
		fromAddress:     from,
		fromHeader:      fromHeader,
		resetBaseURL:    baseURL,
		appBaseURL:      strings.TrimRight(appBase, "/"),
		messageIDDomain: messageDomain,
	}
	m.configureDKIM()
	mailer = m
	log.Printf("📧 SMTP mailer configured for %s:%d", host, port)
}

func (m *mailerConfig) configureDKIM() {
	selector := strings.TrimSpace(os.Getenv("SMTP_DKIM_SELECTOR"))
	rawDomain := strings.TrimSpace(os.Getenv("SMTP_DKIM_DOMAIN"))
	inlineKey := os.Getenv("SMTP_DKIM_PRIVATE_KEY")
	keyPath := strings.TrimSpace(os.Getenv("SMTP_DKIM_PRIVATE_KEY_FILE"))

	if rawDomain == "" {
		rawDomain = m.messageIDDomain
	}

	inlineKey = strings.TrimSpace(inlineKey)
	if selector == "" || rawDomain == "" || (inlineKey == "" && keyPath == "") {
		if selector != "" || rawDomain != "" || inlineKey != "" || keyPath != "" {
			log.Println("⚠️ DKIM configuration incomplete; skipping signature setup")
		}
		return
	}

	var keyData []byte
	var err error
	if inlineKey != "" {
		keyData = []byte(inlineKey)
	} else {
		keyData, err = os.ReadFile(keyPath)
		if err != nil {
			log.Printf("failed to read DKIM key file: %v", err)
			return
		}
	}

	signer, err := parseDKIMPrivateKey(keyData)
	if err != nil {
		log.Printf("invalid DKIM private key: %v", err)
		return
	}

	m.dkimDomain = strings.ToLower(rawDomain)
	m.dkimSelector = selector
	m.dkimSigner = signer
	m.dkimHeaderKeys = defaultDKIMHeaderKeys()
	log.Printf("🔐 DKIM signing enabled (%s.%s)", selector, m.dkimDomain)
}

func defaultDKIMHeaderKeys() []string {
	return []string{
		"From",
		"To",
		"Subject",
		"Date",
		"Message-ID",
		"MIME-Version",
		"Content-Type",
		"Content-Transfer-Encoding",
	}
}

func domainFromAddress(value string) string {
	addr := strings.TrimSpace(value)
	if addr == "" {
		return ""
	}
	if parsed, err := mail.ParseAddress(addr); err == nil && parsed.Address != "" {
		addr = parsed.Address
	}
	if at := strings.LastIndex(addr, "@"); at >= 0 && at < len(addr)-1 {
		return strings.ToLower(addr[at+1:])
	}
	return ""
}

func parseDKIMPrivateKey(pemBytes []byte) (crypto.Signer, error) {
	data := bytes.TrimSpace(pemBytes)
	for len(data) > 0 {
		block, rest := pem.Decode(data)
		if block == nil {
			return nil, errors.New("dkim: failed to decode PEM block")
		}
		var (
			key interface{}
			err error
		)
		switch block.Type {
		case "PRIVATE KEY":
			key, err = x509.ParsePKCS8PrivateKey(block.Bytes)
		case "RSA PRIVATE KEY":
			key, err = x509.ParsePKCS1PrivateKey(block.Bytes)
		case "ED25519 PRIVATE KEY":
			if len(block.Bytes) != ed25519.PrivateKeySize {
				return nil, errors.New("dkim: invalid ed25519 key length")
			}
			key = ed25519.PrivateKey(block.Bytes)
		default:
			// Skip unrelated blocks (e.g., certificates) and continue
			data = rest
			continue
		}
		if err != nil {
			return nil, err
		}
		switch k := key.(type) {
		case *rsa.PrivateKey:
			return k, nil
		case ed25519.PrivateKey:
			return k, nil
		default:
			return nil, fmt.Errorf("dkim: unsupported private key type %T", k)
		}
		data = rest
	}
	return nil, errors.New("dkim: no private key found in PEM data")
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
	return m.sendEmail(to, subject, body, "", nil)
}

func (m *mailerConfig) sendEmail(to, subject, textBody, htmlBody string, headers map[string]string) error {
	msg, err := m.buildEmailMessage(to, subject, textBody, htmlBody, headers)
	if err != nil {
		return err
	}
	recipient := strings.TrimSpace(to)
	if parsed, parseErr := mail.ParseAddress(recipient); parseErr == nil && parsed.Address != "" {
		recipient = parsed.Address
	}
	return m.sendMail([]string{recipient}, msg)
}

func (m *mailerConfig) sendMail(recipients []string, msg []byte) error {
	signedMsg, err := m.signMessage(msg)
	if err != nil {
		return err
	}
	msg = signedMsg
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

func (m *mailerConfig) buildEmailMessage(to, subject, textBody, htmlBody string, headers map[string]string) ([]byte, error) {
	if strings.TrimSpace(to) == "" {
		return nil, errors.New("recipient address is required")
	}

	toHeader := sanitizeHeader(to)
	if parsed, err := mail.ParseAddress(toHeader); err == nil && parsed.Address != "" {
		toHeader = parsed.String()
	}

	fromHeader := sanitizeHeader(m.fromHeader)
	if parsed, err := mail.ParseAddress(fromHeader); err == nil && parsed.Address != "" {
		fromHeader = parsed.String()
	}

	msgDomain := m.messageIDDomain
	if msgDomain == "" {
		msgDomain = domainFromAddress(m.fromAddress)
	}
	if msgDomain == "" {
		msgDomain = strings.ToLower(m.host)
	}

	messageID := fmt.Sprintf("<%s@%s>", strings.ReplaceAll(uuid.NewString(), "-", ""), msgDomain)
	dateHeader := time.Now().UTC().Format(time.RFC1123Z)
	subjectHeader := sanitizeHeader(subject)

	additionalHeaders := map[string]string{}
	if len(headers) > 0 {
		for k, v := range headers {
			if strings.TrimSpace(k) == "" || strings.TrimSpace(v) == "" {
				continue
			}
			additionalHeaders[k] = v
		}
	}

	useHTML := strings.TrimSpace(htmlBody) != ""

	var sb strings.Builder
	sb.Grow(len(textBody) + len(htmlBody) + 512)
	sb.WriteString("Date: ")
	sb.WriteString(dateHeader)
	sb.WriteString("\r\n")
	sb.WriteString("From: ")
	sb.WriteString(fromHeader)
	sb.WriteString("\r\n")
	sb.WriteString("To: ")
	sb.WriteString(toHeader)
	sb.WriteString("\r\n")
	sb.WriteString("Message-ID: ")
	sb.WriteString(messageID)
	sb.WriteString("\r\n")
	sb.WriteString("Subject: ")
	sb.WriteString(subjectHeader)
	sb.WriteString("\r\n")
	if len(additionalHeaders) > 0 {
		keys := make([]string, 0, len(additionalHeaders))
		for k := range additionalHeaders {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			value := sanitizeHeader(additionalHeaders[k])
			if value == "" {
				continue
			}
			sb.WriteString(k)
			sb.WriteString(": ")
			sb.WriteString(value)
			sb.WriteString("\r\n")
		}
	}
	sb.WriteString("MIME-Version: 1.0\r\n")
	if useHTML {
		boundary := strings.ReplaceAll(uuid.NewString(), "-", "")
		sb.WriteString("Content-Type: multipart/alternative; boundary=\"")
		sb.WriteString(boundary)
		sb.WriteString("\"\r\n\r\n")
		writeBodyPart(&sb, boundary, "text/plain; charset=UTF-8", textBody)
		writeBodyPart(&sb, boundary, "text/html; charset=UTF-8", htmlBody)
		sb.WriteString("--")
		sb.WriteString(boundary)
		sb.WriteString("--\r\n")
	} else {
		sb.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
		sb.WriteString("Content-Transfer-Encoding: 8bit\r\n\r\n")
		sb.WriteString(normalizeBody(textBody))
	}

	return []byte(sb.String()), nil
}

func writeBodyPart(sb *strings.Builder, boundary, contentType, body string) {
	sb.WriteString("--")
	sb.WriteString(boundary)
	sb.WriteString("\r\n")
	sb.WriteString("Content-Type: ")
	sb.WriteString(contentType)
	sb.WriteString("\r\n")
	sb.WriteString("Content-Transfer-Encoding: 8bit\r\n\r\n")
	sb.WriteString(normalizeBody(body))
}

func (m *mailerConfig) signMessage(msg []byte) ([]byte, error) {
	if m == nil || m.dkimSigner == nil {
		return msg, nil
	}
	options := &dkim.SignOptions{
		Domain:                 m.dkimDomain,
		Selector:               m.dkimSelector,
		Signer:                 m.dkimSigner,
		HeaderCanonicalization: dkim.CanonicalizationRelaxed,
		BodyCanonicalization:   dkim.CanonicalizationRelaxed,
	}
	if len(m.dkimHeaderKeys) > 0 {
		options.HeaderKeys = m.dkimHeaderKeys
	}
	var signed bytes.Buffer
	if err := dkim.Sign(&signed, bytes.NewReader(msg), options); err != nil {
		return nil, err
	}
	return signed.Bytes(), nil
}

func normalizeBody(body string) string {
	if body == "" {
		return "\r\n"
	}
	replaced := strings.ReplaceAll(body, "\r\n", "\n")
	replaced = strings.ReplaceAll(replaced, "\r", "\n")
	if !strings.HasSuffix(replaced, "\n") {
		replaced += "\n"
	}
	return strings.ReplaceAll(replaced, "\n", "\r\n")
}

func sanitizeHeader(value string) string {
	replaced := strings.ReplaceAll(value, "\r", " ")
	replaced = strings.ReplaceAll(replaced, "\n", " ")
	replaced = strings.TrimSpace(replaced)
	return replaced
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
