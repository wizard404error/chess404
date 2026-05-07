package platform

import (
	"testing"
	"time"
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
