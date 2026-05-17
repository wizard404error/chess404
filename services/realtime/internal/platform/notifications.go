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

	"github.com/chess404/realtime/internal/contracts"
)

var ErrInvalidAccountNotification = errors.New("invalid account notification")
var ErrAccountNotificationNotFound = errors.New("account notification not found")
var ErrUnauthorizedAccountNotification = errors.New("unauthorized account notification")

const (
	AccountNotificationKindFriendRequestReceived   = "friend_request_received"
	AccountNotificationKindFriendRequestAccepted   = "friend_request_accepted"
	AccountNotificationKindDirectChallengeReceived = "direct_challenge_received"
	AccountNotificationKindDirectChallengeAccepted = "direct_challenge_accepted"
	AccountNotificationKindDirectChallengeDeclined = "direct_challenge_declined"
	AccountNotificationKindDirectChallengeCanceled = "direct_challenge_cancelled"
)

type AccountNotification struct {
	NotificationID  string                `json:"notificationId"`
	AccountID       string                `json:"accountId"`
	ActorAccountID  string                `json:"actorAccountId"`
	Kind            string                `json:"kind"`
	FriendRequestID string                `json:"friendRequestId,omitempty"`
	ChallengeID     string                `json:"challengeId,omitempty"`
	MatchID         string                `json:"matchId,omitempty"`
	ModeID          contracts.MatchModeID `json:"modeId,omitempty"`
	ChallengerSeat  string                `json:"challengerSeat,omitempty"`
	CreatedAt       time.Time             `json:"createdAt"`
	UpdatedAt       time.Time             `json:"updatedAt"`
	ReadAt          *time.Time            `json:"readAt,omitempty"`
}

type AccountNotificationOptions struct {
	FriendRequestID string
	ChallengeID     string
	MatchID         string
	ModeID          contracts.MatchModeID
	ChallengerSeat  string
}

type AccountNotificationOverview struct {
	Notifications []AccountNotification `json:"notifications"`
	UnreadCount   int                   `json:"unreadCount"`
}

type AccountNotificationEvent struct {
	EventID        string    `json:"eventId"`
	AccountID      string    `json:"accountId"`
	Kind           string    `json:"kind"`
	NotificationID string    `json:"notificationId,omitempty"`
	UnreadCount    int       `json:"unreadCount"`
	OccurredAt     time.Time `json:"occurredAt"`
}

type AccountNotificationStoreStats struct {
	NotificationCount int `json:"notificationCount"`
	UnreadCount       int `json:"unreadCount"`
}

type notificationPersistence interface {
	backend() string
	load() (map[string]AccountNotification, error)
	persist(map[string]AccountNotification) error
	close() error
}

type AccountNotificationStore struct {
	mu            sync.Mutex
	store         notificationPersistence
	notifications map[string]AccountNotification
	subscribers   map[string]map[chan AccountNotificationEvent]struct{}
}

type notificationStoreFile struct {
	Notifications map[string]AccountNotification `json:"notifications"`
}

type fileNotificationStore struct {
	path string
}

func NewAccountNotificationStore(path string) (*AccountNotificationStore, error) {
	return newAccountNotificationStore(&fileNotificationStore{path: path})
}

func newAccountNotificationStore(store notificationPersistence) (*AccountNotificationStore, error) {
	notifications, err := store.load()
	if err != nil {
		return nil, err
	}
	if notifications == nil {
		notifications = make(map[string]AccountNotification)
	}
	return &AccountNotificationStore{
		store:         store,
		notifications: notifications,
		subscribers:   make(map[string]map[chan AccountNotificationEvent]struct{}),
	}, nil
}

func (s *AccountNotificationStore) Backend() string {
	if s == nil || s.store == nil {
		return "memory"
	}
	return s.store.backend()
}

func (s *AccountNotificationStore) Close() error {
	if s == nil || s.store == nil {
		return nil
	}
	return s.store.close()
}

func (s *AccountNotificationStore) Stats() AccountNotificationStoreStats {
	s.mu.Lock()
	defer s.mu.Unlock()

	unread := 0
	for _, notification := range s.notifications {
		if notification.ReadAt == nil {
			unread++
		}
	}
	return AccountNotificationStoreStats{
		NotificationCount: len(s.notifications),
		UnreadCount:       unread,
	}
}

func (s *AccountNotificationStore) CreateNotification(accountID, actorAccountID, kind string, options AccountNotificationOptions) (AccountNotification, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	resolvedAccountID := strings.TrimSpace(accountID)
	resolvedActorAccountID := strings.TrimSpace(actorAccountID)
	resolvedKind, ok := normalizeAccountNotificationKind(kind)
	if resolvedAccountID == "" || resolvedActorAccountID == "" || resolvedAccountID == resolvedActorAccountID || !ok {
		return AccountNotification{}, ErrInvalidAccountNotification
	}

	now := time.Now().UTC()
	notification := AccountNotification{
		NotificationID:  "notif_" + randomToken(8),
		AccountID:       resolvedAccountID,
		ActorAccountID:  resolvedActorAccountID,
		Kind:            resolvedKind,
		FriendRequestID: strings.TrimSpace(options.FriendRequestID),
		ChallengeID:     strings.TrimSpace(options.ChallengeID),
		MatchID:         strings.TrimSpace(options.MatchID),
		ModeID:          contracts.NormalizeMatchModeID(string(options.ModeID)),
		ChallengerSeat:  normalizeChallengeSeat(options.ChallengerSeat),
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	s.notifications[notification.NotificationID] = notification
	if err := s.persistLocked(); err != nil {
		return AccountNotification{}, err
	}
	s.publishLocked(resolvedAccountID, AccountNotificationEvent{
		EventID:        "notif_evt_" + randomToken(8),
		AccountID:      resolvedAccountID,
		Kind:           "created",
		NotificationID: notification.NotificationID,
		UnreadCount:    s.unreadCountLocked(resolvedAccountID),
		OccurredAt:     now,
	})
	return notification, nil
}

func (s *AccountNotificationStore) MarkRead(accountID, notificationID string) (AccountNotification, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	resolvedAccountID := strings.TrimSpace(accountID)
	resolvedNotificationID := strings.TrimSpace(notificationID)
	if resolvedAccountID == "" || resolvedNotificationID == "" {
		return AccountNotification{}, ErrInvalidAccountNotification
	}
	notification, ok := s.notifications[resolvedNotificationID]
	if !ok {
		return AccountNotification{}, ErrAccountNotificationNotFound
	}
	if notification.AccountID != resolvedAccountID {
		return AccountNotification{}, ErrUnauthorizedAccountNotification
	}
	if notification.ReadAt == nil {
		now := time.Now().UTC()
		notification.ReadAt = &now
		notification.UpdatedAt = now
		s.notifications[resolvedNotificationID] = notification
		if err := s.persistLocked(); err != nil {
			return AccountNotification{}, err
		}
		s.publishLocked(resolvedAccountID, AccountNotificationEvent{
			EventID:        "notif_evt_" + randomToken(8),
			AccountID:      resolvedAccountID,
			Kind:           "read",
			NotificationID: resolvedNotificationID,
			UnreadCount:    s.unreadCountLocked(resolvedAccountID),
			OccurredAt:     now,
		})
	}
	return notification, nil
}

func (s *AccountNotificationStore) MarkAllRead(accountID string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	resolvedAccountID := strings.TrimSpace(accountID)
	if resolvedAccountID == "" {
		return 0, ErrInvalidAccountNotification
	}
	now := time.Now().UTC()
	updated := 0
	for notificationID, notification := range s.notifications {
		if notification.AccountID != resolvedAccountID || notification.ReadAt != nil {
			continue
		}
		readAt := now
		notification.ReadAt = &readAt
		notification.UpdatedAt = now
		s.notifications[notificationID] = notification
		updated++
	}
	if updated == 0 {
		return 0, nil
	}
	if err := s.persistLocked(); err != nil {
		return 0, err
	}
	s.publishLocked(resolvedAccountID, AccountNotificationEvent{
		EventID:     "notif_evt_" + randomToken(8),
		AccountID:   resolvedAccountID,
		Kind:        "bulk_read",
		UnreadCount: s.unreadCountLocked(resolvedAccountID),
		OccurredAt:  now,
	})
	return updated, nil
}

func (s *AccountNotificationStore) PurgePair(accountID, otherAccountID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	resolvedAccountID := strings.TrimSpace(accountID)
	resolvedOtherAccountID := strings.TrimSpace(otherAccountID)
	if resolvedAccountID == "" || resolvedOtherAccountID == "" || resolvedAccountID == resolvedOtherAccountID {
		return ErrInvalidAccountNotification
	}
	for notificationID, notification := range s.notifications {
		if (notification.AccountID == resolvedAccountID && notification.ActorAccountID == resolvedOtherAccountID) ||
			(notification.AccountID == resolvedOtherAccountID && notification.ActorAccountID == resolvedAccountID) {
			delete(s.notifications, notificationID)
		}
	}
	if err := s.persistLocked(); err != nil {
		return err
	}
	now := time.Now().UTC()
	s.publishLocked(resolvedAccountID, AccountNotificationEvent{
		EventID:     "notif_evt_" + randomToken(8),
		AccountID:   resolvedAccountID,
		Kind:        "purged",
		UnreadCount: s.unreadCountLocked(resolvedAccountID),
		OccurredAt:  now,
	})
	s.publishLocked(resolvedOtherAccountID, AccountNotificationEvent{
		EventID:     "notif_evt_" + randomToken(8),
		AccountID:   resolvedOtherAccountID,
		Kind:        "purged",
		UnreadCount: s.unreadCountLocked(resolvedOtherAccountID),
		OccurredAt:  now,
	})
	return nil
}

func (s *AccountNotificationStore) ListOverview(accountID string, limit int) AccountNotificationOverview {
	s.mu.Lock()
	defer s.mu.Unlock()

	resolvedAccountID := strings.TrimSpace(accountID)
	if resolvedAccountID == "" {
		return AccountNotificationOverview{
			Notifications: make([]AccountNotification, 0),
		}
	}
	resolvedLimit := limit
	if resolvedLimit <= 0 {
		resolvedLimit = 48
	}
	items := make([]AccountNotification, 0)
	unread := 0
	for _, notification := range s.notifications {
		if notification.AccountID != resolvedAccountID {
			continue
		}
		if notification.ReadAt == nil {
			unread++
		}
		items = append(items, notification)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].UpdatedAt.Equal(items[j].UpdatedAt) {
			if items[i].CreatedAt.Equal(items[j].CreatedAt) {
				return items[i].NotificationID < items[j].NotificationID
			}
			return items[i].CreatedAt.After(items[j].CreatedAt)
		}
		return items[i].UpdatedAt.After(items[j].UpdatedAt)
	})
	if len(items) > resolvedLimit {
		items = items[:resolvedLimit]
	}
	return AccountNotificationOverview{
		Notifications: items,
		UnreadCount:   unread,
	}
}

func (s *AccountNotificationStore) Subscribe(accountID string, buffer int) (<-chan AccountNotificationEvent, func()) {
	resolvedAccountID := strings.TrimSpace(accountID)
	if buffer <= 0 {
		buffer = 1
	}
	ch := make(chan AccountNotificationEvent, buffer)
	if resolvedAccountID == "" {
		close(ch)
		return ch, func() {}
	}

	s.mu.Lock()
	if s.subscribers == nil {
		s.subscribers = make(map[string]map[chan AccountNotificationEvent]struct{})
	}
	if s.subscribers[resolvedAccountID] == nil {
		s.subscribers[resolvedAccountID] = make(map[chan AccountNotificationEvent]struct{})
	}
	s.subscribers[resolvedAccountID][ch] = struct{}{}
	s.mu.Unlock()

	cancel := func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		subscribers := s.subscribers[resolvedAccountID]
		if subscribers == nil {
			return
		}
		if _, ok := subscribers[ch]; !ok {
			return
		}
		delete(subscribers, ch)
		if len(subscribers) == 0 {
			delete(s.subscribers, resolvedAccountID)
		}
		close(ch)
	}

	return ch, cancel
}

func (s *AccountNotificationStore) persistLocked() error {
	if s.store == nil {
		return nil
	}
	return s.store.persist(s.notifications)
}

func (s *AccountNotificationStore) unreadCountLocked(accountID string) int {
	unread := 0
	for _, notification := range s.notifications {
		if notification.AccountID == accountID && notification.ReadAt == nil {
			unread++
		}
	}
	return unread
}

func (s *AccountNotificationStore) publishLocked(accountID string, event AccountNotificationEvent) {
	subscribers := s.subscribers[accountID]
	if len(subscribers) == 0 {
		return
	}
	for ch := range subscribers {
		select {
		case ch <- event:
		default:
		}
	}
}

func normalizeAccountNotificationKind(kind string) (string, bool) {
	switch strings.TrimSpace(kind) {
	case AccountNotificationKindFriendRequestReceived,
		AccountNotificationKindFriendRequestAccepted,
		AccountNotificationKindDirectChallengeReceived,
		AccountNotificationKindDirectChallengeAccepted,
		AccountNotificationKindDirectChallengeDeclined,
		AccountNotificationKindDirectChallengeCanceled:
		return strings.TrimSpace(kind), true
	default:
		return "", false
	}
}

func (s *fileNotificationStore) backend() string {
	return "file"
}

func (s *fileNotificationStore) load() (map[string]AccountNotification, error) {
	if strings.TrimSpace(s.path) == "" {
		return make(map[string]AccountNotification), nil
	}
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return make(map[string]AccountNotification), nil
	}
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return make(map[string]AccountNotification), nil
	}
	var payload notificationStoreFile
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, err
	}
	if payload.Notifications == nil {
		payload.Notifications = make(map[string]AccountNotification)
	}
	return payload.Notifications, nil
}

func (s *fileNotificationStore) persist(notifications map[string]AccountNotification) error {
	if strings.TrimSpace(s.path) == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	payload := notificationStoreFile{
		Notifications: notifications,
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0o644)
}

func (s *fileNotificationStore) close() error {
	return nil
}
