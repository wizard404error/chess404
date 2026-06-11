'use client';
import React from 'react';
import { GamePanel } from './GamePanel';
import type { Board, PieceType, PieceColor, Sq, GameCard, CardPendingState, DoubleMove, BombPiece, LavaSquare, Rarity } from '../../types';
import { RARITY_STYLE, RARITY_WEIGHTS, OPP, FILES, RANKS, SQ, MAX_HAND_SIZE, ABORT_SECS, DRAW_FROM, DRAW_EVERY, INITIAL_DEAL_ROUND, PIECE_VALUE } from '../../constants';
import { findKing, positionKey, toFEN, uciToSan } from '../../chessEngine';
import { BoardCanvas, type TransformAnim, type SniperAnim, type TeleportAnim, type JumpAnim, type SacrificeAnim, type MindControlAnim, type FuseAnim, type BoardArrow } from '../../BoardCanvas';

const DRAW_COOLDOWN_MS = 15000;

const useFocusTrap = (ref: React.RefObject<HTMLElement | null>, active: boolean) => {
  React.useEffect(() => {
    if (!active || !ref.current) return;
    const el = ref.current;
    const focusable = el.querySelectorAll<HTMLElement>(
      'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])'
    );
    const first = focusable[0];
    const last = focusable[focusable.length - 1];
    if (first) first.focus();
    const handler = (e: KeyboardEvent) => {
      if (e.key !== 'Tab') return;
      if (e.shiftKey && document.activeElement === first) {
        e.preventDefault();
        last?.focus();
      } else if (!e.shiftKey && document.activeElement === last) {
        e.preventDefault();
        first?.focus();
      }
    };
    el.addEventListener('keydown', handler);
    return () => el.removeEventListener('keydown', handler);
  }, [active, ref]);
};

interface MatchBoardViewProps {
  board: Board;
  turn: PieceColor;
  sel: Sq | null;
  hints: Sq[];
  lm: { from: Sq; to: Sq } | null;
  drag: Sq | null;
  dragPos: { x: number; y: number } | null;
  check: boolean;
  kingPos: Sq | null;
  over: boolean;
  winner: PieceColor | 'draw' | 'aborted' | null;
  topSeat: PieceColor;
  bottomSeat: PieceColor;
  topPlayerName: string;
  bottomPlayerName: string;
  topSeatBadge: string | null;
  bottomSeatBadge: string | null;
  displayedWhiteRating: number;
  displayedBlackRating: number;
  displayedWhiteName: string;
  displayedBlackName: string;
  whiteSeatBadge: string | null;
  blackSeatBadge: string | null;
  timeW: number;
  timeB: number;
  clockActive: boolean;
  tickingState: PieceColor | null;
  fmtClock: (ms: number) => string;
  hostedRuntime: boolean | null;
  authoritativeMatchId: string | null;
  authoritativeMatchIdRef: React.MutableRefObject<string | null>;
  viewerSeat: PieceColor | null;
  controlSender: PieceColor;
  authoritativeLive: boolean;
  authoritativeStatus: string | null;
  engineOn: boolean;
  setEngineOn: (v: boolean | ((prev: boolean) => boolean)) => void;
  ev: { score: number; mate: number | null; best: string } | null;
  selectedCard: GameCard | null;
  cardPending: CardPendingState | null;
  whiteHand: GameCard[];
  blackHand: GameCard[];
  topHand: GameCard[];
  bottomHand: GameCard[];
  cardUsedBy: Record<string, boolean>;
  canUseCard: (card: GameCard, ownerColor: PieceColor) => boolean;
  applyCard: (card: GameCard, ownerColor: PieceColor) => void;
  cancelCard: () => void;
  cardMsg: string;
  setCardMsg: (msg: string) => void;
  streamDisconnected: boolean;
  onReconnect: () => void;
  clickSq: (r: number, c: number) => void;
  getMoves: (r: number, c: number) => Sq[];
  doMove: (fr: number, fc: number, tr: number, tc: number) => void;
  promo: { row: number; col: number; color: PieceColor } | null;
  doPromo: (type: PieceType) => void;
  promoPicker: { options: PieceType[] } | null;
  handlePromoPick: (type: PieceType) => void;
  cardPromo: { sq: { row: number; col: number }; color: PieceColor } | null;
  setCardPromo: (v: any) => void;
  getCardHighlight: (r: number, c: number) => string | null;
  getDoubleMoveHighlight: (r: number, c: number) => string | null;
  bombPieces: BombPiece[];
  bombExploding: Sq[];
  lavaSquares: LavaSquare[];
  lavaExploding: Sq[];
  swapAnim: { sq1: Sq; sq2: Sq; color1: string; color2: string } | null;
  transformAnim: TransformAnim | null;
  sniperAnim: SniperAnim | null;
  teleportAnim: TeleportAnim | null;
  jumpAnim: JumpAnim | null;
  sacrificeAnim: SacrificeAnim | null;
  mindControlAnim: MindControlAnim | null;
  fuseAnim: FuseAnim | null;
  fogZones: any[];
  ghostPiece: any;
  ghostRef: React.MutableRefObject<any>;
  analysisArrows: BoardArrow[];
  toggleAnalysisArrow: (from: Sq, to: Sq) => void;
  clearAnalysisArrows: () => void;
  premove: any;
  setPremove: (v: any) => void;
  isReviewing: boolean;
  reviewBoard: Board | null;
  reviewIdx: number;
  renderHand: (hand: GameCard[], seat: PieceColor, side: 'top' | 'bottom') => React.ReactNode;
  renderPlayerCard: (seat: PieceColor) => React.ReactNode;
  chatMessages: { sender: PieceColor; text: string }[];
  setChatMessages: (v: any | ((prev: any) => any)) => void;
  movHist: any[];
  submitAuthoritativeIntent: (intent: any) => void;
  authoritativeActorForColor: (color: PieceColor) => any;
  createAuthoritativeRematchRoom: () => void;
  hostedActionLocked: boolean;
  drawOffer: PieceColor | null;
  canRespondToDrawOffer: boolean;
  setDrawOffer: (v: PieceColor | null) => void;
  abortActive: boolean;
  abortCountdown: number;
  stopAbortCountdown: () => void;
  activeFinishReasonLabel: string | null;
  authoritativeRematchBusy: boolean;
  canCreateDirectRematch: boolean;
  canQueueSameLane: boolean;
  returnToSameQueueLane: () => void;
  returnToQueueHome: () => void;
  newGame: () => void;
  finishedPrimaryActionLabel: string;
  finishedSecondaryActionLabel: string;
  boardStatusLabel: string;
  roundNumber: number;
  lastDrawAnim: { rarity: Rarity } | null;
  soundEnabled: boolean;
  toggleSound: () => void;
  colorBlindMode: boolean;
  toggleColorBlind: () => void;
  showHostedReconnectWarning: boolean | null;
  intentInFlight: boolean;
  activeDisconnectGraceFor: PieceColor | null;
  bootstrapAuthoritativeMatch: () => void;
  showHostedSoloBanner: boolean | null;
  isAttackedWithFusion: (board: Board, r: number, c: number, byColor: PieceColor) => boolean;
  checkEndGame: (board: Board, turn: PieceColor, moved: Set<string>, lm: { from: Sq; to: Sq } | null, hmc: number, posHist: string[], posKey: string, fen: string, opp: PieceColor) => void;
  setSel: (v: Sq | null) => void;
  setHints: (v: Sq[]) => void;
  setDrag: (v: Sq | null) => void;
  setDragPos: (v: { x: number; y: number } | null) => void;
  setBoard: (b: Board) => void;
  setPosHist: (h: string[]) => void;
  setOver: (v: boolean) => void;
  setWinner: (v: PieceColor | 'draw' | 'aborted') => void;
  moved: any;
  hmc: number;
  fmn: number;
  posHist: string[];
  doubleMove: DoubleMove | null;
  radarActive: boolean;
  finalPositionRef: React.MutableRefObject<any>;
}

export function MatchBoardView(props: MatchBoardViewProps) {
  const {
    board,
    turn,
    sel,
    hints,
    lm,
    drag,
    dragPos,
    check,
    kingPos,
    over,
    winner,
    topSeat,
    bottomSeat,
    topPlayerName,
    bottomPlayerName,
    topSeatBadge,
    bottomSeatBadge,
    displayedWhiteRating,
    displayedBlackRating,
    displayedWhiteName,
    displayedBlackName,
    whiteSeatBadge,
    blackSeatBadge,
    timeW,
    timeB,
    clockActive,
    tickingState,
    fmtClock,
    hostedRuntime,
    authoritativeMatchId,
    authoritativeMatchIdRef,
    viewerSeat,
    controlSender,
    authoritativeLive,
    authoritativeStatus,
    engineOn,
    setEngineOn,
    ev,
    selectedCard,
    cardPending,
    whiteHand,
    blackHand,
    topHand,
    bottomHand,
    cardUsedBy,
    canUseCard,
    applyCard,
    cancelCard,
    cardMsg,
    setCardMsg,
    streamDisconnected,
    onReconnect,
    clickSq,
    getMoves,
    doMove,
    promo,
    doPromo,
    promoPicker,
    handlePromoPick,
    cardPromo,
    setCardPromo,
    getCardHighlight,
    getDoubleMoveHighlight,
    bombPieces,
    bombExploding,
    lavaSquares,
    lavaExploding,
    swapAnim,
    transformAnim,
    sniperAnim,
    teleportAnim,
    jumpAnim,
    sacrificeAnim,
    mindControlAnim,
    fuseAnim,
    fogZones,
    ghostPiece,
    ghostRef,
    analysisArrows,
    toggleAnalysisArrow,
    clearAnalysisArrows,
    premove,
    setPremove,
    isReviewing,
    reviewBoard,
    reviewIdx,
    renderHand,
    renderPlayerCard,
    chatMessages,
    setChatMessages,
    movHist,
    submitAuthoritativeIntent,
    authoritativeActorForColor,
    createAuthoritativeRematchRoom,
    hostedActionLocked,
    drawOffer,
    canRespondToDrawOffer,
    setDrawOffer,
    abortActive,
    abortCountdown,
    stopAbortCountdown,
    activeFinishReasonLabel,
    authoritativeRematchBusy,
    canCreateDirectRematch,
    canQueueSameLane,
    returnToSameQueueLane,
    returnToQueueHome,
    newGame,
    finishedPrimaryActionLabel,
    finishedSecondaryActionLabel,
    boardStatusLabel,
    roundNumber,
    lastDrawAnim,
    soundEnabled,
    toggleSound,
    colorBlindMode,
    toggleColorBlind,
    showHostedReconnectWarning,
    intentInFlight,
    activeDisconnectGraceFor,
    bootstrapAuthoritativeMatch,
    showHostedSoloBanner,
    isAttackedWithFusion,
    checkEndGame,
    setSel,
    setHints,
    setDrag,
    setDragPos,
    setBoard,
    setPosHist,
    setOver,
    setWinner,
    moved,
    hmc,
    fmn,
    posHist,
    doubleMove,
    radarActive,
    finalPositionRef,
  } = props;

  const boardWrapperRef = React.useRef<HTMLDivElement>(null);
  const [boardWrapperPx, setBoardWrapperPx] = React.useState(SQ * 8);
  const promoRef = React.useRef<HTMLDivElement>(null);
  const promoFullRef = React.useRef<HTMLDivElement>(null);
  const cardPromoRef = React.useRef<HTMLDivElement>(null);
  const [confirmResign, setConfirmResign] = React.useState<'idle' | 'prompting'>('idle');
  const lastDrawOfferTime = React.useRef(0);

  useFocusTrap(promoRef, promoPicker !== null);
  useFocusTrap(promoFullRef, promo !== null);
  useFocusTrap(cardPromoRef, cardPromo !== null);

  React.useEffect(() => {
    const el = boardWrapperRef.current;
    if (!el) return;
    const ro = new ResizeObserver((entries) => {
      for (const entry of entries) {
        const w = entry.contentBoxSize?.[0]?.inlineSize ?? entry.contentRect.width;
        if (w > 0) setBoardWrapperPx(Math.round(w));
      }
    });
    ro.observe(el);
    return () => ro.disconnect();
  }, []);

  return (
    <div className="match-layout">
      {/* ── Left column ── */}
      <div className="match-layout__left">
        {renderPlayerCard(topSeat)}
        {false && (
        <div style={{
          background: topSeat === 'white' ? 'rgba(8,45,18,0.50)' : 'rgba(40,10,80,0.50)',
          backdropFilter:'blur(16px)', WebkitBackdropFilter:'blur(16px)',
          border: topSeat === 'white' ? '1px solid rgba(60,220,110,0.45)' : '1px solid rgba(200,120,255,0.45)',
          borderRadius:'16px', padding:'12px 16px',
          display:'flex', alignItems:'center', gap:'12px',
          boxShadow: topSeat === 'white'
            ? '0 8px 32px rgba(0,0,0,0.35), inset 0 1px 0 rgba(80,240,130,0.2), 0 0 30px rgba(30,180,70,0.2)'
            : '0 8px 32px rgba(0,0,0,0.35), inset 0 1px 0 rgba(220,140,255,0.2), 0 0 30px rgba(160,60,240,0.2)',
        }}>
          <div style={{ width:'58px', height:'58px', borderRadius:'50%', flexShrink:0, background:'linear-gradient(135deg, #1a0a30, #0d0520)', border:'2px solid rgba(150,100,220,0.7)', display:'flex', alignItems:'center', justifyContent:'center', fontSize:'28px', boxShadow:'0 0 20px rgba(150,100,220,0.5)' }}>🕵️</div>
          <div style={{ flex:1, minWidth:0 }}>
            <div style={{ display:'flex', alignItems:'center', gap:'8px' }}>
              <div style={{ color: topSeat === 'white' ? '#d0fce8' : '#e8d8ff', fontWeight:700, fontSize:'16px', letterSpacing:'0.3px' }}>{topPlayerName}</div>
              {topSeatBadge && (
                <span style={{ padding:'2px 7px', borderRadius:'999px', background: topSeatBadge === 'You' ? (topSeat === 'white' ? 'rgba(74,222,128,0.16)' : 'rgba(96,165,250,0.18)') : 'rgba(255,255,255,0.06)', border: topSeatBadge === 'You' ? (topSeat === 'white' ? '1px solid rgba(74,222,128,0.32)' : '1px solid rgba(96,165,250,0.35)') : '1px solid rgba(255,255,255,0.10)', color: topSeatBadge === 'You' ? (topSeat === 'white' ? '#86efac' : '#93c5fd') : 'rgba(255,255,255,0.6)', fontSize:'9px', fontWeight:800, textTransform:'uppercase', letterSpacing:'0.8px' }}>
                  {topSeatBadge}
                </span>
              )}
            </div>
            <div style={{ display:'flex', alignItems:'center', gap:'10px', marginTop:'5px' }}>
              <span style={{ color:'#b088f0', fontSize:'12px', fontWeight:600 }}>♟ {displayedBlackRating}</span>
              <span style={{ color: timeB <= 30 ? '#ff5555' : '#f0a030', fontSize:'13px', fontFamily:'monospace', fontWeight:700, background: tickingState==='black'&&clockActive&&!over ? 'rgba(240,160,48,0.18)' : 'rgba(0,0,0,0.3)', padding:'2px 8px', borderRadius:'5px', border:'1px solid rgba(240,160,48,0.2)' }}>⏱ {fmtClock(timeB)}</span>
            </div>
          </div>
          <div style={{ width:'10px', height:'10px', borderRadius:'50%', background:'#2ecc71', boxShadow:'0 0 12px #2ecc71', flexShrink:0 }} />
        </div>
        )}

        {/* Card preview panel */}
        <div style={{
          border:'1px solid rgba(180,130,60,0.4)',
          borderRadius:'14px', overflow:'hidden',
          background: selectedCard ? selectedCard.color : 'rgba(8,4,20,0.88)',
          backdropFilter:'blur(20px)', WebkitBackdropFilter:'blur(20px)',
          boxShadow:'0 4px 30px rgba(0,0,0,0.8)',
          flex:1, minHeight:0, display:'flex', flexDirection:'column',
        }}>
          {cardPending ? (
            <div style={{ flex:1, display:'flex', flexDirection:'column', alignItems:'center', justifyContent:'center', gap:'12px', padding:'16px' }}>
              <div style={{ fontSize:'32px', animation: (cardPending.mechanic === 'smallsacrifice' || cardPending.mechanic === 'bigsacrifice') ? 'sacrificePulse 1.2s ease-in-out infinite' : cardPending.mechanic === 'mindcontrol' ? 'mindControlPulse 1.5s ease-in-out infinite' : (cardPending.mechanic === 'halffuse' || cardPending.mechanic === 'fullfusion') ? 'fusePulse 1.0s ease-in-out infinite' : 'none' }}>{cardPending.card.icon}</div>
              <div style={{ color:'#fff', fontWeight:700, fontSize:'13px', textAlign:'center' }}>{cardPending.card.name}</div>
              {cardPending.mechanic === 'mindcontrol' && (
                <div style={{ width:'100%' }}>
                  <div style={{ padding:'8px 10px', background:'rgba(139,0,255,0.12)', border:'1px solid rgba(168,85,247,0.4)', borderRadius:'8px', marginBottom:'6px' }}>
                    <div style={{ fontSize:'9px', fontWeight:700, color:'rgba(200,100,255,0.9)', textTransform:'uppercase', letterSpacing:'1px', marginBottom:'4px' }}>⚡ Psychic Scan Active</div>
                    <div style={{ fontSize:'10px', color:'rgba(180,140,255,0.8)', lineHeight:'1.5' }}>All enemy pieces are highlighted. Click any non-king enemy piece to permanently take control of it.</div>
                  </div>
                  <div style={{ display:'flex', gap:'6px', justifyContent:'center', flexWrap:'wrap' }}>
                    {(['queen','rook','bishop','knight','pawn'] as const).map(pt => {
                      const oppColor = OPP[cardPending.playerColor];
                      const hasPiece = board.some(r => r.some(p => p?.type === pt && p.color === oppColor));
                      return hasPiece ? (
                        <span key={pt} style={{ padding:'2px 7px', background:'rgba(139,0,255,0.18)', border:'1px solid rgba(168,85,247,0.4)', borderRadius:'4px', fontSize:'9px', color:'#d8b4fe', fontWeight:700, textTransform:'capitalize' }}>
                          🎯 {pt}
                        </span>
                      ) : null;
                    })}
                  </div>
                </div>
              )}
              {(cardPending.mechanic === 'halffuse' || cardPending.mechanic === 'fullfusion') && (() => {
                const type1 = cardPending.data?.type1 as PieceType | undefined;
                const val1  = cardPending.data?.val1 as number | undefined;
                const isHalf = cardPending.mechanic === 'halffuse';
                const HALF_CAP = 6;
                const accentColor = isHalf ? '#fbbf24' : '#a78bfa';
                const accentRgb   = isHalf ? '251,191,36' : '167,139,250';
                return (
                  <div style={{ width:'100%' }}>
                    <div style={{ padding:'8px 10px', background:`rgba(${accentRgb},0.10)`, border:`1px solid rgba(${accentRgb},0.4)`, borderRadius:'8px', marginBottom:'6px' }}>
                      <div style={{ fontSize:'9px', fontWeight:700, color:`rgba(${accentRgb},0.9)`, textTransform:'uppercase', letterSpacing:'1px', marginBottom:'4px' }}>
                        {isHalf ? '⚗️ Half Fuse' : '🔮 Full Fusion'} — Step {cardPending.step}/2
                      </div>
                      {cardPending.step === 1 && (
                        <div style={{ fontSize:'10px', color:`rgba(${accentRgb},0.8)`, lineHeight:'1.5' }}>
                          {isHalf
                            ? `Pick a piece to sacrifice. Combined value with absorber must be ≤${HALF_CAP}pts. Must be adjacent.`
                            : 'Pick any piece to sacrifice. No value cap, no distance restriction.'}
                        </div>
                      )}
                      {cardPending.step === 2 && type1 && val1 !== undefined && (
                        <div>
                          <div style={{ display:'flex', alignItems:'center', gap:'6px', marginBottom:'6px' }}>
                            <span style={{ padding:'2px 8px', background:`rgba(${accentRgb},0.2)`, border:`1px solid rgba(${accentRgb},0.5)`, borderRadius:'4px', fontSize:'10px', color: accentColor, fontWeight:700, textTransform:'capitalize' }}>
                              {'💀 '}{type1}{' ('}{val1}{'pt)'}
                            </span>
                            <span style={{ color:`rgba(${accentRgb},0.6)`, fontSize:'12px' }}>{'→'}</span>
                            <span style={{ padding:'2px 8px', background:'rgba(255,255,255,0.07)', border:'1px solid rgba(255,255,255,0.2)', borderRadius:'4px', fontSize:'10px', color:'#cbd5e1', fontWeight:700 }}>
                              {'? absorber'}
                            </span>
                          </div>
                          {isHalf && (
                            <div style={{ marginBottom:'4px' }}>
                              <div style={{ display:'flex', justifyContent:'space-between', marginBottom:'3px' }}>
                                <span style={{ fontSize:'9px', color:'#fbbf24', fontWeight:700 }}>{'Value budget'}</span>
                                <span style={{ fontSize:'9px', fontWeight:800, color:'#fbbf24' }}>{val1}{' / '}{HALF_CAP}{' pts'}</span>
                              </div>
                              <div style={{ height:'5px', background:'rgba(0,0,0,0.4)', borderRadius:'3px', overflow:'hidden', border:'1px solid rgba(251,191,36,0.3)' }}>
                                <div style={{ height:'100%', borderRadius:'3px', width:`${Math.min(100,(val1/HALF_CAP)*100)}%`, background:'linear-gradient(90deg,#92400e,#fbbf24)' }} />
                              </div>
                            </div>
                          )}
                          <div style={{ fontSize:'9px', color:`rgba(${accentRgb},0.7)`, marginTop:'4px' }}>
                            {isHalf ? 'Adjacent only. Red-tinted squares exceed the 6pt cap.' : 'Pick any piece — it absorbs the sacrifice and gains both movement types.'}
                          </div>
                        </div>
                      )}
                    </div>
                  </div>
                );
              })()}
              {(cardPending.mechanic === 'smallsacrifice' || cardPending.mechanic === 'bigsacrifice') && (() => {
                const selected = (cardPending.data?.selected as Sq[] | undefined) ?? [];
                const totalVal = selected.reduce((sum, sq) => sum + PIECE_VALUE[board[sq.row][sq.col]?.type ?? 'pawn'], 0);
                const goal = cardPending.mechanic === 'smallsacrifice' ? 6 : 14;
                const pct = Math.min(100, (totalVal / goal) * 100);
                const ready = totalVal >= goal;
                return (
                  <div style={{ width:'100%' }}>
                    <div style={{ display:'flex', justifyContent:'space-between', marginBottom:'5px' }}>
                      <span style={{ fontSize:'10px', color: ready ? '#ef4444' : '#a0b8d8', fontWeight:700 }}>Blood Price</span>
                      <span style={{ fontSize:'11px', fontWeight:800, color: ready ? '#ef4444' : '#a0b8d8' }}>{totalVal} / {goal} pts</span>
                    </div>
                    <div style={{ height:'8px', background:'rgba(0,0,0,0.5)', borderRadius:'4px', overflow:'hidden', border:'1px solid rgba(220,20,20,0.3)' }}>
                      <div style={{
                        height:'100%', borderRadius:'4px',
                        width:`${pct}%`,
                        background: ready
                          ? 'linear-gradient(90deg, #dc1414, #ff4444)'
                          : 'linear-gradient(90deg, #7f1d1d, #dc2626)',
                        boxShadow: ready ? '0 0 8px rgba(220,20,20,0.9)' : 'none',
                        transition:'width 0.3s ease, background 0.3s ease',
                      }} />
                    </div>
                    {selected.length > 0 && (
                      <div style={{ marginTop:'6px', display:'flex', flexWrap:'wrap', gap:'3px', justifyContent:'center' }}>
                        {selected.map((sq, i) => {
                          const p = board[sq.row][sq.col];
                          return p ? (
                            <span key={i} style={{ padding:'2px 6px', background:'rgba(220,20,20,0.25)', border:'1px solid rgba(220,20,20,0.5)', borderRadius:'4px', fontSize:'9px', color:'#fca5a5', fontWeight:700, textTransform:'capitalize' }}>
                              🩸 {p.type} ({PIECE_VALUE[p.type]}pt)
                            </span>
                          ) : null;
                        })}
                      </div>
                    )}
                    {ready && (
                      <div style={{ marginTop:'8px', padding:'5px 10px', background:'rgba(220,20,20,0.2)', border:'1px solid rgba(220,20,20,0.5)', borderRadius:'6px', fontSize:'10px', color:'#fca5a5', textAlign:'center', fontWeight:700 }}>
                        ✅ Click empty square to confirm sacrifice
                      </div>
                    )}
                  </div>
                );
              })()}
              <div style={{ color:'#a0b8d8', fontSize:'11px', textAlign:'center', lineHeight:1.5, padding:'8px 10px', background: (cardPending.mechanic === 'smallsacrifice' || cardPending.mechanic === 'bigsacrifice') ? 'rgba(220,20,20,0.08)' : cardPending.mechanic === 'mindcontrol' ? 'rgba(139,0,255,0.08)' : (cardPending.mechanic === 'halffuse') ? 'rgba(251,191,36,0.08)' : (cardPending.mechanic === 'fullfusion') ? 'rgba(167,139,250,0.08)' : 'rgba(74,144,210,0.1)', border:`1px solid ${(cardPending.mechanic === 'smallsacrifice' || cardPending.mechanic === 'bigsacrifice') ? 'rgba(220,20,20,0.3)' : cardPending.mechanic === 'mindcontrol' ? 'rgba(139,0,255,0.35)' : (cardPending.mechanic === 'halffuse') ? 'rgba(251,191,36,0.35)' : (cardPending.mechanic === 'fullfusion') ? 'rgba(167,139,250,0.35)' : 'rgba(74,144,210,0.3)'}`, borderRadius:'8px' }}>{cardMsg || 'Click a square on the board...'}</div>
              <button onClick={cancelCard} style={{ padding:'7px 18px', background:'rgba(231,76,60,0.2)', color:'#e74c3c', border:'1px solid rgba(231,76,60,0.4)', borderRadius:'6px', cursor:'pointer', fontSize:'12px', fontWeight:700 }}>✕ Cancel</button>
              {promoPicker && (
                <div ref={promoRef} role="dialog" aria-modal="true" aria-label="Choose promotion piece" style={{ display:'flex', gap:'6px', flexWrap:'wrap', justifyContent:'center', marginTop:'4px' }}>
                  {promoPicker.options.map(t => (
                    <button key={t} onClick={() => handlePromoPick(t)}
                      style={{ padding:'6px 10px', background:'rgba(245,158,11,0.2)', color:'#f59e0b', border:'1px solid rgba(245,158,11,0.5)', borderRadius:'6px', cursor:'pointer', fontSize:'11px', fontWeight:700, textTransform:'capitalize' }}>
                      {t}
                    </button>
                  ))}
                </div>
              )}
            </div>
          ) : selectedCard ? (() => {
            const ownerColor: PieceColor = whiteHand.some(c => c.id === selectedCard.id) ? 'white' : 'black';
            const isViewerOwner = (hostedRuntime || authoritativeMatchId) ? viewerSeat === ownerColor : true;
            const canUse = canUseCard(selectedCard, ownerColor) && isViewerOwner;
            const usedThisTurn = cardUsedBy[ownerColor];
            let blockReason = '';
            if (over)           blockReason = 'Game is over';
            else if (!isViewerOwner) blockReason = "Not your card to use";
            else if (usedThisTurn) blockReason = 'Already used a card this turn';
            else if (selectedCard.type !== 'trap' && turn !== ownerColor) blockReason = `Only usable on ${ownerColor}'s turn`;
            return (
              <div style={{ display:'flex', flexDirection:'column', background: selectedCard.color, animation:'cardReveal 0.22s cubic-bezier(0.34,1.56,0.64,1)', flex:1, overflow:'hidden' }}>
                <div style={{ padding:'10px 14px 8px', display:'flex', justifyContent:'space-between', alignItems:'center', borderBottom:`1px solid ${selectedCard.accent}55` }}>
                  <div>
                    <div style={{ color:'#fff', fontWeight:800, fontSize:'14px', textShadow:'0 1px 6px rgba(0,0,0,0.9)' }}>{selectedCard.name}</div>
                    <div style={{ marginTop:'3px', display:'inline-block', padding:'2px 8px', borderRadius:'4px', fontSize:'9px', fontWeight:800, textTransform:'uppercase', letterSpacing:'0.8px', color: RARITY_STYLE[selectedCard.rarity].accent, background:`${RARITY_STYLE[selectedCard.rarity].accent}33`, border:`1px solid ${RARITY_STYLE[selectedCard.rarity].accent}88` }}>
                      {RARITY_STYLE[selectedCard.rarity].label} · {RARITY_WEIGHTS[selectedCard.rarity]}% drop
                    </div>
                  </div>
                  <div style={{ width:'26px', height:'26px', borderRadius:'6px', background:`${selectedCard.accent}44`, border:`1px solid ${selectedCard.accent}88`, display:'flex', alignItems:'center', justifyContent:'center', fontSize:'14px' }}>地</div>
                </div>
                <div style={{ height:'150px', margin:'10px 12px', borderRadius:'10px', background:`radial-gradient(ellipse at 50% 40%, ${selectedCard.accent}66 0%, rgba(0,0,0,0.8) 70%)`, border:`2px solid ${selectedCard.accent}55`, display:'flex', alignItems:'center', justifyContent:'center', fontSize:'68px', position:'relative', overflow:'hidden', boxShadow:`0 0 24px ${selectedCard.accent}33` }}>
                  <div style={{ animation: selectedCard.mechanic === 'joker' ? 'jokerFloat 2s ease-in-out infinite' : 'none', filter:`drop-shadow(0 0 10px ${selectedCard.accent})` }}>
                    {selectedCard.icon}
                  </div>
                  {selectedCard.mechanic === 'joker' && (
                    <>
                      {[0,1,2,3,4].map(j => (
                        <div key={j} style={{
                          position:'absolute',
                          top:`${10+j*18}%`, left:`${5+j*22}%`,
                          fontSize:'10px',
                          animation:`jokerGlitter ${1+j*0.3}s ease-in-out infinite`,
                          animationDelay:`${j*0.2}s`,
                          pointerEvents:'none',
                        }}>✦</div>
                      ))}
                    </>
                  )}
                </div>
                <div style={{ margin:'0 14px 8px', padding:'4px 10px', background:`${selectedCard.accent}28`, border:`1px solid ${selectedCard.accent}55`, borderRadius:'5px', fontSize:'10px', color:selectedCard.accent, fontWeight:700, textTransform:'uppercase', letterSpacing:'1px', display:'inline-flex', alignSelf:'flex-start' }}>[{selectedCard.type === 'spell' ? 'Spell Card' : 'Trap Card'}]</div>
                <div style={{ margin:'0 14px 12px', fontSize:'11px', color:'rgba(235,225,210,0.95)', lineHeight:'1.65', fontWeight:500 }}>{selectedCard.desc}</div>
                {blockReason && <div style={{ margin:'0 14px 8px', padding:'7px 10px', background:'rgba(200,40,40,0.2)', border:'1px solid rgba(220,60,60,0.5)', borderRadius:'6px', fontSize:'10px', color:'#ff8080', fontWeight:600, textAlign:'center' }}>🔒 {blockReason}</div>}
                <div style={{ flex:1 }} />
                <div style={{ padding:'4px 14px 16px' }}>
                  <button onClick={() => applyCard(selectedCard, ownerColor)} disabled={!canUse || intentInFlight}
                    style={{ width:'100%', padding:'11px', borderRadius:'22px', border:'none', background: canUse ? (selectedCard.mechanic === 'joker' ? 'linear-gradient(135deg, #f59e0b, #b45309)' : 'linear-gradient(135deg, #3b9edd, #1a5fa8)') : 'rgba(40,40,60,0.8)', color: canUse ? '#fff' : 'rgba(255,255,255,0.25)', fontWeight:700, fontSize:'13px', cursor: (canUse && !intentInFlight) ? 'pointer' : 'not-allowed', boxShadow: canUse ? (selectedCard.mechanic === 'joker' ? '0 4px 16px rgba(245,158,11,0.55)' : '0 4px 16px rgba(26,111,196,0.55)') : 'none', letterSpacing:'0.3px', opacity: intentInFlight ? 0.6 : 1 }}>
                    {intentInFlight ? 'Sending...' : canUse ? (selectedCard.mechanic === 'joker' ? '🃏 Choose Transformation' : 'use card') : '🔒 blocked'}
                  </button>
                </div>
              </div>
            );
          })() : (
            <div style={{ flex:1, display:'flex', flexDirection:'column', alignItems:'center', justifyContent:'center', gap:'10px' }}>
              <div style={{ fontSize:'40px', opacity:0.15 }}>🃏</div>
              <div style={{ color:'rgba(180,150,100,0.4)', fontSize:'12px', letterSpacing:'0.5px' }}>Click a card to preview</div>
              {cardMsg && <div style={{ color:'#f59e0b', fontSize:'11px', textAlign:'center', padding:'6px 10px', background:'rgba(245,158,11,0.1)', border:'1px solid rgba(245,158,11,0.3)', borderRadius:'6px', maxWidth:'180px' }}>{cardMsg}</div>}
              {streamDisconnected && (
                <button
                  type="button"
                  onClick={onReconnect}
                  style={{ padding:'6px 12px', background:'rgba(74,222,128,0.15)', border:'1px solid rgba(74,222,128,0.5)', borderRadius:'6px', color:'#4ade80', fontWeight:700, fontSize:'11px', cursor:'pointer' }}
                >
                  ↻ Reconnect
                </button>
              )}
              {doubleMove && (
                <div style={{ padding:'8px 12px', background: doubleMove.type === 'same' ? 'rgba(74,222,128,0.12)' : 'rgba(96,165,250,0.12)', border:`1px solid ${doubleMove.type === 'same' ? 'rgba(74,222,128,0.5)' : 'rgba(96,165,250,0.5)'}`, borderRadius:'8px', fontSize:'10px', color: doubleMove.type === 'same' ? '#4ade80' : '#60a5fa', fontWeight:700, textAlign:'center', maxWidth:'180px', animation:'pulse 1.5s ease infinite' }}>
                  {doubleMove.type === 'same' ? '🏃 SOLO' : '👥 TWIN'} — Move {3 - doubleMove.movesLeft}/2
                  {doubleMove.trackedSq && doubleMove.type === 'same' && <div style={{ fontSize:'9px', marginTop:'3px', opacity:0.8 }}>Move piece at {FILES[doubleMove.trackedSq.col]}{RANKS[doubleMove.trackedSq.row]}</div>}
                  {doubleMove.trackedSq && doubleMove.type === 'diff' && <div style={{ fontSize:'9px', marginTop:'3px', opacity:0.8 }}>Don't move {FILES[doubleMove.trackedSq.col]}{RANKS[doubleMove.trackedSq.row]} again</div>}
                </div>
              )}
              {/* Bomb status display */}
              {bombPieces.length > 0 && (
                <div style={{ padding:'8px 12px', background:'rgba(255,80,0,0.12)', border:'1px solid rgba(255,80,0,0.5)', borderRadius:'8px', fontSize:'10px', color:'#ff6030', fontWeight:700, textAlign:'center', maxWidth:'180px', animation:'bombGlow 1s ease-in-out infinite' }}>
                  💣 {bombPieces.length} BOMB{bombPieces.length > 1 ? 'S' : ''} ACTIVE
                  {bombPieces.map((b, i) => (
                    <div key={i} style={{ fontSize:'9px', marginTop:'2px', opacity:0.8 }}>
                      {FILES[b.col]}{RANKS[b.row]} — {b.turnsLeft} turn{b.turnsLeft !== 1 ? 's' : ''} left
                    </div>
                  ))}
                </div>
              )}
              <div style={{ display:'flex', flexDirection:'column', alignItems:'center', gap:'4px', marginTop:'8px' }}>
                {(['white','black'] as PieceColor[]).map(color => (
                  <div key={color} style={{ display:'flex', alignItems:'center', gap:'6px', fontSize:'10px' }}>
                    <span style={{ color: color==='white' ? '#e8eaf0' : '#7ab8f5' }}>{color==='white' ? '⚪' : '⚫'}</span>
                    <span style={{ color: cardUsedBy[color] ? '#e74c3c' : '#2ecc71', fontWeight:600 }}>{cardUsedBy[color] ? '✓ Card used' : '○ Card available'}</span>
                  </div>
                ))}
                <div style={{ marginTop:'6px', fontSize:'9px', color:'rgba(160,184,216,0.6)', textAlign:'center' }}>Max {MAX_HAND_SIZE} cards per player</div>
              </div>
            </div>
          )}
        </div>

        {renderPlayerCard(bottomSeat)}
        {false && (
        <div style={{
          background:'rgba(8,45,18,0.50)',
          backdropFilter:'blur(16px)', WebkitBackdropFilter:'blur(16px)',
          border:'1px solid rgba(60,220,110,0.45)',
          borderRadius:'16px', padding:'12px 16px',
          display:'flex', alignItems:'center', gap:'12px',
          boxShadow:'0 8px 32px rgba(0,0,0,0.35), inset 0 1px 0 rgba(80,240,130,0.2), 0 0 30px rgba(30,180,70,0.2)',
        }}>
          <div style={{ width:'58px', height:'58px', borderRadius:'50%', flexShrink:0, background:'linear-gradient(135deg, #0a200f, #051208)', border:'2px solid rgba(46,180,90,0.7)', display:'flex', alignItems:'center', justifyContent:'center', fontSize:'28px', boxShadow:'0 0 20px rgba(46,180,90,0.4)' }}>🧑‍💻</div>
          <div style={{ flex:1, minWidth:0 }}>
            <div style={{ display:'flex', alignItems:'center', gap:'8px' }}>
              <div style={{ color:'#d0fce8', fontWeight:700, fontSize:'16px', letterSpacing:'0.3px' }}>{displayedWhiteName}</div>
              {whiteSeatBadge && (
                <span style={{ padding:'2px 7px', borderRadius:'999px', background:whiteSeatBadge === 'You' ? 'rgba(74,222,128,0.16)' : 'rgba(255,255,255,0.06)', border:whiteSeatBadge === 'You' ? '1px solid rgba(74,222,128,0.32)' : '1px solid rgba(255,255,255,0.10)', color:whiteSeatBadge === 'You' ? '#86efac' : 'rgba(255,255,255,0.6)', fontSize:'9px', fontWeight:800, textTransform:'uppercase', letterSpacing:'0.8px' }}>
                  {whiteSeatBadge}
                </span>
              )}
            </div>
            <div style={{ display:'flex', alignItems:'center', gap:'10px', marginTop:'5px' }}>
              <span style={{ color:'#52c77a', fontSize:'12px', fontWeight:600 }}>♟ {displayedWhiteRating}</span>
              <span style={{ color: timeW <= 30 ? '#ff5555' : '#f0a030', fontSize:'13px', fontFamily:'monospace', fontWeight:700, background: tickingState==='white'&&clockActive&&!over ? 'rgba(240,160,48,0.18)' : 'rgba(0,0,0,0.3)', padding:'2px 8px', borderRadius:'5px', border:'1px solid rgba(240,160,48,0.2)' }}>⏱ {fmtClock(timeW)}</span>
            </div>
          </div>
          <div style={{ width:'10px', height:'10px', borderRadius:'50%', background:'#2ecc71', boxShadow:'0 0 12px #2ecc71', flexShrink:0 }} />
        </div>
        )}
      </div>

      {/* ── Board column ── */}
    <div className="match-layout__center">

        {authoritativeLive && authoritativeStatus === 'waiting' && authoritativeMatchId ? (
          <div style={{ marginBottom:'8px', padding:'9px 14px', background:'rgba(255,212,135,0.10)', border:'1px solid rgba(255,212,135,0.28)', borderRadius:'8px', color:'#ffd487', fontSize:'11px', fontWeight:700, textAlign:'center' }}>
            Private room is waiting for the second player to open the invite link. This seat is reserved, but the game will only start once both seats are claimed.
          </div>
        ) : null}
        {activeDisconnectGraceFor && activeDisconnectGraceFor !== viewerSeat && (
          <div style={{ marginBottom:'8px', padding:'9px 14px', background:'rgba(245,158,11,0.15)', border:'1px solid rgba(245,158,11,0.4)', borderRadius:'8px', color:'#f59e0b', fontSize:'11px', fontWeight:700, textAlign:'center', display:'flex', alignItems:'center', justifyContent:'center', gap:'8px' }}>
            <span style={{ animation:'pulse 1.5s ease-in-out infinite' }}>⏳</span>
            Opponent disconnected. Waiting for them to return...
          </div>
        )}
        {showHostedSoloBanner && (
          <div style={{ marginBottom:'8px', padding:'8px 14px', background:'rgba(96,165,250,0.10)', border:'1px solid rgba(96,165,250,0.28)', borderRadius:'8px', color:'#93c5fd', fontSize:'11px', fontWeight:700, textAlign:'center' }}>
            Solo board: use the Queue tab to find a real online opponent.
          </div>
        )}
        {cardPending && (
          <div style={{ marginBottom:'6px', padding:'8px 18px', background:'rgba(245,158,11,0.15)', border:'1px solid rgba(245,158,11,0.5)', borderRadius:'8px', color:'#f59e0b', fontSize:'12px', fontWeight:700, textAlign:'center', animation:'pulse 1.5s ease infinite' }}>
            🃏 {cardMsg} &nbsp;<span onClick={cancelCard} style={{ cursor:'pointer', color:'#e74c3c', marginLeft:'8px' }}>✕ cancel</span>
          </div>
        )}
        {doubleMove && !cardPending && (
          <div style={{ marginBottom:'6px', padding:'7px 16px', background: doubleMove.type === 'same' ? 'rgba(74,222,128,0.12)' : 'rgba(96,165,250,0.12)', border:`1px solid ${doubleMove.type === 'same' ? 'rgba(74,222,128,0.5)' : 'rgba(96,165,250,0.5)'}`, borderRadius:'8px', color: doubleMove.type === 'same' ? '#4ade80' : '#60a5fa', fontSize:'12px', fontWeight:700, textAlign:'center', animation:'pulse 1.5s ease infinite' }}>
            {doubleMove.movesLeft === 2
              ? (doubleMove.type === 'same' ? '🏃 Solo: Make your first move!' : '👥 Twin: Make your first move!')
              : doubleMove.type === 'same'
                ? `🏃 Solo: Now move the SAME piece at ${doubleMove.trackedSq ? FILES[doubleMove.trackedSq.col]+RANKS[doubleMove.trackedSq.row] : '?'} again!`
                : `👥 Twin: Now move a DIFFERENT piece! (not ${doubleMove.trackedSq ? FILES[doubleMove.trackedSq.col]+RANKS[doubleMove.trackedSq.row] : '?'})`
            }
          </div>
        )}
        {radarActive && (
          <div style={{ marginBottom:'4px', padding:'5px 14px', background:'rgba(96,165,250,0.15)', border:'1px solid rgba(96,165,250,0.5)', borderRadius:'8px', color:'#60a5fa', fontSize:'11px', fontWeight:700, textAlign:'center', display:'flex', alignItems:'center', gap:'8px', justifyContent:'center' }}>
            <span style={{ animation:'radarSweep 1.5s linear infinite', display:'inline-block' }}>📡</span>
            RADAR ACTIVE — Enemy hand revealed!
            <span style={{ animation:'radarPing 1s ease-out infinite', display:'inline-block', width:'8px', height:'8px', borderRadius:'50%', background:'#60a5fa' }} />
          </div>
        )}
        {/* Bomb warning banner */}
        {bombPieces.length > 0 && (
          <div style={{ marginBottom:'4px', padding:'5px 14px', background:'rgba(255,80,0,0.12)', border:'1px solid rgba(255,80,0,0.45)', borderRadius:'8px', color:'#ff7040', fontSize:'11px', fontWeight:700, textAlign:'center', display:'flex', alignItems:'center', gap:'8px', justifyContent:'center', animation:'bombGlow 1s ease-in-out infinite' }}>
            <span style={{ animation:'bombTick 0.8s ease-in-out infinite' }}>💣</span>
            BOMB{bombPieces.length > 1 ? 'S' : ''} ACTIVE — {bombPieces.map(b => `${FILES[b.col]}${RANKS[b.row]}(${b.turnsLeft}t)`).join(', ')}
          </div>
        )}

        {renderHand(topHand, topSeat, 'top')}

        <div
          ref={boardWrapperRef}
          className="match-layout__board-wrapper"
          style={{
            border:'2px solid rgba(220,160,40,0.8)',
            borderRadius:'4px',
            boxShadow:'0 0 0 1px rgba(255,200,60,0.2), 0 0 60px rgba(200,100,10,0.5), 0 0 120px rgba(180,60,0,0.25)',
            maxWidth: `${SQ * 8}px`,
            boxSizing:'border-box',
          }}
        >
            {hostedRuntime && !viewerSeat && authoritativeMatchId && (
              <div style={{ position:'absolute', inset:0, zIndex:40, display:'flex', alignItems:'center', justifyContent:'center', pointerEvents:'none' }}>
                <div style={{
                  padding:'8px 20px',
                  borderRadius:'8px',
                  background:'rgba(0,0,0,0.55)',
                  backdropFilter:'blur(4px)',
                  border:'1px solid rgba(255,212,135,0.3)',
                  color:'#ffd487',
                  fontSize:'13px',
                  fontWeight:800,
                  letterSpacing:'1.5px',
                  textTransform:'uppercase',
                  boxShadow:'0 4px 20px rgba(0,0,0,0.5)',
                }}>
                  Spectating
                </div>
              </div>
            )}
            <BoardCanvas
              reverseAnim={null}
              board={board}
              turn={turn}
              sel={sel}
              hints={hints}
              lm={lm}
              drag={drag}
              dragPos={dragPos}
              check={check}
              kingPos={kingPos}
              cardHighlight={getCardHighlight}
              doubleMoveHighlight={getDoubleMoveHighlight}
              bombPieces={bombPieces}
              bombExploding={bombExploding}
              lavaSquares={lavaSquares}
              lavaExploding={lavaExploding}
              swapAnim={swapAnim}
              isReviewing={isReviewing}
              reviewBoard={reviewBoard}
              cardPending={cardPending}
              onClick={clickSq}
              onPremove={() => { if (premove) { setPremove(null); } }}
              premove={premove}
              onDragStart={(e, r, c) => {
                if (cardPending || isReviewing || over || promo || (hostedRuntime && authoritativeStatus !== 'active')) return;
                const p = board[r][c];
                const ghostDs = ghostRef.current;
                const actingColor = (hostedRuntime || authoritativeMatchId) ? viewerSeat : turn;
                const isGhostDs = ghostDs && actingColor && ghostDs.ownerColor === actingColor && turn === actingColor && ghostDs.row === r && ghostDs.col === c;
                if (!actingColor) return;
                if (!isGhostDs && (!p || p.color !== actingColor || turn !== actingColor)) return;
                setDrag({ row: r, col: c });
                setSel({ row: r, col: c });
                setHints(getMoves(r, c));
                const rect = (e.target as HTMLElement).getBoundingClientRect();
                setDragPos({ x: e.clientX - rect.left, y: e.clientY - rect.top });
              }}
              onDrop={(r, c) => {
                if (!drag || isReviewing || cardPending) { setDrag(null); setDragPos(null); setSel(null); setHints([]); return; }
                const mv = getMoves(drag.row, drag.col);
                if (mv.some(m => m.row === r && m.col === c)) doMove(drag.row, drag.col, r, c);
                setDrag(null); setDragPos(null); setSel(null); setHints([]);
              }}
              doubleMove={doubleMove}
              transformAnim={transformAnim}
              sniperAnim={sniperAnim}
              teleportAnim={teleportAnim}
              jumpAnim={jumpAnim}
              sacrificeAnim={sacrificeAnim}
              sacrificeSelectedSquares={
                cardPending?.mechanic === 'smallsacrifice' || cardPending?.mechanic === 'bigsacrifice'
                  ? ((cardPending.data?.selected as Sq[] | undefined) ?? [])
                  : []
              }
              mindControlAnim={mindControlAnim}
              mindControlTargetSquare={null}
              fuseAnim={fuseAnim}
              fuseSelectedSq={
                (cardPending?.mechanic === 'halffuse' || cardPending?.mechanic === 'fullfusion') && cardPending.step === 2
                  ? (cardPending.data?.sq1 as { row: number; col: number } | undefined) ?? null
                  : null
              }
              fogZones={fogZones}
              viewerColor={hostedRuntime ? viewerSeat : turn}
              invisibleUnder={ghostPiece}
              analysisArrows={analysisArrows}
              onToggleAnalysisArrow={toggleAnalysisArrow}
              onClearAnalysisArrows={clearAnalysisArrows}
              colorBlindMode={colorBlindMode}
            />

            {/* Promotion overlay */}
            {promo && (() => {
              const order: PieceType[] = promo.color === 'white'
                ? ['queen','knight','rook','bishop']
                : ['bishop','rook','knight','queen'];
              const vSQ = boardWrapperPx / 8;
              const left = promo.col * vSQ + 4;
              const top  = promo.color === 'white' ? 4 : 4 * vSQ + 4;
              return (
                <div ref={promoFullRef}>
                  <div role="dialog" aria-modal="true" aria-label="Choose promotion piece" style={{ position:'absolute', inset:0, background:'rgba(0,0,0,0.45)', zIndex:50 }} />
                  <div style={{ position:'absolute', left:`${left}px`, top:`${top}px`, zIndex:51, display:'flex', flexDirection:'column' }}>
                    {order.map(t => (
                      <div key={t} onClick={() => doPromo(t)}
                        style={{ width:`${vSQ}px`, height:`${vSQ}px`, background:'#c8c8c8', display:'flex', alignItems:'center', justifyContent:'center', cursor:'pointer', boxShadow:'0 2px 8px rgba(0,0,0,0.6)', userSelect:'none' }}
                        onMouseEnter={e => { (e.currentTarget as HTMLDivElement).style.background = '#e8a020'; }}
                        onMouseLeave={e => { (e.currentTarget as HTMLDivElement).style.background = '#c8c8c8'; }}
                      >
                        <div style={{ width:'50px', height:'50px', borderRadius:'50%', background:'rgba(255,255,255,0.18)', display:'flex', alignItems:'center', justifyContent:'center' }}>
                          <img src={`/pieces/${promo.color}_${t}.svg`} alt={`${promo.color} ${t}`} style={{ width:'40px', height:'40px', objectFit:'contain', pointerEvents:'none' }} draggable={false} />
                        </div>
                      </div>
                    ))}
                  </div>
                </div>
              );
            })()}
            {cardPromo && (() => {
              const order: PieceType[] = cardPromo.color === 'white'
                ? ['queen','knight','rook','bishop']
                : ['bishop','rook','knight','queen'];
              const vSQ = boardWrapperPx / 8;
              const left = cardPromo.sq.col * vSQ + 4;
              const top  = cardPromo.color === 'white' ? 4 : 4 * vSQ + 4;
              return (
                <div ref={cardPromoRef}>
                  <div role="dialog" aria-modal="true" aria-label="Choose promotion piece" style={{ position:'absolute', inset:0, background:'rgba(0,0,0,0.45)', zIndex:50 }} />
                  <div style={{ position:'absolute', left:`${left}px`, top:`${top}px`, zIndex:51, display:'flex', flexDirection:'column' }}>
                    {order.map(t => (
                      <div key={t} onClick={() => {
                        const nb = board.map(r => r.map(p => p ? { ...p } : null));
                        nb[cardPromo.sq.row][cardPromo.sq.col] = { type: t, color: cardPromo.color };
                        const myKp = findKing(nb, OPP[turn]);
                        if (myKp && isAttackedWithFusion(nb, myKp.row, myKp.col, turn)) {
                          setCardMsg('❌ That promotion would leave your king in check!');
                          setTimeout(() => setCardMsg(''), 2000);
                          return;
                        }
                        setBoard(nb);
                        setCardPromo(null);
                        const posKey = positionKey(nb, turn, moved, lm);
                        const newPh = [...posHist, posKey];
                        setPosHist(newPh);
                        checkEndGame(nb, turn, moved, lm, hmc, newPh, posKey, toFEN(nb, turn, moved, lm, hmc, fmn), OPP[turn]);
                      }}
                        style={{ width:`${vSQ}px`, height:`${vSQ}px`, background:'#c8c8c8', display:'flex', alignItems:'center', justifyContent:'center', cursor:'pointer', boxShadow:'0 2px 8px rgba(0,0,0,0.6)', userSelect:'none' }}
                        onMouseEnter={e => { (e.currentTarget as HTMLDivElement).style.background = '#e8a020'; }}
                        onMouseLeave={e => { (e.currentTarget as HTMLDivElement).style.background = '#c8c8c8'; }}
                      >
                        <div style={{ width:'50px', height:'50px', borderRadius:'50%', background:'rgba(255,255,255,0.18)', display:'flex', alignItems:'center', justifyContent:'center' }}>
                          <img src={`/pieces/${cardPromo.color}_${t}.svg`} alt={`${cardPromo.color} ${t}`} style={{ width:'40px', height:'40px', objectFit:'contain', pointerEvents:'none' }} draggable={false} />
                        </div>
                      </div>
                    ))}
                  </div>
                </div>
              );
            })()}
        </div>

        {renderHand(bottomHand, bottomSeat, 'bottom')}
      </div>

      {/* ── Right panel ── */}
      <div style={{
        flex:1, minWidth:0,
        background:'rgba(20,8,45,0.45)',
        backdropFilter:'blur(24px)', WebkitBackdropFilter:'blur(24px)',
        border:'1px solid rgba(255,165,40,0.3)',
        borderRadius:'16px', padding:'12px',
        display:'flex', flexDirection:'column', gap:'8px',
        boxShadow:'0 8px 40px rgba(0,0,0,0.35), inset 0 1px 0 rgba(255,165,40,0.12), 0 0 40px rgba(200,80,10,0.12)',
        overflow:'auto', margin:'10px 0',
      }}>

        {/* Round + rarity panel */}
        <div style={{
          display:'flex', flexDirection:'column', gap:'6px',
          padding:'10px 14px',
          background:'rgba(255,140,0,0.06)',
          border:'1px solid rgba(255,165,40,0.18)',
          borderRadius:'12px', flexShrink:0,
        }}>
          <div style={{ display:'flex', alignItems:'center', justifyContent:'space-between', gap:'8px' }}>
            <div style={{ color: authoritativeLive ? '#4ade80' : (hostedRuntime ? '#f59e0b' : 'rgba(160,184,216,0.55)'), fontSize:'9px', fontWeight:700, textTransform:'uppercase', letterSpacing:'1px' }}>
              {boardStatusLabel}
            </div>
            <div style={{ color:'rgba(160,184,216,0.4)', fontSize:'8px' }}>
              {authoritativeMatchIdRef.current ? `match ${authoritativeMatchIdRef.current.slice(-6)}` : 'no match'}
            </div>
          </div>
          {showHostedReconnectWarning && (
            <div style={{ display:'flex', alignItems:'center', justifyContent:'space-between', gap:'10px', padding:'7px 9px', borderRadius:'8px', background:'rgba(245,158,11,0.10)', border:'1px solid rgba(245,158,11,0.28)' }}>
              <div style={{ color:'#fcd34d', fontSize:'10px', lineHeight:1.35 }}>
                Live match sync is reconnecting, so updates may briefly fall back to slower refreshes until the stream is back.
              </div>
              <button
                onClick={() => { void bootstrapAuthoritativeMatch(); }}
                style={{ padding:'6px 10px', background:'linear-gradient(180deg,#d97706,#92400e)', color:'#fff', border:'1px solid rgba(251,191,36,0.35)', borderRadius:'7px', cursor:'pointer', fontSize:'10px', fontWeight:800, whiteSpace:'nowrap' }}
              >
                Retry Sync
              </button>
            </div>
          )}
          <div style={{ display:'flex', alignItems:'center', gap:'10px' }}>
            <div style={{ flex:1 }}>
              <div style={{ color:'#a0b8d8', fontSize:'9px', fontWeight:600, textTransform:'uppercase', letterSpacing:'0.8px' }}>Round</div>
              <div style={{ color: roundNumber >= 7 ? '#f39c12' : '#fff', fontSize:'20px', fontWeight:800, lineHeight:1.1 }}>{roundNumber}</div>
              <div style={{ color:'#4a6080', fontSize:'9px', marginTop:'1px' }}>
                {roundNumber < INITIAL_DEAL_ROUND ? `cards dealt at r${INITIAL_DEAL_ROUND}`
                : roundNumber < DRAW_FROM        ? `next draw at r${DRAW_FROM}`
                : (roundNumber - DRAW_FROM) % DRAW_EVERY === 0 ? '🃏 draw now!'
                : `next draw r${DRAW_FROM + Math.ceil((roundNumber - DRAW_FROM) / DRAW_EVERY) * DRAW_EVERY}`}
              </div>
            </div>
            <div style={{ display:'flex', flexDirection:'column', alignItems:'center', gap:'3px' }}>
              <div style={{ width:'30px', height:'30px', borderRadius:'50%', background: turn==='white' ? 'radial-gradient(circle, #fff 0%, #ccc 100%)' : 'radial-gradient(circle, #444 0%, #111 100%)', border: turn==='white' ? '2px solid rgba(255,255,255,0.6)' : '2px solid rgba(74,144,210,0.5)', display:'flex', alignItems:'center', justifyContent:'center', fontSize:'15px' }}>{turn==='white'?'♔':'♚'}</div>
              <div style={{ color:'rgba(200,215,235,0.55)', fontSize:'8px', fontWeight:600 }}>{turn==='white'?'WHITE':'BLACK'}</div>
            </div>
            <div style={{ flex:1, textAlign:'right' }}>
              <div style={{ color:'#a0b8d8', fontSize:'9px', fontWeight:600, textTransform:'uppercase', letterSpacing:'0.8px', marginBottom:'2px' }}>∞ Card Pool</div>
              <div style={{ color:'rgba(160,184,216,0.5)', fontSize:'8px', lineHeight:1.4 }}>Infinite draws<br/>Rarity weighted</div>
            </div>
          </div>
          <div style={{ display:'flex', flexDirection:'column', gap:'3px' }}>
            <div style={{ color:'rgba(160,184,216,0.5)', fontSize:'8px', fontWeight:700, textTransform:'uppercase', letterSpacing:'1px', marginBottom:'1px' }}>Drop Rates</div>
            {(Object.entries(RARITY_WEIGHTS) as [Rarity, number][]).map(([rarity, weight]) => {
              const style = RARITY_STYLE[rarity];
              return (
                <div key={rarity} style={{ display:'flex', alignItems:'center', gap:'5px' }}>
                  <div style={{ width:'52px', fontSize:'8px', fontWeight:700, color: style.accent, textTransform:'uppercase', letterSpacing:'0.5px' }}>{style.label}</div>
                  <div style={{ flex:1, height:'5px', borderRadius:'3px', background:'rgba(255,255,255,0.06)', overflow:'hidden' }}>
                    <div style={{ height:'100%', width:`${weight}%`, borderRadius:'3px', background:`linear-gradient(90deg, ${style.accent}99, ${style.accent})`, boxShadow:`0 0 4px ${style.glow}` }} />
                  </div>
                  <div style={{ width:'28px', fontSize:'8px', fontWeight:700, color:'rgba(200,215,235,0.5)', textAlign:'right' }}>{weight}%</div>
                </div>
              );
            })}
          </div>
          {lastDrawAnim && (
            roundNumber === INITIAL_DEAL_ROUND ? (
              <div style={{ textAlign:'center', padding:'4px 8px', borderRadius:'5px', background:'rgba(245,158,11,0.15)', border:'1px solid rgba(245,158,11,0.5)', animation:'pulse 0.5s ease infinite' }}>
                <span style={{ fontSize:'10px', fontWeight:800, color:'#f59e0b' }}>🃏 Both players received 3 starter cards!</span>
              </div>
            ) : (
              <div style={{ textAlign:'center', padding:'4px 8px', borderRadius:'5px', background:`${RARITY_STYLE[lastDrawAnim.rarity].accent}22`, border:`1px solid ${RARITY_STYLE[lastDrawAnim.rarity].accent}66`, animation:'pulse 0.5s ease infinite' }}>
                <span style={{ fontSize:'10px', fontWeight:800, color:RARITY_STYLE[lastDrawAnim.rarity].accent }}>🃏 Both players drew a {RARITY_STYLE[lastDrawAnim.rarity].label} card!</span>
              </div>
            )
          )}
        </div>

        <div style={{ flex: 1, display: 'flex', flexDirection: 'column', minHeight: 0 }}>
          <GamePanel
            chatMessages={chatMessages}
            onSendMessage={(text) => {
              if (authoritativeMatchIdRef.current) {
                void submitAuthoritativeIntent({ type: 'send_chat', ...authoritativeActorForColor(controlSender), text });
              } else {
                setChatMessages((prev: any) => [...prev, { sender: controlSender, text }]);
              }
            }}
            isChatDisabled={hostedActionLocked}
            movHist={movHist}
            engineNode={
              engineOn && ev ? (
                <div style={{ padding: '12px', fontFamily: 'monospace' }}>
                  <div style={{ fontSize: '22px', fontWeight: 'bold', color: ev.score > 0 ? '#2ecc71' : ev.score < 0 ? '#e74c3c' : '#ecf0f1', textAlign: 'center', marginBottom: '8px' }}>
                    {ev.mate != null ? (ev.mate === 0 ? 'Mate' : `M${Math.abs(ev.mate)}`) : (ev.score / 100).toFixed(2)}
                  </div>
                  {ev.best && (
                    <div style={{ color: '#f39c12', textAlign: 'center', fontSize: '13px' }}>
                      Best: {uciToSan(ev.best, reviewIdx >= 0 ? (reviewBoard ?? board) : board)} <span style={{ color: '#7f8c8d', fontSize: '10px' }}>({ev.best})</span>
                    </div>
                  )}
                </div>
              ) : (
                <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', height: '100%', gap: '8px' }}>
                  <div style={{ color: 'rgba(255,255,255,0.55)', fontSize: '12px' }}>Engine {engineOn ? 'calculating...' : 'off'}</div>
                  {over && (
                    <button onClick={() => setEngineOn(v => !v)} style={{ padding: '6px 14px', fontSize: '11px', background: engineOn ? 'linear-gradient(180deg,#1a6fc4,#0d4a8a)' : 'rgba(60,70,90,0.6)', color: '#fff', border: 'none', borderRadius: '6px', cursor: 'pointer', fontWeight: 'bold' }}>
                      {engineOn ? 'ENGINE ON' : 'ENGINE OFF'}
                    </button>
                  )}
                </div>
              )
            }
          />
        </div>
        <div style={{ marginTop:'7px', textAlign:'center', color:'rgba(160,184,216,0.3)', fontSize:'8px' }}>&nbsp;</div>

        {/* Game controls */}
        <div style={{ display:'flex', flexDirection:'column', gap:'6px', flexShrink:0 }}>
          {drawOffer && drawOffer !== turn && canRespondToDrawOffer && (
            <div style={{ background:'rgba(243,156,18,0.12)', border:'1px solid rgba(243,156,18,0.45)', borderRadius:'7px', padding:'8px 10px' }}>
              <div style={{ marginBottom:'6px', fontWeight:'bold', fontSize:'12px', color:'#f39c12' }}>{drawOffer==='white'?'⚪ White':'⚫ Black'} offers a draw</div>
              <div style={{ display:'flex', gap:'6px' }}>
                <button onClick={() => {
                  if (authoritativeMatchIdRef.current) {
                    void submitAuthoritativeIntent({ type: 'respond_draw', ...authoritativeActorForColor(controlSender), accept: true });
                    return;
                  }
                  finalPositionRef.current = { fen: toFEN(board, turn, moved, lm, hmc, fmn), turn };
                  setOver(true);
                  setWinner('draw');
                  setDrawOffer(null);
                }} style={{ flex:1, padding:'5px', background:'linear-gradient(180deg,#27ae60,#1e8449)', color:'#fff', border:'none', borderRadius:'5px', cursor:'pointer', fontWeight:'bold', fontSize:'12px' }}>✓ Accept</button>
                <button onClick={() => {
                  if (authoritativeMatchIdRef.current) {
                    void submitAuthoritativeIntent({ type: 'respond_draw', ...authoritativeActorForColor(controlSender), accept: false });
                    return;
                  }
                  setDrawOffer(null);
                }} style={{ flex:1, padding:'5px', background:'linear-gradient(180deg,#c0392b,#96281b)', color:'#fff', border:'none', borderRadius:'5px', cursor:'pointer', fontWeight:'bold', fontSize:'12px' }}>✕ Decline</button>
              </div>
            </div>
          )}

          {abortActive && !over && !authoritativeMatchId && (
            <div style={{ padding:'8px 12px', borderRadius:'6px', textAlign:'center', background:'linear-gradient(90deg, rgba(180,30,20,0.95) 0%, rgba(220,50,35,0.95) 100%)', border:'1px solid rgba(231,76,60,0.5)', boxShadow:'0 0 14px rgba(231,76,60,0.3)' }}>
              <div style={{ color:'#fff', fontWeight:800, fontSize:'13px', marginBottom:'6px' }}>⚡ {movHist.length===0?'White':'Black'} must move — {abortCountdown}s left</div>
              <div style={{ height:'5px', borderRadius:'3px', background:'rgba(0,0,0,0.35)', overflow:'hidden' }}>
                <div style={{ height:'100%', borderRadius:'3px', width:`${(abortCountdown/ABORT_SECS)*100}%`, background: abortCountdown<=3?'#ff4444':abortCountdown<=6?'#f39c12':'#2ecc71', transition:'width 0.9s linear, background 0.3s' }} />
              </div>
            </div>
          )}

          <div style={{ padding:'8px 10px', background:'rgba(0,0,0,0.2)', border:'1px solid rgba(255,165,40,0.12)', borderRadius:'8px', textAlign:'center', fontSize:'12px', fontWeight:'bold', color:'#fff' }}>
            {over ? (
              <div>
                <div style={{ fontSize:'13px', marginBottom:'2px' }}>
                  {winner==='aborted' ? <span style={{ color:'#e74c3c' }}>Game Aborted 🚫</span>
                  : winner==='draw'   ? <span style={{ color:'#f39c12' }}>Draw! 🤝</span>
                  : <span style={{ color:'#2ecc71' }}>{winner==='white'?'⚪ White':'⚫ Black'} Wins! 🏆</span>}
                </div>
                {activeFinishReasonLabel ? (
                  <div style={{ fontSize:'10px', color: winner === 'draw' || winner === 'aborted' ? '#f39c12' : '#e8f7cf' }}>
                    by {activeFinishReasonLabel}
                  </div>
                ) : null}
              </div>
            ) : (
              <div>
                {check
                  ? <span style={{ color:'#ffaa00', fontSize:'11px' }}>⚠️ CHECK! {turn==='white'?'⚪ White':'⚫ Black'} to move</span>
                  : <span style={{ color:'#a0b8d8', fontSize:'11px' }}>Turn: {turn==='white'?'⚪ White':'⚫ Black'}</span>}
                {hmc > 40 && <div style={{ fontSize:'10px', color:'#f39c12', marginTop:'2px' }}>50-move rule: {50-Math.floor(hmc/2)} left</div>}
              </div>
            )}
          </div>

          <div style={{ display:'flex', gap:'6px' }}>
            {over ? (
              <>
                {winner !== 'aborted' && (
                  <button
                    disabled={authoritativeRematchBusy}
                    onClick={() => {
                      if (canCreateDirectRematch) {
                        void createAuthoritativeRematchRoom();
                        return;
                      }
                      if (canQueueSameLane) {
                        returnToSameQueueLane();
                        return;
                      }
                      if (hostedRuntime) {
                        returnToQueueHome();
                        return;
                      }
                      newGame();
                    }}
                    style={{
                      flex:1,
                      padding:'9px',
                      fontSize:'12px',
                      background:'linear-gradient(180deg,#7b2fd4,#4a1a8a)',
                      color:'#fff',
                      border:'1px solid rgba(150,80,255,0.5)',
                      borderRadius:'7px',
                      cursor: authoritativeRematchBusy ? 'default' : 'pointer',
                      fontWeight:'bold',
                      boxShadow:'0 2px 12px rgba(120,50,220,0.4)',
                      opacity: authoritativeRematchBusy ? 0.75 : 1,
                    }}
                  >
                    {finishedPrimaryActionLabel}
                  </button>
                )}
                <button
                  onClick={() => {
                    if (hostedRuntime) {
                      returnToQueueHome();
                      return;
                    }
                    newGame();
                  }}
                  style={{ flex:1, padding:'9px', fontSize:'12px', background:'linear-gradient(180deg,#1a8a40,#0f5a28)', color:'#fff', border:'1px solid rgba(46,204,113,0.4)', borderRadius:'7px', cursor:'pointer', fontWeight:'bold', boxShadow:'0 2px 12px rgba(30,140,70,0.4)' }}
                >
                  {finishedSecondaryActionLabel}
                </button>
              </>
            ) : (
              <>
                {movHist.length === 0 || (movHist.length === 1 && !movHist[0].b) ? (
                  <button disabled={hostedActionLocked} onClick={() => {
                    if (hostedActionLocked) {
                      return;
                    }
                    if (authoritativeMatchIdRef.current) {
                      stopAbortCountdown();
                      void submitAuthoritativeIntent({ type: 'abort', ...authoritativeActorForColor(controlSender) });
                      return;
                    }
                    stopAbortCountdown();
                    setWinner('aborted');
                    setOver(true);
                  }}
                    style={{ flex:1, padding:'9px', fontSize:'12px', background:'linear-gradient(180deg,#3a4055,#222638)', color:'#ccc', border:'1px solid rgba(255,255,255,0.1)', borderRadius:'7px', cursor:'pointer', fontWeight:'bold' }}>✕ Abort</button>
                ) : (
                  <button onClick={newGame} style={{ flex:1, padding:'9px', fontSize:'12px', background:'linear-gradient(180deg,#1a8a40,#0f5a28)', color:'#fff', border:'1px solid rgba(46,204,113,0.4)', borderRadius:'7px', cursor:'pointer', fontWeight:'bold', boxShadow:'0 2px 12px rgba(30,140,70,0.4)' }}>♟ New Game</button>
                )}
          <button disabled={hostedActionLocked} onClick={() => {
            if (hostedActionLocked) {
              return;
            }
            if (window.confirm("Are you sure you want to resign?")) {
              if (authoritativeMatchIdRef.current) {
                void submitAuthoritativeIntent({ type: 'resign', ...authoritativeActorForColor(controlSender) });
                return;
              }
              finalPositionRef.current = { fen: toFEN(board, turn, moved, lm, hmc, fmn), turn };
              setOver(true);
              setWinner(OPP[turn]);
            }
          }}
          style={{
            flex:1, padding:'9px', fontSize:'12px',
            background: 'linear-gradient(180deg,#8a1a1a,#5a0f0f)',
            color:'#fff',
            border: '1px solid rgba(220,60,60,0.4)',
            borderRadius:'7px', cursor:'pointer', fontWeight:'bold',
            boxShadow: '0 2px 12px rgba(180,30,30,0.4)'
          }}>🏳 Resign</button>
          {!drawOffer
            ? <button disabled={hostedActionLocked} onClick={() => {
              if (hostedActionLocked) return;
              const now = Date.now();
              if (now - lastDrawOfferTime.current < DRAW_COOLDOWN_MS) return;
              lastDrawOfferTime.current = now;
              if (authoritativeMatchIdRef.current) {
              void submitAuthoritativeIntent({ type: 'offer_draw', ...authoritativeActorForColor(controlSender) });
                return;
              }
              setDrawOffer(turn);
            }} style={{ flex:1, padding:'9px', fontSize:'12px', background:'linear-gradient(180deg,#8a6010,#5a3e08)', color:'#fff', border:'1px solid rgba(240,160,30,0.4)', borderRadius:'7px', cursor:'pointer', fontWeight:'bold', boxShadow:'0 2px 12px rgba(180,120,20,0.4)' }}>🤝 Draw</button>
            : <button disabled style={{ flex:1, padding:'9px', fontSize:'12px', background:'rgba(60,60,80,0.35)', color:'rgba(255,255,255,0.3)', border:'1px solid rgba(255,255,255,0.08)', borderRadius:'7px', fontWeight:'bold', cursor:'not-allowed' }}>Draw sent…</button>
          }
              </>
            )}
          </div>
        </div>

        <div style={{ display:'flex', justifyContent:'center', gap:'6px', flexShrink:0, marginTop:'4px', flexWrap:'wrap' }}>
          <button onClick={toggleSound} style={{
            padding:'5px 10px', fontSize:'10px', fontWeight:700, cursor:'pointer',
            background: soundEnabled ? 'rgba(74,222,128,0.12)' : 'rgba(100,100,120,0.15)',
            color: soundEnabled ? '#86efac' : 'rgba(200,200,220,0.5)',
            border: soundEnabled ? '1px solid rgba(74,222,128,0.3)' : '1px solid rgba(200,200,220,0.08)',
            borderRadius:'6px',
          }}>
            {soundEnabled ? '🔊 Sound On' : '🔇 Sound Off'}
          </button>
          <button onClick={toggleColorBlind} style={{
            padding:'5px 10px', fontSize:'10px', fontWeight:700, cursor:'pointer',
            background: colorBlindMode ? 'rgba(96,165,250,0.12)' : 'rgba(100,100,120,0.15)',
            color: colorBlindMode ? '#93c5fd' : 'rgba(200,200,220,0.5)',
            border: colorBlindMode ? '1px solid rgba(96,165,250,0.3)' : '1px solid rgba(200,200,220,0.08)',
            borderRadius:'6px',
          }}>
            {colorBlindMode ? '🎨 CB On' : '🏳 CB Off'}
          </button>
          <div style={{ padding:'5px 10px', fontSize:'9px', color:'rgba(160,184,216,0.55)' }}>
            ← → review · Esc cancel
          </div>
        </div>

      </div>
    </div>
  );
}
