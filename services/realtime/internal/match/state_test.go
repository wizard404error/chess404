package match

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chess404/realtime/internal/contracts"
	"github.com/chess404/realtime/internal/platform"
)

type captureArchiver struct {
	snapshots []contracts.MatchSnapshotResponse
}

func (a *captureArchiver) Upsert(snapshot contracts.MatchSnapshotResponse) error {
	a.snapshots = append(a.snapshots, snapshot)
	return nil
}

func TestCreateMatchStartsActive(t *testing.T) {
	service := NewService()
	now := time.Date(2026, 5, 5, 8, 0, 0, 0, time.UTC)

	snapshot := service.CreateMatch(contracts.CreateMatchRequest{
		MatchID:      "test_match",
		ClockSeconds: 120,
	}, now)

	if snapshot.Match.MatchID != "test_match" {
		t.Fatalf("expected match id to be preserved, got %q", snapshot.Match.MatchID)
	}
	if snapshot.Match.Status != "active" {
		t.Fatalf("expected active match, got %q", snapshot.Match.Status)
	}
	if snapshot.Match.FullMoveNum != 1 {
		t.Fatalf("expected fullmove 1, got %d", snapshot.Match.FullMoveNum)
	}
	if snapshot.Match.Clock.WhiteMS != 120000 || snapshot.Match.Clock.BlackMS != 120000 {
		t.Fatalf("expected 120s clocks, got white=%d black=%d", snapshot.Match.Clock.WhiteMS, snapshot.Match.Clock.BlackMS)
	}
}

func TestCreateMatchPreservesModeID(t *testing.T) {
	service := NewService()
	now := time.Date(2026, 5, 5, 8, 0, 30, 0, time.UTC)

	snapshot := service.CreateMatch(contracts.CreateMatchRequest{
		MatchID: "mode_match",
		ModeID:  contracts.MatchModeHiddenCards,
	}, now)

	if snapshot.Match.ModeID != contracts.MatchModeHiddenCards {
		t.Fatalf("expected match mode to be preserved, got %#v", snapshot.Match.ModeID)
	}
}

func TestCreateMatchWithSingleSeatWaitsForOpponent(t *testing.T) {
	service := NewService()
	now := time.Date(2026, 5, 5, 8, 0, 45, 0, time.UTC)

	snapshot := service.CreateMatch(contracts.CreateMatchRequest{
		MatchID:      "private_waiting",
		WhiteGuestID: "guest-white",
		WhiteName:    "Aurora",
	}, now)

	if snapshot.Match.Status != "waiting" {
		t.Fatalf("expected waiting private room, got %q", snapshot.Match.Status)
	}
	if snapshot.Match.Clock.RunningFor != "" || snapshot.Match.Clock.StartedAt != nil {
		t.Fatalf("expected waiting room clock to stay idle, got %#v", snapshot.Match.Clock)
	}
}

func TestJoinMatchSeatAssignsOpponentAndActivatesWaitingRoom(t *testing.T) {
	service := NewService()
	now := time.Date(2026, 5, 5, 8, 0, 50, 0, time.UTC)

	service.CreateMatch(contracts.CreateMatchRequest{
		MatchID:           "invite_room",
		WhiteGuestID:      "guest-white",
		WhiteName:         "Aurora",
		WhitePlayerSecret: "white-room-secret",
	}, now)

	joined, err := service.JoinMatchSeat("invite_room", contracts.JoinMatchSeatRequest{
		GuestID:      "guest-black",
		DisplayName:  "Velvet",
		PlayerSecret: "black-room-secret",
	}, now.Add(10*time.Second))
	if err != nil {
		t.Fatalf("expected invite room join to succeed, got %v", err)
	}

	if joined.SeatColor != "black" || !joined.Joined {
		t.Fatalf("expected join response to assign black seat, got %#v", joined)
	}
	if joined.WaitingForOpponent {
		t.Fatalf("expected match to become active after second seat joined")
	}
	if joined.Match.Match.Status != "active" {
		t.Fatalf("expected room to activate, got %q", joined.Match.Match.Status)
	}
	if joined.Match.Match.BlackGuestID != "guest-black" || joined.Match.Match.BlackName != "Velvet" {
		t.Fatalf("expected black seat to be assigned, got %#v", joined.Match.Match)
	}
	if joined.Match.Match.Clock.RunningFor != "white" || joined.Match.Match.Clock.StartedAt == nil {
		t.Fatalf("expected clock to start when opponent joins, got %#v", joined.Match.Match.Clock)
	}
}

func TestCreateMatchStarterThreeModeStartsWithThreeCards(t *testing.T) {
	service := NewService()
	now := time.Date(2026, 5, 5, 8, 1, 0, 0, time.UTC)

	snapshot := service.CreateMatch(contracts.CreateMatchRequest{
		MatchID:         "starter_three",
		StarterHandMode: "starter_three",
	}, now)

	if len(snapshot.Match.WhiteHand) != 3 || len(snapshot.Match.BlackHand) != 3 {
		t.Fatalf("expected three starter cards per side, got white=%d black=%d", len(snapshot.Match.WhiteHand), len(snapshot.Match.BlackHand))
	}
	if cardIDByMechanic(t, snapshot.Match.WhiteHand, "freeze") == "" {
		t.Fatalf("expected freeze in starter hand")
	}
	if cardIDByMechanic(t, snapshot.Match.WhiteHand, "shield") == "" {
		t.Fatalf("expected shield in starter hand")
	}
	if cardIDByMechanic(t, snapshot.Match.WhiteHand, "joker") == "" {
		t.Fatalf("expected joker in starter hand")
	}
}

func TestReplayFramesTrackPostMoveAuthoritativeState(t *testing.T) {
	service := NewService()
	now := time.Date(2026, 5, 5, 8, 5, 0, 0, time.UTC)
	snapshot := service.CreateMatch(contracts.CreateMatchRequest{MatchID: "replay_frames"}, now)

	if len(snapshot.ReplayFrames) != 1 || snapshot.ReplayFrames[0].Turn != "white" {
		t.Fatalf("expected initial replay frame for white to move, got %#v", snapshot.ReplayFrames)
	}

	moved, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "make_move",
		MatchID:  "replay_frames",
		PlayerID: "white_player",
		From:     &contracts.Square{Row: 1, Col: 4},
		To:       &contracts.Square{Row: 3, Col: 4},
	}, now.Add(15*time.Second))
	if err != nil {
		t.Fatalf("expected opening move to succeed, got %v", err)
	}

	if len(moved.ReplayFrames) != 2 {
		t.Fatalf("expected two replay frames after one move, got %d", len(moved.ReplayFrames))
	}
	latest := moved.ReplayFrames[1]
	if latest.Turn != "black" {
		t.Fatalf("expected replay frame to reflect black to move after e4, got %q", latest.Turn)
	}
	if len(latest.MoveHistory) != 1 || latest.MoveHistory[0] != "e4" {
		t.Fatalf("expected replay frame to carry move history, got %#v", latest.MoveHistory)
	}
	if latest.Board[3][4] == nil || latest.Board[3][4].Type != "pawn" || latest.Board[3][4].Color != "white" {
		t.Fatalf("expected replay frame board to contain the moved pawn on e4, got %#v", latest.Board[3][4])
	}
}

func TestGuestSeatIDsCanOwnMoveIntents(t *testing.T) {
	service := NewService()
	now := time.Date(2026, 5, 5, 8, 7, 0, 0, time.UTC)
	snapshot := service.CreateMatch(contracts.CreateMatchRequest{
		MatchID:      "guest_owned_move",
		WhiteGuestID: "aurora-101",
		BlackGuestID: "velvet-202",
	}, now)

	if snapshot.Match.WhiteGuestID != "aurora-101" || snapshot.Match.BlackGuestID != "velvet-202" {
		t.Fatalf("expected guest seat ids to persist on match creation, got %#v", snapshot.Match)
	}

	moved, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "make_move",
		MatchID:  "guest_owned_move",
		PlayerID: "aurora-101",
		From:     &contracts.Square{Row: 1, Col: 4},
		To:       &contracts.Square{Row: 3, Col: 4},
	}, now.Add(10*time.Second))
	if err != nil {
		t.Fatalf("expected guest-owned move to succeed, got %v", err)
	}
	if moved.Match.Turn != "black" {
		t.Fatalf("expected black to move after guest-owned white move, got %q", moved.Match.Turn)
	}
	if moved.Match.LastMove == nil || moved.Match.LastMove.To.Row != 3 || moved.Match.LastMove.To.Col != 4 {
		t.Fatalf("expected last move to reach e4, got %#v", moved.Match.LastMove)
	}
}

func TestGuestSeatSecretsGateMoveIntents(t *testing.T) {
	service := NewService()
	now := time.Date(2026, 5, 5, 8, 8, 0, 0, time.UTC)

	service.CreateMatch(contracts.CreateMatchRequest{
		MatchID:           "guest_secret_move",
		WhiteGuestID:      "aurora-101",
		BlackGuestID:      "velvet-202",
		WhitePlayerSecret: "white-secret",
		BlackPlayerSecret: "black-secret",
	}, now)

	_, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:         "make_move",
		MatchID:      "guest_secret_move",
		PlayerID:     "aurora-101",
		PlayerSecret: "wrong-secret",
		From:         &contracts.Square{Row: 1, Col: 4},
		To:           &contracts.Square{Row: 3, Col: 4},
	}, now.Add(5*time.Second))
	if err == nil || !strings.Contains(err.Error(), "unauthorized player secret") {
		t.Fatalf("expected wrong secret to fail authorization, got %v", err)
	}

	moved, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:         "make_move",
		MatchID:      "guest_secret_move",
		PlayerID:     "aurora-101",
		PlayerSecret: "white-secret",
		From:         &contracts.Square{Row: 1, Col: 4},
		To:           &contracts.Square{Row: 3, Col: 4},
	}, now.Add(10*time.Second))
	if err != nil {
		t.Fatalf("expected correct secret to authorize move, got %v", err)
	}
	if moved.Match.Turn != "black" {
		t.Fatalf("expected move to advance turn after authorized secret, got %q", moved.Match.Turn)
	}
}

func TestArchivedMatchReloadKeepsSecretsAndReverseHistory(t *testing.T) {
	tempDir := t.TempDir()
	archivePath := filepath.Join(tempDir, "match-archive.json")
	archive, err := platform.NewMatchArchiveStore(archivePath)
	if err != nil {
		t.Fatalf("expected archive store to initialize, got %v", err)
	}

	now := time.Date(2026, 5, 5, 8, 9, 0, 0, time.UTC)
	service := NewServiceWithArchive(archive)
	snapshot := service.CreateMatch(contracts.CreateMatchRequest{
		MatchID:           "restore_room",
		WhitePlayerSecret: "white-secret",
		BlackPlayerSecret: "black-secret",
	}, now)

	afterWhite, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:         "make_move",
		MatchID:      "restore_room",
		PlayerID:     "white_player",
		PlayerSecret: "white-secret",
		From:         &contracts.Square{Row: 1, Col: 4},
		To:           &contracts.Square{Row: 3, Col: 4},
	}, now.Add(time.Second))
	if err != nil {
		t.Fatalf("expected white move to succeed, got %v", err)
	}

	if _, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:         "make_move",
		MatchID:      "restore_room",
		PlayerID:     "black_player",
		PlayerSecret: "black-secret",
		From:         &contracts.Square{Row: 6, Col: 4},
		To:           &contracts.Square{Row: 4, Col: 4},
	}, now.Add(2*time.Second)); err != nil {
		t.Fatalf("expected black move to succeed, got %v", err)
	}

	restartedArchive, err := platform.NewMatchArchiveStore(archivePath)
	if err != nil {
		t.Fatalf("expected archive reload to succeed, got %v", err)
	}
	restarted := NewServiceWithArchive(restartedArchive)

	reloaded, err := restarted.GetMatch("restore_room")
	if err != nil {
		t.Fatalf("expected archived match to reload, got %v", err)
	}
	if reloaded.Match.Turn != "white" || len(reloaded.Match.MoveHistory) != 2 {
		t.Fatalf("expected match state to survive restart, got %#v", reloaded.Match)
	}

	if _, err := restarted.ApplyIntent(contracts.PlayerIntent{
		Type:         "make_move",
		MatchID:      "restore_room",
		PlayerID:     "white_player",
		PlayerSecret: "wrong-secret",
		From:         &contracts.Square{Row: 1, Col: 3},
		To:           &contracts.Square{Row: 3, Col: 3},
	}, now.Add(3*time.Second)); err == nil || !strings.Contains(err.Error(), "unauthorized player secret") {
		t.Fatalf("expected restored match to keep enforcing seat secrets, got %v", err)
	}

	reverseCardID := cardIDByMechanic(t, snapshot.Match.WhiteHand, "reverse")
	reversed, err := restarted.ApplyIntent(contracts.PlayerIntent{
		Type:         "play_card",
		MatchID:      "restore_room",
		PlayerID:     "white_player",
		PlayerSecret: "white-secret",
		CardID:       reverseCardID,
	}, now.Add(4*time.Second))
	if err != nil {
		t.Fatalf("expected reverse to work after archive reload, got %v", err)
	}
	if len(reversed.Match.MoveHistory) != 1 || reversed.Match.MoveHistory[0] != afterWhite.Match.MoveHistory[0] {
		t.Fatalf("expected reverse after restart to restore the pre-black-move position, got %#v", reversed.Match.MoveHistory)
	}
}

func TestPersistedArchiveSnapshotKeepsFullEventHistory(t *testing.T) {
	archive := &captureArchiver{}
	service := NewServiceWithArchive(archive)
	now := time.Date(2026, 5, 5, 8, 10, 0, 0, time.UTC)

	service.CreateMatch(contracts.CreateMatchRequest{MatchID: "archive_events"}, now)

	if _, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "make_move",
		MatchID:  "archive_events",
		PlayerID: "white_player",
		From:     &contracts.Square{Row: 1, Col: 4},
		To:       &contracts.Square{Row: 3, Col: 4},
	}, now.Add(10*time.Second)); err != nil {
		t.Fatalf("expected first move to succeed, got %v", err)
	}

	if _, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "make_move",
		MatchID:  "archive_events",
		PlayerID: "black_player",
		From:     &contracts.Square{Row: 6, Col: 4},
		To:       &contracts.Square{Row: 4, Col: 4},
	}, now.Add(20*time.Second)); err != nil {
		t.Fatalf("expected second move to succeed, got %v", err)
	}

	if len(archive.snapshots) == 0 {
		t.Fatalf("expected snapshots to be archived")
	}

	last := archive.snapshots[len(archive.snapshots)-1]
	if len(last.Events) < 5 {
		t.Fatalf("expected archived snapshot to keep full event log, got %d events", len(last.Events))
	}
	if last.Events[0].Type != "match_started" {
		t.Fatalf("expected archived event log to include match start, got %#v", last.Events)
	}
}

func TestPlayCardRejectsUnknownPlayerID(t *testing.T) {
	service := NewService()
	now := time.Date(2026, 5, 5, 8, 0, 0, 0, time.UTC)
	snapshot := service.CreateMatch(contracts.CreateMatchRequest{MatchID: "bad_player"}, now)

	_, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "play_card",
		MatchID:  "bad_player",
		PlayerID: "mystery_actor",
		CardID:   cardIDByMechanic(t, snapshot.Match.WhiteHand, "freeze"),
	}, now.Add(time.Second))
	if err == nil || !strings.Contains(err.Error(), "unrecognized player id") {
		t.Fatalf("expected unrecognized player id error, got %v", err)
	}
}

func TestFrozenPieceCannotMove(t *testing.T) {
	service := NewService()
	now := time.Date(2026, 5, 5, 8, 0, 0, 0, time.UTC)
	service.CreateMatch(contracts.CreateMatchRequest{MatchID: "frozen"}, now)

	state := service.matches["frozen"]
	state.Board[1][4].Frozen = true

	_, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "make_move",
		MatchID:  "frozen",
		PlayerID: "white_player",
		From:     &contracts.Square{Row: 1, Col: 4},
		To:       &contracts.Square{Row: 3, Col: 4},
	}, now.Add(2*time.Second))
	if err == nil || !strings.Contains(err.Error(), "frozen") {
		t.Fatalf("expected frozen move error, got %v", err)
	}
}

func TestShieldedCaptureIsBlockedAndShieldRemoved(t *testing.T) {
	service := NewService()
	now := time.Date(2026, 5, 5, 8, 0, 0, 0, time.UTC)
	service.CreateMatch(contracts.CreateMatchRequest{MatchID: "shielded"}, now)

	state := service.matches["shielded"]
	state.Board = emptyBoard()
	state.Board[0][4] = &contracts.Piece{Type: "king", Color: "white"}
	state.Board[7][4] = &contracts.Piece{Type: "king", Color: "black"}
	state.Board[3][3] = &contracts.Piece{Type: "bishop", Color: "white"}
	state.Board[1][0] = &contracts.Piece{Type: "pawn", Color: "white"}
	shieldTurn := 2
	state.Board[4][4] = &contracts.Piece{Type: "knight", Color: "black", Shielded: true, ShieldTurn: &shieldTurn}

	snapshot, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "make_move",
		MatchID:  "shielded",
		PlayerID: "white_player",
		From:     &contracts.Square{Row: 3, Col: 3},
		To:       &contracts.Square{Row: 4, Col: 4},
	}, now.Add(2*time.Second))
	if err != nil {
		t.Fatalf("expected shielded capture to resolve as blocked event, got error %v", err)
	}

	if state.Board[3][3] == nil || state.Board[3][3].Type != "bishop" {
		t.Fatalf("expected attacking bishop to stay put after blocked capture")
	}
	if state.Board[4][4] == nil || state.Board[4][4].Type != "knight" {
		t.Fatalf("expected shielded target to remain on board")
	}
	if state.Board[4][4].Shielded {
		t.Fatalf("expected shield to be consumed")
	}
	if len(snapshot.Events) != 1 || snapshot.Events[0].Type != "target_selected" {
		t.Fatalf("expected shield-block event, got %#v", snapshot.Events)
	}
}

func TestDrawOfferCannotBeAcceptedByOfferingSide(t *testing.T) {
	service := NewService()
	now := time.Date(2026, 5, 5, 8, 0, 0, 0, time.UTC)
	service.CreateMatch(contracts.CreateMatchRequest{MatchID: "draw_self_accept"}, now)

	if _, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "offer_draw",
		MatchID:  "draw_self_accept",
		PlayerID: "white_player",
	}, now.Add(time.Second)); err != nil {
		t.Fatalf("expected draw offer to succeed, got %v", err)
	}

	accept := true
	_, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "respond_draw",
		MatchID:  "draw_self_accept",
		PlayerID: "white_player",
		Accept:   &accept,
	}, now.Add(2*time.Second))
	if err == nil || !strings.Contains(err.Error(), "offering side") {
		t.Fatalf("expected offering side rejection, got %v", err)
	}
}

func TestAbortFinishesEarlyMatch(t *testing.T) {
	service := NewService()
	now := time.Date(2026, 5, 5, 8, 0, 0, 0, time.UTC)
	service.CreateMatch(contracts.CreateMatchRequest{MatchID: "abort_early"}, now)

	snapshot, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "abort",
		MatchID:  "abort_early",
		PlayerID: "white_player",
	}, now.Add(time.Second))
	if err != nil {
		t.Fatalf("expected early abort to succeed, got %v", err)
	}

	if snapshot.Match.Status != "finished" || snapshot.Match.Winner != "aborted" || snapshot.Match.FinishReason != "abort" {
		t.Fatalf("expected aborted finished match, got status=%q winner=%q reason=%q", snapshot.Match.Status, snapshot.Match.Winner, snapshot.Match.FinishReason)
	}
	if len(snapshot.Events) != 1 || snapshot.Events[0].Type != "match_finished" || snapshot.Events[0].Payload["result"] != "abort" {
		t.Fatalf("expected abort finish event, got %#v", snapshot.Events)
	}
}

func TestAbortRejectedAfterBlackFirstReply(t *testing.T) {
	service := NewService()
	now := time.Date(2026, 5, 5, 8, 0, 0, 0, time.UTC)
	service.CreateMatch(contracts.CreateMatchRequest{MatchID: "abort_late"}, now)

	if _, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "make_move",
		MatchID:  "abort_late",
		PlayerID: "white_player",
		From:     &contracts.Square{Row: 1, Col: 4},
		To:       &contracts.Square{Row: 3, Col: 4},
	}, now.Add(time.Second)); err != nil {
		t.Fatalf("expected white opening move to succeed, got %v", err)
	}

	if _, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "make_move",
		MatchID:  "abort_late",
		PlayerID: "black_player",
		From:     &contracts.Square{Row: 6, Col: 4},
		To:       &contracts.Square{Row: 4, Col: 4},
	}, now.Add(2*time.Second)); err != nil {
		t.Fatalf("expected black reply to succeed, got %v", err)
	}

	_, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "abort",
		MatchID:  "abort_late",
		PlayerID: "white_player",
	}, now.Add(3*time.Second))
	if err == nil || !strings.Contains(err.Error(), "only allowed") {
		t.Fatalf("expected late abort rejection, got %v", err)
	}
}

func TestPawnPromotionResolvedByBackend(t *testing.T) {
	service := NewService()
	now := time.Date(2026, 5, 5, 8, 0, 0, 0, time.UTC)
	service.CreateMatch(contracts.CreateMatchRequest{MatchID: "promotion"}, now)

	state := service.matches["promotion"]
	state.Board = emptyBoard()
	state.Board[0][4] = &contracts.Piece{Type: "king", Color: "white"}
	state.Board[7][4] = &contracts.Piece{Type: "king", Color: "black"}
	state.Board[6][0] = &contracts.Piece{Type: "pawn", Color: "white"}

	snapshot, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:      "make_move",
		MatchID:   "promotion",
		PlayerID:  "white_player",
		From:      &contracts.Square{Row: 6, Col: 0},
		To:        &contracts.Square{Row: 7, Col: 0},
		Promotion: "queen",
	}, now.Add(2*time.Second))
	if err != nil {
		t.Fatalf("expected promotion move to succeed, got %v", err)
	}

	if promoted := state.Board[7][0]; promoted == nil || promoted.Type != "queen" {
		t.Fatalf("expected backend promotion to queen, got %#v", state.Board[7][0])
	}
	if len(snapshot.Events) == 0 || snapshot.Events[0].Payload["promotion"] != "queen" {
		t.Fatalf("expected promotion in event payload, got %#v", snapshot.Events)
	}
}

func TestClockTimeoutFinishesMatchBeforeLateIntent(t *testing.T) {
	service := NewService()
	now := time.Date(2026, 5, 5, 8, 0, 0, 0, time.UTC)
	service.CreateMatch(contracts.CreateMatchRequest{MatchID: "timeout", ClockSeconds: 1}, now)

	snapshot, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "make_move",
		MatchID:  "timeout",
		PlayerID: "white_player",
		From:     &contracts.Square{Row: 1, Col: 4},
		To:       &contracts.Square{Row: 3, Col: 4},
	}, now.Add(2*time.Second))
	if err != nil {
		t.Fatalf("expected timeout resolution instead of validation error, got %v", err)
	}

	if snapshot.Match.Status != "finished" || snapshot.Match.Winner != "black" || snapshot.Match.FinishReason != "timeout" {
		t.Fatalf("expected white to flag and black to win, got status=%q winner=%q reason=%q", snapshot.Match.Status, snapshot.Match.Winner, snapshot.Match.FinishReason)
	}
	if len(snapshot.Events) != 2 || snapshot.Events[1].Type != "match_finished" {
		t.Fatalf("expected timeout events, got %#v", snapshot.Events)
	}
}

func TestMoveCheckmateFinishesAuthoritatively(t *testing.T) {
	service := NewService()
	now := time.Date(2026, 5, 5, 8, 5, 0, 0, time.UTC)
	service.CreateMatch(contracts.CreateMatchRequest{MatchID: "mate_finish"}, now)

	state := service.matches["mate_finish"]
	state.Board = emptyBoard()
	state.Board[5][5] = &contracts.Piece{Type: "king", Color: "white"}
	state.Board[5][6] = &contracts.Piece{Type: "queen", Color: "white"}
	state.Board[7][7] = &contracts.Piece{Type: "king", Color: "black"}
	state.Turn = "white"
	state.Moved = nil
	state.LastMove = nil
	state.HalfMoveClock = 0
	state.FullMoveNum = 1
	state.MoveHistory = nil
	state.History = []contracts.PositionState{capturePositionState(state)}

	snapshot, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "make_move",
		MatchID:  "mate_finish",
		PlayerID: "white_player",
		From:     &contracts.Square{Row: 5, Col: 6},
		To:       &contracts.Square{Row: 6, Col: 6},
	}, now.Add(time.Second))
	if err != nil {
		t.Fatalf("expected mating move to succeed, got %v", err)
	}

	if snapshot.Match.Status != "finished" || snapshot.Match.Winner != "white" || snapshot.Match.FinishReason != "checkmate" {
		t.Fatalf("expected authoritative checkmate, got status=%q winner=%q reason=%q", snapshot.Match.Status, snapshot.Match.Winner, snapshot.Match.FinishReason)
	}
	lastEvent := snapshot.Events[len(snapshot.Events)-1]
	if lastEvent.Type != "match_finished" || lastEvent.Payload["result"] != "checkmate" {
		t.Fatalf("expected checkmate finish event, got %#v", lastEvent)
	}
}

func TestFiftyMoveRuleFinishesAuthoritatively(t *testing.T) {
	service := NewService()
	now := time.Date(2026, 5, 5, 8, 10, 0, 0, time.UTC)
	service.CreateMatch(contracts.CreateMatchRequest{MatchID: "fifty_move_finish"}, now)

	state := service.matches["fifty_move_finish"]
	state.Board = emptyBoard()
	state.Board[0][4] = &contracts.Piece{Type: "king", Color: "white"}
	state.Board[7][4] = &contracts.Piece{Type: "king", Color: "black"}
	state.Board[0][0] = &contracts.Piece{Type: "rook", Color: "white"}
	state.Turn = "white"
	state.HalfMoveClock = 99
	state.FullMoveNum = 1
	state.MoveHistory = nil
	state.History = []contracts.PositionState{capturePositionState(state)}

	snapshot, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "make_move",
		MatchID:  "fifty_move_finish",
		PlayerID: "white_player",
		From:     &contracts.Square{Row: 0, Col: 0},
		To:       &contracts.Square{Row: 1, Col: 0},
	}, now.Add(time.Second))
	if err != nil {
		t.Fatalf("expected non-capturing rook move to succeed, got %v", err)
	}

	if snapshot.Match.Status != "finished" || snapshot.Match.Winner != "draw" || snapshot.Match.FinishReason != "fifty_move_rule" {
		t.Fatalf("expected 50-move draw, got status=%q winner=%q reason=%q", snapshot.Match.Status, snapshot.Match.Winner, snapshot.Match.FinishReason)
	}
	lastEvent := snapshot.Events[len(snapshot.Events)-1]
	if lastEvent.Type != "match_finished" || lastEvent.Payload["result"] != "fifty_move_rule" {
		t.Fatalf("expected 50-move finish event, got %#v", lastEvent)
	}
}

func TestThreefoldRepetitionFinishesAuthoritatively(t *testing.T) {
	service := NewService()
	now := time.Date(2026, 5, 5, 8, 15, 0, 0, time.UTC)
	service.CreateMatch(contracts.CreateMatchRequest{MatchID: "threefold_finish"}, now)

	state := service.matches["threefold_finish"]
	state.Board = emptyBoard()
	state.Board[0][4] = &contracts.Piece{Type: "king", Color: "white"}
	state.Board[7][4] = &contracts.Piece{Type: "king", Color: "black"}
	state.Board[1][0] = &contracts.Piece{Type: "rook", Color: "white"}
	state.Turn = "white"
	state.Moved = []string{"1-0"}
	state.LastMove = nil
	state.HalfMoveClock = 10
	state.FullMoveNum = 1
	state.MoveHistory = nil
	repeated := contracts.PositionState{
		Board: cloneBoard(emptyBoard()),
		Turn:  "black",
		Moved: []string{"1-0"},
	}
	repeated.Board[0][4] = &contracts.Piece{Type: "king", Color: "white"}
	repeated.Board[7][4] = &contracts.Piece{Type: "king", Color: "black"}
	repeated.Board[0][0] = &contracts.Piece{Type: "rook", Color: "white"}
	state.History = []contracts.PositionState{
		repeated,
		repeated,
	}

	snapshot, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "make_move",
		MatchID:  "threefold_finish",
		PlayerID: "white_player",
		From:     &contracts.Square{Row: 1, Col: 0},
		To:       &contracts.Square{Row: 0, Col: 0},
	}, now.Add(time.Second))
	if err != nil {
		t.Fatalf("expected repetition move to succeed, got %v", err)
	}

	if snapshot.Match.Status != "finished" || snapshot.Match.Winner != "draw" || snapshot.Match.FinishReason != "threefold_repetition" {
		t.Fatalf("expected threefold draw, got status=%q winner=%q reason=%q", snapshot.Match.Status, snapshot.Match.Winner, snapshot.Match.FinishReason)
	}
	lastEvent := snapshot.Events[len(snapshot.Events)-1]
	if lastEvent.Type != "match_finished" || lastEvent.Payload["result"] != "threefold_repetition" {
		t.Fatalf("expected threefold finish event, got %#v", lastEvent)
	}
}

func TestPresenceHeartbeatMarksSeatsConnectedInSnapshot(t *testing.T) {
	service := NewService()
	now := time.Date(2026, 5, 5, 8, 30, 0, 0, time.UTC)
	service.CreateMatch(contracts.CreateMatchRequest{
		MatchID:      "presence_connected",
		WhiteGuestID: "guest-white",
		BlackGuestID: "guest-black",
	}, now)

	if err := service.HeartbeatPresence("presence_connected", contracts.MatchPresenceRequest{
		PlayerID: "guest-white",
	}, now.Add(3*time.Second)); err != nil {
		t.Fatalf("expected white heartbeat to succeed, got %v", err)
	}
	if err := service.HeartbeatPresence("presence_connected", contracts.MatchPresenceRequest{
		PlayerID: "guest-black",
	}, now.Add(4*time.Second)); err != nil {
		t.Fatalf("expected black heartbeat to succeed, got %v", err)
	}

	snapshot, err := service.GetMatch("presence_connected")
	if err != nil {
		t.Fatalf("expected snapshot to load, got %v", err)
	}
	if !snapshot.Match.WhiteConnected || !snapshot.Match.BlackConnected {
		t.Fatalf("expected both seats to be connected, got white=%v black=%v", snapshot.Match.WhiteConnected, snapshot.Match.BlackConnected)
	}
	if snapshot.Match.DisconnectGraceFor != "" || snapshot.Match.DisconnectGraceDeadline != nil {
		t.Fatalf("expected no active disconnect grace, got %#v", snapshot.Match)
	}
}

func TestDisconnectGraceFinishesAbandonedMatch(t *testing.T) {
	service := NewService()
	now := time.Date(2026, 5, 5, 8, 35, 0, 0, time.UTC)
	service.CreateMatch(contracts.CreateMatchRequest{
		MatchID:      "presence_abandon",
		WhiteGuestID: "guest-white",
		BlackGuestID: "guest-black",
	}, now)

	if err := service.HeartbeatPresence("presence_abandon", contracts.MatchPresenceRequest{PlayerID: "guest-white"}, now); err != nil {
		t.Fatalf("expected white heartbeat to succeed, got %v", err)
	}
	if err := service.HeartbeatPresence("presence_abandon", contracts.MatchPresenceRequest{PlayerID: "guest-black"}, now); err != nil {
		t.Fatalf("expected black heartbeat to succeed, got %v", err)
	}
	if err := service.HeartbeatPresence("presence_abandon", contracts.MatchPresenceRequest{PlayerID: "guest-black"}, now.Add(20*time.Second)); err != nil {
		t.Fatalf("expected follow-up black heartbeat to succeed, got %v", err)
	}

	service.collectAndBroadcast(now.Add(30 * time.Second))
	graceSnapshot, err := service.GetMatch("presence_abandon")
	if err != nil {
		t.Fatalf("expected grace snapshot to load, got %v", err)
	}
	if graceSnapshot.Match.DisconnectGraceFor != "white" || graceSnapshot.Match.DisconnectGraceDeadline == nil {
		t.Fatalf("expected white disconnect grace to start, got %#v", graceSnapshot.Match)
	}
	if graceSnapshot.Match.Status != "active" {
		t.Fatalf("expected match to stay active during grace, got %q", graceSnapshot.Match.Status)
	}

	if err := service.HeartbeatPresence("presence_abandon", contracts.MatchPresenceRequest{PlayerID: "guest-black"}, now.Add(65*time.Second)); err != nil {
		t.Fatalf("expected black heartbeat during grace to succeed, got %v", err)
	}
	service.collectAndBroadcast(now.Add(80 * time.Second))

	finishedSnapshot, err := service.GetMatch("presence_abandon")
	if err != nil {
		t.Fatalf("expected finished snapshot to load, got %v", err)
	}
	if finishedSnapshot.Match.Status != "finished" || finishedSnapshot.Match.Winner != "black" || finishedSnapshot.Match.FinishReason != "abandon" {
		t.Fatalf("expected black to win abandoned match, got status=%q winner=%q reason=%q", finishedSnapshot.Match.Status, finishedSnapshot.Match.Winner, finishedSnapshot.Match.FinishReason)
	}
	if len(finishedSnapshot.Events) == 0 {
		t.Fatalf("expected abandon finish event to be recorded")
	}
	lastEvent := finishedSnapshot.Events[len(finishedSnapshot.Events)-1]
	if lastEvent.Type != "match_finished" || lastEvent.Payload["result"] != "abandon" || lastEvent.Payload["disconnected"] != "white" {
		t.Fatalf("expected abandon finish payload, got %#v", lastEvent)
	}
}

func TestDisconnectGraceFinishesBothDisconnectedNoMoveMatchAsAbort(t *testing.T) {
	service := NewService()
	now := time.Date(2026, 5, 5, 9, 0, 0, 0, time.UTC)
	service.CreateMatch(contracts.CreateMatchRequest{
		MatchID:      "presence_both_abort",
		WhiteGuestID: "guest-white",
		BlackGuestID: "guest-black",
	}, now)

	service.collectAndBroadcast(now.Add(presenceHeartbeatTimeout + disconnectGracePeriod + time.Second))

	finishedSnapshot, err := service.GetMatch("presence_both_abort")
	if err != nil {
		t.Fatalf("expected finished snapshot to load, got %v", err)
	}
	if finishedSnapshot.Match.Status != "finished" || finishedSnapshot.Match.Winner != "aborted" || finishedSnapshot.Match.FinishReason != "abort" {
		t.Fatalf("expected both-disconnected no-move room to abort, got status=%q winner=%q reason=%q", finishedSnapshot.Match.Status, finishedSnapshot.Match.Winner, finishedSnapshot.Match.FinishReason)
	}
	lastEvent := finishedSnapshot.Events[len(finishedSnapshot.Events)-1]
	if lastEvent.Type != "match_finished" || lastEvent.Payload["result"] != "abort" || lastEvent.Payload["disconnected"] != disconnectGraceBoth {
		t.Fatalf("expected both-disconnected abort payload, got %#v", lastEvent)
	}
}

func TestRestartedServiceReconcilesArchivedActiveMatch(t *testing.T) {
	tempDir := t.TempDir()
	archivePath := filepath.Join(tempDir, "reconcile-archive.json")
	archive, err := platform.NewMatchArchiveStore(archivePath)
	if err != nil {
		t.Fatalf("expected archive store to initialize, got %v", err)
	}
	defer func() { _ = archive.Close() }()

	base := time.Now().Add(-2 * time.Hour).UTC()
	service := NewServiceWithArchive(archive)
	service.CreateMatch(contracts.CreateMatchRequest{
		MatchID:      "reconcile_room",
		WhiteGuestID: "guest-white",
		BlackGuestID: "guest-black",
	}, base)
	if _, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "make_move",
		MatchID:  "reconcile_room",
		PlayerID: "guest-white",
		From:     &contracts.Square{Row: 1, Col: 4},
		To:       &contracts.Square{Row: 3, Col: 4},
	}, base.Add(10*time.Second)); err != nil {
		t.Fatalf("expected archived match move to succeed, got %v", err)
	}

	restartedArchive, err := platform.NewMatchArchiveStore(archivePath)
	if err != nil {
		t.Fatalf("expected archive reload to succeed, got %v", err)
	}
	defer func() { _ = restartedArchive.Close() }()
	restarted := NewServiceWithArchive(restartedArchive)

	stats := restarted.Stats()
	if stats.LoadedMatches != 1 || stats.ActiveMatches != 1 {
		t.Fatalf("expected unfinished archived room to preload on startup, got %#v", stats)
	}

	restarted.collectAndBroadcast(time.Now().UTC())

	reconciled, err := restarted.GetMatch("reconcile_room")
	if err != nil {
		t.Fatalf("expected reconciled match to load, got %v", err)
	}
	if reconciled.Match.Status != "finished" || reconciled.Match.Winner != "draw" || reconciled.Match.FinishReason != "abandon" {
		t.Fatalf("expected restarted active room to reconcile into a draw abandon, got status=%q winner=%q reason=%q", reconciled.Match.Status, reconciled.Match.Winner, reconciled.Match.FinishReason)
	}
	lastEvent := reconciled.Events[len(reconciled.Events)-1]
	if lastEvent.Type != "match_finished" || lastEvent.Payload["result"] != "abandon" || lastEvent.Payload["disconnected"] != disconnectGraceBoth {
		t.Fatalf("expected reconciled finish payload to reflect both disconnected players, got %#v", lastEvent)
	}
}

func TestPlayCardCreatesPendingFreezeSelection(t *testing.T) {
	service := NewService()
	now := time.Date(2026, 5, 5, 8, 0, 0, 0, time.UTC)
	snapshot := service.CreateMatch(contracts.CreateMatchRequest{MatchID: "freeze_card"}, now)

	cardID := cardIDByMechanic(t, snapshot.Match.WhiteHand, "freeze")
	result, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "play_card",
		MatchID:  "freeze_card",
		PlayerID: "white_player",
		CardID:   cardID,
	}, now.Add(time.Second))
	if err != nil {
		t.Fatalf("expected freeze card to enter pending state, got %v", err)
	}
	if result.Match.PendingCard == nil || result.Match.PendingCard.Mechanic != "freeze" {
		t.Fatalf("expected pending freeze card, got %#v", result.Match.PendingCard)
	}
}

func TestSelectTargetAppliesFreezeAndConsumesCard(t *testing.T) {
	service := NewService()
	now := time.Date(2026, 5, 5, 8, 0, 0, 0, time.UTC)
	snapshot := service.CreateMatch(contracts.CreateMatchRequest{MatchID: "freeze_apply"}, now)
	cardID := cardIDByMechanic(t, snapshot.Match.WhiteHand, "freeze")

	if _, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "play_card",
		MatchID:  "freeze_apply",
		PlayerID: "white_player",
		CardID:   cardID,
	}, now.Add(time.Second)); err != nil {
		t.Fatalf("expected play_card to succeed, got %v", err)
	}

	result, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "select_target",
		MatchID:  "freeze_apply",
		PlayerID: "white_player",
		Target:   &contracts.Square{Row: 6, Col: 0},
	}, now.Add(2*time.Second))
	if err != nil {
		t.Fatalf("expected target selection to freeze piece, got %v", err)
	}

	if frozen := service.matches["freeze_apply"].Board[6][0]; frozen == nil || !frozen.Frozen {
		t.Fatalf("expected backend to mark target frozen")
	}
	if result.Match.PendingCard != nil {
		t.Fatalf("expected pending card to clear after target selection")
	}
	if len(result.Match.WhiteHand) != len(snapshot.Match.WhiteHand)-1 {
		t.Fatalf("expected card to be consumed from hand, got %d cards", len(result.Match.WhiteHand))
	}
}

func TestSelectTargetAppliesShieldAndSetsExpiry(t *testing.T) {
	service := NewService()
	now := time.Date(2026, 5, 5, 8, 0, 0, 0, time.UTC)
	snapshot := service.CreateMatch(contracts.CreateMatchRequest{MatchID: "shield_apply"}, now)
	cardID := cardIDByMechanic(t, snapshot.Match.WhiteHand, "shield")

	if _, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "play_card",
		MatchID:  "shield_apply",
		PlayerID: "white_player",
		CardID:   cardID,
	}, now.Add(time.Second)); err != nil {
		t.Fatalf("expected play_card to succeed, got %v", err)
	}

	result, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "select_target",
		MatchID:  "shield_apply",
		PlayerID: "white_player",
		Target:   &contracts.Square{Row: 1, Col: 0},
	}, now.Add(2*time.Second))
	if err != nil {
		t.Fatalf("expected target selection to shield piece, got %v", err)
	}

	shielded := service.matches["shield_apply"].Board[1][0]
	if shielded == nil || !shielded.Shielded || shielded.ShieldTurn == nil {
		t.Fatalf("expected backend to shield target with expiry, got %#v", shielded)
	}
	if result.Match.PendingCard != nil {
		t.Fatalf("expected pending card to clear after shield selection")
	}
}

func TestPlayCardCreatesPendingSniperSelection(t *testing.T) {
	service := NewService()
	now := time.Date(2026, 5, 5, 8, 0, 0, 0, time.UTC)
	snapshot := service.CreateMatch(contracts.CreateMatchRequest{MatchID: "sniper_card"}, now)
	cardID := cardIDByMechanic(t, snapshot.Match.WhiteHand, "sniper")

	result, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "play_card",
		MatchID:  "sniper_card",
		PlayerID: "white_player",
		CardID:   cardID,
	}, now.Add(time.Second))
	if err != nil {
		t.Fatalf("expected sniper card to enter pending state, got %v", err)
	}
	if result.Match.PendingCard == nil || result.Match.PendingCard.Mechanic != "sniper" {
		t.Fatalf("expected pending sniper card, got %#v", result.Match.PendingCard)
	}
}

func TestSelectTargetAppliesSniperAndConsumesCard(t *testing.T) {
	service := NewService()
	now := time.Date(2026, 5, 5, 8, 0, 0, 0, time.UTC)
	snapshot := service.CreateMatch(contracts.CreateMatchRequest{MatchID: "sniper_apply"}, now)
	cardID := cardIDByMechanic(t, snapshot.Match.WhiteHand, "sniper")

	if _, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "play_card",
		MatchID:  "sniper_apply",
		PlayerID: "white_player",
		CardID:   cardID,
	}, now.Add(time.Second)); err != nil {
		t.Fatalf("expected play_card to succeed, got %v", err)
	}

	result, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "select_target",
		MatchID:  "sniper_apply",
		PlayerID: "white_player",
		Target:   &contracts.Square{Row: 6, Col: 0},
	}, now.Add(2*time.Second))
	if err != nil {
		t.Fatalf("expected sniper target selection to succeed, got %v", err)
	}

	if removed := service.matches["sniper_apply"].Board[6][0]; removed != nil {
		t.Fatalf("expected backend to remove sniped piece, got %#v", removed)
	}
	if result.Match.PendingCard != nil {
		t.Fatalf("expected pending sniper card to clear after target selection")
	}
	if len(result.Match.WhiteHand) != len(snapshot.Match.WhiteHand)-1 {
		t.Fatalf("expected one sniper card to be consumed, got %d cards", len(result.Match.WhiteHand))
	}
}

func TestSelectTargetRejectsSniperIfRemovalChecksEnemyKing(t *testing.T) {
	service := NewService()
	now := time.Date(2026, 5, 5, 8, 0, 0, 0, time.UTC)
	snapshot := service.CreateMatch(contracts.CreateMatchRequest{MatchID: "sniper_enemy_check"}, now)
	cardID := cardIDByMechanic(t, snapshot.Match.WhiteHand, "sniper")

	state := service.matches["sniper_enemy_check"]
	state.Board = emptyBoard()
	state.Board[0][0] = &contracts.Piece{Type: "king", Color: "white"}
	state.Board[7][4] = &contracts.Piece{Type: "king", Color: "black"}
	state.Board[7][0] = &contracts.Piece{Type: "rook", Color: "white"}
	state.Board[7][2] = &contracts.Piece{Type: "bishop", Color: "black"}

	if _, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "play_card",
		MatchID:  "sniper_enemy_check",
		PlayerID: "white_player",
		CardID:   cardID,
	}, now.Add(time.Second)); err != nil {
		t.Fatalf("expected play_card to succeed, got %v", err)
	}

	_, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "select_target",
		MatchID:  "sniper_enemy_check",
		PlayerID: "white_player",
		Target:   &contracts.Square{Row: 7, Col: 2},
	}, now.Add(2*time.Second))
	if err == nil || !strings.Contains(err.Error(), "enemy king in check") {
		t.Fatalf("expected sniper rejection for exposing enemy king to check, got %v", err)
	}
}

func TestSelectTargetAppliesBadSniperAndConsumesCard(t *testing.T) {
	service := NewService()
	now := time.Date(2026, 5, 5, 8, 0, 0, 0, time.UTC)
	snapshot := service.CreateMatch(contracts.CreateMatchRequest{MatchID: "badsniper_apply"}, now)
	cardID := cardIDByMechanic(t, snapshot.Match.WhiteHand, "badsniper")

	state := service.matches["badsniper_apply"]
	state.Board = emptyBoard()
	state.Board[0][0] = &contracts.Piece{Type: "king", Color: "white"}
	state.Board[7][7] = &contracts.Piece{Type: "king", Color: "black"}
	state.Board[2][2] = &contracts.Piece{Type: "knight", Color: "white"}
	state.Board[6][6] = &contracts.Piece{Type: "rook", Color: "black"}

	if _, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "play_card",
		MatchID:  "badsniper_apply",
		PlayerID: "white_player",
		CardID:   cardID,
	}, now.Add(time.Second)); err != nil {
		t.Fatalf("expected play_card to succeed, got %v", err)
	}

	result, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "select_target",
		MatchID:  "badsniper_apply",
		PlayerID: "white_player",
		Target:   &contracts.Square{Row: 2, Col: 2},
	}, now.Add(2*time.Second))
	if err != nil {
		t.Fatalf("expected bad sniper target selection to succeed, got %v", err)
	}

	if removed := service.matches["badsniper_apply"].Board[2][2]; removed != nil {
		t.Fatalf("expected backend to remove own targeted piece, got %#v", removed)
	}
	if result.Match.PendingCard != nil {
		t.Fatalf("expected pending bad sniper card to clear after target selection")
	}
	if len(result.Match.WhiteHand) != len(snapshot.Match.WhiteHand)-1 {
		t.Fatalf("expected one bad sniper card to be consumed, got %d cards", len(result.Match.WhiteHand))
	}
}

func TestPromoteFlowBuildsOptionsThenAppliesSelection(t *testing.T) {
	service := NewService()
	now := time.Date(2026, 5, 5, 8, 0, 0, 0, time.UTC)
	snapshot := service.CreateMatch(contracts.CreateMatchRequest{MatchID: "promote_flow"}, now)
	cardID := cardIDByMechanic(t, snapshot.Match.WhiteHand, "promote")

	if _, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "play_card",
		MatchID:  "promote_flow",
		PlayerID: "white_player",
		CardID:   cardID,
	}, now.Add(time.Second)); err != nil {
		t.Fatalf("expected promote play_card to succeed, got %v", err)
	}

	targeted, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "select_target",
		MatchID:  "promote_flow",
		PlayerID: "white_player",
		Target:   &contracts.Square{Row: 1, Col: 0},
	}, now.Add(2*time.Second))
	if err != nil {
		t.Fatalf("expected promote target step to succeed, got %v", err)
	}

	if targeted.Match.PendingCard == nil || targeted.Match.PendingCard.Target == nil {
		t.Fatalf("expected pending promote target to be stored, got %#v", targeted.Match.PendingCard)
	}
	if !containsString(targeted.Match.PendingCard.Options, "queen") {
		t.Fatalf("expected promote options to include queen, got %#v", targeted.Match.PendingCard.Options)
	}

	resolved, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:        "select_target",
		MatchID:     "promote_flow",
		PlayerID:    "white_player",
		SelectionID: "queen",
	}, now.Add(3*time.Second))
	if err != nil {
		t.Fatalf("expected promote selection step to succeed, got %v", err)
	}

	if piece := service.matches["promote_flow"].Board[1][0]; piece == nil || piece.Type != "queen" {
		t.Fatalf("expected promoted piece to become queen, got %#v", piece)
	}
	if resolved.Match.PendingCard != nil {
		t.Fatalf("expected pending promote card to clear after selection")
	}
	if len(resolved.Match.WhiteHand) != len(snapshot.Match.WhiteHand)-1 {
		t.Fatalf("expected one promote card to be consumed, got %d cards", len(resolved.Match.WhiteHand))
	}
}

func TestDemoteFlowRejectsMissingSelection(t *testing.T) {
	service := NewService()
	now := time.Date(2026, 5, 5, 8, 0, 0, 0, time.UTC)
	snapshot := service.CreateMatch(contracts.CreateMatchRequest{MatchID: "demote_flow"}, now)
	cardID := cardIDByMechanic(t, snapshot.Match.WhiteHand, "demote")

	state := service.matches["demote_flow"]
	state.Board = emptyBoard()
	state.Board[0][0] = &contracts.Piece{Type: "king", Color: "white"}
	state.Board[7][7] = &contracts.Piece{Type: "king", Color: "black"}
	state.Board[3][3] = &contracts.Piece{Type: "queen", Color: "white"}

	if _, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "play_card",
		MatchID:  "demote_flow",
		PlayerID: "white_player",
		CardID:   cardID,
	}, now.Add(time.Second)); err != nil {
		t.Fatalf("expected demote play_card to succeed, got %v", err)
	}

	if _, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "select_target",
		MatchID:  "demote_flow",
		PlayerID: "white_player",
		Target:   &contracts.Square{Row: 3, Col: 3},
	}, now.Add(2*time.Second)); err != nil {
		t.Fatalf("expected demote target step to succeed, got %v", err)
	}

	_, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "select_target",
		MatchID:  "demote_flow",
		PlayerID: "white_player",
	}, now.Add(3*time.Second))
	if err == nil || !strings.Contains(err.Error(), "selectionId") {
		t.Fatalf("expected missing selectionId error, got %v", err)
	}
}

func TestPromoteHimFlowBuildsOptionsAndAppliesSelection(t *testing.T) {
	service := NewService()
	now := time.Date(2026, 5, 5, 8, 0, 0, 0, time.UTC)
	snapshot := service.CreateMatch(contracts.CreateMatchRequest{MatchID: "promotehim_flow"}, now)
	cardID := cardIDByMechanic(t, snapshot.Match.WhiteHand, "promotehim")

	if _, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "play_card",
		MatchID:  "promotehim_flow",
		PlayerID: "white_player",
		CardID:   cardID,
	}, now.Add(time.Second)); err != nil {
		t.Fatalf("expected promotehim play_card to succeed, got %v", err)
	}

	targeted, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "select_target",
		MatchID:  "promotehim_flow",
		PlayerID: "white_player",
		Target:   &contracts.Square{Row: 6, Col: 0},
	}, now.Add(2*time.Second))
	if err != nil {
		t.Fatalf("expected promotehim target step to succeed, got %v", err)
	}

	if targeted.Match.PendingCard == nil || targeted.Match.PendingCard.Target == nil {
		t.Fatalf("expected pending promotehim target to be stored, got %#v", targeted.Match.PendingCard)
	}
	if !containsString(targeted.Match.PendingCard.Options, "queen") {
		t.Fatalf("expected promotehim options to include queen, got %#v", targeted.Match.PendingCard.Options)
	}

	resolved, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:        "select_target",
		MatchID:     "promotehim_flow",
		PlayerID:    "white_player",
		SelectionID: "queen",
	}, now.Add(3*time.Second))
	if err != nil {
		t.Fatalf("expected promotehim selection step to succeed, got %v", err)
	}

	if piece := service.matches["promotehim_flow"].Board[6][0]; piece == nil || piece.Type != "queen" || piece.Color != "black" {
		t.Fatalf("expected enemy piece to become black queen, got %#v", piece)
	}
	if resolved.Match.PendingCard != nil {
		t.Fatalf("expected pending promotehim card to clear after selection")
	}
	if len(resolved.Match.WhiteHand) != len(snapshot.Match.WhiteHand)-1 {
		t.Fatalf("expected one promotehim card to be consumed, got %d cards", len(resolved.Match.WhiteHand))
	}
}

func TestDemoteHimCanTargetOwnPieceAndApplySelection(t *testing.T) {
	service := NewService()
	now := time.Date(2026, 5, 5, 8, 0, 0, 0, time.UTC)
	snapshot := service.CreateMatch(contracts.CreateMatchRequest{MatchID: "demotehim_flow"}, now)
	cardID := cardIDByMechanic(t, snapshot.Match.WhiteHand, "demotehim")

	state := service.matches["demotehim_flow"]
	state.Board = emptyBoard()
	state.Board[0][0] = &contracts.Piece{Type: "king", Color: "white"}
	state.Board[7][7] = &contracts.Piece{Type: "king", Color: "black"}
	state.Board[3][3] = &contracts.Piece{Type: "queen", Color: "white"}

	if _, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "play_card",
		MatchID:  "demotehim_flow",
		PlayerID: "white_player",
		CardID:   cardID,
	}, now.Add(time.Second)); err != nil {
		t.Fatalf("expected demotehim play_card to succeed, got %v", err)
	}

	targeted, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "select_target",
		MatchID:  "demotehim_flow",
		PlayerID: "white_player",
		Target:   &contracts.Square{Row: 3, Col: 3},
	}, now.Add(2*time.Second))
	if err != nil {
		t.Fatalf("expected demotehim target step to succeed, got %v", err)
	}

	if !containsString(targeted.Match.PendingCard.Options, "pawn") {
		t.Fatalf("expected demotehim options to include pawn, got %#v", targeted.Match.PendingCard.Options)
	}

	resolved, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:        "select_target",
		MatchID:     "demotehim_flow",
		PlayerID:    "white_player",
		SelectionID: "pawn",
	}, now.Add(3*time.Second))
	if err != nil {
		t.Fatalf("expected demotehim selection step to succeed, got %v", err)
	}

	if piece := service.matches["demotehim_flow"].Board[3][3]; piece == nil || piece.Type != "pawn" || piece.Color != "white" {
		t.Fatalf("expected targeted piece to become white pawn, got %#v", piece)
	}
	if len(resolved.Match.WhiteHand) != len(snapshot.Match.WhiteHand)-1 {
		t.Fatalf("expected one demotehim card to be consumed, got %d cards", len(resolved.Match.WhiteHand))
	}
}

func TestTeleportFlowSelectsSourceThenDestination(t *testing.T) {
	service := NewService()
	now := time.Date(2026, 5, 5, 8, 0, 0, 0, time.UTC)
	snapshot := service.CreateMatch(contracts.CreateMatchRequest{MatchID: "teleport_flow"}, now)
	cardID := cardIDByMechanic(t, snapshot.Match.WhiteHand, "teleport")

	if _, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "play_card",
		MatchID:  "teleport_flow",
		PlayerID: "white_player",
		CardID:   cardID,
	}, now.Add(time.Second)); err != nil {
		t.Fatalf("expected teleport play_card to succeed, got %v", err)
	}

	step1, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "select_target",
		MatchID:  "teleport_flow",
		PlayerID: "white_player",
		Target:   &contracts.Square{Row: 1, Col: 0},
	}, now.Add(2*time.Second))
	if err != nil {
		t.Fatalf("expected teleport source selection to succeed, got %v", err)
	}

	if step1.Match.PendingCard == nil || step1.Match.PendingCard.Target == nil {
		t.Fatalf("expected teleport pending source to be stored, got %#v", step1.Match.PendingCard)
	}

	step2, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "select_target",
		MatchID:  "teleport_flow",
		PlayerID: "white_player",
		Target:   &contracts.Square{Row: 4, Col: 4},
	}, now.Add(3*time.Second))
	if err != nil {
		t.Fatalf("expected teleport destination selection to succeed, got %v", err)
	}

	if piece := service.matches["teleport_flow"].Board[4][4]; piece == nil || piece.Type != "pawn" || piece.Color != "white" {
		t.Fatalf("expected white pawn teleported to e5, got %#v", piece)
	}
	if source := service.matches["teleport_flow"].Board[1][0]; source != nil {
		t.Fatalf("expected original square to be empty after teleport, got %#v", source)
	}
	if step2.Match.PendingCard != nil {
		t.Fatalf("expected teleport pending state to clear after destination selection")
	}
	if len(step2.Match.WhiteHand) != len(snapshot.Match.WhiteHand)-1 {
		t.Fatalf("expected one teleport card to be consumed, got %d cards", len(step2.Match.WhiteHand))
	}
}

func TestJumpFlowSelectsSourceThenDestination(t *testing.T) {
	service := NewService()
	now := time.Date(2026, 5, 5, 8, 0, 0, 0, time.UTC)
	snapshot := service.CreateMatch(contracts.CreateMatchRequest{MatchID: "jump_flow"}, now)
	cardID := cardIDByMechanic(t, snapshot.Match.WhiteHand, "jump")

	state := service.matches["jump_flow"]
	state.Board = emptyBoard()
	state.Board[0][0] = &contracts.Piece{Type: "king", Color: "white"}
	state.Board[7][7] = &contracts.Piece{Type: "king", Color: "black"}
	state.Board[3][3] = &contracts.Piece{Type: "rook", Color: "white"}
	state.Board[3][4] = &contracts.Piece{Type: "pawn", Color: "black"}

	if _, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "play_card",
		MatchID:  "jump_flow",
		PlayerID: "white_player",
		CardID:   cardID,
	}, now.Add(time.Second)); err != nil {
		t.Fatalf("expected jump play_card to succeed, got %v", err)
	}

	step1, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "select_target",
		MatchID:  "jump_flow",
		PlayerID: "white_player",
		Target:   &contracts.Square{Row: 3, Col: 3},
	}, now.Add(2*time.Second))
	if err != nil {
		t.Fatalf("expected jump source selection to succeed, got %v", err)
	}

	if step1.Match.PendingCard == nil || step1.Match.PendingCard.Target == nil {
		t.Fatalf("expected jump pending source to be stored, got %#v", step1.Match.PendingCard)
	}

	step2, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "select_target",
		MatchID:  "jump_flow",
		PlayerID: "white_player",
		Target:   &contracts.Square{Row: 3, Col: 5},
	}, now.Add(3*time.Second))
	if err != nil {
		t.Fatalf("expected jump destination selection to succeed, got %v", err)
	}

	if piece := service.matches["jump_flow"].Board[3][5]; piece == nil || piece.Type != "rook" || piece.Color != "white" {
		t.Fatalf("expected white rook to land on jump destination, got %#v", piece)
	}
	if source := service.matches["jump_flow"].Board[3][3]; source != nil {
		t.Fatalf("expected original jump square to be empty, got %#v", source)
	}
	if step2.Match.PendingCard != nil {
		t.Fatalf("expected jump pending state to clear after destination selection")
	}
	if len(step2.Match.WhiteHand) != len(snapshot.Match.WhiteHand)-1 {
		t.Fatalf("expected one jump card to be consumed, got %d cards", len(step2.Match.WhiteHand))
	}
}

func TestSwapMeFlowSelectsTwoOwnedPieces(t *testing.T) {
	service := NewService()
	now := time.Date(2026, 5, 5, 8, 0, 0, 0, time.UTC)
	snapshot := service.CreateMatch(contracts.CreateMatchRequest{MatchID: "swapme_flow"}, now)
	cardID := cardIDByMechanic(t, snapshot.Match.WhiteHand, "swapme")

	state := service.matches["swapme_flow"]
	state.Board = emptyBoard()
	state.Board[0][0] = &contracts.Piece{Type: "king", Color: "white"}
	state.Board[7][7] = &contracts.Piece{Type: "king", Color: "black"}
	state.Board[2][2] = &contracts.Piece{Type: "knight", Color: "white"}
	state.Board[4][4] = &contracts.Piece{Type: "rook", Color: "white"}

	if _, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "play_card",
		MatchID:  "swapme_flow",
		PlayerID: "white_player",
		CardID:   cardID,
	}, now.Add(time.Second)); err != nil {
		t.Fatalf("expected swapme play_card to succeed, got %v", err)
	}

	step1, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "select_target",
		MatchID:  "swapme_flow",
		PlayerID: "white_player",
		Target:   &contracts.Square{Row: 2, Col: 2},
	}, now.Add(2*time.Second))
	if err != nil {
		t.Fatalf("expected swapme first selection to succeed, got %v", err)
	}

	if step1.Match.PendingCard == nil || step1.Match.PendingCard.Target == nil {
		t.Fatalf("expected swapme pending first square to be stored, got %#v", step1.Match.PendingCard)
	}

	step2, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "select_target",
		MatchID:  "swapme_flow",
		PlayerID: "white_player",
		Target:   &contracts.Square{Row: 4, Col: 4},
	}, now.Add(3*time.Second))
	if err != nil {
		t.Fatalf("expected swapme second selection to succeed, got %v", err)
	}

	if piece := service.matches["swapme_flow"].Board[2][2]; piece == nil || piece.Type != "rook" || piece.Color != "white" {
		t.Fatalf("expected rook to move onto first swap square, got %#v", piece)
	}
	if piece := service.matches["swapme_flow"].Board[4][4]; piece == nil || piece.Type != "knight" || piece.Color != "white" {
		t.Fatalf("expected knight to move onto second swap square, got %#v", piece)
	}
	if step2.Match.PendingCard != nil {
		t.Fatalf("expected swapme pending state to clear after second selection")
	}
	if len(step2.Match.WhiteHand) != len(snapshot.Match.WhiteHand)-1 {
		t.Fatalf("expected one swapme card to be consumed, got %d cards", len(step2.Match.WhiteHand))
	}
}

func TestSwapUsFlowSelectsOwnedThenEnemyPiece(t *testing.T) {
	service := NewService()
	now := time.Date(2026, 5, 5, 8, 0, 0, 0, time.UTC)
	snapshot := service.CreateMatch(contracts.CreateMatchRequest{MatchID: "swapus_flow"}, now)
	cardID := cardIDByMechanic(t, snapshot.Match.WhiteHand, "swapus")

	state := service.matches["swapus_flow"]
	state.Board = emptyBoard()
	state.Board[0][0] = &contracts.Piece{Type: "king", Color: "white"}
	state.Board[7][7] = &contracts.Piece{Type: "king", Color: "black"}
	state.Board[2][2] = &contracts.Piece{Type: "rook", Color: "white"}
	state.Board[5][5] = &contracts.Piece{Type: "knight", Color: "black"}

	if _, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "play_card",
		MatchID:  "swapus_flow",
		PlayerID: "white_player",
		CardID:   cardID,
	}, now.Add(time.Second)); err != nil {
		t.Fatalf("expected swapus play_card to succeed, got %v", err)
	}

	step1, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "select_target",
		MatchID:  "swapus_flow",
		PlayerID: "white_player",
		Target:   &contracts.Square{Row: 2, Col: 2},
	}, now.Add(2*time.Second))
	if err != nil {
		t.Fatalf("expected swapus first selection to succeed, got %v", err)
	}

	if step1.Match.PendingCard == nil || step1.Match.PendingCard.Target == nil {
		t.Fatalf("expected swapus pending first square to be stored, got %#v", step1.Match.PendingCard)
	}

	step2, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "select_target",
		MatchID:  "swapus_flow",
		PlayerID: "white_player",
		Target:   &contracts.Square{Row: 5, Col: 5},
	}, now.Add(3*time.Second))
	if err != nil {
		t.Fatalf("expected swapus second selection to succeed, got %v", err)
	}

	if piece := service.matches["swapus_flow"].Board[2][2]; piece == nil || piece.Type != "knight" || piece.Color != "black" {
		t.Fatalf("expected enemy knight to move onto first swap square, got %#v", piece)
	}
	if piece := service.matches["swapus_flow"].Board[5][5]; piece == nil || piece.Type != "rook" || piece.Color != "white" {
		t.Fatalf("expected white rook to move onto enemy square, got %#v", piece)
	}
	if step2.Match.PendingCard != nil {
		t.Fatalf("expected swapus pending state to clear after second selection")
	}
	if len(step2.Match.WhiteHand) != len(snapshot.Match.WhiteHand)-1 {
		t.Fatalf("expected one swapus card to be consumed, got %d cards", len(step2.Match.WhiteHand))
	}
}

func TestSwapHimFlowSelectsTwoEnemyPieces(t *testing.T) {
	service := NewService()
	now := time.Date(2026, 5, 5, 8, 0, 0, 0, time.UTC)
	snapshot := service.CreateMatch(contracts.CreateMatchRequest{MatchID: "swaphim_flow"}, now)
	cardID := cardIDByMechanic(t, snapshot.Match.WhiteHand, "swaphim")

	state := service.matches["swaphim_flow"]
	state.Board = emptyBoard()
	state.Board[0][0] = &contracts.Piece{Type: "king", Color: "white"}
	state.Board[7][7] = &contracts.Piece{Type: "king", Color: "black"}
	state.Board[5][5] = &contracts.Piece{Type: "knight", Color: "black"}
	state.Board[6][4] = &contracts.Piece{Type: "rook", Color: "black"}

	if _, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "play_card",
		MatchID:  "swaphim_flow",
		PlayerID: "white_player",
		CardID:   cardID,
	}, now.Add(time.Second)); err != nil {
		t.Fatalf("expected swaphim play_card to succeed, got %v", err)
	}

	step1, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "select_target",
		MatchID:  "swaphim_flow",
		PlayerID: "white_player",
		Target:   &contracts.Square{Row: 5, Col: 5},
	}, now.Add(2*time.Second))
	if err != nil {
		t.Fatalf("expected swaphim first selection to succeed, got %v", err)
	}

	if step1.Match.PendingCard == nil || step1.Match.PendingCard.Target == nil {
		t.Fatalf("expected swaphim pending first square to be stored, got %#v", step1.Match.PendingCard)
	}

	step2, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "select_target",
		MatchID:  "swaphim_flow",
		PlayerID: "white_player",
		Target:   &contracts.Square{Row: 6, Col: 4},
	}, now.Add(3*time.Second))
	if err != nil {
		t.Fatalf("expected swaphim second selection to succeed, got %v", err)
	}

	if piece := service.matches["swaphim_flow"].Board[5][5]; piece == nil || piece.Type != "rook" || piece.Color != "black" {
		t.Fatalf("expected enemy rook to move onto first swap square, got %#v", piece)
	}
	if piece := service.matches["swaphim_flow"].Board[6][4]; piece == nil || piece.Type != "knight" || piece.Color != "black" {
		t.Fatalf("expected enemy knight to move onto second swap square, got %#v", piece)
	}
	if step2.Match.PendingCard != nil {
		t.Fatalf("expected swaphim pending state to clear after second selection")
	}
	if len(step2.Match.WhiteHand) != len(snapshot.Match.WhiteHand)-1 {
		t.Fatalf("expected one swaphim card to be consumed, got %d cards", len(step2.Match.WhiteHand))
	}
}

func TestBorrowTemporarilyTransfersControlUntilTurnEnds(t *testing.T) {
	service := NewService()
	now := time.Date(2026, 5, 5, 8, 0, 0, 0, time.UTC)
	snapshot := service.CreateMatch(contracts.CreateMatchRequest{MatchID: "borrow_flow"}, now)
	cardID := cardIDByMechanic(t, snapshot.Match.WhiteHand, "borrow")

	state := service.matches["borrow_flow"]
	state.Board = emptyBoard()
	state.Board[0][0] = &contracts.Piece{Type: "king", Color: "white"}
	state.Board[7][7] = &contracts.Piece{Type: "king", Color: "black"}
	state.Board[3][3] = &contracts.Piece{Type: "rook", Color: "black"}

	if _, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "play_card",
		MatchID:  "borrow_flow",
		PlayerID: "white_player",
		CardID:   cardID,
	}, now.Add(time.Second)); err != nil {
		t.Fatalf("expected borrow play_card to succeed, got %v", err)
	}

	result, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "select_target",
		MatchID:  "borrow_flow",
		PlayerID: "white_player",
		Target:   &contracts.Square{Row: 3, Col: 3},
	}, now.Add(2*time.Second))
	if err != nil {
		t.Fatalf("expected borrow target selection to succeed, got %v", err)
	}

	if piece := service.matches["borrow_flow"].Board[3][3]; piece == nil || piece.Color != "white" || !piece.Borrowed {
		t.Fatalf("expected borrowed piece to become temporarily white, got %#v", piece)
	}
	if len(result.Match.WhiteHand) != len(snapshot.Match.WhiteHand)-1 {
		t.Fatalf("expected one borrow card to be consumed, got %d cards", len(result.Match.WhiteHand))
	}

	if _, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "make_move",
		MatchID:  "borrow_flow",
		PlayerID: "white_player",
		From:     &contracts.Square{Row: 3, Col: 3},
		To:       &contracts.Square{Row: 3, Col: 6},
	}, now.Add(3*time.Second)); err != nil {
		t.Fatalf("expected borrowed piece to be movable by white, got %v", err)
	}

	if piece := service.matches["borrow_flow"].Board[3][6]; piece == nil || piece.Color != "black" || piece.Borrowed {
		t.Fatalf("expected borrowed piece to revert to black after turn end, got %#v", piece)
	}
}

func TestMindControlPermanentlyTransfersControl(t *testing.T) {
	service := NewService()
	now := time.Date(2026, 5, 5, 8, 0, 0, 0, time.UTC)
	snapshot := service.CreateMatch(contracts.CreateMatchRequest{MatchID: "mindcontrol_flow"}, now)
	cardID := cardIDByMechanic(t, snapshot.Match.WhiteHand, "mindcontrol")

	state := service.matches["mindcontrol_flow"]
	state.Board = emptyBoard()
	state.Board[0][0] = &contracts.Piece{Type: "king", Color: "white"}
	state.Board[7][7] = &contracts.Piece{Type: "king", Color: "black"}
	state.Board[3][3] = &contracts.Piece{Type: "rook", Color: "black"}

	result, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "play_card",
		MatchID:  "mindcontrol_flow",
		PlayerID: "white_player",
		CardID:   cardID,
	}, now.Add(time.Second))
	if err != nil {
		t.Fatalf("expected mindcontrol play_card to succeed, got %v", err)
	}
	if result.Match.PendingCard == nil {
		t.Fatalf("expected pending mindcontrol state after play_card")
	}

	result, err = service.ApplyIntent(contracts.PlayerIntent{
		Type:     "select_target",
		MatchID:  "mindcontrol_flow",
		PlayerID: "white_player",
		Target:   &contracts.Square{Row: 3, Col: 3},
	}, now.Add(2*time.Second))
	if err != nil {
		t.Fatalf("expected mindcontrol target selection to succeed, got %v", err)
	}

	if piece := service.matches["mindcontrol_flow"].Board[3][3]; piece == nil || piece.Color != "white" || piece.Borrowed {
		t.Fatalf("expected mind-controlled piece to become permanently white, got %#v", piece)
	}
	if len(result.Match.WhiteHand) != len(snapshot.Match.WhiteHand)-1 {
		t.Fatalf("expected one mindcontrol card to be consumed, got %d cards", len(result.Match.WhiteHand))
	}

	if _, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "make_move",
		MatchID:  "mindcontrol_flow",
		PlayerID: "white_player",
		From:     &contracts.Square{Row: 3, Col: 3},
		To:       &contracts.Square{Row: 3, Col: 6},
	}, now.Add(3*time.Second)); err != nil {
		t.Fatalf("expected mind-controlled piece to be movable by white, got %v", err)
	}

	if piece := service.matches["mindcontrol_flow"].Board[3][6]; piece == nil || piece.Color != "white" || piece.Borrowed {
		t.Fatalf("expected mind-controlled piece to stay white after turn end, got %#v", piece)
	}
}

func TestParasiteLinksHostToEqualValueEnemyPiece(t *testing.T) {
	service := NewService()
	now := time.Date(2026, 5, 5, 8, 0, 0, 0, time.UTC)
	snapshot := service.CreateMatch(contracts.CreateMatchRequest{MatchID: "parasite_flow"}, now)
	cardID := cardIDByMechanic(t, snapshot.Match.WhiteHand, "parasite")

	state := service.matches["parasite_flow"]
	state.Board = emptyBoard()
	state.Board[0][0] = &contracts.Piece{Type: "king", Color: "white"}
	state.Board[7][7] = &contracts.Piece{Type: "king", Color: "black"}
	state.Board[3][3] = &contracts.Piece{Type: "rook", Color: "white"}
	state.Board[5][5] = &contracts.Piece{Type: "rook", Color: "black"}

	if _, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "play_card",
		MatchID:  "parasite_flow",
		PlayerID: "white_player",
		CardID:   cardID,
	}, now.Add(time.Second)); err != nil {
		t.Fatalf("expected parasite play_card to succeed, got %v", err)
	}

	step1, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "select_target",
		MatchID:  "parasite_flow",
		PlayerID: "white_player",
		Target:   &contracts.Square{Row: 3, Col: 3},
	}, now.Add(2*time.Second))
	if err != nil {
		t.Fatalf("expected parasite host selection to succeed, got %v", err)
	}

	if step1.Match.PendingCard == nil || step1.Match.PendingCard.Target == nil || len(step1.Match.PendingCard.Options) != 1 || step1.Match.PendingCard.Options[0] != "5" {
		t.Fatalf("expected parasite host value metadata, got %#v", step1.Match.PendingCard)
	}

	result, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "select_target",
		MatchID:  "parasite_flow",
		PlayerID: "white_player",
		Target:   &contracts.Square{Row: 5, Col: 5},
	}, now.Add(3*time.Second))
	if err != nil {
		t.Fatalf("expected parasite target selection to succeed, got %v", err)
	}

	if piece := service.matches["parasite_flow"].Board[3][3]; piece == nil || piece.ParasiteTarget != "5,5" {
		t.Fatalf("expected host to store parasite target, got %#v", piece)
	}
	if len(result.Match.WhiteHand) != len(snapshot.Match.WhiteHand)-1 {
		t.Fatalf("expected one parasite card to be consumed, got %d cards", len(result.Match.WhiteHand))
	}
}

func TestParasiteTriggerRemovesHostWhenLinkedTargetDies(t *testing.T) {
	service := NewService()
	now := time.Date(2026, 5, 5, 8, 0, 0, 0, time.UTC)
	service.CreateMatch(contracts.CreateMatchRequest{MatchID: "parasite_trigger"}, now)

	state := service.matches["parasite_trigger"]
	state.Board = emptyBoard()
	state.Board[0][0] = &contracts.Piece{Type: "king", Color: "white"}
	state.Board[7][7] = &contracts.Piece{Type: "king", Color: "black"}
	state.Board[3][3] = &contracts.Piece{Type: "rook", Color: "white", ParasiteTarget: "5,5"}
	state.Board[4][4] = &contracts.Piece{Type: "bishop", Color: "white"}
	state.Board[5][5] = &contracts.Piece{Type: "rook", Color: "black"}

	if _, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "make_move",
		MatchID:  "parasite_trigger",
		PlayerID: "white_player",
		From:     &contracts.Square{Row: 4, Col: 4},
		To:       &contracts.Square{Row: 5, Col: 5},
	}, now.Add(time.Second)); err != nil {
		t.Fatalf("expected parasite trigger capture to succeed, got %v", err)
	}

	if host := service.matches["parasite_trigger"].Board[3][3]; host != nil {
		t.Fatalf("expected host piece to die when linked target died, got %#v", host)
	}
}

func TestParasiteRejectedCaptureDoesNotMutateBoard(t *testing.T) {
	service := NewService()
	now := time.Date(2026, 5, 5, 8, 0, 0, 0, time.UTC)
	service.CreateMatch(contracts.CreateMatchRequest{MatchID: "parasite_reject"}, now)

	state := service.matches["parasite_reject"]
	state.Board = emptyBoard()
	state.Board[0][0] = &contracts.Piece{Type: "king", Color: "white"}
	state.Board[0][1] = &contracts.Piece{Type: "rook", Color: "white", ParasiteTarget: "2,2"}
	state.Board[1][1] = &contracts.Piece{Type: "bishop", Color: "white"}
	state.Board[2][2] = &contracts.Piece{Type: "rook", Color: "black"}
	state.Board[0][7] = &contracts.Piece{Type: "rook", Color: "black"}
	state.Board[7][7] = &contracts.Piece{Type: "king", Color: "black"}

	_, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "make_move",
		MatchID:  "parasite_reject",
		PlayerID: "white_player",
		From:     &contracts.Square{Row: 1, Col: 1},
		To:       &contracts.Square{Row: 2, Col: 2},
	}, now.Add(time.Second))
	if err == nil || !strings.Contains(err.Error(), "parasite") {
		t.Fatalf("expected parasite rejection, got %v", err)
	}

	if piece := service.matches["parasite_reject"].Board[1][1]; piece == nil || piece.Type != "bishop" || piece.Color != "white" {
		t.Fatalf("expected moving bishop to remain on original square, got %#v", piece)
	}
	if piece := service.matches["parasite_reject"].Board[2][2]; piece == nil || piece.Type != "rook" || piece.Color != "black" {
		t.Fatalf("expected captured target to remain on board after rejection, got %#v", piece)
	}
	if piece := service.matches["parasite_reject"].Board[0][1]; piece == nil || piece.Type != "rook" || piece.Color != "white" {
		t.Fatalf("expected parasite host to remain on board after rejection, got %#v", piece)
	}
}

func TestCloneFlowSelectsSourceThenAdjacentEmptySquare(t *testing.T) {
	service := NewService()
	now := time.Date(2026, 5, 5, 8, 0, 0, 0, time.UTC)
	snapshot := service.CreateMatch(contracts.CreateMatchRequest{MatchID: "clone_flow"}, now)
	cardID := cardIDByMechanic(t, snapshot.Match.WhiteHand, "clone")

	state := service.matches["clone_flow"]
	state.Board = emptyBoard()
	state.Board[0][0] = &contracts.Piece{Type: "king", Color: "white"}
	state.Board[7][7] = &contracts.Piece{Type: "king", Color: "black"}
	state.Board[3][3] = &contracts.Piece{Type: "knight", Color: "white"}

	if _, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "play_card",
		MatchID:  "clone_flow",
		PlayerID: "white_player",
		CardID:   cardID,
	}, now.Add(time.Second)); err != nil {
		t.Fatalf("expected clone play_card to succeed, got %v", err)
	}

	step1, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "select_target",
		MatchID:  "clone_flow",
		PlayerID: "white_player",
		Target:   &contracts.Square{Row: 3, Col: 3},
	}, now.Add(2*time.Second))
	if err != nil {
		t.Fatalf("expected clone source selection to succeed, got %v", err)
	}

	if step1.Match.PendingCard == nil || step1.Match.PendingCard.Target == nil {
		t.Fatalf("expected clone pending source to be stored, got %#v", step1.Match.PendingCard)
	}

	step2, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "select_target",
		MatchID:  "clone_flow",
		PlayerID: "white_player",
		Target:   &contracts.Square{Row: 4, Col: 4},
	}, now.Add(3*time.Second))
	if err != nil {
		t.Fatalf("expected clone destination selection to succeed, got %v", err)
	}

	if piece := service.matches["clone_flow"].Board[4][4]; piece == nil || piece.Type != "knight" || piece.Color != "white" {
		t.Fatalf("expected cloned knight on destination square, got %#v", piece)
	}
	if source := service.matches["clone_flow"].Board[3][3]; source == nil || source.Type != "knight" || source.Color != "white" {
		t.Fatalf("expected source knight to remain in place, got %#v", source)
	}
	if step2.Match.PendingCard != nil {
		t.Fatalf("expected clone pending state to clear after destination selection")
	}
	if len(step2.Match.WhiteHand) != len(snapshot.Match.WhiteHand)-1 {
		t.Fatalf("expected one clone card to be consumed, got %d cards", len(step2.Match.WhiteHand))
	}
}

func TestLavaGroundPlacementConsumesCardAndStoresTrap(t *testing.T) {
	service := NewService()
	now := time.Date(2026, 5, 5, 8, 0, 0, 0, time.UTC)
	snapshot := service.CreateMatch(contracts.CreateMatchRequest{MatchID: "lava_place"}, now)
	cardID := cardIDByMechanic(t, snapshot.Match.WhiteHand, "lavaground")

	if _, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "play_card",
		MatchID:  "lava_place",
		PlayerID: "white_player",
		CardID:   cardID,
	}, now.Add(time.Second)); err != nil {
		t.Fatalf("expected lavaground play_card to succeed, got %v", err)
	}

	result, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "select_target",
		MatchID:  "lava_place",
		PlayerID: "white_player",
		Target:   &contracts.Square{Row: 4, Col: 4},
	}, now.Add(2*time.Second))
	if err != nil {
		t.Fatalf("expected lavaground target selection to succeed, got %v", err)
	}

	if len(service.matches["lava_place"].LavaSquares) != 1 {
		t.Fatalf("expected one active lava square, got %#v", service.matches["lava_place"].LavaSquares)
	}
	lava := service.matches["lava_place"].LavaSquares[0]
	if lava.Row != 4 || lava.Col != 4 || lava.MovesLeft != 2 {
		t.Fatalf("expected lava trap at e5 with 2 moves left, got %#v", lava)
	}
	if len(result.Match.WhiteHand) != len(snapshot.Match.WhiteHand)-1 {
		t.Fatalf("expected lavaground card to be consumed")
	}
}

func TestLavaGroundBurnsLandingPieceAndTicksOtherTraps(t *testing.T) {
	service := NewService()
	now := time.Date(2026, 5, 5, 8, 0, 0, 0, time.UTC)
	service.CreateMatch(contracts.CreateMatchRequest{MatchID: "lava_trigger"}, now)

	state := service.matches["lava_trigger"]
	state.Board = emptyBoard()
	state.Board[0][4] = &contracts.Piece{Type: "king", Color: "white"}
	state.Board[7][4] = &contracts.Piece{Type: "king", Color: "black"}
	state.Board[1][4] = &contracts.Piece{Type: "pawn", Color: "white"}
	state.Board[6][0] = &contracts.Piece{Type: "pawn", Color: "black"}
	state.LavaSquares = []contracts.LavaSquare{
		{Row: 3, Col: 4, MovesLeft: 2},
		{Row: 5, Col: 5, MovesLeft: 2},
	}

	result, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "make_move",
		MatchID:  "lava_trigger",
		PlayerID: "white_player",
		From:     &contracts.Square{Row: 1, Col: 4},
		To:       &contracts.Square{Row: 3, Col: 4},
	}, now.Add(2*time.Second))
	if err != nil {
		t.Fatalf("expected move onto lava to succeed, got %v", err)
	}

	if piece := service.matches["lava_trigger"].Board[3][4]; piece != nil {
		t.Fatalf("expected landing piece to be burned by lava, got %#v", piece)
	}
	if len(service.matches["lava_trigger"].LavaSquares) != 1 {
		t.Fatalf("expected triggered lava to be removed and other lava to decay, got %#v", service.matches["lava_trigger"].LavaSquares)
	}
	other := service.matches["lava_trigger"].LavaSquares[0]
	if other.Row != 5 || other.Col != 5 || other.MovesLeft != 1 {
		t.Fatalf("expected other lava trap to decay to 1 move left, got %#v", other)
	}
	if len(result.Events) == 0 || result.Events[0].Payload["lavaTriggered"] != true {
		t.Fatalf("expected lava trigger event payload, got %#v", result.Events)
	}
}

func TestInvisibleRemovesPieceFromBoardAndStoresGhostState(t *testing.T) {
	service := NewService()
	now := time.Date(2026, 5, 5, 8, 0, 0, 0, time.UTC)
	snapshot := service.CreateMatch(contracts.CreateMatchRequest{MatchID: "invisible_apply"}, now)
	cardID := cardIDByMechanic(t, snapshot.Match.WhiteHand, "invisible")

	state := service.matches["invisible_apply"]
	state.Board = emptyBoard()
	state.Board[0][4] = &contracts.Piece{Type: "king", Color: "white"}
	state.Board[7][4] = &contracts.Piece{Type: "king", Color: "black"}
	state.Board[0][0] = &contracts.Piece{Type: "rook", Color: "white"}

	if _, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "play_card",
		MatchID:  "invisible_apply",
		PlayerID: "white_player",
		CardID:   cardID,
	}, now.Add(time.Second)); err != nil {
		t.Fatalf("expected invisible play_card to succeed, got %v", err)
	}

	result, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "select_target",
		MatchID:  "invisible_apply",
		PlayerID: "white_player",
		Target:   &contracts.Square{Row: 0, Col: 0},
	}, now.Add(2*time.Second))
	if err != nil {
		t.Fatalf("expected invisible target selection to succeed, got %v", err)
	}

	if piece := service.matches["invisible_apply"].Board[0][0]; piece != nil {
		t.Fatalf("expected invisible piece to be removed from the board, got %#v", piece)
	}
	if service.matches["invisible_apply"].InvisiblePiece == nil {
		t.Fatalf("expected invisible ghost state to be stored")
	}
	if service.matches["invisible_apply"].InvisiblePiece.Row != 0 || service.matches["invisible_apply"].InvisiblePiece.Col != 0 {
		t.Fatalf("expected invisible ghost to start on a1, got %#v", service.matches["invisible_apply"].InvisiblePiece)
	}
	if len(result.Match.WhiteHand) != len(snapshot.Match.WhiteHand)-1 {
		t.Fatalf("expected invisible card to be consumed")
	}
}

func TestInvisibleMoveMaterializesWhenGivingCheck(t *testing.T) {
	service := NewService()
	now := time.Date(2026, 5, 5, 8, 0, 0, 0, time.UTC)
	snapshot := service.CreateMatch(contracts.CreateMatchRequest{MatchID: "invisible_move"}, now)
	cardID := cardIDByMechanic(t, snapshot.Match.WhiteHand, "invisible")

	state := service.matches["invisible_move"]
	state.Board = emptyBoard()
	state.Board[0][4] = &contracts.Piece{Type: "king", Color: "white"}
	state.Board[7][4] = &contracts.Piece{Type: "king", Color: "black"}
	state.Board[0][0] = &contracts.Piece{Type: "rook", Color: "white"}

	if _, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "play_card",
		MatchID:  "invisible_move",
		PlayerID: "white_player",
		CardID:   cardID,
	}, now.Add(time.Second)); err != nil {
		t.Fatalf("expected invisible play_card to succeed, got %v", err)
	}
	if _, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "select_target",
		MatchID:  "invisible_move",
		PlayerID: "white_player",
		Target:   &contracts.Square{Row: 0, Col: 0},
	}, now.Add(2*time.Second)); err != nil {
		t.Fatalf("expected invisible target selection to succeed, got %v", err)
	}

	result, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "make_move",
		MatchID:  "invisible_move",
		PlayerID: "white_player",
		From:     &contracts.Square{Row: 0, Col: 0},
		To:       &contracts.Square{Row: 7, Col: 0},
	}, now.Add(3*time.Second))
	if err != nil {
		t.Fatalf("expected invisible move to succeed, got %v", err)
	}

	if service.matches["invisible_move"].InvisiblePiece != nil {
		t.Fatalf("expected invisible piece to materialize after giving check")
	}
	if piece := service.matches["invisible_move"].Board[7][0]; piece == nil || piece.Type != "rook" || piece.Color != "white" {
		t.Fatalf("expected rook to materialize on a8, got %#v", piece)
	}
	if len(result.Events) == 0 || result.Events[0].Payload["materialized"] != "check" {
		t.Fatalf("expected invisible move to record check materialization, got %#v", result.Events)
	}
}

func TestUnabomberAttachesBombAndConsumesCard(t *testing.T) {
	service := NewService()
	now := time.Date(2026, 5, 5, 8, 0, 0, 0, time.UTC)
	snapshot := service.CreateMatch(contracts.CreateMatchRequest{MatchID: "unabomber_apply"}, now)
	cardID := cardIDByMechanic(t, snapshot.Match.WhiteHand, "unabomber")

	state := service.matches["unabomber_apply"]
	state.Board = emptyBoard()
	state.Board[0][4] = &contracts.Piece{Type: "king", Color: "white"}
	state.Board[7][4] = &contracts.Piece{Type: "king", Color: "black"}
	state.Board[1][0] = &contracts.Piece{Type: "pawn", Color: "white"}

	if _, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "play_card",
		MatchID:  "unabomber_apply",
		PlayerID: "white_player",
		CardID:   cardID,
	}, now.Add(time.Second)); err != nil {
		t.Fatalf("expected unabomber play_card to succeed, got %v", err)
	}

	result, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "select_target",
		MatchID:  "unabomber_apply",
		PlayerID: "white_player",
		Target:   &contracts.Square{Row: 1, Col: 0},
	}, now.Add(2*time.Second))
	if err != nil {
		t.Fatalf("expected unabomber target selection to succeed, got %v", err)
	}

	if piece := service.matches["unabomber_apply"].Board[1][0]; piece == nil || !piece.Bomb {
		t.Fatalf("expected target piece to carry a bomb, got %#v", piece)
	}
	if len(service.matches["unabomber_apply"].BombPieces) != 1 {
		t.Fatalf("expected one tracked bomb, got %#v", service.matches["unabomber_apply"].BombPieces)
	}
	if len(result.Match.WhiteHand) != len(snapshot.Match.WhiteHand)-1 {
		t.Fatalf("expected unabomber card to be consumed")
	}
}

func TestUnabomberExplodesOnWhiteTurnHandoff(t *testing.T) {
	service := NewService()
	now := time.Date(2026, 5, 5, 8, 0, 0, 0, time.UTC)
	service.CreateMatch(contracts.CreateMatchRequest{MatchID: "unabomber_explode"}, now)

	state := service.matches["unabomber_explode"]
	state.Board = emptyBoard()
	state.Board[0][4] = &contracts.Piece{Type: "king", Color: "white"}
	state.Board[7][4] = &contracts.Piece{Type: "king", Color: "black"}
	state.Board[6][6] = &contracts.Piece{Type: "pawn", Color: "black"}
	state.Board[4][4] = &contracts.Piece{Type: "rook", Color: "white", Bomb: true}
	state.Board[4][5] = &contracts.Piece{Type: "knight", Color: "white"}
	state.Board[5][4] = &contracts.Piece{Type: "bishop", Color: "black"}
	state.BombPieces = []contracts.BombPiece{{Row: 4, Col: 4, TurnsLeft: 1, OwnerColor: "white"}}
	state.Turn = "black"
	startedAt := now.UnixMilli()
	state.Clock.RunningFor = "black"
	state.Clock.StartedAt = &startedAt

	result, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "make_move",
		MatchID:  "unabomber_explode",
		PlayerID: "black_player",
		From:     &contracts.Square{Row: 6, Col: 6},
		To:       &contracts.Square{Row: 5, Col: 6},
	}, now.Add(2*time.Second))
	if err != nil {
		t.Fatalf("expected black move before bomb explosion to succeed, got %v", err)
	}

	if piece := service.matches["unabomber_explode"].Board[4][4]; piece != nil {
		t.Fatalf("expected bomb carrier to be destroyed, got %#v", piece)
	}
	if piece := service.matches["unabomber_explode"].Board[4][5]; piece != nil {
		t.Fatalf("expected adjacent white piece to be destroyed, got %#v", piece)
	}
	if piece := service.matches["unabomber_explode"].Board[5][4]; piece != nil {
		t.Fatalf("expected adjacent black piece to be destroyed, got %#v", piece)
	}
	if len(service.matches["unabomber_explode"].BombPieces) != 0 {
		t.Fatalf("expected bomb tracker to clear after explosion, got %#v", service.matches["unabomber_explode"].BombPieces)
	}
	if len(result.Events) == 0 || result.Events[0].Payload["bombExplodedSquares"] == nil {
		t.Fatalf("expected move payload to include exploded bomb squares, got %#v", result.Events)
	}
}

func TestUnabomberTrackerFollowsMovedCarrier(t *testing.T) {
	service := NewService()
	now := time.Date(2026, 5, 5, 8, 5, 0, 0, time.UTC)
	service.CreateMatch(contracts.CreateMatchRequest{MatchID: "unabomber_tracker"}, now)

	state := service.matches["unabomber_tracker"]
	state.Board = emptyBoard()
	state.Board[0][4] = &contracts.Piece{Type: "king", Color: "white"}
	state.Board[7][4] = &contracts.Piece{Type: "king", Color: "black"}
	state.Board[1][0] = &contracts.Piece{Type: "rook", Color: "white", Bomb: true}
	state.BombPieces = []contracts.BombPiece{{Row: 1, Col: 0, TurnsLeft: 2, OwnerColor: "white"}}
	startedAt := now.UnixMilli()
	state.Clock.RunningFor = "white"
	state.Clock.StartedAt = &startedAt

	if _, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "make_move",
		MatchID:  "unabomber_tracker",
		PlayerID: "white_player",
		From:     &contracts.Square{Row: 1, Col: 0},
		To:       &contracts.Square{Row: 3, Col: 0},
	}, now.Add(2*time.Second)); err != nil {
		t.Fatalf("expected bomb carrier move to succeed, got %v", err)
	}

	if len(state.BombPieces) != 1 {
		t.Fatalf("expected one tracked bomb after moving carrier, got %#v", state.BombPieces)
	}
	if state.BombPieces[0].Row != 3 || state.BombPieces[0].Col != 0 {
		t.Fatalf("expected bomb tracker to follow moved carrier to a4, got %#v", state.BombPieces[0])
	}
}

func TestRoundDrawAddsCardsOnAuthoritativeSchedule(t *testing.T) {
	service := NewService()
	now := time.Date(2026, 5, 5, 8, 10, 0, 0, time.UTC)
	snapshot := service.CreateMatch(contracts.CreateMatchRequest{
		MatchID:         "round_draw_schedule",
		StarterHandMode: "starter_three",
	}, now)

	state := service.matches["round_draw_schedule"]
	state.Board = emptyBoard()
	state.Board[0][4] = &contracts.Piece{Type: "king", Color: "white"}
	state.Board[7][4] = &contracts.Piece{Type: "king", Color: "black"}
	state.Board[6][0] = &contracts.Piece{Type: "pawn", Color: "black"}
	state.Turn = "black"
	state.FullMoveNum = 7
	startedAt := now.UnixMilli()
	state.Clock.RunningFor = "black"
	state.Clock.StartedAt = &startedAt

	result, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "make_move",
		MatchID:  "round_draw_schedule",
		PlayerID: "black_player",
		From:     &contracts.Square{Row: 6, Col: 0},
		To:       &contracts.Square{Row: 5, Col: 0},
	}, now.Add(2*time.Second))
	if err != nil {
		t.Fatalf("expected black move into draw round to succeed, got %v", err)
	}

	if len(result.Match.WhiteHand) != len(snapshot.Match.WhiteHand)+1 {
		t.Fatalf("expected white to draw one card on round start, got %d cards", len(result.Match.WhiteHand))
	}
	if len(result.Match.BlackHand) != len(snapshot.Match.BlackHand)+1 {
		t.Fatalf("expected black to draw one card on round start, got %d cards", len(result.Match.BlackHand))
	}
	if len(result.Events) == 0 {
		t.Fatalf("expected authoritative move events for scheduled round draw")
	}
	roundDrawWhite, ok := result.Events[0].Payload["roundDrawWhite"].([]contracts.GameCard)
	if !ok || len(roundDrawWhite) != 1 {
		t.Fatalf("expected roundDrawWhite payload with one card, got %#v", result.Events[0].Payload["roundDrawWhite"])
	}
	roundDrawBlack, ok := result.Events[0].Payload["roundDrawBlack"].([]contracts.GameCard)
	if !ok || len(roundDrawBlack) != 1 {
		t.Fatalf("expected roundDrawBlack payload with one card, got %#v", result.Events[0].Payload["roundDrawBlack"])
	}
}

func TestHalfFuseSelectsTwoPiecesAndAppliesFusion(t *testing.T) {
	service := NewService()
	now := time.Date(2026, 5, 5, 8, 0, 0, 0, time.UTC)
	snapshot := service.CreateMatch(contracts.CreateMatchRequest{MatchID: "halffuse_flow"}, now)
	cardID := cardIDByMechanic(t, snapshot.Match.WhiteHand, "halffuse")

	state := service.matches["halffuse_flow"]
	state.Board = emptyBoard()
	state.Board[0][4] = &contracts.Piece{Type: "king", Color: "white"}
	state.Board[7][4] = &contracts.Piece{Type: "king", Color: "black"}
	state.Board[3][3] = &contracts.Piece{Type: "pawn", Color: "white"}
	state.Board[3][4] = &contracts.Piece{Type: "knight", Color: "white"}

	if _, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "play_card",
		MatchID:  "halffuse_flow",
		PlayerID: "white_player",
		CardID:   cardID,
	}, now.Add(time.Second)); err != nil {
		t.Fatalf("expected halffuse play_card to succeed, got %v", err)
	}

	step1, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "select_target",
		MatchID:  "halffuse_flow",
		PlayerID: "white_player",
		Target:   &contracts.Square{Row: 3, Col: 3},
	}, now.Add(2*time.Second))
	if err != nil {
		t.Fatalf("expected halffuse first selection to succeed, got %v", err)
	}
	if step1.Match.PendingCard == nil || step1.Match.PendingCard.Target == nil || len(step1.Match.PendingCard.Options) < 2 {
		t.Fatalf("expected halffuse pending state with metadata, got %#v", step1.Match.PendingCard)
	}

	result, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "select_target",
		MatchID:  "halffuse_flow",
		PlayerID: "white_player",
		Target:   &contracts.Square{Row: 3, Col: 4},
	}, now.Add(3*time.Second))
	if err != nil {
		t.Fatalf("expected halffuse second selection to succeed, got %v", err)
	}

	if source := service.matches["halffuse_flow"].Board[3][3]; source != nil {
		t.Fatalf("expected first piece to be consumed by fusion, got %#v", source)
	}
	if fused := service.matches["halffuse_flow"].Board[3][4]; fused == nil || fused.Type != "knight" || fused.FusedWith != "pawn" {
		t.Fatalf("expected fused knight+pawn result, got %#v", fused)
	}
	if len(result.Match.WhiteHand) != len(snapshot.Match.WhiteHand)-1 {
		t.Fatalf("expected halffuse card to be consumed")
	}
}

func TestHalfFuseBishopAndRookBecomeQueen(t *testing.T) {
	service := NewService()
	now := time.Date(2026, 5, 5, 8, 0, 0, 0, time.UTC)
	snapshot := service.CreateMatch(contracts.CreateMatchRequest{MatchID: "halffuse_queen"}, now)
	cardID := cardIDByMechanic(t, snapshot.Match.WhiteHand, "halffuse")

	state := service.matches["halffuse_queen"]
	state.Board = emptyBoard()
	state.Board[0][4] = &contracts.Piece{Type: "king", Color: "white"}
	state.Board[7][7] = &contracts.Piece{Type: "king", Color: "black"}
	state.Board[3][3] = &contracts.Piece{Type: "bishop", Color: "white"}
	state.Board[3][4] = &contracts.Piece{Type: "rook", Color: "white"}

	if _, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "play_card",
		MatchID:  "halffuse_queen",
		PlayerID: "white_player",
		CardID:   cardID,
	}, now.Add(time.Second)); err != nil {
		t.Fatalf("expected halffuse play_card to succeed, got %v", err)
	}
	if _, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "select_target",
		MatchID:  "halffuse_queen",
		PlayerID: "white_player",
		Target:   &contracts.Square{Row: 3, Col: 3},
	}, now.Add(2*time.Second)); err != nil {
		t.Fatalf("expected halffuse first selection to succeed, got %v", err)
	}

	result, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "select_target",
		MatchID:  "halffuse_queen",
		PlayerID: "white_player",
		Target:   &contracts.Square{Row: 3, Col: 4},
	}, now.Add(3*time.Second))
	if err != nil {
		t.Fatalf("expected bishop+rook halffuse to succeed, got %v", err)
	}

	if fused := service.matches["halffuse_queen"].Board[3][4]; fused == nil || fused.Type != "queen" || fused.FusedWith != "" {
		t.Fatalf("expected bishop+rook to become queen, got %#v", fused)
	}
	if len(result.Match.WhiteHand) != len(snapshot.Match.WhiteHand)-1 {
		t.Fatalf("expected halffuse card to be consumed")
	}
}

func TestFullFusionSelectsTwoPiecesAndAppliesFusion(t *testing.T) {
	service := NewService()
	now := time.Date(2026, 5, 5, 8, 0, 0, 0, time.UTC)
	snapshot := service.CreateMatch(contracts.CreateMatchRequest{MatchID: "fullfusion_flow"}, now)
	cardID := cardIDByMechanic(t, snapshot.Match.WhiteHand, "fullfusion")

	state := service.matches["fullfusion_flow"]
	state.Board = emptyBoard()
	state.Board[0][4] = &contracts.Piece{Type: "king", Color: "white"}
	state.Board[7][7] = &contracts.Piece{Type: "king", Color: "black"}
	state.Board[3][3] = &contracts.Piece{Type: "queen", Color: "white"}
	state.Board[3][4] = &contracts.Piece{Type: "knight", Color: "white"}

	if _, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "play_card",
		MatchID:  "fullfusion_flow",
		PlayerID: "white_player",
		CardID:   cardID,
	}, now.Add(time.Second)); err != nil {
		t.Fatalf("expected fullfusion play_card to succeed, got %v", err)
	}

	step1, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "select_target",
		MatchID:  "fullfusion_flow",
		PlayerID: "white_player",
		Target:   &contracts.Square{Row: 3, Col: 3},
	}, now.Add(2*time.Second))
	if err != nil {
		t.Fatalf("expected fullfusion first selection to succeed, got %v", err)
	}
	if step1.Match.PendingCard == nil || step1.Match.PendingCard.Target == nil || len(step1.Match.PendingCard.Options) < 1 {
		t.Fatalf("expected fullfusion pending state with metadata, got %#v", step1.Match.PendingCard)
	}

	result, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "select_target",
		MatchID:  "fullfusion_flow",
		PlayerID: "white_player",
		Target:   &contracts.Square{Row: 3, Col: 4},
	}, now.Add(3*time.Second))
	if err != nil {
		t.Fatalf("expected fullfusion second selection to succeed, got %v", err)
	}

	if source := service.matches["fullfusion_flow"].Board[3][3]; source != nil {
		t.Fatalf("expected first piece to be consumed by full fusion, got %#v", source)
	}
	if fused := service.matches["fullfusion_flow"].Board[3][4]; fused == nil || fused.Type != "knight" || fused.FusedWith != "queen" {
		t.Fatalf("expected fused knight+queen result, got %#v", fused)
	}
	if len(result.Match.WhiteHand) != len(snapshot.Match.WhiteHand)-1 {
		t.Fatalf("expected fullfusion card to be consumed")
	}
}

func TestFullFusionBishopAndRookBecomeQueen(t *testing.T) {
	service := NewService()
	now := time.Date(2026, 5, 5, 8, 0, 0, 0, time.UTC)
	snapshot := service.CreateMatch(contracts.CreateMatchRequest{MatchID: "fullfusion_queen"}, now)
	cardID := cardIDByMechanic(t, snapshot.Match.WhiteHand, "fullfusion")

	state := service.matches["fullfusion_queen"]
	state.Board = emptyBoard()
	state.Board[0][4] = &contracts.Piece{Type: "king", Color: "white"}
	state.Board[7][7] = &contracts.Piece{Type: "king", Color: "black"}
	state.Board[3][3] = &contracts.Piece{Type: "bishop", Color: "white"}
	state.Board[3][4] = &contracts.Piece{Type: "rook", Color: "white"}

	if _, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "play_card",
		MatchID:  "fullfusion_queen",
		PlayerID: "white_player",
		CardID:   cardID,
	}, now.Add(time.Second)); err != nil {
		t.Fatalf("expected fullfusion play_card to succeed, got %v", err)
	}
	if _, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "select_target",
		MatchID:  "fullfusion_queen",
		PlayerID: "white_player",
		Target:   &contracts.Square{Row: 3, Col: 3},
	}, now.Add(2*time.Second)); err != nil {
		t.Fatalf("expected fullfusion first selection to succeed, got %v", err)
	}

	result, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "select_target",
		MatchID:  "fullfusion_queen",
		PlayerID: "white_player",
		Target:   &contracts.Square{Row: 3, Col: 4},
	}, now.Add(3*time.Second))
	if err != nil {
		t.Fatalf("expected bishop+rook fullfusion to succeed, got %v", err)
	}

	if fused := service.matches["fullfusion_queen"].Board[3][4]; fused == nil || fused.Type != "queen" || fused.FusedWith != "" {
		t.Fatalf("expected bishop+rook to become queen, got %#v", fused)
	}
	if len(result.Match.WhiteHand) != len(snapshot.Match.WhiteHand)-1 {
		t.Fatalf("expected fullfusion card to be consumed")
	}
}

func TestFogVillagePlacesClampedZone(t *testing.T) {
	service := NewService()
	now := time.Date(2026, 5, 5, 9, 0, 0, 0, time.UTC)
	snapshot := service.CreateMatch(contracts.CreateMatchRequest{MatchID: "fog_place"}, now)
	cardID := cardIDByMechanic(t, snapshot.Match.WhiteHand, "fog_village")

	if _, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "play_card",
		MatchID:  "fog_place",
		PlayerID: "white_player",
		CardID:   cardID,
	}, now.Add(time.Second)); err != nil {
		t.Fatalf("expected fog_village play_card to succeed, got %v", err)
	}

	result, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "select_target",
		MatchID:  "fog_place",
		PlayerID: "white_player",
		Target:   &contracts.Square{Row: 0, Col: 7},
	}, now.Add(2*time.Second))
	if err != nil {
		t.Fatalf("expected fog_village select_target to succeed, got %v", err)
	}

	if len(result.Match.FogZones) != 1 {
		t.Fatalf("expected one fog zone, got %#v", result.Match.FogZones)
	}
	zone := result.Match.FogZones[0]
	if zone.CenterRow != 1 || zone.CenterCol != 6 || zone.TurnsLeft != 2 || zone.OwnerColor != "white" {
		t.Fatalf("unexpected fog zone %#v", zone)
	}
}

func TestFogVillageExpiresAfterTwoFullRounds(t *testing.T) {
	service := NewService()
	now := time.Date(2026, 5, 5, 9, 30, 0, 0, time.UTC)
	snapshot := service.CreateMatch(contracts.CreateMatchRequest{MatchID: "fog_decay"}, now)
	cardID := cardIDByMechanic(t, snapshot.Match.WhiteHand, "fog_village")

	if _, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "play_card",
		MatchID:  "fog_decay",
		PlayerID: "white_player",
		CardID:   cardID,
	}, now.Add(time.Second)); err != nil {
		t.Fatalf("expected fog_village play_card to succeed, got %v", err)
	}
	if _, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "select_target",
		MatchID:  "fog_decay",
		PlayerID: "white_player",
		Target:   &contracts.Square{Row: 3, Col: 3},
	}, now.Add(2*time.Second)); err != nil {
		t.Fatalf("expected fog_village placement to succeed, got %v", err)
	}

	moveTimes := []time.Time{
		now.Add(3 * time.Second),
		now.Add(4 * time.Second),
		now.Add(5 * time.Second),
		now.Add(6 * time.Second),
	}
	moves := []contracts.PlayerIntent{
		{Type: "make_move", MatchID: "fog_decay", PlayerID: "white_player", From: &contracts.Square{Row: 1, Col: 4}, To: &contracts.Square{Row: 3, Col: 4}},
		{Type: "make_move", MatchID: "fog_decay", PlayerID: "black_player", From: &contracts.Square{Row: 6, Col: 4}, To: &contracts.Square{Row: 4, Col: 4}},
		{Type: "make_move", MatchID: "fog_decay", PlayerID: "white_player", From: &contracts.Square{Row: 1, Col: 3}, To: &contracts.Square{Row: 3, Col: 3}},
		{Type: "make_move", MatchID: "fog_decay", PlayerID: "black_player", From: &contracts.Square{Row: 6, Col: 3}, To: &contracts.Square{Row: 4, Col: 3}},
	}

	for idx, move := range moves {
		result, err := service.ApplyIntent(move, moveTimes[idx])
		if err != nil {
			t.Fatalf("expected fog decay move %d to succeed, got %v", idx, err)
		}
		if idx == 1 {
			if len(result.Match.FogZones) != 1 || result.Match.FogZones[0].TurnsLeft != 1 {
				t.Fatalf("expected fog to have 1 turn left after one full round, got %#v", result.Match.FogZones)
			}
		}
		if idx == 3 && len(result.Match.FogZones) != 0 {
			t.Fatalf("expected fog to expire after two full rounds, got %#v", result.Match.FogZones)
		}
	}
}

func TestFortressPlacesClampedZone(t *testing.T) {
	service := NewService()
	now := time.Date(2026, 5, 5, 9, 45, 0, 0, time.UTC)
	snapshot := service.CreateMatch(contracts.CreateMatchRequest{MatchID: "fortress_place"}, now)
	cardID := cardIDByMechanic(t, snapshot.Match.WhiteHand, "fortress")

	if _, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "play_card",
		MatchID:  "fortress_place",
		PlayerID: "white_player",
		CardID:   cardID,
	}, now.Add(time.Second)); err != nil {
		t.Fatalf("expected fortress play_card to succeed, got %v", err)
	}

	result, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "select_target",
		MatchID:  "fortress_place",
		PlayerID: "white_player",
		Target:   &contracts.Square{Row: 7, Col: 7},
	}, now.Add(2*time.Second))
	if err != nil {
		t.Fatalf("expected fortress placement to succeed, got %v", err)
	}

	if len(result.Match.FortressZones) != 1 {
		t.Fatalf("expected one fortress zone, got %#v", result.Match.FortressZones)
	}
	zone := result.Match.FortressZones[0]
	if zone.TopRow != 6 || zone.LeftCol != 6 || zone.TurnsLeft != 2 || zone.OwnerColor != "white" {
		t.Fatalf("unexpected fortress zone %#v", zone)
	}
}

func TestFortressBlocksEnemyEntry(t *testing.T) {
	service := NewService()
	now := time.Date(2026, 5, 5, 9, 50, 0, 0, time.UTC)
	snapshot := service.CreateMatch(contracts.CreateMatchRequest{MatchID: "fortress_block"}, now)
	cardID := cardIDByMechanic(t, snapshot.Match.WhiteHand, "fortress")

	if _, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "play_card",
		MatchID:  "fortress_block",
		PlayerID: "white_player",
		CardID:   cardID,
	}, now.Add(time.Second)); err != nil {
		t.Fatalf("expected fortress play_card to succeed, got %v", err)
	}
	if _, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "select_target",
		MatchID:  "fortress_block",
		PlayerID: "white_player",
		Target:   &contracts.Square{Row: 4, Col: 3},
	}, now.Add(2*time.Second)); err != nil {
		t.Fatalf("expected fortress placement to succeed, got %v", err)
	}
	if _, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "make_move",
		MatchID:  "fortress_block",
		PlayerID: "white_player",
		From:     &contracts.Square{Row: 1, Col: 4},
		To:       &contracts.Square{Row: 3, Col: 4},
	}, now.Add(3*time.Second)); err != nil {
		t.Fatalf("expected white move before fortress test to succeed, got %v", err)
	}

	_, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "make_move",
		MatchID:  "fortress_block",
		PlayerID: "black_player",
		From:     &contracts.Square{Row: 6, Col: 3},
		To:       &contracts.Square{Row: 4, Col: 3},
	}, now.Add(4*time.Second))
	if err == nil || !strings.Contains(err.Error(), "fortress") {
		t.Fatalf("expected black entry into fortress to be blocked, got %v", err)
	}
}

func TestFortressExpiresAfterTwoFullRounds(t *testing.T) {
	service := NewService()
	now := time.Date(2026, 5, 5, 9, 55, 0, 0, time.UTC)
	snapshot := service.CreateMatch(contracts.CreateMatchRequest{MatchID: "fortress_decay"}, now)
	cardID := cardIDByMechanic(t, snapshot.Match.WhiteHand, "fortress")

	if _, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "play_card",
		MatchID:  "fortress_decay",
		PlayerID: "white_player",
		CardID:   cardID,
	}, now.Add(time.Second)); err != nil {
		t.Fatalf("expected fortress play_card to succeed, got %v", err)
	}
	if _, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "select_target",
		MatchID:  "fortress_decay",
		PlayerID: "white_player",
		Target:   &contracts.Square{Row: 4, Col: 3},
	}, now.Add(2*time.Second)); err != nil {
		t.Fatalf("expected fortress placement to succeed, got %v", err)
	}

	moveTimes := []time.Time{
		now.Add(3 * time.Second),
		now.Add(4 * time.Second),
		now.Add(5 * time.Second),
		now.Add(6 * time.Second),
	}
	moves := []contracts.PlayerIntent{
		{Type: "make_move", MatchID: "fortress_decay", PlayerID: "white_player", From: &contracts.Square{Row: 1, Col: 4}, To: &contracts.Square{Row: 3, Col: 4}},
		{Type: "make_move", MatchID: "fortress_decay", PlayerID: "black_player", From: &contracts.Square{Row: 6, Col: 0}, To: &contracts.Square{Row: 5, Col: 0}},
		{Type: "make_move", MatchID: "fortress_decay", PlayerID: "white_player", From: &contracts.Square{Row: 1, Col: 3}, To: &contracts.Square{Row: 3, Col: 3}},
		{Type: "make_move", MatchID: "fortress_decay", PlayerID: "black_player", From: &contracts.Square{Row: 6, Col: 2}, To: &contracts.Square{Row: 4, Col: 2}},
	}

	for idx, move := range moves {
		result, err := service.ApplyIntent(move, moveTimes[idx])
		if err != nil {
			t.Fatalf("expected fortress decay move %d to succeed, got %v", idx, err)
		}
		if idx == 1 {
			if len(result.Match.FortressZones) != 1 || result.Match.FortressZones[0].TurnsLeft != 1 {
				t.Fatalf("expected fortress to have 1 turn left after one full round, got %#v", result.Match.FortressZones)
			}
		}
		if idx == 3 && len(result.Match.FortressZones) != 0 {
			t.Fatalf("expected fortress to expire after two full rounds, got %#v", result.Match.FortressZones)
		}
	}
}

func TestDoubleMoveTwinKeepsTurnForSecondMove(t *testing.T) {
	service := NewService()
	now := time.Date(2026, 5, 5, 10, 0, 0, 0, time.UTC)
	snapshot := service.CreateMatch(contracts.CreateMatchRequest{MatchID: "doublemove_twin"}, now)
	cardID := cardIDByMechanic(t, snapshot.Match.WhiteHand, "doublemove_diff")

	if _, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "play_card",
		MatchID:  "doublemove_twin",
		PlayerID: "white_player",
		CardID:   cardID,
	}, now.Add(time.Second)); err != nil {
		t.Fatalf("expected doublemove_diff play_card to succeed, got %v", err)
	}

	first, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "make_move",
		MatchID:  "doublemove_twin",
		PlayerID: "white_player",
		From:     &contracts.Square{Row: 1, Col: 4},
		To:       &contracts.Square{Row: 3, Col: 4},
	}, now.Add(2*time.Second))
	if err != nil {
		t.Fatalf("expected first double move to succeed, got %v", err)
	}
	if first.Match.Turn != "white" {
		t.Fatalf("expected turn to stay white after first double move, got %q", first.Match.Turn)
	}
	if first.Match.DoubleMove == nil || first.Match.DoubleMove.Type != "diff" || first.Match.DoubleMove.MovesLeft != 1 {
		t.Fatalf("expected active twin double move after first move, got %#v", first.Match.DoubleMove)
	}
	if len(first.Match.MoveHistory) != 0 {
		t.Fatalf("expected combined notation to wait for second move, got %#v", first.Match.MoveHistory)
	}

	second, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "make_move",
		MatchID:  "doublemove_twin",
		PlayerID: "white_player",
		From:     &contracts.Square{Row: 1, Col: 3},
		To:       &contracts.Square{Row: 3, Col: 3},
	}, now.Add(3*time.Second))
	if err != nil {
		t.Fatalf("expected second double move to succeed, got %v", err)
	}
	if second.Match.Turn != "black" {
		t.Fatalf("expected turn to pass after second double move, got %q", second.Match.Turn)
	}
	if second.Match.DoubleMove != nil {
		t.Fatalf("expected double move state to clear after second move, got %#v", second.Match.DoubleMove)
	}
	if len(second.Match.MoveHistory) != 1 || second.Match.MoveHistory[0] != "e4+d4" {
		t.Fatalf("expected combined move history after double move, got %#v", second.Match.MoveHistory)
	}
}

func TestDoubleMoveSoloRequiresSamePiece(t *testing.T) {
	service := NewService()
	now := time.Date(2026, 5, 5, 10, 30, 0, 0, time.UTC)
	snapshot := service.CreateMatch(contracts.CreateMatchRequest{MatchID: "doublemove_solo"}, now)
	cardID := cardIDByMechanic(t, snapshot.Match.WhiteHand, "doublemove_same")

	if _, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "play_card",
		MatchID:  "doublemove_solo",
		PlayerID: "white_player",
		CardID:   cardID,
	}, now.Add(time.Second)); err != nil {
		t.Fatalf("expected doublemove_same play_card to succeed, got %v", err)
	}

	if _, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "make_move",
		MatchID:  "doublemove_solo",
		PlayerID: "white_player",
		From:     &contracts.Square{Row: 1, Col: 4},
		To:       &contracts.Square{Row: 3, Col: 4},
	}, now.Add(2*time.Second)); err != nil {
		t.Fatalf("expected first solo double move to succeed, got %v", err)
	}

	if _, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "make_move",
		MatchID:  "doublemove_solo",
		PlayerID: "white_player",
		From:     &contracts.Square{Row: 1, Col: 3},
		To:       &contracts.Square{Row: 3, Col: 3},
	}, now.Add(3*time.Second)); err == nil {
		t.Fatalf("expected moving a different piece during solo double move to fail")
	}
}

func TestReverseRestoresPreviousCompletedMoveState(t *testing.T) {
	service := NewService()
	now := time.Date(2026, 5, 5, 11, 0, 0, 0, time.UTC)
	snapshot := service.CreateMatch(contracts.CreateMatchRequest{MatchID: "reverse_flow"}, now)
	cardID := cardIDByMechanic(t, snapshot.Match.WhiteHand, "reverse")

	if _, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "make_move",
		MatchID:  "reverse_flow",
		PlayerID: "white_player",
		From:     &contracts.Square{Row: 1, Col: 4},
		To:       &contracts.Square{Row: 3, Col: 4},
	}, now.Add(time.Second)); err != nil {
		t.Fatalf("expected white move to succeed, got %v", err)
	}
	if _, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "make_move",
		MatchID:  "reverse_flow",
		PlayerID: "black_player",
		From:     &contracts.Square{Row: 6, Col: 4},
		To:       &contracts.Square{Row: 4, Col: 4},
	}, now.Add(2*time.Second)); err != nil {
		t.Fatalf("expected black move to succeed, got %v", err)
	}

	result, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "play_card",
		MatchID:  "reverse_flow",
		PlayerID: "white_player",
		CardID:   cardID,
	}, now.Add(3*time.Second))
	if err != nil {
		t.Fatalf("expected reverse play_card to succeed, got %v", err)
	}

	if piece := service.matches["reverse_flow"].Board[4][4]; piece != nil {
		t.Fatalf("expected reversed black pawn to disappear from e5, got %#v", piece)
	}
	if piece := service.matches["reverse_flow"].Board[6][4]; piece == nil || piece.Type != "pawn" || piece.Color != "black" {
		t.Fatalf("expected black pawn restored to e7, got %#v", piece)
	}
	if result.Match.Turn != "white" {
		t.Fatalf("expected reverse to keep turn on white, got %q", result.Match.Turn)
	}
	if len(result.Match.MoveHistory) != 1 || result.Match.MoveHistory[0] != "e4" {
		t.Fatalf("expected only white move to remain in history, got %#v", result.Match.MoveHistory)
	}
	if len(result.Match.WhiteHand) != len(snapshot.Match.WhiteHand)-1 {
		t.Fatalf("expected reverse card to be consumed")
	}
}

func TestUndoNullifiesOpponentsNextCardPlay(t *testing.T) {
	service := NewService()
	now := time.Date(2026, 5, 5, 11, 30, 0, 0, time.UTC)
	snapshot := service.CreateMatch(contracts.CreateMatchRequest{MatchID: "undo_flow"}, now)
	undoCardID := cardIDByMechanic(t, snapshot.Match.WhiteHand, "undo")
	freezeCardID := cardIDByMechanic(t, snapshot.Match.BlackHand, "freeze")

	armed, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "play_card",
		MatchID:  "undo_flow",
		PlayerID: "white_player",
		CardID:   undoCardID,
	}, now.Add(time.Second))
	if err != nil {
		t.Fatalf("expected undo play_card to succeed, got %v", err)
	}
	if armed.Match.UndoAgainst != "black" {
		t.Fatalf("expected undo to arm against black, got %q", armed.Match.UndoAgainst)
	}
	if _, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "make_move",
		MatchID:  "undo_flow",
		PlayerID: "white_player",
		From:     &contracts.Square{Row: 1, Col: 4},
		To:       &contracts.Square{Row: 3, Col: 4},
	}, now.Add(1500*time.Millisecond)); err != nil {
		t.Fatalf("expected white move after undo to succeed, got %v", err)
	}

	nullified, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "play_card",
		MatchID:  "undo_flow",
		PlayerID: "black_player",
		CardID:   freezeCardID,
	}, now.Add(2*time.Second))
	if err != nil {
		t.Fatalf("expected black card play to be nullified cleanly, got %v", err)
	}
	if nullified.Match.PendingCard != nil {
		t.Fatalf("expected nullified card not to create pending state, got %#v", nullified.Match.PendingCard)
	}
	if nullified.Match.UndoAgainst != "" {
		t.Fatalf("expected undo trap to clear after nullifying a card, got %q", nullified.Match.UndoAgainst)
	}
	if len(nullified.Match.BlackHand) != len(snapshot.Match.BlackHand)-1 {
		t.Fatalf("expected black nullified card to still be consumed")
	}
}

func TestMirrorMovesFirstMatchingPieceAndRefreshesCurrentHistoryState(t *testing.T) {
	service := NewService()
	now := time.Date(2026, 5, 5, 11, 45, 0, 0, time.UTC)
	snapshot := service.CreateMatch(contracts.CreateMatchRequest{MatchID: "mirror_flow"}, now)
	cardID := cardIDByMechanic(t, snapshot.Match.WhiteHand, "mirror")

	state := service.matches["mirror_flow"]
	state.Board = emptyBoard()
	state.Board[0][4] = &contracts.Piece{Type: "king", Color: "white"}
	state.Board[7][4] = &contracts.Piece{Type: "king", Color: "black"}
	state.Board[3][1] = &contracts.Piece{Type: "knight", Color: "white"}
	state.Board[5][2] = &contracts.Piece{Type: "knight", Color: "black"}
	state.LastMove = &contracts.LastMove{
		From: contracts.Square{Row: 7, Col: 1},
		To:   contracts.Square{Row: 5, Col: 2},
	}
	state.History = []contracts.PositionState{capturePositionState(state)}

	result, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "play_card",
		MatchID:  "mirror_flow",
		PlayerID: "white_player",
		CardID:   cardID,
	}, now.Add(time.Second))
	if err != nil {
		t.Fatalf("expected mirror play_card to succeed, got %v", err)
	}

	if state.Board[3][1] != nil {
		t.Fatalf("expected mirrored knight to leave b4, got %#v", state.Board[3][1])
	}
	if mirrored := state.Board[1][2]; mirrored == nil || mirrored.Type != "knight" || mirrored.Color != "white" {
		t.Fatalf("expected white knight to land on c2 after mirror, got %#v", mirrored)
	}
	if len(result.Match.WhiteHand) != len(snapshot.Match.WhiteHand)-1 {
		t.Fatalf("expected mirror card to be consumed")
	}
	if len(state.History) != 1 || state.History[0].Board[1][2] == nil || state.History[0].Board[1][2].Color != "white" {
		t.Fatalf("expected current history snapshot to refresh with mirrored board, got %#v", state.History)
	}
}

func TestFakePiecePlacesAuthoritativePawnOnEmptySquare(t *testing.T) {
	service := NewService()
	now := time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC)
	snapshot := service.CreateMatch(contracts.CreateMatchRequest{MatchID: "fakepiece_flow"}, now)
	cardID := cardIDByMechanic(t, snapshot.Match.WhiteHand, "fakepiece")

	state := service.matches["fakepiece_flow"]
	state.Board = emptyBoard()
	state.Board[0][4] = &contracts.Piece{Type: "king", Color: "white"}
	state.Board[7][4] = &contracts.Piece{Type: "king", Color: "black"}
	state.History = []contracts.PositionState{capturePositionState(state)}

	if _, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "play_card",
		MatchID:  "fakepiece_flow",
		PlayerID: "white_player",
		CardID:   cardID,
	}, now.Add(time.Second)); err != nil {
		t.Fatalf("expected fakepiece play_card to succeed, got %v", err)
	}

	result, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "select_target",
		MatchID:  "fakepiece_flow",
		PlayerID: "white_player",
		Target:   &contracts.Square{Row: 3, Col: 3},
	}, now.Add(2*time.Second))
	if err != nil {
		t.Fatalf("expected fakepiece placement to succeed, got %v", err)
	}

	if fake := state.Board[3][3]; fake == nil || fake.Type != "pawn" || fake.Color != "white" {
		t.Fatalf("expected fakepiece to place a white pawn on d4, got %#v", fake)
	}
	if len(result.Match.WhiteHand) != len(snapshot.Match.WhiteHand)-1 {
		t.Fatalf("expected fakepiece card to be consumed")
	}
	if len(state.History) != 1 || state.History[0].Board[3][3] == nil || state.History[0].Board[3][3].Type != "pawn" {
		t.Fatalf("expected current history snapshot to include fakepiece placement, got %#v", state.History)
	}
}

func TestBlackHoleArmsAndExplodesAfterCountdown(t *testing.T) {
	service := NewService()
	now := time.Date(2026, 5, 5, 12, 15, 0, 0, time.UTC)
	snapshot := service.CreateMatch(contracts.CreateMatchRequest{MatchID: "blackhole_flow"}, now)
	cardID := cardIDByMechanic(t, snapshot.Match.WhiteHand, "blackhole")

	state := service.matches["blackhole_flow"]
	state.Board = emptyBoard()
	state.Board[0][4] = &contracts.Piece{Type: "king", Color: "white"}
	state.Board[7][4] = &contracts.Piece{Type: "king", Color: "black"}
	state.Board[4][4] = &contracts.Piece{Type: "rook", Color: "white"}
	state.Board[6][3] = &contracts.Piece{Type: "pawn", Color: "black"}
	state.Board[2][2] = &contracts.Piece{Type: "bishop", Color: "white"}
	state.History = []contracts.PositionState{capturePositionState(state)}

	if _, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "play_card",
		MatchID:  "blackhole_flow",
		PlayerID: "white_player",
		CardID:   cardID,
	}, now.Add(time.Second)); err != nil {
		t.Fatalf("expected blackhole play_card to succeed, got %v", err)
	}

	step1, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "select_target",
		MatchID:  "blackhole_flow",
		PlayerID: "white_player",
		Target:   &contracts.Square{Row: 3, Col: 3},
	}, now.Add(2*time.Second))
	if err != nil {
		t.Fatalf("expected first blackhole target to succeed, got %v", err)
	}
	if step1.Match.PendingCard == nil || step1.Match.PendingCard.Target == nil {
		t.Fatalf("expected pending blackhole first target, got %#v", step1.Match.PendingCard)
	}

	armed, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "select_target",
		MatchID:  "blackhole_flow",
		PlayerID: "white_player",
		Target:   &contracts.Square{Row: 5, Col: 3},
	}, now.Add(3*time.Second))
	if err != nil {
		t.Fatalf("expected second blackhole target to succeed, got %v", err)
	}
	if len(armed.Match.BlackHoles) != 1 || armed.Match.BlackHoles[0].TurnsLeft != 2 {
		t.Fatalf("expected armed black hole with 2 turns left, got %#v", armed.Match.BlackHoles)
	}

	if _, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "make_move",
		MatchID:  "blackhole_flow",
		PlayerID: "white_player",
		From:     &contracts.Square{Row: 4, Col: 4},
		To:       &contracts.Square{Row: 4, Col: 5},
	}, now.Add(4*time.Second)); err != nil {
		t.Fatalf("expected first post-blackhole move to succeed, got %v", err)
	}
	if len(state.BlackHoles) != 1 || state.BlackHoles[0].TurnsLeft != 2 {
		t.Fatalf("expected black hole to remain at 2 until black completes a turn, got %#v", state.BlackHoles)
	}

	if _, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "make_move",
		MatchID:  "blackhole_flow",
		PlayerID: "black_player",
		From:     &contracts.Square{Row: 6, Col: 3},
		To:       &contracts.Square{Row: 5, Col: 3},
	}, now.Add(5*time.Second)); err != nil {
		t.Fatalf("expected black move to succeed, got %v", err)
	}
	if len(state.BlackHoles) != 1 || state.BlackHoles[0].TurnsLeft != 1 {
		t.Fatalf("expected black hole to tick down to 1 after black move, got %#v", state.BlackHoles)
	}

	if _, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "make_move",
		MatchID:  "blackhole_flow",
		PlayerID: "white_player",
		From:     &contracts.Square{Row: 2, Col: 2},
		To:       &contracts.Square{Row: 1, Col: 3},
	}, now.Add(6*time.Second)); err != nil {
		t.Fatalf("expected white follow-up move to succeed, got %v", err)
	}

	result, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "make_move",
		MatchID:  "blackhole_flow",
		PlayerID: "black_player",
		From:     &contracts.Square{Row: 7, Col: 4},
		To:       &contracts.Square{Row: 6, Col: 4},
	}, now.Add(7*time.Second))
	if err != nil {
		t.Fatalf("expected detonation move to succeed, got %v", err)
	}

	if len(state.BlackHoles) != 0 {
		t.Fatalf("expected black hole to clear after detonation, got %#v", state.BlackHoles)
	}
	if state.Board[5][3] != nil {
		t.Fatalf("expected black pawn near second black hole square to be destroyed, got %#v", state.Board[5][3])
	}
	foundExplosion := false
	for _, event := range result.Events {
		if event.Type == "move_applied" {
			if squares, ok := event.Payload["blackHoleExplodedSquares"].([]contracts.Square); ok && len(squares) > 0 {
				foundExplosion = true
			}
		}
	}
	if !foundExplosion {
		t.Fatalf("expected move_applied payload to include black hole explosion squares, got %#v", result.Events)
	}
}

func TestSmallSacrificeRemovesPiecesAndDrawsRewardCards(t *testing.T) {
	service := NewService()
	now := time.Date(2026, 5, 5, 12, 30, 0, 0, time.UTC)
	snapshot := service.CreateMatch(contracts.CreateMatchRequest{MatchID: "smallsacrifice_flow"}, now)
	cardID := cardIDByMechanic(t, snapshot.Match.WhiteHand, "smallsacrifice")

	state := service.matches["smallsacrifice_flow"]
	state.Board = emptyBoard()
	state.Board[0][4] = &contracts.Piece{Type: "king", Color: "white"}
	state.Board[7][4] = &contracts.Piece{Type: "king", Color: "black"}
	state.Board[3][3] = &contracts.Piece{Type: "rook", Color: "white"}
	state.Board[3][4] = &contracts.Piece{Type: "bishop", Color: "white"}
	state.History = []contracts.PositionState{capturePositionState(state)}

	if _, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "play_card",
		MatchID:  "smallsacrifice_flow",
		PlayerID: "white_player",
		CardID:   cardID,
	}, now.Add(time.Second)); err != nil {
		t.Fatalf("expected smallsacrifice play_card to succeed, got %v", err)
	}

	if _, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "select_target",
		MatchID:  "smallsacrifice_flow",
		PlayerID: "white_player",
		Target:   &contracts.Square{Row: 3, Col: 3},
	}, now.Add(2*time.Second)); err != nil {
		t.Fatalf("expected first sacrifice selection to succeed, got %v", err)
	}

	step2, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "select_target",
		MatchID:  "smallsacrifice_flow",
		PlayerID: "white_player",
		Target:   &contracts.Square{Row: 3, Col: 4},
	}, now.Add(3*time.Second))
	if err != nil {
		t.Fatalf("expected second sacrifice selection to succeed, got %v", err)
	}
	if step2.Match.PendingCard == nil || len(step2.Match.PendingCard.Options) != 2 {
		t.Fatalf("expected pending sacrifice selections to be tracked, got %#v", step2.Match.PendingCard)
	}

	result, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "select_target",
		MatchID:  "smallsacrifice_flow",
		PlayerID: "white_player",
		Target:   &contracts.Square{Row: 4, Col: 4},
	}, now.Add(4*time.Second))
	if err != nil {
		t.Fatalf("expected sacrifice confirmation to succeed, got %v", err)
	}

	if state.Board[3][3] != nil || state.Board[3][4] != nil {
		t.Fatalf("expected sacrificed pieces to be removed, got %#v %#v", state.Board[3][3], state.Board[3][4])
	}
	if len(result.Match.WhiteHand) != len(snapshot.Match.WhiteHand)+1 {
		t.Fatalf("expected sacrifice to consume one card and draw two, got hand len %d", len(result.Match.WhiteHand))
	}
	if result.Match.PendingCard != nil {
		t.Fatalf("expected pending sacrifice state to clear, got %#v", result.Match.PendingCard)
	}
}

func TestGamblerResolvesCardTransferOnBackend(t *testing.T) {
	service := NewService()
	now := time.Date(2026, 5, 5, 12, 45, 0, 0, time.UTC)
	snapshot := service.CreateMatch(contracts.CreateMatchRequest{MatchID: "gambler_flow", Seed: 2}, now)
	cardID := cardIDByMechanic(t, snapshot.Match.WhiteHand, "gambler")

	result, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "play_card",
		MatchID:  "gambler_flow",
		PlayerID: "white_player",
		CardID:   cardID,
	}, now.Add(time.Second))
	if err != nil {
		t.Fatalf("expected gambler play_card to succeed, got %v", err)
	}

	if len(result.Events) == 0 || result.Events[0].Type != "card_played" {
		t.Fatalf("expected gambler to emit a card_played event, got %#v", result.Events)
	}
	outcome, _ := result.Events[0].Payload["outcome"].(string)
	if outcome != "win" && outcome != "lose" && outcome != "none" {
		t.Fatalf("expected gambler outcome payload, got %#v", result.Events[0].Payload)
	}
	if outcome == "win" {
		if len(result.Match.WhiteHand) != len(snapshot.Match.WhiteHand) {
			t.Fatalf("expected win branch to consume gambler and gain one opponent card, got %d", len(result.Match.WhiteHand))
		}
	} else if outcome == "lose" {
		if len(result.Match.WhiteHand) != len(snapshot.Match.WhiteHand)-2 {
			t.Fatalf("expected lose branch to consume gambler and give away one more card, got %d", len(result.Match.WhiteHand))
		}
	}
}

func TestRadarRevealsEnemyHandUntilTurnPasses(t *testing.T) {
	service := NewService()
	now := time.Date(2026, 5, 5, 13, 0, 0, 0, time.UTC)
	snapshot := service.CreateMatch(contracts.CreateMatchRequest{MatchID: "radar_flow"}, now)
	cardID := cardIDByMechanic(t, snapshot.Match.WhiteHand, "radar")

	armed, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "play_card",
		MatchID:  "radar_flow",
		PlayerID: "white_player",
		CardID:   cardID,
	}, now.Add(time.Second))
	if err != nil {
		t.Fatalf("expected radar play_card to succeed, got %v", err)
	}
	if armed.Match.RadarRevealFor != "white" {
		t.Fatalf("expected radar reveal to arm for white, got %q", armed.Match.RadarRevealFor)
	}

	moved, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "make_move",
		MatchID:  "radar_flow",
		PlayerID: "white_player",
		From:     &contracts.Square{Row: 1, Col: 4},
		To:       &contracts.Square{Row: 3, Col: 4},
	}, now.Add(2*time.Second))
	if err != nil {
		t.Fatalf("expected white move after radar to succeed, got %v", err)
	}
	if moved.Match.RadarRevealFor != "" {
		t.Fatalf("expected radar reveal to clear after turn passes, got %q", moved.Match.RadarRevealFor)
	}
}

func TestCheaterCountsDownAfterOwnersTurns(t *testing.T) {
	service := NewService()
	now := time.Date(2026, 5, 5, 13, 15, 0, 0, time.UTC)
	snapshot := service.CreateMatch(contracts.CreateMatchRequest{MatchID: "cheater_flow"}, now)
	cardID := cardIDByMechanic(t, snapshot.Match.WhiteHand, "cheater")

	armed, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "play_card",
		MatchID:  "cheater_flow",
		PlayerID: "white_player",
		CardID:   cardID,
	}, now.Add(time.Second))
	if err != nil {
		t.Fatalf("expected cheater play_card to succeed, got %v", err)
	}
	if armed.Match.CheaterState == nil || armed.Match.CheaterState.OwnerColor != "white" || armed.Match.CheaterState.TurnsLeft != 3 {
		t.Fatalf("expected cheater state to start at 3 turns for white, got %#v", armed.Match.CheaterState)
	}

	white1, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "make_move",
		MatchID:  "cheater_flow",
		PlayerID: "white_player",
		From:     &contracts.Square{Row: 1, Col: 4},
		To:       &contracts.Square{Row: 3, Col: 4},
	}, now.Add(2*time.Second))
	if err != nil {
		t.Fatalf("expected first white move to succeed, got %v", err)
	}
	if white1.Match.CheaterState == nil || white1.Match.CheaterState.TurnsLeft != 2 {
		t.Fatalf("expected cheater to drop to 2 after white turn ends, got %#v", white1.Match.CheaterState)
	}

	black1, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "make_move",
		MatchID:  "cheater_flow",
		PlayerID: "black_player",
		From:     &contracts.Square{Row: 6, Col: 4},
		To:       &contracts.Square{Row: 4, Col: 4},
	}, now.Add(3*time.Second))
	if err != nil {
		t.Fatalf("expected black reply to succeed, got %v", err)
	}
	if black1.Match.CheaterState == nil || black1.Match.CheaterState.TurnsLeft != 2 {
		t.Fatalf("expected cheater to stay at 2 during black turn, got %#v", black1.Match.CheaterState)
	}

	white2, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "make_move",
		MatchID:  "cheater_flow",
		PlayerID: "white_player",
		From:     &contracts.Square{Row: 1, Col: 3},
		To:       &contracts.Square{Row: 3, Col: 3},
	}, now.Add(4*time.Second))
	if err != nil {
		t.Fatalf("expected second white move to succeed, got %v", err)
	}
	if white2.Match.CheaterState == nil || white2.Match.CheaterState.TurnsLeft != 1 {
		t.Fatalf("expected cheater to drop to 1 after second white turn, got %#v", white2.Match.CheaterState)
	}
}

func TestJokerTransformsIntoBackendOwnedCard(t *testing.T) {
	service := NewService()
	now := time.Date(2026, 5, 5, 13, 30, 0, 0, time.UTC)
	snapshot := service.CreateMatch(contracts.CreateMatchRequest{MatchID: "joker_flow"}, now)
	cardID := cardIDByMechanic(t, snapshot.Match.WhiteHand, "joker")

	armed, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:     "play_card",
		MatchID:  "joker_flow",
		PlayerID: "white_player",
		CardID:   cardID,
	}, now.Add(time.Second))
	if err != nil {
		t.Fatalf("expected joker play_card to succeed, got %v", err)
	}
	if armed.Match.PendingCard == nil || armed.Match.PendingCard.Mechanic != "joker" {
		t.Fatalf("expected joker to enter pending transform state, got %#v", armed.Match.PendingCard)
	}

	result, err := service.ApplyIntent(contracts.PlayerIntent{
		Type:        "select_target",
		MatchID:     "joker_flow",
		PlayerID:    "white_player",
		SelectionID: "freeze",
	}, now.Add(2*time.Second))
	if err != nil {
		t.Fatalf("expected joker transform selection to succeed, got %v", err)
	}
	if result.Match.PendingCard != nil {
		t.Fatalf("expected joker pending state to clear, got %#v", result.Match.PendingCard)
	}
	if len(result.Match.WhiteHand) != len(snapshot.Match.WhiteHand) {
		t.Fatalf("expected joker transform to keep hand size stable, got %d", len(result.Match.WhiteHand))
	}
	foundTransformed := false
	for _, card := range result.Match.WhiteHand {
		if card.ID == cardID {
			t.Fatalf("expected original joker card to be removed from the hand")
		}
		if strings.HasPrefix(card.ID, "joker_freeze_white_") && card.Mechanic == "freeze" {
			foundTransformed = true
		}
	}
	if !foundTransformed {
		t.Fatalf("expected transformed backend freeze card in hand, got %#v", result.Match.WhiteHand)
	}
}

func cardIDByMechanic(t *testing.T, hand []contracts.GameCard, mechanic string) string {
	t.Helper()
	for _, card := range hand {
		if card.Mechanic == mechanic {
			return card.ID
		}
	}
	t.Fatalf("card with mechanic %q not found in hand %#v", mechanic, hand)
	return ""
}

func emptyBoard() [][]*contracts.Piece {
	board := make([][]*contracts.Piece, 8)
	for r := range board {
		board[r] = make([]*contracts.Piece, 8)
	}
	return board
}
