package matchmaking

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/chess404/realtime/internal/contracts"
)

type QueueName string

const (
	QueueCasual QueueName = "casual"
	QueueRated  QueueName = "rated"
)

const (
	defaultQueuedTTL          = 1 * time.Hour
	defaultMatchedRecoveryTTL = 3 * time.Minute
	defaultCancelledTicketTTL = 30 * time.Second
	defaultMaxRatingDiff      = 400
)

type TicketStatus string

const (
	StatusQueued    TicketStatus = "queued"
	StatusMatched   TicketStatus = "matched"
	StatusCancelled TicketStatus = "cancelled"
)

type Ticket struct {
	TicketID     string                `json:"ticketId"`
	GuestID      string                `json:"guestId"`
	AccountID    string                `json:"accountId,omitempty"`
	DisplayName  string                `json:"displayName,omitempty"`
	Queue        QueueName             `json:"queue"`
	ModeID       contracts.MatchModeID `json:"modeId,omitempty"`
	Status       TicketStatus          `json:"status"`
	Rating       int                   `json:"rating"`
	CreatedAt    time.Time             `json:"createdAt"`
	UpdatedAt    time.Time             `json:"updatedAt"`
	MatchedAt    *time.Time            `json:"matchedAt,omitempty"`
	MatchedWith  string                `json:"matchedWith,omitempty"`
	SeatColor    string                `json:"seatColor,omitempty"`
	OpponentName string                `json:"opponentName,omitempty"`
	AssignedRoom string                `json:"assignedRoom,omitempty"`
}

type QueueSnapshot struct {
	Queue          QueueName             `json:"queue"`
	ModeID         contracts.MatchModeID `json:"modeId,omitempty"`
	QueuedCount    int                   `json:"queuedCount"`
	MatchedCount   int                   `json:"matchedCount"`
	CancelledCount int                   `json:"cancelledCount"`
}

type Service struct {
	mu                  sync.Mutex
	store               ticketStore
	tickets             map[string]Ticket
	creator             MatchCreator
	now                 func() time.Time
	queuedTTL           time.Duration
	matchedRecoveryTTL  time.Duration
	cancelledTicketTTL  time.Duration
	cleanupStopCh       chan struct{}
}

type MatchAssignment struct {
	Queue             QueueName
	ModeID            contracts.MatchModeID
	RoomID            string
	WhiteGuestID      string
	BlackGuestID      string
	WhiteAccountID    string
	BlackAccountID    string
	WhiteName         string
	BlackName         string
	WhitePlayerSecret string
	BlackPlayerSecret string
}

type MatchCreator interface {
	CreateMatch(assignment MatchAssignment) error
}

var ErrGuestAlreadyQueued = errors.New("guest already has an active queue ticket")

type ActiveTicketError struct {
	Ticket Ticket
}

func (e ActiveTicketError) Error() string {
	return ErrGuestAlreadyQueued.Error()
}

type ServiceStats struct {
	Backend      string        `json:"backend"`
	TotalTickets int           `json:"totalTickets"`
	Casual       QueueSnapshot `json:"casual"`
	Rated        QueueSnapshot `json:"rated"`
}

func NewService() *Service {
	return newService(nil)
}

func NewPersistentService(path string) (*Service, error) {
	return newPersistentService(newFileTicketStore(path))
}

func NewSQLitePersistentService(path string) (*Service, error) {
	store, err := newSQLiteTicketStore(path)
	if err != nil {
		return nil, err
	}
	return newPersistentService(store)
}

func NewRedisPersistentService(redisURL, key string) (*Service, error) {
	store, err := newRedisTicketStore(redisURL, key)
	if err != nil {
		return nil, err
	}
	return newPersistentService(store)
}

func newPersistentService(store ticketStore) (*Service, error) {
	service := newService(store)
	if err := service.loadLocked(); err != nil {
		_ = store.close()
		return nil, err
	}
	return service, nil
}

func newService(store ticketStore) *Service {
	s := &Service{
		store:              store,
		tickets:            make(map[string]Ticket),
		now:                time.Now,
		queuedTTL:          defaultQueuedTTL,
		matchedRecoveryTTL: defaultMatchedRecoveryTTL,
		cancelledTicketTTL: defaultCancelledTicketTTL,
		cleanupStopCh:      make(chan struct{}),
	}
	s.startCleanupLoop()
	return s
}

func (s *Service) startCleanupLoop() {
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-s.cleanupStopCh:
				return
			case <-ticker.C:
				s.mu.Lock()
				s.pruneExpiredLocked(s.nowUTC())
				s.mu.Unlock()
			}
		}
	}()
}

func (s *Service) SetMatchCreator(creator MatchCreator) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.creator = creator
}

func (s *Service) Close() error {
	close(s.cleanupStopCh)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.store == nil {
		return nil
	}
	return s.store.close()
}

func (s *Service) Backend() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.store == nil {
		return "memory"
	}
	return s.store.backend()
}

func (s *Service) Enqueue(queue QueueName, modeID contracts.MatchModeID, guestID string, rating int, displayName string) (Ticket, error) {
	return s.EnqueueWithAccount(queue, modeID, guestID, rating, displayName, "")
}

func (s *Service) EnqueueWithAccount(queue QueueName, modeID contracts.MatchModeID, guestID string, rating int, displayName, accountID string) (Ticket, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.nowUTC()
	if s.pruneExpiredLocked(now) {
		if err := s.persistLocked(); err != nil {
			return Ticket{}, err
		}
	}

	modeID = normalizeModeID(modeID)
	if active, ok := s.findActiveTicketForGuestLocked(guestID); ok {
		if active.Queue == queue && normalizeModeID(active.ModeID) == modeID {
			return active, nil
		}
		return Ticket{}, ActiveTicketError{Ticket: active}
	}

	ticket := Ticket{
		TicketID:    "ticket_" + randomToken(6),
		GuestID:     guestID,
		AccountID:   accountID,
		DisplayName: normalizeDisplayName(displayName, guestID),
		Queue:       queue,
		ModeID:      modeID,
		Status:      StatusQueued,
		Rating:      rating,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if opponent, ok := s.findMatchCandidateLocked(queue, modeID, guestID, rating); ok {
		matchedAt := now
		roomID := "room_" + randomToken(5)
		
		whiteGuest, blackGuest := opponent.GuestID, guestID
		whiteAccount, blackAccount := opponent.AccountID, ticket.AccountID
		whiteName, blackName := normalizeDisplayName(opponent.DisplayName, opponent.GuestID), ticket.DisplayName
		
		b := make([]byte, 1)
		if _, err := rand.Read(b); err == nil && b[0]%2 == 0 {
			whiteGuest, blackGuest = blackGuest, whiteGuest
			whiteAccount, blackAccount = blackAccount, whiteAccount
			whiteName, blackName = blackName, whiteName
		}
		
		assignment := MatchAssignment{
			Queue:             queue,
			ModeID:            modeID,
			RoomID:            roomID,
			WhiteGuestID:      whiteGuest,
			BlackGuestID:      blackGuest,
			WhiteAccountID:    whiteAccount,
			BlackAccountID:    blackAccount,
			WhiteName:         whiteName,
			BlackName:         blackName,
			WhitePlayerSecret: "seat_" + randomToken(12),
			BlackPlayerSecret: "seat_" + randomToken(12),
		}
		s.tickets[ticket.TicketID] = ticket
		s.tickets[opponent.TicketID] = opponent
		if err := s.persistLocked(); err != nil {
			delete(s.tickets, ticket.TicketID)
			delete(s.tickets, opponent.TicketID)
			return Ticket{}, err
		}

		if s.creator != nil {
			resultCh := make(chan error, 1)
			go func() {
				resultCh <- s.creator.CreateMatch(assignment)
			}()
			s.mu.Unlock()
			err := <-resultCh
			s.mu.Lock()
			// Reload fresh tickets after re-acquiring lock (TOCTOU guard)
			currentTicket, ticketOK := s.tickets[ticket.TicketID]
			currentOpponent, opponentOK := s.tickets[opponent.TicketID]
			if err != nil {
				if ticketOK {
					delete(s.tickets, ticket.TicketID)
				}
				if opponentOK {
					delete(s.tickets, opponent.TicketID)
				}
				if err2 := s.persistLocked(); err2 != nil {
					log.Printf("failed to persist after CreateMatch rollback: %v", err2)
				}
				return Ticket{}, err
			}
			if !ticketOK || !opponentOK {
				return Ticket{}, errors.New("ticket disappeared during match creation")
			}
			if currentTicket.Status != StatusQueued {
				return Ticket{}, errors.New("ticket status changed during match creation")
			}
			if currentOpponent.Status != StatusQueued {
				return Ticket{}, errors.New("opponent ticket status changed during match creation")
			}
			// Use re-fetched tickets for the remainder
			ticket = currentTicket
			opponent = currentOpponent
		}

		ticket.Status = StatusMatched
		ticket.MatchedAt = &matchedAt
		ticket.MatchedWith = opponent.GuestID
		if assignment.WhiteGuestID == ticket.GuestID {
			ticket.SeatColor = "white"
			ticket.OpponentName = assignment.BlackName
		} else {
			ticket.SeatColor = "black"
			ticket.OpponentName = assignment.WhiteName
		}
		ticket.AssignedRoom = roomID
		ticket.UpdatedAt = matchedAt

		opponent.Status = StatusMatched
		opponent.MatchedAt = &matchedAt
		opponent.MatchedWith = guestID
		if assignment.WhiteGuestID == opponent.GuestID {
			opponent.SeatColor = "white"
			opponent.OpponentName = assignment.BlackName
		} else {
			opponent.SeatColor = "black"
			opponent.OpponentName = assignment.WhiteName
		}
		opponent.AssignedRoom = roomID
		opponent.UpdatedAt = matchedAt

		s.tickets[ticket.TicketID] = ticket
		s.tickets[opponent.TicketID] = opponent
		if err := s.persistLocked(); err != nil {
			delete(s.tickets, ticket.TicketID)
			delete(s.tickets, opponent.TicketID)
			return Ticket{}, err
		}
		return ticket, nil
	}

	s.tickets[ticket.TicketID] = ticket
	if err := s.persistLocked(); err != nil {
		delete(s.tickets, ticket.TicketID)
		return Ticket{}, err
	}
	return ticket, nil
}

func (s *Service) Get(ticketID string) (Ticket, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pruneExpiredLocked(s.nowUTC()) {
		if err := s.persistLocked(); err != nil {
			log.Printf("warning: failed to persist after pruning: %v", err)
		}
	}
	ticket, ok := s.tickets[ticketID]
	return ticket, ok
}

func (s *Service) FindActiveTicket(guestID, accountID string) (Ticket, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pruneExpiredLocked(s.nowUTC()) {
		if err := s.persistLocked(); err != nil {
			log.Printf("warning: failed to persist after pruning: %v", err)
		}
	}
	return s.findActiveTicketLocked(guestID, accountID)
}

func (s *Service) Cancel(ticketID string) (Ticket, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.nowUTC()
	if s.pruneExpiredLocked(now) {
		if err := s.persistLocked(); err != nil {
			return Ticket{}, false, err
		}
	}
	ticket, ok := s.tickets[ticketID]
	if !ok {
		return Ticket{}, false, nil
	}
	if ticket.Status != StatusQueued {
		return ticket, false, nil
	}
	ticket.Status = StatusCancelled
	ticket.UpdatedAt = now
	if err := s.persistLocked(); err != nil {
		return Ticket{}, false, err
	}
	s.tickets[ticketID] = ticket
	return ticket, true, nil
}

func (s *Service) Snapshot(queue QueueName, modeID contracts.MatchModeID) QueueSnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pruneExpiredLocked(s.nowUTC()) {
		if err := s.persistLocked(); err != nil {
			log.Printf("warning: failed to persist after pruning: %v", err)
		}
	}

	modeID = normalizeModeID(modeID)
	snapshot := QueueSnapshot{Queue: queue, ModeID: modeID}
	for _, ticket := range s.tickets {
		if ticket.Queue != queue {
			continue
		}
		if normalizeModeID(ticket.ModeID) != modeID {
			continue
		}
		switch ticket.Status {
		case StatusQueued:
			snapshot.QueuedCount++
		case StatusMatched:
			snapshot.MatchedCount++
		case StatusCancelled:
			snapshot.CancelledCount++
		}
	}
	return snapshot
}

func (s *Service) List(queue QueueName, modeID contracts.MatchModeID) []Ticket {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pruneExpiredLocked(s.nowUTC()) {
		if err := s.persistLocked(); err != nil {
			log.Printf("warning: failed to persist after pruning: %v", err)
		}
	}

	modeID = normalizeModeID(modeID)
	items := make([]Ticket, 0)
	for _, ticket := range s.tickets {
		if queue != "" && ticket.Queue != queue {
			continue
		}
		if modeID != "" && normalizeModeID(ticket.ModeID) != modeID {
			continue
		}
		items = append(items, ticket)
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].CreatedAt.Before(items[j].CreatedAt)
	})
	return items
}

func (s *Service) Stats() ServiceStats {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pruneExpiredLocked(s.nowUTC()) {
		if err := s.persistLocked(); err != nil {
			log.Printf("warning: failed to persist after pruning: %v", err)
		}
	}

	stats := ServiceStats{
		Backend:      "memory",
		TotalTickets: len(s.tickets),
		Casual:       QueueSnapshot{Queue: QueueCasual},
		Rated:        QueueSnapshot{Queue: QueueRated},
	}
	if s.store != nil {
		stats.Backend = s.store.backend()
	}
	for _, ticket := range s.tickets {
		var snapshot *QueueSnapshot
		switch ticket.Queue {
		case QueueCasual:
			snapshot = &stats.Casual
		case QueueRated:
			snapshot = &stats.Rated
		default:
			continue
		}
		switch ticket.Status {
		case StatusQueued:
			snapshot.QueuedCount++
		case StatusMatched:
			snapshot.MatchedCount++
		case StatusCancelled:
			snapshot.CancelledCount++
		}
	}
	return stats
}

func (s *Service) findMatchCandidateLocked(queue QueueName, modeID contracts.MatchModeID, guestID string, rating int) (Ticket, bool) {
	candidates := make([]Ticket, 0)
	now := s.nowUTC()
	for _, ticket := range s.tickets {
		if ticket.Queue != queue || normalizeModeID(ticket.ModeID) != modeID || ticket.Status != StatusQueued || ticket.GuestID == guestID {
			continue
		}
		diff := ticket.Rating - rating
		if diff < 0 {
			diff = -diff
		}
		expansion := int(now.Sub(ticket.CreatedAt).Seconds() / 30) * 50
		maxDiff := defaultMaxRatingDiff + expansion
		if diff > maxDiff {
			continue
		}
		candidates = append(candidates, ticket)
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].CreatedAt.Before(candidates[j].CreatedAt)
	})
	if len(candidates) == 0 {
		return Ticket{}, false
	}
	return candidates[0], true
}

func (s *Service) findActiveTicketForGuestLocked(guestID string) (Ticket, bool) {
	return s.findActiveTicketLocked(guestID, "")
}

func (s *Service) findActiveTicketLocked(guestID, accountID string) (Ticket, bool) {
	guestID = strings.TrimSpace(guestID)
	accountID = strings.TrimSpace(accountID)
	var latest Ticket
	found := false
	for _, ticket := range s.tickets {
		if ticket.Status == StatusCancelled {
			continue
		}
		if guestID != "" && ticket.GuestID != guestID {
			continue
		}
		if guestID == "" && accountID != "" && strings.TrimSpace(ticket.AccountID) != accountID {
			continue
		}
		if guestID == "" && accountID == "" {
			continue
		}
		if !found || ticket.UpdatedAt.After(latest.UpdatedAt) {
			latest = ticket
			found = true
		}
	}
	return latest, found
}

func randomToken(bytesCount int) string {
	buf := make([]byte, bytesCount)
	if _, err := rand.Read(buf); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 16)
	}
	return hex.EncodeToString(buf)
}

func normalizeDisplayName(displayName, fallback string) string {
	value := displayName
	if value == "" {
		value = fallback
	}
	if value == "" {
		return "Player"
	}
	return value
}

func (s *Service) loadLocked() error {
	if s.store == nil {
		return nil
	}
	tickets, err := s.store.load()
	if err != nil {
		return err
	}
	if tickets == nil {
		tickets = make(map[string]Ticket)
	}
	for ticketID, ticket := range tickets {
		ticket.ModeID = normalizeModeID(ticket.ModeID)
		tickets[ticketID] = ticket
	}
	s.tickets = tickets
	if s.pruneExpiredLocked(s.nowUTC()) {
		return s.persistLocked()
	}
	return nil
}

func (s *Service) nowUTC() time.Time {
	if s.now == nil {
		return time.Now().UTC()
	}
	return s.now().UTC()
}

func (s *Service) pruneExpiredLocked(now time.Time) bool {
	changed := false
	for ticketID, ticket := range s.tickets {
		if !s.ticketRecoverableLocked(ticket, now) {
			delete(s.tickets, ticketID)
			changed = true
		}
	}
	return changed
}

func (s *Service) ticketRecoverableLocked(ticket Ticket, now time.Time) bool {
	switch ticket.Status {
	case StatusQueued:
		return ticket.UpdatedAt.Add(s.queuedTTL).After(now)
	case StatusMatched:
		return ticket.UpdatedAt.Add(s.matchedRecoveryTTL).After(now)
	case StatusCancelled:
		return ticket.UpdatedAt.Add(s.cancelledTicketTTL).After(now)
	default:
		return false
	}
}

func (s *Service) persistLocked() error {
	if s.store == nil {
		return nil
	}
	return s.store.persist(s.tickets)
}

func normalizeModeID(modeID contracts.MatchModeID) contracts.MatchModeID {
	return contracts.NormalizeMatchModeID(string(modeID))
}
