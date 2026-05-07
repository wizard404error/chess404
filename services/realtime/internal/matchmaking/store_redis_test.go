package matchmaking

import (
	"testing"

	"github.com/alicebob/miniredis/v2"
)

func TestRedisQueueStorePersistsAcrossReload(t *testing.T) {
	redisServer := miniredis.RunT(t)
	redisURL := "redis://" + redisServer.Addr() + "/0"

	service, err := NewRedisPersistentService(redisURL, "")
	if err != nil {
		t.Fatalf("create redis persistent service: %v", err)
	}
	defer func() { _ = service.Close() }()

	first, err := service.Enqueue(QueueRated, "guest_a", 1200)
	if err != nil {
		t.Fatalf("enqueue first ticket: %v", err)
	}
	second, err := service.Enqueue(QueueRated, "guest_b", 1210)
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
}
