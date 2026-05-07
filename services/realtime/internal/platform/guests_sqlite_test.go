package platform

import (
	"path/filepath"
	"testing"
)

func TestSQLiteGuestStoreEnsureGuestPersistsAndReloads(t *testing.T) {
	tempDir := t.TempDir()
	storePath := filepath.Join(tempDir, "guest-profiles.sqlite")
	store, err := NewSQLiteGuestStore(storePath)
	if err != nil {
		t.Fatalf("expected sqlite guest store to initialize, got %v", err)
	}
	defer func() { _ = store.Close() }()

	first, err := store.EnsureGuest("", "")
	if err != nil {
		t.Fatalf("expected guest creation to succeed, got %v", err)
	}
	if first.Guest.GuestID == "" || first.Guest.DisplayName == "" || first.Guest.Rating != 1200 || first.SessionSecret == "" || first.SessionToken == "" || first.ExpiresAt.IsZero() {
		t.Fatalf("unexpected guest profile %#v", first)
	}

	reloaded, err := NewSQLiteGuestStore(storePath)
	if err != nil {
		t.Fatalf("expected sqlite guest store reload to succeed, got %v", err)
	}
	defer func() { _ = reloaded.Close() }()
	same, err := reloaded.EnsureGuest(first.Guest.GuestID, first.SessionSecret)
	if err != nil {
		t.Fatalf("expected guest rehydrate to succeed, got %v", err)
	}
	if same.Guest.GuestID != first.Guest.GuestID || same.Guest.DisplayName != first.Guest.DisplayName || same.SessionSecret != first.SessionSecret || same.SessionToken == "" || same.ExpiresAt.IsZero() {
		t.Fatalf("expected persisted guest to round-trip, got %#v vs %#v", first, same)
	}
}

func TestSQLiteGuestStoreResumeGuestByToken(t *testing.T) {
	tempDir := t.TempDir()
	storePath := filepath.Join(tempDir, "guest-profiles.sqlite")
	store, err := NewSQLiteGuestStore(storePath)
	if err != nil {
		t.Fatalf("expected sqlite guest store to initialize, got %v", err)
	}
	defer func() { _ = store.Close() }()

	first, err := store.EnsureGuest("guest_token", "")
	if err != nil {
		t.Fatalf("expected guest creation to succeed, got %v", err)
	}

	resumed, err := store.ResumeGuestByToken(first.Guest.GuestID, first.SessionToken)
	if err != nil {
		t.Fatalf("expected token resume to succeed, got %v", err)
	}
	if resumed.Guest.GuestID != first.Guest.GuestID || resumed.SessionToken != first.SessionToken || resumed.ExpiresAt.IsZero() {
		t.Fatalf("unexpected resumed guest session %#v", resumed)
	}
}

func TestSQLiteGuestStoreFinalizeMatchIsIdempotent(t *testing.T) {
	tempDir := t.TempDir()
	storePath := filepath.Join(tempDir, "guest-profiles.sqlite")
	store, err := NewSQLiteGuestStore(storePath)
	if err != nil {
		t.Fatalf("expected sqlite guest store to initialize, got %v", err)
	}
	defer func() { _ = store.Close() }()

	white, _ := store.EnsureGuest("guest_white", "")
	black, _ := store.EnsureGuest("guest_black", "")
	updatedWhite, updatedBlack, changed, err := store.FinalizeMatch("room_123", white.Guest.GuestID, black.Guest.GuestID, "white")
	if err != nil {
		t.Fatalf("expected finalize to succeed, got %v", err)
	}
	if !changed || updatedWhite.Rating != 1216 || updatedBlack.Rating != 1184 {
		t.Fatalf("unexpected rating change %#v %#v changed=%v", updatedWhite, updatedBlack, changed)
	}

	repeatWhite, repeatBlack, changedAgain, err := store.FinalizeMatch("room_123", white.Guest.GuestID, black.Guest.GuestID, "white")
	if err != nil {
		t.Fatalf("expected repeated finalize to be harmless, got %v", err)
	}
	if changedAgain || repeatWhite.Rating != 1216 || repeatBlack.Rating != 1184 {
		t.Fatalf("expected repeated finalize to be idempotent, got %#v %#v changed=%v", repeatWhite, repeatBlack, changedAgain)
	}
}

func TestSQLiteGuestStoreStatsReflectProfilesAndRatedResults(t *testing.T) {
	tempDir := t.TempDir()
	storePath := filepath.Join(tempDir, "guest-profiles.sqlite")
	store, err := NewSQLiteGuestStore(storePath)
	if err != nil {
		t.Fatalf("expected sqlite guest store to initialize, got %v", err)
	}
	defer func() { _ = store.Close() }()

	white, _ := store.EnsureGuest("guest_white", "")
	black, _ := store.EnsureGuest("guest_black", "")
	if _, _, _, err := store.FinalizeMatch("room_stats", white.Guest.GuestID, black.Guest.GuestID, "draw"); err != nil {
		t.Fatalf("expected finalize to succeed, got %v", err)
	}

	stats := store.Stats()
	if stats.GuestCount != 2 || stats.FinalizedMatchCount != 1 || stats.RankedPlayers != 2 {
		t.Fatalf("unexpected sqlite guest store stats %#v", stats)
	}
}
