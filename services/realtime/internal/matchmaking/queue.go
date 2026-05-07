package matchmaking

import (
	"crypto/rand"
	"encoding/hex"
	"sort"
	"sync"
	"time"
)

type QueueName string

const (
	QueueCasual QueueName = "casual"
	QueueRated  QueueName = "rated"
)

type TicketStatus string

const (
	StatusQueued    TicketStatus = "queued"
	StatusMatched   TicketStatus = "matched"
	StatusCancelled TicketStatus = "cancelled"
)

type Ticket struct {
	TicketID     string       `json:"ticketId"`
	GuestID      string       `json:"guestId"`
	Queue        QueueName    `json:"queue"`
	Status       TicketStatus `json:"status"`
	Rating       int          `json:"rating"`
	CreatedAt    time.Time    `json:"createdAt"`
	UpdatedAt    time.Time    `json:"updatedAt"`
	MatchedAt    *time.Time   `json:"matchedAt,omitempty"`
	MatchedWith  string       `json:"matchedWith,omitempty"`
	AssignedRoom string       `json:"assignedRoom,omitempty"`
}

type QueueSnapshot struct {
	Queue          QueueName `json:"queue"`
	QueuedCount    int       `json:"queuedCount"`
	MatchedCount   int       `json:"matchedCount"`
	CancelledCount int       `json:"cancelledCount"`
}

type Service struct {
	mu      sync.Mutex
	store   ticketStore
	tickets map[string]Ticket
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
	return &Service{
		store:   store,
		tickets: make(map[string]Ticket),
	}
}

func (s *Service) Close() error {
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

func (s *Service) Enqueue(queue QueueName, guestID string, rating int) (Ticket, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	ticket := Ticket{
		TicketID:  "ticket_" + randomToken(6),
		GuestID:   guestID,
		Queue:     queue,
		Status:    StatusQueued,
		Rating:    rating,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if opponent, ok := s.findMatchCandidateLocked(queue, guestID); ok {
		matchedAt := now
		roomID := "room_" + randomToken(5)
		ticket.Status = StatusMatched
		ticket.MatchedAt = &matchedAt
		ticket.MatchedWith = opponent.GuestID
		ticket.AssignedRoom = roomID
		ticket.UpdatedAt = matchedAt

		opponent.Status = StatusMatched
		opponent.MatchedAt = &matchedAt
		opponent.MatchedWith = guestID
		opponent.AssignedRoom = roomID
		opponent.UpdatedAt = matchedAt
		s.tickets[opponent.TicketID] = opponent
	}

	s.tickets[ticket.TicketID] = ticket
	if err := s.persistLocked(); err != nil {
		return Ticket{}, err
	}
	return ticket, nil
}

func (s *Service) Get(ticketID string) (Ticket, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ticket, ok := s.tickets[ticketID]
	return ticket, ok
}

func (s *Service) Cancel(ticketID string) (Ticket, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ticket, ok := s.tickets[ticketID]
	if !ok {
		return Ticket{}, false, nil
	}
	if ticket.Status == StatusQueued {
		ticket.Status = StatusCancelled
		ticket.UpdatedAt = time.Now().UTC()
		s.tickets[ticketID] = ticket
		if err := s.persistLocked(); err != nil {
			return Ticket{}, true, err
		}
	}
	return ticket, true, nil
}

func (s *Service) Snapshot(queue QueueName) QueueSnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()

	snapshot := QueueSnapshot{Queue: queue}
	for _, ticket := range s.tickets {
		if ticket.Queue != queue {
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

func (s *Service) List(queue QueueName) []Ticket {
	s.mu.Lock()
	defer s.mu.Unlock()

	items := make([]Ticket, 0)
	for _, ticket := range s.tickets {
		if queue != "" && ticket.Queue != queue {
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
		if ticket.Queue == QueueCasual {
			snapshot = &stats.Casual
		} else {
			snapshot = &stats.Rated
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

func (s *Service) findMatchCandidateLocked(queue QueueName, guestID string) (Ticket, bool) {
	candidates := make([]Ticket, 0)
	for _, ticket := range s.tickets {
		if ticket.Queue != queue || ticket.Status != StatusQueued || ticket.GuestID == guestID {
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

func randomToken(bytesCount int) string {
	buf := make([]byte, bytesCount)
	if _, err := rand.Read(buf); err != nil {
		return time.Now().UTC().Format("150405.000000000")
	}
	return hex.EncodeToString(buf)
}

func (s *Service) loadLocked() error {
	if s.store == nil {
		return nil
	}
	tickets, err := s.store.load()
	if err != nil {
		return err
	}
	s.tickets = tickets
	return nil
}

func (s *Service) persistLocked() error {
	if s.store == nil {
		return nil
	}
	return s.store.persist(s.tickets)
}
