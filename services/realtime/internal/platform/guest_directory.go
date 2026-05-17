package platform

type GuestDirectory interface {
	Backend() string
	Close() error
	EnsureGuest(guestID, sessionSecret string) (GuestSession, error)
	IssueGuestSession(guestID string) (GuestSession, error)
	ResumeGuest(guestID, sessionSecret string) (GuestSession, error)
	ResumeGuestByToken(guestID, sessionToken string) (GuestSession, error)
	FinalizeMatch(matchID, whiteGuestID, blackGuestID, winner string) (GuestProfile, GuestProfile, bool, error)
	ListGuests(limit int) []GuestProfile
	GetGuest(guestID string) (GuestProfile, bool)
	ListRecentGuests(limit int) []GuestProfile
	Stats() GuestStoreStats
}
