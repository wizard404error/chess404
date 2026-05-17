package platform

type ModerationDirectory interface {
	Backend() string
	Close() error
	BlockAccount(blockerAccountID, targetAccountID, reason string) (AccountBlock, error)
	UnblockAccount(blockerAccountID, targetAccountID string) error
	IsBlocked(blockerAccountID, targetAccountID string) bool
	IsBlockedEitherDirection(accountID, otherAccountID string) bool
	SetAccountRestriction(moderatorAccountID, accountID, kind, reason, reportID string) (AccountRestriction, error)
	ClearAccountRestriction(accountID string) error
	GetAccountRestriction(accountID string) (AccountRestriction, bool)
	ListOverview(accountID string) ModerationOverview
	ListAdminOverview(limit int, status string) ModerationAdminOverview
	CreateReport(reporterAccountID, targetAccountID, category, details string) (PlayerReport, error)
	ResolveReport(moderatorAccountID, reportID, action, note string) (PlayerReport, ModerationActionAudit, error)
	Stats() ModerationStoreStats
}
