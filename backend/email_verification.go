package main

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

const emailVerificationTTL = 48 * time.Hour

var (
	errInvalidVerificationToken = errors.New("invalid or expired verification token")
	errEmailAlreadyInUse        = errors.New("email already registered")
)

func createOrUpdatePendingStudent(email, hash string, name *string) (uuid.UUID, string, error) {
	tx, err := DB.Beginx()
	if err != nil {
		return uuid.Nil, "", err
	}
	defer func() { _ = tx.Rollback() }()

	var (
		userID        uuid.UUID
		emailVerified bool
	)
	err = tx.QueryRow(`SELECT id, email_verified FROM users WHERE email=$1 FOR UPDATE`, email).Scan(&userID, &emailVerified)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		err = tx.QueryRow(`INSERT INTO users (email, password_hash, name, role, email_verified)
                            VALUES ($1,$2,$3,'student',FALSE)
                            RETURNING id`, email, hash, name).Scan(&userID)
		if err != nil {
			return uuid.Nil, "", err
		}
	case err != nil:
		return uuid.Nil, "", err
	default:
		if emailVerified {
			return uuid.Nil, "", errEmailAlreadyInUse
		}
		if _, err := tx.Exec(`UPDATE users
                                 SET password_hash=$1,
                                     name=$2,
                                     role='student',
                                     email_verified=FALSE,
                                     email_verified_at=NULL
                               WHERE id=$3`, hash, name, userID); err != nil {
			return uuid.Nil, "", err
		}
	}

	token, err := issueEmailVerificationTokenTx(tx, userID)
	if err != nil {
		return uuid.Nil, "", err
	}

	if err := tx.Commit(); err != nil {
		return uuid.Nil, "", err
	}
	return userID, token, nil
}

func issueEmailVerificationToken(userID uuid.UUID) (string, error) {
	tx, err := DB.Beginx()
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback() }()

	token, err := issueEmailVerificationTokenTx(tx, userID)
	if err != nil {
		return "", err
	}
	if err := tx.Commit(); err != nil {
		return "", err
	}
	return token, nil
}

func issueEmailVerificationTokenTx(tx *sqlx.Tx, userID uuid.UUID) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	hash := hashVerificationToken(token)
	expires := time.Now().Add(emailVerificationTTL)

	if _, err := tx.Exec(`DELETE FROM email_verification_tokens WHERE user_id=$1 OR expires_at < now()`, userID); err != nil {
		return "", err
	}
	if _, err := tx.Exec(`INSERT INTO email_verification_tokens (token_hash, user_id, expires_at) VALUES ($1,$2,$3)`, hash, userID, expires); err != nil {
		return "", err
	}
	return token, nil
}

func verifyEmailWithToken(token string) (uuid.UUID, error) {
	hash := hashVerificationToken(token)
	tx, err := DB.Beginx()
	if err != nil {
		return uuid.Nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var userID uuid.UUID
	err = tx.QueryRow(`SELECT user_id FROM email_verification_tokens WHERE token_hash=$1 AND used_at IS NULL AND expires_at > now() FOR UPDATE`, hash).Scan(&userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return uuid.Nil, errInvalidVerificationToken
		}
		return uuid.Nil, err
	}

	if _, err := tx.Exec(`UPDATE email_verification_tokens SET used_at=now() WHERE token_hash=$1`, hash); err != nil {
		return uuid.Nil, err
	}
	if _, err := tx.Exec(`UPDATE users SET email_verified=TRUE, email_verified_at=COALESCE(email_verified_at, now()) WHERE id=$1`, userID); err != nil {
		return uuid.Nil, err
	}
	if _, err := tx.Exec(`DELETE FROM email_verification_tokens WHERE user_id=$1 AND used_at IS NULL`, userID); err != nil {
		return uuid.Nil, err
	}

	if err := tx.Commit(); err != nil {
		return uuid.Nil, err
	}
	return userID, nil
}

func hashVerificationToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func verifyEmail(c *gin.Context) {
	var req struct {
		Token string `json:"token" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID, err := verifyEmailWithToken(req.Token)
	if err != nil {
		if errors.Is(err, errInvalidVerificationToken) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid or expired token"})
			return
		}
		log.Printf("email verification failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not verify email"})
		return
	}

	log.Printf("email verified for user %s", userID)
	c.Status(http.StatusNoContent)
}
