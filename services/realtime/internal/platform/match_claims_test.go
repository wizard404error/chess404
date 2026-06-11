package platform

import (
	"testing"
	"time"

	"github.com/chess404/realtime/internal/contracts"
)

func TestMatchClaimStoreFindByGuestReturnsNewestClaim(t *testing.T) {
	store := NewMatchClaimStoreWithTTL(2 * time.Hour)

	first := MatchSeatClaim{
		MatchID:      "room_one",
		GuestID:      "guest_alpha",
		SeatColor:    "white",
		PlayerID:     "guest_alpha",
		PlayerSecret: "secret_one",
		Queue:        "direct",
		ModeID:       contracts.MatchModeOpenCards,
	}
	if err := store.Put(first); err != nil {
		t.Fatalf("store first claim: %v", err)
	}

	time.Sleep(5 * time.Millisecond)

	second := MatchSeatClaim{
		MatchID:      "room_two",
		GuestID:      "guest_alpha",
		SeatColor:    "black",
		PlayerID:     "guest_alpha",
		PlayerSecret: "secret_two",
		Queue:        "rated",
		ModeID:       contracts.MatchModeHiddenCards,
	}
	if err := store.Put(second); err != nil {
		t.Fatalf("store second claim: %v", err)
	}

	found, ok := store.FindByGuest("guest_alpha")
	if !ok {
		t.Fatalf("expected active claim for guest")
	}
	if found.MatchID != "room_two" || found.PlayerSecret != "secret_two" || found.SeatColor != "black" {
		t.Fatalf("expected newest guest claim, got %#v", found)
	}
}
