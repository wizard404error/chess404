package platform

type AccountNotificationDirectory interface {
	Backend() string
	Close() error
	CreateNotification(accountID, actorAccountID, kind string, options AccountNotificationOptions) (AccountNotification, error)
	MarkRead(accountID, notificationID string) (AccountNotification, error)
	MarkAllRead(accountID string) (int, error)
	PurgePair(accountID, otherAccountID string) error
	ListOverview(accountID string, limit int) AccountNotificationOverview
	Subscribe(accountID string, buffer int) (<-chan AccountNotificationEvent, func())
	Stats() AccountNotificationStoreStats
}
