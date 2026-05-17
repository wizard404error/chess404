package platform

import (
	"crypto/subtle"
	"errors"
	"sort"
	"strings"
	"time"
)

var ErrAccountLoginUnavailable = errors.New("account login is not enabled")
var ErrAccountEmailAlreadyVerified = errors.New("account email already verified")
var ErrUnauthorizedAccountEmailVerification = errors.New("unauthorized account email verification")
var ErrAccountEmailNotVerified = errors.New("account email is not verified")
var ErrUnauthorizedAccountPasswordReset = errors.New("unauthorized account password reset")

const accountEmailVerificationTTL = 24 * time.Hour
const accountPasswordResetTTL = 2 * time.Hour
const maxAccountVerificationRecords = 5
const maxAccountPasswordResetRecords = 5

type AccountAuthOverview struct {
	AccountID                string     `json:"accountId"`
	Handle                   string     `json:"handle"`
	Email                    string     `json:"email,omitempty"`
	PasswordLoginEnabled     bool       `json:"passwordLoginEnabled"`
	EmailVerified            bool       `json:"emailVerified"`
	EmailVerifiedAt          *time.Time `json:"emailVerifiedAt,omitempty"`
	PendingEmailVerification bool       `json:"pendingEmailVerification"`
	VerificationExpiresAt    *time.Time `json:"verificationExpiresAt,omitempty"`
}

type AccountEmailVerificationRecord struct {
	Token     string     `json:"token"`
	Email     string     `json:"email"`
	ExpiresAt time.Time  `json:"expiresAt"`
	CreatedAt time.Time  `json:"createdAt"`
	UsedAt    *time.Time `json:"usedAt,omitempty"`
}

type AccountPasswordResetRecord struct {
	Token     string     `json:"token"`
	ExpiresAt time.Time  `json:"expiresAt"`
	CreatedAt time.Time  `json:"createdAt"`
	UsedAt    *time.Time `json:"usedAt,omitempty"`
}

type AccountEmailVerificationChallenge struct {
	AccountID string    `json:"accountId"`
	Email     string    `json:"email"`
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expiresAt"`
	CreatedAt time.Time `json:"createdAt"`
}

type AccountPasswordResetChallenge struct {
	Requested bool      `json:"requested"`
	AccountID string    `json:"accountId,omitempty"`
	Email     string    `json:"email,omitempty"`
	Token     string    `json:"token,omitempty"`
	ExpiresAt time.Time `json:"expiresAt,omitempty"`
	CreatedAt time.Time `json:"createdAt,omitempty"`
}

func buildAccountAuthOverview(account AccountProfile, state AccountPrivateState, now time.Time) AccountAuthOverview {
	state = normalizeAccountPrivateState(state)
	overview := AccountAuthOverview{
		AccountID:            account.AccountID,
		Handle:               account.Handle,
		Email:                strings.TrimSpace(state.Email),
		PasswordLoginEnabled: strings.TrimSpace(state.PasswordHash) != "",
		EmailVerified:        !state.EmailVerifiedAt.IsZero(),
	}
	if !state.EmailVerifiedAt.IsZero() {
		verifiedAt := state.EmailVerifiedAt.UTC()
		overview.EmailVerifiedAt = &verifiedAt
	}
	if record, ok := latestPendingAccountEmailVerification(state, now); ok {
		overview.PendingEmailVerification = true
		expiresAt := record.ExpiresAt.UTC()
		overview.VerificationExpiresAt = &expiresAt
	}
	return overview
}

func normalizeAccountEmailVerificationRecords(records []AccountEmailVerificationRecord) []AccountEmailVerificationRecord {
	normalizedByToken := make(map[string]AccountEmailVerificationRecord, len(records))
	for _, record := range records {
		record.Token = strings.TrimSpace(record.Token)
		record.Email = strings.TrimSpace(record.Email)
		record.ExpiresAt = record.ExpiresAt.UTC()
		record.CreatedAt = record.CreatedAt.UTC()
		if record.UsedAt != nil {
			usedAt := record.UsedAt.UTC()
			record.UsedAt = &usedAt
		}
		if record.Token == "" || record.Email == "" || record.ExpiresAt.IsZero() {
			continue
		}
		if record.CreatedAt.IsZero() {
			record.CreatedAt = record.ExpiresAt.Add(-accountEmailVerificationTTL).UTC()
		}
		existing, exists := normalizedByToken[record.Token]
		if !exists || record.CreatedAt.After(existing.CreatedAt) {
			normalizedByToken[record.Token] = record
		}
	}
	next := make([]AccountEmailVerificationRecord, 0, len(normalizedByToken))
	for _, record := range normalizedByToken {
		next = append(next, record)
	}
	sort.Slice(next, func(i, j int) bool {
		if next[i].CreatedAt.Equal(next[j].CreatedAt) {
			return next[i].Token < next[j].Token
		}
		return next[i].CreatedAt.After(next[j].CreatedAt)
	})
	if len(next) > maxAccountVerificationRecords {
		next = append([]AccountEmailVerificationRecord{}, next[:maxAccountVerificationRecords]...)
	}
	return next
}

func normalizeAccountPasswordResetRecords(records []AccountPasswordResetRecord) []AccountPasswordResetRecord {
	normalizedByToken := make(map[string]AccountPasswordResetRecord, len(records))
	for _, record := range records {
		record.Token = strings.TrimSpace(record.Token)
		record.ExpiresAt = record.ExpiresAt.UTC()
		record.CreatedAt = record.CreatedAt.UTC()
		if record.UsedAt != nil {
			usedAt := record.UsedAt.UTC()
			record.UsedAt = &usedAt
		}
		if record.Token == "" || record.ExpiresAt.IsZero() {
			continue
		}
		if record.CreatedAt.IsZero() {
			record.CreatedAt = record.ExpiresAt.Add(-accountPasswordResetTTL).UTC()
		}
		existing, exists := normalizedByToken[record.Token]
		if !exists || record.CreatedAt.After(existing.CreatedAt) {
			normalizedByToken[record.Token] = record
		}
	}
	next := make([]AccountPasswordResetRecord, 0, len(normalizedByToken))
	for _, record := range normalizedByToken {
		next = append(next, record)
	}
	sort.Slice(next, func(i, j int) bool {
		if next[i].CreatedAt.Equal(next[j].CreatedAt) {
			return next[i].Token < next[j].Token
		}
		return next[i].CreatedAt.After(next[j].CreatedAt)
	})
	if len(next) > maxAccountPasswordResetRecords {
		next = append([]AccountPasswordResetRecord{}, next[:maxAccountPasswordResetRecords]...)
	}
	return next
}

func issueAccountEmailVerification(state AccountPrivateState, email string, now time.Time) (AccountPrivateState, AccountEmailVerificationRecord) {
	state = normalizeAccountPrivateState(state)
	record := AccountEmailVerificationRecord{
		Token:     "acctverify_" + randomToken(18),
		Email:     strings.TrimSpace(email),
		ExpiresAt: now.Add(accountEmailVerificationTTL).UTC(),
		CreatedAt: now.UTC(),
	}
	active := make([]AccountEmailVerificationRecord, 0, len(state.EmailVerifications)+1)
	for _, existing := range state.EmailVerifications {
		if existing.UsedAt != nil {
			continue
		}
		if !existing.ExpiresAt.After(now) {
			continue
		}
		if strings.EqualFold(existing.Email, record.Email) {
			continue
		}
		active = append(active, existing)
	}
	active = append(active, record)
	state.EmailVerifications = normalizeAccountEmailVerificationRecords(active)
	return state, record
}

func issueAccountPasswordReset(state AccountPrivateState, now time.Time) (AccountPrivateState, AccountPasswordResetRecord) {
	state = normalizeAccountPrivateState(state)
	record := AccountPasswordResetRecord{
		Token:     "acctreset_" + randomToken(18),
		ExpiresAt: now.Add(accountPasswordResetTTL).UTC(),
		CreatedAt: now.UTC(),
	}
	active := make([]AccountPasswordResetRecord, 0, len(state.PasswordResets)+1)
	for _, existing := range state.PasswordResets {
		if existing.UsedAt != nil {
			continue
		}
		if !existing.ExpiresAt.After(now) {
			continue
		}
		active = append(active, existing)
	}
	active = append(active, record)
	state.PasswordResets = normalizeAccountPasswordResetRecords(active)
	return state, record
}

func consumeAccountEmailVerification(state AccountPrivateState, token string, now time.Time) (AccountPrivateState, AccountEmailVerificationRecord, bool) {
	state = normalizeAccountPrivateState(state)
	resolvedToken := strings.TrimSpace(token)
	if resolvedToken == "" {
		return state, AccountEmailVerificationRecord{}, false
	}
	for idx, record := range state.EmailVerifications {
		if !accountAuthTokenMatches(record.Token, resolvedToken) {
			continue
		}
		if record.UsedAt != nil || !record.ExpiresAt.After(now) {
			return state, AccountEmailVerificationRecord{}, false
		}
		usedAt := now.UTC()
		record.UsedAt = &usedAt
		state.EmailVerifications[idx] = record
		state.EmailVerifiedAt = usedAt
		state.Email = record.Email
		state.EmailVerifications = normalizeAccountEmailVerificationRecords(state.EmailVerifications)
		return state, record, true
	}
	return state, AccountEmailVerificationRecord{}, false
}

func consumeAccountPasswordReset(state AccountPrivateState, token string, now time.Time) (AccountPrivateState, AccountPasswordResetRecord, bool) {
	state = normalizeAccountPrivateState(state)
	resolvedToken := strings.TrimSpace(token)
	if resolvedToken == "" {
		return state, AccountPasswordResetRecord{}, false
	}
	for idx, record := range state.PasswordResets {
		if !accountAuthTokenMatches(record.Token, resolvedToken) {
			continue
		}
		if record.UsedAt != nil || !record.ExpiresAt.After(now) {
			return state, AccountPasswordResetRecord{}, false
		}
		usedAt := now.UTC()
		record.UsedAt = &usedAt
		state.PasswordResets[idx] = record
		state.PasswordResets = normalizeAccountPasswordResetRecords(state.PasswordResets)
		return state, record, true
	}
	return state, AccountPasswordResetRecord{}, false
}

func latestPendingAccountEmailVerification(state AccountPrivateState, now time.Time) (AccountEmailVerificationRecord, bool) {
	state = normalizeAccountPrivateState(state)
	for _, record := range state.EmailVerifications {
		if record.UsedAt != nil {
			continue
		}
		if !record.ExpiresAt.After(now) {
			continue
		}
		return record, true
	}
	return AccountEmailVerificationRecord{}, false
}

func accountAuthTokenMatches(left, right string) bool {
	resolvedLeft := strings.TrimSpace(left)
	resolvedRight := strings.TrimSpace(right)
	if resolvedLeft == "" || resolvedRight == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(resolvedLeft), []byte(resolvedRight)) == 1
}
