package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"time"
)

const InvitationLifetime = 72 * time.Hour

func NewInvitationToken() (string, string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", "", err
	}
	raw := base64.RawURLEncoding.EncodeToString(bytes)
	return raw, HashInvitationToken(raw), nil
}

func HashInvitationToken(token string) string {
	digest := sha256.Sum256([]byte(token))
	return hex.EncodeToString(digest[:])
}
