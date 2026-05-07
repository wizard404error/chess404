import type { Board, Piece, PieceColor, PieceType, Sq } from '@chess404/contracts';
import { FILES, OPP, RANKS } from './constants';

export const makeBoard = (): Board => {
  const board: Board = Array.from({ length: 8 }, () => Array(8).fill(null));
  const backRank: PieceType[] = ['rook', 'knight', 'bishop', 'queen', 'king', 'bishop', 'knight', 'rook'];

  backRank.forEach((type, c) => {
    board[0][c] = { type, color: 'white' };
    board[7][c] = { type, color: 'black' };
  });

  for (let c = 0; c < 8; c++) {
    board[1][c] = { type: 'pawn', color: 'white' };
    board[6][c] = { type: 'pawn', color: 'black' };
  }

  return board;
};

export const cloneBoard = (board: Board): Board => board.map((row) => [...row]);
export const b2s = (board: Board): string => board.map((row) => row.map((piece) => (piece ? `${piece.color[0]}${piece.type[0]}` : '-')).join('')).join('|');
export const inB = (r: number, c: number): boolean => r >= 0 && r <= 7 && c >= 0 && c <= 7;

export const findKing = (board: Board, color: PieceColor): Sq | null => {
  for (let r = 0; r < 8; r++) {
    for (let c = 0; c < 8; c++) {
      if (board[r][c]?.type === 'king' && board[r][c]?.color === color) {
        return { row: r, col: c };
      }
    }
  }

  return null;
};

export const PIECE_FEN_MAP: Readonly<Record<PieceType, string>> = {
  king: 'k',
  queen: 'q',
  rook: 'r',
  bishop: 'b',
  knight: 'n',
  pawn: 'p'
};

export const toFEN = (
  board: Board,
  turn: PieceColor,
  moved: Set<string>,
  lm: { from: Sq; to: Sq } | null,
  hmc: number,
  fmn: number
): string => {
  let fen = '';

  for (let r = 7; r >= 0; r--) {
    let empty = 0;
    for (let c = 0; c < 8; c++) {
      const piece = board[r][c];
      if (!piece) {
        empty++;
        continue;
      }
      if (empty) {
        fen += empty;
        empty = 0;
      }
      const ch = PIECE_FEN_MAP[piece.type];
      fen += piece.color === 'white' ? ch.toUpperCase() : ch;
    }
    if (empty) fen += empty;
    if (r > 0) fen += '/';
  }

  fen += ` ${turn === 'white' ? 'w' : 'b'} `;

  let castling = '';
  if (!moved.has('0-4') && board[0][4]?.type === 'king') {
    if (!moved.has('0-7') && board[0][7]?.type === 'rook') castling += 'K';
    if (!moved.has('0-0') && board[0][0]?.type === 'rook') castling += 'Q';
  }
  if (!moved.has('7-4') && board[7][4]?.type === 'king') {
    if (!moved.has('7-7') && board[7][7]?.type === 'rook') castling += 'k';
    if (!moved.has('7-0') && board[7][0]?.type === 'rook') castling += 'q';
  }
  fen += castling || '-';

  let ep = '-';
  if (lm) {
    const lastPiece = board[lm.to.row][lm.to.col];
    if (lastPiece?.type === 'pawn' && Math.abs(lm.from.row - lm.to.row) === 2) {
      ep = FILES[lm.to.col] + RANKS[(lm.from.row + lm.to.row) / 2];
    }
  }

  return `${fen} ${ep} ${hmc} ${fmn}`;
};

export const uciToSan = (uci: string, board: Board): string => {
  if (uci.length < 4) return uci;
  const fc = FILES.indexOf(uci[0] as typeof FILES[number]);
  const fr = RANKS.indexOf(uci[1] as typeof RANKS[number]);
  const tc = FILES.indexOf(uci[2] as typeof FILES[number]);
  const tr = RANKS.indexOf(uci[3] as typeof RANKS[number]);
  if (fc < 0 || fr < 0 || tc < 0 || tr < 0) return uci;
  const piece = board[fr][fc];
  if (!piece) return uci;
  const toSq = FILES[tc] + RANKS[tr];
  if (piece.type === 'king' && Math.abs(tc - fc) === 2) return tc > fc ? 'O-O' : 'O-O-O';
  if (piece.type === 'pawn' && uci[4]) {
    const promoMap: Record<string, string> = { q: 'Q', r: 'R', b: 'B', n: 'N' };
    const capture = board[tr][tc] ? `${FILES[fc]}x` : '';
    return `${capture}${toSq}=${promoMap[uci[4]] ?? uci[4].toUpperCase()}`;
  }
  const capture = board[tr][tc] ? 'x' : '';
  if (piece.type === 'pawn') return tc !== fc ? `${FILES[fc]}x${toSq}` : toSq;
  const pieceChar = piece.type === 'knight' ? 'N' : piece.type[0].toUpperCase();
  return `${pieceChar}${capture}${toSq}`;
};

export const attacks = (board: Board, fr: number, fc: number, tr: number, tc: number, piece: Piece): boolean => {
  const dr = tr - fr;
  const dc = tc - fc;

  switch (piece.type) {
    case 'pawn':
      return dr === (piece.color === 'white' ? 1 : -1) && Math.abs(dc) === 1;
    case 'knight':
      return (Math.abs(dr) === 2 && Math.abs(dc) === 1) || (Math.abs(dr) === 1 && Math.abs(dc) === 2);
    case 'king':
      return Math.abs(dr) <= 1 && Math.abs(dc) <= 1 && (dr !== 0 || dc !== 0);
    case 'rook': {
      if (dr !== 0 && dc !== 0) return false;
      const sR = dr === 0 ? 0 : dr > 0 ? 1 : -1;
      const sC = dc === 0 ? 0 : dc > 0 ? 1 : -1;
      for (let r = fr + sR, c = fc + sC; r !== tr || c !== tc; r += sR, c += sC) {
        if (board[r][c]) return false;
      }
      return true;
    }
    case 'bishop': {
      if (Math.abs(dr) !== Math.abs(dc)) return false;
      const sR = dr > 0 ? 1 : -1;
      const sC = dc > 0 ? 1 : -1;
      for (let r = fr + sR, c = fc + sC; r !== tr; r += sR, c += sC) {
        if (board[r][c]) return false;
      }
      return true;
    }
    case 'queen':
      return attacks(board, fr, fc, tr, tc, { ...piece, type: 'rook' }) || attacks(board, fr, fc, tr, tc, { ...piece, type: 'bishop' });
  }
};

export const isAttacked = (board: Board, r: number, c: number, by: PieceColor): boolean => {
  for (let i = 0; i < 8; i++) {
    for (let j = 0; j < 8; j++) {
      const piece = board[i][j];
      if (piece?.color === by && attacks(board, i, j, r, c, piece)) return true;
    }
  }
  return false;
};

export const KNIGHT_DELTAS: [number, number][] = [[-2, -1], [-2, 1], [-1, -2], [-1, 2], [1, -2], [1, 2], [2, -1], [2, 1]];
export const KING_DELTAS: [number, number][] = [[0, 1], [0, -1], [1, 0], [-1, 0], [1, 1], [1, -1], [-1, 1], [-1, -1]];

export const pseudoMoves = (board: Board, row: number, col: number, lm: { from: Sq; to: Sq } | null, mv: Set<string>): Sq[] => {
  const piece = board[row][col];
  if (!piece || piece.frozen) return [];

  const { type, color } = piece;
  const opponent = OPP[color];
  const moves: Sq[] = [];
  const canTarget = (r: number, c: number) => inB(r, c) && board[r][c]?.color !== color;

  const slide = (dirs: [number, number][]) => {
    for (const [dr, dc] of dirs) {
      for (let i = 1; i <= 7; i++) {
        const r = row + dr * i;
        const c = col + dc * i;
        if (!inB(r, c) || board[r][c]?.color === color) break;
        moves.push({ row: r, col: c });
        if (board[r][c]) break;
      }
    }
  };

  if (type === 'pawn') {
    const dir = color === 'white' ? 1 : -1;
    const startRow = color === 'white' ? 1 : 6;
    if (inB(row + dir, col) && !board[row + dir][col]) {
      moves.push({ row: row + dir, col });
      if (row === startRow && !board[row + 2 * dir][col]) moves.push({ row: row + 2 * dir, col });
    }
    for (const dc of [-1, 1]) {
      const nr = row + dir;
      const nc = col + dc;
      if (inB(nr, nc) && board[nr][nc]?.color === opponent) moves.push({ row: nr, col: nc });
    }
    if (lm) {
      const lastPiece = board[lm.to.row][lm.to.col];
      if (lastPiece?.type === 'pawn' && Math.abs(lm.from.row - lm.to.row) === 2 && lm.to.row === row && Math.abs(lm.to.col - col) === 1) {
        moves.push({ row: row + dir, col: lm.to.col });
      }
    }
  } else if (type === 'knight') {
    for (const [dr, dc] of KNIGHT_DELTAS) {
      if (canTarget(row + dr, col + dc)) moves.push({ row: row + dr, col: col + dc });
    }
  } else if (type === 'bishop') {
    slide([[1, 1], [1, -1], [-1, 1], [-1, -1]]);
  } else if (type === 'rook') {
    slide([[0, 1], [0, -1], [1, 0], [-1, 0]]);
  } else if (type === 'queen') {
    slide([[0, 1], [0, -1], [1, 0], [-1, 0], [1, 1], [1, -1], [-1, 1], [-1, -1]]);
  } else if (type === 'king') {
    for (const [dr, dc] of KING_DELTAS) {
      if (canTarget(row + dr, col + dc)) moves.push({ row: row + dr, col: col + dc });
    }
    if (!mv.has(`${row}-${col}`) && !isAttacked(board, row, col, opponent)) {
      if (!mv.has(`${row}-7`) && board[row][7]?.type === 'rook' && !board[row][5] && !board[row][6] && !isAttacked(board, row, 5, opponent) && !isAttacked(board, row, 6, opponent)) {
        moves.push({ row, col: 6 });
      }
      if (!mv.has(`${row}-0`) && board[row][0]?.type === 'rook' && !board[row][1] && !board[row][2] && !board[row][3] && !isAttacked(board, row, 3, opponent) && !isAttacked(board, row, 2, opponent)) {
        moves.push({ row, col: 2 });
      }
    }
  }

  return moves;
};

export const legalMoves = (board: Board, row: number, col: number, lm: { from: Sq; to: Sq } | null, mv: Set<string>): Sq[] => {
  const piece = board[row][col];
  if (!piece) return [];

  return pseudoMoves(board, row, col, lm, mv).filter((move) => {
    const nextBoard = cloneBoard(board);
    nextBoard[move.row][move.col] = nextBoard[row][col];
    nextBoard[row][col] = null;
    if (piece.type === 'pawn' && move.col !== col && !board[move.row][move.col]) nextBoard[row][move.col] = null;
    const king = findKing(nextBoard, piece.color);
    if (!king) return false;
    return !isAttacked(nextBoard, king.row, king.col, OPP[piece.color]);
  });
};

export const anyLegal = (board: Board, color: PieceColor, lm: { from: Sq; to: Sq } | null, mv: Set<string>): boolean => {
  for (let r = 0; r < 8; r++) {
    for (let c = 0; c < 8; c++) {
      if (board[r][c]?.color === color && legalMoves(board, r, c, lm, mv).length > 0) return true;
    }
  }
  return false;
};

export const gameStatus = (board: Board, player: PieceColor, lm: { from: Sq; to: Sq } | null, mv: Set<string>) => {
  const king = findKing(board, player);
  if (!king) return { isCheck: false, isMate: false, isStale: false };
  const inCheck = isAttacked(board, king.row, king.col, OPP[player]);
  const hasLegal = anyLegal(board, player, lm, mv);
  return {
    isCheck: inCheck,
    isMate: inCheck && !hasLegal,
    isStale: !inCheck && !hasLegal
  };
};

export const insuffMat = (board: Board): boolean => {
  const nonKings: Piece[] = [];
  for (let r = 0; r < 8; r++) {
    for (let c = 0; c < 8; c++) {
      const piece = board[r][c];
      if (piece && piece.type !== 'king') nonKings.push(piece);
    }
  }
  if (nonKings.length === 0) return true;
  if (nonKings.length === 1) return nonKings[0].type === 'bishop' || nonKings[0].type === 'knight';
  if (nonKings.length === 2) {
    const types = nonKings.map((piece) => piece.type).sort();
    return (
      (types[0] === 'knight' && types[1] === 'knight') ||
      (types[0] === 'bishop' && types[1] === 'bishop') ||
      (types[0] === 'bishop' && types[1] === 'knight')
    );
  }
  return false;
};

export const positionKey = (board: Board, turn: PieceColor, moved: Set<string>, lm: { from: Sq; to: Sq } | null): string => {
  let castling = '';
  if (!moved.has('0-4') && board[0][4]?.type === 'king') {
    if (!moved.has('0-7') && board[0][7]?.type === 'rook') castling += 'K';
    if (!moved.has('0-0') && board[0][0]?.type === 'rook') castling += 'Q';
  }
  if (!moved.has('7-4') && board[7][4]?.type === 'king') {
    if (!moved.has('7-7') && board[7][7]?.type === 'rook') castling += 'k';
    if (!moved.has('7-0') && board[7][0]?.type === 'rook') castling += 'q';
  }
  let ep = '-';
  if (lm) {
    const lastPiece = board[lm.to.row][lm.to.col];
    if (lastPiece?.type === 'pawn' && Math.abs(lm.from.row - lm.to.row) === 2) ep = FILES[lm.to.col] + RANKS[(lm.from.row + lm.to.row) / 2];
  }
  return `${b2s(board)}|${turn[0]}|${castling || '-'}|${ep}`;
};

export const threefold = (history: string[], current: string): boolean => history.filter((position) => position === current).length >= 3;

export const moveNotation = (board: Board, fr: number, fc: number, tr: number, tc: number, piece: Piece, capture: boolean): string => {
  if (piece.type === 'king' && Math.abs(tc - fc) === 2) return tc === 6 ? 'O-O' : 'O-O-O';
  const toSq = FILES[tc] + RANKS[tr];
  let notation = '';
  if (piece.type !== 'pawn') {
    notation += piece.type === 'knight' ? 'N' : piece.type[0].toUpperCase();
    const ambiguous: { r: number; c: number }[] = [];
    for (let r = 0; r < 8; r++) {
      for (let c = 0; c < 8; c++) {
        const other = board[r][c];
        if (other?.type === piece.type && other.color === piece.color && !(r === fr && c === fc)) {
          if (pseudoMoves(board, r, c, null, new Set()).some((move) => move.row === tr && move.col === tc)) ambiguous.push({ r, c });
        }
      }
    }
    if (ambiguous.length > 0) {
      if (!ambiguous.some((a) => a.c === fc)) notation += FILES[fc];
      else if (!ambiguous.some((a) => a.r === fr)) notation += RANKS[fr];
      else notation += FILES[fc] + RANKS[fr];
    }
  }
  if (piece.type === 'pawn' && capture) notation += FILES[fc];
  if (capture) notation += 'x';
  return notation + toSq;
};
