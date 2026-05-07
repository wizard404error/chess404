package platform

type AccountDirectory interface {
	Backend() string
	Close() error
	ClaimGuest(guest GuestProfile, handle string) (AccountSession, error)
	ResumeAccount(accountID, sessionToken string) (AccountSession, error)
	SyncGuestStats(guest GuestProfile) (AccountProfile, bool, error)
	FinalizeMatch(matchID, whiteAccountID, blackAccountID, winner string) (AccountProfile, AccountProfile, bool, error)
	GetAccount(accountID string) (AccountProfile, bool)
	GetAccountByGuest(guestID string) (AccountProfile, bool)
	ListAccounts(limit int) []AccountProfile
	Stats() AccountStoreStats
}
