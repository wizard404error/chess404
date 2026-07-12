"""Generate an opening book JSON for the 404-chess engine.

Usage:
  python scripts/generate_opening_book.py --output services/realtime/opening_book.json

Generates a JSON file mapping Zobrist-like hashes → weighted UCI moves
from common chess openings.
"""

import json
import chess
import chess.pgn
import random
from collections import defaultdict

# Common openings as UCI move sequences
OPENINGS = [
    # Ruy Lopez (Italian-ish)
    ["e2e4", "e7e5", "g1f3", "b8c6", "f1b5"],
    ["e2e4", "e7e5", "g1f3", "b8c6", "f1c4"],
    ["e2e4", "e7e5", "g1f3", "b8c6", "d2d4"],
    # Sicilian
    ["e2e4", "c7c5"],
    ["e2e4", "c7c5", "g1f3", "d7d6", "d2d4"],
    ["e2e4", "c7c5", "g1f3", "d7d6", "d2d4", "c5d4", "f3d4"],
    ["e2e4", "c7c5", "g1f3", "e7e6", "d2d4"],
    # French
    ["e2e4", "e7e6", "d2d4", "d7d5"],
    ["e2e4", "e7e6", "d2d4", "d7d5", "e4e5"],
    ["e2e4", "e7e6", "d2d4", "d7d5", "b1c3"],
    # Caro-Kann
    ["e2e4", "c7c6", "d2d4", "d7d5"],
    ["e2e4", "c7c6", "d2d4", "d7d5", "e4d5"],
    # Pirc/Modern
    ["e2e4", "d7d6", "d2d4"],
    ["e2e4", "g7g6", "d2d4"],
    # Queen's Gambit
    ["d2d4", "d7d5", "c2c4"],
    ["d2d4", "d7d5", "c2c4", "e7e6"],
    ["d2d4", "d7d5", "c2c4", "c7c6"],
    ["d2d4", "g8f6", "c2c4"],
    # Indian Defenses
    ["d2d4", "g8f6", "c2c4", "g7g6", "b1c3", "f8g7"],
    ["d2d4", "g8f6", "c2c4", "e7e6"],
    ["d2d4", "g8f6", "c2c4", "e7e6", "g1f3", "b7b6"],
    ["d2d4", "g8f6", "c2c4", "g7g6"],
    # English
    ["c2c4"],
    ["c2c4", "e7e5"],
    ["c2c4", "c7c5"],
    ["c2c4", "g8f6"],
    # Reti/KIA
    ["g1f3"],
    ["g1f3", "d7d5"],
    ["g1f3", "g8f6"],
    # Bird
    ["f2f4"],
    # Dutch
    ["d2d4", "f7f5"],
]


def zobrist_like_hash(board: chess.Board) -> int:
    """Simple Zobrist-style hash for consistent book key."""
    h = 0x123456789ABCDEF
    for sq in chess.SQUARES:
        piece = board.piece_at(sq)
        if piece:
            piece_key = piece.piece_type * 2 + (0 if piece.color else 1)
            h ^= (piece_key * (sq + 1) * 0x9E3779B97F4A7C15) & 0xFFFFFFFFFFFFFFFF
            h = ((h << 31) | (h >> 33)) & 0xFFFFFFFFFFFFFFFF
    if board.turn == chess.BLACK:
        h ^= 0xFFFFFFFFFFFFFFFF
    return h & 0xFFFFFFFFFFFFFFFF


def generate_book(openings, max_depth=12):
    """Generate opening book from move sequences."""
    book = defaultdict(lambda: defaultdict(float))
    for opening in openings:
        board = chess.Board()
        for uci in opening:
            try:
                move = chess.Move.from_uci(uci)
                if move not in board.legal_moves:
                    break
                board.push(move)
            except Exception:
                break

    for opening in openings:
        board = chess.Board()
        seen = set()
        for depth, uci in enumerate(opening):
            if depth >= max_depth:
                break
            try:
                move = chess.Move.from_uci(uci)
                if move not in board.legal_moves:
                    break

                board_hash = zobrist_like_hash(board)
                book[board_hash][uci] += 1.0 / (depth + 1)

                board.push(move)
            except Exception:
                break

    # Convert to output format
    output = {}
    for h, moves in book.items():
        entries = []
        total = sum(moves.values())
        for uci, weight in moves.items():
            entries.append({"move": uci, "weight": round(weight / total, 4)})
        entries.sort(key=lambda x: -x["weight"])
        output[str(h)] = entries

    return output


if __name__ == "__main__":
    import argparse
    parser = argparse.ArgumentParser()
    parser.add_argument("--output", default="services/realtime/opening_book.json")
    parser.add_argument("--max-depth", type=int, default=10)
    args = parser.parse_args()

    book = generate_book(OPENINGS, args.max_depth)
    with open(args.output, "w") as f:
        json.dump(book, f, indent=2)
    print(f"Opening book written to {args.output}")
    print(f"  {len(book)} positions, {sum(len(v) for v in book.values())} entries")
