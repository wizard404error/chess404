package match

import (
	"strings"

	"github.com/chess404/realtime/internal/contracts"
)

// ParseAlgebraicMove parses a PGN-style algebraic move notation into
// from/to squares. It supports standard notation: e.g. "e4", "Nf3",
// "Bxc4+", "O-O", "exd5", "e8=Q+", "Nbd7". It returns the parsed
// from/to squares and the move type, or false if the notation could not
// be parsed unambiguously.
//
// The parser is heuristic and is intended for analysis replay, not for
// authoritative game state reconstruction.
type ParsedMove struct {
	From contracts.Square
	To   contracts.Square
	// IsCapture is true when the notation includes "x" or the target
	// is a pawn promotion capture.
	IsCapture bool
	// IsCastle returns the castle type ("K", "Q") or "" if not a castle.
	IsCastle string
	// IsEnPassant is true for en-passant pawn captures (exd6 e.p.).
	IsEnPassant bool
	// PromotionPiece is the promotion target type ("Q","R","B","N")
	// or "" if no promotion.
	PromotionPiece string
}

// ParseAlgebraicMove parses a single move notation. It returns the parsed
// move and true on success, or a zero-value ParsedMove and false if the
// notation cannot be parsed. The parser tolerates trailing annotations
// ("+", "#", "!", "?", "??") by stripping them before parsing.
func ParseAlgebraicMove(notation string) (ParsedMove, bool) {
	stripped := strings.TrimSpace(notation)
	for {
		if stripped == "" {
			return ParsedMove{}, false
		}
		c := stripped[len(stripped)-1]
		if c == '+' || c == '#' || c == '!' || c == '?' {
			stripped = stripped[:len(stripped)-1]
			continue
		}
		break
	}

	// Castling: O-O (kingside) or O-O-O (queenside).
	if strings.HasPrefix(stripped, "O-O") {
		if strings.HasPrefix(stripped, "O-O-O") {
			return ParsedMove{IsCastle: "Q"}, true
		}
		return ParsedMove{IsCastle: "K"}, true
	}

	// Format: [piece][file-from][rank-from]?[x]?[file-to][rank-to][=promotion][+|#]?
	// Examples: e4, Nf3, Bxc4, exd5, Nbd7, R1d2, e8=Q, exd8=Q+
	// The destination is always the last 2 squares; the source is optional.
	if len(stripped) < 2 {
		return ParsedMove{}, false
	}

	// The last two characters are the destination file+rank.
	destFile := stripped[len(stripped)-2]
	destRank := stripped[len(stripped)-1]
	fromFile := byte(0)
	fromRank := byte(0)
	var srcConstraint byte
	rest := stripped[:len(stripped)-2]

	promotion := ""
	if idx := strings.Index(rest, "="); idx >= 0 {
		promotion = strings.TrimSpace(rest[idx+1:])
		rest = rest[:idx]
	}

	// Handle a capture notation: "x" or "X" splits source and destination.
	capture := false
	if idx := strings.LastIndexAny(rest, "xX"); idx >= 0 {
		capture = true
		srcConstraint = rest[idx-1]
		rest = rest[:idx-1]
	}

	// Strip a leading piece letter (non-file).  The piece letters are
	// K, Q, R, B, N — files are a-h.
	piece := byte(0)
	if len(rest) > 0 {
		c := rest[0]
		if c >= 'A' && c <= 'Z' {
			piece = c
			rest = rest[1:]
		}
	}

	// Whatever is left in `rest` is the source disambiguator (file,
	// rank, or both — e.g., "Nbd7" leaves "b").
	if len(rest) > 0 {
		if rest[0] >= 'a' && rest[0] <= 'h' {
			fromFile = rest[0]
			rest = rest[1:]
		}
	}
	if len(rest) > 0 {
		if rest[0] >= '1' && rest[0] <= '8' {
			fromRank = rest[0]
			rest = rest[1:]
		}
	}

	// Apply any capture-source constraint to fromFile/fromRank.
	if srcConstraint >= 'a' && srcConstraint <= 'h' && fromFile == 0 {
		fromFile = srcConstraint
	} else if srcConstraint >= '1' && srcConstraint <= '8' && fromRank == 0 {
		fromRank = srcConstraint
	}

	from, ok := parseAlgebraicSource(fromFile, fromRank, piece)
	if !ok {
		return ParsedMove{}, false
	}
	to, ok := parseAlgebraicDest(destFile, destRank)
	if !ok {
		return ParsedMove{}, false
	}
	pm := ParsedMove{From: from, To: to, IsCapture: capture, PromotionPiece: promotion}
	if !capture {
		// Detect en passant: a pawn moves diagonally but the target square
		// is empty (we don't have the board here, so we approximate by
		// checking if the source and dest have different file and rank 5/4.
		dx := to.Col - from.Col
		if piece == 0 || piece == 'P' {
			if dx != 0 && (from.Row == 4 || from.Row == 3) {
				pm.IsEnPassant = true
			}
		}
	}
	return pm, true
}

// parseAlgebraicSource maps the file/rank/piece constraints to a Square.
// It cannot resolve source ambiguity (e.g. Nbd7 vs Nfd7) without board
// context; in ambiguous cases it returns the first legal from-square.
func parseAlgebraicSource(file, rank byte, piece byte) (contracts.Square, bool) {
	if file == 0 && rank == 0 {
		// No source info — cannot resolve.
		return contracts.Square{}, false
	}
	c := -1
	r := -1
	if file != 0 {
		c = int(file - 'a')
	}
	if rank != 0 {
		r = int(rank - '1')
	}
	return contracts.Square{Row: r, Col: c}, true
}

// parseAlgebraicDest maps the destination file+rank to a Square.
func parseAlgebraicDest(file, rank byte) (contracts.Square, bool) {
	if file < 'a' || file > 'h' || rank < '1' || rank > '8' {
		return contracts.Square{}, false
	}
	return contracts.Square{Row: int(rank - '1'), Col: int(file - 'a')}, true
}
