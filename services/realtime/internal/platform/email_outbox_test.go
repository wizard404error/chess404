package platform

import (
	"path/filepath"
	"testing"
	"time"
)

func TestAccountEmailOutboxStoreRecordsRetryAndDeliveryLifecycle(t *testing.T) {
	t.Parallel()

	store, err := NewAccountEmailOutboxStore(filepath.Join(t.TempDir(), "account-email-outbox.json"))
	if err != nil {
		t.Fatalf("NewAccountEmailOutboxStore error = %v", err)
	}
	defer func() { _ = store.Close() }()

	delivery, err := store.QueueDelivery(AccountEmailDeliveryRequest{
		AccountID: "acct_alpha",
		Email:     "alpha@example.com",
		Kind:      AccountEmailDeliveryKindEmailVerification,
		Subject:   "Verify your Chess404 email",
		TextBody:  "verify text",
		HTMLBody:  "<p>verify</p>",
		ActionURL: "https://chess404.example/auth?auth=verify-email",
	})
	if err != nil {
		t.Fatalf("QueueDelivery error = %v", err)
	}

	now := time.Date(2026, 5, 16, 10, 0, 0, 0, time.UTC)
	pending := store.ListPendingDeliveries(10, now)
	if len(pending) != 1 || pending[0].DeliveryID != delivery.DeliveryID {
		t.Fatalf("ListPendingDeliveries() = %#v, want queued delivery", pending)
	}

	retryAt := now.Add(30 * time.Second)
	delivery, err = store.RecordDeliveryResult(AccountEmailDeliveryResultRequest{
		DeliveryID:    delivery.DeliveryID,
		Provider:      "preview",
		AttemptedAt:   now,
		FailureReason: "temporary preview failure",
		NextAttemptAt: &retryAt,
	})
	if err != nil {
		t.Fatalf("RecordDeliveryResult retry error = %v", err)
	}
	if delivery.Status != AccountEmailDeliveryStatusQueued {
		t.Fatalf("retry status = %q, want queued", delivery.Status)
	}
	if delivery.AttemptCount != 1 {
		t.Fatalf("retry attempts = %d, want 1", delivery.AttemptCount)
	}
	if delivery.NextAttemptAt == nil || !delivery.NextAttemptAt.Equal(retryAt) {
		t.Fatalf("retry nextAttemptAt = %#v, want %s", delivery.NextAttemptAt, retryAt)
	}

	if pending := store.ListPendingDeliveries(10, now.Add(10*time.Second)); len(pending) != 0 {
		t.Fatalf("pending before retry window = %#v, want empty", pending)
	}

	delivery, err = store.RecordDeliveryResult(AccountEmailDeliveryResultRequest{
		DeliveryID:        delivery.DeliveryID,
		Provider:          "preview",
		AttemptedAt:       retryAt.Add(5 * time.Second),
		Delivered:         true,
		ProviderMessageID: "preview:mail_123",
	})
	if err != nil {
		t.Fatalf("RecordDeliveryResult delivered error = %v", err)
	}
	if delivery.Status != AccountEmailDeliveryStatusDelivered {
		t.Fatalf("delivered status = %q, want delivered", delivery.Status)
	}
	if delivery.AttemptCount != 2 {
		t.Fatalf("delivered attempts = %d, want 2", delivery.AttemptCount)
	}
	if delivery.DeliveredAt == nil {
		t.Fatalf("DeliveredAt is nil")
	}
	if pending := store.ListPendingDeliveries(10, retryAt.Add(time.Minute)); len(pending) != 0 {
		t.Fatalf("pending after delivery = %#v, want empty", pending)
	}

	stats := store.Stats()
	if stats.DeliveryCount != 1 || stats.QueuedCount != 0 || stats.DeliveredCount != 1 || stats.FailedCount != 0 {
		t.Fatalf("Stats() = %#v", stats)
	}
}

func TestAccountEmailOutboxStoreMarksTerminalFailures(t *testing.T) {
	t.Parallel()

	store, err := NewAccountEmailOutboxStore(filepath.Join(t.TempDir(), "account-email-outbox.json"))
	if err != nil {
		t.Fatalf("NewAccountEmailOutboxStore error = %v", err)
	}
	defer func() { _ = store.Close() }()

	delivery, err := store.QueueDelivery(AccountEmailDeliveryRequest{
		AccountID: "acct_beta",
		Email:     "beta@example.com",
		Kind:      AccountEmailDeliveryKindPasswordReset,
		Subject:   "Reset your password",
		TextBody:  "reset text",
		HTMLBody:  "<p>reset</p>",
		ActionURL: "https://chess404.example/auth?auth=reset-password",
	})
	if err != nil {
		t.Fatalf("QueueDelivery error = %v", err)
	}

	attemptedAt := time.Date(2026, 5, 16, 11, 0, 0, 0, time.UTC)
	delivery, err = store.RecordDeliveryResult(AccountEmailDeliveryResultRequest{
		DeliveryID:      delivery.DeliveryID,
		Provider:        "smtp",
		AttemptedAt:     attemptedAt,
		FailureReason:   "550 mailbox unavailable",
		TerminalFailure: true,
	})
	if err != nil {
		t.Fatalf("RecordDeliveryResult terminal error = %v", err)
	}
	if delivery.Status != AccountEmailDeliveryStatusFailed {
		t.Fatalf("terminal status = %q, want failed", delivery.Status)
	}
	if delivery.AttemptCount != 1 {
		t.Fatalf("terminal attempts = %d, want 1", delivery.AttemptCount)
	}
	if delivery.FailedAt == nil || !delivery.FailedAt.Equal(attemptedAt) {
		t.Fatalf("FailedAt = %#v, want %s", delivery.FailedAt, attemptedAt)
	}
	if pending := store.ListPendingDeliveries(10, attemptedAt.Add(time.Hour)); len(pending) != 0 {
		t.Fatalf("pending after terminal failure = %#v, want empty", pending)
	}

	stats := store.Stats()
	if stats.DeliveryCount != 1 || stats.QueuedCount != 0 || stats.DeliveredCount != 0 || stats.FailedCount != 1 {
		t.Fatalf("Stats() = %#v", stats)
	}
}
