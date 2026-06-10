package messaging

import (
	"context"
	"testing"
	"time"
)

func TestEventEncodeDecode(t *testing.T) {
	event := Event{
		ID:        "test-1",
		Type:      EventMatchCreated,
		Source:    "match-service",
		Timestamp: time.Now().Truncate(time.Millisecond),
		Payload:   []byte(`{"match_id":"abc123"}`),
	}

	data, err := encodeEvent(event)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	var decoded Event
	if err := decodeEvent(data, &decoded); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if decoded.ID != event.ID {
		t.Errorf("ID: got %s, want %s", decoded.ID, event.ID)
	}
	if decoded.Type != event.Type {
		t.Errorf("Type: got %s, want %s", decoded.Type, event.Type)
	}
	if decoded.Source != event.Source {
		t.Errorf("Source: got %s, want %s", decoded.Source, event.Source)
	}
	if !decoded.Timestamp.Equal(event.Timestamp) {
		t.Errorf("Timestamp: got %v, want %v", decoded.Timestamp, event.Timestamp)
	}
}

func TestEncodePayload(t *testing.T) {
	type TestPayload struct {
		MatchID string `json:"match_id"`
		Turn    string `json:"turn"`
	}

	payload := TestPayload{MatchID: "match-123", Turn: "white"}
	data, err := EncodePayload(payload)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	var decoded TestPayload
	if err := DecodePayload(data, &decoded); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if decoded.MatchID != "match-123" {
		t.Errorf("MatchID: got %s, want match-123", decoded.MatchID)
	}
	if decoded.Turn != "white" {
		t.Errorf("Turn: got %s, want white", decoded.Turn)
	}
}

func TestEventTypes(t *testing.T) {
	types := []EventType{
		EventMatchCreated,
		EventMatchStarted,
		EventMatchFinished,
		EventMatchMove,
		EventMatchCardPlayed,
		EventMatchTimedOut,
		EventQueueJoined,
		EventQueueMatched,
		EventQueueLeft,
		EventPresenceUpdate,
	}

	seen := make(map[EventType]bool)
	for _, et := range types {
		if seen[et] {
			t.Errorf("duplicate event type: %s", et)
		}
		seen[et] = true
		if et == "" {
			t.Error("empty event type")
		}
	}
}

func TestNoopHandler(t *testing.T) {
	handler := func(ctx context.Context, event Event) error {
		return nil
	}

	event := Event{
		ID:   "test",
		Type: EventMatchCreated,
	}

	err := handler(context.Background(), event)
	if err != nil {
		t.Errorf("handler returned error: %v", err)
	}
}
