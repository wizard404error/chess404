package platform

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/chess404/realtime/internal/contracts"
)

func TestAccountNotificationStoreLifecycle(t *testing.T) {
	store, err := NewAccountNotificationStore(filepath.Join(t.TempDir(), "notifications.json"))
	if err != nil {
		t.Fatalf("expected notification store to initialize, got %v", err)
	}
	defer func() { _ = store.Close() }()

	first, err := store.CreateNotification("acct_beta", "acct_alpha", AccountNotificationKindFriendRequestReceived, AccountNotificationOptions{
		FriendRequestID: "friendreq_1",
	})
	if err != nil {
		t.Fatalf("expected first notification create to succeed, got %v", err)
	}
	if first.Kind != AccountNotificationKindFriendRequestReceived || first.ReadAt != nil {
		t.Fatalf("unexpected first notification %#v", first)
	}

	second, err := store.CreateNotification("acct_alpha", "acct_beta", AccountNotificationKindDirectChallengeAccepted, AccountNotificationOptions{
		ChallengeID:    "challenge_1",
		MatchID:        "room_1",
		ModeID:         contracts.MatchModeHiddenCards,
		ChallengerSeat: "black",
	})
	if err != nil {
		t.Fatalf("expected second notification create to succeed, got %v", err)
	}

	betaOverview := store.ListOverview("acct_beta", 24)
	if betaOverview.UnreadCount != 1 || len(betaOverview.Notifications) != 1 || betaOverview.Notifications[0].NotificationID != first.NotificationID {
		t.Fatalf("unexpected beta notification overview %#v", betaOverview)
	}

	marked, err := store.MarkRead("acct_beta", first.NotificationID)
	if err != nil {
		t.Fatalf("expected mark read to succeed, got %v", err)
	}
	if marked.ReadAt == nil {
		t.Fatalf("expected read timestamp after mark read, got %#v", marked)
	}

	alphaOverview := store.ListOverview("acct_alpha", 24)
	if alphaOverview.UnreadCount != 1 || len(alphaOverview.Notifications) != 1 || alphaOverview.Notifications[0].NotificationID != second.NotificationID {
		t.Fatalf("unexpected alpha notification overview %#v", alphaOverview)
	}

	if count, err := store.MarkAllRead("acct_alpha"); err != nil {
		t.Fatalf("expected mark all read to succeed, got %v", err)
	} else if count != 1 {
		t.Fatalf("expected one notification marked read, got %d", count)
	}

	if err := store.PurgePair("acct_alpha", "acct_beta"); err != nil {
		t.Fatalf("expected purge pair to succeed, got %v", err)
	}
	postPurgeAlpha := store.ListOverview("acct_alpha", 24)
	postPurgeBeta := store.ListOverview("acct_beta", 24)
	if len(postPurgeAlpha.Notifications) != 0 || len(postPurgeBeta.Notifications) != 0 {
		t.Fatalf("expected purge pair to clear both directions, got alpha=%#v beta=%#v", postPurgeAlpha, postPurgeBeta)
	}
}

func TestAccountNotificationStorePublishesEvents(t *testing.T) {
	store, err := NewAccountNotificationStore(filepath.Join(t.TempDir(), "notifications.json"))
	if err != nil {
		t.Fatalf("expected notification store to initialize, got %v", err)
	}
	defer func() { _ = store.Close() }()

	events, cancel := store.Subscribe("acct_beta", 4)
	defer cancel()

	created, err := store.CreateNotification("acct_beta", "acct_alpha", AccountNotificationKindFriendRequestReceived, AccountNotificationOptions{
		FriendRequestID: "friendreq_live",
	})
	if err != nil {
		t.Fatalf("expected create notification to succeed, got %v", err)
	}

	select {
	case event := <-events:
		if event.Kind != "created" || event.NotificationID != created.NotificationID || event.UnreadCount != 1 {
			t.Fatalf("unexpected create event %#v", event)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected create event to be published")
	}

	if _, err := store.MarkRead("acct_beta", created.NotificationID); err != nil {
		t.Fatalf("expected mark read to succeed, got %v", err)
	}

	select {
	case event := <-events:
		if event.Kind != "read" || event.NotificationID != created.NotificationID || event.UnreadCount != 0 {
			t.Fatalf("unexpected read event %#v", event)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected read event to be published")
	}
}
