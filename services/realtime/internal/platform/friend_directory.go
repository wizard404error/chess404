package platform

type FriendshipDirectory interface {
	Backend() string
	Close() error
	SendRequest(requesterAccountID, targetAccountID string) (FriendRequest, error)
	RespondToRequest(targetAccountID, requestID string, accept bool) (FriendRequest, error)
	RemoveFriend(accountID, friendAccountID string) error
	ListOverview(accountID string) FriendshipOverview
	AreFriends(accountID, friendAccountID string) bool
	Stats() FriendshipStoreStats
}
