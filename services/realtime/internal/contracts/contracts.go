package contracts

import (
	"strings"
	"time"
)

type MatchModeID string

const (
	MatchModeOpenCards   MatchModeID = "open_cards"
	MatchModeHiddenCards MatchModeID = "hidden_cards"
)

func NormalizeMatchModeID(value string) MatchModeID {
	switch MatchModeID(strings.TrimSpace(value)) {
	case MatchModeHiddenCards:
		return MatchModeHiddenCards
	default:
		return MatchModeOpenCards
	}
}

type Square struct {
	Row int `json:"row"`
	Col int `json:"col"`
}

type Piece struct {
	Type           string `json:"type"`
	Color          string `json:"color"`
	Shielded       bool   `json:"shielded,omitempty"`
	ShieldTurn     *int   `json:"shieldTurn,omitempty"`
	Frozen         bool   `json:"frozen,omitempty"`
	Borrowed       bool   `json:"borrowed,omitempty"`
	ParasiteTarget string `json:"parasiteTarget,omitempty"`
	Bomb           bool   `json:"bomb,omitempty"`
	Invisible      bool   `json:"invisible,omitempty"`
	InvisibleTurn  *int   `json:"invisibleTurn,omitempty"`
	InvisibleOver  bool   `json:"invisibleOver,omitempty"`
	FusedWith      string `json:"fusedWith,omitempty"`
	// Fake marks a piece placed by the fakepiece card. Fakes look like pawns
	// to the opponent (to bait captures) but cannot promote, are not counted
	// in the 8-pawn promotion budget, and disappear if an enemy piece lands
	// on them (they are removed without giving the opponent a normal
	// pawn-capture event).
	Fake bool `json:"fake,omitempty"`
}

type GameCard struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Mechanic string `json:"mechanic"`
	Type     string `json:"type"`
	Rarity   string `json:"rarity"`
	Color    string `json:"color"`
	Accent   string `json:"accent"`
	Icon     string `json:"icon"`
	Desc     string `json:"desc"`
}

type PendingCardState struct {
	CardID     string   `json:"cardId"`
	Mechanic   string   `json:"mechanic"`
	OwnerColor string   `json:"ownerColor"`
	Target     *Square  `json:"target,omitempty"`
	Options    []string `json:"options,omitempty"`
}

type LastMove struct {
	From Square `json:"from"`
	To   Square `json:"to"`
}

type ChatMessage struct {
	Sender string    `json:"sender"`
	Text   string    `json:"text"`
	SentAt time.Time `json:"sentAt"`
}

type MatchClock struct {
	WhiteMS    int64  `json:"whiteMs"`
	BlackMS    int64  `json:"blackMs"`
	RunningFor string `json:"runningFor,omitempty"`
	StartedAt  *int64 `json:"startedAtMs,omitempty"`
	Increment  int64  `json:"increment,omitempty"`
}

type DoubleMoveState struct {
	Type      string  `json:"type"`
	MovesLeft int     `json:"movesLeft"`
	TrackedSq *Square `json:"trackedSq,omitempty"`
	FirstNote string  `json:"firstNote,omitempty"`
}

type LavaSquare struct {
	Row       int `json:"row"`
	Col       int `json:"col"`
	MovesLeft int `json:"movesLeft"`
}

type BombPiece struct {
	Row        int    `json:"row"`
	Col        int    `json:"col"`
	TurnsLeft  int    `json:"turnsLeft"`
	OwnerColor string `json:"ownerColor"`
}

type BlackHoleZone struct {
	Sq1        Square `json:"sq1"`
	Sq2        Square `json:"sq2"`
	TurnsLeft  int    `json:"turnsLeft"`
	OwnerColor string `json:"ownerColor"`
}

type FogZone struct {
	CenterRow  int    `json:"centerRow"`
	CenterCol  int    `json:"centerCol"`
	TurnsLeft  int    `json:"turnsLeft"`
	OwnerColor string `json:"ownerColor"`
}

type FortressZone struct {
	TopRow     int    `json:"topRow"`
	LeftCol    int    `json:"leftCol"`
	TurnsLeft  int    `json:"turnsLeft"`
	OwnerColor string `json:"ownerColor"`
}

type InvisiblePieceState struct {
	Row        int    `json:"row"`
	Col        int    `json:"col"`
	Piece      Piece  `json:"piece"`
	OwnerColor string `json:"ownerColor"`
	RoundsLeft int    `json:"roundsLeft"`
}

type CheaterState struct {
	OwnerColor string `json:"ownerColor"`
	TurnsLeft  int    `json:"turnsLeft"`
}

type PositionState struct {
	Board          [][]*Piece           `json:"board"`
	LavaSquares    []LavaSquare         `json:"lavaSquares,omitempty"`
	BombPieces     []BombPiece          `json:"bombPieces,omitempty"`
	BlackHoles     []BlackHoleZone      `json:"blackHoles,omitempty"`
	FogZones       []FogZone            `json:"fogZones,omitempty"`
	FortressZones  []FortressZone       `json:"fortressZones,omitempty"`
	InvisiblePiece *InvisiblePieceState `json:"invisiblePiece,omitempty"`
	CheaterState   *CheaterState        `json:"cheaterState,omitempty"`
	RadarRevealFor string               `json:"radarRevealFor,omitempty"`
	Turn           string               `json:"turn"`
	Moved          []string             `json:"moved"`
	LastMove       *LastMove            `json:"lastMove,omitempty"`
	HalfMoveClock  int                  `json:"halfMoveClock"`
	FullMoveNum    int                  `json:"fullMoveNumber"`
	MoveHistory    []string             `json:"moveHistory"`
	// Hands, pending card, double move, undo, and draw offer are snapshotted
	// alongside the board so the Reverse card can restore the full match
	// state to a previous turn (not just the chess position).
	WhiteHand    []GameCard        `json:"whiteHand,omitempty"`
	BlackHand    []GameCard        `json:"blackHand,omitempty"`
	PendingCard  *PendingCardState `json:"pendingCard,omitempty"`
	DoubleMove   *DoubleMoveState  `json:"doubleMove,omitempty"`
	UndoAgainst  string            `json:"undoAgainst,omitempty"`
	DrawOfferedBy string           `json:"drawOfferedBy,omitempty"`
}

type ReplayFrame struct {
	Index         int        `json:"index"`
	Turn          string     `json:"turn"`
	Board         [][]*Piece `json:"board"`
	LastMove      *LastMove  `json:"lastMove,omitempty"`
	HalfMoveClock int        `json:"halfMoveClock"`
	FullMoveNum   int        `json:"fullMoveNumber"`
	MoveHistory   []string   `json:"moveHistory"`
}

type MatchState struct {
	MatchID                 string               `json:"matchId"`
	RulesVersion            string               `json:"rulesVersion"`
	RNGSeed                 int64                `json:"rngSeed"`
	Queue                   string               `json:"queue,omitempty"`
	ModeID                  MatchModeID          `json:"modeId,omitempty"`
	WhiteGuestID            string               `json:"whiteGuestId,omitempty"`
	BlackGuestID            string               `json:"blackGuestId,omitempty"`
	WhiteAccountID          string               `json:"whiteAccountId,omitempty"`
	BlackAccountID          string               `json:"blackAccountId,omitempty"`
	WhiteName               string               `json:"whiteName,omitempty"`
	BlackName               string               `json:"blackName,omitempty"`
	WhitePlayerSecret       string               `json:"-"`
	BlackPlayerSecret       string               `json:"-"`
	Board                   [][]*Piece           `json:"board"`
	LavaSquares             []LavaSquare         `json:"lavaSquares,omitempty"`
	BombPieces              []BombPiece          `json:"bombPieces,omitempty"`
	BlackHoles              []BlackHoleZone      `json:"blackHoles,omitempty"`
	FogZones                []FogZone            `json:"fogZones,omitempty"`
	FortressZones           []FortressZone       `json:"fortressZones,omitempty"`
	InvisiblePiece          *InvisiblePieceState `json:"invisiblePiece,omitempty"`
	CheaterState            *CheaterState        `json:"cheaterState,omitempty"`
	RadarRevealFor          string               `json:"radarRevealFor,omitempty"`
	DoubleMove              *DoubleMoveState     `json:"doubleMove,omitempty"`
	UndoAgainst             string               `json:"undoAgainst,omitempty"`
	Turn                    string               `json:"turn"`
	Moved                   []string             `json:"moved"`
	LastMove                *LastMove            `json:"lastMove,omitempty"`
	HalfMoveClock           int                  `json:"halfMoveClock"`
	FullMoveNum             int                  `json:"fullMoveNumber"`
	WhiteHand               []GameCard           `json:"whiteHand"`
	BlackHand               []GameCard           `json:"blackHand"`
	MoveHistory             []string             `json:"moveHistory"`
	ChatMessages            []ChatMessage        `json:"chatMessages"`
	Clock                   MatchClock           `json:"clock"`
	WhiteConnected          bool                 `json:"whiteConnected"`
	BlackConnected          bool                 `json:"blackConnected"`
	DisconnectGraceFor      string               `json:"disconnectGraceFor,omitempty"`
	DisconnectGraceDeadline *time.Time           `json:"disconnectGraceDeadline,omitempty"`
	Status                  string               `json:"status"`
	Winner                  string               `json:"winner,omitempty"`
	FinishReason            string               `json:"finishReason,omitempty"`
	DrawOfferedBy           string               `json:"drawOfferedBy,omitempty"`
	DrawOfferTime           time.Time            `json:"drawOfferTime,omitempty"`
	SeenClientMoveIDs       []string             `json:"seenClientMoveIds,omitempty"`
	PendingCard             *PendingCardState    `json:"pendingCard,omitempty"`
	CreatedAt               time.Time            `json:"createdAt"`
	UpdatedAt               time.Time            `json:"updatedAt"`
	History                 []PositionState      `json:"-"`
}

type PlayerIntent struct {
	Type             string  `json:"type"`
	MatchID          string  `json:"matchId"`
	PlayerID         string  `json:"playerId"`
	PlayerSecret     string  `json:"playerSecret,omitempty"`
	PlayerClaimToken string  `json:"playerClaimToken,omitempty"`
	CardID           string  `json:"cardId,omitempty"`
	Text             string  `json:"text,omitempty"`
	Accept           *bool   `json:"accept,omitempty"`
	From             *Square `json:"from,omitempty"`
	To               *Square `json:"to,omitempty"`
	Target           *Square `json:"target,omitempty"`
	SelectionID      string  `json:"selectionId,omitempty"`
	Promotion        string  `json:"promotion,omitempty"`
	ClientMoveID     string  `json:"clientMoveId,omitempty"`
}

type MatchPresenceRequest struct {
	PlayerID         string `json:"playerId"`
	PlayerSecret     string `json:"playerSecret,omitempty"`
	PlayerClaimToken string `json:"playerClaimToken,omitempty"`
}

type ResolvedEvent struct {
	ID      string         `json:"id"`
	MatchID string         `json:"matchId"`
	Type    string         `json:"type"`
	ActorID string         `json:"actorId,omitempty"`
	At      time.Time      `json:"at"`
	Payload map[string]any `json:"payload"`
}

type MatchSnapshotResponse struct {
	Match        MatchState      `json:"match"`
	ReplayHead   int             `json:"replayHead"`
	ReplayFrames []ReplayFrame   `json:"replayFrames,omitempty"`
	Events       []ResolvedEvent `json:"events,omitempty"`
	SeqNum       int64           `json:"seqNum,omitempty"`
}

type CreateMatchRequest struct {
	MatchID           string      `json:"matchId,omitempty"`
	Seed              int64       `json:"seed,omitempty"`
	ClockSeconds      int64       `json:"clockSeconds,omitempty"`
	ClockIncrement    int64       `json:"clockIncrement,omitempty"`
	StarterHandMode   string      `json:"starterHandMode,omitempty"`
	Queue             string      `json:"queue,omitempty"`
	ModeID            MatchModeID `json:"modeId,omitempty"`
	WhiteGuestID      string      `json:"whiteGuestId,omitempty"`
	BlackGuestID      string      `json:"blackGuestId,omitempty"`
	WhiteAccountID    string      `json:"whiteAccountId,omitempty"`
	BlackAccountID    string      `json:"blackAccountId,omitempty"`
	WhiteName         string      `json:"whiteName,omitempty"`
	BlackName         string      `json:"blackName,omitempty"`
	WhitePlayerSecret string      `json:"whitePlayerSecret,omitempty"`
	BlackPlayerSecret string      `json:"blackPlayerSecret,omitempty"`
}

type JoinMatchSeatRequest struct {
	GuestID       string `json:"guestId"`
	AccountID     string `json:"accountId,omitempty"`
	DisplayName   string `json:"displayName,omitempty"`
	PlayerSecret  string `json:"playerSecret,omitempty"`
	PreferredSeat string `json:"preferredSeat,omitempty"`
}

type JoinMatchSeatResponse struct {
	Match              MatchSnapshotResponse `json:"match"`
	SeatColor          string                `json:"seatColor"`
	Joined             bool                  `json:"joined"`
	WaitingForOpponent bool                  `json:"waitingForOpponent"`
}

type ApplyIntentRequest struct {
	Intent PlayerIntent `json:"intent"`
}

type Envelope struct {
	Type    string `json:"type"`
	Payload any    `json:"payload"`
}
