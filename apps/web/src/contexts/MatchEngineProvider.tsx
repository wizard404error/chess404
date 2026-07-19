'use client';

import React, { createContext, useContext } from 'react';
import { useMatchEngineFacade, type UseMatchEngineProps } from '../hooks/useMatchEngineFacade';
import MatchStateContext from './MatchStateContext';
import MatchAnimContext from './MatchAnimContext';
import MatchCardContext from './MatchCardContext';

interface MatchEngineProviderProps extends UseMatchEngineProps {
  children: React.ReactNode;
}

const MatchEngineRawContext = createContext<ReturnType<typeof useMatchEngineFacade> | null>(null);
const MatchEnginePropsContext = createContext<UseMatchEngineProps | null>(null);

export function useMatchEngineContext() {
  const ctx = useContext(MatchEngineRawContext);
  if (!ctx) throw new Error('useMatchEngineContext must be used within MatchEngineProvider');
  return ctx;
}

export function useMatchEnginePropsContext(): UseMatchEngineProps {
  const ctx = useContext(MatchEnginePropsContext);
  if (!ctx) throw new Error('useMatchEnginePropsContext must be used within MatchEngineProvider');
  return ctx;
}

export default function MatchEngineProvider(props: MatchEngineProviderProps) {
  const { children, hostedRuntime: runtime, viewerSeat: vSeat, authoritativeRematchBusy: rematchBusy, ...restHookProps } = props;
  const hookProps = { hostedRuntime: runtime, viewerSeat: vSeat, authoritativeRematchBusy: rematchBusy, ...restHookProps } as UseMatchEngineProps;
  const engine = useMatchEngineFacade(hookProps);

  const stateValue = React.useMemo(() => ({
    hostedRuntime: runtime,
    viewerSeat: vSeat,
    authoritativeRematchBusy: rematchBusy,
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
    authoritativeMatchId: engine.authoritativeMatchId,
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
    snapshots: engine.snapshots,
    reviewPrev: engine.reviewPrev,
    reviewNext: engine.reviewNext,
    visibleSocialAlert: engine.visibleSocialAlert,
    handleSocialAlertAction: engine.handleSocialAlertAction,
    dismissSocialAlert: engine.dismissSocialAlert,
    activeMatchRoomMeta: engine.activeMatchRoomMeta,
    primaryAccountIdentity: engine.primaryAccountIdentity,
    setAuthoritativeMatchId: engine.setAuthoritativeMatchId,
    shellAccountNotice: engine.shellAccountNotice,
    setShellAccountNotice: engine.setShellAccountNotice,
    hasPrimaryAccountSession: engine.hasPrimaryAccountSession,
    handlePrimaryShellAuthenticated: engine.handlePrimaryShellAuthenticated,
    handleSeatAuthenticated: engine.handleSeatAuthenticated,
    syncPrimaryAccountIdentity: engine.syncPrimaryAccountIdentity,
    requestedMatchIdRef: engine.requestedMatchIdRef,
    showReturnToMatch: engine.showReturnToMatch,
    copyLiveMatchLink: engine.copyLiveMatchLink,
    openLiveMatch: engine.openLiveMatch,
    openReplayMatch: engine.openReplayMatch,
    openProfileHandle: engine.openProfileHandle,
    openGuestHistory: engine.openGuestHistory,
    primaryNavItems: engine.primaryNavItems,
    secondaryNavItems: engine.secondaryNavItems,
    shellPageMeta: engine.shellPageMeta,
    utilityGroups: engine.utilityGroups,
    activeSecondaryNav: engine.activeSecondaryNav,
    showPlayHub: engine.showPlayHub,
    showBoardSurface: engine.showBoardSurface,
    cardAnim: engine.cardAnim,
    cardAnimLbl: engine.cardAnimLbl,
    renderJokerPicker: engine.renderJokerPicker,
    engineOn: engine.engineOn,
    setEngineOn: engine.setEngineOn,
    finalPositionRef: engine.finalPositionRef,
    setPromo: engine.setPromo,
    controlSender: engine.controlSender,
  }), [engine, runtime, vSeat, rematchBusy]);

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
    <MatchEnginePropsContext.Provider value={hookProps}>
      <MatchStateContext.Provider value={stateValue}>
        <MatchAnimContext.Provider value={animValue}>
          <MatchCardContext.Provider value={cardValue}>
            <MatchEngineRawContext.Provider value={engine}>
              {children}
            </MatchEngineRawContext.Provider>
          </MatchCardContext.Provider>
        </MatchAnimContext.Provider>
      </MatchStateContext.Provider>
    </MatchEnginePropsContext.Provider>
  );
}
