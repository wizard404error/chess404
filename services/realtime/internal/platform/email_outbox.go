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

var ErrInvalidAccountEmailDelivery = errors.New("invalid account email delivery")
var ErrAccountEmailDeliveryNotFound = errors.New("account email delivery not found")
var ErrInvalidAccountEmailDeliveryResult = errors.New("invalid account email delivery result")

const (
	AccountEmailDeliveryKindEmailVerification = "account_email_verification"
	AccountEmailDeliveryKindPasswordReset     = "account_password_reset"

	AccountEmailDeliveryStatusQueued    = "queued"
	AccountEmailDeliveryStatusDelivered = "delivered"
	AccountEmailDeliveryStatusFailed    = "failed"
)

type AccountEmailDelivery struct {
	DeliveryID        string     `json:"deliveryId"`
	AccountID         string     `json:"accountId"`
	Email             string     `json:"email"`
	Kind              string     `json:"kind"`
	Subject           string     `json:"subject"`
	TextBody          string     `json:"textBody"`
	HTMLBody          string     `json:"htmlBody"`
	ActionURL         string     `json:"actionUrl,omitempty"`
	Status            string     `json:"status"`
	Provider          string     `json:"provider,omitempty"`
	ProviderMessageID string     `json:"providerMessageId,omitempty"`
	AttemptCount      int        `json:"attemptCount"`
	LastAttemptAt     *time.Time `json:"lastAttemptAt,omitempty"`
	NextAttemptAt     *time.Time `json:"nextAttemptAt,omitempty"`
	DeliveredAt       *time.Time `json:"deliveredAt,omitempty"`
	FailedAt          *time.Time `json:"failedAt,omitempty"`
	FailureReason     string     `json:"failureReason,omitempty"`
	CreatedAt         time.Time  `json:"createdAt"`
	UpdatedAt         time.Time  `json:"updatedAt"`
}

type AccountEmailDeliveryRequest struct {
	AccountID string
	Email     string
	Kind      string
	Subject   string
	TextBody  string
	HTMLBody  string
	ActionURL string
}

type AccountEmailDeliveryResultRequest struct {
	DeliveryID        string
	Provider          string
	AttemptedAt       time.Time
	Delivered         bool
	ProviderMessageID string
	FailureReason     string
	NextAttemptAt     *time.Time
	TerminalFailure   bool
}

type AccountEmailDeliveryOverview struct {
	Deliveries []AccountEmailDelivery `json:"deliveries"`
}

type AccountEmailDeliveryStoreStats struct {
	DeliveryCount  int `json:"deliveryCount"`
	QueuedCount    int `json:"queuedCount"`
	DeliveredCount int `json:"deliveredCount"`
	FailedCount    int `json:"failedCount"`
}

type emailOutboxPersistence interface {
	backend() string
	load() (map[string]AccountEmailDelivery, error)
	persist(map[string]AccountEmailDelivery) error
	close() error
}

type AccountEmailOutboxStore struct {
	mu         sync.Mutex
	store      emailOutboxPersistence
	deliveries map[string]AccountEmailDelivery
}

type emailOutboxStoreFile struct {
	Deliveries map[string]AccountEmailDelivery `json:"deliveries"`
}

type fileEmailOutboxStore struct {
	path string
}

func NewAccountEmailOutboxStore(path string) (*AccountEmailOutboxStore, error) {
	return newAccountEmailOutboxStore(&fileEmailOutboxStore{path: path})
}

func newAccountEmailOutboxStore(store emailOutboxPersistence) (*AccountEmailOutboxStore, error) {
	deliveries, err := store.load()
	if err != nil {
		return nil, err
	}
	if deliveries == nil {
		deliveries = make(map[string]AccountEmailDelivery)
	}
	for id, delivery := range deliveries {
		deliveries[id] = normalizePersistedAccountEmailDelivery(delivery)
	}
	return &AccountEmailOutboxStore{
		store:      store,
		deliveries: deliveries,
	}, nil
}

func (s *AccountEmailOutboxStore) Backend() string {
	if s == nil || s.store == nil {
		return "memory"
	}
	return s.store.backend()
}

func (s *AccountEmailOutboxStore) Close() error {
	if s == nil || s.store == nil {
		return nil
	}
	return s.store.close()
}

func (s *AccountEmailOutboxStore) Stats() AccountEmailDeliveryStoreStats {
	s.mu.Lock()
	defer s.mu.Unlock()

	stats := AccountEmailDeliveryStoreStats{
		DeliveryCount: len(s.deliveries),
	}
	for _, delivery := range s.deliveries {
		switch delivery.Status {
		case AccountEmailDeliveryStatusDelivered:
			stats.DeliveredCount++
		case AccountEmailDeliveryStatusFailed:
			stats.FailedCount++
		default:
			stats.QueuedCount++
		}
	}
	return stats
}

func (s *AccountEmailOutboxStore) QueueDelivery(request AccountEmailDeliveryRequest) (AccountEmailDelivery, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delivery, err := normalizeAccountEmailDeliveryRequest(request)
	if err != nil {
		return AccountEmailDelivery{}, err
	}
	s.deliveries[delivery.DeliveryID] = delivery
	if err := s.persistLocked(); err != nil {
		return AccountEmailDelivery{}, err
	}
	return delivery, nil
}

func (s *AccountEmailOutboxStore) ListOverview(accountID string, limit int) AccountEmailDeliveryOverview {
	s.mu.Lock()
	defer s.mu.Unlock()

	resolvedAccountID := strings.TrimSpace(accountID)
	if resolvedAccountID == "" {
		return AccountEmailDeliveryOverview{
			Deliveries: make([]AccountEmailDelivery, 0),
		}
	}
	resolvedLimit := normalizeAccountEmailDeliveryLimit(limit, 12)
	items := make([]AccountEmailDelivery, 0)
	for _, delivery := range s.deliveries {
		if delivery.AccountID != resolvedAccountID {
			continue
		}
		items = append(items, delivery)
	}
	sortAccountEmailDeliveriesForOverview(items)
	if len(items) > resolvedLimit {
		items = items[:resolvedLimit]
	}
	return AccountEmailDeliveryOverview{
		Deliveries: items,
	}
}

func (s *AccountEmailOutboxStore) ListPendingDeliveries(limit int, now time.Time) []AccountEmailDelivery {
	s.mu.Lock()
	defer s.mu.Unlock()

	resolvedLimit := normalizeAccountEmailDeliveryLimit(limit, 16)
	items := make([]AccountEmailDelivery, 0)
	for _, delivery := range s.deliveries {
		if !accountEmailDeliveryReadyForAttempt(delivery, now) {
			continue
		}
		items = append(items, delivery)
	}
	sortAccountEmailDeliveriesForAttempt(items)
	if len(items) > resolvedLimit {
		items = items[:resolvedLimit]
	}
	return items
}

func (s *AccountEmailOutboxStore) RecordDeliveryResult(request AccountEmailDeliveryResultRequest) (AccountEmailDelivery, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	resolvedRequest, err := normalizeAccountEmailDeliveryResultRequest(request)
	if err != nil {
		return AccountEmailDelivery{}, err
	}
	delivery, ok := s.deliveries[resolvedRequest.DeliveryID]
	if !ok {
		return AccountEmailDelivery{}, ErrAccountEmailDeliveryNotFound
	}
	delivery = normalizePersistedAccountEmailDelivery(delivery)
	delivery.AttemptCount++
	delivery.UpdatedAt = resolvedRequest.AttemptedAt.UTC()
	delivery.Provider = firstNonEmpty(resolvedRequest.Provider, delivery.Provider)
	lastAttemptAt := resolvedRequest.AttemptedAt.UTC()
	delivery.LastAttemptAt = &lastAttemptAt
	if resolvedRequest.Delivered {
		delivery.Status = AccountEmailDeliveryStatusDelivered
		delivery.ProviderMessageID = strings.TrimSpace(resolvedRequest.ProviderMessageID)
		delivery.FailureReason = ""
		delivery.NextAttemptAt = nil
		delivery.FailedAt = nil
		deliveredAt := resolvedRequest.AttemptedAt.UTC()
		delivery.DeliveredAt = &deliveredAt
	} else if resolvedRequest.TerminalFailure {
		delivery.Status = AccountEmailDeliveryStatusFailed
		delivery.ProviderMessageID = ""
		delivery.FailureReason = strings.TrimSpace(resolvedRequest.FailureReason)
		delivery.NextAttemptAt = nil
		delivery.DeliveredAt = nil
		failedAt := resolvedRequest.AttemptedAt.UTC()
		delivery.FailedAt = &failedAt
	} else {
		delivery.Status = AccountEmailDeliveryStatusQueued
		delivery.ProviderMessageID = ""
		delivery.FailureReason = strings.TrimSpace(resolvedRequest.FailureReason)
		delivery.DeliveredAt = nil
		delivery.FailedAt = nil
		if resolvedRequest.NextAttemptAt != nil {
			nextAttemptAt := resolvedRequest.NextAttemptAt.UTC()
			delivery.NextAttemptAt = &nextAttemptAt
		} else {
			delivery.NextAttemptAt = nil
		}
	}
	s.deliveries[delivery.DeliveryID] = delivery
	if err := s.persistLocked(); err != nil {
		return AccountEmailDelivery{}, err
	}
	return delivery, nil
}

func (s *AccountEmailOutboxStore) persistLocked() error {
	if s.store == nil {
		return nil
	}
	return s.store.persist(s.deliveries)
}

func normalizeAccountEmailDeliveryRequest(request AccountEmailDeliveryRequest) (AccountEmailDelivery, error) {
	resolvedKind, ok := normalizeAccountEmailDeliveryKind(request.Kind)
	if !ok {
		return AccountEmailDelivery{}, ErrInvalidAccountEmailDelivery
	}
	resolvedAccountID := strings.TrimSpace(request.AccountID)
	resolvedEmail := strings.TrimSpace(request.Email)
	resolvedSubject := strings.TrimSpace(request.Subject)
	resolvedTextBody := strings.TrimSpace(request.TextBody)
	resolvedHTMLBody := strings.TrimSpace(request.HTMLBody)
	resolvedActionURL := strings.TrimSpace(request.ActionURL)
	if resolvedAccountID == "" || resolvedEmail == "" || resolvedSubject == "" || resolvedTextBody == "" || resolvedHTMLBody == "" {
		return AccountEmailDelivery{}, ErrInvalidAccountEmailDelivery
	}
	now := time.Now().UTC()
	return AccountEmailDelivery{
		DeliveryID:   "mail_" + randomToken(8),
		AccountID:    resolvedAccountID,
		Email:        resolvedEmail,
		Kind:         resolvedKind,
		Subject:      resolvedSubject,
		TextBody:     resolvedTextBody,
		HTMLBody:     resolvedHTMLBody,
		ActionURL:    resolvedActionURL,
		Status:       AccountEmailDeliveryStatusQueued,
		AttemptCount: 0,
		CreatedAt:    now,
		UpdatedAt:    now,
	}, nil
}

func normalizeAccountEmailDeliveryResultRequest(request AccountEmailDeliveryResultRequest) (AccountEmailDeliveryResultRequest, error) {
	resolved := request
	resolved.DeliveryID = strings.TrimSpace(resolved.DeliveryID)
	resolved.Provider = strings.TrimSpace(resolved.Provider)
	resolved.ProviderMessageID = strings.TrimSpace(resolved.ProviderMessageID)
	resolved.FailureReason = strings.TrimSpace(resolved.FailureReason)
	if resolved.DeliveryID == "" {
		return AccountEmailDeliveryResultRequest{}, ErrInvalidAccountEmailDeliveryResult
	}
	if resolved.AttemptedAt.IsZero() {
		resolved.AttemptedAt = time.Now().UTC()
	} else {
		resolved.AttemptedAt = resolved.AttemptedAt.UTC()
	}
	if resolved.Delivered {
		resolved.NextAttemptAt = nil
		return resolved, nil
	}
	if resolved.TerminalFailure {
		if resolved.FailureReason == "" {
			resolved.FailureReason = "delivery failed"
		}
		resolved.NextAttemptAt = nil
		return resolved, nil
	}
	if resolved.FailureReason == "" {
		resolved.FailureReason = "delivery failed"
	}
	if resolved.NextAttemptAt != nil {
		nextAttemptAt := resolved.NextAttemptAt.UTC()
		resolved.NextAttemptAt = &nextAttemptAt
	}
	return resolved, nil
}

func normalizePersistedAccountEmailDelivery(delivery AccountEmailDelivery) AccountEmailDelivery {
	delivery.DeliveryID = strings.TrimSpace(delivery.DeliveryID)
	delivery.AccountID = strings.TrimSpace(delivery.AccountID)
	delivery.Email = strings.TrimSpace(delivery.Email)
	delivery.Kind, _ = normalizeAccountEmailDeliveryKind(delivery.Kind)
	if delivery.Kind == "" {
		delivery.Kind = AccountEmailDeliveryKindEmailVerification
	}
	delivery.Subject = strings.TrimSpace(delivery.Subject)
	delivery.TextBody = strings.TrimSpace(delivery.TextBody)
	delivery.HTMLBody = strings.TrimSpace(delivery.HTMLBody)
	delivery.ActionURL = strings.TrimSpace(delivery.ActionURL)
	delivery.Provider = strings.TrimSpace(delivery.Provider)
	delivery.ProviderMessageID = strings.TrimSpace(delivery.ProviderMessageID)
	delivery.FailureReason = strings.TrimSpace(delivery.FailureReason)
	delivery.CreatedAt = normalizeAccountEmailOptionalTime(delivery.CreatedAt, delivery.UpdatedAt, time.Now().UTC())
	delivery.UpdatedAt = normalizeAccountEmailOptionalTime(delivery.UpdatedAt, delivery.CreatedAt, delivery.CreatedAt)
	delivery.LastAttemptAt = normalizeAccountEmailPointerTime(delivery.LastAttemptAt)
	delivery.NextAttemptAt = normalizeAccountEmailPointerTime(delivery.NextAttemptAt)
	delivery.DeliveredAt = normalizeAccountEmailPointerTime(delivery.DeliveredAt)
	delivery.FailedAt = normalizeAccountEmailPointerTime(delivery.FailedAt)
	if delivery.AttemptCount < 0 {
		delivery.AttemptCount = 0
	}
	switch strings.TrimSpace(delivery.Status) {
	case AccountEmailDeliveryStatusDelivered:
		delivery.Status = AccountEmailDeliveryStatusDelivered
	case AccountEmailDeliveryStatusFailed:
		delivery.Status = AccountEmailDeliveryStatusFailed
	default:
		delivery.Status = AccountEmailDeliveryStatusQueued
	}
	if delivery.Status != AccountEmailDeliveryStatusQueued {
		delivery.NextAttemptAt = nil
	}
	return delivery
}

func normalizeAccountEmailOptionalTime(value time.Time, fallbacks ...time.Time) time.Time {
	if !value.IsZero() {
		return value.UTC()
	}
	for _, fallback := range fallbacks {
		if !fallback.IsZero() {
			return fallback.UTC()
		}
	}
	return time.Now().UTC()
}

func normalizeAccountEmailPointerTime(value *time.Time) *time.Time {
	if value == nil || value.IsZero() {
		return nil
	}
	resolved := value.UTC()
	return &resolved
}

func normalizeAccountEmailDeliveryKind(kind string) (string, bool) {
	switch strings.TrimSpace(kind) {
	case AccountEmailDeliveryKindEmailVerification, AccountEmailDeliveryKindPasswordReset:
		return strings.TrimSpace(kind), true
	default:
		return "", false
	}
}

func normalizeAccountEmailDeliveryLimit(limit int, fallback int) int {
	if limit > 0 {
		return limit
	}
	if fallback > 0 {
		return fallback
	}
	return 12
}

func accountEmailDeliveryReadyForAttempt(delivery AccountEmailDelivery, now time.Time) bool {
	delivery = normalizePersistedAccountEmailDelivery(delivery)
	if delivery.Status != AccountEmailDeliveryStatusQueued {
		return false
	}
	if delivery.NextAttemptAt == nil {
		return true
	}
	return !delivery.NextAttemptAt.After(now.UTC())
}

func sortAccountEmailDeliveriesForOverview(items []AccountEmailDelivery) {
	sort.Slice(items, func(i, j int) bool {
		if items[i].UpdatedAt.Equal(items[j].UpdatedAt) {
			if items[i].CreatedAt.Equal(items[j].CreatedAt) {
				return items[i].DeliveryID < items[j].DeliveryID
			}
			return items[i].CreatedAt.After(items[j].CreatedAt)
		}
		return items[i].UpdatedAt.After(items[j].UpdatedAt)
	})
}

func sortAccountEmailDeliveriesForAttempt(items []AccountEmailDelivery) {
	sort.Slice(items, func(i, j int) bool {
		leftReadyAt := items[i].CreatedAt
		if items[i].NextAttemptAt != nil {
			leftReadyAt = items[i].NextAttemptAt.UTC()
		}
		rightReadyAt := items[j].CreatedAt
		if items[j].NextAttemptAt != nil {
			rightReadyAt = items[j].NextAttemptAt.UTC()
		}
		if leftReadyAt.Equal(rightReadyAt) {
			if items[i].CreatedAt.Equal(items[j].CreatedAt) {
				return items[i].DeliveryID < items[j].DeliveryID
			}
			return items[i].CreatedAt.Before(items[j].CreatedAt)
		}
		return leftReadyAt.Before(rightReadyAt)
	})
}

func (s *fileEmailOutboxStore) backend() string {
	return "file"
}

func (s *fileEmailOutboxStore) load() (map[string]AccountEmailDelivery, error) {
	if strings.TrimSpace(s.path) == "" {
		return make(map[string]AccountEmailDelivery), nil
	}
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return make(map[string]AccountEmailDelivery), nil
	}
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return make(map[string]AccountEmailDelivery), nil
	}
	var payload emailOutboxStoreFile
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, err
	}
	if payload.Deliveries == nil {
		payload.Deliveries = make(map[string]AccountEmailDelivery)
	}
	return payload.Deliveries, nil
}

func (s *fileEmailOutboxStore) persist(deliveries map[string]AccountEmailDelivery) error {
	if strings.TrimSpace(s.path) == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	payload := emailOutboxStoreFile{
		Deliveries: deliveries,
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0o644)
}

func (s *fileEmailOutboxStore) close() error {
	return nil
}

var _ AccountEmailOutboxDirectory = (*AccountEmailOutboxStore)(nil)
