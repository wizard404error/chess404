package platform

import "time"

type AccountPresenceStatus string

const (
	AccountPresenceOnline         AccountPresenceStatus = "online"
	AccountPresenceRecentlyActive AccountPresenceStatus = "recently_active"
	AccountPresenceOffline        AccountPresenceStatus = "offline"
)

const (
	accountPresenceOnlineWindow         = 2 * time.Minute
	accountPresenceRecentlyActiveWindow = 15 * time.Minute
)

type PublicAccountPresence struct {
	Status         AccountPresenceStatus `json:"status"`
	Online         bool                  `json:"online"`
	RecentlyActive bool                  `json:"recentlyActive"`
	LastActiveAt   time.Time             `json:"lastActiveAt,omitempty"`
}

func BuildPublicAccountPresence(account AccountProfile) PublicAccountPresence {
	return buildPublicAccountPresenceAt(account, time.Now().UTC())
}

func buildPublicAccountPresenceAt(account AccountProfile, now time.Time) PublicAccountPresence {
	lastActiveAt := resolveAccountLastActiveAt(account)
	if lastActiveAt.IsZero() {
		return PublicAccountPresence{Status: AccountPresenceOffline}
	}

	sinceActive := now.Sub(lastActiveAt)
	switch {
	case sinceActive <= accountPresenceOnlineWindow:
		return PublicAccountPresence{
			Status:         AccountPresenceOnline,
			Online:         true,
			RecentlyActive: true,
			LastActiveAt:   lastActiveAt,
		}
	case sinceActive <= accountPresenceRecentlyActiveWindow:
		return PublicAccountPresence{
			Status:         AccountPresenceRecentlyActive,
			RecentlyActive: true,
			LastActiveAt:   lastActiveAt,
		}
	default:
		return PublicAccountPresence{
			Status:       AccountPresenceOffline,
			LastActiveAt: lastActiveAt,
		}
	}
}

func resolveAccountLastActiveAt(account AccountProfile) time.Time {
	if !account.LastActiveAt.IsZero() {
		return account.LastActiveAt.UTC()
	}
	if !account.LastSeenAt.IsZero() {
		return account.LastSeenAt.UTC()
	}
	return time.Time{}
}

func touchAccountPresence(account *AccountProfile, now time.Time) {
	if account == nil {
		return
	}
	account.LastSeenAt = now.UTC()
	account.LastActiveAt = now.UTC()
}

func nullableTime(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return value.UTC()
}

func nullTimeString(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return timeString(value)
}
