package platform

import "github.com/chess404/realtime/internal/contracts"

type AccountDirectory interface {
	Backend() string
	Close() error
	ClaimGuest(guest GuestProfile, handle string) (AccountSession, error)
	RegisterGuestAccount(guest GuestProfile, handle, email, password string) (AccountSession, error)
	ResumeAccount(accountID, sessionToken string) (AccountSession, error)
	ListAccountSessions(accountID, sessionToken string) (AccountSessionOverview, error)
	RevokeAccountSession(accountID, sessionToken, revokeToken string) error
	RevokeOtherAccountSessions(accountID, sessionToken string) error
	TouchPresence(accountID, sessionToken string) (AccountSession, error)
	EnablePasswordLogin(accountID, sessionToken, email, password string) (AccountSession, error)
	GetAccountAuthOverview(accountID, sessionToken string) (AccountAuthOverview, error)
	StartEmailVerification(accountID, sessionToken string) (AccountEmailVerificationChallenge, error)
	VerifyEmail(accountID, token string) (AccountAuthOverview, error)
	LoginWithPassword(identifier, password string) (AccountSession, error)
	StartPasswordReset(identifier string) (AccountPasswordResetChallenge, error)
	ResetPassword(accountID, token, password string) (AccountSession, error)
	LogoutAccount(accountID, sessionToken string) error
	SyncGuestStats(guest GuestProfile) (AccountProfile, bool, error)
	FinalizeMatch(matchID, whiteAccountID, blackAccountID, winner, queue string, modeID contracts.MatchModeID) (AccountProfile, AccountProfile, bool, error)
	GetAccount(accountID string) (AccountProfile, bool)
	GetAccountByGuest(guestID string) (AccountProfile, bool)
	ListAccounts(limit int) []AccountProfile
	Stats() AccountStoreStats
}
