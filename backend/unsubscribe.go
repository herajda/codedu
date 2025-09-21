package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"

	"github.com/google/uuid"
)

const (
	unsubscribeScopeAlerts = "alerts"
	unsubscribeScopeDigest = "digest"
)

func createUnsubscribeToken(userID uuid.UUID, scope string) (string, error) {
	if len(jwtSecret) == 0 {
		return "", errors.New("unsubscribe token signer not initialized")
	}
	normalizedScope := strings.ToLower(strings.TrimSpace(scope))
	if !isValidUnsubscribeScope(normalizedScope) {
		return "", errors.New("invalid unsubscribe scope")
	}
	payload := userID.String() + "|" + normalizedScope
	mac := hmac.New(sha256.New, jwtSecret)
	mac.Write([]byte(payload))
	signature := hex.EncodeToString(mac.Sum(nil))
	token := payload + "|" + signature
	return base64.RawURLEncoding.EncodeToString([]byte(token)), nil
}

func parseUnsubscribeToken(token string) (uuid.UUID, string, error) {
	if len(jwtSecret) == 0 {
		return uuid.Nil, "", errors.New("unsubscribe token signer not initialized")
	}
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(token))
	if err != nil {
		return uuid.Nil, "", err
	}
	parts := strings.Split(string(raw), "|")
	if len(parts) != 3 {
		return uuid.Nil, "", errors.New("invalid token format")
	}
	uid, err := uuid.Parse(parts[0])
	if err != nil {
		return uuid.Nil, "", err
	}
	scope := strings.ToLower(parts[1])
	if !isValidUnsubscribeScope(scope) {
		return uuid.Nil, "", errors.New("invalid unsubscribe scope")
	}
	payload := parts[0] + "|" + scope
	expected := make([]byte, hex.DecodedLen(len(parts[2])))
	n, err := hex.Decode(expected, []byte(parts[2]))
	if err != nil {
		return uuid.Nil, "", err
	}
	mac := hmac.New(sha256.New, jwtSecret)
	mac.Write([]byte(payload))
	actual := mac.Sum(nil)
	if !hmac.Equal(actual, expected[:n]) {
		return uuid.Nil, "", errors.New("invalid token signature")
	}
	return uid, scope, nil
}

func isValidUnsubscribeScope(scope string) bool {
	switch scope {
	case unsubscribeScopeAlerts, unsubscribeScopeDigest:
		return true
	}
	return false
}
