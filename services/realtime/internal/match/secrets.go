package match

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

var secretKey = []byte("chess404-player-secret-hash-v1")

func SetSecretHashKey(key string) {
	if key != "" {
		secretKey = []byte(key)
	}
}

func hashSecret(secret string) string {
	if secret == "" {
		return ""
	}
	mac := hmac.New(sha256.New, secretKey)
	mac.Write([]byte(secret))
	return hex.EncodeToString(mac.Sum(nil))
}
