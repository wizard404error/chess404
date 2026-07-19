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

var ErrInvalidFriendRequest = errors.New("invalid friend request")
var ErrFriendRequestAlreadyExists = errors.New("friend request already exists")
var ErrAlreadyFriends = errors.New("accounts are already friends")
var ErrFriendRequestNotFound = errors.New("friend request not found")
var ErrUnauthorizedFriendRequest = errors.New("unauthorized friend request")

const (
	FriendRequestStatusPending  = "pending"
	FriendRequestStatusAccepted = "accepted"
	FriendRequestStatusDeclined = "declined"
)

type FriendRequest struct {
	RequestID          string    `json:"requestId"`
	RequesterAccountID string    `json:"requesterAccountId"`
	TargetAccountID    string    `json:"targetAccountId"`
	Status             string    `json:"status"`
	CreatedAt          time.Time `json:"createdAt"`
	UpdatedAt          time.Time `json:"updatedAt"`
}

type Friendship struct {
	FriendshipID  string    `json:"friendshipId"`
	LowAccountID  string    `json:"lowAccountId"`
	HighAccountID string    `json:"highAccountId"`
	CreatedAt     time.Time `json:"createdAt"`
}

type FriendshipOverview struct {
	Friends  []Friendship    `json:"friends"`
	Incoming []FriendRequest `json:"incoming"`
	Outgoing []FriendRequest `json:"outgoing"`
}

type FriendshipStoreStats struct {
	FriendshipCount     int `json:"friendshipCount"`
	PendingRequestCount int `json:"pendingRequestCount"`
}

type friendshipPersistence interface {
	backend() string
	load() (map[string]FriendRequest, map[string]Friendship, error)
	persist(map[string]FriendRequest, map[string]Friendship) error
	close() error
}

type FriendshipStore struct {
	mu          sync.Mutex
	store       friendshipPersistence
	requests    map[string]FriendRequest
	friendships map[string]Friendship
}

type friendshipStoreFile struct {
	Requests    map[string]FriendRequest `json:"requests"`
	Friendships map[string]Friendship    `json:"friendships"`
}

type fileFriendshipStore struct {
	path string
}

func NewFriendshipStore(path string) (*FriendshipStore, error) {
	return NewFriendshipStoreFromDB(&fileFriendshipStore{path: path})
}

func NewFriendshipStoreFromDB(store friendshipPersistence) (*FriendshipStore, error) {
	requests, friendships, err := store.load()
	if err != nil {
		return nil, err
	}
	if requests == nil {
		requests = make(map[string]FriendRequest)
	}
	if friendships == nil {
		friendships = make(map[string]Friendship)
	}
	return &FriendshipStore{
		store:       store,
		requests:    requests,
		friendships: friendships,
	}, nil
}

func (s *FriendshipStore) Backend() string {
	if s == nil || s.store == nil {
		return "memory"
	}
	return s.store.backend()
}

func (s *FriendshipStore) Close() error {
	if s == nil || s.store == nil {
		return nil
	}
	return s.store.close()
}

func (s *FriendshipStore) Stats() FriendshipStoreStats {
	s.mu.Lock()
	defer s.mu.Unlock()

	return FriendshipStoreStats{
		FriendshipCount:     len(s.friendships),
		PendingRequestCount: len(s.requests),
	}
}

func (s *FriendshipStore) SendRequest(requesterAccountID, targetAccountID string) (FriendRequest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	requester := strings.TrimSpace(requesterAccountID)
	target := strings.TrimSpace(targetAccountID)
	low, high, friendshipID, err := normalizeFriendshipPair(requester, target)
	if err != nil {
		return FriendRequest{}, err
	}
	if _, ok := s.friendships[friendshipID]; ok {
		return FriendRequest{}, ErrAlreadyFriends
	}

	now := time.Now().UTC()
	for requestID, request := range s.requests {
		if request.RequesterAccountID == requester && request.TargetAccountID == target {
			return FriendRequest{}, ErrFriendRequestAlreadyExists
		}
		if request.RequesterAccountID == target && request.TargetAccountID == requester {
			delete(s.requests, requestID)
			friendship := Friendship{
				FriendshipID:  friendshipID,
				LowAccountID:  low,
				HighAccountID: high,
				CreatedAt:     now,
			}
			s.friendships[friendshipID] = friendship
			if err := s.persistLocked(); err != nil {
				return FriendRequest{}, err
			}
			return FriendRequest{
				RequestID:          request.RequestID,
				RequesterAccountID: requester,
				TargetAccountID:    target,
				Status:             FriendRequestStatusAccepted,
				CreatedAt:          request.CreatedAt,
				UpdatedAt:          now,
			}, nil
		}
	}

	request := FriendRequest{
		RequestID:          "friendreq_" + randomToken(8),
		RequesterAccountID: requester,
		TargetAccountID:    target,
		Status:             FriendRequestStatusPending,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	s.requests[request.RequestID] = request
	if err := s.persistLocked(); err != nil {
		return FriendRequest{}, err
	}
	return request, nil
}

func (s *FriendshipStore) RespondToRequest(targetAccountID, requestID string, accept bool) (FriendRequest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	resolvedTargetID := strings.TrimSpace(targetAccountID)
	resolvedRequestID := strings.TrimSpace(requestID)
	if resolvedTargetID == "" || resolvedRequestID == "" {
		return FriendRequest{}, ErrInvalidFriendRequest
	}
	request, ok := s.requests[resolvedRequestID]
	if !ok {
		return FriendRequest{}, ErrFriendRequestNotFound
	}
	if request.TargetAccountID != resolvedTargetID {
		return FriendRequest{}, ErrUnauthorizedFriendRequest
	}

	delete(s.requests, resolvedRequestID)
	now := time.Now().UTC()
	request.UpdatedAt = now
	if accept {
		request.Status = FriendRequestStatusAccepted
		low, high, friendshipID, err := normalizeFriendshipPair(request.RequesterAccountID, request.TargetAccountID)
		if err != nil {
			return FriendRequest{}, err
		}
		s.friendships[friendshipID] = Friendship{
			FriendshipID:  friendshipID,
			LowAccountID:  low,
			HighAccountID: high,
			CreatedAt:     now,
		}
	} else {
		request.Status = FriendRequestStatusDeclined
	}
	if err := s.persistLocked(); err != nil {
		return FriendRequest{}, err
	}
	return request, nil
}

func (s *FriendshipStore) RemoveFriend(accountID, friendAccountID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, _, friendshipID, err := normalizeFriendshipPair(accountID, friendAccountID)
	if err != nil {
		return err
	}
	delete(s.friendships, friendshipID)
	for requestID, request := range s.requests {
		if (request.RequesterAccountID == strings.TrimSpace(accountID) && request.TargetAccountID == strings.TrimSpace(friendAccountID)) ||
			(request.RequesterAccountID == strings.TrimSpace(friendAccountID) && request.TargetAccountID == strings.TrimSpace(accountID)) {
			delete(s.requests, requestID)
		}
	}
	return s.persistLocked()
}

func (s *FriendshipStore) ListOverview(accountID string) FriendshipOverview {
	s.mu.Lock()
	defer s.mu.Unlock()

	resolvedAccountID := strings.TrimSpace(accountID)
	if resolvedAccountID == "" {
		return FriendshipOverview{}
	}

	overview := FriendshipOverview{
		Friends:  make([]Friendship, 0),
		Incoming: make([]FriendRequest, 0),
		Outgoing: make([]FriendRequest, 0),
	}
	for _, friendship := range s.friendships {
		if friendship.LowAccountID == resolvedAccountID || friendship.HighAccountID == resolvedAccountID {
			overview.Friends = append(overview.Friends, friendship)
		}
	}
	for _, request := range s.requests {
		switch {
		case request.TargetAccountID == resolvedAccountID:
			overview.Incoming = append(overview.Incoming, request)
		case request.RequesterAccountID == resolvedAccountID:
			overview.Outgoing = append(overview.Outgoing, request)
		}
	}
	sort.Slice(overview.Friends, func(i, j int) bool {
		if overview.Friends[i].CreatedAt.Equal(overview.Friends[j].CreatedAt) {
			return overview.Friends[i].FriendshipID < overview.Friends[j].FriendshipID
		}
		return overview.Friends[i].CreatedAt.After(overview.Friends[j].CreatedAt)
	})
	sort.Slice(overview.Incoming, func(i, j int) bool {
		if overview.Incoming[i].UpdatedAt.Equal(overview.Incoming[j].UpdatedAt) {
			return overview.Incoming[i].RequestID < overview.Incoming[j].RequestID
		}
		return overview.Incoming[i].UpdatedAt.After(overview.Incoming[j].UpdatedAt)
	})
	sort.Slice(overview.Outgoing, func(i, j int) bool {
		if overview.Outgoing[i].UpdatedAt.Equal(overview.Outgoing[j].UpdatedAt) {
			return overview.Outgoing[i].RequestID < overview.Outgoing[j].RequestID
		}
		return overview.Outgoing[i].UpdatedAt.After(overview.Outgoing[j].UpdatedAt)
	})
	return overview
}

func (s *FriendshipStore) AreFriends(accountID, friendAccountID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, _, friendshipID, err := normalizeFriendshipPair(accountID, friendAccountID)
	if err != nil {
		return false
	}
	_, ok := s.friendships[friendshipID]
	return ok
}

func (s *FriendshipStore) persistLocked() error {
	if s.store == nil {
		return nil
	}
	return s.store.persist(s.requests, s.friendships)
}

func (s *fileFriendshipStore) backend() string {
	return "file"
}

func (s *fileFriendshipStore) load() (map[string]FriendRequest, map[string]Friendship, error) {
	if strings.TrimSpace(s.path) == "" {
		return make(map[string]FriendRequest), make(map[string]Friendship), nil
	}
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]FriendRequest), make(map[string]Friendship), nil
		}
		return nil, nil, err
	}
	if len(data) == 0 {
		return make(map[string]FriendRequest), make(map[string]Friendship), nil
	}
	var payload friendshipStoreFile
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, nil, err
	}
	if payload.Requests == nil {
		payload.Requests = make(map[string]FriendRequest)
	}
	if payload.Friendships == nil {
		payload.Friendships = make(map[string]Friendship)
	}
	return payload.Requests, payload.Friendships, nil
}

func (s *fileFriendshipStore) persist(requests map[string]FriendRequest, friendships map[string]Friendship) error {
	if strings.TrimSpace(s.path) == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	payload := friendshipStoreFile{
		Requests:    requests,
		Friendships: friendships,
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0o644)
}

func (s *fileFriendshipStore) close() error {
	return nil
}

func normalizeFriendshipPair(accountID, friendAccountID string) (string, string, string, error) {
	left := strings.TrimSpace(accountID)
	right := strings.TrimSpace(friendAccountID)
	if left == "" || right == "" || left == right {
		return "", "", "", ErrInvalidFriendRequest
	}
	if left > right {
		left, right = right, left
	}
	return left, right, left + "::" + right, nil
}

func FriendAccountForViewer(friendship Friendship, viewerAccountID string) string {
	if friendship.LowAccountID == viewerAccountID {
		return friendship.HighAccountID
	}
	if friendship.HighAccountID == viewerAccountID {
		return friendship.LowAccountID
	}
	return ""
}
