package platform

import (
	"path/filepath"
	"testing"
)

func TestFriendshipStoreRequestLifecycle(t *testing.T) {
	store, err := NewFriendshipStore("")
	if err != nil {
		t.Fatalf("expected friendship store to initialize, got %v", err)
	}
	defer func() { _ = store.Close() }()

	request, err := store.SendRequest("acct_white", "acct_black")
	if err != nil {
		t.Fatalf("expected send friend request to succeed, got %v", err)
	}
	if request.Status != FriendRequestStatusPending {
		t.Fatalf("expected pending friend request, got %#v", request)
	}

	whiteOverview := store.ListOverview("acct_white")
	blackOverview := store.ListOverview("acct_black")
	if len(whiteOverview.Outgoing) != 1 || len(blackOverview.Incoming) != 1 {
		t.Fatalf("expected outgoing/incoming request to appear, got white=%#v black=%#v", whiteOverview, blackOverview)
	}

	accepted, err := store.RespondToRequest("acct_black", request.RequestID, true)
	if err != nil {
		t.Fatalf("expected friend request acceptance to succeed, got %v", err)
	}
	if accepted.Status != FriendRequestStatusAccepted {
		t.Fatalf("expected accepted request status, got %#v", accepted)
	}
	if !store.AreFriends("acct_white", "acct_black") {
		t.Fatalf("expected accounts to be friends after acceptance")
	}

	overview := store.ListOverview("acct_white")
	if len(overview.Outgoing) != 0 || len(overview.Incoming) != 0 || len(overview.Friends) != 1 {
		t.Fatalf("expected pending requests cleared after acceptance, got %#v", overview)
	}

	if err := store.RemoveFriend("acct_white", "acct_black"); err != nil {
		t.Fatalf("expected friend removal to succeed, got %v", err)
	}
	if store.AreFriends("acct_white", "acct_black") {
		t.Fatalf("expected friend removal to clear friendship")
	}
}

func TestFriendshipStoreReverseRequestAutoAccepts(t *testing.T) {
	store, err := NewSQLiteFriendshipStore(filepath.Join(t.TempDir(), "friends.sqlite"))
	if err != nil {
		t.Fatalf("expected sqlite friendship store to initialize, got %v", err)
	}
	defer func() { _ = store.Close() }()

	if _, err := store.SendRequest("acct_alpha", "acct_beta"); err != nil {
		t.Fatalf("expected initial request to succeed, got %v", err)
	}
	request, err := store.SendRequest("acct_beta", "acct_alpha")
	if err != nil {
		t.Fatalf("expected reverse request to auto-accept, got %v", err)
	}
	if request.Status != FriendRequestStatusAccepted {
		t.Fatalf("expected reverse request to auto-accept, got %#v", request)
	}
	if !store.AreFriends("acct_alpha", "acct_beta") {
		t.Fatalf("expected accounts to become friends after reverse request")
	}
	overview := store.ListOverview("acct_alpha")
	if len(overview.Friends) != 1 || len(overview.Incoming) != 0 || len(overview.Outgoing) != 0 {
		t.Fatalf("expected only friendship after reverse auto-accept, got %#v", overview)
	}
}
