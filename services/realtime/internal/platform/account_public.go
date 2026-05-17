package platform

import (
	"sort"
	"strings"
	"time"

	"github.com/chess404/realtime/internal/contracts"
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

type AccountLeaderboardSpotlight struct {
	AccountID     string `json:"accountId"`
	Handle        string `json:"handle"`
	DisplayName   string `json:"displayName"`
	Rating        int    `json:"rating"`
	PeakRating    int    `json:"peakRating"`
	NetDelta      int    `json:"netDelta"`
	MatchesPlayed int    `json:"matchesPlayed"`
	Wins          int    `json:"wins"`
	Losses        int    `json:"losses"`
	Draws         int    `json:"draws"`
}

type AccountLeaderboardSummary struct {
	ModeID         contracts.MatchModeID        `json:"modeId,omitempty"`
	SeasonID       string                       `json:"seasonId,omitempty"`
	SeasonLabel    string                       `json:"seasonLabel,omitempty"`
	PlayerCount    int                          `json:"playerCount"`
	MatchCount     int                          `json:"matchCount"`
	Leader         *AccountLeaderboardSpotlight `json:"leader,omitempty"`
	BiggestClimber *AccountLeaderboardSpotlight `json:"biggestClimber,omitempty"`
	HighestPeak    *AccountLeaderboardSpotlight `json:"highestPeak,omitempty"`
	MostActive     *AccountLeaderboardSpotlight `json:"mostActive,omitempty"`
}

type PublicAccountProfile struct {
	AccountID      string                `json:"accountId"`
	Handle         string                `json:"handle"`
	PrimaryGuestID string                `json:"primaryGuestId"`
	LinkedGuestIDs []string              `json:"linkedGuestIds"`
	CreatedAt      time.Time             `json:"createdAt"`
	LastSeenAt     time.Time             `json:"lastSeenAt"`
	LastActiveAt   time.Time             `json:"lastActiveAt,omitempty"`
	PresenceStatus AccountPresenceStatus `json:"presenceStatus,omitempty"`
	Online         bool                  `json:"online"`
	RecentlyActive bool                  `json:"recentlyActive"`
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
	return BuildPublicAccountProfileForSeasonAndMode(account, guests, "", "")
}

func BuildPublicAccountProfileForSeason(account AccountProfile, guests GuestDirectory, seasonID string) PublicAccountProfile {
	return BuildPublicAccountProfileForSeasonAndMode(account, guests, seasonID, "")
}

func BuildPublicAccountProfileForSeasonAndMode(account AccountProfile, guests GuestDirectory, seasonID string, modeID contracts.MatchModeID) PublicAccountProfile {
	modeID = normalizeOptionalMatchModeID(modeID)
	public := PublicAccountProfile{
		AccountID:      account.AccountID,
		Handle:         account.Handle,
		PrimaryGuestID: account.PrimaryGuestID,
		LinkedGuestIDs: append([]string{}, account.LinkedGuestIDs...),
		CreatedAt:      account.CreatedAt,
		LastSeenAt:     account.LastSeenAt,
		LastActiveAt:   resolveAccountLastActiveAt(account),
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

	presence := buildPublicAccountPresenceAt(account, time.Now().UTC())
	public.PresenceStatus = presence.Status
	public.Online = presence.Online
	public.RecentlyActive = presence.RecentlyActive
	if public.LastActiveAt.IsZero() {
		public.LastActiveAt = presence.LastActiveAt
	}

	if modeID != "" {
		filteredHistory := filterAccountRatingHistoryByMode(account.RatingHistory, modeID)
		if len(filteredHistory) > 0 {
			public.MatchesPlayed = len(filteredHistory)
			public.Wins = 0
			public.Losses = 0
			public.Draws = 0
			for _, entry := range filteredHistory {
				switch entry.Result {
				case "win":
					public.Wins++
				case "loss":
					public.Losses++
				default:
					public.Draws++
				}
			}
			public.Rating = filteredHistory[len(filteredHistory)-1].RatingAfter
		} else {
			public.MatchesPlayed = 0
			public.Wins = 0
			public.Losses = 0
			public.Draws = 0
		}
	}

	if seasons := BuildAccountSeasonHistoryForMode(account, modeID); len(seasons) > 0 {
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
	return BuildDetailedPublicAccountProfileForSeasonAndMode(account, guests, "", "")
}

func BuildDetailedPublicAccountProfileForSeason(account AccountProfile, guests GuestDirectory, seasonID string) DetailedPublicAccountProfile {
	return BuildDetailedPublicAccountProfileForSeasonAndMode(account, guests, seasonID, "")
}

func BuildDetailedPublicAccountProfileForSeasonAndMode(account AccountProfile, guests GuestDirectory, seasonID string, modeID contracts.MatchModeID) DetailedPublicAccountProfile {
	modeID = normalizeOptionalMatchModeID(modeID)
	detailed := DetailedPublicAccountProfile{
		PublicAccountProfile: BuildPublicAccountProfileForSeasonAndMode(account, guests, seasonID, modeID),
	}
	filteredHistory := filterAccountRatingHistoryByMode(account.RatingHistory, modeID)
	if len(filteredHistory) > 0 {
		detailed.RatingHistory = filteredHistory
	}
	if seasons := BuildAccountSeasonHistoryForMode(account, modeID); len(seasons) > 0 {
		detailed.SeasonHistory = seasons
	}
	return detailed
}

func BuildAccountLeaderboardSummary(items []PublicAccountProfile, seasonID string, modeID contracts.MatchModeID) *AccountLeaderboardSummary {
	if len(items) == 0 {
		return nil
	}
	summary := &AccountLeaderboardSummary{
		ModeID:      normalizeOptionalMatchModeID(modeID),
		PlayerCount: len(items),
	}
	seasonLabels := make(map[string]string)
	mixedSeasonIDs := make(map[string]struct{})

	for _, item := range items {
		season := item.SelectedSeason
		if season == nil {
			season = item.CurrentSeason
		}
		spotlight := buildAccountLeaderboardSpotlight(item, season)
		if season != nil {
			summary.MatchCount += season.MatchesPlayed
			if season.SeasonID != "" {
				seasonLabels[season.SeasonID] = season.Label
				mixedSeasonIDs[season.SeasonID] = struct{}{}
			}
		} else {
			summary.MatchCount += item.MatchesPlayed
		}

		if summary.Leader == nil || spotlight.Rating > summary.Leader.Rating || (spotlight.Rating == summary.Leader.Rating && spotlight.MatchesPlayed > summary.Leader.MatchesPlayed) {
			candidate := spotlight
			summary.Leader = &candidate
		}
		if summary.BiggestClimber == nil || spotlight.NetDelta > summary.BiggestClimber.NetDelta || (spotlight.NetDelta == summary.BiggestClimber.NetDelta && spotlight.Rating > summary.BiggestClimber.Rating) {
			candidate := spotlight
			summary.BiggestClimber = &candidate
		}
		if summary.HighestPeak == nil || spotlight.PeakRating > summary.HighestPeak.PeakRating || (spotlight.PeakRating == summary.HighestPeak.PeakRating && spotlight.Rating > summary.HighestPeak.Rating) {
			candidate := spotlight
			summary.HighestPeak = &candidate
		}
		if summary.MostActive == nil || spotlight.MatchesPlayed > summary.MostActive.MatchesPlayed || (spotlight.MatchesPlayed == summary.MostActive.MatchesPlayed && spotlight.Rating > summary.MostActive.Rating) {
			candidate := spotlight
			summary.MostActive = &candidate
		}
	}

	if seasonID != "" {
		summary.SeasonID = seasonID
		if label, ok := seasonLabels[seasonID]; ok {
			summary.SeasonLabel = label
		} else {
			summary.SeasonLabel = seasonID
		}
	} else if len(mixedSeasonIDs) == 1 {
		for id := range mixedSeasonIDs {
			summary.SeasonID = id
			summary.SeasonLabel = seasonLabels[id]
		}
	} else {
		summary.SeasonLabel = "Current ladder"
	}

	return summary
}

func AccountHasSeason(account AccountProfile, seasonID string) bool {
	return AccountHasSeasonForMode(account, seasonID, "")
}

func AccountHasSeasonForMode(account AccountProfile, seasonID string, modeID contracts.MatchModeID) bool {
	if seasonID == "" {
		return true
	}
	_, ok := findAccountSeasonSummary(BuildAccountSeasonHistoryForMode(account, modeID), seasonID)
	return ok
}

func BuildAvailableSeasonOptions(accounts []AccountProfile) []SeasonOption {
	return BuildAvailableSeasonOptionsForMode(accounts, "")
}

func BuildAvailableSeasonOptionsForMode(accounts []AccountProfile, modeID contracts.MatchModeID) []SeasonOption {
	modeID = normalizeOptionalMatchModeID(modeID)
	seen := make(map[string]SeasonOption)
	for _, account := range accounts {
		for _, season := range BuildAccountSeasonHistoryForMode(account, modeID) {
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
	return BuildAccountSeasonHistoryForMode(account, "")
}

func BuildAccountSeasonHistoryForMode(account AccountProfile, modeID contracts.MatchModeID) []AccountSeasonSummary {
	entries := filterAccountRatingHistoryByMode(account.RatingHistory, normalizeOptionalMatchModeID(modeID))
	if len(entries) == 0 {
		return nil
	}
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

func filterAccountRatingHistoryByMode(history []AccountRatingHistoryEntry, modeID contracts.MatchModeID) []AccountRatingHistoryEntry {
	if len(history) == 0 {
		return nil
	}
	normalizedModeID := normalizeOptionalMatchModeID(modeID)
	if normalizedModeID == "" {
		return append([]AccountRatingHistoryEntry{}, history...)
	}
	filtered := make([]AccountRatingHistoryEntry, 0, len(history))
	for _, entry := range history {
		if contracts.NormalizeMatchModeID(string(entry.ModeID)) != normalizedModeID {
			continue
		}
		filtered = append(filtered, entry)
	}
	return filtered
}

func normalizeOptionalMatchModeID(modeID contracts.MatchModeID) contracts.MatchModeID {
	if strings.TrimSpace(string(modeID)) == "" {
		return ""
	}
	return contracts.NormalizeMatchModeID(string(modeID))
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

func buildAccountLeaderboardSpotlight(item PublicAccountProfile, season *AccountSeasonSummary) AccountLeaderboardSpotlight {
	spotlight := AccountLeaderboardSpotlight{
		AccountID:     item.AccountID,
		Handle:        item.Handle,
		DisplayName:   item.DisplayName,
		Rating:        item.Rating,
		PeakRating:    item.Rating,
		NetDelta:      0,
		MatchesPlayed: item.MatchesPlayed,
		Wins:          item.Wins,
		Losses:        item.Losses,
		Draws:         item.Draws,
	}
	if season != nil {
		spotlight.Rating = season.RatingEnd
		spotlight.PeakRating = season.PeakRating
		spotlight.NetDelta = season.NetDelta
		spotlight.MatchesPlayed = season.MatchesPlayed
		spotlight.Wins = season.Wins
		spotlight.Losses = season.Losses
		spotlight.Draws = season.Draws
	}
	return spotlight
}
