package messaging

import (
	"context"
	"encoding/json"
	"time"
)

type EventType string

const (
	EventMatchCreated    EventType = "match.created"
	EventMatchStarted    EventType = "match.started"
	EventMatchFinished   EventType = "match.finished"
	EventMatchMove       EventType = "match.move"
	EventMatchCardPlayed EventType = "match.card_played"
	EventMatchTimedOut   EventType = "match.timed_out"
	EventQueueJoined     EventType = "queue.joined"
	EventQueueMatched    EventType = "queue.matched"
	EventQueueLeft       EventType = "queue.left"
	EventPresenceUpdate  EventType = "presence.update"
)

type Event struct {
	ID        string    `json:"id"`
	Type      EventType `json:"type"`
	Source    string    `json:"source"`
	Timestamp time.Time `json:"timestamp"`
	Payload   []byte    `json:"payload"`
}

type EventHandler func(ctx context.Context, event Event) error

type MessageBus interface {
	Publish(ctx context.Context, topic string, event Event) error
	Subscribe(ctx context.Context, topic string, handler EventHandler) error
	Close() error
}

func EncodePayload(v any) ([]byte, error) {
	return json.Marshal(v)
}

func DecodePayload(data []byte, v any) error {
	return json.Unmarshal(data, v)
}

func encodeEvent(event Event) ([]byte, error) {
	return json.Marshal(event)
}

func decodeEvent(data []byte, event *Event) error {
	return json.Unmarshal(data, event)
}
