package platform

import (
	"path/filepath"
	"testing"

	"github.com/chess404/realtime/internal/contracts"
)

func TestDirectChallengesCreateAcceptDeclineAndCancel(t *testing.T) {
	store, err := NewDirectChallengeStore(filepath.Join(t.TempDir(), "challenges.json"))
	if err != nil {
		t.Fatalf("expected direct challenge store to initialize, got %v", err)
	}
	defer func() { _ = store.Close() }()

	if err := store.CanCreateChallenge("acct-alpha", "acct-alpha"); err != ErrInvalidDirectChallenge {
		t.Fatalf("expected invalid self challenge, got %v", err)
	}

	first, err := store.CreateChallenge("acct-alpha", "acct-beta", "room-1", contracts.MatchModeHiddenCards, 900, "black")
	if err != nil {
		t.Fatalf("expected first challenge create to succeed, got %v", err)
	}
	if first.Status != DirectChallengeStatusPending || first.ModeID != contracts.MatchModeHiddenCards || first.ChallengerSeat != "black" {
		t.Fatalf("unexpected first challenge: %#v", first)
	}

	if _, err := store.CreateChallenge("acct-beta", "acct-alpha", "room-2", contracts.MatchModeOpenCards, 600, "white"); err != ErrDirectChallengeAlreadyExists {
		t.Fatalf("expected reverse pending challenge conflict, got %v", err)
	}

	accepted, err := store.RespondToChallenge("acct-beta", first.ChallengeID, true)
	if err != nil {
		t.Fatalf("expected challenge acceptance to succeed, got %v", err)
	}
	if accepted.Status != DirectChallengeStatusAccepted {
		t.Fatalf("expected accepted status, got %#v", accepted)
	}

	second, err := store.CreateChallenge("acct-alpha", "acct-beta", "room-3", contracts.MatchModeOpenCards, 600, "white")
	if err != nil {
		t.Fatalf("expected second challenge create after acceptance, got %v", err)
	}
	declined, err := store.RespondToChallenge("acct-beta", second.ChallengeID, false)
	if err != nil {
		t.Fatalf("expected challenge decline to succeed, got %v", err)
	}
	if declined.Status != DirectChallengeStatusDeclined {
		t.Fatalf("expected declined status, got %#v", declined)
	}

	third, err := store.CreateChallenge("acct-alpha", "acct-beta", "room-4", contracts.MatchModeOpenCards, 600, "white")
	if err != nil {
		t.Fatalf("expected third challenge create to succeed, got %v", err)
	}
	cancelled, err := store.CancelChallenge("acct-alpha", third.ChallengeID)
	if err != nil {
		t.Fatalf("expected challenge cancellation to succeed, got %v", err)
	}
	if cancelled.Status != DirectChallengeStatusCancelled {
		t.Fatalf("expected cancelled status, got %#v", cancelled)
	}

	if _, err := store.RespondToChallenge("acct-beta", cancelled.ChallengeID, true); err != ErrDirectChallengeNotPending {
		t.Fatalf("expected cancelled challenge to reject acceptance, got %v", err)
	}
}
