package platform

import "github.com/chess404/realtime/internal/contracts"

type DirectChallengeDirectory interface {
	Backend() string
	Close() error
	CanCreateChallenge(challengerAccountID, targetAccountID string) error
	CreateChallenge(challengerAccountID, targetAccountID, matchID string, modeID contracts.MatchModeID, clockSeconds int64, challengerSeat string) (DirectChallenge, error)
	GetChallenge(challengeID string) (DirectChallenge, bool)
	RespondToChallenge(targetAccountID, challengeID string, accept bool) (DirectChallenge, error)
	CancelChallenge(challengerAccountID, challengeID string) (DirectChallenge, error)
	PurgePair(accountID, otherAccountID string) error
	ListOverview(accountID string) DirectChallengeOverview
	Stats() DirectChallengeStoreStats
}
