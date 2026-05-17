package platform

import "time"

type AccountEmailOutboxDirectory interface {
	Backend() string
	Close() error
	QueueDelivery(request AccountEmailDeliveryRequest) (AccountEmailDelivery, error)
	ListOverview(accountID string, limit int) AccountEmailDeliveryOverview
	ListPendingDeliveries(limit int, now time.Time) []AccountEmailDelivery
	RecordDeliveryResult(request AccountEmailDeliveryResultRequest) (AccountEmailDelivery, error)
	Stats() AccountEmailDeliveryStoreStats
}
