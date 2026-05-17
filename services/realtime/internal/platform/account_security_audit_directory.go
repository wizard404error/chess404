package platform

type AccountSecurityAuditDirectory interface {
	Backend() string
	Close() error
	RecordEvent(request AccountSecurityEventRequest) (AccountSecurityEvent, error)
	ListOverview(accountID string, limit int) AccountSecurityEventOverview
	Stats() AccountSecurityAuditStats
}
