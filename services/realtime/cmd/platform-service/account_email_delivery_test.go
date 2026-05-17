package main

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/chess404/realtime/internal/platform"
)

type stubAccountEmailSender struct {
	provider string
	results  []stubAccountEmailSendResult
	calls    int
}

type stubAccountEmailSendResult struct {
	messageID string
	err       error
}

func (s *stubAccountEmailSender) Provider() string { return s.provider }
func (s *stubAccountEmailSender) Enabled() bool    { return true }

func (s *stubAccountEmailSender) Send(ctx context.Context, delivery platform.AccountEmailDelivery) (string, error) {
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}
	if s.calls >= len(s.results) {
		return "", errors.New("unexpected send call")
	}
	result := s.results[s.calls]
	s.calls++
	return result.messageID, result.err
}

func TestAccountEmailDispatcherMarksPreviewDeliveryDelivered(t *testing.T) {
	t.Parallel()

	outbox, err := platform.NewAccountEmailOutboxStore(filepath.Join(t.TempDir(), "outbox.json"))
	if err != nil {
		t.Fatalf("NewAccountEmailOutboxStore error = %v", err)
	}
	defer func() { _ = outbox.Close() }()

	delivery, err := outbox.QueueDelivery(platform.AccountEmailDeliveryRequest{
		AccountID: "acct_alpha",
		Email:     "alpha@example.com",
		Kind:      platform.AccountEmailDeliveryKindEmailVerification,
		Subject:   "Verify",
		TextBody:  "verify",
		HTMLBody:  "<p>verify</p>",
		ActionURL: "https://example.com/auth?auth=verify-email",
	})
	if err != nil {
		t.Fatalf("QueueDelivery error = %v", err)
	}

	sender := &stubAccountEmailSender{
		provider: "preview",
		results: []stubAccountEmailSendResult{
			{messageID: "preview:" + delivery.DeliveryID},
		},
	}
	now := time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)
	dispatcher := newAccountEmailDispatcher(outbox, sender, func() time.Time { return now })
	dispatcher.processBatch(context.Background())

	overview := outbox.ListOverview("acct_alpha", 8)
	if len(overview.Deliveries) != 1 {
		t.Fatalf("ListOverview deliveries = %d, want 1", len(overview.Deliveries))
	}
	got := overview.Deliveries[0]
	if got.Status != platform.AccountEmailDeliveryStatusDelivered {
		t.Fatalf("delivery status = %q, want delivered", got.Status)
	}
	if got.AttemptCount != 1 {
		t.Fatalf("delivery attempts = %d, want 1", got.AttemptCount)
	}
	if got.Provider != "preview" {
		t.Fatalf("delivery provider = %q, want preview", got.Provider)
	}
	if sender.calls != 1 {
		t.Fatalf("sender calls = %d, want 1", sender.calls)
	}
}

func TestAccountEmailDispatcherRetriesThenFails(t *testing.T) {
	t.Parallel()

	outbox, err := platform.NewAccountEmailOutboxStore(filepath.Join(t.TempDir(), "outbox.json"))
	if err != nil {
		t.Fatalf("NewAccountEmailOutboxStore error = %v", err)
	}
	defer func() { _ = outbox.Close() }()

	_, err = outbox.QueueDelivery(platform.AccountEmailDeliveryRequest{
		AccountID: "acct_beta",
		Email:     "beta@example.com",
		Kind:      platform.AccountEmailDeliveryKindPasswordReset,
		Subject:   "Reset",
		TextBody:  "reset",
		HTMLBody:  "<p>reset</p>",
		ActionURL: "https://example.com/auth?auth=reset-password",
	})
	if err != nil {
		t.Fatalf("QueueDelivery error = %v", err)
	}

	sender := &stubAccountEmailSender{
		provider: "smtp",
		results: []stubAccountEmailSendResult{
			{err: errors.New("temporary smtp error")},
			{err: errors.New("final smtp error")},
		},
	}
	firstAttempt := time.Date(2026, 5, 16, 12, 30, 0, 0, time.UTC)
	dispatcher := newAccountEmailDispatcher(outbox, sender, func() time.Time { return firstAttempt })
	dispatcher.maxAttempts = 2
	dispatcher.baseRetry = 10 * time.Second
	dispatcher.maxRetry = 10 * time.Second
	dispatcher.processBatch(context.Background())

	overview := outbox.ListOverview("acct_beta", 8)
	if len(overview.Deliveries) != 1 {
		t.Fatalf("ListOverview deliveries = %d, want 1", len(overview.Deliveries))
	}
	if overview.Deliveries[0].Status != platform.AccountEmailDeliveryStatusQueued {
		t.Fatalf("after first attempt status = %q, want queued", overview.Deliveries[0].Status)
	}
	if overview.Deliveries[0].NextAttemptAt == nil {
		t.Fatalf("after first attempt nextAttemptAt is nil")
	}

	secondAttempt := firstAttempt.Add(15 * time.Second)
	dispatcher.now = func() time.Time { return secondAttempt }
	dispatcher.processBatch(context.Background())

	overview = outbox.ListOverview("acct_beta", 8)
	if len(overview.Deliveries) != 1 {
		t.Fatalf("ListOverview deliveries = %d, want 1", len(overview.Deliveries))
	}
	got := overview.Deliveries[0]
	if got.Status != platform.AccountEmailDeliveryStatusFailed {
		t.Fatalf("after second attempt status = %q, want failed", got.Status)
	}
	if got.AttemptCount != 2 {
		t.Fatalf("after second attempt attempts = %d, want 2", got.AttemptCount)
	}
	if sender.calls != 2 {
		t.Fatalf("sender calls = %d, want 2", sender.calls)
	}
}
