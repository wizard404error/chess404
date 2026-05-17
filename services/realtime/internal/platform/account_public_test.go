package platform

import (
	"testing"
	"time"

	"github.com/chess404/realtime/internal/contracts"
)

func TestBuildAccountSeasonHistoryGroupsMonthlyProgression(t *testing.T) {
	account := AccountProfile{
		AccountID: "acct_seasonal",
		Handle:    "seasonal_player",
		RatingHistory: []AccountRatingHistoryEntry{
			{
				MatchID:       "m1",
				Result:        "win",
				Delta:         16,
				RatingBefore:  1200,
				RatingAfter:   1216,
				MatchesPlayed: 1,
				At:            time.Date(2026, 4, 2, 9, 0, 0, 0, time.UTC),
			},
			{
				MatchID:       "m2",
				Result:        "draw",
				Delta:         0,
				RatingBefore:  1216,
				RatingAfter:   1216,
				MatchesPlayed: 2,
				At:            time.Date(2026, 4, 12, 10, 0, 0, 0, time.UTC),
			},
			{
				MatchID:       "m3",
				Result:        "loss",
				Delta:         -16,
				RatingBefore:  1216,
				RatingAfter:   1200,
				MatchesPlayed: 3,
				At:            time.Date(2026, 5, 1, 11, 0, 0, 0, time.UTC),
			},
		},
	}

	seasons := BuildAccountSeasonHistory(account)
	if len(seasons) != 2 {
		t.Fatalf("expected two season summaries, got %#v", seasons)
	}
	if seasons[0].SeasonID != "2026-05" || seasons[0].MatchesPlayed != 1 || seasons[0].NetDelta != -16 || seasons[0].RatingStart != 1216 || seasons[0].RatingEnd != 1200 {
		t.Fatalf("unexpected most recent season summary %#v", seasons[0])
	}
	if seasons[1].SeasonID != "2026-04" || seasons[1].MatchesPlayed != 2 || seasons[1].Wins != 1 || seasons[1].Draws != 1 || seasons[1].PeakRating != 1216 || seasons[1].NetDelta != 16 {
		t.Fatalf("unexpected older season summary %#v", seasons[1])
	}
}

func TestBuildDetailedPublicAccountProfileIncludesCurrentSeason(t *testing.T) {
	guests, err := NewGuestStore("")
	if err != nil {
		t.Fatalf("expected in-memory guest store to initialize, got %v", err)
	}

	account := AccountProfile{
		AccountID: "acct_detail",
		Handle:    "detail_player",
		Rating:    1200,
		RatingHistory: []AccountRatingHistoryEntry{
			{
				MatchID:       "m1",
				Result:        "win",
				Delta:         16,
				RatingBefore:  1200,
				RatingAfter:   1216,
				MatchesPlayed: 1,
				At:            time.Date(2026, 5, 3, 12, 0, 0, 0, time.UTC),
			},
		},
	}

	detail := BuildDetailedPublicAccountProfile(account, guests)
	if detail.CurrentSeason == nil {
		t.Fatalf("expected current season summary in detailed public account profile")
	}
	if detail.CurrentSeason.SeasonID != "2026-05" || detail.CurrentSeason.NetDelta != 16 || detail.CurrentSeason.RatingEnd != 1216 {
		t.Fatalf("unexpected current season summary %#v", detail.CurrentSeason)
	}
	if len(detail.SeasonHistory) != 1 || detail.SeasonHistory[0].SeasonID != "2026-05" {
		t.Fatalf("expected season history in detailed public account profile, got %#v", detail.SeasonHistory)
	}
}

func TestBuildPublicAccountProfileForSeasonSelectsRequestedSeason(t *testing.T) {
	guests, err := NewGuestStore("")
	if err != nil {
		t.Fatalf("expected in-memory guest store to initialize, got %v", err)
	}

	account := AccountProfile{
		AccountID: "acct_selected",
		Handle:    "selected_player",
		RatingHistory: []AccountRatingHistoryEntry{
			{
				MatchID:       "m1",
				Result:        "win",
				Delta:         16,
				RatingBefore:  1200,
				RatingAfter:   1216,
				MatchesPlayed: 1,
				At:            time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC),
			},
			{
				MatchID:       "m2",
				Result:        "loss",
				Delta:         -16,
				RatingBefore:  1216,
				RatingAfter:   1200,
				MatchesPlayed: 2,
				At:            time.Date(2026, 5, 3, 12, 0, 0, 0, time.UTC),
			},
		},
	}

	profile := BuildPublicAccountProfileForSeason(account, guests, "2026-04")
	if profile.CurrentSeason == nil || profile.CurrentSeason.SeasonID != "2026-05" {
		t.Fatalf("expected current season to stay most recent, got %#v", profile.CurrentSeason)
	}
	if profile.SelectedSeason == nil || profile.SelectedSeason.SeasonID != "2026-04" || profile.SelectedSeason.NetDelta != 16 {
		t.Fatalf("expected requested season selection, got %#v", profile.SelectedSeason)
	}
}

func TestBuildAvailableSeasonOptionsDeduplicatesAndSortsDescending(t *testing.T) {
	accounts := []AccountProfile{
		{
			AccountID: "acct_one",
			RatingHistory: []AccountRatingHistoryEntry{
				{MatchID: "m1", Result: "win", Delta: 16, RatingBefore: 1200, RatingAfter: 1216, MatchesPlayed: 1, At: time.Date(2026, 4, 2, 12, 0, 0, 0, time.UTC)},
				{MatchID: "m2", Result: "draw", Delta: 0, RatingBefore: 1216, RatingAfter: 1216, MatchesPlayed: 2, At: time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)},
			},
		},
		{
			AccountID: "acct_two",
			RatingHistory: []AccountRatingHistoryEntry{
				{MatchID: "m3", Result: "loss", Delta: -16, RatingBefore: 1200, RatingAfter: 1184, MatchesPlayed: 1, At: time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC)},
			},
		},
	}

	options := BuildAvailableSeasonOptions(accounts)
	if len(options) != 2 {
		t.Fatalf("expected two unique season options, got %#v", options)
	}
	if options[0].SeasonID != "2026-05" || options[1].SeasonID != "2026-04" {
		t.Fatalf("expected descending season order, got %#v", options)
	}
}

func TestBuildAccountSeasonHistoryForModeFiltersOtherModes(t *testing.T) {
	account := AccountProfile{
		AccountID: "acct_mode_history",
		RatingHistory: []AccountRatingHistoryEntry{
			{MatchID: "m1", Result: "win", Winner: "white", Delta: 16, Queue: "rated", ModeID: contracts.MatchModeOpenCards, RatingBefore: 1200, RatingAfter: 1216, MatchesPlayed: 1, At: time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)},
			{MatchID: "m2", Result: "loss", Winner: "black", Delta: -16, Queue: "rated", ModeID: contracts.MatchModeHiddenCards, RatingBefore: 1216, RatingAfter: 1200, MatchesPlayed: 2, At: time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC)},
			{MatchID: "m3", Result: "draw", Winner: "draw", Delta: 0, Queue: "rated", ModeID: contracts.MatchModeHiddenCards, RatingBefore: 1200, RatingAfter: 1200, MatchesPlayed: 3, At: time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC)},
		},
	}

	allSeasons := BuildAccountSeasonHistoryForMode(account, "")
	if len(allSeasons) != 2 {
		t.Fatalf("expected all-mode season history to include both months, got %#v", allSeasons)
	}

	hiddenSeasons := BuildAccountSeasonHistoryForMode(account, contracts.MatchModeHiddenCards)
	if len(hiddenSeasons) != 2 {
		t.Fatalf("expected hidden-card season history to include two months, got %#v", hiddenSeasons)
	}
	if hiddenSeasons[0].SeasonID != "2026-05" || hiddenSeasons[0].MatchesPlayed != 1 || hiddenSeasons[1].SeasonID != "2026-04" || hiddenSeasons[1].MatchesPlayed != 1 {
		t.Fatalf("unexpected hidden-card season summaries %#v", hiddenSeasons)
	}
}

func TestBuildPublicAccountProfileForSeasonAndModeScopesStats(t *testing.T) {
	guests, err := NewGuestStore("")
	if err != nil {
		t.Fatalf("expected in-memory guest store to initialize, got %v", err)
	}

	account := AccountProfile{
		AccountID:     "acct_mode_profile",
		Handle:        "mode_profile",
		Rating:        1216,
		MatchesPlayed: 3,
		Wins:          1,
		Losses:        1,
		Draws:         1,
		RatingHistory: []AccountRatingHistoryEntry{
			{MatchID: "m1", Result: "win", Winner: "white", Delta: 16, Queue: "rated", ModeID: contracts.MatchModeOpenCards, RatingBefore: 1200, RatingAfter: 1216, MatchesPlayed: 1, At: time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)},
			{MatchID: "m2", Result: "loss", Winner: "black", Delta: -16, Queue: "rated", ModeID: contracts.MatchModeHiddenCards, RatingBefore: 1216, RatingAfter: 1200, MatchesPlayed: 2, At: time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC)},
			{MatchID: "m3", Result: "draw", Winner: "draw", Delta: 0, Queue: "rated", ModeID: contracts.MatchModeHiddenCards, RatingBefore: 1200, RatingAfter: 1200, MatchesPlayed: 3, At: time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC)},
		},
	}

	profile := BuildPublicAccountProfileForSeasonAndMode(account, guests, "", contracts.MatchModeHiddenCards)
	if profile.MatchesPlayed != 2 || profile.Wins != 0 || profile.Losses != 1 || profile.Draws != 1 {
		t.Fatalf("expected hidden-card stats only, got %#v", profile)
	}
	if profile.Rating != 1200 {
		t.Fatalf("expected hidden-card rating to come from filtered history, got %#v", profile)
	}
	if profile.CurrentSeason == nil || profile.CurrentSeason.SeasonID != "2026-05" {
		t.Fatalf("expected hidden-card current season to come from hidden-card history, got %#v", profile.CurrentSeason)
	}
}

func TestBuildAvailableSeasonOptionsForModeOnlyReturnsRelevantSeasons(t *testing.T) {
	accounts := []AccountProfile{
		{
			AccountID: "acct_mode_one",
			RatingHistory: []AccountRatingHistoryEntry{
				{MatchID: "m1", Result: "win", Delta: 16, Queue: "rated", ModeID: contracts.MatchModeOpenCards, RatingBefore: 1200, RatingAfter: 1216, MatchesPlayed: 1, At: time.Date(2026, 4, 2, 12, 0, 0, 0, time.UTC)},
				{MatchID: "m2", Result: "draw", Delta: 0, Queue: "rated", ModeID: contracts.MatchModeHiddenCards, RatingBefore: 1216, RatingAfter: 1216, MatchesPlayed: 2, At: time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)},
			},
		},
		{
			AccountID: "acct_mode_two",
			RatingHistory: []AccountRatingHistoryEntry{
				{MatchID: "m3", Result: "loss", Delta: -16, Queue: "rated", ModeID: contracts.MatchModeHiddenCards, RatingBefore: 1200, RatingAfter: 1184, MatchesPlayed: 1, At: time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC)},
			},
		},
	}

	options := BuildAvailableSeasonOptionsForMode(accounts, contracts.MatchModeOpenCards)
	if len(options) != 1 || options[0].SeasonID != "2026-04" {
		t.Fatalf("expected only open-card season options, got %#v", options)
	}
}

func TestBuildPublicAccountPresenceWindows(t *testing.T) {
	now := time.Now().UTC()

	online := buildPublicAccountPresenceAt(AccountProfile{
		AccountID:    "acct_online",
		LastActiveAt: now.Add(-45 * time.Second),
		LastSeenAt:   now.Add(-45 * time.Second),
	}, now)
	if online.Status != AccountPresenceOnline || !online.Online || !online.RecentlyActive {
		t.Fatalf("expected online presence, got %#v", online)
	}

	recent := buildPublicAccountPresenceAt(AccountProfile{
		AccountID:    "acct_recent",
		LastActiveAt: now.Add(-8 * time.Minute),
		LastSeenAt:   now.Add(-8 * time.Minute),
	}, now)
	if recent.Status != AccountPresenceRecentlyActive || recent.Online || !recent.RecentlyActive {
		t.Fatalf("expected recently active presence, got %#v", recent)
	}

	offline := buildPublicAccountPresenceAt(AccountProfile{
		AccountID:    "acct_offline",
		LastActiveAt: now.Add(-30 * time.Minute),
		LastSeenAt:   now.Add(-30 * time.Minute),
	}, now)
	if offline.Status != AccountPresenceOffline || offline.Online || offline.RecentlyActive {
		t.Fatalf("expected offline presence, got %#v", offline)
	}
}

func TestBuildPublicAccountProfileFallsBackToLastSeenForPresence(t *testing.T) {
	guests, err := NewGuestStore("")
	if err != nil {
		t.Fatalf("expected in-memory guest store to initialize, got %v", err)
	}

	account := AccountProfile{
		AccountID:  "acct_presence_fallback",
		Handle:     "presence_fallback",
		LastSeenAt: time.Now().UTC().Add(-90 * time.Second),
	}

	profile := BuildPublicAccountProfile(account, guests)
	if profile.PresenceStatus != AccountPresenceOnline || !profile.Online {
		t.Fatalf("expected profile presence to fall back to last seen, got %#v", profile)
	}
	if profile.LastActiveAt.IsZero() {
		t.Fatalf("expected profile to expose derived lastActiveAt, got %#v", profile)
	}
}
