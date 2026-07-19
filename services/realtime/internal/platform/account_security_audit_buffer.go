package platform

import (
	"log"
	"sync"
	"time"
)

type bufferedAuditEvent struct {
	request AccountSecurityEventRequest
	event   AccountSecurityEvent
}

func NewBufferedAuditStore(inner AccountSecurityAuditDirectory, bufferSize int, flushInterval time.Duration) *BufferedAuditStore {
	s := &BufferedAuditStore{
		inner:   inner,
		events:  make(chan bufferedAuditEvent, bufferSize),
		flushCh: make(chan struct{}),
		stopCh:  make(chan struct{}),
	}
	go s.flushLoop(flushInterval)
	return s
}

type BufferedAuditStore struct {
	inner   AccountSecurityAuditDirectory
	events  chan bufferedAuditEvent
	flushCh chan struct{}
	stopCh  chan struct{}
	wg      sync.WaitGroup
}

func (s *BufferedAuditStore) Backend() string {
	return s.inner.Backend()
}

func (s *BufferedAuditStore) RecordEvent(request AccountSecurityEventRequest) (AccountSecurityEvent, error) {
	event, err := normalizeAccountSecurityEventRequest(request)
	if err != nil {
		return AccountSecurityEvent{}, err
	}
	select {
	case s.events <- bufferedAuditEvent{request: request, event: event}:
	default:
		log.Printf("buffered audit store queue full (%d), recording synchronously", cap(s.events))
		return s.inner.RecordEvent(request)
	}
	return event, nil
}

func (s *BufferedAuditStore) ListOverview(accountID string, limit int) AccountSecurityEventOverview {
	return s.inner.ListOverview(accountID, limit)
}

func (s *BufferedAuditStore) Stats() AccountSecurityAuditStats {
	return s.inner.Stats()
}

func (s *BufferedAuditStore) Close() error {
	close(s.stopCh)
	s.flush()
	return s.inner.Close()
}

func (s *BufferedAuditStore) flush() {
	for {
		select {
		case be := <-s.events:
			if _, err := s.inner.RecordEvent(be.request); err != nil {
				log.Printf("buffered audit store failed to flush event %s: %v", be.event.EventID, err)
			}
		default:
			return
		}
	}
}

func (s *BufferedAuditStore) flushLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.flush()
		case <-s.flushCh:
			s.flush()
		}
	}
}
