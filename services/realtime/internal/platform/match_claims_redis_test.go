package platform

import (
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
)

func TestRedisMatchClaimStoreRoundTripsClaims(t *testing.T) {
	server, err := miniredis.Run()
	if err != nil {
		t.Fatalf("expected miniredis to start, got %v", err)
	}
	defer server.Close()

	store, err := NewRedisMatchClaimStore("redis://"+server.Addr()+"/0", "claims:test")
	if err != nil {
		t.Fatalf("expected redis claim store to initialize, got %v", err)
	}
	defer func() { _ = store.Close() }()

	claim := MatchSeatClaim{
		MatchID:      "room_live",
		GuestID:      "guest_alpha",
		SeatColor:    "white",
		PlayerID:     "guest_alpha",
		PlayerSecret: "seat_secret_alpha",
		Queue:        "rated",
		Status:       "active",
	}
	if err := store.Put(claim); err != nil {
		t.Fatalf("expected claim put to succeed, got %v", err)
	}

	loaded, ok := store.Get("room_live", "guest_alpha")
	if !ok {
		t.Fatalf("expected claim lookup to succeed")
	}
	if loaded.PlayerSecret != claim.PlayerSecret || loaded.SeatColor != "white" || loaded.ClaimToken == "" {
		t.Fatalf("expected persisted claim to round-trip, got %#v", loaded)
	}
	if loaded.ExpiresAt.IsZero() {
		t.Fatalf("expected persisted claim to have an expiry, got %#v", loaded)
	}

	tokenClaim, ok := store.GetByToken("room_live", loaded.ClaimToken)
	if !ok || tokenClaim.GuestID != "guest_alpha" {
		t.Fatalf("expected token lookup to succeed, got %#v %#v", ok, tokenClaim)
	}
	if _, ok := store.Get("room_live", "guest_alpha"); ok {
		t.Fatalf("expected token lookup to consume the stored claim")
	}

	reloaded, err := NewRedisMatchClaimStore("redis://"+server.Addr()+"/0", "claims:test")
	if err != nil {
		t.Fatalf("expected redis claim store reload to succeed, got %v", err)
	}
	defer func() { _ = reloaded.Close() }()

	if _, ok := reloaded.Get("room_live", "guest_alpha"); ok {
		t.Fatalf("expected consumed claim to stay deleted after reload")
	}
	if reloaded.Stats().CachedClaims != 0 {
		t.Fatalf("expected cached claim stats to reflect stored claim, got %#v", reloaded.Stats())
	}
}

func TestMatchClaimStoreExpiresClaims(t *testing.T) {
	store := NewMatchClaimStoreWithTTL(20 * time.Millisecond)

	if err := store.Put(MatchSeatClaim{
		MatchID:      "room_expire",
		GuestID:      "guest_expire",
		SeatColor:    "white",
		PlayerID:     "guest_expire",
		PlayerSecret: "expire_secret",
	}); err != nil {
		t.Fatalf("expected claim put to succeed, got %v", err)
	}

	time.Sleep(35 * time.Millisecond)

	if _, ok := store.Get("room_expire", "guest_expire"); ok {
		t.Fatalf("expected expired claim to be evicted")
	}
	if store.Stats().CachedClaims != 0 {
		t.Fatalf("expected expired claim stats to be empty, got %#v", store.Stats())
	}
}
