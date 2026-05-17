package platform

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

var ErrInvalidAccountSecurityEvent = errors.New("invalid account security event")

const (
	AccountSecurityEventKindAccountClaimed             = "account_claimed"
	AccountSecurityEventKindPasswordLoginEnabled       = "password_login_enabled"
	AccountSecurityEventKindEmailVerificationRequested = "email_verification_requested"
	AccountSecurityEventKindEmailVerified              = "email_verified"
	AccountSecurityEventKindPasswordLoginSucceeded     = "password_login_succeeded"
	AccountSecurityEventKindPasswordResetRequested     = "password_reset_requested"
	AccountSecurityEventKindPasswordResetCompleted     = "password_reset_completed"
	AccountSecurityEventKindSessionSignedOut           = "session_signed_out"
	AccountSecurityEventKindSessionRevoked             = "session_revoked"
	AccountSecurityEventKindOtherSessionsRevoked       = "other_sessions_revoked"
	AccountSecurityEventKindModeratorReviewRecorded    = "moderator_review_recorded"
	maxAccountSecurityEventsPerAccount                 = 40
)

type AccountSecurityEvent struct {
	EventID   string    `json:"eventId"`
	AccountID string    `json:"accountId"`
	Kind      string    `json:"kind"`
	Detail    string    `json:"detail,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}

type AccountSecurityEventRequest struct {
	AccountID string
	Kind      string
	Detail    string
}

type AccountSecurityEventOverview struct {
	Events []AccountSecurityEvent `json:"events"`
}

type AccountSecurityAuditStats struct {
	EventCount int `json:"eventCount"`
}

type accountSecurityAuditPersistence interface {
	backend() string
	load() (map[string]AccountSecurityEvent, error)
	persist(map[string]AccountSecurityEvent) error
	close() error
}

type AccountSecurityAuditStore struct {
	mu     sync.Mutex
	store  accountSecurityAuditPersistence
	events map[string]AccountSecurityEvent
}

type accountSecurityAuditStoreFile struct {
	Events map[string]AccountSecurityEvent `json:"events"`
}

type fileAccountSecurityAuditStore struct {
	path string
}

func NewAccountSecurityAuditStore(path string) (*AccountSecurityAuditStore, error) {
	return newAccountSecurityAuditStore(&fileAccountSecurityAuditStore{path: path})
}

func newAccountSecurityAuditStore(store accountSecurityAuditPersistence) (*AccountSecurityAuditStore, error) {
	events, err := store.load()
	if err != nil {
		return nil, err
	}
	if events == nil {
		events = make(map[string]AccountSecurityEvent)
	}
	return &AccountSecurityAuditStore{
		store:  store,
		events: events,
	}, nil
}

func (s *AccountSecurityAuditStore) Backend() string {
	if s == nil || s.store == nil {
		return "memory"
	}
	return s.store.backend()
}

func (s *AccountSecurityAuditStore) Close() error {
	if s == nil || s.store == nil {
		return nil
	}
	return s.store.close()
}

func (s *AccountSecurityAuditStore) RecordEvent(request AccountSecurityEventRequest) (AccountSecurityEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	event, err := normalizeAccountSecurityEventRequest(request)
	if err != nil {
		return AccountSecurityEvent{}, err
	}
	s.events[event.EventID] = event
	if err := s.persistLocked(); err != nil {
		return AccountSecurityEvent{}, err
	}
	return event, nil
}

func (s *AccountSecurityAuditStore) ListOverview(accountID string, limit int) AccountSecurityEventOverview {
	s.mu.Lock()
	defer s.mu.Unlock()

	resolvedAccountID := strings.TrimSpace(accountID)
	if resolvedAccountID == "" {
		return AccountSecurityEventOverview{Events: make([]AccountSecurityEvent, 0)}
	}
	resolvedLimit := limit
	if resolvedLimit <= 0 {
		resolvedLimit = 12
	}
	items := make([]AccountSecurityEvent, 0)
	for _, event := range s.events {
		if event.AccountID != resolvedAccountID {
			continue
		}
		items = append(items, event)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].CreatedAt.Equal(items[j].CreatedAt) {
			return items[i].EventID < items[j].EventID
		}
		return items[i].CreatedAt.After(items[j].CreatedAt)
	})
	if len(items) > resolvedLimit {
		items = items[:resolvedLimit]
	}
	return AccountSecurityEventOverview{Events: items}
}

func (s *AccountSecurityAuditStore) Stats() AccountSecurityAuditStats {
	s.mu.Lock()
	defer s.mu.Unlock()
	return AccountSecurityAuditStats{EventCount: len(s.events)}
}

func (s *AccountSecurityAuditStore) persistLocked() error {
	if s.store == nil {
		return nil
	}
	return s.store.persist(s.events)
}

func normalizeAccountSecurityEventRequest(request AccountSecurityEventRequest) (AccountSecurityEvent, error) {
	resolvedAccountID := strings.TrimSpace(request.AccountID)
	resolvedKind, ok := normalizeAccountSecurityEventKind(request.Kind)
	resolvedDetail := strings.TrimSpace(request.Detail)
	if resolvedAccountID == "" || !ok {
		return AccountSecurityEvent{}, ErrInvalidAccountSecurityEvent
	}
	return AccountSecurityEvent{
		EventID:   "sec_" + randomToken(8),
		AccountID: resolvedAccountID,
		Kind:      resolvedKind,
		Detail:    resolvedDetail,
		CreatedAt: time.Now().UTC(),
	}, nil
}

func normalizeAccountSecurityEventKind(kind string) (string, bool) {
	switch strings.TrimSpace(kind) {
	case
		AccountSecurityEventKindAccountClaimed,
		AccountSecurityEventKindPasswordLoginEnabled,
		AccountSecurityEventKindEmailVerificationRequested,
		AccountSecurityEventKindEmailVerified,
		AccountSecurityEventKindPasswordLoginSucceeded,
		AccountSecurityEventKindPasswordResetRequested,
		AccountSecurityEventKindPasswordResetCompleted,
		AccountSecurityEventKindSessionSignedOut,
		AccountSecurityEventKindSessionRevoked,
		AccountSecurityEventKindOtherSessionsRevoked,
		AccountSecurityEventKindModeratorReviewRecorded:
		return strings.TrimSpace(kind), true
	default:
		return "", false
	}
}

func (s *fileAccountSecurityAuditStore) backend() string {
	return "file"
}

func (s *fileAccountSecurityAuditStore) load() (map[string]AccountSecurityEvent, error) {
	if strings.TrimSpace(s.path) == "" {
		return make(map[string]AccountSecurityEvent), nil
	}
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return make(map[string]AccountSecurityEvent), nil
	}
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return make(map[string]AccountSecurityEvent), nil
	}
	var payload accountSecurityAuditStoreFile
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, err
	}
	if payload.Events == nil {
		payload.Events = make(map[string]AccountSecurityEvent)
	}
	return payload.Events, nil
}

func (s *fileAccountSecurityAuditStore) persist(events map[string]AccountSecurityEvent) error {
	if strings.TrimSpace(s.path) == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	payload := accountSecurityAuditStoreFile{Events: events}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0o644)
}

func (s *fileAccountSecurityAuditStore) close() error {
	return nil
}

var _ AccountSecurityAuditDirectory = (*AccountSecurityAuditStore)(nil)
