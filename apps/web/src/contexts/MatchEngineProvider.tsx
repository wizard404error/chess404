'use client';

import React from 'react';
import MatchStateContext from './MatchStateContext';
import MatchAnimContext from './MatchAnimContext';
import MatchCardContext from './MatchCardContext';

interface MatchEngineProviderProps {
  engine: ReturnType<typeof import('../hooks/useMatchEngine').useMatchEngine>;
  children: React.ReactNode;
  hostedRuntime: boolean | null;
  viewerSeat: import('../types').PieceColor | null;
  authoritativeRematchBusy: boolean;
}

export default function MatchEngineProvider({
  children, engine,
  hostedRuntime, viewerSeat, authoritativeRematchBusy,
}: MatchEngineProviderProps) {
  const stateValue = React.useMemo(() => ({
    board: engine.board,
    turn: engine.turn,
    sel: engine.sel,
    hints: engine.hints,
    lm: engine.lm,
    drag: engine.drag,
    dragPos: engine.dragPos,
    check: engine.check,
    kingPos: engine.kingPos,
    over: engine.over,
    winner: engine.winner,
    promo: engine.promo,
    promoPicker: engine.promoPicker,
    moved: engine.moved,
    hmc: engine.hmc,
    fmn: engine.fmn,
    posHist: engine.posHist,
    timeW: engine.timeW,
    timeB: engine.timeB,
    clockActive: engine.clockActive,
    tickingState: engine.tickingState,
    fmtClock: engine.fmtClock,
    hostedRuntime,
    authoritativeMatchId: engine.authoritativeMatchId,
    viewerSeat,
    authoritativeLive: engine.authoritativeLive,
    authoritativeStatus: engine.authoritativeStatus,
    topSeat: engine.topSeat,
    bottomSeat: engine.bottomSeat,
    topPlayerName: engine.topPlayerName,
    bottomPlayerName: engine.bottomPlayerName,
    topSeatBadge: engine.topSeatBadge,
    bottomSeatBadge: engine.bottomSeatBadge,
    displayedWhiteRating: engine.displayedWhiteRating,
    displayedBlackRating: engine.displayedBlackRating,
    displayedWhiteName: engine.displayedWhiteName,
    displayedBlackName: engine.displayedBlackName,
    whiteSeatBadge: engine.whiteSeatBadge,
    blackSeatBadge: engine.blackSeatBadge,
    doubleMove: engine.doubleMove,
    radarActive: engine.radarActive,
    lavaSqs: engine.lavaSquares,
    lavaExploding: engine.lavaExploding,
    fogZones: engine.fogZones,
    ghostPiece: engine.ghostPiece,
    ghostRef: engine.ghostRef,
    analysisArrows: engine.analysisArrows,
    premove: engine.premove,
    isReviewing: engine.isReviewing,
    reviewBoard: engine.reviewBoard,
    reviewIdx: engine.reviewIdx,
    drawOffer: engine.drawOffer,
    abortActive: engine.abortActive,
    abortCountdown: engine.abortCountdown,
    streamDisconnected: engine.streamDisconnected,
    setSel: engine.setSel,
    setHints: engine.setHints,
    setDrag: engine.setDrag,
    setDragPos: engine.setDragPos,
    setBoard: engine.setBoard,
    setOver: engine.setOver,
    setWinner: engine.setWinner,
    setPremove: engine.setPremove,
    setDrawOffer: engine.setDrawOffer,
    setPosHist: engine.setPosHist,
    clickSq: engine.clickSq,
    getMoves: engine.getMoves,
    doMove: engine.doMove,
    doPromo: engine.doPromo,
    handlePromoPick: engine.handlePromoPick,
    cardPromo: engine.cardPromo,
    setCardPromo: engine.setCardPromo,
    getCardHighlight: engine.getCardHighlight,
    getDoubleMoveHighlight: engine.getDoubleMoveHighlight,
    toggleAnalysisArrow: engine.toggleAnalysisArrow,
    clearAnalysisArrows: engine.clearAnalysisArrows,
    submitAuthoritativeIntent: engine.submitAuthoritativeIntent,
    authoritativeActorForColor: engine.authoritativeActorForColor,
    createAuthoritativeRematchRoom: engine.createAuthoritativeRematchRoom,
    stopAbortCountdown: engine.stopAbortCountdown,
    activeFinishReasonLabel: engine.activeFinishReasonLabel,
    authoritativeRematchBusy,
    canCreateDirectRematch: engine.canCreateDirectRematch,
    canQueueSameLane: engine.canQueueSameLane,
    returnToSameQueueLane: engine.returnToSameQueueLane,
    returnToQueueHome: engine.returnToQueueHome,
    newGame: engine.newGame,
    finishedPrimaryActionLabel: engine.finishedPrimaryActionLabel,
    finishedSecondaryActionLabel: engine.finishedSecondaryActionLabel,
    boardStatusLabel: engine.boardStatusLabel,
    bootstrapAuthoritativeMatch: engine.bootstrapAuthoritativeMatch,
    showHostedSoloBanner: engine.showHostedSoloBanner,
    showHostedReconnectWarning: engine.showHostedReconnectWarning,
    intentInFlight: engine.intentInFlight,
    activeDisconnectGraceFor: engine.activeDisconnectGraceFor,
    isAttackedWithFusion: engine.isAttackedWithFusion,
    checkEndGame: engine.checkEndGame,
    canRespondToDrawOffer: engine.canRespondToDrawOffer,
    hostedActionLocked: engine.hostedActionLocked,
    movHist: engine.movHist,
    roundNumber: engine.roundNumber,
    chatMessages: engine.chatMessages,
    setChatMessages: engine.setChatMessages,
  }), [engine, hostedRuntime, viewerSeat, authoritativeRematchBusy]);

  const animValue = React.useMemo(() => ({
    cardAnim: engine.cardAnim,
    cardAnimLbl: engine.cardAnimLbl,
    bombPieces: engine.bombPieces,
    bombExploding: engine.bombExploding,
    swapAnim: engine.swapAnim,
    transformAnim: engine.transformAnim,
    sniperAnim: engine.sniperAnim,
    teleportAnim: engine.teleportAnim,
    jumpAnim: engine.jumpAnim,
    sacrificeAnim: engine.sacrificeAnim,
    mindControlAnim: engine.mindControlAnim,
    fuseAnim: engine.fuseAnim,
  }), [engine]);

  const cardValue = React.useMemo(() => ({
    selectedCard: engine.selectedCard,
    setSelectedCard: engine.setSelectedCard,
    cardPending: engine.cardPending,
    whiteHand: engine.whiteHand,
    blackHand: engine.blackHand,
    topHand: engine.topHand,
    bottomHand: engine.bottomHand,
    cardUsedBy: engine.cardUsedBy,
    canUseCard: engine.canUseCard,
    lastDrawAnim: engine.lastDrawAnim,
    dealPhase: engine.dealPhase,
  }), [engine]);

  return (
    <MatchStateContext.Provider value={stateValue}>
      <MatchAnimContext.Provider value={animValue}>
        <MatchCardContext.Provider value={cardValue}>
          {children}
        </MatchCardContext.Provider>
      </MatchAnimContext.Provider>
    </MatchStateContext.Provider>
  );
}
