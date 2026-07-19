'use client';

import React from 'react';
import type { Board, PieceColor, Piece, Sq, PieceType, GameCard, CardMechanic, CardPendingState, DoubleMove, LavaSquare, BombPiece, Rarity, Snapshot } from '../types';
import type { MatchFinishReason, MatchModeId, MatchSnapshotMessage, PlayerIntent } from '@chess404/contracts';
import { DEFAULT_MATCH_MODE_ID } from '@chess404/contracts';
import type { StoredRoomMeta } from '../lib/match-service';
import { applyIntent, fetchMatch, readStoredRoomMeta, sendIntentViaWs, writeStoredRoomMeta } from '../lib/match-service';
import {
  readStoredGuestIdentity,
  readStoredAccountIdentity,
  writeStoredActiveMatchId,
  readStoredActiveMatchId,
} from '../lib/session-storage';
import { cloneBoard, positionKey, toFEN, gameStatus, insuffMat } from '../chessEngine';
import { OPP } from '../constants';
import type { GuestProfile, MatchSeatClaim } from '../lib/platform-service';

type GameSetters = Record<string, (...args: any[]) => void> & {
  setBoard: React.Dispatch<React.SetStateAction<Board>>;
  setTurn: React.Dispatch<React.SetStateAction<PieceColor>>;
  setMoved: React.Dispatch<React.SetStateAction<Set<string>>>;
  setLm: React.Dispatch<React.SetStateAction<{ from: Sq; to: Sq } | null>>;
  setHmc: React.Dispatch<React.SetStateAction<number>>;
  setFmn: React.Dispatch<React.SetStateAction<number>>;
  setCheck: React.Dispatch<React.SetStateAction<boolean>>;
  setMate: React.Dispatch<React.SetStateAction<boolean>>;
  setStale: React.Dispatch<React.SetStateAction<boolean>>;
  setInsuf: React.Dispatch<React.SetStateAction<boolean>>;
  setOver: React.Dispatch<React.SetStateAction<boolean>>;
  setWinner: (...args: any[]) => void;
  setDrawOffer: (...args: any[]) => void;
  setMovHist: React.Dispatch<React.SetStateAction<{ n: string; w?: string; b?: string }[]>>;
  setPosHist: React.Dispatch<React.SetStateAction<string[]>>;
  setSnapshots: React.Dispatch<React.SetStateAction<Snapshot[]>>;
  setChatMessages: React.Dispatch<React.SetStateAction<{ sender: 'white' | 'black'; text: string }[]>>;
  setReviewIdx: React.Dispatch<React.SetStateAction<number>>;
  setReviewBoard: React.Dispatch<React.SetStateAction<Board | null>>;
  setTimeW: React.Dispatch<React.SetStateAction<number>>;
  setTimeB: React.Dispatch<React.SetStateAction<number>>;
  setClockActive: React.Dispatch<React.SetStateAction<boolean>>;
  setTicking: (...args: any[]) => void;
};

type CardSetters = {
  setWhiteHand: React.Dispatch<React.SetStateAction<GameCard[]>>;
  setBlackHand: React.Dispatch<React.SetStateAction<GameCard[]>>;
  setSelectedCard: React.Dispatch<React.SetStateAction<GameCard | null>>;
  setLastDrawAnim: React.Dispatch<React.SetStateAction<{ color: PieceColor; rarity: Rarity } | null>>;
  setCardPending: React.Dispatch<React.SetStateAction<CardPendingState>>;
  setPromoPicker: React.Dispatch<React.SetStateAction<{ sq: Sq; options: PieceType[]; mechanic: CardMechanic } | null>>;
  setCardUsedBy: React.Dispatch<React.SetStateAction<{ white: boolean; black: boolean }>>;
  setDealPhase: React.Dispatch<React.SetStateAction<'idle' | 'dealing' | 'done'>>;
};

type AuthSetters = {
  setViewerSeat: React.Dispatch<React.SetStateAction<PieceColor | null>>;
  setMatchSeatMeta: React.Dispatch<React.SetStateAction<{ whiteGuestId?: string; blackGuestId?: string; whiteName?: string; blackName?: string } | null>>;
};

type OtherSetters = {
  setLavaSquares: React.Dispatch<React.SetStateAction<LavaSquare[]>>;
  setBombPieces: React.Dispatch<React.SetStateAction<BombPiece[]>>;
  setFogZones: React.Dispatch<React.SetStateAction<{ centerRow: number; centerCol: number; ownerColor: PieceColor; turnsLeft: number }[]>>;
  setDoubleMove: React.Dispatch<React.SetStateAction<DoubleMove | null>>;
  setGhostPiece: React.Dispatch<React.SetStateAction<{ row: number; col: number; piece: Piece; ownerColor: PieceColor; roundsLeft: number } | null>>;
  setRadarActive: React.Dispatch<React.SetStateAction<boolean>>;
  setCheaterTurnsLeft: React.Dispatch<React.SetStateAction<number>>;
  setCheaterColor: React.Dispatch<React.SetStateAction<PieceColor | null>>;
};

type AuthoritativeSetters = {
  setAuthoritativeMatchId: React.Dispatch<React.SetStateAction<string | null>>;
  setAuthoritativeLive: React.Dispatch<React.SetStateAction<boolean>>;
  setAuthoritativeStatus: React.Dispatch<React.SetStateAction<'waiting' | 'active' | 'finished' | null>>;
  setAuthoritativeFinishReason: React.Dispatch<React.SetStateAction<MatchFinishReason | null>>;
  setAuthoritativeWhiteConnected: React.Dispatch<React.SetStateAction<boolean>>;
  setAuthoritativeBlackConnected: React.Dispatch<React.SetStateAction<boolean>>;
  setAuthoritativeDisconnectGraceFor: React.Dispatch<React.SetStateAction<PieceColor | null>>;
  setAuthoritativeDisconnectGraceDeadline: React.Dispatch<React.SetStateAction<string | null>>;
};

type GameRefs = {
  turnRef: React.MutableRefObject<PieceColor>;
  cheaterColorRef: React.MutableRefObject<PieceColor | null>;
  authoritativeSeatIdsRef: React.MutableRefObject<{ white: string | null; black: string | null }>;
  authoritativeSeatSecretsRef: React.MutableRefObject<{ white: string | null; black: string | null }>;
  authoritativeClaimTokensRef: React.MutableRefObject<{ white: string | null; black: string | null }>;
  cardUsedByRef: React.MutableRefObject<{ white: boolean; black: boolean }>;
  pendingCardUseRef: React.MutableRefObject<Set<string>>;
};

export function useGameState(
  gameSetters: GameSetters,
  cardSetters: CardSetters,
  authSetters: AuthSetters,
  otherSetters: OtherSetters,
  authoritativeSetters: AuthoritativeSetters,
  refs: GameRefs,
  hostedRuntime: boolean | null,
  whiteProfileRef: React.MutableRefObject<GuestProfile | null>,
  blackProfileRef: React.MutableRefObject<GuestProfile | null>,
  requestedMatchIdRef: React.MutableRefObject<string | null>,
  gatewayRecoveredMatchIdRef: React.MutableRefObject<string | null>,
) {
  const finalizedResultRef = React.useRef<string | null>(null);
  const lastAppliedSeqNumRef = React.useRef<number>(0);
  const finalPositionRef = React.useRef<{ fen: string; turn: PieceColor } | null>(null);
  const authoritativeBootstrapRef = React.useRef(0);

  const buildMoveRows = React.useCallback((history: string[]) => {
    const rows: { n: string; w?: string; b?: string }[] = [];
    for (let i = 0; i < history.length; i += 2) {
      rows.push({ n: `${Math.floor(i / 2) + 1}.`, w: history[i], b: history[i + 1] });
    }
    return rows;
  }, []);

  const buildPendingCardFromSnapshot = React.useCallback((
    pending: import('@chess404/contracts').MatchState['pendingCard'],
    whiteCards: GameCard[],
    blackCards: GameCard[],
  ): CardPendingState => {
    if (!pending || pending.mechanic === 'joker') return null;
    const ownerCards = pending.ownerColor === 'white' ? whiteCards : blackCards;
    const card = ownerCards.find(item => item.id === pending.cardId);
    if (!card) return null;
    return {
      card,
      playerColor: pending.ownerColor as PieceColor,
      mechanic: pending.mechanic as CardMechanic,
      step: pending.target ? 2 : 1,
      data: {
        sq: pending.target ?? undefined,
        from: pending.mechanic === 'teleport' || pending.mechanic === 'jump' || pending.mechanic === 'clone' ? (pending.target ?? undefined) : undefined,
        sq1: ['swapme', 'swapus', 'swaphim', 'halffuse', 'fullfusion'].includes(pending.mechanic) ? (pending.target ?? undefined) : undefined,
        hostSq: pending.mechanic === 'parasite' ? (pending.target ?? undefined) : undefined,
        type1: pending.mechanic === 'halffuse' || pending.mechanic === 'fullfusion' ? (pending.options?.[0] as PieceType | undefined) : undefined,
        selected: pending.mechanic === 'smallsacrifice' || pending.mechanic === 'bigsacrifice'
          ? (pending.options ?? []).map(v => { const [r, c] = v.split(',').map(Number); return { row: r, col: c }; }).filter(sq => Number.isInteger(sq.row) && Number.isInteger(sq.col))
          : undefined,
        options: pending.options ?? undefined,
      },
    };
  }, []);

  const applyAuthoritativeSnapshot = React.useCallback((snapshot: MatchSnapshotMessage) => {
    if (snapshot.seqNum != null && snapshot.seqNum <= lastAppliedSeqNumRef.current) return;
    if (snapshot.seqNum != null) lastAppliedSeqNumRef.current = snapshot.seqNum;

    refs.cardUsedByRef.current = { white: false, black: false };
    cardSetters.setCardUsedBy({ white: false, black: false });
    cardSetters.setDealPhase('done');
    refs.pendingCardUseRef.current = new Set();

    const match = snapshot.match;
    const nextBoard = cloneBoard(match.board as unknown as Board);
    const nextMoved = new Set(match.moved);
    const nextLm = match.lastMove ? { from: match.lastMove.from as Sq, to: match.lastMove.to as Sq } : null;
    const nextTurn = match.turn as PieceColor;
    const nextHmc = match.halfMoveClock;
    const nextFmn = match.fullMoveNumber;
    const nextLavaSquares = (match.lavaSquares ?? []) as LavaSquare[];
    const nextBombPieces = (match.bombPieces ?? []) as BombPiece[];
    const nextFogZones = (match.fogZones ?? []).map(z => ({ centerRow: z.centerRow, centerCol: z.centerCol, ownerColor: z.ownerColor as PieceColor, turnsLeft: z.turnsLeft }));
    const nextDoubleMove = (match.doubleMove ?? null) as DoubleMove | null;
    const nextRadarRevealFor = (match.radarRevealFor ?? null) as PieceColor | null;
    const nextCheaterState = match.cheaterState ? { ownerColor: match.cheaterState.ownerColor as PieceColor, turnsLeft: match.cheaterState.turnsLeft } : null;
    const nextInvisiblePiece = match.invisiblePiece ? { row: match.invisiblePiece.row, col: match.invisiblePiece.col, piece: match.invisiblePiece.piece as Piece, ownerColor: match.invisiblePiece.ownerColor, roundsLeft: match.invisiblePiece.roundsLeft } : null;
    const nextTicking = match.clock.runningFor ?? null;
    const nextClockActive = nextTicking !== null && match.status === 'active';
    const nextPosKey = positionKey(nextBoard, nextTurn, nextMoved, nextLm);
    const nextFen = toFEN(nextBoard, nextTurn, nextMoved, nextLm, nextHmc, nextFmn);
    const nextStatus = gameStatus(nextBoard, nextTurn, nextLm, nextMoved);
    const nextInsuf = insuffMat(nextBoard);
    const nextOver = match.status === 'finished' || nextStatus.isMate || nextStatus.isStale || nextInsuf;
    const nextWinner = match.winner ?? (nextStatus.isMate ? OPP[nextTurn] : (nextStatus.isStale || nextInsuf ? 'draw' : null));

    const storedRoomMeta = readStoredRoomMeta(match.matchId);
    refs.authoritativeSeatIdsRef.current = { white: match.whiteGuestId ?? null, black: match.blackGuestId ?? null };
    const storedMeta = storedRoomMeta?.viewerSeat;
    const localWhite = whiteProfileRef.current?.guestId ?? readStoredGuestIdentity('white').guestId ?? null;
    const localBlack = blackProfileRef.current?.guestId ?? readStoredGuestIdentity('black').guestId ?? null;
    let derivedViewerSeat: PieceColor | null = null;
    if (hostedRuntime) {
      const matchWhiteOk = Boolean(localWhite) && match.whiteGuestId === localWhite;
      const matchBlackOk = Boolean(localBlack) && match.blackGuestId === localBlack;
      derivedViewerSeat = matchWhiteOk ? 'white' : matchBlackOk ? 'black' : storedMeta ?? null;
    }
    if (!derivedViewerSeat && storedMeta) derivedViewerSeat = storedMeta;
    authSetters.setViewerSeat(derivedViewerSeat);

    refs.authoritativeSeatSecretsRef.current = {
      white: storedRoomMeta?.whitePlayerSecret ?? refs.authoritativeSeatSecretsRef.current.white,
      black: storedRoomMeta?.blackPlayerSecret ?? refs.authoritativeSeatSecretsRef.current.black,
    };
    refs.authoritativeClaimTokensRef.current = {
      white: storedRoomMeta?.whiteClaimToken ?? refs.authoritativeClaimTokensRef.current.white,
      black: storedRoomMeta?.blackClaimToken ?? refs.authoritativeClaimTokensRef.current.black,
    };

    const nextRoomMeta: StoredRoomMeta = {
      ...storedRoomMeta,
      queue: match.queue ?? storedRoomMeta?.queue,
      modeId: match.modeId ?? storedRoomMeta?.modeId ?? DEFAULT_MATCH_MODE_ID,
      viewerSeat: derivedViewerSeat ?? storedRoomMeta?.viewerSeat ?? null,
      whiteGuestId: match.whiteGuestId ?? storedRoomMeta?.whiteGuestId,
      blackGuestId: match.blackGuestId ?? storedRoomMeta?.blackGuestId,
      whiteName: match.whiteName ?? storedRoomMeta?.whiteName ?? whiteProfileRef.current?.displayName,
      blackName: match.blackName ?? storedRoomMeta?.blackName ?? blackProfileRef.current?.displayName,
      whitePlayerSecret: refs.authoritativeSeatSecretsRef.current.white ?? storedRoomMeta?.whitePlayerSecret,
      blackPlayerSecret: refs.authoritativeSeatSecretsRef.current.black ?? storedRoomMeta?.blackPlayerSecret,
      whiteClaimToken: refs.authoritativeClaimTokensRef.current.white ?? storedRoomMeta?.whiteClaimToken,
      blackClaimToken: refs.authoritativeClaimTokensRef.current.black ?? storedRoomMeta?.blackClaimToken,
    };
    if (hostedRuntime && derivedViewerSeat === 'white') {
      delete (nextRoomMeta as any).blackPlayerSecret;
      delete (nextRoomMeta as any).blackClaimToken;
    } else if (hostedRuntime && derivedViewerSeat === 'black') {
      delete (nextRoomMeta as any).whitePlayerSecret;
      delete (nextRoomMeta as any).whiteClaimToken;
    }
    writeStoredRoomMeta(match.matchId, nextRoomMeta);
    authSetters.setMatchSeatMeta({
      whiteGuestId: match.whiteGuestId ?? nextRoomMeta.whiteGuestId,
      blackGuestId: match.blackGuestId ?? nextRoomMeta.blackGuestId,
      whiteName: match.whiteName ?? nextRoomMeta.whiteName,
      blackName: match.blackName ?? nextRoomMeta.blackName,
    });

    authoritativeSetters.setAuthoritativeMatchId(match.matchId);
    authoritativeSetters.setAuthoritativeLive(true);
    authoritativeSetters.setAuthoritativeStatus(match.status);
    authoritativeSetters.setAuthoritativeFinishReason((match.finishReason as MatchFinishReason | undefined) ?? null);
    authoritativeSetters.setAuthoritativeWhiteConnected(Boolean(match.whiteConnected));
    authoritativeSetters.setAuthoritativeBlackConnected(Boolean(match.blackConnected));
    authoritativeSetters.setAuthoritativeDisconnectGraceFor((match.disconnectGraceFor as PieceColor | undefined) ?? null);
    authoritativeSetters.setAuthoritativeDisconnectGraceDeadline(match.disconnectGraceDeadline ?? null);

    cardSetters.setWhiteHand(match.whiteHand as GameCard[]);
    cardSetters.setBlackHand(match.blackHand as GameCard[]);

    const drawEvent = [...(snapshot.events ?? [])].reverse().find(e => {
      const p = e.payload as any;
      return p?.roundDrawWhite?.length || p?.roundDrawBlack?.length;
    });
    const drawPayload = drawEvent?.payload as any;
    const drawColor: PieceColor | null = drawPayload?.roundDrawWhite?.[0] ? 'white' : drawPayload?.roundDrawBlack?.[0] ? 'black' : null;
    const drawCard = drawPayload?.roundDrawWhite?.[0] ?? drawPayload?.roundDrawBlack?.[0];
    if (drawCard && drawColor) {
      cardSetters.setLastDrawAnim({ color: drawColor, rarity: drawCard.rarity as Rarity });
      setTimeout(() => cardSetters.setLastDrawAnim(null), 2000);
    }

    gameSetters.setBoard(nextBoard);
    gameSetters.setTurn(nextTurn);
    gameSetters.setMoved(nextMoved);
    gameSetters.setLm(nextLm);
    gameSetters.setHmc(nextHmc);
    gameSetters.setFmn(nextFmn);
    otherSetters.setLavaSquares(nextLavaSquares);
    otherSetters.setBombPieces(nextBombPieces);
    otherSetters.setFogZones(nextFogZones);
    otherSetters.setDoubleMove(nextDoubleMove);
    otherSetters.setGhostPiece(nextInvisiblePiece);
    otherSetters.setRadarActive(Boolean(nextRadarRevealFor));
    otherSetters.setCheaterTurnsLeft(nextCheaterState?.turnsLeft ?? 0);
    otherSetters.setCheaterColor(nextCheaterState?.ownerColor ?? null);
    refs.cheaterColorRef.current = nextCheaterState?.ownerColor ?? null;
    gameSetters.setDrawOffer(match.drawOfferedBy ?? null);
    gameSetters.setMovHist(buildMoveRows(match.moveHistory));
    gameSetters.setChatMessages(match.chatMessages.map(msg => ({ sender: msg.sender as any, text: msg.text })));
    cardSetters.setCardPending(buildPendingCardFromSnapshot(match.pendingCard ?? null, match.whiteHand as GameCard[], match.blackHand as GameCard[]));

    if (match.pendingCard?.target && match.pendingCard.options?.length && ['promote', 'demote', 'promotehim', 'demotehim'].includes(match.pendingCard.mechanic)) {
      cardSetters.setPromoPicker({
        sq: match.pendingCard.target as Sq,
        options: match.pendingCard.options as PieceType[],
        mechanic: (match.pendingCard.mechanic === 'promotehim' ? 'promote' : match.pendingCard.mechanic === 'demotehim' ? 'demote' : match.pendingCard.mechanic) as CardMechanic,
      });
    } else {
      cardSetters.setPromoPicker(null);
    }

    cardSetters.setSelectedCard(prev => {
      if (!prev) return null;
      return [...(match.whiteHand as GameCard[]), ...(match.blackHand as GameCard[])].find(c => c.id === prev.id) ?? null;
    });

    gameSetters.setPosHist(prev => {
      if (prev.length === 0) return match.moveHistory.length > 0 ? [nextPosKey] : [];
      const capped = prev.slice(0, Math.max(1, match.moveHistory.length));
      if (capped.length === 0 || capped[capped.length - 1] !== nextPosKey) capped.push(nextPosKey);
      return capped;
    });

    gameSetters.setSnapshots(prev => {
      const ns: Snapshot = { board: cloneBoard(nextBoard), turn: nextTurn, moved: new Set(nextMoved), lm: nextLm, hmc: nextHmc, fmn: nextFmn, fen: nextFen };
      if (match.moveHistory.length === 0) return [];
      if (prev.length >= match.moveHistory.length) return [...prev.slice(0, match.moveHistory.length - 1), ns];
      return [...prev, ns];
    });

    gameSetters.setCheck(nextStatus.isCheck);
    gameSetters.setMate(nextStatus.isMate);
    gameSetters.setStale(nextStatus.isStale);
    gameSetters.setInsuf(nextInsuf);
    gameSetters.setOver(nextOver);
    gameSetters.setWinner(nextWinner);
    gameSetters.setReviewIdx(-1);
    gameSetters.setReviewBoard(null);
    gameSetters.setTimeW(Math.max(0, Math.ceil(match.clock.whiteMs / 1000)));
    gameSetters.setTimeB(Math.max(0, Math.ceil(match.clock.blackMs / 1000)));
    gameSetters.setClockActive(nextClockActive);
    gameSetters.setTicking(nextTicking);
    finalPositionRef.current = nextOver ? { fen: nextFen, turn: nextTurn } : null;
  }, [buildMoveRows, buildPendingCardFromSnapshot, gameSetters, cardSetters, authSetters, otherSetters, authoritativeSetters, refs, hostedRuntime, whiteProfileRef, blackProfileRef]);

  const bootstrapAuthoritativeMatch = React.useCallback(async () => {
    const bootstrapId = authoritativeBootstrapRef.current + 1;
    authoritativeBootstrapRef.current = bootstrapId;
    try {
      const explicitMatchId = requestedMatchIdRef.current;
      const restoredMatchId = explicitMatchId ?? (hostedRuntime ? gatewayRecoveredMatchIdRef.current : readStoredActiveMatchId());
      if (hostedRuntime && !explicitMatchId && !restoredMatchId) {
        authoritativeSetters.setAuthoritativeMatchId(null);
        authoritativeSetters.setAuthoritativeLive(false);
        authoritativeSetters.setAuthoritativeStatus(null);
        authoritativeSetters.setAuthoritativeFinishReason(null);
        authoritativeSetters.setAuthoritativeWhiteConnected(false);
        authoritativeSetters.setAuthoritativeBlackConnected(false);
        authoritativeSetters.setAuthoritativeDisconnectGraceFor(null);
        authoritativeSetters.setAuthoritativeDisconnectGraceDeadline(null);
        writeStoredActiveMatchId(null);
        return;
      }
      const roomMeta = restoredMatchId ? readStoredRoomMeta(restoredMatchId) : null;
      if (roomMeta?.viewerSeat) authSetters.setViewerSeat(roomMeta.viewerSeat);
      let snapshot: MatchSnapshotMessage | undefined;
      if (restoredMatchId) {
        try { snapshot = await fetchMatch(restoredMatchId); } catch { snapshot = undefined; }
      }
      if (!snapshot) {
        authoritativeSetters.setAuthoritativeMatchId(null);
        authoritativeSetters.setAuthoritativeLive(false);
        authoritativeSetters.setAuthoritativeStatus(null);
        authoritativeSetters.setAuthoritativeFinishReason(null);
        authoritativeSetters.setAuthoritativeWhiteConnected(false);
        authoritativeSetters.setAuthoritativeBlackConnected(false);
        authoritativeSetters.setAuthoritativeDisconnectGraceFor(null);
        authoritativeSetters.setAuthoritativeDisconnectGraceDeadline(null);
        writeStoredActiveMatchId(null);
        return;
      }
      applyAuthoritativeSnapshot(snapshot);
    } catch (err) {
      console.error('[bootstrapAuthoritativeMatch] error:', err);
    }
  }, [applyAuthoritativeSnapshot, hostedRuntime, requestedMatchIdRef, authSetters, authoritativeSetters]);

  const submitAuthoritativeIntent = React.useCallback(async (intent: Omit<PlayerIntent, 'matchId'>, matchId: string) => {
    if (sendIntentViaWs(matchId, intent as PlayerIntent)) {
      return null as unknown as MatchSnapshotMessage;
    }
    const snapshot = await applyIntent(matchId, intent as PlayerIntent);
    applyAuthoritativeSnapshot(snapshot);
    return snapshot;
  }, [applyAuthoritativeSnapshot]);

  const authoritativePlayerIdForColor = React.useCallback((color: PieceColor): string => {
    return color === 'white' ? refs.authoritativeSeatIdsRef.current.white ?? '' : color === 'black' ? refs.authoritativeSeatIdsRef.current.black ?? '' : '';
  }, [refs.authoritativeSeatIdsRef]);
  const authoritativePlayerSecretForColor = React.useCallback((color: PieceColor): string | undefined => {
    return color === 'white' ? refs.authoritativeSeatSecretsRef.current.white ?? undefined : color === 'black' ? refs.authoritativeSeatSecretsRef.current.black ?? undefined : undefined;
  }, [refs.authoritativeSeatSecretsRef]);
  const authoritativePlayerClaimTokenForColor = React.useCallback((color: PieceColor): string | undefined => {
    return color === 'white' ? refs.authoritativeClaimTokensRef.current.white ?? undefined : color === 'black' ? refs.authoritativeClaimTokensRef.current.black ?? undefined : undefined;
  }, [refs.authoritativeClaimTokensRef]);
  const authoritativeActorForColor = React.useCallback((color: PieceColor): { playerId: string; playerSecret?: string; playerClaimToken?: string } => {
    return { playerId: authoritativePlayerIdForColor(color), playerSecret: authoritativePlayerSecretForColor(color), playerClaimToken: authoritativePlayerClaimTokenForColor(color) };
  }, [authoritativePlayerIdForColor, authoritativePlayerSecretForColor, authoritativePlayerClaimTokenForColor]);

  return {
    finalizedResultRef,
    finalPositionRef,
    lastAppliedSeqNumRef,
    authoritativeBootstrapRef,
    buildMoveRows,
    buildPendingCardFromSnapshot,
    applyAuthoritativeSnapshot,
    bootstrapAuthoritativeMatch,
    submitAuthoritativeIntent,
    authoritativePlayerIdForColor,
    authoritativePlayerSecretForColor,
    authoritativePlayerClaimTokenForColor,
    authoritativeActorForColor,
  };
}
