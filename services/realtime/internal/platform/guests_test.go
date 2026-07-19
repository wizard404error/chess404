package platform

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestGuestStoreEnsureGuestPersistsAndReloads(t *testing.T) {
	tempDir := t.TempDir()
	storePath := filepath.Join(tempDir, "guest-profiles.json")
	store, err := NewGuestStore(storePath)
	if err != nil {
		t.Fatalf("expected guest store to initialize, got %v", err)
	}

	first, err := store.EnsureGuest("", "")
	if err != nil {
		t.Fatalf("expected guest creation to succeed, got %v", err)
	}
	if first.Guest.GuestID == "" || first.Guest.DisplayName == "" || first.Guest.Rating != 1200 || first.SessionSecret == "" {
		t.Fatalf("unexpected guest profile %#v", first)
	}

	reloaded, err := NewGuestStore(storePath)
	if err != nil {
		t.Fatalf("expected guest store reload to succeed, got %v", err)
	}
	same, err := reloaded.EnsureGuest(first.Guest.GuestID, first.SessionSecret)
	if err != nil {
		t.Fatalf("expected guest rehydrate to succeed, got %v", err)
	}
	if same.Guest.GuestID != first.Guest.GuestID || same.Guest.DisplayName != first.Guest.DisplayName || same.SessionSecret != first.SessionSecret {
		t.Fatalf("expected persisted guest to round-trip, got %#v vs %#v", first, same)
	}
}

func TestGuestStoreRejectsWrongSessionSecret(t *testing.T) {
	tempDir := t.TempDir()
	storePath := filepath.Join(tempDir, "guest-profiles.json")
	store, err := NewGuestStore(storePath)
	if err != nil {
		t.Fatalf("expected guest store to initialize, got %v", err)
	}

	session, err := store.EnsureGuest("guest_secure", "")
	if err != nil {
		t.Fatalf("expected guest creation to succeed, got %v", err)
	}

	if _, err := store.EnsureGuest(session.Guest.GuestID, "wrong-secret"); !errors.Is(err, ErrUnauthorizedGuestSession) {
		t.Fatalf("expected wrong session secret to be rejected, got %v", err)
	}
}

func TestGuestStoreFinalizeMatchIsIdempotent(t *testing.T) {
	tempDir := t.TempDir()
	storePath := filepath.Join(tempDir, "guest-profiles.json")
	store, err := NewGuestStore(storePath)
	if err != nil {
		t.Fatalf("expected guest store to initialize, got %v", err)
	}

	white, _ := store.EnsureGuest("guest_white", "")
	black, _ := store.EnsureGuest("guest_black", "")
	updatedWhite, updatedBlack, changed, err := store.FinalizeMatch("room_123", white.Guest.GuestID, black.Guest.GuestID, "white")
	if err != nil {
		t.Fatalf("expected finalize to succeed, got %v", err)
	}
	if !changed || updatedWhite.Rating != 1232 || updatedBlack.Rating != 1168 {
		t.Fatalf("unexpected rating change %#v %#v changed=%v", updatedWhite, updatedBlack, changed)
	}
	if updatedWhite.Wins != 1 || updatedWhite.MatchesPlayed != 1 || updatedBlack.Losses != 1 || updatedBlack.MatchesPlayed != 1 {
		t.Fatalf("expected match stats to update, got %#v %#v", updatedWhite, updatedBlack)
	}

	repeatWhite, repeatBlack, changedAgain, err := store.FinalizeMatch("room_123", white.Guest.GuestID, black.Guest.GuestID, "white")
	if err != nil {
		t.Fatalf("expected repeated finalize to be harmless, got %v", err)
	}
	if changedAgain || repeatWhite.Rating != 1232 || repeatBlack.Rating != 1168 {
		t.Fatalf("expected repeated finalize to be idempotent, got %#v %#v changed=%v", repeatWhite, repeatBlack, changedAgain)
	}
}

func TestGuestStoreListGuestsByRating(t *testing.T) {
	tempDir := t.TempDir()
	storePath := filepath.Join(tempDir, "guest-profiles.json")
	store, err := NewGuestStore(storePath)
	if err != nil {
		t.Fatalf("expected guest store to initialize, got %v", err)
	}

	alpha, _ := store.EnsureGuest("guest_alpha", "")
	bravo, _ := store.EnsureGuest("guest_bravo", "")
	charlie, _ := store.EnsureGuest("guest_charlie", "")

	if _, _, _, err := store.FinalizeMatch("room_1", alpha.Guest.GuestID, bravo.Guest.GuestID, "white"); err != nil {
		t.Fatalf("expected first finalize to succeed, got %v", err)
	}
	if _, _, _, err := store.FinalizeMatch("room_2", charlie.Guest.GuestID, bravo.Guest.GuestID, "white"); err != nil {
		t.Fatalf("expected second finalize to succeed, got %v", err)
	}

	leaders := store.ListGuests(2)
	if len(leaders) != 2 {
		t.Fatalf("expected 2 leaders, got %d", len(leaders))
	}
	if leaders[0].GuestID != charlie.Guest.GuestID && leaders[0].GuestID != alpha.Guest.GuestID {
		t.Fatalf("unexpected leader ordering %#v", leaders)
	}
	if leaders[1].Rating > leaders[0].Rating {
		t.Fatalf("expected descending rating order, got %#v", leaders)
	}
}

func TestGuestStoreListRecentGuests(t *testing.T) {
	tempDir := t.TempDir()
	storePath := filepath.Join(tempDir, "guest-profiles.json")
	store, err := NewGuestStore(storePath)
	if err != nil {
		t.Fatalf("expected guest store to initialize, got %v", err)
	}

	first, _ := store.EnsureGuest("guest_first", "")
	second, _ := store.EnsureGuest("guest_second", "")
	if _, err := store.EnsureGuest(first.Guest.GuestID, first.SessionSecret); err != nil {
		t.Fatalf("expected guest touch to succeed, got %v", err)
	}

	recent := store.ListRecentGuests(2)
	if len(recent) != 2 {
		t.Fatalf("expected 2 recent guests, got %d", len(recent))
	}
	if recent[0].GuestID != first.Guest.GuestID && recent[0].GuestID != second.Guest.GuestID {
		t.Fatalf("unexpected recent guest ordering %#v", recent)
	}
}

func TestGuestStoreGetGuest(t *testing.T) {
	tempDir := t.TempDir()
	storePath := filepath.Join(tempDir, "guest-profiles.json")
	store, err := NewGuestStore(storePath)
	if err != nil {
		t.Fatalf("expected guest store to initialize, got %v", err)
	}

	created, err := store.EnsureGuest("guest_lookup", "")
	if err != nil {
		t.Fatalf("expected guest creation to succeed, got %v", err)
	}

	found, ok := store.GetGuest(created.Guest.GuestID)
	if !ok {
		t.Fatalf("expected guest lookup to succeed")
	}
	if found.GuestID != created.Guest.GuestID || found.DisplayName != created.Guest.DisplayName {
		t.Fatalf("unexpected guest lookup result %#v vs %#v", found, created)
	}

	if _, missing := store.GetGuest("guest_missing"); missing {
		t.Fatalf("expected missing guest lookup to fail")
	}
}

func TestGuestStoreStatsReflectProfilesAndRatedResults(t *testing.T) {
	tempDir := t.TempDir()
	storePath := filepath.Join(tempDir, "guest-profiles.json")
	store, err := NewGuestStore(storePath)
	if err != nil {
		t.Fatalf("expected guest store to initialize, got %v", err)
	}

	white, _ := store.EnsureGuest("guest_white", "")
	black, _ := store.EnsureGuest("guest_black", "")
	if _, _, _, err := store.FinalizeMatch("room_stats", white.Guest.GuestID, black.Guest.GuestID, "draw"); err != nil {
		t.Fatalf("expected finalize to succeed, got %v", err)
	}

	stats := store.Stats()
	if stats.GuestCount != 2 || stats.FinalizedMatchCount != 1 || stats.RankedPlayers != 2 {
		t.Fatalf("unexpected guest store stats %#v", stats)
	}
}
