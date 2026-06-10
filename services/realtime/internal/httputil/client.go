package httputil

import (
	"net/http"
	"sync"
	"time"
)

var defaultClient = &http.Client{
	Timeout: 3 * time.Second,
	Transport: &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 20,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 5 * time.Second,
	},
}

func HTTPClient() *http.Client {
	return defaultClient
}

func NewHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 20,
			IdleConnTimeout:     90 * time.Second,
			TLSHandshakeTimeout: 5 * time.Second,
		},
	}
}

type CircuitBreaker struct {
	name           string
	failureCount   int
	successCount   int
	state          string
	lastFailure    time.Time
	failureThresh  int
	successThresh  int
	halfOpenMax    int
	halfOpenCount  int
	timeout        time.Duration
	mu             sync.Mutex
}

type BreakerPool struct {
	breakers map[string]*CircuitBreaker
	mu       sync.Mutex
}

func NewBreakerPool() *BreakerPool {
	return &BreakerPool{
		breakers: make(map[string]*CircuitBreaker),
	}
}

func (p *BreakerPool) Get(name string) *CircuitBreaker {
	p.mu.Lock()
	defer p.mu.Unlock()
	if cb, ok := p.breakers[name]; ok {
		return cb
	}
	cb := &CircuitBreaker{
		name:          name,
		state:         "closed",
		failureThresh: 5,
		successThresh: 3,
		halfOpenMax:   3,
		timeout:       30 * time.Second,
	}
	p.breakers[name] = cb
	return cb
}

func (cb *CircuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case "open":
		if time.Since(cb.lastFailure) > cb.timeout {
			cb.state = "half-open"
			cb.halfOpenCount = 0
			cb.successCount = 0
			return true
		}
		return false
	case "half-open":
		return cb.halfOpenCount < cb.halfOpenMax
	default:
		return true
	}
}

func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case "half-open":
		cb.successCount++
		if cb.successCount >= cb.successThresh {
			cb.state = "closed"
			cb.failureCount = 0
			cb.successCount = 0
		}
	case "closed":
		cb.failureCount = 0
	}
}

func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case "half-open":
		cb.state = "open"
		cb.lastFailure = time.Now()
	case "closed":
		cb.failureCount++
		if cb.failureCount >= cb.failureThresh {
			cb.state = "open"
			cb.lastFailure = time.Now()
		}
	}
}

func (cb *CircuitBreaker) State() string {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.state
}
