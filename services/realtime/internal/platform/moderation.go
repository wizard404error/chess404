package platform

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

var ErrInvalidAccountBlock = errors.New("invalid account block")
var ErrAccountBlockNotFound = errors.New("account block not found")
var ErrAccountInteractionBlocked = errors.New("account interaction blocked")
var ErrInvalidAccountRestriction = errors.New("invalid account restriction")
var ErrAccountRestrictionNotFound = errors.New("account restriction not found")
var ErrAccountRestricted = errors.New("account restricted")
var ErrInvalidPlayerReport = errors.New("invalid player report")
var ErrPlayerReportNotFound = errors.New("player report not found")
var ErrInvalidModerationReview = errors.New("invalid moderation review")

const (
	PlayerReportStatusOpen              = "open"
	PlayerReportStatusUnderReview       = "under_review"
	PlayerReportStatusResolvedActioned  = "resolved_actioned"
	PlayerReportStatusResolvedDismissed = "resolved_dismissed"
)

const (
	ModerationActionReview           = "under_review"
	ModerationActionResolveActioned  = "resolved_actioned"
	ModerationActionResolveDismissed = "resolved_dismissed"
)

const (
	AccountRestrictionKindSuspended = "suspended"
	AccountRestrictionKindBanned    = "banned"
)

type PlayerReportCategory string

const (
	PlayerReportCategoryAbuse         PlayerReportCategory = "abuse"
	PlayerReportCategoryHarassment    PlayerReportCategory = "harassment"
	PlayerReportCategorySpam          PlayerReportCategory = "spam"
	PlayerReportCategoryImpersonation PlayerReportCategory = "impersonation"
	PlayerReportCategoryCheating      PlayerReportCategory = "cheating"
	PlayerReportCategoryOther         PlayerReportCategory = "other"
)

type AccountBlock struct {
	BlockID          string    `json:"blockId"`
	BlockerAccountID string    `json:"blockerAccountId"`
	TargetAccountID  string    `json:"targetAccountId"`
	Reason           string    `json:"reason,omitempty"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

type AccountRestriction struct {
	RestrictionID      string    `json:"restrictionId"`
	AccountID          string    `json:"accountId"`
	Kind               string    `json:"kind"`
	Reason             string    `json:"reason,omitempty"`
	ReportID           string    `json:"reportId,omitempty"`
	AppliedByAccountID string    `json:"appliedByAccountId,omitempty"`
	CreatedAt          time.Time `json:"createdAt"`
	UpdatedAt          time.Time `json:"updatedAt"`
}

type PlayerReport struct {
	ReportID            string               `json:"reportId"`
	ReporterAccountID   string               `json:"reporterAccountId"`
	TargetAccountID     string               `json:"targetAccountId"`
	Category            PlayerReportCategory `json:"category"`
	Details             string               `json:"details,omitempty"`
	Status              string               `json:"status"`
	ReviewedByAccountID string               `json:"reviewedByAccountId,omitempty"`
	ReviewedAt          *time.Time           `json:"reviewedAt,omitempty"`
	ResolutionNote      string               `json:"resolutionNote,omitempty"`
	CreatedAt           time.Time            `json:"createdAt"`
	UpdatedAt           time.Time            `json:"updatedAt"`
}

type ModerationActionAudit struct {
	ActionID           string    `json:"actionId"`
	ReportID           string    `json:"reportId"`
	ModeratorAccountID string    `json:"moderatorAccountId"`
	ReporterAccountID  string    `json:"reporterAccountId"`
	TargetAccountID    string    `json:"targetAccountId"`
	PreviousStatus     string    `json:"previousStatus"`
	NextStatus         string    `json:"nextStatus"`
	Action             string    `json:"action"`
	Note               string    `json:"note,omitempty"`
	CreatedAt          time.Time `json:"createdAt"`
}

type ModerationOverview struct {
	OutgoingBlocks   []AccountBlock `json:"outgoingBlocks"`
	IncomingBlocks   []AccountBlock `json:"incomingBlocks"`
	SubmittedReports []PlayerReport `json:"submittedReports"`
}

type ModerationAdminOverview struct {
	Reports            []PlayerReport          `json:"reports"`
	RecentActions      []ModerationActionAudit `json:"recentActions"`
	ActiveRestrictions []AccountRestriction    `json:"activeRestrictions"`
}

type ModerationStoreStats struct {
	BlockCount       int `json:"blockCount"`
	ReportCount      int `json:"reportCount"`
	ActionCount      int `json:"actionCount"`
	RestrictionCount int `json:"restrictionCount"`
}

type moderationPersistence interface {
	backend() string
	load() (map[string]AccountBlock, map[string]PlayerReport, map[string]ModerationActionAudit, map[string]AccountRestriction, error)
	persist(map[string]AccountBlock, map[string]PlayerReport, map[string]ModerationActionAudit, map[string]AccountRestriction) error
	close() error
}

type ModerationStore struct {
	mu           sync.Mutex
	store        moderationPersistence
	blocks       map[string]AccountBlock
	reports      map[string]PlayerReport
	actions      map[string]ModerationActionAudit
	restrictions map[string]AccountRestriction
}

type moderationStoreFile struct {
	Blocks       map[string]AccountBlock          `json:"blocks"`
	Reports      map[string]PlayerReport          `json:"reports"`
	Actions      map[string]ModerationActionAudit `json:"actions"`
	Restrictions map[string]AccountRestriction    `json:"restrictions,omitempty"`
}

type fileModerationStore struct {
	path string
}

func NewModerationStore(path string) (*ModerationStore, error) {
	return newModerationStore(&fileModerationStore{path: path})
}

func newModerationStore(store moderationPersistence) (*ModerationStore, error) {
	blocks, reports, actions, restrictions, err := store.load()
	if err != nil {
		return nil, err
	}
	if blocks == nil {
		blocks = make(map[string]AccountBlock)
	}
	if reports == nil {
		reports = make(map[string]PlayerReport)
	}
	if actions == nil {
		actions = make(map[string]ModerationActionAudit)
	}
	if restrictions == nil {
		restrictions = make(map[string]AccountRestriction)
	}
	return &ModerationStore{
		store:        store,
		blocks:       blocks,
		reports:      reports,
		actions:      actions,
		restrictions: restrictions,
	}, nil
}

func (s *ModerationStore) Backend() string {
	if s == nil || s.store == nil {
		return "memory"
	}
	return s.store.backend()
}

func (s *ModerationStore) Close() error {
	if s == nil || s.store == nil {
		return nil
	}
	return s.store.close()
}

func (s *ModerationStore) Stats() ModerationStoreStats {
	s.mu.Lock()
	defer s.mu.Unlock()

	return ModerationStoreStats{
		BlockCount:       len(s.blocks),
		ReportCount:      len(s.reports),
		ActionCount:      len(s.actions),
		RestrictionCount: len(s.restrictions),
	}
}

func (s *ModerationStore) BlockAccount(blockerAccountID, targetAccountID, reason string) (AccountBlock, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	resolvedBlocker, resolvedTarget, err := normalizeModerationPair(blockerAccountID, targetAccountID)
	if err != nil {
		return AccountBlock{}, err
	}
	now := time.Now().UTC()
	resolvedReason := strings.TrimSpace(reason)
	for blockID, block := range s.blocks {
		if block.BlockerAccountID == resolvedBlocker && block.TargetAccountID == resolvedTarget {
			block.Reason = resolvedReason
			block.UpdatedAt = now
			s.blocks[blockID] = block
			if err := s.persistLocked(); err != nil {
				return AccountBlock{}, err
			}
			return block, nil
		}
	}

	block := AccountBlock{
		BlockID:          "block_" + randomToken(8),
		BlockerAccountID: resolvedBlocker,
		TargetAccountID:  resolvedTarget,
		Reason:           resolvedReason,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	s.blocks[block.BlockID] = block
	if err := s.persistLocked(); err != nil {
		return AccountBlock{}, err
	}
	return block, nil
}

func (s *ModerationStore) UnblockAccount(blockerAccountID, targetAccountID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	resolvedBlocker, resolvedTarget, err := normalizeModerationPair(blockerAccountID, targetAccountID)
	if err != nil {
		return err
	}
	for blockID, block := range s.blocks {
		if block.BlockerAccountID == resolvedBlocker && block.TargetAccountID == resolvedTarget {
			delete(s.blocks, blockID)
			return s.persistLocked()
		}
	}
	return ErrAccountBlockNotFound
}

func (s *ModerationStore) IsBlocked(blockerAccountID, targetAccountID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.isBlockedLocked(blockerAccountID, targetAccountID)
}

func (s *ModerationStore) IsBlockedEitherDirection(accountID, otherAccountID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.isBlockedLocked(accountID, otherAccountID) || s.isBlockedLocked(otherAccountID, accountID)
}

func (s *ModerationStore) SetAccountRestriction(moderatorAccountID, accountID, kind, reason, reportID string) (AccountRestriction, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	resolvedModeratorID := strings.TrimSpace(moderatorAccountID)
	resolvedAccountID := strings.TrimSpace(accountID)
	resolvedKind, ok := normalizeAccountRestrictionKind(kind)
	if resolvedModeratorID == "" || resolvedAccountID == "" || !ok {
		return AccountRestriction{}, ErrInvalidAccountRestriction
	}
	resolvedReason := strings.TrimSpace(reason)
	if len(resolvedReason) > 1000 {
		resolvedReason = resolvedReason[:1000]
	}
	now := time.Now().UTC()
	restriction, exists := s.restrictions[resolvedAccountID]
	if exists {
		restriction.Kind = resolvedKind
		restriction.Reason = resolvedReason
		restriction.ReportID = strings.TrimSpace(reportID)
		restriction.AppliedByAccountID = resolvedModeratorID
		restriction.UpdatedAt = now
	} else {
		restriction = AccountRestriction{
			RestrictionID:      "restrict_" + randomToken(8),
			AccountID:          resolvedAccountID,
			Kind:               resolvedKind,
			Reason:             resolvedReason,
			ReportID:           strings.TrimSpace(reportID),
			AppliedByAccountID: resolvedModeratorID,
			CreatedAt:          now,
			UpdatedAt:          now,
		}
	}
	s.restrictions[resolvedAccountID] = restriction
	if err := s.persistLocked(); err != nil {
		return AccountRestriction{}, err
	}
	return restriction, nil
}

func (s *ModerationStore) ClearAccountRestriction(accountID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	resolvedAccountID := strings.TrimSpace(accountID)
	if resolvedAccountID == "" {
		return ErrInvalidAccountRestriction
	}
	if _, exists := s.restrictions[resolvedAccountID]; !exists {
		return ErrAccountRestrictionNotFound
	}
	delete(s.restrictions, resolvedAccountID)
	return s.persistLocked()
}

func (s *ModerationStore) GetAccountRestriction(accountID string) (AccountRestriction, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	restriction, ok := s.restrictions[strings.TrimSpace(accountID)]
	return restriction, ok
}

func (s *ModerationStore) ListOverview(accountID string) ModerationOverview {
	s.mu.Lock()
	defer s.mu.Unlock()

	resolvedAccountID := strings.TrimSpace(accountID)
	if resolvedAccountID == "" {
		return ModerationOverview{}
	}

	overview := ModerationOverview{
		OutgoingBlocks:   make([]AccountBlock, 0),
		IncomingBlocks:   make([]AccountBlock, 0),
		SubmittedReports: make([]PlayerReport, 0),
	}
	for _, block := range s.blocks {
		switch {
		case block.BlockerAccountID == resolvedAccountID:
			overview.OutgoingBlocks = append(overview.OutgoingBlocks, block)
		case block.TargetAccountID == resolvedAccountID:
			overview.IncomingBlocks = append(overview.IncomingBlocks, block)
		}
	}
	for _, report := range s.reports {
		if report.ReporterAccountID == resolvedAccountID {
			overview.SubmittedReports = append(overview.SubmittedReports, report)
		}
	}
	sort.Slice(overview.OutgoingBlocks, func(i, j int) bool {
		if overview.OutgoingBlocks[i].UpdatedAt.Equal(overview.OutgoingBlocks[j].UpdatedAt) {
			return overview.OutgoingBlocks[i].BlockID < overview.OutgoingBlocks[j].BlockID
		}
		return overview.OutgoingBlocks[i].UpdatedAt.After(overview.OutgoingBlocks[j].UpdatedAt)
	})
	sort.Slice(overview.IncomingBlocks, func(i, j int) bool {
		if overview.IncomingBlocks[i].UpdatedAt.Equal(overview.IncomingBlocks[j].UpdatedAt) {
			return overview.IncomingBlocks[i].BlockID < overview.IncomingBlocks[j].BlockID
		}
		return overview.IncomingBlocks[i].UpdatedAt.After(overview.IncomingBlocks[j].UpdatedAt)
	})
	sort.Slice(overview.SubmittedReports, func(i, j int) bool {
		if overview.SubmittedReports[i].UpdatedAt.Equal(overview.SubmittedReports[j].UpdatedAt) {
			return overview.SubmittedReports[i].ReportID < overview.SubmittedReports[j].ReportID
		}
		return overview.SubmittedReports[i].UpdatedAt.After(overview.SubmittedReports[j].UpdatedAt)
	})
	return overview
}

func (s *ModerationStore) CreateReport(reporterAccountID, targetAccountID, category, details string) (PlayerReport, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	resolvedReporter, resolvedTarget, err := normalizeModerationPair(reporterAccountID, targetAccountID)
	if err != nil {
		return PlayerReport{}, err
	}
	resolvedCategory, ok := normalizePlayerReportCategory(category)
	if !ok {
		return PlayerReport{}, ErrInvalidPlayerReport
	}
	resolvedDetails := strings.TrimSpace(details)
	if len(resolvedDetails) > 1000 {
		resolvedDetails = resolvedDetails[:1000]
	}
	now := time.Now().UTC()
	report := PlayerReport{
		ReportID:          "report_" + randomToken(8),
		ReporterAccountID: resolvedReporter,
		TargetAccountID:   resolvedTarget,
		Category:          resolvedCategory,
		Details:           resolvedDetails,
		Status:            PlayerReportStatusOpen,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	s.reports[report.ReportID] = report
	if err := s.persistLocked(); err != nil {
		return PlayerReport{}, err
	}
	return report, nil
}

func (s *ModerationStore) ListAdminOverview(limit int, status string) ModerationAdminOverview {
	s.mu.Lock()
	defer s.mu.Unlock()

	normalizedStatus := normalizePlayerReportStatus(status)
	reports := make([]PlayerReport, 0, len(s.reports))
	for _, report := range s.reports {
		if normalizedStatus != "" && report.Status != normalizedStatus {
			continue
		}
		reports = append(reports, report)
	}
	sort.Slice(reports, func(i, j int) bool {
		if reports[i].UpdatedAt.Equal(reports[j].UpdatedAt) {
			return reports[i].ReportID < reports[j].ReportID
		}
		return reports[i].UpdatedAt.After(reports[j].UpdatedAt)
	})
	if limit > 0 && len(reports) > limit {
		reports = reports[:limit]
	}

	actions := make([]ModerationActionAudit, 0, len(s.actions))
	for _, action := range s.actions {
		actions = append(actions, action)
	}
	sort.Slice(actions, func(i, j int) bool {
		if actions[i].CreatedAt.Equal(actions[j].CreatedAt) {
			return actions[i].ActionID < actions[j].ActionID
		}
		return actions[i].CreatedAt.After(actions[j].CreatedAt)
	})
	if limit > 0 && len(actions) > limit {
		actions = actions[:limit]
	}

	restrictions := make([]AccountRestriction, 0, len(s.restrictions))
	for _, restriction := range s.restrictions {
		restrictions = append(restrictions, restriction)
	}
	sort.Slice(restrictions, func(i, j int) bool {
		if restrictions[i].UpdatedAt.Equal(restrictions[j].UpdatedAt) {
			return restrictions[i].RestrictionID < restrictions[j].RestrictionID
		}
		return restrictions[i].UpdatedAt.After(restrictions[j].UpdatedAt)
	})
	if limit > 0 && len(restrictions) > limit {
		restrictions = restrictions[:limit]
	}

	return ModerationAdminOverview{
		Reports:            reports,
		RecentActions:      actions,
		ActiveRestrictions: restrictions,
	}
}

func (s *ModerationStore) ResolveReport(moderatorAccountID, reportID, action, note string) (PlayerReport, ModerationActionAudit, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	resolvedModeratorID := strings.TrimSpace(moderatorAccountID)
	resolvedReportID := strings.TrimSpace(reportID)
	nextStatus, ok := normalizeModerationAction(action)
	if resolvedModeratorID == "" || resolvedReportID == "" || !ok {
		return PlayerReport{}, ModerationActionAudit{}, ErrInvalidModerationReview
	}
	report, exists := s.reports[resolvedReportID]
	if !exists {
		return PlayerReport{}, ModerationActionAudit{}, ErrPlayerReportNotFound
	}
	resolvedNote := strings.TrimSpace(note)
	if len(resolvedNote) > 1000 {
		resolvedNote = resolvedNote[:1000]
	}
	now := time.Now().UTC()
	reviewedAt := now
	updatedReport := report
	updatedReport.Status = nextStatus
	updatedReport.ReviewedByAccountID = resolvedModeratorID
	updatedReport.ReviewedAt = &reviewedAt
	updatedReport.ResolutionNote = resolvedNote
	updatedReport.UpdatedAt = now
	s.reports[resolvedReportID] = updatedReport

	audit := ModerationActionAudit{
		ActionID:           "modact_" + randomToken(8),
		ReportID:           updatedReport.ReportID,
		ModeratorAccountID: resolvedModeratorID,
		ReporterAccountID:  updatedReport.ReporterAccountID,
		TargetAccountID:    updatedReport.TargetAccountID,
		PreviousStatus:     report.Status,
		NextStatus:         nextStatus,
		Action:             nextStatus,
		Note:               resolvedNote,
		CreatedAt:          now,
	}
	s.actions[audit.ActionID] = audit
	if err := s.persistLocked(); err != nil {
		return PlayerReport{}, ModerationActionAudit{}, err
	}
	return updatedReport, audit, nil
}

func (s *ModerationStore) isBlockedLocked(blockerAccountID, targetAccountID string) bool {
	resolvedBlocker := strings.TrimSpace(blockerAccountID)
	resolvedTarget := strings.TrimSpace(targetAccountID)
	if resolvedBlocker == "" || resolvedTarget == "" {
		return false
	}
	for _, block := range s.blocks {
		if block.BlockerAccountID == resolvedBlocker && block.TargetAccountID == resolvedTarget {
			return true
		}
	}
	return false
}

func (s *ModerationStore) persistLocked() error {
	return s.store.persist(s.blocks, s.reports, s.actions, s.restrictions)
}

func normalizeModerationPair(accountID, otherAccountID string) (string, string, error) {
	resolvedAccountID := strings.TrimSpace(accountID)
	resolvedOtherAccountID := strings.TrimSpace(otherAccountID)
	if resolvedAccountID == "" || resolvedOtherAccountID == "" || resolvedAccountID == resolvedOtherAccountID {
		return "", "", ErrInvalidAccountBlock
	}
	return resolvedAccountID, resolvedOtherAccountID, nil
}

func normalizePlayerReportCategory(category string) (PlayerReportCategory, bool) {
	switch PlayerReportCategory(strings.ToLower(strings.TrimSpace(category))) {
	case PlayerReportCategoryAbuse:
		return PlayerReportCategoryAbuse, true
	case PlayerReportCategoryHarassment:
		return PlayerReportCategoryHarassment, true
	case PlayerReportCategorySpam:
		return PlayerReportCategorySpam, true
	case PlayerReportCategoryImpersonation:
		return PlayerReportCategoryImpersonation, true
	case PlayerReportCategoryCheating:
		return PlayerReportCategoryCheating, true
	case PlayerReportCategoryOther:
		return PlayerReportCategoryOther, true
	default:
		return "", false
	}
}

func normalizePlayerReportStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case PlayerReportStatusOpen:
		return PlayerReportStatusOpen
	case PlayerReportStatusUnderReview:
		return PlayerReportStatusUnderReview
	case PlayerReportStatusResolvedActioned:
		return PlayerReportStatusResolvedActioned
	case PlayerReportStatusResolvedDismissed:
		return PlayerReportStatusResolvedDismissed
	default:
		return ""
	}
}

func normalizeModerationAction(action string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(action)) {
	case ModerationActionReview, "review":
		return PlayerReportStatusUnderReview, true
	case ModerationActionResolveActioned, "actioned":
		return PlayerReportStatusResolvedActioned, true
	case ModerationActionResolveDismissed, "dismissed":
		return PlayerReportStatusResolvedDismissed, true
	default:
		return "", false
	}
}

func normalizeAccountRestrictionKind(kind string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case AccountRestrictionKindSuspended:
		return AccountRestrictionKindSuspended, true
	case AccountRestrictionKindBanned:
		return AccountRestrictionKindBanned, true
	default:
		return "", false
	}
}

func (s *fileModerationStore) backend() string {
	return "file"
}

func (s *fileModerationStore) load() (map[string]AccountBlock, map[string]PlayerReport, map[string]ModerationActionAudit, map[string]AccountRestriction, error) {
	if s.path == "" {
		return make(map[string]AccountBlock), make(map[string]PlayerReport), make(map[string]ModerationActionAudit), make(map[string]AccountRestriction), nil
	}
	data, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return make(map[string]AccountBlock), make(map[string]PlayerReport), make(map[string]ModerationActionAudit), make(map[string]AccountRestriction), nil
		}
		return nil, nil, nil, nil, err
	}
	var payload moderationStoreFile
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, nil, nil, nil, err
	}
	if payload.Blocks == nil {
		payload.Blocks = make(map[string]AccountBlock)
	}
	if payload.Reports == nil {
		payload.Reports = make(map[string]PlayerReport)
	}
	if payload.Actions == nil {
		payload.Actions = make(map[string]ModerationActionAudit)
	}
	if payload.Restrictions == nil {
		payload.Restrictions = make(map[string]AccountRestriction)
	}
	return payload.Blocks, payload.Reports, payload.Actions, payload.Restrictions, nil
}

func (s *fileModerationStore) persist(blocks map[string]AccountBlock, reports map[string]PlayerReport, actions map[string]ModerationActionAudit, restrictions map[string]AccountRestriction) error {
	if s.path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	payload := moderationStoreFile{
		Blocks:       blocks,
		Reports:      reports,
		Actions:      actions,
		Restrictions: restrictions,
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0o644)
}

func (s *fileModerationStore) close() error {
	return nil
}
