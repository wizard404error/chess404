package matchmaking

import (
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/chess404/realtime/internal/contracts"
)

func TestRedisQueueStorePersistsAcrossReload(t *testing.T) {
	redisServer := miniredis.RunT(t)
	redisURL := "redis://" + redisServer.Addr() + "/0"

	service, err := NewRedisPersistentService(redisURL, "")
	if err != nil {
		t.Fatalf("create redis persistent service: %v", err)
	}
	defer func() { _ = service.Close() }()

	first, err := service.Enqueue(QueueRated, contracts.MatchModeOpenCards, "guest_a", 1200, "Alpha")
	if err != nil {
		t.Fatalf("enqueue first ticket: %v", err)
	}
	second, err := service.Enqueue(QueueRated, contracts.MatchModeOpenCards, "guest_b", 1210, "Bravo")
	if err != nil {
		t.Fatalf("enqueue second ticket: %v", err)
	}

	reloaded, err := NewRedisPersistentService(redisURL, "")
	if err != nil {
		t.Fatalf("reload redis persistent service: %v", err)
	}
	defer func() { _ = reloaded.Close() }()

	firstReloaded, ok := reloaded.Get(first.TicketID)
	if !ok {
		t.Fatalf("expected first redis ticket after reload")
	}
	secondReloaded, ok := reloaded.Get(second.TicketID)
	if !ok {
		t.Fatalf("expected second redis ticket after reload")
	}
	if firstReloaded.AssignedRoom == "" || firstReloaded.AssignedRoom != secondReloaded.AssignedRoom {
		t.Fatalf("expected redis matched room to survive reload, got %#v and %#v", firstReloaded, secondReloaded)
	}
	if reloaded.Backend() != "redis" {
		t.Fatalf("expected redis backend, got %s", reloaded.Backend())
	}
	if firstReloaded.ModeID != contracts.MatchModeOpenCards || secondReloaded.ModeID != contracts.MatchModeOpenCards {
		t.Fatalf("expected redis reload to preserve mode metadata, got %#v and %#v", firstReloaded, secondReloaded)
	}
}
