"""Train NNUE weights for 404-chess engine.

Usage:
  pip install torch numpy python-chess tqdm
  python scripts/train_nnue.py --games 1000 --epochs 10 --output services/realtime/nnue_weights.bin

Generates a binary weight file loadable by internal/engine/nnue.go.
Architecture: 12×64 + 5 → 256 → 1 (ClampedReLU hidden, linear output).
"""

import struct
import numpy as np
import torch
import torch.nn as nn
import torch.optim as optim
from torch.utils.data import Dataset, DataLoader
import random
from tqdm import tqdm

# Feature dimensions
PIECE_TYPES = 6
COLORS = 2
SQUARES = 64
MODIFIERS = 5
INPUT_SIZE = PIECE_TYPES * COLORS * SQUARES + MODIFIERS
HIDDEN_SIZE = 256

class NNUE(nn.Module):
    def __init__(self):
        super().__init__()
        self.input_layer = nn.Linear(INPUT_SIZE, HIDDEN_SIZE)
        self.output_layer = nn.Linear(HIDDEN_SIZE, 1)

    def forward(self, x):
        h = self.input_layer(x)
        h = torch.clamp(h, min=0)  # ReLU
        out = self.output_layer(h)
        return out


def encode_position(board_fen: str, score: float, turn: str):
    """Convert a position to NNUE input features + target."""
    pieces_map = {
        'P': (0, 0), 'N': (1, 0), 'B': (2, 0), 'R': (3, 0), 'Q': (4, 0), 'K': (5, 0),
        'p': (0, 1), 'n': (1, 1), 'b': (2, 1), 'r': (3, 1), 'q': (4, 1), 'k': (5, 1),
    }
    rows = board_fen.split('/')
    features = np.zeros(INPUT_SIZE, dtype=np.float32)

    for r, row_str in enumerate(rows):
        col = 0
        for ch in row_str:
            if ch.isdigit():
                col += int(ch)
                continue
            if ch in pieces_map:
                ptype, color_idx = pieces_map[ch]
                sq = (7 - r) * 8 + col  # board row 0 = rank 8
                idx = (color_idx * PIECE_TYPES + ptype) * SQUARES + sq
                features[idx] = 1.0
                col += 1

    target = score / 100.0  # centipawns → score
    return features, target


class ChessDataset(Dataset):
    def __init__(self, positions, max_samples=100000):
        self.positions = positions[:max_samples]

    def __len__(self):
        return len(self.positions)

    def __getitem__(self, idx):
        fen, score, turn = self.positions[idx]
        features, target = encode_position(fen, score, turn)
        return torch.tensor(features), torch.tensor([target], dtype=torch.float32)


def generate_self_play_positions(num_games: int = 100):
    """Self-play positions with random moves to bootstrap training data."""
    import chess
    positions = []
    for _ in tqdm(range(num_games), desc="Self-play"):
        board = chess.Board()
        while not board.is_game_over() and len(positions) < 50000:
            if board.is_check():
                score = 0
            else:
                score = random.randint(-500, 500)
            positions.append((board.fen(), float(score), board.turn))
            moves = list(board.legal_moves)
            if not moves:
                break
            # Playout: mix of random and material-seeking moves
            if random.random() < 0.3:
                move = max(moves, key=lambda m: _material_gain(board, m))
            else:
                move = random.choice(moves)
            board.push(move)
    return positions


def _material_gain(board, move):
    """Quick material gain estimate for move selection."""
    gain = 0
    if board.is_capture(move):
        victim = board.piece_at(move.to_square)
        if victim:
            values = {'p': 1, 'n': 3, 'b': 3, 'r': 5, 'q': 9}
            gain = values.get(victim.symbol().lower(), 0) * 100
    if move.promotion:
        gain += 800
    return gain


def save_weights(model: NNUE, path: str):
    """Save trained weights in the format expected by nnue.go."""
    w1 = model.input_layer.weight.detach().numpy()
    b1 = model.input_layer.bias.detach().numpy()
    w2 = model.output_layer.weight.detach().numpy()
    b2 = model.output_layer.bias.detach().numpy()

    with open(path, 'wb') as f:
        # Header: input_size, hidden_size
        f.write(struct.pack('<II', INPUT_SIZE, HIDDEN_SIZE))
        # Weights[0]: input→hidden (InputSize × HiddenSize)
        for val in w1.flatten():
            f.write(struct.pack('<f', val))
        # Biases[0]: hidden
        for val in b1.flatten():
            f.write(struct.pack('<f', val))
        # Weights[1]: hidden→output
        for val in w2.flatten():
            f.write(struct.pack('<f', val))
        # Biases[1]: output
        for val in b2.flatten():
            f.write(struct.pack('<f', val))

    print(f"Weights saved to {path} ({INPUT_SIZE * HIDDEN_SIZE * 4 + HIDDEN_SIZE * 4 + HIDDEN_SIZE * 4 + 4 + 8} bytes)")


if __name__ == "__main__":
    import argparse
    parser = argparse.ArgumentParser()
    parser.add_argument("--games", type=int, default=200, help="Self-play games")
    parser.add_argument("--epochs", type=int, default=10)
    parser.add_argument("--lr", type=float, default=0.001)
    parser.add_argument("--batch-size", type=int, default=256)
    parser.add_argument("--output", type=str, default="services/realtime/nnue_weights.bin")
    args = parser.parse_args()

    print(f"Generating {args.games} self-play games...")
    positions = generate_self_play_positions(args.games)
    print(f"  {len(positions)} positions generated")

    dataset = ChessDataset(positions)
    loader = DataLoader(dataset, batch_size=args.batch_size, shuffle=True)

    model = NNUE()
    criterion = nn.MSELoss()
    optimizer = optim.Adam(model.parameters(), lr=args.lr)

    for epoch in range(args.epochs):
        total_loss = 0.0
        for batch_x, batch_y in tqdm(loader, desc=f"Epoch {epoch+1}"):
            optimizer.zero_grad()
            pred = model(batch_x)
            loss = criterion(pred, batch_y)
            loss.backward()
            optimizer.step()
            total_loss += loss.item()
        avg_loss = total_loss / len(loader)
        print(f"  Epoch {epoch+1}: loss={avg_loss:.6f}")

    save_weights(model, args.output)
    print("Done!")
