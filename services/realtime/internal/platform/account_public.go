package platform

import (
	"sort"
	"time"
)

type AccountSeasonSummary struct {
	SeasonID      string    `json:"seasonId"`
	Label         string    `json:"label"`
	RatingStart   int       `json:"ratingStart"`
	RatingEnd     int       `json:"ratingEnd"`
	PeakRating    int       `json:"peakRating"`
	MatchesPlayed int       `json:"matchesPlayed"`
	Wins          int       `json:"wins"`
	Losses        int       `json:"losses"`
	Draws         int       `json:"draws"`
	NetDelta      int       `json:"netDelta"`
	StartedAt     time.Time `json:"startedAt"`
	LastPlayedAt  time.Time `json:"lastPlayedAt"`
}

type SeasonOption struct {
	SeasonID string `json:"seasonId"`
	Label    string `json:"label"`
}

type PublicAccountProfile struct {
	AccountID      string                `json:"accountId"`
	Handle         string                `json:"handle"`
	PrimaryGuestID string                `json:"primaryGuestId"`
	LinkedGuestIDs []string              `json:"linkedGuestIds"`
	CreatedAt      time.Time             `json:"createdAt"`
	LastSeenAt     time.Time             `json:"lastSeenAt"`
	DisplayName    string                `json:"displayName,omitempty"`
	Rating         int                   `json:"rating"`
	MatchesPlayed  int                   `json:"matchesPlayed"`
	Wins           int                   `json:"wins"`
	Losses         int                   `json:"losses"`
	Draws          int                   `json:"draws"`
	GuestCount     int                   `json:"guestCount"`
	CurrentSeason  *AccountSeasonSummary `json:"currentSeason,omitempty"`
	SelectedSeason *AccountSeasonSummary `json:"selectedSeason,omitempty"`
}

type DetailedPublicAccountProfile struct {
	PublicAccountProfile
	RatingHistory []AccountRatingHistoryEntry `json:"ratingHistory,omitempty"`
	SeasonHistory []AccountSeasonSummary      `json:"seasonHistory,omitempty"`
}

func BuildPublicAccountProfile(account AccountProfile, guests GuestDirectory) PublicAccountProfile {
	return BuildPublicAccountProfileForSeason(account, guests, "")
}

func BuildPublicAccountProfileForSeason(account AccountProfile, guests GuestDirectory, seasonID string) PublicAccountProfile {
	public := PublicAccountProfile{
		AccountID:      account.AccountID,
		Handle:         account.Handle,
		PrimaryGuestID: account.PrimaryGuestID,
		LinkedGuestIDs: append([]string{}, account.LinkedGuestIDs...),
		CreatedAt:      account.CreatedAt,
		LastSeenAt:     account.LastSeenAt,
		DisplayName:    account.Handle,
		Rating:         account.Rating,
		MatchesPlayed:  account.MatchesPlayed,
		Wins:           account.Wins,
		Losses:         account.Losses,
		Draws:          account.Draws,
		GuestCount:     len(account.LinkedGuestIDs),
	}

	var (
		primaryGuest  GuestProfile
		primaryFound  bool
		fallback      GuestProfile
		fallbackFound bool
	)

	seen := make(map[string]struct{}, len(account.LinkedGuestIDs))
	for _, guestID := range account.LinkedGuestIDs {
		if guestID == "" {
			continue
		}
		if _, ok := seen[guestID]; ok {
			continue
		}
		seen[guestID] = struct{}{}

		guest, ok := guests.GetGuest(guestID)
		if !ok {
			continue
		}
		if !fallbackFound {
			fallback = guest
			fallbackFound = true
		}
		if guest.GuestID == account.PrimaryGuestID {
			primaryGuest = guest
			primaryFound = true
		}
		if !accountHasDirectStats(account) {
			public.MatchesPlayed += guest.MatchesPlayed
			public.Wins += guest.Wins
			public.Losses += guest.Losses
			public.Draws += guest.Draws
		}
	}

	switch {
	case primaryFound:
		public.DisplayName = primaryGuest.DisplayName
		if !accountHasDirectStats(account) {
			public.Rating = primaryGuest.Rating
		}
	case fallbackFound:
		public.DisplayName = fallback.DisplayName
		if !accountHasDirectStats(account) {
			public.Rating = fallback.Rating
		}
	}

	if public.Rating <= 0 {
		public.Rating = 1200
	}

	if seasons := BuildAccountSeasonHistory(account); len(seasons) > 0 {
		current := seasons[0]
		public.CurrentSeason = &current
		if seasonID != "" {
			if selected, ok := findAccountSeasonSummary(seasons, seasonID); ok {
				public.SelectedSeason = &selected
			}
		}
	}

	return public
}

func BuildPublicAccountProfiles(accounts []AccountProfile, guests GuestDirectory) []PublicAccountProfile {
	items := make([]PublicAccountProfile, 0, len(accounts))
	for _, account := range accounts {
		items = append(items, BuildPublicAccountProfile(account, guests))
	}
	return items
}

func BuildDetailedPublicAccountProfile(account AccountProfile, guests GuestDirectory) DetailedPublicAccountProfile {
	return BuildDetailedPublicAccountProfileForSeason(account, guests, "")
}

func BuildDetailedPublicAccountProfileForSeason(account AccountProfile, guests GuestDirectory, seasonID string) DetailedPublicAccountProfile {
	detailed := DetailedPublicAccountProfile{
		PublicAccountProfile: BuildPublicAccountProfileForSeason(account, guests, seasonID),
	}
	if len(account.RatingHistory) > 0 {
		detailed.RatingHistory = append([]AccountRatingHistoryEntry{}, account.RatingHistory...)
	}
	if seasons := BuildAccountSeasonHistory(account); len(seasons) > 0 {
		detailed.SeasonHistory = seasons
	}
	return detailed
}

func AccountHasSeason(account AccountProfile, seasonID string) bool {
	if seasonID == "" {
		return true
	}
	_, ok := findAccountSeasonSummary(BuildAccountSeasonHistory(account), seasonID)
	return ok
}

func BuildAvailableSeasonOptions(accounts []AccountProfile) []SeasonOption {
	seen := make(map[string]SeasonOption)
	for _, account := range accounts {
		for _, season := range BuildAccountSeasonHistory(account) {
			if _, ok := seen[season.SeasonID]; ok {
				continue
			}
			seen[season.SeasonID] = SeasonOption{
				SeasonID: season.SeasonID,
				Label:    season.Label,
			}
		}
	}

	items := make([]SeasonOption, 0, len(seen))
	for _, season := range seen {
		items = append(items, season)
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].SeasonID > items[j].SeasonID
	})
	return items
}

func BuildAccountSeasonHistory(account AccountProfile) []AccountSeasonSummary {
	if len(account.RatingHistory) == 0 {
		return nil
	}

	entries := append([]AccountRatingHistoryEntry{}, account.RatingHistory...)
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].At.Equal(entries[j].At) {
			return entries[i].MatchID < entries[j].MatchID
		}
		return entries[i].At.Before(entries[j].At)
	})

	itemsBySeason := make(map[string]*AccountSeasonSummary)
	seasonOrder := make([]string, 0, len(entries))

	for _, entry := range entries {
		playedAt := entry.At.UTC()
		seasonID := accountSeasonIDForTime(playedAt)
		summary, ok := itemsBySeason[seasonID]
		if !ok {
			summary = &AccountSeasonSummary{
				SeasonID:      seasonID,
				Label:         accountSeasonLabelForTime(playedAt),
				RatingStart:   entry.RatingBefore,
				RatingEnd:     entry.RatingAfter,
				PeakRating:    maxInt(entry.RatingBefore, entry.RatingAfter),
				StartedAt:     playedAt,
				LastPlayedAt:  playedAt,
				MatchesPlayed: 0,
			}
			itemsBySeason[seasonID] = summary
			seasonOrder = append(seasonOrder, seasonID)
		}

		summary.MatchesPlayed++
		summary.RatingEnd = entry.RatingAfter
		summary.LastPlayedAt = playedAt
		summary.PeakRating = maxInt(summary.PeakRating, entry.RatingAfter)
		summary.NetDelta += entry.Delta
		switch entry.Result {
		case "win":
			summary.Wins++
		case "loss":
			summary.Losses++
		default:
			summary.Draws++
		}
	}

	summaries := make([]AccountSeasonSummary, 0, len(seasonOrder))
	for _, seasonID := range seasonOrder {
		if summary := itemsBySeason[seasonID]; summary != nil {
			summaries = append(summaries, *summary)
		}
	}
	sort.Slice(summaries, func(i, j int) bool {
		if summaries[i].LastPlayedAt.Equal(summaries[j].LastPlayedAt) {
			return summaries[i].SeasonID > summaries[j].SeasonID
		}
		return summaries[i].LastPlayedAt.After(summaries[j].LastPlayedAt)
	})
	return summaries
}

func SortPublicAccountsByRating(items []PublicAccountProfile) {
	sort.Slice(items, func(i, j int) bool {
		return sortPublicAccountFallback(items[i], items[j])
	})
}

func SortPublicAccountsBySelectedSeason(items []PublicAccountProfile) {
	sort.Slice(items, func(i, j int) bool {
		left := items[i].SelectedSeason
		right := items[j].SelectedSeason
		if left == nil || right == nil {
			if left == nil && right == nil {
				return sortPublicAccountFallback(items[i], items[j])
			}
			return left != nil
		}
		if left.RatingEnd == right.RatingEnd {
			if left.MatchesPlayed == right.MatchesPlayed {
				if left.LastPlayedAt.Equal(right.LastPlayedAt) {
					return sortPublicAccountFallback(items[i], items[j])
				}
				return left.LastPlayedAt.After(right.LastPlayedAt)
			}
			return left.MatchesPlayed > right.MatchesPlayed
		}
		return left.RatingEnd > right.RatingEnd
	})
}

func accountSeasonIDForTime(playedAt time.Time) string {
	return playedAt.UTC().Format("2006-01")
}

func accountSeasonLabelForTime(playedAt time.Time) string {
	return playedAt.UTC().Format("Jan 2006")
}

func findAccountSeasonSummary(summaries []AccountSeasonSummary, seasonID string) (AccountSeasonSummary, bool) {
	for _, summary := range summaries {
		if summary.SeasonID == seasonID {
			return summary, true
		}
	}
	return AccountSeasonSummary{}, false
}

func sortPublicAccountFallback(left, right PublicAccountProfile) bool {
	if left.Rating == right.Rating {
		if left.MatchesPlayed == right.MatchesPlayed {
			if left.LastSeenAt.Equal(right.LastSeenAt) {
				return left.CreatedAt.After(right.CreatedAt)
			}
			return left.LastSeenAt.After(right.LastSeenAt)
		}
		return left.MatchesPlayed > right.MatchesPlayed
	}
	return left.Rating > right.Rating
}
