package match

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/chess404/realtime/internal/contracts"
	"github.com/chess404/realtime/internal/logging"
)

const (
	rulesVersion                 = "v1-alpha-foundation"
	defaultClock                 = int64(10 * 60 * 1000)
	maxHandSize                  = 10
	drawFromRound                = 8
	drawEveryRounds              = 3
	presenceHeartbeatTimeout     = 25 * time.Second
	disconnectGracePeriod        = 45 * time.Second
	disconnectGraceBothPeriod    = 2 * time.Minute
	disconnectGraceBoth          = "both"
	maxIntentsPerSecondPerPlayer = 10
)

var (
	ErrMatchNotFound     = errors.New("match not found")
	ErrMatchSeatFull     = errors.New("match has no open seats")
	ErrMatchJoinFinished = errors.New("match is finished")
)

type Service struct {
	mu          sync.RWMutex
	matches     map[string]*contracts.MatchState
	events      map[string][]contracts.ResolvedEvent
	subs        map[string]map[chan contracts.MatchSnapshotResponse]string
	presence    map[string]*matchPresenceState
	matchMu     map[string]*sync.Mutex
	archive     MatchArchiver
	store       MatchStore
	broadcaster Broadcaster
	stopCh      chan struct{}
	matchSeqNum map[string]int64
	authTokens  map[string]authTokenEntry
	Log         *logging.Logger
	broadcastWG sync.WaitGroup
}

type authTokenEntry struct {
	PlayerID     string
	PlayerSecret string
	ExpiresAt    time.Time
}

type matchPresenceState struct {
	WhiteLastSeenAt         time.Time
	BlackLastSeenAt         time.Time
	WhiteConnected          bool
	BlackConnected          bool
	DisconnectGraceFor      string
	DisconnectGraceDeadline *time.Time
	WhiteLastIntentAt       time.Time
	BlackLastIntentAt       time.Time
}

type MatchArchiver interface {
	Upsert(snapshot contracts.MatchSnapshotResponse) error
}

type MatchArchiveLoader interface {
	MatchArchiver
	LoadMatch(matchID string) (contracts.MatchState, []contracts.ResolvedEvent, bool)
}

type MatchArchiveBootstrapper interface {
	MatchArchiveLoader
	ListUnfinishedMatchIDs(limit int) []string
}

type ServiceStats struct {
	LoadedMatches     int `json:"loadedMatches"`
	ActiveMatches     int `json:"activeMatches"`
	FinishedMatches   int `json:"finishedMatches"`
	SubscriberCount   int `json:"subscriberCount"`
	BufferedEventSets int `json:"bufferedEventSets"`
}

func NewService() *Service {
	return NewServiceWithArchive(nil)
}

func NewServiceWithArchive(archive MatchArchiver) *Service {
	return NewServiceWithStoreAndBroadcaster(archive, NewMemoryMatchStore(), NoopBroadcaster{})
}

func NewServiceWithStoreAndBroadcaster(archive MatchArchiver, store MatchStore, broadcaster Broadcaster) *Service {
	service := &Service{
		matches:     make(map[string]*contracts.MatchState),
		events:      make(map[string][]contracts.ResolvedEvent),
		subs:        make(map[string]map[chan contracts.MatchSnapshotResponse]string),
		presence:    make(map[string]*matchPresenceState),
		matchMu:     make(map[string]*sync.Mutex),
		matchSeqNum: make(map[string]int64),
		archive:     archive,
		store:       store,
		broadcaster: broadcaster,
		stopCh:      make(chan struct{}),
		authTokens:  make(map[string]authTokenEntry),
		Log:         logging.New("match-service"),
	}
	if loader, ok := archive.(MatchArchiveBootstrapper); ok {
		service.mu.Lock()
		service.restoreArchivedMatchesLocked(loader)
		service.mu.Unlock()
	}

	go service.startBroadcaster()
	go service.cleanupAuthTokensLoop()

	return service
}

var starterCards = []contracts.GameCard{
	{
		ID:       "freeze",
		Name:     "Freeze",
		Mechanic: "freeze",
		Type:     "trap",
		Rarity:   "common",
		Color:    "#1a2e1a",
		Accent:   "#4ade80",
		Icon:     "\U0001F9CA",
		Desc:     "Freeze one enemy piece for 1 turn. Not king.",
	},
	{
		ID:       "shield",
		Name:     "Shield",
		Mechanic: "shield",
		Type:     "trap",
		Rarity:   "common",
		Color:    "#1a2e1a",
		Accent:   "#4ade80",
		Icon:     "\U0001F6E1\uFE0F",
		Desc:     "Protect one of your pieces from capture for 1 turn.",
	},
	{
		ID:       "sniper",
		Name:     "Sniper",
		Mechanic: "sniper",
		Type:     "spell",
		Rarity:   "epic",
		Color:    "#2d1a4a",
		Accent:   "#c084fc",
		Icon:     "\U0001F3AF",
		Desc:     "Remove any enemy piece from the board. Not king.",
	},
	{
		ID:       "badsniper",
		Name:     "Bad Sniper",
		Mechanic: "badsniper",
		Type:     "spell",
		Rarity:   "trash",
		Color:    "#1c1c1c",
		Accent:   "#6b7280",
		Icon:     "\U0001F52B",
		Desc:     "Remove one of your own pieces from the board. Not king.",
	},
	{
		ID:       "promote",
		Name:     "Promote",
		Mechanic: "promote",
		Type:     "spell",
		Rarity:   "common",
		Color:    "#1a2e1a",
		Accent:   "#4ade80",
		Icon:     "\u2B06\uFE0F",
		Desc:     "Upgrade one of your pieces to a stronger type. Not king.",
	},
	{
		ID:       "demote",
		Name:     "Demote",
		Mechanic: "demote",
		Type:     "spell",
		Rarity:   "trash",
		Color:    "#1c1c1c",
		Accent:   "#6b7280",
		Icon:     "\u2B07\uFE0F",
		Desc:     "Lower one of your own pieces to a weaker type. Not king.",
	},
	{
		ID:       "promotehim",
		Name:     "Promote Him",
		Mechanic: "promotehim",
		Type:     "spell",
		Rarity:   "trash",
		Color:    "#1c1c1c",
		Accent:   "#6b7280",
		Icon:     "\U0001F4C8",
		Desc:     "Promote an enemy piece to a stronger type. Not king.",
	},
	{
		ID:       "demotehim",
		Name:     "Demote Him",
		Mechanic: "demotehim",
		Type:     "spell",
		Rarity:   "rare",
		Color:    "#1a2a4a",
		Accent:   "#60a5fa",
		Icon:     "\U0001F4C9",
		Desc:     "Lower any piece to a weaker type. Not king.",
	},
	{
		ID:       "teleport",
		Name:     "Teleport",
		Mechanic: "teleport",
		Type:     "spell",
		Rarity:   "rare",
		Color:    "#1a2a4a",
		Accent:   "#60a5fa",
		Icon:     "\U0001F300",
		Desc:     "Move one of your pieces to any empty square. Not king.",
	},
	{
		ID:       "jump",
		Name:     "Jump",
		Mechanic: "jump",
		Type:     "spell",
		Rarity:   "common",
		Color:    "#1a2e1a",
		Accent:   "#4ade80",
		Icon:     "\U0001F998",
		Desc:     "Jump over exactly one piece using your piece's movement pattern. Not king or knight.",
	},
	{
		ID:       "doublemove_diff",
		Name:     "Double Move (Twin)",
		Mechanic: "doublemove_diff",
		Type:     "spell",
		Rarity:   "rare",
		Color:    "#1a2a4a",
		Accent:   "#60a5fa",
		Icon:     "\U0001F465",
		Desc:     "Move two different pieces this turn.",
	},
	{
		ID:       "doublemove_same",
		Name:     "Double Move (Solo)",
		Mechanic: "doublemove_same",
		Type:     "spell",
		Rarity:   "rare",
		Color:    "#1a2a4a",
		Accent:   "#60a5fa",
		Icon:     "\U0001F3C3",
		Desc:     "Move the same piece twice this turn.",
	},
	{
		ID:       "swapme",
		Name:     "Swap Me",
		Mechanic: "swapme",
		Type:     "spell",
		Rarity:   "common",
		Color:    "#1a2e1a",
		Accent:   "#4ade80",
		Icon:     "\U0001F504",
		Desc:     "Exchange positions of two of your pieces. No check. No king.",
	},
	{
		ID:       "swapus",
		Name:     "Swap Us",
		Mechanic: "swapus",
		Type:     "spell",
		Rarity:   "rare",
		Color:    "#1a2a4a",
		Accent:   "#60a5fa",
		Icon:     "\u2194\uFE0F",
		Desc:     "Swap one of your pieces with one enemy piece. No kings.",
	},
	{
		ID:       "swaphim",
		Name:     "Swap Him",
		Mechanic: "swaphim",
		Type:     "spell",
		Rarity:   "rare",
		Color:    "#1a2a4a",
		Accent:   "#60a5fa",
		Icon:     "\U0001F501",
		Desc:     "Swap two enemy pieces. No kings.",
	},
	{
		ID:       "borrow",
		Name:     "Borrow",
		Mechanic: "borrow",
		Type:     "spell",
		Rarity:   "epic",
		Color:    "#2d1a4a",
		Accent:   "#c084fc",
		Icon:     "\U0001F90F",
		Desc:     "Control one enemy piece for 1 turn. Not king.",
	},
	{
		ID:       "mindcontrol",
		Name:     "Mind Control",
		Mechanic: "mindcontrol",
		Type:     "spell",
		Rarity:   "legendary",
		Color:    "#4a2a00",
		Accent:   "#f59e0b",
		Icon:     "\U0001F9E0",
		Desc:     "Permanently steal one enemy piece. Not king.",
	},
	{
		ID:       "parasite",
		Name:     "Parasite",
		Mechanic: "parasite",
		Type:     "spell",
		Rarity:   "epic",
		Color:    "#2d1a4a",
		Accent:   "#c084fc",
		Icon:     "\U0001F9A0",
		Desc:     "Link your piece to enemy piece. If yours dies, theirs dies too.",
	},
	{
		ID:       "clone",
		Name:     "Clone",
		Mechanic: "clone",
		Type:     "spell",
		Rarity:   "epic",
		Color:    "#2d1a4a",
		Accent:   "#c084fc",
		Icon:     "\U0001F9EC",
		Desc:     "Copy one of your pieces onto an adjacent empty square. Not king.",
	},
	{
		ID:       "lavaground",
		Name:     "Lava Ground",
		Mechanic: "lavaground",
		Type:     "trap",
		Rarity:   "rare",
		Color:    "#1a2a4a",
		Accent:   "#60a5fa",
		Icon:     "\U0001F30B",
		Desc:     "Mark one square. Any piece there next turn is destroyed. Not king.",
	},
	{
		ID:       "fog_village",
		Name:     "Fog Village",
		Mechanic: "fog_village",
		Type:     "spell",
		Rarity:   "common",
		Color:    "#1a2e1a",
		Accent:   "#4ade80",
		Icon:     "\U0001F32B\uFE0F",
		Desc:     "Create a 3x3 fog zone that hides your pieces for 2 turns.",
	},
	{
		ID:       "invisible",
		Name:     "Invisible",
		Mechanic: "invisible",
		Type:     "trap",
		Rarity:   "rare",
		Color:    "#1a2a4a",
		Accent:   "#60a5fa",
		Icon:     "\U0001F441\uFE0F",
		Desc:     "Hide one of your pieces for 1 round. Not king.",
	},
	{
		ID:       "unabomber",
		Name:     "Unabomber",
		Mechanic: "unabomber",
		Type:     "trap",
		Rarity:   "epic",
		Color:    "#2d1a4a",
		Accent:   "#c084fc",
		Icon:     "\U0001F4A3",
		Desc:     "Attach a bomb to one of your pieces. It explodes in 2 turns.",
	},
	{
		ID:       "halffuse",
		Name:     "Half Fuse",
		Mechanic: "halffuse",
		Type:     "spell",
		Rarity:   "rare",
		Color:    "#1a2a4a",
		Accent:   "#60a5fa",
		Icon:     "\u2697\uFE0F",
		Desc:     "Fuse two adjacent own pieces if their combined value is 6 or less.",
	},
	{
		ID:       "fullfusion",
		Name:     "Full Fusion",
		Mechanic: "fullfusion",
		Type:     "spell",
		Rarity:   "legendary",
		Color:    "#4a2a00",
		Accent:   "#f59e0b",
		Icon:     "\U0001F52E",
		Desc:     "Fuse two adjacent own pieces without a value cap.",
	},
	{
		ID:       "fortress",
		Name:     "Fortress",
		Mechanic: "fortress",
		Type:     "spell",
		Rarity:   "epic",
		Color:    "#2d1a4a",
		Accent:   "#c084fc",
		Icon:     "\U0001F3F0",
		Desc:     "Create a 2x2 zone enemies cannot enter for 2 turns.",
	},
	{
		ID:       "reverse",
		Name:     "Reverse",
		Mechanic: "reverse",
		Type:     "trap",
		Rarity:   "epic",
		Color:    "#2d1a4a",
		Accent:   "#c084fc",
		Icon:     "\u23EA",
		Desc:     "Undo opponent's last move.",
	},
	{
		ID:       "undo",
		Name:     "Undo",
		Mechanic: "undo",
		Type:     "trap",
		Rarity:   "epic",
		Color:    "#2d1a4a",
		Accent:   "#c084fc",
		Icon:     "\u21A9\uFE0F",
		Desc:     "Nullify the next card your opponent plays.",
	},
	{
		ID:       "mirror",
		Name:     "Mirror",
		Mechanic: "mirror",
		Type:     "spell",
		Rarity:   "rare",
		Color:    "#142033",
		Accent:   "#93c5fd",
		Icon:     "\U0001FA9E",
		Desc:     "Repeat the last move pattern with one of your matching pieces.",
	},
	{
		ID:       "fakepiece",
		Name:     "Fake Piece",
		Mechanic: "fakepiece",
		Type:     "trap",
		Rarity:   "common",
		Color:    "#1a2e1a",
		Accent:   "#4ade80",
		Icon:     "\U0001F47B",
		Desc:     "Place a fake pawn on an empty square.",
	},
	{
		ID:       "blackhole",
		Name:     "Black Hole",
		Mechanic: "blackhole",
		Type:     "spell",
		Rarity:   "epic",
		Color:    "#2d1a4a",
		Accent:   "#c084fc",
		Icon:     "\U0001F573\uFE0F",
		Desc:     "Choose 2 squares. After 2 turns all adjacent pieces explode. Kings immune.",
	},
	{
		ID:       "smallsacrifice",
		Name:     "Small Sacrifice",
		Mechanic: "smallsacrifice",
		Type:     "spell",
		Rarity:   "rare",
		Color:    "#4a1a1a",
		Accent:   "#f87171",
		Icon:     "\U0001FAF8",
		Desc:     "Sacrifice your own pieces totaling 6+ points to draw 2 cards.",
	},
	{
		ID:       "bigsacrifice",
		Name:     "Big Sacrifice",
		Mechanic: "bigsacrifice",
		Type:     "spell",
		Rarity:   "epic",
		Color:    "#4a1a1a",
		Accent:   "#fb7185",
		Icon:     "\U0001F48E",
		Desc:     "Sacrifice your own pieces totaling 14+ points to draw 3 cards.",
	},
	{
		ID:       "gambler",
		Name:     "Gambler",
		Mechanic: "gambler",
		Type:     "spell",
		Rarity:   "trash",
		Color:    "#1c1c1c",
		Accent:   "#6b7280",
		Icon:     "\U0001F3B2",
		Desc:     "50% steal a card from opponent. 50% give one of yours away.",
	},
	{
		ID:       "radar",
		Name:     "Radar",
		Mechanic: "radar",
		Type:     "spell",
		Rarity:   "common",
		Color:    "#1a2a4a",
		Accent:   "#60a5fa",
		Icon:     "\U0001F4E1",
		Desc:     "Reveal the enemy hand for the rest of your turn.",
	},
	{
		ID:       "cheater",
		Name:     "Cheater",
		Mechanic: "cheater",
		Type:     "spell",
		Rarity:   "trash",
		Color:    "#1c1c1c",
		Accent:   "#f59e0b",
		Icon:     "\U0001F4A1",
		Desc:     "Show engine help for this turn and your next two turns.",
	},
	{
		ID:       "joker",
		Name:     "Joker",
		Mechanic: "joker",
		Type:     "spell",
		Rarity:   "rare",
		Color:    "#4a2a00",
		Accent:   "#f59e0b",
		Icon:     "\U0001F0CF",
		Desc:     "Choose any backend-supported card from the full pool instantly.",
	},
}

const starterHandModeStarterThree = "starter_three"

func cardTemplateByMechanic(mechanic string) contracts.GameCard {
	for _, card := range starterCards {
		if card.Mechanic == mechanic {
			return card
		}
	}
	return contracts.GameCard{}
}

func starterHandCardsForMode(mode string) []contracts.GameCard {
	if strings.EqualFold(strings.TrimSpace(mode), starterHandModeStarterThree) {
		return []contracts.GameCard{
			cardTemplateByMechanic("freeze"),
			cardTemplateByMechanic("shield"),
			cardTemplateByMechanic("smallsacrifice"),
		}
	}
	return starterCards
}

func (s *Service) CreateMatch(req contracts.CreateMatchRequest, now time.Time) contracts.MatchSnapshotResponse {
	s.mu.Lock()
	defer s.mu.Unlock()

	matchID := req.MatchID
	if matchID == "" {
		b := make([]byte, 4)
		rand.Read(b)
		matchID = fmt.Sprintf("match_%d_%s", now.UnixMilli(), hex.EncodeToString(b))
	}

	clockMS := defaultClock
	if req.ClockSeconds > 0 {
		clockMS = req.ClockSeconds * 1000
	}
	increment := int64(0)
	if req.ClockIncrement > 0 {
		increment = req.ClockIncrement * 1000
	}

	startedAt := now.UnixMilli()
	hasWhiteSeat := strings.TrimSpace(req.WhiteGuestID) != ""
	hasBlackSeat := strings.TrimSpace(req.BlackGuestID) != ""
	hasPartialSeats := hasWhiteSeat != hasBlackSeat
	runningFor := ""
	var startedAtPtr *int64
	status := "active"
	if hasPartialSeats {
		status = "waiting"
	} else {
		runningFor = "white"
		startedAtPtr = &startedAt
	}
	state := &contracts.MatchState{
		MatchID:           matchID,
		RulesVersion:      rulesVersion,
		RNGSeed:           chooseSeed(req.Seed, startedAt),
		Queue:             req.Queue,
		ModeID:            contracts.NormalizeMatchModeID(string(req.ModeID)),
		WhiteGuestID:      strings.TrimSpace(req.WhiteGuestID),
		BlackGuestID:      strings.TrimSpace(req.BlackGuestID),
		WhiteAccountID:    req.WhiteAccountID,
		BlackAccountID:    req.BlackAccountID,
		WhiteName:         req.WhiteName,
		BlackName:         req.BlackName,
		WhitePlayerSecret: req.WhitePlayerSecret,
		BlackPlayerSecret: req.BlackPlayerSecret,
		Board:             makeBoard(),
		Turn:              "white",
		Moved:             []string{},
		HalfMoveClock:     0,
		FullMoveNum:       1,
		WhiteHand:         cloneCardsWithOwner(starterHandCardsForMode(req.StarterHandMode), "white"),
		BlackHand:         cloneCardsWithOwner(starterHandCardsForMode(req.StarterHandMode), "black"),
		MoveHistory:       []string{},
		ChatMessages:      []contracts.ChatMessage{},
		Clock: contracts.MatchClock{
			WhiteMS:    clockMS,
			BlackMS:    clockMS,
			RunningFor: runningFor,
			StartedAt:  startedAtPtr,
			Increment:  increment,
		},
		Status:    status,
		CreatedAt: now.UTC(),
		UpdatedAt: now.UTC(),
	}
	state.History = []contracts.PositionState{capturePositionState(state)}

	startEvent := makeEvent(matchID, "match_started", now, "", map[string]any{
		"turn": "white",
	})

	s.matches[matchID] = state
	s.events[matchID] = []contracts.ResolvedEvent{startEvent}
	s.subs[matchID] = make(map[chan contracts.MatchSnapshotResponse]string)
	s.presence[matchID] = newMatchPresenceState(state, now)

	snapshot := buildSnapshotWithPresence(state, s.presence[matchID], len(s.events[matchID]), s.events[matchID], now)
	s.persistSnapshot(snapshot)
	s.saveToRedis(snapshot, s.presence[matchID])
	return snapshot
}

func (s *Service) JoinMatchSeat(matchID string, req contracts.JoinMatchSeatRequest, now time.Time) (contracts.JoinMatchSeatResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	state, ok := s.ensureMatchLoadedLocked(matchID)
	if !ok {
		return contracts.JoinMatchSeatResponse{}, ErrMatchNotFound
	}
	if state.Status == "finished" {
		return contracts.JoinMatchSeatResponse{}, ErrMatchJoinFinished
	}

	guestID := strings.TrimSpace(req.GuestID)
	if guestID == "" {
		return contracts.JoinMatchSeatResponse{}, errors.New("guestId is required")
	}

	displayName := strings.TrimSpace(req.DisplayName)
	accountID := strings.TrimSpace(req.AccountID)
	playerSecret := strings.TrimSpace(req.PlayerSecret)
	preferredSeat := strings.ToLower(strings.TrimSpace(req.PreferredSeat))
	if preferredSeat != "" && preferredSeat != "white" && preferredSeat != "black" {
		return contracts.JoinMatchSeatResponse{}, errors.New("preferredSeat must be white or black")
	}

	seatColor := ""
	joined := false
	updated := false

	switch {
	case strings.EqualFold(state.WhiteGuestID, guestID):
		seatColor = "white"
		if displayName != "" && state.WhiteName != displayName {
			state.WhiteName = displayName
			updated = true
		}
		if accountID != "" && state.WhiteAccountID != accountID {
			state.WhiteAccountID = accountID
			updated = true
		}
		if state.WhitePlayerSecret == "" && playerSecret != "" {
			state.WhitePlayerSecret = playerSecret
			updated = true
		}
	case strings.EqualFold(state.BlackGuestID, guestID):
		seatColor = "black"
		if displayName != "" && state.BlackName != displayName {
			state.BlackName = displayName
			updated = true
		}
		if accountID != "" && state.BlackAccountID != accountID {
			state.BlackAccountID = accountID
			updated = true
		}
		if state.BlackPlayerSecret == "" && playerSecret != "" {
			state.BlackPlayerSecret = playerSecret
			updated = true
		}
	default:
		seatColor = chooseOpenSeat(state, preferredSeat)
		if seatColor == "" {
			return contracts.JoinMatchSeatResponse{}, ErrMatchSeatFull
		}
		if seatColor == "white" {
			state.WhiteGuestID = guestID
			state.WhiteName = displayName
			state.WhiteAccountID = accountID
			state.WhitePlayerSecret = playerSecret
		} else {
			state.BlackGuestID = guestID
			state.BlackName = displayName
			state.BlackAccountID = accountID
			state.BlackPlayerSecret = playerSecret
		}
		joined = true
		updated = true
	}

	events := make([]contracts.ResolvedEvent, 0, 2)
	if joined {
		events = append(events, makeEvent(matchID, "seat_joined", now, guestID, map[string]any{
			"event":     "seat_joined",
			"seatColor": seatColor,
			"guestId":   guestID,
		}))
	}

	if strings.TrimSpace(state.WhiteGuestID) != "" && strings.TrimSpace(state.BlackGuestID) != "" {
		if state.Status != "active" {
			startedAt := now.UnixMilli()
			state.Status = "active"
			state.Clock.RunningFor = "white"
			state.Clock.StartedAt = &startedAt
			updated = true
			events = append(events, makeEvent(matchID, "match_started", now, guestID, map[string]any{
				"turn": "white",
			}))
		}
	} else {
		if state.Status != "waiting" {
			state.Status = "waiting"
			updated = true
		}
		state.Clock.RunningFor = ""
		state.Clock.StartedAt = nil
	}

	if updated {
		state.UpdatedAt = now.UTC()
	}

	if len(events) > 0 {
		s.events[matchID] = append(s.events[matchID], events...)
		presence := s.ensurePresenceStateLocked(matchID, state, now)
		snapshot := buildSnapshotWithPresence(state, presence, len(s.events[matchID]), events, now)
		persistSnap := buildSnapshotWithPresence(state, presence, len(s.events[matchID]), s.events[matchID], now)
		s.persistSnapshot(persistSnap)
		s.saveToRedis(persistSnap, presence)
		s.broadcastLocked(matchID, snapshot)
		return contracts.JoinMatchSeatResponse{
			Match:              snapshot,
			SeatColor:          seatColor,
			Joined:             joined,
			WaitingForOpponent: state.Status == "waiting",
		}, nil
	}

	fullSnapshot := buildSnapshotWithPresence(state, s.ensurePresenceStateLocked(matchID, state, now), len(s.events[matchID]), nil, now)
	if updated {
		s.persistSnapshot(fullSnapshot)
		s.saveToRedis(fullSnapshot, s.presence[matchID])
	}
	return contracts.JoinMatchSeatResponse{
		Match:              fullSnapshot,
		SeatColor:          seatColor,
		Joined:             joined,
		WaitingForOpponent: state.Status == "waiting",
	}, nil
}

func (s *Service) GetMatch(matchID string) (contracts.MatchSnapshotResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	state, ok := s.ensureMatchLoadedLocked(matchID)
	if !ok {
		return contracts.MatchSnapshotResponse{}, ErrMatchNotFound
	}

	return buildSnapshotWithPresence(state, s.ensurePresenceStateLocked(matchID, state, time.Now().UTC()), 0, nil, time.Now().UTC()), nil
}

func (s *Service) HeartbeatPresence(matchID string, req contracts.MatchPresenceRequest, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	state, ok := s.ensureMatchLoadedLocked(matchID)
	if !ok {
		return ErrMatchNotFound
	}
	if state.Status == "finished" {
		return nil
	}

	color, err := requireIntentColor(state, strings.TrimSpace(req.PlayerID), strings.TrimSpace(req.PlayerSecret))
	if err != nil {
		return err
	}

	presence := s.ensurePresenceStateLocked(matchID, state, now)
	changed := presenceHeartbeat(presence, color, now)
	if !changed {
		return nil
	}

	return nil
}

func (s *Service) MarkDisconnected(matchID string, playerID string, playerSecret string, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	state, ok := s.ensureMatchLoadedLocked(matchID)
	if !ok || state.Status == "finished" {
		return nil
	}

	color, err := requireIntentColor(state, strings.TrimSpace(playerID), strings.TrimSpace(playerSecret))
	if err != nil {
		return err
	}

	presence := s.ensurePresenceStateLocked(matchID, state, now)
	if color == "white" {
		if !presence.WhiteConnected {
			return nil
		}
		presence.WhiteLastSeenAt = time.Time{}
		presence.WhiteConnected = false
	} else {
		if !presence.BlackConnected {
			return nil
		}
		presence.BlackLastSeenAt = time.Time{}
		presence.BlackConnected = false
	}

	snapshot := buildSnapshotWithPresence(state, presence, len(s.events[matchID]), nil, now)
	s.broadcastLocked(matchID, snapshot)
	return nil
}

func (s *Service) ApplyIntent(intent contracts.PlayerIntent, now time.Time) (contracts.MatchSnapshotResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	state, ok := s.ensureMatchLoadedLocked(intent.MatchID)
	if !ok {
		return contracts.MatchSnapshotResponse{}, ErrMatchNotFound
	}

	if intent.ClientMoveID != "" {
		for _, id := range state.SeenClientMoveIDs {
			if id == intent.ClientMoveID {
				presence := s.ensurePresenceStateLocked(intent.MatchID, state, now)
				return buildSnapshotWithPresence(state, presence, len(s.events[intent.MatchID]), nil, now), nil
			}
		}
	}

	presence := s.ensurePresenceStateLocked(intent.MatchID, state, now)

	actorColor, _ := requireIntentColor(state, intent.PlayerID, intent.PlayerSecret)
	if err := rateLimitIntent(presence, actorColor, now); err != nil {
		return contracts.MatchSnapshotResponse{}, err
	}

	timeoutEvents := syncClockForMutation(state, now)
	if len(timeoutEvents) > 0 {
		if events, err := applyIntent(state, intent, now); err == nil {
			trackIntentTime(presence, actorColor, now)
			events = append(events, timeoutEvents...)
			s.events[intent.MatchID] = append(s.events[intent.MatchID], events...)
			snapshot := buildSnapshotWithPresence(state, presence, len(s.events[intent.MatchID]), events, now)
			persistSnap := buildSnapshot(state, len(s.events[intent.MatchID]), s.events[intent.MatchID], now)
			s.persistSnapshot(persistSnap)
			s.saveToRedis(persistSnap, presence)
			s.broadcastLocked(intent.MatchID, snapshot)
			return snapshot, nil
		}
		s.events[intent.MatchID] = append(s.events[intent.MatchID], timeoutEvents...)
		snapshot := buildSnapshotWithPresence(state, presence, len(s.events[intent.MatchID]), timeoutEvents, now)
		persistSnap := buildSnapshot(state, len(s.events[intent.MatchID]), s.events[intent.MatchID], now)
		s.persistSnapshot(persistSnap)
		s.saveToRedis(persistSnap, presence)
		s.broadcastLocked(intent.MatchID, snapshot)
		return snapshot, nil
	}

	savedClock := state.Clock
	savedStatus := state.Status
	savedWinner := state.Winner
	savedFinishReason := state.FinishReason

	events, err := applyIntent(state, intent, now)
	if err != nil {
		state.Clock = savedClock
		state.Status = savedStatus
		state.Winner = savedWinner
		state.FinishReason = savedFinishReason
		return contracts.MatchSnapshotResponse{}, err
	}

	trackIntentTime(presence, actorColor, now)

	if shouldEvaluateAutomaticMatchFinish(state, intent) {
		events = finalizeAutomaticMatchFinish(state, events, now, intent.PlayerID)
	}

	timeoutEvents = syncClockForMutation(state, now)
	events = append(events, timeoutEvents...)

	if intent.ClientMoveID != "" {
		state.SeenClientMoveIDs = append(state.SeenClientMoveIDs, intent.ClientMoveID)
		if len(state.SeenClientMoveIDs) > 1000 {
			state.SeenClientMoveIDs = state.SeenClientMoveIDs[len(state.SeenClientMoveIDs)-1000:]
		}
	}

	s.events[intent.MatchID] = append(s.events[intent.MatchID], events...)
	snapshot := buildSnapshotWithPresence(state, presence, len(s.events[intent.MatchID]), events, now)
	persistSnap := buildSnapshot(state, len(s.events[intent.MatchID]), s.events[intent.MatchID], now)
	s.persistSnapshot(persistSnap)
	s.saveToRedis(persistSnap, presence)
	s.broadcastLocked(intent.MatchID, snapshot)

	return snapshot, nil
}

func rateLimitIntent(presence *matchPresenceState, actorColor string, now time.Time) error {
	if presence == nil || actorColor == "" {
		return nil
	}
	var lastIntentAt *time.Time
	if actorColor == "white" {
		lastIntentAt = &presence.WhiteLastIntentAt
	} else if actorColor == "black" {
		lastIntentAt = &presence.BlackLastIntentAt
	} else {
		return nil
	}
	if !lastIntentAt.IsZero() && now.Sub(*lastIntentAt) < time.Second/time.Duration(maxIntentsPerSecondPerPlayer) {
		return fmt.Errorf("rate limited: too many intents (max %d/sec)", maxIntentsPerSecondPerPlayer)
	}
	return nil
}

func trackIntentTime(presence *matchPresenceState, actorColor string, now time.Time) {
	if presence == nil || actorColor == "" {
		return
	}
	if actorColor == "white" {
		presence.WhiteLastIntentAt = now
	} else if actorColor == "black" {
		presence.BlackLastIntentAt = now
	}
}

func finalizeAutomaticMatchFinish(state *contracts.MatchState, events []contracts.ResolvedEvent, now time.Time, actorID string) []contracts.ResolvedEvent {
	if state == nil || state.Status != "active" {
		return events
	}

	winner, finishReason := evaluateAutomaticMatchFinish(state)
	if finishReason == "" {
		return events
	}

	markMatchFinished(state, winner, finishReason, now)

	clockUpdated := false
	for index := len(events) - 1; index >= 0; index-- {
		if events[index].Type != "clock_updated" {
			continue
		}
		if events[index].Payload == nil {
			events[index].Payload = map[string]any{}
		}
		events[index].Payload["runningFor"] = ""
		clockUpdated = true
		break
	}
	if !clockUpdated {
		events = append(events, makeEvent(state.MatchID, "clock_updated", now, actorID, map[string]any{
			"runningFor": "",
		}))
	}

	payload := map[string]any{
		"result": finishReason,
	}
	if winner != "" {
		payload["winner"] = winner
	}
	events = append(events, makeEvent(state.MatchID, "match_finished", now, actorID, payload))
	return events
}

func shouldEvaluateAutomaticMatchFinish(state *contracts.MatchState, intent contracts.PlayerIntent) bool {
	if state == nil || state.Status != "active" {
		return false
	}
	if state.InvisiblePiece != nil {
		return false
	}
	if state.DoubleMove != nil && state.DoubleMove.MovesLeft > 0 {
		return false
	}

	switch intent.Type {
	case "make_move":
		return true
	case "play_card", "select_target":
		return state.PendingCard == nil
	default:
		return false
	}
}

func evaluateAutomaticMatchFinish(state *contracts.MatchState) (string, string) {
	if state == nil {
		return "", ""
	}

	if state.HalfMoveClock >= 100 {
		return "draw", "fifty_move_rule"
	}

	historyKeys := make([]string, 0, len(state.History))
	for _, position := range state.History {
		historyKeys = append(historyKeys, positionKey(position.Board, position.Turn, sliceToSet(position.Moved), position.LastMove))
	}
	currentKey := positionKey(state.Board, state.Turn, sliceToSet(state.Moved), state.LastMove)
	if threefold(historyKeys, currentKey) {
		return "draw", "threefold_repetition"
	}

	_, isMate, isStale := gameStatusWithFusion(state.Board, state.Turn, state.LastMove, sliceToSet(state.Moved))
	if isMate {
		return opposite(state.Turn), "checkmate"
	}
	if isStale {
		return "draw", "stalemate"
	}
	if insufficientMaterial(state.Board) {
		return "draw", "insufficient_material"
	}

	return "", ""
}

func markMatchFinished(state *contracts.MatchState, winner string, finishReason string, now time.Time) {
	if state == nil {
		return
	}
	state.Status = "finished"
	state.Winner = winner
	state.FinishReason = finishReason
	state.DrawOfferedBy = ""
	state.Clock.RunningFor = ""
	state.Clock.StartedAt = nil
	state.PendingCard = nil
	state.DoubleMove = nil
	state.InvisiblePiece = nil
	state.FogZones = nil
	state.FortressZones = nil
	state.BombPieces = nil
	state.LavaSquares = nil
	// Clear all remaining transient effect state so archived/replayed snapshots are clean
	state.BlackHoles = nil
	state.UndoAgainst = ""
	state.RadarRevealFor = ""
	state.CheaterState = nil
	state.UpdatedAt = now.UTC()
}

func (s *Service) persistSnapshot(snapshot contracts.MatchSnapshotResponse) {
	if s.archive == nil {
		return
	}
	persisted := snapshot
	persisted.Match.WhiteConnected = false
	persisted.Match.BlackConnected = false
	persisted.Match.DisconnectGraceFor = ""
	persisted.Match.DisconnectGraceDeadline = nil
	if err := s.archive.Upsert(persisted); err != nil {
		s.Log.Error("failed to persist snapshot", "matchId", snapshot.Match.MatchID, "error", err)
	}
}

func (s *Service) saveToRedis(snapshot contracts.MatchSnapshotResponse, presence *matchPresenceState) {
	if s.store == nil {
		return
	}
	matchID := snapshot.Match.MatchID

	if err := s.store.SaveState(matchID, snapshot); err != nil {
		s.Log.Error("failed to save state to redis", "matchId", matchID, "error", err)
	}

	if err := s.store.SaveSecrets(matchID, snapshot.Match.WhitePlayerSecret, snapshot.Match.BlackPlayerSecret); err != nil {
		s.Log.Error("failed to save secrets to redis", "matchId", matchID, "error", err)
	}

	historyData, err := json.Marshal(snapshot.Match.History)
	if err == nil {
		_ = s.store.SaveHistory(matchID, historyData)
	}

	eventsData, err := json.Marshal(snapshot.Events)
	if err == nil {
		_ = s.store.SaveEvents(matchID, eventsData)
	}

	if presence != nil {
		presenceData, err := json.Marshal(presence)
		if err == nil {
			_ = s.store.SavePresence(matchID, presenceData)
		}
	}
}

func (s *Service) publishToRedis(matchID string, snapshot contracts.MatchSnapshotResponse) {
	if s.broadcaster == nil {
		return
	}
	data, err := json.Marshal(snapshot)
	if err != nil {
		s.Log.Error("failed to marshal snapshot for broadcast", "matchId", matchID, "error", err)
		return
	}
	if err := s.broadcaster.Publish(matchID, data); err != nil {
		s.Log.Error("failed to publish to redis", "matchId", matchID, "error", err)
	}
}

func (s *Service) Subscribe(matchID string, playerID string) (<-chan contracts.MatchSnapshotResponse, func(), contracts.MatchSnapshotResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	state, ok := s.ensureMatchLoadedLocked(matchID)
	if !ok {
		return nil, nil, contracts.MatchSnapshotResponse{}, ErrMatchNotFound
	}

	if s.subs[matchID] == nil {
		s.subs[matchID] = make(map[chan contracts.MatchSnapshotResponse]string)
	}

	const maxSubscribersPerMatch = 50
	if len(s.subs[matchID]) >= maxSubscribersPerMatch {
		return nil, nil, contracts.MatchSnapshotResponse{}, errors.New("max subscribers reached for match")
	}

	playerColor := ""
	if (state.WhiteGuestID != "" && strings.EqualFold(state.WhiteGuestID, playerID)) || (state.WhiteAccountID != "" && strings.EqualFold(state.WhiteAccountID, playerID)) {
		playerColor = "white"
	} else if (state.BlackGuestID != "" && strings.EqualFold(state.BlackGuestID, playerID)) || (state.BlackAccountID != "" && strings.EqualFold(state.BlackAccountID, playerID)) {
		playerColor = "black"
	}

	if playerColor == "" {
		return nil, nil, contracts.MatchSnapshotResponse{}, errors.New("unauthorized: must be a seated player to subscribe")
	}

	ch := make(chan contracts.MatchSnapshotResponse, 128)
	s.subs[matchID][ch] = playerColor

	now := time.Now().UTC()
	baseInitial := buildSnapshotWithPresence(state, s.ensurePresenceStateLocked(matchID, state, now), len(s.events[matchID]), s.events[matchID], now)
	initial := contracts.MatchSnapshotResponse{
		Match:      filterStateForColor(baseInitial.Match, playerColor),
		ReplayHead: baseInitial.ReplayHead,
		Events:     baseInitial.Events,
	}

	unsubscribe := func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		if subs, exists := s.subs[matchID]; exists {
			if _, present := subs[ch]; present {
				delete(subs, ch)
				close(ch)
			}
		}
	}

	return ch, unsubscribe, initial, nil
}

func (s *Service) ensureMatchLoadedLocked(matchID string) (*contracts.MatchState, bool) {
	if state, ok := s.matches[matchID]; ok {
		return state, true
	}

	loader, ok := s.archive.(MatchArchiveLoader)
	if !ok {
		return nil, false
	}

	restored, events, ok := loader.LoadMatch(matchID)
	if !ok {
		return nil, false
	}

	if len(restored.History) == 0 {
		restored.History = []contracts.PositionState{capturePositionState(&restored)}
	}

	return s.loadArchivedMatchLocked(matchID, restored, events), true
}

func (s *Service) restoreArchivedMatchesLocked(loader MatchArchiveBootstrapper) {
	for _, matchID := range loader.ListUnfinishedMatchIDs(0) {
		if _, ok := s.matches[matchID]; ok {
			continue
		}
		restored, events, ok := loader.LoadMatch(matchID)
		if !ok {
			continue
		}
		if len(restored.History) == 0 {
			restored.History = []contracts.PositionState{capturePositionState(&restored)}
		}
		s.loadArchivedMatchLocked(matchID, restored, events)
	}
}

func (s *Service) lockMatch(matchID string) func() {
	s.mu.Lock()
	mu, ok := s.matchMu[matchID]
	if !ok {
		mu = &sync.Mutex{}
		s.matchMu[matchID] = mu
	}
	s.mu.Unlock()
	mu.Lock()
	return mu.Unlock
}

func (s *Service) loadArchivedMatchLocked(matchID string, restored contracts.MatchState, events []contracts.ResolvedEvent) *contracts.MatchState {
	state := &restored
	s.matches[matchID] = state
	s.events[matchID] = append([]contracts.ResolvedEvent{}, events...)
	if s.subs[matchID] == nil {
		s.subs[matchID] = make(map[chan contracts.MatchSnapshotResponse]string)
	}
	s.presence[matchID] = newRecoveredMatchPresenceState(state)
	return state
}

const authTokenTTL = 5 * time.Minute

func (s *Service) CreateAuthToken(playerID, playerSecret string, now time.Time) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		token := fmt.Sprintf("at_%d_%s", now.UnixNano(), playerID)
		s.authTokens[token] = authTokenEntry{
			PlayerID:     playerID,
			PlayerSecret: playerSecret,
			ExpiresAt:    now.Add(authTokenTTL),
		}
		return token
	}
	token := "at_" + hex.EncodeToString(raw)
	s.authTokens[token] = authTokenEntry{
		PlayerID:     playerID,
		PlayerSecret: playerSecret,
		ExpiresAt:    now.Add(authTokenTTL),
	}
	return token
}

func (s *Service) ResolveAuthToken(token string) (string, string, bool) {
	if token == "" {
		return "", "", false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.authTokens[token]
	if !ok {
		return "", "", false
	}
	if time.Now().After(entry.ExpiresAt) {
		delete(s.authTokens, token)
		return "", "", false
	}
	delete(s.authTokens, token)
	return entry.PlayerID, entry.PlayerSecret, true
}

func (s *Service) cleanupAuthTokensLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-s.stopCh:
			return
		case now := <-ticker.C:
			s.mu.Lock()
			for token, entry := range s.authTokens {
				if now.After(entry.ExpiresAt) {
					delete(s.authTokens, token)
				}
			}
			s.mu.Unlock()
		}
	}
}

func (s *Service) Stats() ServiceStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := ServiceStats{
		LoadedMatches:     len(s.matches),
		BufferedEventSets: len(s.events),
	}
	for matchID, state := range s.matches {
		if state.Status == "finished" {
			stats.FinishedMatches++
		} else {
			stats.ActiveMatches++
		}
		stats.SubscriberCount += len(s.subs[matchID])
	}
	return stats
}

func (s *Service) Close() {
	close(s.stopCh)
	s.broadcastWG.Wait()
}

func newMatchPresenceState(state *contracts.MatchState, now time.Time) *matchPresenceState {
	presence := &matchPresenceState{}
	if strings.TrimSpace(state.WhiteGuestID) != "" {
		presence.WhiteLastSeenAt = now
		presence.WhiteConnected = true
	}
	if strings.TrimSpace(state.BlackGuestID) != "" {
		presence.BlackLastSeenAt = now
		presence.BlackConnected = true
	}
	return presence
}

func newRecoveredMatchPresenceState(state *contracts.MatchState) *matchPresenceState {
	presence := &matchPresenceState{}
	lastSeen := recoveredPresenceSeedTime(state)
	if strings.TrimSpace(state.WhiteGuestID) != "" {
		presence.WhiteLastSeenAt = lastSeen
		presence.WhiteConnected = false
	}
	if strings.TrimSpace(state.BlackGuestID) != "" {
		presence.BlackLastSeenAt = lastSeen
		presence.BlackConnected = false
	}
	return presence
}

func recoveredPresenceSeedTime(state *contracts.MatchState) time.Time {
	if state == nil {
		return time.Time{}
	}
	if !state.UpdatedAt.IsZero() {
		return state.UpdatedAt.UTC()
	}
	if !state.CreatedAt.IsZero() {
		return state.CreatedAt.UTC()
	}
	return time.Time{}
}

func (s *Service) ensurePresenceStateLocked(matchID string, state *contracts.MatchState, now time.Time) *matchPresenceState {
	if presence, ok := s.presence[matchID]; ok && presence != nil {
		return presence
	}
	presence := newMatchPresenceState(state, now)
	s.presence[matchID] = presence
	return presence
}

func presenceHeartbeat(presence *matchPresenceState, color string, now time.Time) bool {
	if presence == nil {
		return false
	}

	changed := false
	switch color {
	case "white":
		if !presence.WhiteConnected || !presence.WhiteLastSeenAt.Equal(now) {
			changed = true
		}
		presence.WhiteConnected = true
		presence.WhiteLastSeenAt = now
	case "black":
		if !presence.BlackConnected || !presence.BlackLastSeenAt.Equal(now) {
			changed = true
		}
		presence.BlackConnected = true
		presence.BlackLastSeenAt = now
	}

	if presence.DisconnectGraceFor == color || presence.DisconnectGraceFor == disconnectGraceBoth {
		presence.DisconnectGraceFor = ""
		presence.DisconnectGraceDeadline = nil
		changed = true
	}

	return changed
}

func evaluatePresenceRuntime(state *contracts.MatchState, presence *matchPresenceState, now time.Time) []contracts.ResolvedEvent {
	if state == nil || presence == nil {
		return nil
	}
	if state.Status == "finished" {
		presence.DisconnectGraceFor = ""
		presence.DisconnectGraceDeadline = nil
		return nil
	}

	whiteOccupied := strings.TrimSpace(state.WhiteGuestID) != ""
	blackOccupied := strings.TrimSpace(state.BlackGuestID) != ""

	if whiteOccupied {
		presence.WhiteConnected = now.Sub(presence.WhiteLastSeenAt) <= presenceHeartbeatTimeout
	} else {
		presence.WhiteConnected = false
	}
	if blackOccupied {
		presence.BlackConnected = now.Sub(presence.BlackLastSeenAt) <= presenceHeartbeatTimeout
	} else {
		presence.BlackConnected = false
	}

	if state.Status != "active" || !whiteOccupied || !blackOccupied {
		presence.DisconnectGraceFor = ""
		presence.DisconnectGraceDeadline = nil
		return nil
	}

	disconnectedColor := ""
	switch {
	case !presence.WhiteConnected && presence.BlackConnected:
		disconnectedColor = "white"
	case presence.WhiteConnected && !presence.BlackConnected:
		disconnectedColor = "black"
	case !presence.WhiteConnected && !presence.BlackConnected:
		disconnectedColor = disconnectGraceBoth
	default:
		disconnectedColor = ""
	}

	if disconnectedColor == "" {
		presence.DisconnectGraceFor = ""
		presence.DisconnectGraceDeadline = nil
		return nil
	}

	deadline := presenceDisconnectDeadline(presence, disconnectedColor)
	if deadline.IsZero() {
		deadline = now.Add(disconnectGracePeriod)
	}
	if presence.DisconnectGraceFor != disconnectedColor || presence.DisconnectGraceDeadline == nil || !presence.DisconnectGraceDeadline.Equal(deadline) {
		presence.DisconnectGraceFor = disconnectedColor
		presence.DisconnectGraceDeadline = &deadline
		if now.Before(deadline) {
			return nil
		}
	}

	if now.Before(*presence.DisconnectGraceDeadline) {
		return nil
	}

	presence.DisconnectGraceFor = ""
	presence.DisconnectGraceDeadline = nil
	if disconnectedColor == disconnectGraceBoth {
		winner := "draw"
		finishReason := "abandon"
		result := "abandon"
		if len(state.MoveHistory) == 0 {
			winner = "aborted"
			finishReason = "abort"
			result = "abort"
		}
		markMatchFinished(state, winner, finishReason, now)
		return []contracts.ResolvedEvent{
			makeEvent(state.MatchID, "match_finished", now, "", map[string]any{
				"result":       result,
				"winner":       state.Winner,
				"disconnected": disconnectedColor,
			}),
		}
	}
	markMatchFinished(state, opposite(disconnectedColor), "abandon", now)
	return []contracts.ResolvedEvent{
		makeEvent(state.MatchID, "match_finished", now, "", map[string]any{
			"result":       "abandon",
			"winner":       state.Winner,
			"disconnected": disconnectedColor,
		}),
	}
}

func presenceDisconnectDeadline(presence *matchPresenceState, disconnectedColor string) time.Time {
	if presence == nil {
		return time.Time{}
	}
	switch disconnectedColor {
	case "white":
		if presence.WhiteLastSeenAt.IsZero() {
			return time.Time{}
		}
		return presence.WhiteLastSeenAt.Add(presenceHeartbeatTimeout + disconnectGracePeriod)
	case "black":
		if presence.BlackLastSeenAt.IsZero() {
			return time.Time{}
		}
		return presence.BlackLastSeenAt.Add(presenceHeartbeatTimeout + disconnectGracePeriod)
	case disconnectGraceBoth:
		lastSeen := presence.WhiteLastSeenAt
		if presence.BlackLastSeenAt.After(lastSeen) {
			lastSeen = presence.BlackLastSeenAt
		}
		if lastSeen.IsZero() {
			return time.Time{}
		}
		return lastSeen.Add(presenceHeartbeatTimeout + disconnectGraceBothPeriod)
	default:
		return time.Time{}
	}
}

func (s *Service) broadcastLocked(matchID string, snapshot contracts.MatchSnapshotResponse) {
	subscribers := s.subs[matchID]

	s.matchSeqNum[matchID]++
	snapshot.SeqNum = s.matchSeqNum[matchID]

	s.publishToRedis(matchID, snapshot)

	if len(subscribers) == 0 {
		return
	}

	cachedWhite := snapshot
	cachedWhite.Match = filterStateForColor(snapshot.Match, "white")
	cachedBlack := snapshot
	cachedBlack.Match = filterStateForColor(snapshot.Match, "black")
	cachedSpec := snapshot
	cachedSpec.Match = filterStateForColor(snapshot.Match, "")

	for ch, color := range subscribers {
		if color == "white" {
			pushSnapshot(ch, cachedWhite)
		} else if color == "black" {
			pushSnapshot(ch, cachedBlack)
		} else {
			pushSnapshot(ch, cachedSpec)
		}
	}
}

func pushSnapshot(ch chan contracts.MatchSnapshotResponse, snapshot contracts.MatchSnapshotResponse) {
	defer func() {
		_ = recover()
	}()
	select {
	case ch <- snapshot:
	default:
		log.Printf("pushSnapshot: dropping event seq=%d for channel %p (buffer full)", snapshot.SeqNum, ch)
	}
}

func (s *Service) startBroadcaster() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case now, ok := <-ticker.C:
			if !ok {
				return
			}
			s.broadcastWG.Add(1)
			s.collectAndBroadcast(now.UTC())
			s.broadcastWG.Done()
			s.gcFinishedMatches(now.UTC())
		}
	}
}

func (s *Service) collectAndBroadcast(now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for matchID, state := range s.matches {
		if state.Status == "finished" {
			continue
		}

		presence := s.ensurePresenceStateLocked(matchID, state, now)

		recentCutoff := now.Add(-presenceHeartbeatTimeout)
		hasRecentActivity := (!presence.WhiteLastSeenAt.IsZero() && presence.WhiteLastSeenAt.After(recentCutoff)) ||
			(!presence.BlackLastSeenAt.IsZero() && presence.BlackLastSeenAt.After(recentCutoff))
		if hasRecentActivity {
			timeoutEvents := syncClockForMutation(state, now)
			if len(timeoutEvents) > 0 {
				s.events[matchID] = append(s.events[matchID], timeoutEvents...)
				s.broadcastLocked(matchID, buildSnapshotWithPresence(state, presence, len(s.events[matchID]), timeoutEvents, now))
			}
			s.persistSnapshot(buildSnapshot(state, len(s.events[matchID]), s.events[matchID], now))
			if len(timeoutEvents) > 0 {
				continue
			}
		}

		runtimeEvents := evaluatePresenceRuntime(state, presence, now)
		if len(runtimeEvents) > 0 {
			s.events[matchID] = append(s.events[matchID], runtimeEvents...)
			s.persistSnapshot(buildSnapshot(state, len(s.events[matchID]), s.events[matchID], now))
			s.broadcastLocked(matchID, buildSnapshotWithPresence(state, presence, len(s.events[matchID]), runtimeEvents, now))
			continue
		}
		if len(s.subs[matchID]) == 0 {
			continue
		}
		s.broadcastLocked(matchID, buildSnapshotWithPresence(state, presence, len(s.events[matchID]), nil, now))
	}
}

func (s *Service) gcFinishedMatches(now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	const finishedMatchTTL = 5 * time.Minute
	const waitingMatchTTL = 5 * time.Minute

	for matchID, state := range s.matches {
		if state.Status == "finished" {
			if now.Sub(state.UpdatedAt) >= finishedMatchTTL {
				delete(s.matches, matchID)
				delete(s.events, matchID)
				delete(s.subs, matchID)
				delete(s.matchSeqNum, matchID)
				delete(s.presence, matchID)
			}
		} else if state.Status == "waiting" {
			if now.Sub(state.UpdatedAt) >= waitingMatchTTL {
				delete(s.matches, matchID)
				delete(s.events, matchID)
				delete(s.subs, matchID)
				delete(s.matchSeqNum, matchID)
				delete(s.presence, matchID)
			}
		}
	}
}

func applyIntent(state *contracts.MatchState, intent contracts.PlayerIntent, now time.Time) ([]contracts.ResolvedEvent, error) {
	switch intent.Type {
	case "make_move":
		return applyMove(state, intent, now)
	case "play_card":
		return applyPlayCard(state, intent, now)
	case "select_target":
		return applySelectTarget(state, intent, now)
	case "send_chat":
		return applyChat(state, intent, now)
	case "offer_draw":
		return applyOfferDraw(state, intent, now)
	case "respond_draw":
		return applyRespondDraw(state, intent, now)
	case "abort":
		return applyAbort(state, intent, now)
	case "resign":
		return applyResign(state, intent, now)
	default:
		return nil, fmt.Errorf("unsupported intent type: %s", intent.Type)
	}
}

func applyPlayCard(state *contracts.MatchState, intent contracts.PlayerIntent, now time.Time) ([]contracts.ResolvedEvent, error) {
	if err := ensureActive(state); err != nil {
		return nil, err
	}

	owner, err := requireIntentColor(state, intent.PlayerID, intent.PlayerSecret)
	if err != nil {
		return nil, err
	}
	if owner != state.Turn {
		return nil, errors.New("cards can only be played on your turn")
	}
	if state.DoubleMove != nil {
		return nil, errors.New("resolve the active double move before playing another card")
	}
	if state.PendingCard != nil {
		return nil, errors.New("resolve the pending card target first")
	}

	card, found := cardFromHand(state, owner, intent.CardID)
	if !found {
		return nil, errors.New("card not found in hand")
	}
	if state.UndoAgainst == owner {
		removeCardFromHand(state, owner, card.ID)
		state.UndoAgainst = ""
		state.DrawOfferedBy = ""
		state.UpdatedAt = now.UTC()
		return []contracts.ResolvedEvent{
			makeEvent(state.MatchID, "card_played", now, intent.PlayerID, map[string]any{
				"cardId":    card.ID,
				"mechanic":  card.Mechanic,
				"nullified": true,
			}),
			makeEvent(state.MatchID, "target_selected", now, intent.PlayerID, map[string]any{
				"effect": "undo_nullified_card",
			}),
		}, nil
	}

	if card.Mechanic == "doublemove_diff" || card.Mechanic == "doublemove_same" {
		moveType := "diff"
		if card.Mechanic == "doublemove_same" {
			moveType = "same"
		}
		removeCardFromHand(state, owner, card.ID)
		state.DoubleMove = &contracts.DoubleMoveState{
			Type:      moveType,
			MovesLeft: 2,
		}
		state.DrawOfferedBy = ""
		state.UpdatedAt = now.UTC()
		return []contracts.ResolvedEvent{
			makeEvent(state.MatchID, "card_played", now, intent.PlayerID, map[string]any{
				"cardId":     card.ID,
				"mechanic":   card.Mechanic,
				"doubleMove": state.DoubleMove,
			}),
		}, nil
	}
	if card.Mechanic == "undo" {
		removeCardFromHand(state, owner, card.ID)
		state.UndoAgainst = opposite(owner)
		state.DrawOfferedBy = ""
		state.UpdatedAt = now.UTC()
		return []contracts.ResolvedEvent{
			makeEvent(state.MatchID, "card_played", now, intent.PlayerID, map[string]any{
				"cardId":      card.ID,
				"mechanic":    card.Mechanic,
				"undoAgainst": state.UndoAgainst,
			}),
		}, nil
	}
	if card.Mechanic == "reverse" {
		if len(state.History) < 2 {
			return nil, errors.New("no move to reverse yet")
		}
		restored := state.History[len(state.History)-2]
		if king := findKing(restored.Board, owner); king != nil && isAttackedWithFusion(restored.Board, *king, opposite(owner)) {
			return nil, errors.New("cannot reverse because your king would be in check")
		}
		if oppKing := findKing(restored.Board, opposite(owner)); oppKing != nil && isAttackedWithFusion(restored.Board, *oppKing, owner) {
			return nil, errors.New("cannot reverse because enemy king would be in check")
		}
		// Restore first so we consume the card from the restored hand (which
		// is the pre-reverse state) — not the live hand. If the reverse card
		// is not in the restored hand (e.g. it was drawn after the move
		// being reversed) the call is a no-op and the card remains
		// unconsumed, which is the correct semantic: the card was added
		// after the action it claims to undo.
		restorePositionState(state, restored)
		removeCardFromHand(state, owner, card.ID)
		if len(state.History) > 0 {
			state.History = append([]contracts.PositionState{}, state.History[:len(state.History)-1]...)
		}
		state.UpdatedAt = now.UTC()
		return []contracts.ResolvedEvent{
			makeEvent(state.MatchID, "card_played", now, intent.PlayerID, map[string]any{
				"cardId":   card.ID,
				"mechanic": card.Mechanic,
			}),
			makeEvent(state.MatchID, "target_selected", now, intent.PlayerID, map[string]any{
				"effect": "reverse_applied",
			}),
		}, nil
	}
	if card.Mechanic == "mirror" {
		removeCardFromHand(state, owner, card.ID)
		mirrored, from, to, err := applyMirrorCard(state, owner)
		if err != nil {
			return nil, err
		}
		state.DrawOfferedBy = ""
		state.UpdatedAt = now.UTC()

		payload := map[string]any{
			"cardId":   card.ID,
			"mechanic": card.Mechanic,
			"mirrored": mirrored,
		}
		if from != nil {
			payload["from"] = from
		}
		if to != nil {
			payload["to"] = to
		}

		return []contracts.ResolvedEvent{
			makeEvent(state.MatchID, "card_played", now, intent.PlayerID, payload),
			makeEvent(state.MatchID, "target_selected", now, intent.PlayerID, payload),
		}, nil
	}
	if card.Mechanic == "gambler" {
		removeCardFromHand(state, owner, card.ID)
		payload := resolveGamblerCard(state, owner, now)
		state.DrawOfferedBy = ""
		state.UpdatedAt = now.UTC()
		payload["cardId"] = card.ID
		payload["mechanic"] = card.Mechanic
		return []contracts.ResolvedEvent{
			makeEvent(state.MatchID, "card_played", now, intent.PlayerID, payload),
		}, nil
	}
	if card.Mechanic == "radar" {
		removeCardFromHand(state, owner, card.ID)
		state.RadarRevealFor = owner
		state.DrawOfferedBy = ""
		state.UpdatedAt = now.UTC()
		return []contracts.ResolvedEvent{
			makeEvent(state.MatchID, "card_played", now, intent.PlayerID, map[string]any{
				"cardId":         card.ID,
				"mechanic":       card.Mechanic,
				"radarRevealFor": owner,
			}),
		}, nil
	}
	if card.Mechanic == "cheater" {
		removeCardFromHand(state, owner, card.ID)
		state.CheaterState = &contracts.CheaterState{
			OwnerColor: owner,
			TurnsLeft:  3,
		}
		state.DrawOfferedBy = ""
		state.UpdatedAt = now.UTC()
		return []contracts.ResolvedEvent{
			makeEvent(state.MatchID, "card_played", now, intent.PlayerID, map[string]any{
				"cardId":       card.ID,
				"mechanic":     card.Mechanic,
				"cheaterState": state.CheaterState,
			}),
		}, nil
	}
	if card.Mechanic == "joker" {
		state.PendingCard = &contracts.PendingCardState{
			CardID:     card.ID,
			Mechanic:   card.Mechanic,
			OwnerColor: owner,
			Options:    jokerTransformOptions(),
		}
		state.DrawOfferedBy = ""
		state.UpdatedAt = now.UTC()
		return []contracts.ResolvedEvent{
			makeEvent(state.MatchID, "card_played", now, intent.PlayerID, map[string]any{
				"cardId":   card.ID,
				"mechanic": card.Mechanic,
				"options":  state.PendingCard.Options,
			}),
		}, nil
	}

	state.PendingCard = &contracts.PendingCardState{
		CardID:     card.ID,
		Mechanic:   card.Mechanic,
		OwnerColor: owner,
	}
	state.UpdatedAt = now.UTC()

	return []contracts.ResolvedEvent{
		makeEvent(state.MatchID, "card_played", now, intent.PlayerID, map[string]any{
			"cardId":   card.ID,
			"mechanic": card.Mechanic,
		}),
	}, nil
}

func applySelectTarget(state *contracts.MatchState, intent contracts.PlayerIntent, now time.Time) ([]contracts.ResolvedEvent, error) {
	if err := ensureActive(state); err != nil {
		return nil, err
	}
	if state.PendingCard == nil {
		return nil, errors.New("no pending card target selection")
	}
	pending := state.PendingCard
	owner, err := requireIntentColor(state, intent.PlayerID, intent.PlayerSecret)
	if err != nil {
		return nil, err
	}
	if owner != pending.OwnerColor {
		return nil, errors.New("only the card owner can select the target")
	}

	switch pending.Mechanic {
	case "freeze":
		if intent.Target == nil {
			return nil, errors.New("target selection requires a target square")
		}
		targetPiece := pieceAt(state.Board, *intent.Target)
		if targetPiece == nil {
			return nil, errors.New("target square has no piece")
		}
		if targetPiece.Color == pending.OwnerColor || targetPiece.Type == "king" {
			return nil, errors.New("freeze requires an enemy non-king target")
		}
		targetPiece.Frozen = true
		replaceLastHistorySnapshot(state)
	case "shield":
		if intent.Target == nil {
			return nil, errors.New("target selection requires a target square")
		}
		targetPiece := pieceAt(state.Board, *intent.Target)
		if targetPiece == nil {
			return nil, errors.New("target square has no piece")
		}
		if targetPiece.Color != pending.OwnerColor || targetPiece.Type == "king" {
			return nil, errors.New("shield requires your own non-king target")
		}
		targetPiece.Shielded = true
		shieldTurn := state.FullMoveNum + 1
		targetPiece.ShieldTurn = &shieldTurn
		replaceLastHistorySnapshot(state)
	case "sniper":
		if intent.Target == nil {
			return nil, errors.New("target selection requires a target square")
		}
		targetPiece := pieceAt(state.Board, *intent.Target)
		if targetPiece == nil {
			return nil, errors.New("target square has no piece")
		}
		if targetPiece.Color == pending.OwnerColor || targetPiece.Type == "king" {
			return nil, errors.New("sniper requires an enemy non-king target")
		}
		if err := ensureRemovalDoesNotCreateCheck(state.Board, *intent.Target, pending.OwnerColor); err != nil {
			return nil, err
		}
		if targetPiece.Shielded {
			targetPiece.Shielded = false
			targetPiece.ShieldTurn = nil
			state.DrawOfferedBy = ""
			state.UpdatedAt = now.UTC()
			return []contracts.ResolvedEvent{
				makeEvent(state.MatchID, "target_selected", now, intent.PlayerID, map[string]any{
					"effect": "shield_blocked_capture",
					"target": intent.Target,
				}),
			}, nil
		}
		state.Board[intent.Target.Row][intent.Target.Col] = nil
		replaceLastHistorySnapshot(state)
	case "badsniper":
		if intent.Target == nil {
			return nil, errors.New("target selection requires a target square")
		}
		targetPiece := pieceAt(state.Board, *intent.Target)
		if targetPiece == nil {
			return nil, errors.New("target square has no piece")
		}
		if targetPiece.Color != pending.OwnerColor || targetPiece.Type == "king" {
			return nil, errors.New("badsniper requires your own non-king target")
		}
		if err := ensureRemovalDoesNotCreateCheck(state.Board, *intent.Target, pending.OwnerColor); err != nil {
			return nil, err
		}
		if targetPiece.Shielded {
			targetPiece.Shielded = false
			targetPiece.ShieldTurn = nil
			state.DrawOfferedBy = ""
			state.UpdatedAt = now.UTC()
			return []contracts.ResolvedEvent{
				makeEvent(state.MatchID, "target_selected", now, intent.PlayerID, map[string]any{
					"effect": "shield_blocked_capture",
					"target": intent.Target,
				}),
			}, nil
		}
		state.Board[intent.Target.Row][intent.Target.Col] = nil
		replaceLastHistorySnapshot(state)
	case "promote", "demote", "promotehim", "demotehim":
		if pending.Target == nil {
			if intent.Target == nil {
				return nil, errors.New("target selection requires a target square")
			}
			targetPiece := pieceAt(state.Board, *intent.Target)
			if targetPiece == nil {
				return nil, errors.New("target square has no piece")
			}
			if err := validateTransformTarget(targetPiece, pending.OwnerColor, pending.Mechanic); err != nil {
				return nil, err
			}
			if targetPiece.Frozen && (pending.Mechanic == "demote" || pending.Mechanic == "demotehim" ||
				(pending.Mechanic == "promote" && targetPiece.Color == pending.OwnerColor) ||
				(pending.Mechanic == "promotehim" && targetPiece.Color != pending.OwnerColor)) {
				return nil, errors.New("transform cannot target a frozen piece")
			}
			options := safeTransformOptions(state.Board, *intent.Target, pending.Mechanic)
			if len(options) == 0 {
				return nil, fmt.Errorf("no safe %s options available", pending.Mechanic)
			}
			pending.Target = intent.Target
			pending.Options = options
			state.UpdatedAt = now.UTC()
			return []contracts.ResolvedEvent{
				makeEvent(state.MatchID, "target_selected", now, intent.PlayerID, map[string]any{
					"cardId":   pending.CardID,
					"mechanic": pending.Mechanic,
					"target":   intent.Target,
					"options":  options,
				}),
			}, nil
		}

		if intent.SelectionID == "" {
			return nil, errors.New("transform selection requires a selectionId")
		}
		if !containsString(pending.Options, intent.SelectionID) {
			return nil, errors.New("selected transform is not allowed")
		}
		targetPiece := pieceAt(state.Board, *pending.Target)
		if targetPiece == nil {
			return nil, errors.New("pending target piece no longer exists")
		}
		targetPiece.Type = intent.SelectionID
		replaceLastHistorySnapshot(state)
	case "teleport":
		if pending.Target == nil {
			if intent.Target == nil {
				return nil, errors.New("teleport requires selecting your piece first")
			}
			targetPiece := pieceAt(state.Board, *intent.Target)
			if targetPiece == nil || targetPiece.Color != pending.OwnerColor || targetPiece.Type == "king" {
				return nil, errors.New("teleport requires your own non-king target")
			}
			if targetPiece.Frozen {
				return nil, errors.New("teleport cannot target a frozen piece")
			}
			pending.Target = intent.Target
			state.UpdatedAt = now.UTC()
			return []contracts.ResolvedEvent{
				makeEvent(state.MatchID, "target_selected", now, intent.PlayerID, map[string]any{
					"cardId":   pending.CardID,
					"mechanic": pending.Mechanic,
					"target":   intent.Target,
				}),
			}, nil
		}
		if intent.Target == nil {
			return nil, errors.New("teleport requires choosing a destination square")
		}
		if !inBounds(intent.Target.Row, intent.Target.Col) {
			return nil, errors.New("teleport destination is out of bounds")
		}
		if pieceAt(state.Board, *intent.Target) != nil {
			return nil, errors.New("teleport destination must be empty")
		}
		if fortressEntryBlocked(state.FortressZones, pending.OwnerColor, *intent.Target) {
			return nil, errors.New("teleport destination is protected by an enemy fortress")
		}
		movingPiece := pieceAt(state.Board, *pending.Target)
		if movingPiece == nil {
			return nil, errors.New("teleport source piece no longer exists")
		}
		if movingPiece.Frozen {
			return nil, errors.New("teleport cannot move a frozen piece")
		}
		nextBoard := cloneBoard(state.Board)
		nextMovingPiece := nextBoard[pending.Target.Row][pending.Target.Col]
		nextBoard[intent.Target.Row][intent.Target.Col] = nextMovingPiece
		nextBoard[pending.Target.Row][pending.Target.Col] = nil
		if !kingsRemainSafe(nextBoard) {
			return nil, errors.New("teleport destination is not safe")
		}
		state.Board = nextBoard
		invalidateCastlingRightsForSquare(state, *pending.Target)
		replaceLastHistorySnapshot(state)
	case "jump":
		if pending.Target == nil {
			if intent.Target == nil {
				return nil, errors.New("jump requires selecting your piece first")
			}
			targetPiece := pieceAt(state.Board, *intent.Target)
			if targetPiece == nil || targetPiece.Color != pending.OwnerColor || targetPiece.Type == "king" || targetPiece.Type == "knight" {
				return nil, errors.New("jump requires your own non-king, non-knight target")
			}
			if targetPiece.Frozen {
				return nil, errors.New("jump cannot target a frozen piece")
			}
			pending.Target = intent.Target
			state.UpdatedAt = now.UTC()
			return []contracts.ResolvedEvent{
				makeEvent(state.MatchID, "target_selected", now, intent.PlayerID, map[string]any{
					"cardId":   pending.CardID,
					"mechanic": pending.Mechanic,
					"target":   intent.Target,
				}),
			}, nil
		}
		if intent.Target == nil {
			return nil, errors.New("jump requires choosing a destination square")
		}
		if !inBounds(intent.Target.Row, intent.Target.Col) {
			return nil, errors.New("jump destination is out of bounds")
		}
		if fortressEntryBlocked(state.FortressZones, pending.OwnerColor, *intent.Target) {
			return nil, errors.New("jump destination is protected by an enemy fortress")
		}
		fromPiece := pieceAt(state.Board, *pending.Target)
		if fromPiece == nil {
			return nil, errors.New("jump source piece no longer exists")
		}
		if fromPiece.Frozen {
			return nil, errors.New("jump cannot move a frozen piece")
		}
		destinationPiece := pieceAt(state.Board, *intent.Target)
		if destinationPiece != nil && destinationPiece.Color == pending.OwnerColor {
			return nil, errors.New("jump cannot land on your own piece")
		}
		if destinationPiece != nil && destinationPiece.Type == "king" {
			return nil, errors.New("jump cannot capture the king")
		}
		if !jumpDirectionValid(*pending.Target, *intent.Target, fromPiece.Type, fromPiece.Color) {
			return nil, errors.New("jump destination is invalid for that piece")
		}
		if !jumpHasExactlyOnePieceBetween(state.Board, *pending.Target, *intent.Target) {
			return nil, errors.New("jump must have exactly one piece in between")
		}
		if fromPiece.Type == "pawn" && pending.Target.Col == intent.Target.Col && destinationPiece != nil {
			return nil, errors.New("pawn can only jump straight to an empty square")
		}
		nextBoard := cloneBoard(state.Board)
		nextMovingPiece := nextBoard[pending.Target.Row][pending.Target.Col]
		nextBoard[intent.Target.Row][intent.Target.Col] = nextMovingPiece
		nextBoard[pending.Target.Row][pending.Target.Col] = nil
		if !kingsRemainSafe(nextBoard) {
			return nil, errors.New("jump destination is not safe")
		}
		state.Board = nextBoard
		invalidateCastlingRightsForSquare(state, *pending.Target)
		replaceLastHistorySnapshot(state)
	case "swapme":
		if pending.Target == nil {
			if intent.Target == nil {
				return nil, errors.New("swapme requires selecting your first piece")
			}
			targetPiece := pieceAt(state.Board, *intent.Target)
			if targetPiece == nil || targetPiece.Color != pending.OwnerColor || targetPiece.Type == "king" {
				return nil, errors.New("swapme requires your own non-king target")
			}
			if targetPiece.Frozen {
				return nil, errors.New("swapme cannot target a frozen piece")
			}
			pending.Target = intent.Target
			state.UpdatedAt = now.UTC()
			return []contracts.ResolvedEvent{
				makeEvent(state.MatchID, "target_selected", now, intent.PlayerID, map[string]any{
					"cardId":   pending.CardID,
					"mechanic": pending.Mechanic,
					"target":   intent.Target,
				}),
			}, nil
		}
		if intent.Target == nil {
			return nil, errors.New("swapme requires selecting your second piece")
		}
		firstPiece := pieceAt(state.Board, *pending.Target)
		secondPiece := pieceAt(state.Board, *intent.Target)
		if firstPiece == nil {
			return nil, errors.New("swapme first piece no longer exists")
		}
		if secondPiece.Frozen {
			return nil, errors.New("swapme cannot target a frozen piece")
		}
		if secondPiece == nil || secondPiece.Color != pending.OwnerColor || secondPiece.Type == "king" {
			return nil, errors.New("swapme requires your own non-king second piece")
		}
		if pending.Target.Row == intent.Target.Row && pending.Target.Col == intent.Target.Col {
			return nil, errors.New("swapme requires two different pieces")
		}
		nextBoard := cloneBoard(state.Board)
		nextBoard[pending.Target.Row][pending.Target.Col], nextBoard[intent.Target.Row][intent.Target.Col] = nextBoard[intent.Target.Row][intent.Target.Col], nextBoard[pending.Target.Row][pending.Target.Col]
		if !kingsRemainSafe(nextBoard) {
			return nil, errors.New("swapme would create check")
		}
		state.Board = nextBoard
		invalidateCastlingRightsForSquare(state, *pending.Target)
		invalidateCastlingRightsForSquare(state, *intent.Target)
		replaceLastHistorySnapshot(state)
	case "swapus":
		if pending.Target == nil {
			if intent.Target == nil {
				return nil, errors.New("swapus requires selecting your piece first")
			}
			targetPiece := pieceAt(state.Board, *intent.Target)
			if targetPiece == nil || targetPiece.Color != pending.OwnerColor || targetPiece.Type == "king" {
				return nil, errors.New("swapus requires your own non-king target")
			}
			if targetPiece.Frozen {
				return nil, errors.New("swapus cannot target a frozen piece")
			}
			pending.Target = intent.Target
			state.UpdatedAt = now.UTC()
			return []contracts.ResolvedEvent{
				makeEvent(state.MatchID, "target_selected", now, intent.PlayerID, map[string]any{
					"cardId":   pending.CardID,
					"mechanic": pending.Mechanic,
					"target":   intent.Target,
				}),
			}, nil
		}
		if intent.Target == nil {
			return nil, errors.New("swapus requires selecting an enemy piece")
		}
		firstPiece := pieceAt(state.Board, *pending.Target)
		secondPiece := pieceAt(state.Board, *intent.Target)
		if firstPiece == nil {
			return nil, errors.New("swapus first piece no longer exists")
		}
		if firstPiece.Frozen {
			return nil, errors.New("swapus cannot move a frozen piece")
		}
		if secondPiece == nil || secondPiece.Color == pending.OwnerColor || secondPiece.Type == "king" {
			return nil, errors.New("swapus requires an enemy non-king second piece")
		}
		if secondPiece.Frozen {
			return nil, errors.New("swapus cannot target a frozen piece")
		}
		nextBoard := cloneBoard(state.Board)
		nextBoard[pending.Target.Row][pending.Target.Col], nextBoard[intent.Target.Row][intent.Target.Col] = nextBoard[intent.Target.Row][intent.Target.Col], nextBoard[pending.Target.Row][pending.Target.Col]
		if !kingsRemainSafe(nextBoard) {
			return nil, errors.New("swapus would create check")
		}
		state.Board = nextBoard
		invalidateCastlingRightsForSquare(state, *pending.Target)
		invalidateCastlingRightsForSquare(state, *intent.Target)
		replaceLastHistorySnapshot(state)
	case "swaphim":
		if pending.Target == nil {
			if intent.Target == nil {
				return nil, errors.New("swaphim requires selecting the first enemy piece")
			}
			targetPiece := pieceAt(state.Board, *intent.Target)
			if targetPiece == nil || targetPiece.Color == pending.OwnerColor || targetPiece.Type == "king" {
				return nil, errors.New("swaphim requires an enemy non-king target")
			}
			if targetPiece.Frozen {
				return nil, errors.New("swaphim cannot target a frozen piece")
			}
			pending.Target = intent.Target
			state.UpdatedAt = now.UTC()
			return []contracts.ResolvedEvent{
				makeEvent(state.MatchID, "target_selected", now, intent.PlayerID, map[string]any{
					"cardId":   pending.CardID,
					"mechanic": pending.Mechanic,
					"target":   intent.Target,
				}),
			}, nil
		}
		if intent.Target == nil {
			return nil, errors.New("swaphim requires selecting the second enemy piece")
		}
		firstPiece := pieceAt(state.Board, *pending.Target)
		secondPiece := pieceAt(state.Board, *intent.Target)
		if firstPiece == nil {
			return nil, errors.New("swaphim first piece no longer exists")
		}
		if secondPiece == nil || secondPiece.Color == pending.OwnerColor || secondPiece.Type == "king" {
			return nil, errors.New("swaphim requires an enemy non-king second piece")
		}
		if secondPiece.Frozen {
			return nil, errors.New("swaphim cannot target a frozen piece")
		}
		if pending.Target.Row == intent.Target.Row && pending.Target.Col == intent.Target.Col {
			return nil, errors.New("swaphim requires two different enemy pieces")
		}
		nextBoard := cloneBoard(state.Board)
		nextBoard[pending.Target.Row][pending.Target.Col], nextBoard[intent.Target.Row][intent.Target.Col] = nextBoard[intent.Target.Row][intent.Target.Col], nextBoard[pending.Target.Row][pending.Target.Col]
		if !kingsRemainSafe(nextBoard) {
			return nil, errors.New("swaphim would create check")
		}
		state.Board = nextBoard
		invalidateCastlingRightsForSquare(state, *pending.Target)
		invalidateCastlingRightsForSquare(state, *intent.Target)
		replaceLastHistorySnapshot(state)
	case "borrow":
		if intent.Target == nil {
			return nil, errors.New("borrow requires a target square")
		}
		targetPiece := pieceAt(state.Board, *intent.Target)
		if targetPiece == nil || targetPiece.Color == pending.OwnerColor || targetPiece.Type == "king" {
			return nil, errors.New("borrow requires an enemy non-king target")
		}
		if targetPiece.Frozen {
			return nil, errors.New("borrow cannot target a frozen piece")
		}
		nextBoard := cloneBoard(state.Board)
		nextTarget := nextBoard[intent.Target.Row][intent.Target.Col]
		nextTarget.Color = pending.OwnerColor
		nextTarget.Borrowed = true
		if !kingsRemainSafe(nextBoard) {
			return nil, errors.New("borrow target is not safe")
		}
		state.Board = nextBoard
		replaceLastHistorySnapshot(state)
	case "mindcontrol":
		if intent.Target == nil {
			return nil, errors.New("mindcontrol requires a target square")
		}
		targetPiece := pieceAt(state.Board, *intent.Target)
		if targetPiece == nil || targetPiece.Color == pending.OwnerColor || targetPiece.Type == "king" {
			return nil, errors.New("mindcontrol requires an enemy non-king target")
		}
		if targetPiece.Frozen {
			return nil, errors.New("mindcontrol cannot target a frozen piece")
		}
		nextBoard := cloneBoard(state.Board)
		nextTarget := nextBoard[intent.Target.Row][intent.Target.Col]
		nextTarget.Color = pending.OwnerColor
		nextTarget.Borrowed = false
		if !kingsRemainSafe(nextBoard) {
			return nil, errors.New("mindcontrol target is not safe")
		}
		state.Board = nextBoard
		replaceLastHistorySnapshot(state)
	case "parasite":
		if pending.Target == nil {
			if intent.Target == nil {
				return nil, errors.New("parasite requires selecting your host piece first")
			}
			targetPiece := pieceAt(state.Board, *intent.Target)
			if targetPiece == nil || targetPiece.Color != pending.OwnerColor || targetPiece.Type == "king" {
				return nil, errors.New("parasite requires your own non-king host")
			}
			pending.Target = intent.Target
			pending.Options = []string{strconv.Itoa(pieceValue(targetPiece.Type))}
			state.UpdatedAt = now.UTC()
			return []contracts.ResolvedEvent{
				makeEvent(state.MatchID, "target_selected", now, intent.PlayerID, map[string]any{
					"cardId":   pending.CardID,
					"mechanic": pending.Mechanic,
					"target":   intent.Target,
					"options":  pending.Options,
				}),
			}, nil
		}
		if intent.Target == nil {
			return nil, errors.New("parasite requires selecting an enemy target")
		}
		targetPiece := pieceAt(state.Board, *intent.Target)
		if targetPiece == nil || targetPiece.Color == pending.OwnerColor || targetPiece.Type == "king" {
			return nil, errors.New("parasite requires an enemy non-king target")
		}
		hostPiece := pieceAt(state.Board, *pending.Target)
		if hostPiece == nil {
			return nil, errors.New("parasite host no longer exists")
		}
		if len(pending.Options) == 0 {
			return nil, errors.New("parasite host value is missing")
		}
		hostValue, err := strconv.Atoi(pending.Options[0])
		if err != nil {
			return nil, errors.New("parasite host value is invalid")
		}
		if pieceValue(targetPiece.Type) != hostValue {
			return nil, fmt.Errorf("parasite requires an enemy piece with the same value (%d)", hostValue)
		}
		nextBoard := cloneBoard(state.Board)
		nextBoard[pending.Target.Row][pending.Target.Col].ParasiteTarget = fmt.Sprintf("%d,%d", intent.Target.Row, intent.Target.Col)
		state.Board = nextBoard
		replaceLastHistorySnapshot(state)
	case "clone":
		if pending.Target == nil {
			if intent.Target == nil {
				return nil, errors.New("clone requires selecting your piece first")
			}
			targetPiece := pieceAt(state.Board, *intent.Target)
			if targetPiece == nil || targetPiece.Color != pending.OwnerColor || targetPiece.Type == "king" {
				return nil, errors.New("clone requires your own non-king target")
			}
			if targetPiece.Frozen {
				return nil, errors.New("clone cannot target a frozen piece")
			}
			pending.Target = intent.Target
			state.UpdatedAt = now.UTC()
			return []contracts.ResolvedEvent{
				makeEvent(state.MatchID, "target_selected", now, intent.PlayerID, map[string]any{
					"cardId":   pending.CardID,
					"mechanic": pending.Mechanic,
					"target":   intent.Target,
				}),
			}, nil
		}
		if intent.Target == nil {
			return nil, errors.New("clone requires choosing a destination square")
		}
		if !inBounds(intent.Target.Row, intent.Target.Col) {
			return nil, errors.New("clone destination is out of bounds")
		}
		sourcePiece := pieceAt(state.Board, *pending.Target)
		if sourcePiece == nil {
			return nil, errors.New("clone source piece no longer exists")
		}
		if sourcePiece.Frozen {
			return nil, errors.New("clone cannot copy a frozen piece")
		}
		if pieceAt(state.Board, *intent.Target) != nil {
			return nil, errors.New("clone destination must be empty")
		}
		if fortressEntryBlocked(state.FortressZones, pending.OwnerColor, *intent.Target) {
			return nil, errors.New("clone destination is protected by an enemy fortress")
		}
		if abs(intent.Target.Row-pending.Target.Row) > 1 || abs(intent.Target.Col-pending.Target.Col) > 1 || (intent.Target.Row == pending.Target.Row && intent.Target.Col == pending.Target.Col) {
			return nil, errors.New("clone destination must be adjacent")
		}
		nextBoard := cloneBoard(state.Board)
		nextBoard[intent.Target.Row][intent.Target.Col] = &contracts.Piece{
			Type:           sourcePiece.Type,
			Color:          sourcePiece.Color,
			Shielded:       sourcePiece.Shielded,
			ShieldTurn:     sourcePiece.ShieldTurn,
			Frozen:         sourcePiece.Frozen,
			Borrowed:       sourcePiece.Borrowed,
			ParasiteTarget: sourcePiece.ParasiteTarget,
			Bomb:           sourcePiece.Bomb,
			Invisible:      sourcePiece.Invisible,
			InvisibleTurn:  sourcePiece.InvisibleTurn,
			InvisibleOver:  sourcePiece.InvisibleOver,
			FusedWith:      sourcePiece.FusedWith,
		}
		if !kingsRemainSafe(nextBoard) {
			return nil, errors.New("clone destination is not safe")
		}
		state.Board = nextBoard
		replaceLastHistorySnapshot(state)
	case "fakepiece":
		if intent.Target == nil {
			return nil, errors.New("fakepiece requires a target square")
		}
		if !inBounds(intent.Target.Row, intent.Target.Col) {
			return nil, errors.New("fakepiece target is out of bounds")
		}
		if pieceAt(state.Board, *intent.Target) != nil {
			return nil, errors.New("fakepiece must target an empty square")
		}
		if fortressEntryBlocked(state.FortressZones, pending.OwnerColor, *intent.Target) {
			return nil, errors.New("fake piece cannot be placed inside an enemy fortress")
		}
		nextBoard := cloneBoard(state.Board)
		nextBoard[intent.Target.Row][intent.Target.Col] = &contracts.Piece{
			Type:  "pawn",
			Color: pending.OwnerColor,
			Fake:  true,
		}
		king := findKing(nextBoard, pending.OwnerColor)
		if king != nil && isAttackedWithFusion(nextBoard, *king, opposite(pending.OwnerColor)) {
			return nil, errors.New("placing fakepiece there would expose your king")
		}
		state.Board = nextBoard
		replaceLastHistorySnapshot(state)
	case "blackhole":
		if pending.Target == nil {
			if intent.Target == nil {
				return nil, errors.New("blackhole requires the first target square")
			}
			if intent.Target.Row < 0 || intent.Target.Row > 7 || intent.Target.Col < 0 || intent.Target.Col > 7 {
				return nil, errors.New("blackhole target out of bounds")
			}
			pending.Target = intent.Target
			state.UpdatedAt = now.UTC()
			return []contracts.ResolvedEvent{
				makeEvent(state.MatchID, "target_selected", now, intent.PlayerID, map[string]any{
					"cardId":   pending.CardID,
					"mechanic": pending.Mechanic,
					"target":   intent.Target,
				}),
			}, nil
		}
		if intent.Target == nil {
			return nil, errors.New("blackhole requires the second target square")
		}
		if intent.Target.Row < 0 || intent.Target.Row > 7 || intent.Target.Col < 0 || intent.Target.Col > 7 {
			return nil, errors.New("blackhole target out of bounds")
		}
		if pending.Target.Row == intent.Target.Row && pending.Target.Col == intent.Target.Col {
			return nil, errors.New("blackhole requires two different target squares")
		}
		state.BlackHoles = append(state.BlackHoles, contracts.BlackHoleZone{
			Sq1:        *pending.Target,
			Sq2:        *intent.Target,
			TurnsLeft:  2,
			OwnerColor: pending.OwnerColor,
		})
		replaceLastHistorySnapshot(state)
	case "smallsacrifice", "bigsacrifice":
		goal := 6
		rewardCount := 2
		if pending.Mechanic == "bigsacrifice" {
			goal = 14
			rewardCount = 3
		}
		selected := parseSquareOptions(pending.Options)
		if intent.Target == nil {
			return nil, errors.New("sacrifice requires a target square")
		}
		targetPiece := pieceAt(state.Board, *intent.Target)
		if targetPiece == nil {
			totalValue := selectedSquaresValue(state.Board, selected)
			if totalValue < goal {
				return nil, fmt.Errorf("sacrifice requires at least %d points", goal)
			}
			nextBoard := cloneBoard(state.Board)
			for _, sq := range selected {
				nextBoard[sq.Row][sq.Col] = nil
			}
			king := findKing(nextBoard, pending.OwnerColor)
			if king != nil && isAttackedWithFusion(nextBoard, *king, opposite(pending.OwnerColor)) {
				return nil, errors.New("cannot sacrifice because it would leave your king in check")
			}
			state.Board = nextBoard
			drawn := addRewardCards(state, pending.OwnerColor, rewardCount, now)
			replaceLastHistorySnapshot(state)
			removeCardFromHand(state, pending.OwnerColor, pending.CardID)
			state.PendingCard = nil
			state.DrawOfferedBy = ""
			state.UpdatedAt = now.UTC()
			return []contracts.ResolvedEvent{
				makeEvent(state.MatchID, "target_selected", now, intent.PlayerID, map[string]any{
					"cardId":     pending.CardID,
					"mechanic":   pending.Mechanic,
					"target":     intent.Target,
					"selected":   selected,
					"totalValue": totalValue,
					"drawnCards": drawn,
				}),
			}, nil
		}
		if targetPiece.Color != pending.OwnerColor || targetPiece.Type == "king" {
			return nil, errors.New("sacrifice requires your own non-king pieces")
		}
		updated := toggleSquareInList(selected, *intent.Target)
		pending.Options = encodeSquareOptions(updated)
		state.UpdatedAt = now.UTC()
		return []contracts.ResolvedEvent{
			makeEvent(state.MatchID, "target_selected", now, intent.PlayerID, map[string]any{
				"cardId":     pending.CardID,
				"mechanic":   pending.Mechanic,
				"target":     intent.Target,
				"selected":   updated,
				"totalValue": selectedSquaresValue(state.Board, updated),
			}),
		}, nil
	case "lavaground":
		if intent.Target == nil {
			return nil, errors.New("lavaground requires a target square")
		}
		if pieceAt(state.Board, *intent.Target) != nil {
			return nil, errors.New("lavaground must target an empty square")
		}
		for _, lava := range state.LavaSquares {
			if lava.Row == intent.Target.Row && lava.Col == intent.Target.Col {
				return nil, errors.New("lavaground already exists on that square")
			}
		}
		state.LavaSquares = append(state.LavaSquares, contracts.LavaSquare{
			Row:       intent.Target.Row,
			Col:       intent.Target.Col,
			MovesLeft: 2,
		})
		replaceLastHistorySnapshot(state)
	case "fog_village":
		if intent.Target == nil {
			return nil, errors.New("fog_village requires a target square")
		}
		centerRow := intent.Target.Row
		centerCol := intent.Target.Col
		if centerRow < 1 {
			centerRow = 1
		} else if centerRow > 6 {
			centerRow = 6
		}
		if centerCol < 1 {
			centerCol = 1
		} else if centerCol > 6 {
			centerCol = 6
		}
		nextFog := make([]contracts.FogZone, 0, len(state.FogZones)+1)
		for _, zone := range state.FogZones {
			if zone.OwnerColor != pending.OwnerColor {
				nextFog = append(nextFog, zone)
			}
		}
		nextFog = append(nextFog, contracts.FogZone{
			CenterRow:  centerRow,
			CenterCol:  centerCol,
			TurnsLeft:  2,
			OwnerColor: pending.OwnerColor,
		})
		state.FogZones = nextFog
		replaceLastHistorySnapshot(state)
	case "fortress":
		if intent.Target == nil {
			return nil, errors.New("fortress requires a target square")
		}
		topRow := clampInt(intent.Target.Row, 0, 6)
		leftCol := clampInt(intent.Target.Col, 0, 6)
		nextFortress := make([]contracts.FortressZone, 0, len(state.FortressZones)+1)
		for _, zone := range state.FortressZones {
			if zone.OwnerColor != pending.OwnerColor {
				nextFortress = append(nextFortress, zone)
			}
		}
		nextFortress = append(nextFortress, contracts.FortressZone{
			TopRow:     topRow,
			LeftCol:    leftCol,
			TurnsLeft:  2,
			OwnerColor: pending.OwnerColor,
		})
		state.FortressZones = nextFortress
		replaceLastHistorySnapshot(state)
	case "invisible":
		if intent.Target == nil {
			return nil, errors.New("invisible requires a target square")
		}
		targetPiece := pieceAt(state.Board, *intent.Target)
		if targetPiece == nil || targetPiece.Color != pending.OwnerColor || targetPiece.Type == "king" {
			return nil, errors.New("invisible requires your own non-king target")
		}
		nextBoard := cloneBoard(state.Board)
		nextPiece := nextBoard[intent.Target.Row][intent.Target.Col]
		nextBoard[intent.Target.Row][intent.Target.Col] = nil
		state.Board = nextBoard
		replaceLastHistorySnapshot(state)
		state.InvisiblePiece = &contracts.InvisiblePieceState{
			Row:        intent.Target.Row,
			Col:        intent.Target.Col,
			Piece:      *nextPiece,
			OwnerColor: pending.OwnerColor,
			RoundsLeft: 1,
		}
	case "unabomber":
		if intent.Target == nil {
			return nil, errors.New("unabomber requires a target square")
		}
		targetPiece := pieceAt(state.Board, *intent.Target)
		if targetPiece == nil || targetPiece.Color != pending.OwnerColor || targetPiece.Type == "king" {
			return nil, errors.New("unabomber requires your own non-king target")
		}
		nextBoard := cloneBoard(state.Board)
		nextBoard[intent.Target.Row][intent.Target.Col].Bomb = true
		state.Board = nextBoard
		replaceLastHistorySnapshot(state)
		state.BombPieces = append(state.BombPieces, contracts.BombPiece{
			Row:        intent.Target.Row,
			Col:        intent.Target.Col,
			TurnsLeft:  2,
			OwnerColor: pending.OwnerColor,
		})
	case "halffuse":
		const halfFuseCap = 6
		if pending.Target == nil {
			if intent.Target == nil {
				return nil, errors.New("halffuse requires selecting your first piece")
			}
			targetPiece := pieceAt(state.Board, *intent.Target)
			if targetPiece == nil || targetPiece.Color != pending.OwnerColor || targetPiece.Type == "king" {
				return nil, errors.New("halffuse requires your own non-king piece")
			}
			if targetPiece.FusedWith != "" {
				return nil, errors.New("halffuse cannot target an already fused piece")
			}
			value := pieceValue(targetPiece.Type)
			if value >= halfFuseCap {
				return nil, errors.New("halffuse first piece is too expensive")
			}
			pending.Target = intent.Target
			pending.Options = []string{targetPiece.Type, strconv.Itoa(value)}
			state.UpdatedAt = now.UTC()
			return []contracts.ResolvedEvent{
				makeEvent(state.MatchID, "target_selected", now, intent.PlayerID, map[string]any{
					"cardId":   pending.CardID,
					"mechanic": pending.Mechanic,
					"target":   intent.Target,
				}),
			}, nil
		}
		if intent.Target == nil {
			return nil, errors.New("halffuse requires selecting the second piece")
		}
		if pending.Target.Row == intent.Target.Row && pending.Target.Col == intent.Target.Col {
			return nil, errors.New("halffuse requires a different second piece")
		}
		if abs(intent.Target.Row-pending.Target.Row) > 1 || abs(intent.Target.Col-pending.Target.Col) > 1 {
			return nil, errors.New("halffuse requires adjacent pieces")
		}
		if len(pending.Options) < 2 {
			return nil, errors.New("halffuse first piece metadata is missing")
		}
		firstType := pending.Options[0]
		firstValue, err := strconv.Atoi(pending.Options[1])
		if err != nil {
			return nil, errors.New("halffuse first piece value is invalid")
		}
		secondPiece := pieceAt(state.Board, *intent.Target)
		if secondPiece == nil || secondPiece.Color != pending.OwnerColor || secondPiece.Type == "king" {
			return nil, errors.New("halffuse requires your own non-king second piece")
		}
		if secondPiece.FusedWith != "" {
			return nil, errors.New("halffuse cannot target an already fused second piece")
		}
		isBishopRook := (firstType == "bishop" && secondPiece.Type == "rook") || (firstType == "rook" && secondPiece.Type == "bishop")
		if !isBishopRook && firstValue+pieceValue(secondPiece.Type) > halfFuseCap {
			return nil, errors.New("halffuse combined value exceeds the cap")
		}
		if redundancy := fusionRedundancy(firstType, secondPiece.Type, *pending.Target, *intent.Target); redundancy != "" {
			return nil, errors.New(redundancy)
		}
		nextBoard := cloneBoard(state.Board)
		nextBoard[pending.Target.Row][pending.Target.Col] = nil
		targetOnNext := nextBoard[intent.Target.Row][intent.Target.Col]
		if targetOnNext == nil {
			return nil, errors.New("halffuse second piece no longer exists")
		}
		if isBishopRook {
			targetOnNext.Type = "queen"
			targetOnNext.FusedWith = ""
		} else {
			targetOnNext.FusedWith = firstType
		}
		if !kingsRemainSafeWithFusion(nextBoard) {
			return nil, errors.New("halffuse would leave a king in check")
		}
		state.Board = nextBoard
		replaceLastHistorySnapshot(state)
	case "fullfusion":
		if pending.Target == nil {
			if intent.Target == nil {
				return nil, errors.New("fullfusion requires selecting your first piece")
			}
			targetPiece := pieceAt(state.Board, *intent.Target)
			if targetPiece == nil || targetPiece.Color != pending.OwnerColor || targetPiece.Type == "king" {
				return nil, errors.New("fullfusion requires your own non-king piece")
			}
			if targetPiece.FusedWith != "" {
				return nil, errors.New("fullfusion cannot target an already fused piece")
			}
			pending.Target = intent.Target
			pending.Options = []string{targetPiece.Type}
			state.UpdatedAt = now.UTC()
			return []contracts.ResolvedEvent{
				makeEvent(state.MatchID, "target_selected", now, intent.PlayerID, map[string]any{
					"cardId":   pending.CardID,
					"mechanic": pending.Mechanic,
					"target":   intent.Target,
				}),
			}, nil
		}
		if intent.Target == nil {
			return nil, errors.New("fullfusion requires selecting the second piece")
		}
		if pending.Target.Row == intent.Target.Row && pending.Target.Col == intent.Target.Col {
			return nil, errors.New("fullfusion requires a different second piece")
		}
		if abs(intent.Target.Row-pending.Target.Row) > 1 || abs(intent.Target.Col-pending.Target.Col) > 1 {
			return nil, errors.New("fullfusion requires adjacent pieces")
		}
		if len(pending.Options) < 1 {
			return nil, errors.New("fullfusion first piece metadata is missing")
		}
		firstType := pending.Options[0]
		secondPiece := pieceAt(state.Board, *intent.Target)
		if secondPiece == nil || secondPiece.Color != pending.OwnerColor || secondPiece.Type == "king" {
			return nil, errors.New("fullfusion requires your own non-king second piece")
		}
		if secondPiece.FusedWith != "" {
			return nil, errors.New("fullfusion cannot target an already fused second piece")
		}
		if redundancy := fusionRedundancy(firstType, secondPiece.Type, *pending.Target, *intent.Target); redundancy != "" {
			return nil, errors.New(redundancy)
		}
		nextBoard := cloneBoard(state.Board)
		nextBoard[pending.Target.Row][pending.Target.Col] = nil
		targetOnNext := nextBoard[intent.Target.Row][intent.Target.Col]
		if targetOnNext == nil {
			return nil, errors.New("fullfusion second piece no longer exists")
		}
		isBishopRook := (firstType == "bishop" && secondPiece.Type == "rook") || (firstType == "rook" && secondPiece.Type == "bishop")
		if isBishopRook {
			targetOnNext.Type = "queen"
			targetOnNext.FusedWith = ""
		} else {
			targetOnNext.FusedWith = firstType
		}
		if !kingsRemainSafeWithFusion(nextBoard) {
			return nil, errors.New("fullfusion would leave a king in check")
		}
		state.Board = nextBoard
		replaceLastHistorySnapshot(state)
	case "joker":
		if intent.SelectionID == "" {
			return nil, errors.New("joker transform requires a selectionId")
		}
		if !containsString(pending.Options, intent.SelectionID) {
			return nil, errors.New("selected joker transform is not allowed")
		}
		template, found := starterCardTemplate(intent.SelectionID)
		if !found {
			return nil, errors.New("selected joker transform template was not found")
		}
		removeCardFromHand(state, pending.OwnerColor, pending.CardID)
		template.ID = fmt.Sprintf("joker_%s_%s_%d", template.Mechanic, pending.OwnerColor, now.UnixMilli())
		addCardToHand(state, pending.OwnerColor, template)
		state.PendingCard = nil
		state.DrawOfferedBy = ""
		state.UpdatedAt = now.UTC()
		return []contracts.ResolvedEvent{
			makeEvent(state.MatchID, "target_selected", now, intent.PlayerID, map[string]any{
				"cardId":      pending.CardID,
				"mechanic":    pending.Mechanic,
				"selectionId": intent.SelectionID,
				"newCard":     template,
			}),
		}, nil
	default:
		return nil, fmt.Errorf("unsupported pending mechanic: %s", pending.Mechanic)
	}

	targetPayload := any(nil)
	if intent.Target != nil {
		targetPayload = intent.Target
	} else if pending.Target != nil {
		targetPayload = pending.Target
	}

	removeCardFromHand(state, pending.OwnerColor, pending.CardID)
	state.PendingCard = nil
	state.DrawOfferedBy = ""
	state.UpdatedAt = now.UTC()

	payload := map[string]any{
		"cardId":   pending.CardID,
		"mechanic": pending.Mechanic,
		"target":   targetPayload,
	}
	if intent.SelectionID != "" {
		payload["selectionId"] = intent.SelectionID
	}

	return []contracts.ResolvedEvent{
		makeEvent(state.MatchID, "target_selected", now, intent.PlayerID, payload),
	}, nil
}

func applyMove(state *contracts.MatchState, intent contracts.PlayerIntent, now time.Time) ([]contracts.ResolvedEvent, error) {
	if err := ensureActive(state); err != nil {
		return nil, err
	}
	if intent.From == nil || intent.To == nil {
		return nil, errors.New("move intent requires from and to")
	}
	actorColor, err := requireIntentColor(state, intent.PlayerID, intent.PlayerSecret)
	if err != nil {
		return nil, err
	}
	if actorColor != state.Turn {
		return nil, errors.New("cannot move out of turn")
	}

	if state.InvisiblePiece != nil &&
		state.InvisiblePiece.OwnerColor == state.Turn &&
		state.InvisiblePiece.Row == intent.From.Row &&
		state.InvisiblePiece.Col == intent.From.Col {
		return applyInvisibleMove(state, intent, now)
	}

	piece := pieceAt(state.Board, *intent.From)
	if piece == nil {
		return nil, errors.New("no piece on source square")
	}
	if piece.Color != actorColor {
		return nil, errors.New("cannot move opponent piece")
	}
	if piece.Frozen {
		return nil, errors.New("frozen pieces cannot move")
	}
	if state.DoubleMove != nil && state.DoubleMove.MovesLeft == 1 && state.DoubleMove.TrackedSq != nil {
		tracked := state.DoubleMove.TrackedSq
		if state.DoubleMove.Type == "same" && (intent.From.Row != tracked.Row || intent.From.Col != tracked.Col) {
			return nil, errors.New("solo double move requires moving the same piece again")
		}
		if state.DoubleMove.Type == "diff" && intent.From.Row == tracked.Row && intent.From.Col == tracked.Col {
			return nil, errors.New("twin double move requires moving a different piece")
		}
	}

	legal := legalMovesWithFusion(state.Board, *intent.From, state.LastMove, sliceToSet(state.Moved))
	if !containsSquare(legal, *intent.To) {
		return nil, errors.New("illegal move")
	}
	if fortressEntryBlocked(state.FortressZones, piece.Color, *intent.To) {
		return nil, errors.New("destination square is protected by an enemy fortress")
	}

	captured := pieceAt(state.Board, *intent.To)
	isEnPassantCapture := piece.Type == "pawn" && intent.From.Col != intent.To.Col && captured == nil
	capturedSquare := *intent.To
	if isEnPassantCapture {
		capturedSquare = contracts.Square{Row: intent.From.Row, Col: intent.To.Col}
		captured = pieceAt(state.Board, capturedSquare)
	}
	if captured != nil && captured.Shielded {
		captured.Shielded = false
		captured.ShieldTurn = nil
		state.DrawOfferedBy = ""
		state.UpdatedAt = now.UTC()
		return []contracts.ResolvedEvent{
			makeEvent(state.MatchID, "target_selected", now, intent.PlayerID, map[string]any{
				"effect": "shield_blocked_capture",
				"target": intent.To,
			}),
		}, nil
	}

	nextBoard := cloneBoard(state.Board)
	nextPiece := pieceAt(nextBoard, *intent.From)
	movePiece(nextBoard, *intent.From, *intent.To, nextPiece, isEnPassantCapture)
	if err := resolveParasiteEffects(nextBoard, *intent.From, *intent.To, capturedSquare, captured); err != nil {
		return nil, err
	}
	updateParasiteLinksForMove(nextBoard, *intent.From, *intent.To)
	promotion := ""
	if nextPiece != nil && nextPiece.Type == "pawn" && !nextPiece.Fake && (intent.To.Row == 0 || intent.To.Row == 7) {
		promoteTo := "queen"
		if intent.Promotion != "" {
			switch intent.Promotion {
			case "queen", "rook", "bishop", "knight":
				promoteTo = intent.Promotion
			}
		}
		nextBoard[intent.To.Row][intent.To.Col].Type = promoteTo
		promotion = promoteTo
	}
	if state.DoubleMove != nil && state.DoubleMove.MovesLeft == 2 {
		oppKing := findKing(nextBoard, opposite(state.Turn))
		if oppKing != nil && isAttackedWithFusion(nextBoard, *oppKing, state.Turn) {
			return nil, errors.New("first double move cannot put enemy king in check")
		}
	}
	state.Board = nextBoard
	lavaTriggered, lavaCapturedPiece := resolveLavaEffects(state, *intent.To)
	updateBombTracker(state, *intent.From, *intent.To)

	captureOccurred := captured != nil
	if lavaTriggered && lavaCapturedPiece != "" {
		captureOccurred = true
	}
	notation := moveNotation(state.Board, *intent.From, *intent.To, piece, captureOccurred)
	state.Moved = append(state.Moved, keyForSquare(*intent.From))
	state.LastMove = &contracts.LastMove{From: *intent.From, To: *intent.To}
	state.DrawOfferedBy = ""
	state.HalfMoveClock = nextHalfMoveClock(state.HalfMoveClock, piece.Type, captureOccurred)
	if state.DoubleMove != nil {
		newMovesLeft := state.DoubleMove.MovesLeft - 1
		if newMovesLeft > 0 {
			tracked := &contracts.Square{Row: intent.To.Row, Col: intent.To.Col}
			state.DoubleMove = &contracts.DoubleMoveState{
				Type:      state.DoubleMove.Type,
				MovesLeft: newMovesLeft,
				TrackedSq: tracked,
				FirstNote: notation,
			}
			startedAt := now.UnixMilli()
			state.Clock.RunningFor = state.Turn
			state.Clock.StartedAt = &startedAt
			state.UpdatedAt = now.UTC()

			payload := map[string]any{
				"from":       intent.From,
				"to":         intent.To,
				"notation":   notation,
				"nextTurn":   state.Turn,
				"doubleMove": state.DoubleMove,
			}
			if promotion != "" {
				payload["promotion"] = promotion
			}
			if lavaTriggered {
				payload["lavaTriggered"] = true
				if lavaCapturedPiece != "" {
					payload["lavaCapturedPiece"] = lavaCapturedPiece
				}
			}

			return []contracts.ResolvedEvent{
				makeEvent(state.MatchID, "move_applied", now, intent.PlayerID, payload),
				makeEvent(state.MatchID, "clock_updated", now, intent.PlayerID, map[string]any{
					"runningFor": state.Turn,
				}),
			}, nil
		}

		if state.DoubleMove.FirstNote != "" {
			state.MoveHistory = append(state.MoveHistory, state.DoubleMove.FirstNote+"+"+notation)
		} else {
			state.MoveHistory = append(state.MoveHistory, notation)
		}
		state.DoubleMove = nil
	} else {
		state.MoveHistory = append(state.MoveHistory, notation)
	}

	if state.Turn == "black" {
		state.FullMoveNum++
	}
	justMovedColor := state.Turn
	state.Turn = opposite(state.Turn)
	cleanupTemporaryEffects(state, justMovedColor)
	resolveFogEffects(state, justMovedColor)
	resolveFortressEffects(state, justMovedColor)
	bombExplodedSquares := resolveBombEffects(state)
	blackHoleExplodedSquares := resolveBlackHoleEffects(state, justMovedColor)
	roundDrawWhite, roundDrawBlack := drawRoundCards(state, now)
	startedAt := now.UnixMilli()
	if state.Clock.Increment > 0 {
		if justMovedColor == "white" {
			state.Clock.WhiteMS += state.Clock.Increment
		} else {
			state.Clock.BlackMS += state.Clock.Increment
		}
	}
	state.Clock.RunningFor = state.Turn
	state.Clock.StartedAt = &startedAt
	state.UpdatedAt = now.UTC()
	state.History = append(state.History, capturePositionState(state))

	posKey := positionKey(state.Board, state.Turn, sliceToSet(state.Moved), state.LastMove)
	repCount := 1
	for _, pos := range state.History {
		if positionKey(pos.Board, pos.Turn, sliceToSet(pos.Moved), pos.LastMove) == posKey {
			repCount++
		}
	}
	if repCount >= 3 {
		markMatchFinished(state, "draw", "threefold_repetition", now)
		return []contracts.ResolvedEvent{
			makeEvent(state.MatchID, "move_applied", now, intent.PlayerID, map[string]any{"notation": notation, "nextTurn": state.Turn}),
			makeEvent(state.MatchID, "match_finished", now, intent.PlayerID, map[string]any{"result": "threefold_repetition", "winner": "draw"}),
		}, nil
	}
	if state.HalfMoveClock >= 100 {
		markMatchFinished(state, "draw", "fifty_move_rule", now)
		return []contracts.ResolvedEvent{
			makeEvent(state.MatchID, "move_applied", now, intent.PlayerID, map[string]any{"notation": notation, "nextTurn": state.Turn}),
			makeEvent(state.MatchID, "match_finished", now, intent.PlayerID, map[string]any{"result": "fifty_move_rule", "winner": "draw"}),
		}, nil
	}
	if insufficientMaterial(state.Board) {
		markMatchFinished(state, "draw", "insufficient_material", now)
		return []contracts.ResolvedEvent{
			makeEvent(state.MatchID, "move_applied", now, intent.PlayerID, map[string]any{"notation": notation, "nextTurn": state.Turn}),
			makeEvent(state.MatchID, "match_finished", now, intent.PlayerID, map[string]any{"result": "insufficient_material", "winner": "draw"}),
		}, nil
	}

	payload := map[string]any{
		"from":     intent.From,
		"to":       intent.To,
		"notation": notation,
		"nextTurn": state.Turn,
	}
	if promotion != "" {
		payload["promotion"] = promotion
	}
	if lavaTriggered {
		payload["lavaTriggered"] = true
		if lavaCapturedPiece != "" {
			payload["lavaCapturedPiece"] = lavaCapturedPiece
		}
	}
	if len(bombExplodedSquares) > 0 {
		payload["bombExplodedSquares"] = bombExplodedSquares
	}
	if len(blackHoleExplodedSquares) > 0 {
		payload["blackHoleExplodedSquares"] = blackHoleExplodedSquares
	}
	if len(roundDrawWhite) > 0 {
		payload["roundDrawWhite"] = roundDrawWhite
	}
	if len(roundDrawBlack) > 0 {
		payload["roundDrawBlack"] = roundDrawBlack
	}

	return []contracts.ResolvedEvent{
		makeEvent(state.MatchID, "move_applied", now, intent.PlayerID, payload),
		makeEvent(state.MatchID, "clock_updated", now, intent.PlayerID, map[string]any{
			"runningFor": state.Turn,
		}),
	}, nil
}

func applyInvisibleMove(state *contracts.MatchState, intent contracts.PlayerIntent, now time.Time) ([]contracts.ResolvedEvent, error) {
	invisible := state.InvisiblePiece
	if invisible == nil {
		return nil, errors.New("no invisible piece to move")
	}

	ghostBoard := cloneBoard(state.Board)
	ghostBoard[intent.From.Row][intent.From.Col] = &contracts.Piece{
		Type:           invisible.Piece.Type,
		Color:          invisible.Piece.Color,
		Shielded:       invisible.Piece.Shielded,
		ShieldTurn:     invisible.Piece.ShieldTurn,
		Frozen:         invisible.Piece.Frozen,
		Borrowed:       invisible.Piece.Borrowed,
		ParasiteTarget: invisible.Piece.ParasiteTarget,
		Bomb:           invisible.Piece.Bomb,
		Invisible:      invisible.Piece.Invisible,
		InvisibleTurn:  invisible.Piece.InvisibleTurn,
		InvisibleOver:  invisible.Piece.InvisibleOver,
		FusedWith:      invisible.Piece.FusedWith,
	}

	legal := legalMoves(ghostBoard, *intent.From, state.LastMove, sliceToSet(state.Moved))
	if !containsSquare(legal, *intent.To) {
		return nil, errors.New("illegal move")
	}
	if fortressEntryBlocked(state.FortressZones, invisible.Piece.Color, *intent.To) {
		return nil, errors.New("destination square is protected by an enemy fortress")
	}

	targetPiece := pieceAt(state.Board, *intent.To)
	givesCheckBoard := cloneBoard(state.Board)
	givesCheckBoard[intent.To.Row][intent.To.Col] = &contracts.Piece{
		Type:           invisible.Piece.Type,
		Color:          invisible.Piece.Color,
		Shielded:       invisible.Piece.Shielded,
		ShieldTurn:     invisible.Piece.ShieldTurn,
		Frozen:         invisible.Piece.Frozen,
		Borrowed:       invisible.Piece.Borrowed,
		ParasiteTarget: invisible.Piece.ParasiteTarget,
		Bomb:           invisible.Piece.Bomb,
		Invisible:      invisible.Piece.Invisible,
		InvisibleTurn:  invisible.Piece.InvisibleTurn,
		InvisibleOver:  invisible.Piece.InvisibleOver,
		FusedWith:      invisible.Piece.FusedWith,
	}
	oppKing := findKing(givesCheckBoard, opposite(state.Turn))
	givesCheck := oppKing != nil && isAttacked(givesCheckBoard, *oppKing, state.Turn)
	isCapture := targetPiece != nil && targetPiece.Color != state.Turn
	isMove2 := invisible.RoundsLeft <= 0

	if targetPiece != nil && targetPiece.Shielded {
		targetPiece.Shielded = false
		targetPiece.ShieldTurn = nil
		state.DrawOfferedBy = ""
		state.UpdatedAt = now.UTC()
		return []contracts.ResolvedEvent{
			makeEvent(state.MatchID, "target_selected", now, intent.PlayerID, map[string]any{
				"effect": "shield_blocked_capture",
				"target": intent.To,
			}),
		}, nil
	}

	if givesCheck || isMove2 {
		nextBoard := cloneBoard(state.Board)
		nextBoard[intent.To.Row][intent.To.Col] = &contracts.Piece{
			Type:           invisible.Piece.Type,
			Color:          invisible.Piece.Color,
			Shielded:       invisible.Piece.Shielded,
			ShieldTurn:     invisible.Piece.ShieldTurn,
			Frozen:         invisible.Piece.Frozen,
			Borrowed:       invisible.Piece.Borrowed,
			ParasiteTarget: invisible.Piece.ParasiteTarget,
			Bomb:           invisible.Piece.Bomb,
			Invisible:      invisible.Piece.Invisible,
			InvisibleTurn:  invisible.Piece.InvisibleTurn,
			InvisibleOver:  invisible.Piece.InvisibleOver,
			FusedWith:      invisible.Piece.FusedWith,
		}
		state.Board = nextBoard
		replaceLastHistorySnapshot(state)
		state.InvisiblePiece = nil
	} else {
		state.InvisiblePiece = &contracts.InvisiblePieceState{
			Row:        intent.To.Row,
			Col:        intent.To.Col,
			Piece:      invisible.Piece,
			OwnerColor: invisible.OwnerColor,
			RoundsLeft: invisible.RoundsLeft,
		}
	}
	updateBombTracker(state, *intent.From, *intent.To)

	notation := fmt.Sprintf("%s→%s", squareName(*intent.From), squareName(*intent.To))
	state.Moved = append(state.Moved, keyForSquare(*intent.From))
	state.LastMove = &contracts.LastMove{From: *intent.From, To: *intent.To}
	state.MoveHistory = append(state.MoveHistory, notation)
	state.DrawOfferedBy = ""
	captureOccurred := isCapture && (givesCheck || isMove2)
	state.HalfMoveClock = nextHalfMoveClock(state.HalfMoveClock, invisible.Piece.Type, captureOccurred)
	if state.Turn == "black" {
		state.FullMoveNum++
	}
	justMovedColor := state.Turn
	state.Turn = opposite(state.Turn)
	cleanupTemporaryEffects(state, justMovedColor)
	resolveFogEffects(state, justMovedColor)
	resolveFortressEffects(state, justMovedColor)
	bombExplodedSquares := resolveBombEffects(state)
	blackHoleExplodedSquares := resolveBlackHoleEffects(state, justMovedColor)
	roundDrawWhite, roundDrawBlack := drawRoundCards(state, now)
	startedAt := now.UnixMilli()
	if state.Clock.Increment > 0 {
		if justMovedColor == "white" {
			state.Clock.WhiteMS += state.Clock.Increment
		} else {
			state.Clock.BlackMS += state.Clock.Increment
		}
	}
	state.Clock.RunningFor = state.Turn
	state.Clock.StartedAt = &startedAt
	state.UpdatedAt = now.UTC()
	state.History = append(state.History, capturePositionState(state))

	posKey := positionKey(state.Board, state.Turn, sliceToSet(state.Moved), state.LastMove)
	repCount := 1
	for _, pos := range state.History {
		if positionKey(pos.Board, pos.Turn, sliceToSet(pos.Moved), pos.LastMove) == posKey {
			repCount++
		}
	}
	if repCount >= 3 {
		markMatchFinished(state, "draw", "threefold_repetition", now)
		return []contracts.ResolvedEvent{
			makeEvent(state.MatchID, "move_applied", now, intent.PlayerID, map[string]any{"notation": notation, "nextTurn": state.Turn}),
			makeEvent(state.MatchID, "match_finished", now, intent.PlayerID, map[string]any{"result": "threefold_repetition", "winner": "draw"}),
		}, nil
	}
	if state.HalfMoveClock >= 100 {
		markMatchFinished(state, "draw", "fifty_move_rule", now)
		return []contracts.ResolvedEvent{
			makeEvent(state.MatchID, "move_applied", now, intent.PlayerID, map[string]any{"notation": notation, "nextTurn": state.Turn}),
			makeEvent(state.MatchID, "match_finished", now, intent.PlayerID, map[string]any{"result": "fifty_move_rule", "winner": "draw"}),
		}, nil
	}
	if insufficientMaterial(state.Board) {
		markMatchFinished(state, "draw", "insufficient_material", now)
		return []contracts.ResolvedEvent{
			makeEvent(state.MatchID, "move_applied", now, intent.PlayerID, map[string]any{"notation": notation, "nextTurn": state.Turn}),
			makeEvent(state.MatchID, "match_finished", now, intent.PlayerID, map[string]any{"result": "insufficient_material", "winner": "draw"}),
		}, nil
	}

	payload := map[string]any{
		"from":     intent.From,
		"to":       intent.To,
		"notation": notation,
		"nextTurn": state.Turn,
	}
	if givesCheck {
		payload["materialized"] = "check"
	} else if isMove2 && isCapture {
		payload["materialized"] = "capture"
	}
	if len(bombExplodedSquares) > 0 {
		payload["bombExplodedSquares"] = bombExplodedSquares
	}
	if len(blackHoleExplodedSquares) > 0 {
		payload["blackHoleExplodedSquares"] = blackHoleExplodedSquares
	}
	if len(roundDrawWhite) > 0 {
		payload["roundDrawWhite"] = roundDrawWhite
	}
	if len(roundDrawBlack) > 0 {
		payload["roundDrawBlack"] = roundDrawBlack
	}

	return []contracts.ResolvedEvent{
		makeEvent(state.MatchID, "move_applied", now, intent.PlayerID, payload),
		makeEvent(state.MatchID, "clock_updated", now, intent.PlayerID, map[string]any{
			"runningFor": state.Turn,
		}),
	}, nil
}

func applyChat(state *contracts.MatchState, intent contracts.PlayerIntent, now time.Time) ([]contracts.ResolvedEvent, error) {
	text := strings.TrimSpace(intent.Text)
	if text == "" {
		return nil, errors.New("chat text is empty")
	}
	if len(text) > 500 {
		return nil, errors.New("chat message too long (max 500 characters)")
	}

	sender, err := requireIntentColor(state, intent.PlayerID, intent.PlayerSecret)
	if err != nil {
		return nil, err
	}

	for i := len(state.ChatMessages) - 1; i >= 0; i-- {
		msg := state.ChatMessages[i]
		if msg.Sender == sender && now.UTC().Sub(msg.SentAt) < time.Second {
			return nil, errors.New("chat rate limited (one message per second)")
		}
		if msg.Sender != sender && now.UTC().Sub(msg.SentAt) < time.Second {
			break
		}
	}

	state.ChatMessages = append(state.ChatMessages, contracts.ChatMessage{
		Sender: sender,
		Text:   text,
		SentAt: now.UTC(),
	})
	state.UpdatedAt = now.UTC()

	return []contracts.ResolvedEvent{
		makeEvent(state.MatchID, "chat_sent", now, intent.PlayerID, map[string]any{
			"sender": sender,
			"text":   text,
		}),
	}, nil
}

func applyOfferDraw(state *contracts.MatchState, intent contracts.PlayerIntent, now time.Time) ([]contracts.ResolvedEvent, error) {
	if err := ensureActive(state); err != nil {
		return nil, err
	}

	offeredBy, err := requireIntentColor(state, intent.PlayerID, intent.PlayerSecret)
	if err != nil {
		return nil, err
	}

	minInterval := 15 * time.Second
	if !state.DrawOfferTime.IsZero() && now.UTC().Sub(state.DrawOfferTime) < minInterval {
		return nil, errors.New("draw offer rate limited")
	}

	state.DrawOfferedBy = offeredBy
	state.DrawOfferTime = now.UTC()
	state.UpdatedAt = now.UTC()

	return []contracts.ResolvedEvent{
		makeEvent(state.MatchID, "draw_offered", now, intent.PlayerID, map[string]any{
			"offeredBy": offeredBy,
		}),
	}, nil
}

func applyRespondDraw(state *contracts.MatchState, intent contracts.PlayerIntent, now time.Time) ([]contracts.ResolvedEvent, error) {
	if err := ensureActive(state); err != nil {
		return nil, err
	}
	if state.DrawOfferedBy == "" {
		return nil, errors.New("no draw offer to respond to")
	}
	if intent.Accept == nil {
		return nil, errors.New("respond_draw requires accept")
	}
	responder, err := requireIntentColor(state, intent.PlayerID, intent.PlayerSecret)
	if err != nil {
		return nil, err
	}
	if responder == state.DrawOfferedBy {
		return nil, errors.New("draw offer cannot be resolved by the offering side")
	}

	if *intent.Accept {
		markMatchFinished(state, "draw", "draw_agreement", now)
		return []contracts.ResolvedEvent{
			makeEvent(state.MatchID, "draw_resolved", now, intent.PlayerID, map[string]any{"accept": true}),
			makeEvent(state.MatchID, "match_finished", now, intent.PlayerID, map[string]any{"result": "draw_agreement", "winner": "draw"}),
		}, nil
	}

	state.DrawOfferedBy = ""
	state.UpdatedAt = now.UTC()
	return []contracts.ResolvedEvent{
		makeEvent(state.MatchID, "draw_resolved", now, intent.PlayerID, map[string]any{"accept": false}),
	}, nil
}

func applyAbort(state *contracts.MatchState, intent contracts.PlayerIntent, now time.Time) ([]contracts.ResolvedEvent, error) {
	if err := ensureActive(state); err != nil {
		return nil, err
	}
	if _, err := requireIntentColor(state, intent.PlayerID, intent.PlayerSecret); err != nil {
		return nil, err
	}
	if len(state.MoveHistory) > 1 {
		return nil, errors.New("abort is only allowed before black completes the first reply")
	}

	markMatchFinished(state, "aborted", "abort", now)

	return []contracts.ResolvedEvent{
		makeEvent(state.MatchID, "match_finished", now, intent.PlayerID, map[string]any{
			"result": "abort",
			"winner": "aborted",
		}),
	}, nil
}

func applyResign(state *contracts.MatchState, intent contracts.PlayerIntent, now time.Time) ([]contracts.ResolvedEvent, error) {
	if err := ensureActive(state); err != nil {
		return nil, err
	}

	resigningColor, err := requireIntentColor(state, intent.PlayerID, intent.PlayerSecret)
	if err != nil {
		return nil, err
	}
	winner := opposite(resigningColor)
	markMatchFinished(state, winner, "resign", now)

	return []contracts.ResolvedEvent{
		makeEvent(state.MatchID, "match_finished", now, intent.PlayerID, map[string]any{
			"result": "resign",
			"winner": winner,
		}),
	}, nil
}

func ensureActive(state *contracts.MatchState) error {
	if state.Status != "active" {
		return errors.New("match is not active")
	}
	return nil
}

func chooseOpenSeat(state *contracts.MatchState, preferred string) string {
	if state == nil {
		return ""
	}
	isWhiteOpen := strings.TrimSpace(state.WhiteGuestID) == ""
	isBlackOpen := strings.TrimSpace(state.BlackGuestID) == ""
	switch preferred {
	case "white":
		if isWhiteOpen {
			return "white"
		}
		return ""
	case "black":
		if isBlackOpen {
			return "black"
		}
		return ""
	}
	if isWhiteOpen {
		return "white"
	}
	if isBlackOpen {
		return "black"
	}
	return ""
}

func requireIntentColor(state *contracts.MatchState, playerID, playerSecret string) (string, error) {
	value := strings.TrimSpace(playerID)
	resolvedColor := ""
	hasGuestIDs := false
	if state != nil {
		switch {
		case state.WhiteGuestID != "" && strings.EqualFold(value, strings.TrimSpace(state.WhiteGuestID)):
			resolvedColor = "white"
		case state.BlackGuestID != "" && strings.EqualFold(value, strings.TrimSpace(state.BlackGuestID)):
			resolvedColor = "black"
		}
		hasGuestIDs = state.WhiteGuestID != "" || state.BlackGuestID != ""
	}
	if resolvedColor != "" && state != nil {
		requiredSecret := ""
		if resolvedColor == "white" {
			requiredSecret = strings.TrimSpace(state.WhitePlayerSecret)
		} else if resolvedColor == "black" {
			requiredSecret = strings.TrimSpace(state.BlackPlayerSecret)
		}
		if requiredSecret == "" {
			return "", errors.New("match has no player secret configured")
		}
		if subtle.ConstantTimeCompare([]byte(strings.TrimSpace(playerSecret)), []byte(requiredSecret)) != 1 {
			return "", errors.New("unauthorized player secret")
		}
		return resolvedColor, nil
	}
	if state != nil && !hasGuestIDs {
		lowerValue := strings.ToLower(value)
		switch {
		case strings.Contains(lowerValue, "white"):
			resolvedColor = "white"
		case strings.Contains(lowerValue, "black"):
			resolvedColor = "black"
		}
		if resolvedColor != "" {
			requiredSecret := ""
			if resolvedColor == "white" {
				requiredSecret = strings.TrimSpace(state.WhitePlayerSecret)
			} else if resolvedColor == "black" {
				requiredSecret = strings.TrimSpace(state.BlackPlayerSecret)
			}
			if requiredSecret == "" {
				return "", errors.New("match has no player secret configured")
			}
			if subtle.ConstantTimeCompare([]byte(strings.TrimSpace(playerSecret)), []byte(requiredSecret)) != 1 {
				return "", errors.New("unauthorized player secret")
			}
			return resolvedColor, nil
		}
	}
	if state != nil {
		if state.WhitePlayerSecret != "" && subtle.ConstantTimeCompare([]byte(strings.TrimSpace(playerSecret)), []byte(strings.TrimSpace(state.WhitePlayerSecret))) == 1 {
			return "white", nil
		}
		if state.BlackPlayerSecret != "" && subtle.ConstantTimeCompare([]byte(strings.TrimSpace(playerSecret)), []byte(strings.TrimSpace(state.BlackPlayerSecret))) == 1 {
			return "black", nil
		}
	}
	return "", errors.New("unrecognized player id")
}

func opposite(color string) string {
	if color == "white" {
		return "black"
	}
	return "white"
}

func chooseSeed(_ int64, fallback int64) int64 {
	return fallback
}

func makeEvent(matchID, eventType string, now time.Time, actorID string, payload map[string]any) contracts.ResolvedEvent {
	return contracts.ResolvedEvent{
		ID:      fmt.Sprintf("%s_%s_%d", matchID, eventType, now.UnixMilli()),
		MatchID: matchID,
		Type:    eventType,
		ActorID: actorID,
		At:      now.UTC(),
		Payload: payload,
	}
}

func cloneState(state *contracts.MatchState) contracts.MatchState {
	clone := *state
	clone.Board = cloneBoard(state.Board)
	clone.Moved = append([]string{}, state.Moved...)
	clone.MoveHistory = append([]string{}, state.MoveHistory...)
	clone.ChatMessages = append([]contracts.ChatMessage{}, state.ChatMessages...)
	clone.WhiteHand = append([]contracts.GameCard{}, state.WhiteHand...)
	clone.BlackHand = append([]contracts.GameCard{}, state.BlackHand...)
	clone.UndoAgainst = state.UndoAgainst
	clone.LavaSquares = append([]contracts.LavaSquare{}, state.LavaSquares...)
	clone.BombPieces = append([]contracts.BombPiece{}, state.BombPieces...)
	clone.BlackHoles = append([]contracts.BlackHoleZone{}, state.BlackHoles...)
	clone.FogZones = append([]contracts.FogZone{}, state.FogZones...)
	clone.FortressZones = append([]contracts.FortressZone{}, state.FortressZones...)
	clone.History = append([]contracts.PositionState{}, state.History...)
	clone.SeenClientMoveIDs = append([]string{}, state.SeenClientMoveIDs...)
	clone.RadarRevealFor = state.RadarRevealFor
	if state.DoubleMove != nil {
		doubleMove := *state.DoubleMove
		if state.DoubleMove.TrackedSq != nil {
			tracked := *state.DoubleMove.TrackedSq
			doubleMove.TrackedSq = &tracked
		}
		clone.DoubleMove = &doubleMove
	}
	if state.InvisiblePiece != nil {
		invisible := *state.InvisiblePiece
		clone.InvisiblePiece = &invisible
	}
	if state.CheaterState != nil {
		cheater := *state.CheaterState
		clone.CheaterState = &cheater
	}
	if state.LastMove != nil {
		last := *state.LastMove
		clone.LastMove = &last
	}
	if state.PendingCard != nil {
		pending := *state.PendingCard
		clone.PendingCard = &pending
	}
	return clone
}

func capturePositionState(state *contracts.MatchState) contracts.PositionState {
	position := contracts.PositionState{
		Board:          cloneBoard(state.Board),
		LavaSquares:    append([]contracts.LavaSquare{}, state.LavaSquares...),
		BombPieces:     append([]contracts.BombPiece{}, state.BombPieces...),
		BlackHoles:     append([]contracts.BlackHoleZone{}, state.BlackHoles...),
		FogZones:       append([]contracts.FogZone{}, state.FogZones...),
		FortressZones:  append([]contracts.FortressZone{}, state.FortressZones...),
		RadarRevealFor: state.RadarRevealFor,
		Turn:           state.Turn,
		Moved:          append([]string{}, state.Moved...),
		HalfMoveClock:  state.HalfMoveClock,
		FullMoveNum:    state.FullMoveNum,
		MoveHistory:    append([]string{}, state.MoveHistory...),
		WhiteHand:      append([]contracts.GameCard{}, state.WhiteHand...),
		BlackHand:      append([]contracts.GameCard{}, state.BlackHand...),
		UndoAgainst:    state.UndoAgainst,
		DrawOfferedBy:  state.DrawOfferedBy,
	}
	if state.InvisiblePiece != nil {
		invisible := *state.InvisiblePiece
		position.InvisiblePiece = &invisible
	}
	if state.CheaterState != nil {
		cheater := *state.CheaterState
		position.CheaterState = &cheater
	}
	if state.LastMove != nil {
		last := *state.LastMove
		position.LastMove = &last
	}
	if state.PendingCard != nil {
		pending := *state.PendingCard
		if pending.Target != nil {
			t := *pending.Target
			pending.Target = &t
		}
		position.PendingCard = &pending
	}
	if state.DoubleMove != nil {
		dm := *state.DoubleMove
		if state.DoubleMove.TrackedSq != nil {
			tracked := *state.DoubleMove.TrackedSq
			dm.TrackedSq = &tracked
		}
		position.DoubleMove = &dm
	}
	return position
}

func restorePositionState(state *contracts.MatchState, position contracts.PositionState) {
	state.Board = cloneBoard(position.Board)
	replaceLastHistorySnapshot(state)
	state.LavaSquares = append([]contracts.LavaSquare{}, position.LavaSquares...)
	state.BombPieces = append([]contracts.BombPiece{}, position.BombPieces...)
	state.BlackHoles = append([]contracts.BlackHoleZone{}, position.BlackHoles...)
	state.FogZones = append([]contracts.FogZone{}, position.FogZones...)
	state.FortressZones = append([]contracts.FortressZone{}, position.FortressZones...)
	state.RadarRevealFor = position.RadarRevealFor
	state.Moved = append([]string{}, position.Moved...)
	state.HalfMoveClock = position.HalfMoveClock
	state.FullMoveNum = position.FullMoveNum
	state.MoveHistory = append([]string{}, position.MoveHistory...)
	state.WhiteHand = append([]contracts.GameCard{}, position.WhiteHand...)
	state.BlackHand = append([]contracts.GameCard{}, position.BlackHand...)
	state.UndoAgainst = position.UndoAgainst
	state.DrawOfferedBy = position.DrawOfferedBy
	if position.InvisiblePiece != nil {
		invisible := *position.InvisiblePiece
		state.InvisiblePiece = &invisible
	} else {
		state.InvisiblePiece = nil
	}
	if position.CheaterState != nil {
		cheater := *position.CheaterState
		state.CheaterState = &cheater
	} else {
		state.CheaterState = nil
	}
	if position.LastMove != nil {
		last := *position.LastMove
		state.LastMove = &last
	} else {
		state.LastMove = nil
	}
	if position.PendingCard != nil {
		pending := *position.PendingCard
		if pending.Target != nil {
			t := *pending.Target
			pending.Target = &t
		}
		state.PendingCard = &pending
	} else {
		state.PendingCard = nil
	}
	if position.DoubleMove != nil {
		dm := *position.DoubleMove
		if position.DoubleMove.TrackedSq != nil {
			tracked := *position.DoubleMove.TrackedSq
			dm.TrackedSq = &tracked
		}
		state.DoubleMove = &dm
	} else {
		state.DoubleMove = nil
	}
}

func applyMirrorCard(state *contracts.MatchState, owner string) (bool, *contracts.Square, *contracts.Square, error) {
	if state.LastMove == nil {
		return false, nil, nil, nil
	}

	movedPiece := pieceAt(state.Board, state.LastMove.To)
	if movedPiece == nil {
		return false, nil, nil, nil
	}

	dr := state.LastMove.To.Row - state.LastMove.From.Row
	dc := state.LastMove.To.Col - state.LastMove.From.Col
	movedType := movedPiece.Type

	for row := 0; row < 8; row++ {
		for col := 0; col < 8; col++ {
			piece := state.Board[row][col]
			if piece == nil || piece.Color != owner || piece.Type != movedType {
				continue
			}

			from := contracts.Square{Row: row, Col: col}
			to := contracts.Square{Row: row + dr, Col: col + dc}
			if !inBounds(to.Row, to.Col) {
				continue
			}
			if occupant := pieceAt(state.Board, to); occupant != nil && occupant.Color == owner {
				continue
			}

			capturedSquare := to
			captured := pieceAt(state.Board, to)
			if captured != nil && captured.Shielded {
				captured.Shielded = false
				captured.ShieldTurn = nil
				state.DrawOfferedBy = ""
				return true, &from, &to, nil
			}
			nextBoard := cloneBoard(state.Board)
			nextPiece := pieceAt(nextBoard, from)
			if nextPiece == nil {
				continue
			}

			movePiece(nextBoard, from, to, nextPiece, false)
			if err := resolveParasiteEffects(nextBoard, from, to, capturedSquare, captured); err != nil {
				continue
			}
			updateParasiteLinksForMove(nextBoard, from, to)

			king := findKing(nextBoard, owner)
			if king != nil && isAttackedWithFusion(nextBoard, *king, opposite(owner)) {
				continue
			}

			state.Board = nextBoard
			updateBombTracker(state, from, to)
			resolveLavaEffects(state, to)
			replaceLastHistorySnapshot(state)
			return true, &from, &to, nil
		}
	}

	return false, nil, nil, nil
}

func replaceLastHistorySnapshot(state *contracts.MatchState) {
	snapshot := capturePositionState(state)
	if len(state.History) == 0 {
		state.History = []contracts.PositionState{snapshot}
		return
	}
	state.History[len(state.History)-1] = snapshot
}

// invalidateCastlingRightsForSquare marks the given square as "moved" so castling
// rights are revoked if a rook or king was displaced by a card effect (Teleport,
// Jump, Swap, etc.) rather than a normal move.
func invalidateCastlingRightsForSquare(state *contracts.MatchState, sq contracts.Square) {
	key := keyForSquare(sq)
	for _, existing := range state.Moved {
		if existing == key {
			return
		}
	}
	// Only invalidate castling-relevant starting squares:
	// white king e1, white rooks a1/h1, black king e8, black rooks a8/h8.
	switch key {
	case "0-4", "0-0", "0-7", "7-4", "7-0", "7-7":
		state.Moved = append(state.Moved, key)
	}
}

func resolveGamblerCard(state *contracts.MatchState, owner string, now time.Time) map[string]any {
	opponent := opposite(owner)
	var myHand, oppHand []contracts.GameCard
	if owner == "black" {
		myHand = state.BlackHand
		oppHand = state.WhiteHand
	} else {
		myHand = state.WhiteHand
		oppHand = state.BlackHand
	}

	roll := deterministicCardIndex(state, len(myHand)+len(oppHand)+int(now.UnixMilli()%7))
	win := len(oppHand) > 0 && (roll%2 == 0 || len(myHand) <= 1)
	if win && len(oppHand) > 0 {
		stolenIndex := deterministicCardIndex(state, len(oppHand)+1) % len(oppHand)
		stolen := oppHand[stolenIndex]
		removeCardFromHand(state, opponent, stolen.ID)
		addCardToHand(state, owner, stolen)
		return map[string]any{
			"outcome": "win",
			"card":    stolen,
		}
	}

	candidates := filterCardsNotMechanic(myHand, "gambler")
	if len(candidates) > 0 {
		giveIndex := deterministicCardIndex(state, len(candidates)+3) % len(candidates)
		given := candidates[giveIndex]
		removeCardFromHand(state, owner, given.ID)
		addCardToHand(state, opponent, given)
		return map[string]any{
			"outcome": "lose",
			"card":    given,
		}
	}

	return map[string]any{
		"outcome": "none",
	}
}

func jokerTransformOptions() []string {
	options := make([]string, 0, len(starterCards))
	for _, card := range starterCards {
		if card.Mechanic == "joker" {
			continue
		}
		options = append(options, card.Mechanic)
	}
	return options
}

func starterCardTemplate(mechanic string) (contracts.GameCard, bool) {
	for _, card := range starterCards {
		if card.Mechanic == mechanic {
			return card, true
		}
	}
	return contracts.GameCard{}, false
}

func cloneStateAt(state *contracts.MatchState, presence *matchPresenceState, now time.Time) contracts.MatchState {
	clone := cloneState(state)
	if presence != nil {
		clone.WhiteConnected = presence.WhiteConnected
		clone.BlackConnected = presence.BlackConnected
		hideDisconnectGrace := presence.DisconnectGraceFor == disconnectGraceBoth
		if hideDisconnectGrace {
			clone.DisconnectGraceFor = ""
		} else {
			clone.DisconnectGraceFor = presence.DisconnectGraceFor
		}
		if !hideDisconnectGrace && presence.DisconnectGraceDeadline != nil {
			deadline := *presence.DisconnectGraceDeadline
			clone.DisconnectGraceDeadline = &deadline
		} else {
			clone.DisconnectGraceDeadline = nil
		}
	}
	applyClockView(&clone, now)
	return clone
}

func filterStateForColor(state contracts.MatchState, color string) contracts.MatchState {
	if state.InvisiblePiece != nil && state.InvisiblePiece.OwnerColor != color {
		state.InvisiblePiece = nil
	}
	if state.FogZones != nil {
		filteredFog := make([]contracts.FogZone, 0, len(state.FogZones))
		for _, fz := range state.FogZones {
			if fz.OwnerColor == color {
				filteredFog = append(filteredFog, fz)
			}
		}
		state.FogZones = filteredFog
	}
	return state
}

func buildSnapshot(state *contracts.MatchState, replayHead int, events []contracts.ResolvedEvent, now time.Time) contracts.MatchSnapshotResponse {
	return contracts.MatchSnapshotResponse{
		Match:        cloneStateAt(state, nil, now),
		ReplayHead:   replayHead,
		ReplayFrames: buildReplayFrames(state),
		Events:       cloneEvents(events),
	}
}

func buildSnapshotWithPresence(state *contracts.MatchState, presence *matchPresenceState, replayHead int, events []contracts.ResolvedEvent, now time.Time) contracts.MatchSnapshotResponse {
	return contracts.MatchSnapshotResponse{
		Match:        cloneStateAt(state, presence, now),
		ReplayHead:   replayHead,
		ReplayFrames: buildReplayFrames(state),
		Events:       cloneEvents(events),
	}
}

func buildReplayFrames(state *contracts.MatchState) []contracts.ReplayFrame {
	if len(state.History) == 0 {
		return nil
	}

	frames := make([]contracts.ReplayFrame, 0, len(state.History))
	for index, position := range state.History {
		frame := contracts.ReplayFrame{
			Index:         index,
			Turn:          position.Turn,
			Board:         cloneBoard(position.Board),
			HalfMoveClock: position.HalfMoveClock,
			FullMoveNum:   position.FullMoveNum,
			MoveHistory:   append([]string{}, position.MoveHistory...),
		}
		if position.LastMove != nil {
			last := *position.LastMove
			frame.LastMove = &last
		}
		frames = append(frames, frame)
	}
	return frames
}

func syncClockForMutation(state *contracts.MatchState, now time.Time) []contracts.ResolvedEvent {
	if state.Status != "active" || state.Clock.RunningFor == "" || state.Clock.StartedAt == nil {
		return nil
	}

	elapsed := now.UnixMilli() - *state.Clock.StartedAt
	if elapsed <= 0 {
		return nil
	}

	state.UpdatedAt = now.UTC()
	startedAt := now.UnixMilli()
	switch state.Clock.RunningFor {
	case "white":
		state.Clock.WhiteMS -= elapsed
		if state.Clock.WhiteMS <= 0 {
			state.Clock.WhiteMS = 0
			winner := "black"
			reason := "timeout"
			if insufficientMaterial(state.Board) {
				winner = "draw"
				reason = "timeout_vs_insufficient_material"
			}
			markMatchFinished(state, winner, reason, now)
			return timeoutEvents(state.MatchID, now, "white", winner)
		}
	case "black":
		state.Clock.BlackMS -= elapsed
		if state.Clock.BlackMS <= 0 {
			state.Clock.BlackMS = 0
			winner := "white"
			reason := "timeout"
			if insufficientMaterial(state.Board) {
				winner = "draw"
				reason = "timeout_vs_insufficient_material"
			}
			markMatchFinished(state, winner, reason, now)
			return timeoutEvents(state.MatchID, now, "black", winner)
		}
	}

	state.Clock.StartedAt = &startedAt
	return nil
}

func applyClockView(state *contracts.MatchState, now time.Time) {
	if state.Status != "active" || state.Clock.RunningFor == "" || state.Clock.StartedAt == nil {
		return
	}

	elapsed := now.UnixMilli() - *state.Clock.StartedAt
	if elapsed <= 0 {
		return
	}

	switch state.Clock.RunningFor {
	case "white":
		state.Clock.WhiteMS = maxInt64(0, state.Clock.WhiteMS-elapsed)
	case "black":
		state.Clock.BlackMS = maxInt64(0, state.Clock.BlackMS-elapsed)
	}
	startedAt := now.UnixMilli()
	state.Clock.StartedAt = &startedAt
}

func timeoutEvents(matchID string, now time.Time, flaggedColor string, winner string) []contracts.ResolvedEvent {
	return []contracts.ResolvedEvent{
		makeEvent(matchID, "clock_updated", now, "", map[string]any{
			"runningFor": "",
			"flagged":    flaggedColor,
		}),
		makeEvent(matchID, "match_finished", now, "", map[string]any{
			"result": "timeout",
			"winner": winner,
		}),
	}
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func cleanupTemporaryEffects(state *contracts.MatchState, justMovedColor string) {
	for r := 0; r < len(state.Board); r++ {
		for c := 0; c < len(state.Board[r]); c++ {
			piece := state.Board[r][c]
			if piece == nil {
				continue
			}
			if piece.Frozen && piece.Color == justMovedColor {
				piece.Frozen = false
			}
			if piece.Shielded && piece.ShieldTurn != nil && state.FullMoveNum >= *piece.ShieldTurn {
				piece.Shielded = false
				piece.ShieldTurn = nil
			}
			if piece.Borrowed && piece.Color == justMovedColor {
				piece.Color = opposite(justMovedColor)
				piece.Borrowed = false
			}
		}
	}
	if state.InvisiblePiece != nil {
		if state.InvisiblePiece.OwnerColor == justMovedColor {
			state.InvisiblePiece.RoundsLeft--
		} else if state.InvisiblePiece.RoundsLeft <= 0 {
			state.InvisiblePiece = nil
		}
	}
	if state.RadarRevealFor != "" && state.Turn != state.RadarRevealFor {
		state.RadarRevealFor = ""
	}
	if state.CheaterState != nil && state.Turn != state.CheaterState.OwnerColor {
		state.CheaterState.TurnsLeft--
		if state.CheaterState.TurnsLeft <= 0 {
			state.CheaterState = nil
		}
	}
	if state.UndoAgainst == justMovedColor {
		state.UndoAgainst = ""
	}
}

func resolveLavaEffects(state *contracts.MatchState, landing contracts.Square) (bool, string) {
	if len(state.LavaSquares) == 0 {
		return false, ""
	}

	triggered := false
	capturedPieceType := ""
	nextLava := make([]contracts.LavaSquare, 0, len(state.LavaSquares))
	for _, lava := range state.LavaSquares {
		if lava.Row == landing.Row && lava.Col == landing.Col {
			triggered = true
			piece := pieceAt(state.Board, landing)
			if piece != nil && piece.Type != "king" {
				if piece.Shielded {
					piece.Shielded = false
					piece.ShieldTurn = nil
				} else {
					capturedPieceType = piece.Type
					state.Board[landing.Row][landing.Col] = nil
				}
			}
			continue
		}

		lava.MovesLeft--
		if lava.MovesLeft > 0 {
			nextLava = append(nextLava, lava)
		}
	}
	state.LavaSquares = nextLava
	return triggered, capturedPieceType
}

func updateBombTracker(state *contracts.MatchState, from, to contracts.Square) {
	if len(state.BombPieces) == 0 {
		return
	}
	for i := range state.BombPieces {
		bomb := &state.BombPieces[i]
		if bomb.Row == from.Row && bomb.Col == from.Col {
			bomb.Row = to.Row
			bomb.Col = to.Col
			return
		}
	}
}

func resolveBombEffects(state *contracts.MatchState) []contracts.Square {
	if len(state.BombPieces) == 0 {
		return nil
	}

	nextBombs := make([]contracts.BombPiece, 0, len(state.BombPieces))
	exploded := make([]contracts.Square, 0)
	for _, bomb := range state.BombPieces {
		piece := pieceAt(state.Board, contracts.Square{Row: bomb.Row, Col: bomb.Col})
		if piece == nil || !piece.Bomb {
			continue
		}
		if piece.Color != bomb.OwnerColor {
			log.Printf("bomb dropped for match %s: bomb.OwnerColor=%s piece.Color=%s", state.MatchID, bomb.OwnerColor, piece.Color)
			continue
		}

		bomb.TurnsLeft--
		if bomb.TurnsLeft <= 0 {
			piece.Bomb = false
			for dr := -1; dr <= 1; dr++ {
				for dc := -1; dc <= 1; dc++ {
					r := bomb.Row + dr
					c := bomb.Col + dc
					if !inBounds(r, c) {
						continue
					}
					target := state.Board[r][c]
					if target != nil && target.Type != "king" {
						if target.Shielded {
							target.Shielded = false
							target.ShieldTurn = nil
							continue
						}
						state.Board[r][c] = nil
						exploded = append(exploded, contracts.Square{Row: r, Col: c})
					}
				}
			}
			continue
		}

		nextBombs = append(nextBombs, bomb)
	}

	state.BombPieces = nextBombs
	return exploded
}

func resolveFogEffects(state *contracts.MatchState, justMovedColor string) {
	if len(state.FogZones) == 0 {
		return
	}

	nextFog := make([]contracts.FogZone, 0, len(state.FogZones))
	for _, zone := range state.FogZones {
		if zone.OwnerColor != justMovedColor {
			zone.TurnsLeft--
		}
		if zone.TurnsLeft > 0 {
			nextFog = append(nextFog, zone)
		}
	}
	state.FogZones = nextFog
}

func resolveFortressEffects(state *contracts.MatchState, justMovedColor string) {
	if len(state.FortressZones) == 0 {
		return
	}

	nextFortress := make([]contracts.FortressZone, 0, len(state.FortressZones))
	for _, zone := range state.FortressZones {
		if zone.OwnerColor != justMovedColor {
			zone.TurnsLeft--
		}
		if zone.TurnsLeft > 0 {
			nextFortress = append(nextFortress, zone)
		}
	}
	state.FortressZones = nextFortress
}

func fortressEntryBlocked(zones []contracts.FortressZone, moverColor string, target contracts.Square) bool {
	for _, zone := range zones {
		if zone.OwnerColor == moverColor {
			continue
		}
		if target.Row >= zone.TopRow && target.Row <= zone.TopRow+1 && target.Col >= zone.LeftCol && target.Col <= zone.LeftCol+1 {
			return true
		}
	}
	return false
}

func resolveBlackHoleEffects(state *contracts.MatchState, justMovedColor string) []contracts.Square {
	if len(state.BlackHoles) == 0 {
		return nil
	}

	nextBlackHoles := make([]contracts.BlackHoleZone, 0, len(state.BlackHoles))
	exploded := make([]contracts.Square, 0)
	seen := make(map[string]struct{})

	blow := func(center contracts.Square) {
		for dr := -1; dr <= 1; dr++ {
			for dc := -1; dc <= 1; dc++ {
				r := center.Row + dr
				c := center.Col + dc
				if !inBounds(r, c) {
					continue
				}
				target := state.Board[r][c]
				if target != nil && target.Type != "king" {
					if target.Shielded {
						target.Shielded = false
						target.ShieldTurn = nil
						continue
					}
					state.Board[r][c] = nil
					key := keyForCoords(r, c)
					if _, ok := seen[key]; !ok {
						seen[key] = struct{}{}
						exploded = append(exploded, contracts.Square{Row: r, Col: c})
					}
				}
			}
		}
	}

	for _, hole := range state.BlackHoles {
		if hole.OwnerColor != justMovedColor {
			hole.TurnsLeft--
		}
		if hole.TurnsLeft <= 0 {
			blow(hole.Sq1)
			blow(hole.Sq2)
			continue
		}
		nextBlackHoles = append(nextBlackHoles, hole)
	}

	state.BlackHoles = nextBlackHoles
	return exploded
}

func resolveParasiteEffects(board [][]*contracts.Piece, from, to, capturedSquare contracts.Square, capturedPiece *contracts.Piece) error {
	if capturedPiece == nil {
		return nil
	}

	if capturedPiece.ParasiteTarget != "" {
		if hostSq, ok := parseParasiteSquare(capturedPiece.ParasiteTarget); ok {
			hostPiece := pieceAt(board, hostSq)
			if hostPiece != nil && hostPiece.Type != "king" {
				if hostPiece.Shielded {
					hostPiece.Shielded = false
					hostPiece.ShieldTurn = nil
				} else {
					if err := ensurePieceRemovalKeepsOwnKingSafe(board, hostSq); err != nil {
						return errors.New("parasite capture would leave a king in check")
					}
					board[hostSq.Row][hostSq.Col] = nil
				}
			}
		}
	}

	for r := 0; r < len(board); r++ {
		for c := 0; c < len(board[r]); c++ {
			piece := board[r][c]
			if piece == nil || piece.ParasiteTarget == "" {
				continue
			}
			targetSq, ok := parseParasiteSquare(piece.ParasiteTarget)
			if !ok || targetSq.Row != capturedSquare.Row || targetSq.Col != capturedSquare.Col {
				continue
			}
			if piece.Type != "king" {
				if piece.Shielded {
					piece.Shielded = false
					piece.ShieldTurn = nil
					continue
				}
				if err := ensurePieceRemovalKeepsOwnKingSafe(board, contracts.Square{Row: r, Col: c}); err != nil {
					return errors.New("parasite capture would leave a king in check")
				}
				board[r][c] = nil
			}
		}
	}

	return nil
}

func updateParasiteLinksForMove(board [][]*contracts.Piece, from, to contracts.Square) {
	for r := 0; r < len(board); r++ {
		for c := 0; c < len(board[r]); c++ {
			piece := board[r][c]
			if piece == nil || piece.ParasiteTarget == "" {
				continue
			}
			targetSq, ok := parseParasiteSquare(piece.ParasiteTarget)
			if !ok || targetSq.Row != from.Row || targetSq.Col != from.Col {
				continue
			}
			piece.ParasiteTarget = fmt.Sprintf("%d,%d", to.Row, to.Col)
		}
	}
}

func parseParasiteSquare(value string) (contracts.Square, bool) {
	parts := strings.Split(value, ",")
	if len(parts) != 2 {
		return contracts.Square{}, false
	}
	row, err := strconv.Atoi(parts[0])
	if err != nil {
		return contracts.Square{}, false
	}
	col, err := strconv.Atoi(parts[1])
	if err != nil {
		return contracts.Square{}, false
	}
	if !inBounds(row, col) {
		return contracts.Square{}, false
	}
	return contracts.Square{Row: row, Col: col}, true
}

func pieceValue(pieceType string) int {
	switch pieceType {
	case "pawn":
		return 1
	case "knight", "bishop":
		return 3
	case "rook":
		return 5
	case "queen":
		return 9
	default:
		return 0
	}
}

func clampInt(value, minValue, maxValue int) int {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func ensurePieceRemovalKeepsOwnKingSafe(board [][]*contracts.Piece, square contracts.Square) error {
	piece := pieceAt(board, square)
	if piece == nil {
		return nil
	}
	nextBoard := cloneBoard(board)
	nextBoard[square.Row][square.Col] = nil
	king := findKing(nextBoard, piece.Color)
	if king != nil && isAttacked(nextBoard, *king, opposite(piece.Color)) {
		return errors.New("removal would leave king in check")
	}
	return nil
}

func ensureRemovalDoesNotCreateCheck(board [][]*contracts.Piece, target contracts.Square, ownerColor string) error {
	nextBoard := cloneBoard(board)
	nextBoard[target.Row][target.Col] = nil

	ownerKing := findKing(nextBoard, ownerColor)
	if ownerKing != nil && isAttacked(nextBoard, *ownerKing, opposite(ownerColor)) {
		return errors.New("cannot remove that piece because it would leave your king in check")
	}

	enemyKing := findKing(nextBoard, opposite(ownerColor))
	if enemyKing != nil && isAttacked(nextBoard, *enemyKing, ownerColor) {
		return errors.New("cannot remove that piece because it would put the enemy king in check")
	}

	return nil
}

func safeTransformOptions(board [][]*contracts.Piece, target contracts.Square, mechanic string) []string {
	piece := pieceAt(board, target)
	if piece == nil {
		return nil
	}

	options := transformOptions(piece.Type, mechanic)
	if len(options) == 0 {
		return nil
	}

	safe := make([]string, 0, len(options))
	for _, option := range options {
		nextBoard := cloneBoard(board)
		nextPiece := nextBoard[target.Row][target.Col]
		if nextPiece == nil {
			continue
		}
		nextPiece.Type = option

		if !kingsRemainSafe(nextBoard) {
			continue
		}

		safe = append(safe, option)
	}

	return safe
}

func transformOptions(pieceType string, mechanic string) []string {
	switch mechanic {
	case "promote", "promotehim":
		switch pieceType {
		case "pawn":
			return []string{"knight", "bishop", "rook", "queen"}
		case "knight":
			return []string{"bishop", "rook", "queen"}
		case "bishop":
			return []string{"knight", "rook", "queen"}
		case "rook":
			return []string{"queen"}
		}
	case "demote", "demotehim":
		switch pieceType {
		case "queen":
			return []string{"rook", "bishop", "knight", "pawn"}
		case "rook":
			return []string{"bishop", "knight", "pawn"}
		case "bishop":
			return []string{"knight", "pawn"}
		case "knight":
			return []string{"pawn"}
		}
	}

	return nil
}

func validateTransformTarget(piece *contracts.Piece, ownerColor string, mechanic string) error {
	if piece == nil || piece.Type == "king" {
		switch mechanic {
		case "promotehim":
			return errors.New("promotehim requires an enemy non-king target")
		case "demotehim":
			return errors.New("demotehim requires a non-king target")
		case "promote":
			return errors.New("promote requires your own non-king target")
		default:
			return errors.New("demote requires your own non-king target")
		}
	}

	switch mechanic {
	case "promote":
		if piece.Color != ownerColor {
			return errors.New("promote requires your own non-king target")
		}
	case "demote":
		if piece.Color != ownerColor {
			return errors.New("demote requires your own non-king target")
		}
	case "promotehim":
		if piece.Color == ownerColor {
			return errors.New("promotehim requires an enemy non-king target")
		}
	case "demotehim":
		return nil
	}

	return nil
}

func kingsRemainSafe(board [][]*contracts.Piece) bool {
	whiteKing := findKing(board, "white")
	if whiteKing != nil && isAttacked(board, *whiteKing, "black") {
		return false
	}
	blackKing := findKing(board, "black")
	if blackKing != nil && isAttacked(board, *blackKing, "white") {
		return false
	}
	return true
}

func kingsRemainSafeWithFusion(board [][]*contracts.Piece) bool {
	whiteKing := findKing(board, "white")
	if whiteKing != nil && isAttackedWithFusion(board, *whiteKing, "black") {
		return false
	}
	blackKing := findKing(board, "black")
	if blackKing != nil && isAttackedWithFusion(board, *blackKing, "white") {
		return false
	}
	return true
}

func isAttackedWithFusion(board [][]*contracts.Piece, target contracts.Square, by string) bool {
	if isAttacked(board, target, by) {
		return true
	}
	for r := 0; r < len(board); r++ {
		for c := 0; c < len(board[r]); c++ {
			piece := board[r][c]
			if piece == nil || piece.Color != by || piece.FusedWith == "" {
				continue
			}
			tempBoard := cloneBoard(board)
			tempBoard[r][c] = &contracts.Piece{
				Type:           piece.FusedWith,
				Color:          piece.Color,
				Shielded:       piece.Shielded,
				ShieldTurn:     piece.ShieldTurn,
				Frozen:         piece.Frozen,
				Borrowed:       piece.Borrowed,
				ParasiteTarget: piece.ParasiteTarget,
				Bomb:           piece.Bomb,
				Invisible:      piece.Invisible,
				InvisibleTurn:  piece.InvisibleTurn,
				InvisibleOver:  piece.InvisibleOver,
			}
			if isAttacked(tempBoard, target, by) {
				return true
			}
		}
	}
	return false
}

func fusionRedundancy(typeA string, typeB string, sqA contracts.Square, sqB contracts.Square) string {
	if typeA == typeB {
		return "cannot fuse identical piece types"
	}
	if (typeA == "queen" && typeB == "rook") || (typeA == "rook" && typeB == "queen") {
		return "queen already moves like a rook"
	}
	if (typeA == "queen" && typeB == "bishop") || (typeA == "bishop" && typeB == "queen") {
		return "queen already moves like a bishop"
	}
	if (typeA == "queen" && typeB == "pawn") || (typeA == "pawn" && typeB == "queen") {
		return "queen already outclasses pawn movement"
	}
	if typeA == "bishop" && typeB == "bishop" && ((sqA.Row+sqA.Col)%2 == (sqB.Row+sqB.Col)%2) {
		return "bishops on the same color add no new movement"
	}
	return ""
}

func jumpHasExactlyOnePieceBetween(board [][]*contracts.Piece, from contracts.Square, to contracts.Square) bool {
	dr := to.Row - from.Row
	dc := to.Col - from.Col
	if dr == 0 && dc == 0 {
		return false
	}

	sr := sign(dr)
	sc := sign(dc)
	r := from.Row + sr
	c := from.Col + sc
	count := 0
	for r != to.Row || c != to.Col {
		if !inBounds(r, c) {
			return false
		}
		if board[r][c] != nil {
			count++
		}
		r += sr
		c += sc
	}

	return count == 1
}

func jumpDirectionValid(from contracts.Square, to contracts.Square, pieceType string, pieceColor string) bool {
	dr := to.Row - from.Row
	dc := to.Col - from.Col
	diag := abs(dr) == abs(dc)
	straight := dr == 0 || dc == 0

	switch pieceType {
	case "bishop":
		return diag
	case "rook":
		return straight
	case "queen":
		return diag || straight
	case "pawn":
		fwd := 1
		if pieceColor == "black" {
			fwd = -1
		}
		return (dc == 0 && (dr == fwd || dr == fwd*2)) || (abs(dc) == 2 && dr == fwd*2)
	default:
		return false
	}
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func cloneCardsWithOwner(cards []contracts.GameCard, owner string) []contracts.GameCard {
	out := make([]contracts.GameCard, 0, len(cards))
	for index, card := range cards {
		next := card
		next.ID = fmt.Sprintf("%s_%d_%s", card.ID, index+1, owner)
		out = append(out, next)
	}
	return out
}

func cardFromHand(state *contracts.MatchState, owner string, cardID string) (contracts.GameCard, bool) {
	hand := state.WhiteHand
	if owner == "black" {
		hand = state.BlackHand
	}
	for _, card := range hand {
		if card.ID == cardID {
			return card, true
		}
	}
	return contracts.GameCard{}, false
}

func removeCardFromHand(state *contracts.MatchState, owner string, cardID string) {
	hand := state.WhiteHand
	if owner == "black" {
		hand = state.BlackHand
	}
	filtered := make([]contracts.GameCard, 0, len(hand))
	for _, card := range hand {
		if card.ID != cardID {
			filtered = append(filtered, card)
		}
	}
	if owner == "black" {
		state.BlackHand = filtered
	} else {
		state.WhiteHand = filtered
	}
}

func addRewardCards(state *contracts.MatchState, owner string, count int, now time.Time) []contracts.GameCard {
	var hand *[]contracts.GameCard
	if owner == "black" {
		hand = &state.BlackHand
	} else {
		hand = &state.WhiteHand
	}

	drawn := make([]contracts.GameCard, 0, count)
	for i := 0; i < count; i++ {
		template := rewardTemplateForState(state, len(drawn))
		next := template
		next.ID = fmt.Sprintf("%s_reward_%d_%d_%s", template.ID, now.UnixMilli(), i+1, owner)
		*hand = append(*hand, next)
		drawn = append(drawn, next)
	}
	return drawn
}

func drawRoundCards(state *contracts.MatchState, now time.Time) (white []contracts.GameCard, black []contracts.GameCard) {
	if state.Turn != "white" || state.FullMoveNum < drawFromRound {
		return nil, nil
	}
	if (state.FullMoveNum-drawFromRound)%drawEveryRounds != 0 {
		return nil, nil
	}
	if len(state.WhiteHand) < maxHandSize {
		white = addRewardCards(state, "white", 1, now)
	}
	if len(state.BlackHand) < maxHandSize {
		black = addRewardCards(state, "black", 1, now)
	}
	return white, black
}

func rewardTemplateForState(state *contracts.MatchState, offset int) contracts.GameCard {
	index := deterministicCardIndex(state, offset)
	if index < 0 {
		index = 0
	}
	return starterCards[index]
}

func deterministicCardIndex(state *contracts.MatchState, offset int) int {
	rng := state.RNGSeed + int64(len(state.MoveHistory))*7 + int64(len(state.History))*3 + int64(len(state.WhiteHand)+len(state.BlackHand)) + int64(offset)
	return int(uint64(rng) % uint64(len(starterCards)))
}

func parseSquareOptions(options []string) []contracts.Square {
	selected := make([]contracts.Square, 0, len(options))
	for _, option := range options {
		if sq, ok := parseParasiteSquare(option); ok {
			selected = append(selected, sq)
		}
	}
	return selected
}

func encodeSquareOptions(squares []contracts.Square) []string {
	options := make([]string, 0, len(squares))
	for _, sq := range squares {
		options = append(options, fmt.Sprintf("%d,%d", sq.Row, sq.Col))
	}
	return options
}

func toggleSquareInList(values []contracts.Square, target contracts.Square) []contracts.Square {
	next := make([]contracts.Square, 0, len(values))
	removed := false
	for _, value := range values {
		if value.Row == target.Row && value.Col == target.Col {
			removed = true
			continue
		}
		next = append(next, value)
	}
	if !removed {
		next = append(next, target)
	}
	return next
}

func selectedSquaresValue(board [][]*contracts.Piece, selected []contracts.Square) int {
	total := 0
	for _, sq := range selected {
		piece := pieceAt(board, sq)
		if piece == nil {
			continue
		}
		total += pieceValue(piece.Type)
	}
	return total
}

func addCardToHand(state *contracts.MatchState, owner string, card contracts.GameCard) {
	if owner == "black" {
		if len(state.BlackHand) >= maxHandSize {
			return
		}
		state.BlackHand = append(state.BlackHand, card)
		return
	}
	if len(state.WhiteHand) >= maxHandSize {
		return
	}
	state.WhiteHand = append(state.WhiteHand, card)
}

func filterCardsNotMechanic(hand []contracts.GameCard, mechanic string) []contracts.GameCard {
	filtered := make([]contracts.GameCard, 0, len(hand))
	for _, card := range hand {
		if card.Mechanic != mechanic {
			filtered = append(filtered, card)
		}
	}
	return filtered
}

func cloneEvents(events []contracts.ResolvedEvent) []contracts.ResolvedEvent {
	cloned := make([]contracts.ResolvedEvent, 0, len(events))
	for _, event := range events {
		next := event
		next.Payload = copyPayload(event.Payload)
		cloned = append(cloned, next)
	}
	return cloned
}

func copyPayload(input map[string]any) map[string]any {
	out := make(map[string]any, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

func cloneBoard(board [][]*contracts.Piece) [][]*contracts.Piece {
	next := make([][]*contracts.Piece, len(board))
	for r := range board {
		next[r] = make([]*contracts.Piece, len(board[r]))
		for c := range board[r] {
			if board[r][c] == nil {
				continue
			}
			piece := *board[r][c]
			next[r][c] = &piece
		}
	}
	return next
}
