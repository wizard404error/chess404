package matchmaking

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/chess404/realtime/internal/contracts"
)

type captureMatchCreator struct {
	assignments []MatchAssignment
}

func (c *captureMatchCreator) CreateMatch(assignment MatchAssignment) error {
	c.assignments = append(c.assignments, assignment)
	return nil
}

func TestQueueMatchesSecondTicket(t *testing.T) {
	service := NewService()
	first, err := service.Enqueue(QueueCasual, contracts.MatchModeOpenCards, "guest_a", 1200, "Alpha")
	if err != nil {
		t.Fatalf("enqueue first ticket: %v", err)
	}
	if first.Status != StatusQueued {
		t.Fatalf("expected first ticket queued, got %s", first.Status)
	}

	second, err := service.Enqueue(QueueCasual, contracts.MatchModeOpenCards, "guest_b", 1210, "Bravo")
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
	if (reloadedFirst.SeatColor != "white" || second.SeatColor != "black") &&
		(reloadedFirst.SeatColor != "black" || second.SeatColor != "white") {
		t.Fatalf("expected white/black seat assignment (in either order), got %#v and %#v", reloadedFirst, second)
	}
	if reloadedFirst.OpponentName != "Bravo" || second.OpponentName != "Alpha" {
		t.Fatalf("expected opponent names to be persisted, got %#v and %#v", reloadedFirst, second)
	}
}

func TestQueueMatchAssignmentCarriesAccountIDs(t *testing.T) {
	service := NewService()
	creator := &captureMatchCreator{}
	service.SetMatchCreator(creator)

	if _, err := service.EnqueueWithAccount(QueueRated, contracts.MatchModeOpenCards, "guest_a", 1200, "Alpha", "acct_alpha"); err != nil {
		t.Fatalf("enqueue first ticket: %v", err)
	}
	if _, err := service.EnqueueWithAccount(QueueRated, contracts.MatchModeOpenCards, "guest_b", 1210, "Bravo", "acct_bravo"); err != nil {
		t.Fatalf("enqueue second ticket: %v", err)
	}

	if len(creator.assignments) != 1 {
		t.Fatalf("expected one match assignment, got %#v", creator.assignments)
	}
	assignment := creator.assignments[0]
	whiteAccount := assignment.WhiteAccountID
	blackAccount := assignment.BlackAccountID
	if (whiteAccount != "acct_alpha" || blackAccount != "acct_bravo") &&
		(whiteAccount != "acct_bravo" || blackAccount != "acct_alpha") {
		t.Fatalf("expected account IDs to flow into match assignment as {alpha,bravo} or {bravo,alpha}, got %#v", assignment)
	}
}

func TestQueueCancelQueuedTicket(t *testing.T) {
	service := NewService()
	ticket, err := service.Enqueue(QueueRated, contracts.MatchModeOpenCards, "guest_a", 1300, "Alpha")
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
	snapshot := service.Snapshot(QueueRated, contracts.MatchModeOpenCards)
	if snapshot.CancelledCount != 1 {
		t.Fatalf("expected cancelled count 1, got %#v", snapshot)
	}
}

func TestQueueReturnsExistingActiveTicketForSameGuestAndQueue(t *testing.T) {
	service := NewService()

	first, err := service.Enqueue(QueueCasual, contracts.MatchModeOpenCards, "guest_a", 1200, "Alpha")
	if err != nil {
		t.Fatalf("enqueue first ticket: %v", err)
	}

	second, err := service.Enqueue(QueueCasual, contracts.MatchModeOpenCards, "guest_a", 1200, "Alpha")
	if err != nil {
		t.Fatalf("enqueue duplicate ticket: %v", err)
	}

	if second.TicketID != first.TicketID {
		t.Fatalf("expected duplicate join to return the same ticket, got %#v and %#v", first, second)
	}
	if len(service.List(QueueCasual, contracts.MatchModeOpenCards)) != 1 {
		t.Fatalf("expected only one active ticket in queue, got %#v", service.List(QueueCasual, contracts.MatchModeOpenCards))
	}
}

func TestQueueRejectsSecondActiveTicketInDifferentQueue(t *testing.T) {
	service := NewService()

	first, err := service.Enqueue(QueueCasual, contracts.MatchModeOpenCards, "guest_a", 1200, "Alpha")
	if err != nil {
		t.Fatalf("enqueue first ticket: %v", err)
	}

	_, err = service.Enqueue(QueueRated, contracts.MatchModeOpenCards, "guest_a", 1200, "Alpha")
	if err == nil {
		t.Fatalf("expected second active queue join to fail")
	}

	var activeErr ActiveTicketError
	if !errors.As(err, &activeErr) {
		t.Fatalf("expected ActiveTicketError, got %T (%v)", err, err)
	}
	if activeErr.Ticket.TicketID != first.TicketID {
		t.Fatalf("expected active ticket reference to match first ticket, got %#v and %#v", first, activeErr.Ticket)
	}
}

func TestQueueFindActiveTicketSupportsGuestAndAccountRecovery(t *testing.T) {
	service := NewService()

	guestTicket, err := service.EnqueueWithAccount(QueueRated, contracts.MatchModeOpenCards, "guest_a", 1200, "Alpha", "acct_alpha")
	if err != nil {
		t.Fatalf("enqueue guest ticket: %v", err)
	}
	accountTicket, err := service.EnqueueWithAccount(QueueCasual, contracts.MatchModeOpenCards, "guest_b", 1210, "Bravo", "acct_bravo")
	if err != nil {
		t.Fatalf("enqueue account ticket: %v", err)
	}

	foundByGuest, ok := service.FindActiveTicket("guest_a", "")
	if !ok || foundByGuest.TicketID != guestTicket.TicketID {
		t.Fatalf("expected guest lookup to recover the active ticket, got %#v ok=%v", foundByGuest, ok)
	}

	foundByAccount, ok := service.FindActiveTicket("", "acct_bravo")
	if !ok || foundByAccount.TicketID != accountTicket.TicketID {
		t.Fatalf("expected account lookup to recover the active ticket, got %#v ok=%v", foundByAccount, ok)
	}
}

func TestQueuePrunesTerminalTicketsAfterRecoveryTTL(t *testing.T) {
	service := NewService()
	base := time.Date(2026, 5, 22, 12, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return base }

	firstMatched, err := service.Enqueue(QueueRated, contracts.MatchModeOpenCards, "guest_a", 1200, "Alpha")
	if err != nil {
		t.Fatalf("enqueue first matched ticket: %v", err)
	}
	secondMatched, err := service.Enqueue(QueueRated, contracts.MatchModeOpenCards, "guest_b", 1210, "Bravo")
	if err != nil {
		t.Fatalf("enqueue second matched ticket: %v", err)
	}
	queued, err := service.Enqueue(QueueCasual, contracts.MatchModeOpenCards, "guest_c", 1190, "Charlie")
	if err != nil {
		t.Fatalf("enqueue queued ticket: %v", err)
	}
	cancelled, err := service.Enqueue(QueueCasual, contracts.MatchModeHiddenCards, "guest_d", 1185, "Delta")
	if err != nil {
		t.Fatalf("enqueue cancellable ticket: %v", err)
	}
	if _, _, err := service.Cancel(cancelled.TicketID); err != nil {
		t.Fatalf("cancel queued ticket: %v", err)
	}

	service.now = func() time.Time { return base.Add(defaultCancelledTicketTTL - time.Second) }
	if _, ok := service.Get(cancelled.TicketID); !ok {
		t.Fatalf("expected cancelled ticket to remain visible before TTL expires")
	}

	service.now = func() time.Time { return base.Add(defaultCancelledTicketTTL + time.Second) }
	if _, ok := service.Get(cancelled.TicketID); ok {
		t.Fatalf("expected cancelled ticket to be pruned after TTL")
	}
	if snapshot := service.Snapshot(QueueCasual, contracts.MatchModeOpenCards); snapshot.QueuedCount != 1 {
		t.Fatalf("expected open-cards queued ticket to remain, got %#v", snapshot)
	}
	if snapshot := service.Snapshot(QueueCasual, contracts.MatchModeHiddenCards); snapshot.CancelledCount != 0 {
		t.Fatalf("expected hidden-cards cancelled ticket to be pruned, got %#v", snapshot)
	}
	if recovered, ok := service.FindActiveTicket("guest_a", ""); !ok || recovered.TicketID != firstMatched.TicketID {
		t.Fatalf("expected matched ticket to stay recoverable during TTL, got %#v ok=%v", recovered, ok)
	}

	service.now = func() time.Time { return base.Add(defaultMatchedRecoveryTTL + time.Second) }
	if _, ok := service.Get(firstMatched.TicketID); ok {
		t.Fatalf("expected first matched ticket to be pruned after recovery TTL")
	}
	if _, ok := service.Get(secondMatched.TicketID); ok {
		t.Fatalf("expected second matched ticket to be pruned after recovery TTL")
	}
	if _, ok := service.FindActiveTicket("guest_a", ""); ok {
		t.Fatalf("expected matched recovery to disappear after TTL")
	}
	if stats := service.Stats(); stats.TotalTickets != 1 || stats.Casual.QueuedCount != 1 || stats.Rated.MatchedCount != 0 {
		t.Fatalf("expected only queued ticket to remain after pruning, got %#v", stats)
	}
	if items := service.List(QueueRated, contracts.MatchModeOpenCards); len(items) != 0 {
		t.Fatalf("expected stale matched tickets removed from list, got %#v", items)
	}
	if queuedTicket, ok := service.Get(queued.TicketID); !ok || queuedTicket.Status != StatusQueued {
		t.Fatalf("expected queued ticket to survive pruning, got %#v ok=%v", queuedTicket, ok)
	}
}

func TestQueueStorePersistsAcrossReload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tickets.json")

	service, err := NewPersistentService(path)
	if err != nil {
		t.Fatalf("create persistent service: %v", err)
	}

	first, err := service.Enqueue(QueueCasual, contracts.MatchModeOpenCards, "guest_a", 1200, "Alpha")
	if err != nil {
		t.Fatalf("enqueue first ticket: %v", err)
	}
	second, err := service.Enqueue(QueueCasual, contracts.MatchModeOpenCards, "guest_b", 1210, "Bravo")
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
	if firstReloaded.ModeID != contracts.MatchModeOpenCards || secondReloaded.ModeID != contracts.MatchModeOpenCards {
		t.Fatalf("expected mode metadata to survive reload, got %#v and %#v", firstReloaded, secondReloaded)
	}
}

func TestSQLiteQueueStorePersistsAcrossReload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tickets.sqlite")

	service, err := NewSQLitePersistentService(path)
	if err != nil {
		t.Fatalf("create sqlite persistent service: %v", err)
	}
	defer func() { _ = service.Close() }()

	first, err := service.Enqueue(QueueCasual, contracts.MatchModeHiddenCards, "guest_a", 1200, "Alpha")
	if err != nil {
		t.Fatalf("enqueue first ticket: %v", err)
	}
	second, err := service.Enqueue(QueueCasual, contracts.MatchModeHiddenCards, "guest_b", 1210, "Bravo")
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
	if firstReloaded.ModeID != contracts.MatchModeHiddenCards || secondReloaded.ModeID != contracts.MatchModeHiddenCards {
		t.Fatalf("expected sqlite reload to preserve mode metadata, got %#v and %#v", firstReloaded, secondReloaded)
	}
}

func TestSQLiteQueueStorePersistsAccountIDsAcrossReload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tickets-accounts.sqlite")

	service, err := NewSQLitePersistentService(path)
	if err != nil {
		t.Fatalf("create sqlite persistent service: %v", err)
	}
	defer func() { _ = service.Close() }()

	first, err := service.EnqueueWithAccount(QueueRated, contracts.MatchModeOpenCards, "guest_a", 1200, "Alpha", "acct_alpha")
	if err != nil {
		t.Fatalf("enqueue first ticket: %v", err)
	}
	second, err := service.EnqueueWithAccount(QueueRated, contracts.MatchModeOpenCards, "guest_b", 1210, "Bravo", "acct_bravo")
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
	if firstReloaded.AccountID != "acct_alpha" || secondReloaded.AccountID != "acct_bravo" {
		t.Fatalf("expected sqlite reload to preserve account metadata, got %#v and %#v", firstReloaded, secondReloaded)
	}
}

func TestQueueStatsReflectTicketState(t *testing.T) {
	service := NewService()
	first, err := service.Enqueue(QueueRated, contracts.MatchModeOpenCards, "guest_a", 1200, "Alpha")
	if err != nil {
		t.Fatalf("enqueue first rated ticket: %v", err)
	}
	if _, err := service.Enqueue(QueueRated, contracts.MatchModeOpenCards, "guest_b", 1210, "Bravo"); err != nil {
		t.Fatalf("enqueue second rated ticket: %v", err)
	}
	casual, err := service.Enqueue(QueueCasual, contracts.MatchModeOpenCards, "guest_c", 1190, "Charlie")
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

func TestQueueDoesNotCrossMatchOfficialModes(t *testing.T) {
	service := NewService()

	first, err := service.Enqueue(QueueCasual, contracts.MatchModeOpenCards, "guest_a", 1200, "Alpha")
	if err != nil {
		t.Fatalf("enqueue first ticket: %v", err)
	}
	second, err := service.Enqueue(QueueCasual, contracts.MatchModeHiddenCards, "guest_b", 1210, "Bravo")
	if err != nil {
		t.Fatalf("enqueue second ticket in different mode: %v", err)
	}

	if first.Status != StatusQueued || second.Status != StatusQueued {
		t.Fatalf("expected both tickets to stay queued in separate modes, got %#v and %#v", first, second)
	}
	if service.Snapshot(QueueCasual, contracts.MatchModeOpenCards).QueuedCount != 1 {
		t.Fatalf("expected open-cards queue snapshot to remain isolated")
	}
	if service.Snapshot(QueueCasual, contracts.MatchModeHiddenCards).QueuedCount != 1 {
		t.Fatalf("expected hidden-cards queue snapshot to remain isolated")
	}
}
