import type { PieceType, PieceColor, Piece, Sq, Board } from './types';
import { FILES, RANKS, OPP } from './constants';

// ─── Board helpers ────────────────────────────────────────────────────────────
export const makeBoard = (): Board => {
  const b: Board = Array.from({ length: 8 }, () => Array(8).fill(null));
  const backRank: PieceType[] = ['rook','knight','bishop','queen','king','bishop','knight','rook'];
  backRank.forEach((type, c) => {
    b[0][c] = { type, color: 'white' };
    b[7][c] = { type, color: 'black' };
  });
  for (let c = 0; c < 8; c++) {
    b[1][c] = { type: 'pawn', color: 'white' };
    b[6][c] = { type: 'pawn', color: 'black' };
  }
  return b;
};

export const cloneBoard = (b: Board): Board => b.map(r => [...r]);

export const b2s = (b: Board): string =>
  b.map(r => r.map(p => p ? `${p.color[0]}${p.type[0]}` : '-').join('')).join('|');

export const inB = (r: number, c: number): boolean => r >= 0 && r <= 7 && c >= 0 && c <= 7;

export const findKing = (b: Board, color: PieceColor): Sq | null => {
  for (let r = 0; r < 8; r++)
    for (let c = 0; c < 8; c++)
      if (b[r][c]?.type === 'king' && b[r][c]?.color === color) return { row: r, col: c };
  return null;
};

// ─── FEN ─────────────────────────────────────────────────────────────────────
export const PIECE_FEN_MAP: Readonly<Record<PieceType, string>> = {
  king:'k', queen:'q', rook:'r', bishop:'b', knight:'n', pawn:'p',
};

export const toFEN = (
  b: Board,
  turn: PieceColor,
  moved: Set<string>,
  lm: { from: Sq; to: Sq } | null,
  hmc: number,
  fmn: number,
): string => {
  let fen = '';
  for (let r = 7; r >= 0; r--) {
    let empty = 0;
    for (let c = 0; c < 8; c++) {
      const p = b[r][c];
      if (!p) { empty++; continue; }
      if (empty) { fen += empty; empty = 0; }
      const ch = PIECE_FEN_MAP[p.type];
      fen += p.color === 'white' ? ch.toUpperCase() : ch;
    }
    if (empty) fen += empty;
    if (r > 0) fen += '/';
  }

  fen += ` ${turn === 'white' ? 'w' : 'b'} `;

  let castling = '';
  if (!moved.has('0-4') && b[0][4]?.type === 'king') {
    if (!moved.has('0-7') && b[0][7]?.type === 'rook') castling += 'K';
    if (!moved.has('0-0') && b[0][0]?.type === 'rook') castling += 'Q';
  }
  if (!moved.has('7-4') && b[7][4]?.type === 'king') {
    if (!moved.has('7-7') && b[7][7]?.type === 'rook') castling += 'k';
    if (!moved.has('7-0') && b[7][0]?.type === 'rook') castling += 'q';
  }
  fen += castling || '-';

  let ep = '-';
  if (lm) {
    const lp = b[lm.to.row][lm.to.col];
    if (lp?.type === 'pawn' && Math.abs(lm.from.row - lm.to.row) === 2)
      ep = FILES[lm.to.col] + RANKS[(lm.from.row + lm.to.row) / 2];
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
    const promoMap: Record<string, string> = { q:'Q', r:'R', b:'B', n:'N' };
    const capture = board[tr][tc] ? `${FILES[fc]}x` : '';
    return `${capture}${toSq}=${promoMap[uci[4]] ?? uci[4].toUpperCase()}`;
  }
  const capture = board[tr][tc] ? 'x' : '';
  if (piece.type === 'pawn') { return tc !== fc ? `${FILES[fc]}x${toSq}` : toSq; }
  const pieceChar = piece.type === 'knight' ? 'N' : piece.type[0].toUpperCase();
  return `${pieceChar}${capture}${toSq}`;
};

// ─── Chess logic ──────────────────────────────────────────────────────────────
export const attacks = (b: Board, fr: number, fc: number, tr: number, tc: number, p: Piece): boolean => {
  const dr = tr - fr;
  const dc = tc - fc;

  switch (p.type) {
    case 'pawn':
      return dr === (p.color === 'white' ? 1 : -1) && Math.abs(dc) === 1;

    case 'knight':
      return (Math.abs(dr) === 2 && Math.abs(dc) === 1)
          || (Math.abs(dr) === 1 && Math.abs(dc) === 2);

    case 'king':
      return Math.abs(dr) <= 1 && Math.abs(dc) <= 1 && (dr !== 0 || dc !== 0);

    case 'rook': {
      if (dr !== 0 && dc !== 0) return false;
      const sR = dr === 0 ? 0 : dr > 0 ? 1 : -1;
      const sC = dc === 0 ? 0 : dc > 0 ? 1 : -1;
      for (let r = fr + sR, c = fc + sC; r !== tr || c !== tc; r += sR, c += sC)
        if (b[r][c]) return false;
      return true;
    }

    case 'bishop': {
      if (Math.abs(dr) !== Math.abs(dc)) return false;
      const sR = dr > 0 ? 1 : -1;
      const sC = dc > 0 ? 1 : -1;
      for (let r = fr + sR, c = fc + sC; r !== tr; r += sR, c += sC)
        if (b[r][c]) return false;
      return true;
    }

    case 'queen':
      return attacks(b, fr, fc, tr, tc, { ...p, type: 'rook' })
          || attacks(b, fr, fc, tr, tc, { ...p, type: 'bishop' });
  }
};

export const isAttacked = (b: Board, r: number, c: number, by: PieceColor): boolean => {
  for (let i = 0; i < 8; i++)
    for (let j = 0; j < 8; j++) {
      const p = b[i][j];
      if (p?.color === by && attacks(b, i, j, r, c, p)) return true;
    }
  return false;
};

export const KNIGHT_DELTAS: [number, number][] = [[-2,-1],[-2,1],[-1,-2],[-1,2],[1,-2],[1,2],[2,-1],[2,1]];
export const KING_DELTAS:   [number, number][] = [[0,1],[0,-1],[1,0],[-1,0],[1,1],[1,-1],[-1,1],[-1,-1]];

export const pseudoMoves = (
  b: Board,
  row: number,
  col: number,
  lm: { from: Sq; to: Sq } | null,
  mv: Set<string>,
): Sq[] => {
  const p = b[row][col];
  if (!p || p.frozen) return [];

  const { type, color } = p;
  const opp = OPP[color];
  const moves: Sq[] = [];
  const canTarget = (r: number, c: number) => inB(r, c) && b[r][c]?.color !== color;

  const slide = (dirs: [number, number][]) => {
    for (const [dr, dc] of dirs) {
      for (let i = 1; i <= 7; i++) {
        const r = row + dr * i;
        const c = col + dc * i;
        if (!inB(r, c) || b[r][c]?.color === color) break;
        moves.push({ row: r, col: c });
        if (b[r][c]) break;
      }
    }
  };

  if (type === 'pawn') {
    const dir = color === 'white' ? 1 : -1;
    const startRow = color === 'white' ? 1 : 6;
    if (inB(row + dir, col) && !b[row + dir][col]) {
      moves.push({ row: row + dir, col });
      if (row === startRow && !b[row + 2 * dir][col])
        moves.push({ row: row + 2 * dir, col });
    }
    for (const dc of [-1, 1]) {
      const nr = row + dir, nc = col + dc;
      if (inB(nr, nc) && b[nr][nc]?.color === opp) moves.push({ row: nr, col: nc });
    }
    if (lm) {
      const lp = b[lm.to.row][lm.to.col];
      if (
        lp?.type === 'pawn' &&
        Math.abs(lm.from.row - lm.to.row) === 2 &&
        lm.to.row === row &&
        Math.abs(lm.to.col - col) === 1
      ) {
        moves.push({ row: row + dir, col: lm.to.col });
      }
    }
  } else if (type === 'knight') {
    for (const [dr, dc] of KNIGHT_DELTAS) {
      if (canTarget(row + dr, col + dc)) moves.push({ row: row + dr, col: col + dc });
    }
  } else if (type === 'bishop') {
    slide([[1,1],[1,-1],[-1,1],[-1,-1]]);
  } else if (type === 'rook') {
    slide([[0,1],[0,-1],[1,0],[-1,0]]);
  } else if (type === 'queen') {
    slide([[0,1],[0,-1],[1,0],[-1,0],[1,1],[1,-1],[-1,1],[-1,-1]]);
  } else if (type === 'king') {
    for (const [dr, dc] of KING_DELTAS) {
      if (canTarget(row + dr, col + dc)) moves.push({ row: row + dr, col: col + dc });
    }
    if (!mv.has(`${row}-${col}`) && !isAttacked(b, row, col, opp)) {
      if (
        !mv.has(`${row}-7`) && b[row][7]?.type === 'rook' &&
        !b[row][5] && !b[row][6] &&
        !isAttacked(b, row, 5, opp) && !isAttacked(b, row, 6, opp)
      ) moves.push({ row, col: 6 });
      if (
        !mv.has(`${row}-0`) && b[row][0]?.type === 'rook' &&
        !b[row][1] && !b[row][2] && !b[row][3] &&
        !isAttacked(b, row, 3, opp) && !isAttacked(b, row, 2, opp)
      ) moves.push({ row, col: 2 });
    }
  }

  return moves;
};

export const legalMoves = (
  b: Board,
  row: number,
  col: number,
  lm: { from: Sq; to: Sq } | null,
  mv: Set<string>,
): Sq[] => {
  const p = b[row][col];
  if (!p) return [];

  return pseudoMoves(b, row, col, lm, mv).filter(m => {
    const nb = cloneBoard(b);
    nb[m.row][m.col] = nb[row][col];
    nb[row][col] = null;
    if (p.type === 'pawn' && m.col !== col && !b[m.row][m.col]) nb[row][m.col] = null;
    const kp = findKing(nb, p.color);
    if (!kp) return false;
    return !isAttacked(nb, kp.row, kp.col, OPP[p.color]);
  });
};

export const anyLegal = (
  b: Board,
  color: PieceColor,
  lm: { from: Sq; to: Sq } | null,
  mv: Set<string>,
): boolean => {
  for (let r = 0; r < 8; r++)
    for (let c = 0; c < 8; c++)
      if (b[r][c]?.color === color && legalMoves(b, r, c, lm, mv).length > 0) return true;
  return false;
};

export const gameStatus = (
  b: Board,
  player: PieceColor,
  lm: { from: Sq; to: Sq } | null,
  mv: Set<string>,
) => {
  const kp = findKing(b, player);
  if (!kp) return { isCheck: false, isMate: false, isStale: false };
  const inCheck = isAttacked(b, kp.row, kp.col, OPP[player]);
  const hasLegal = anyLegal(b, player, lm, mv);
  return {
    isCheck: inCheck,
    isMate:  inCheck && !hasLegal,
    isStale: !inCheck && !hasLegal,
  };
};

export const insuffMat = (b: Board): boolean => {
  const nonKings: Piece[] = [];
  for (let r = 0; r < 8; r++)
    for (let c = 0; c < 8; c++) {
      const p = b[r][c];
      if (p && p.type !== 'king') nonKings.push(p);
    }
  if (nonKings.length === 0) return true;
  if (nonKings.length === 1) return nonKings[0].type === 'bishop' || nonKings[0].type === 'knight';
  if (nonKings.length === 2) {
    const types = nonKings.map(p => p.type).sort();
    return (
      (types[0] === 'knight' && types[1] === 'knight') ||
      (types[0] === 'bishop' && types[1] === 'bishop') ||
      (types[0] === 'bishop' && types[1] === 'knight')
    );
  }
  return false;
};

export const positionKey = (
  b: Board,
  turn: PieceColor,
  moved: Set<string>,
  lm: { from: Sq; to: Sq } | null,
): string => {
  let castling = '';
  if (!moved.has('0-4') && b[0][4]?.type === 'king') {
    if (!moved.has('0-7') && b[0][7]?.type === 'rook') castling += 'K';
    if (!moved.has('0-0') && b[0][0]?.type === 'rook') castling += 'Q';
  }
  if (!moved.has('7-4') && b[7][4]?.type === 'king') {
    if (!moved.has('7-7') && b[7][7]?.type === 'rook') castling += 'k';
    if (!moved.has('7-0') && b[7][0]?.type === 'rook') castling += 'q';
  }
  let ep = '-';
  if (lm) {
    const lp = b[lm.to.row][lm.to.col];
    if (lp?.type === 'pawn' && Math.abs(lm.from.row - lm.to.row) === 2)
      ep = FILES[lm.to.col] + RANKS[(lm.from.row + lm.to.row) / 2];
  }
  return `${b2s(b)}|${turn[0]}|${castling || '-'}|${ep}`;
};

export const threefold = (hist: string[], cur: string): boolean =>
  hist.filter(p => p === cur).length >= 3;

export const moveNotation = (
  b: Board,
  fr: number,
  fc: number,
  tr: number,
  tc: number,
  p: Piece,
  cap: boolean,
): string => {
  if (p.type === 'king' && Math.abs(tc - fc) === 2) return tc === 6 ? 'O-O' : 'O-O-O';
  const toSq = FILES[tc] + RANKS[tr];
  let notation = '';
  if (p.type !== 'pawn') {
    notation += p.type === 'knight' ? 'N' : p.type[0].toUpperCase();
    const ambiguous: { r: number; c: number }[] = [];
    for (let r = 0; r < 8; r++)
      for (let c = 0; c < 8; c++) {
        const q = b[r][c];
        if (q?.type === p.type && q.color === p.color && !(r === fr && c === fc)) {
          if (pseudoMoves(b, r, c, null, new Set()).some(m => m.row === tr && m.col === tc))
            ambiguous.push({ r, c });
        }
      }
    if (ambiguous.length > 0) {
      if (!ambiguous.some(a => a.c === fc))      notation += FILES[fc];
      else if (!ambiguous.some(a => a.r === fr)) notation += RANKS[fr];
      else                                        notation += FILES[fc] + RANKS[fr];
    }
  }
  if (p.type === 'pawn' && cap) notation += FILES[fc];
  if (cap) notation += 'x';
  return notation + toSq;
};

