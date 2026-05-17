package platform

import (
	"crypto/subtle"
	"sort"
	"strings"
	"time"
)

const maxAccountSessionsPerAccount = 10

type AccountSessionRecord struct {
	SessionToken string    `json:"sessionToken"`
	ExpiresAt    time.Time `json:"expiresAt"`
	CreatedAt    time.Time `json:"createdAt"`
	LastSeenAt   time.Time `json:"lastSeenAt"`
}

type AccountSessionOverview struct {
	Account  AccountProfile         `json:"account"`
	Sessions []AccountSessionRecord `json:"sessions"`
}

func normalizeAccountPrivateState(state AccountPrivateState) AccountPrivateState {
	state.SessionToken = strings.TrimSpace(state.SessionToken)
	state.Email = strings.TrimSpace(state.Email)
	state.PasswordHash = strings.TrimSpace(state.PasswordHash)
	state.EmailVerifiedAt = state.EmailVerifiedAt.UTC()
	state.EmailVerifications = normalizeAccountEmailVerificationRecords(state.EmailVerifications)
	state.PasswordResets = normalizeAccountPasswordResetRecords(state.PasswordResets)

	recordsByToken := make(map[string]AccountSessionRecord, len(state.Sessions)+1)
	for _, record := range state.Sessions {
		normalized, ok := normalizeAccountSessionRecord(record)
		if !ok {
			continue
		}
		existing, exists := recordsByToken[normalized.SessionToken]
		if !exists || normalized.LastSeenAt.After(existing.LastSeenAt) {
			recordsByToken[normalized.SessionToken] = normalized
		}
	}
	if len(recordsByToken) == 0 && state.SessionToken != "" && !state.ExpiresAt.IsZero() {
		legacyRecord, ok := normalizeAccountSessionRecord(AccountSessionRecord{
			SessionToken: state.SessionToken,
			ExpiresAt:    state.ExpiresAt.UTC(),
			CreatedAt:    state.ExpiresAt.Add(-defaultAccountSessionTTL).UTC(),
			LastSeenAt:   state.ExpiresAt.Add(-defaultAccountSessionTTL).UTC(),
		})
		if ok {
			recordsByToken[legacyRecord.SessionToken] = legacyRecord
		}
	}

	state.Sessions = make([]AccountSessionRecord, 0, len(recordsByToken))
	for _, record := range recordsByToken {
		state.Sessions = append(state.Sessions, record)
	}
	state.Sessions = trimAccountSessionRecords(state.Sessions)
	state.SessionToken = ""
	state.ExpiresAt = time.Time{}
	return state
}

func accountSessionTokenMatches(resolvedToken, requestedToken string) bool {
	left := strings.TrimSpace(resolvedToken)
	right := strings.TrimSpace(requestedToken)
	if left == "" || right == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(left), []byte(right)) == 1
}

func normalizeAccountSessionRecord(record AccountSessionRecord) (AccountSessionRecord, bool) {
	record.SessionToken = strings.TrimSpace(record.SessionToken)
	if record.SessionToken == "" {
		return AccountSessionRecord{}, false
	}
	record.ExpiresAt = record.ExpiresAt.UTC()
	record.CreatedAt = record.CreatedAt.UTC()
	record.LastSeenAt = record.LastSeenAt.UTC()
	if record.CreatedAt.IsZero() {
		switch {
		case !record.LastSeenAt.IsZero():
			record.CreatedAt = record.LastSeenAt
		case !record.ExpiresAt.IsZero():
			record.CreatedAt = record.ExpiresAt.Add(-defaultAccountSessionTTL).UTC()
		}
	}
	if record.LastSeenAt.IsZero() {
		switch {
		case !record.CreatedAt.IsZero():
			record.LastSeenAt = record.CreatedAt
		case !record.ExpiresAt.IsZero():
			record.LastSeenAt = record.ExpiresAt.Add(-defaultAccountSessionTTL).UTC()
		}
	}
	return record, true
}

func trimAccountSessionRecords(records []AccountSessionRecord) []AccountSessionRecord {
	next := make([]AccountSessionRecord, 0, len(records))
	for _, record := range records {
		normalized, ok := normalizeAccountSessionRecord(record)
		if !ok {
			continue
		}
		next = append(next, normalized)
	}
	sort.Slice(next, func(i, j int) bool {
		if next[i].LastSeenAt.Equal(next[j].LastSeenAt) {
			if next[i].CreatedAt.Equal(next[j].CreatedAt) {
				return next[i].SessionToken < next[j].SessionToken
			}
			return next[i].CreatedAt.After(next[j].CreatedAt)
		}
		return next[i].LastSeenAt.After(next[j].LastSeenAt)
	})
	if len(next) > maxAccountSessionsPerAccount {
		next = append([]AccountSessionRecord{}, next[:maxAccountSessionsPerAccount]...)
	}
	return next
}

func issueAccountPrivateSession(state AccountPrivateState, now time.Time) (AccountPrivateState, AccountSessionRecord) {
	state = normalizeAccountPrivateState(state)
	record := AccountSessionRecord{
		SessionToken: "accttok_" + randomToken(18),
		ExpiresAt:    now.Add(defaultAccountSessionTTL).UTC(),
		CreatedAt:    now.UTC(),
		LastSeenAt:   now.UTC(),
	}
	state.Sessions = append(state.Sessions, record)
	state.Sessions = trimAccountSessionRecords(state.Sessions)
	return state, record
}

func renewAccountPrivateSession(state AccountPrivateState, sessionToken string, now time.Time) (AccountPrivateState, AccountSessionRecord, bool) {
	state = normalizeAccountPrivateState(state)
	resolvedToken := strings.TrimSpace(sessionToken)
	if resolvedToken == "" {
		return state, AccountSessionRecord{}, false
	}
	for idx, record := range state.Sessions {
		if !accountSessionTokenMatches(record.SessionToken, resolvedToken) {
			continue
		}
		if record.ExpiresAt.IsZero() || !record.ExpiresAt.After(now) {
			return state, AccountSessionRecord{}, false
		}
		record.LastSeenAt = now.UTC()
		record.ExpiresAt = now.Add(defaultAccountSessionTTL).UTC()
		if record.CreatedAt.IsZero() {
			record.CreatedAt = now.UTC()
		}
		state.Sessions[idx] = record
		state.Sessions = trimAccountSessionRecords(state.Sessions)
		return state, record, true
	}
	return state, AccountSessionRecord{}, false
}

func removeAccountPrivateSession(state AccountPrivateState, sessionToken string) (AccountPrivateState, bool) {
	state = normalizeAccountPrivateState(state)
	resolvedToken := strings.TrimSpace(sessionToken)
	if resolvedToken == "" {
		return state, false
	}
	next := make([]AccountSessionRecord, 0, len(state.Sessions))
	removed := false
	for _, record := range state.Sessions {
		if accountSessionTokenMatches(record.SessionToken, resolvedToken) {
			removed = true
			continue
		}
		next = append(next, record)
	}
	state.Sessions = trimAccountSessionRecords(next)
	return state, removed
}

func retainOnlyAccountPrivateSession(state AccountPrivateState, sessionToken string) (AccountPrivateState, bool) {
	state = normalizeAccountPrivateState(state)
	resolvedToken := strings.TrimSpace(sessionToken)
	if resolvedToken == "" {
		return state, false
	}
	next := make([]AccountSessionRecord, 0, 1)
	found := false
	for _, record := range state.Sessions {
		if accountSessionTokenMatches(record.SessionToken, resolvedToken) {
			next = append(next, record)
			found = true
			break
		}
	}
	state.Sessions = trimAccountSessionRecords(next)
	return state, found
}

func activeAccountSessionRecords(state AccountPrivateState, now time.Time) []AccountSessionRecord {
	state = normalizeAccountPrivateState(state)
	active := make([]AccountSessionRecord, 0, len(state.Sessions))
	for _, record := range state.Sessions {
		if record.ExpiresAt.IsZero() || !record.ExpiresAt.After(now) {
			continue
		}
		active = append(active, record)
	}
	return trimAccountSessionRecords(active)
}

func countActiveAccountSessions(state AccountPrivateState, now time.Time) int {
	return len(activeAccountSessionRecords(state, now))
}

func buildAccountSession(account AccountProfile, privateState AccountPrivateState) AccountSession {
	records := trimAccountSessionRecords(normalizeAccountPrivateState(privateState).Sessions)
	var record AccountSessionRecord
	if len(records) > 0 {
		record = records[0]
	}
	return buildAccountSessionFromRecord(account, record)
}

func buildAccountSessionFromRecord(account AccountProfile, record AccountSessionRecord) AccountSession {
	return AccountSession{
		Account:      account,
		SessionToken: strings.TrimSpace(record.SessionToken),
		ExpiresAt:    record.ExpiresAt.UTC(),
	}
}

func buildAccountSessionOverview(account AccountProfile, sessions []AccountSessionRecord) AccountSessionOverview {
	return AccountSessionOverview{
		Account:  account,
		Sessions: trimAccountSessionRecords(sessions),
	}
}
