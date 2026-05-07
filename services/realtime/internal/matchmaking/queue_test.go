package matchmaking

import (
	"path/filepath"
	"testing"
)

func TestQueueMatchesSecondTicket(t *testing.T) {
	service := NewService()
	first, err := service.Enqueue(QueueCasual, "guest_a", 1200)
	if err != nil {
		t.Fatalf("enqueue first ticket: %v", err)
	}
	if first.Status != StatusQueued {
		t.Fatalf("expected first ticket queued, got %s", first.Status)
	}

	second, err := service.Enqueue(QueueCasual, "guest_b", 1210)
	if err != nil {
		t.Fatalf("enqueue second ticket: %v", err)
	}
	if second.Status != StatusMatched {
		t.Fatalf("expected second ticket matched, got %s", second.Status)
	}

	reloadedFirst, ok := service.Get(first.TicketID)
	if !ok {
		t.Fatalf("expected first ticket to remain queryable")
	}
	if reloadedFirst.Status != StatusMatched || reloadedFirst.AssignedRoom == "" || reloadedFirst.MatchedWith != "guest_b" {
		t.Fatalf("unexpected first ticket state %#v", reloadedFirst)
	}
	if second.AssignedRoom == "" || second.AssignedRoom != reloadedFirst.AssignedRoom {
		t.Fatalf("expected shared assigned room, got %#v and %#v", reloadedFirst, second)
	}
}

func TestQueueCancelQueuedTicket(t *testing.T) {
	service := NewService()
	ticket, err := service.Enqueue(QueueRated, "guest_a", 1300)
	if err != nil {
		t.Fatalf("enqueue ticket: %v", err)
	}
	cancelled, ok, err := service.Cancel(ticket.TicketID)
	if !ok {
		t.Fatalf("expected cancel to find ticket")
	}
	if err != nil {
		t.Fatalf("cancel ticket: %v", err)
	}
	if cancelled.Status != StatusCancelled {
		t.Fatalf("expected cancelled status, got %s", cancelled.Status)
	}
	snapshot := service.Snapshot(QueueRated)
	if snapshot.CancelledCount != 1 {
		t.Fatalf("expected cancelled count 1, got %#v", snapshot)
	}
}

func TestQueueStorePersistsAcrossReload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tickets.json")

	service, err := NewPersistentService(path)
	if err != nil {
		t.Fatalf("create persistent service: %v", err)
	}

	first, err := service.Enqueue(QueueCasual, "guest_a", 1200)
	if err != nil {
		t.Fatalf("enqueue first ticket: %v", err)
	}
	second, err := service.Enqueue(QueueCasual, "guest_b", 1210)
	if err != nil {
		t.Fatalf("enqueue second ticket: %v", err)
	}

	reloaded, err := NewPersistentService(path)
	if err != nil {
		t.Fatalf("reload persistent service: %v", err)
	}

	firstReloaded, ok := reloaded.Get(first.TicketID)
	if !ok {
		t.Fatalf("expected first ticket after reload")
	}
	secondReloaded, ok := reloaded.Get(second.TicketID)
	if !ok {
		t.Fatalf("expected second ticket after reload")
	}
	if firstReloaded.AssignedRoom == "" || firstReloaded.AssignedRoom != secondReloaded.AssignedRoom {
		t.Fatalf("expected matched room to survive reload, got %#v and %#v", firstReloaded, secondReloaded)
	}
	if firstReloaded.Status != StatusMatched || secondReloaded.Status != StatusMatched {
		t.Fatalf("expected matched statuses after reload, got %#v and %#v", firstReloaded, secondReloaded)
	}
}

func TestSQLiteQueueStorePersistsAcrossReload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tickets.sqlite")

	service, err := NewSQLitePersistentService(path)
	if err != nil {
		t.Fatalf("create sqlite persistent service: %v", err)
	}
	defer func() { _ = service.Close() }()

	first, err := service.Enqueue(QueueCasual, "guest_a", 1200)
	if err != nil {
		t.Fatalf("enqueue first ticket: %v", err)
	}
	second, err := service.Enqueue(QueueCasual, "guest_b", 1210)
	if err != nil {
		t.Fatalf("enqueue second ticket: %v", err)
	}

	reloaded, err := NewSQLitePersistentService(path)
	if err != nil {
		t.Fatalf("reload sqlite persistent service: %v", err)
	}
	defer func() { _ = reloaded.Close() }()

	firstReloaded, ok := reloaded.Get(first.TicketID)
	if !ok {
		t.Fatalf("expected first ticket after reload")
	}
	secondReloaded, ok := reloaded.Get(second.TicketID)
	if !ok {
		t.Fatalf("expected second ticket after reload")
	}
	if firstReloaded.AssignedRoom == "" || firstReloaded.AssignedRoom != secondReloaded.AssignedRoom {
		t.Fatalf("expected matched room to survive reload, got %#v and %#v", firstReloaded, secondReloaded)
	}
	if reloaded.Backend() != "sqlite" {
		t.Fatalf("expected sqlite backend, got %s", reloaded.Backend())
	}
}

func TestQueueStatsReflectTicketState(t *testing.T) {
	service := NewService()
	first, err := service.Enqueue(QueueRated, "guest_a", 1200)
	if err != nil {
		t.Fatalf("enqueue first rated ticket: %v", err)
	}
	if _, err := service.Enqueue(QueueRated, "guest_b", 1210); err != nil {
		t.Fatalf("enqueue second rated ticket: %v", err)
	}
	casual, err := service.Enqueue(QueueCasual, "guest_c", 1190)
	if err != nil {
		t.Fatalf("enqueue casual ticket: %v", err)
	}
	if _, _, err := service.Cancel(casual.TicketID); err != nil {
		t.Fatalf("cancel casual ticket: %v", err)
	}

	stats := service.Stats()
	if stats.TotalTickets != 3 {
		t.Fatalf("expected 3 total tickets, got %#v", stats)
	}
	if stats.Rated.MatchedCount != 2 || stats.Rated.QueuedCount != 0 {
		t.Fatalf("expected rated snapshot to reflect matched pair, got %#v", stats.Rated)
	}
	if stats.Casual.CancelledCount != 1 {
		t.Fatalf("expected casual snapshot to reflect cancellation, got %#v", stats.Casual)
	}
	if _, ok := service.Get(first.TicketID); !ok {
		t.Fatalf("expected first rated ticket to remain present")
	}
}
