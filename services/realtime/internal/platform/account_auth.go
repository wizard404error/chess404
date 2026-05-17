package platform

import (
	"errors"
	"regexp"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

var ErrInvalidAccountEmail = errors.New("invalid account email")
var ErrAccountEmailTaken = errors.New("account email already taken")
var ErrInvalidAccountPassword = errors.New("invalid account password")
var ErrUnauthorizedAccountCredentials = errors.New("unauthorized account credentials")

const minAccountPasswordLength = 8
const maxAccountPasswordLength = 128

var accountEmailPattern = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)

func normalizeAccountEmail(email string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(email))
	if normalized == "" || !accountEmailPattern.MatchString(normalized) {
		return "", ErrInvalidAccountEmail
	}
	return normalized, nil
}

func validateAccountPassword(password string) error {
	resolved := strings.TrimSpace(password)
	if len(resolved) < minAccountPasswordLength || len(resolved) > maxAccountPasswordLength {
		return ErrInvalidAccountPassword
	}
	return nil
}

func hashAccountPassword(password string) (string, error) {
	if err := validateAccountPassword(password); err != nil {
		return "", err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func verifyAccountPassword(password, passwordHash string) bool {
	if strings.TrimSpace(passwordHash) == "" || strings.TrimSpace(password) == "" {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password)) == nil
}

func clearAccountPrivateSession(state AccountPrivateState) AccountPrivateState {
	state.SessionToken = ""
	state.ExpiresAt = time.Time{}
	state.Sessions = nil
	return state
}
