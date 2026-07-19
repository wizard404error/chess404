import type { CardHandler } from './types';

/**
 * Registry of all 37 card mechanic handlers.
 * Each handler takes (ctx: CardContext) and returns a CardResult or null.
 * null means "no special handling needed — use default behavior".
 */
export const CARD_HANDLERS: Record<string, CardHandler> = {
  freeze: (ctx) => {
    const { piece, opp, row, col } = ctx as any;
    if (!piece || piece.color !== opp || piece.type === 'king') {
      return { cardMsg: 'Click an ENEMY piece (not king) to freeze it!', finishCard: true };
    }
    return null; // default freeze behavior
  },

  shield: (ctx) => {
    const { piece, playerColor } = ctx as any;
    if (!piece || piece.color !== playerColor || piece.type === 'king') {
      return { cardMsg: 'Click YOUR piece (not king) to shield it!', finishCard: true };
    }
    return null;
  },

  sniper: (ctx) => {
    const { piece, opp } = ctx as any;
    if (!piece || piece.color !== opp || piece.type === 'king') {
      return { cardMsg: 'Click an ENEMY piece (not king) to snipe it!', finishCard: true };
    }
    return null;
  },

  badsniper: (ctx) => {
    const { piece, playerColor } = ctx as any;
    if (!piece || piece.color !== playerColor || piece.type === 'king') {
      return { cardMsg: 'Click YOUR piece (not king) to bad-snipe it!', finishCard: true };
    }
    return null;
  },

  teleport: (ctx) => {
    const { step, data, piece, from } = ctx as any;
    if (step === 1 && !piece) {
      return { cardMsg: 'Click your piece to teleport!', finishCard: true };
    }
    if (step === 2 && piece) {
      return { cardMsg: 'Target square is occupied!', finishCard: true };
    }
    return null;
  },

  jump: () => null,

  swapme: (ctx) => {
    const { piece, playerColor } = ctx as any;
    if (step(ctx) === 1) {
      if (!piece || piece.color !== playerColor) {
        return { cardMsg: 'Click YOUR first piece to swap!', finishCard: true };
      }
    } else {
      if (!piece || piece.color !== playerColor) {
        return { cardMsg: 'Click YOUR second piece to swap with!', finishCard: true };
      }
    }
    return null;
  },

  swapus: (ctx) => {
    const { piece, playerColor } = ctx as any;
    if (step(ctx) === 1) {
      if (!piece || piece.color !== playerColor) {
        return { cardMsg: 'Click YOUR piece to swap!', finishCard: true };
      }
    } else {
      if (!piece || piece.color === playerColor) {
        return { cardMsg: 'Click an ENEMY piece to swap with!', finishCard: true };
      }
    }
    return null;
  },

  swaphim: (ctx) => {
    const { piece, opp } = ctx as any;
    if (!piece || piece.color !== opp) {
      return { cardMsg: step(ctx) === 1 ? 'Click first ENEMY piece!' : 'Click second ENEMY piece!', finishCard: true };
    }
    return null;
  },

  borrow: (ctx) => {
    const { piece, opp } = ctx as any;
    if (!piece || piece.color !== opp || piece.type === 'king') {
      return { cardMsg: 'Click an ENEMY piece (not king) to borrow!', finishCard: true };
    }
    return null;
  },

  mindcontrol: (ctx) => {
    const { piece, opp } = ctx as any;
    if (!piece || piece.color !== opp || piece.type === 'king') {
      return { cardMsg: 'Click an ENEMY piece (not king) to steal!', finishCard: true };
    }
    return null;
  },

  parasite: (ctx) => {
    const { piece, playerColor, step: s } = ctx as any;
    if (s === 1) {
      if (!piece || piece.color !== playerColor) {
        return { cardMsg: 'Click YOUR piece to be the host!', finishCard: true };
      }
    } else {
      if (!piece || piece.color === playerColor) {
        return { cardMsg: 'Click an ENEMY piece with the same value!', finishCard: true };
      }
    }
    return null;
  },

  clone: () => null,

  fakepiece: () => null,

  promote: (ctx) => {
    const { piece, playerColor } = ctx as any;
    if (!piece || piece.color !== playerColor) {
      return { cardMsg: 'Click YOUR piece to promote!', finishCard: true };
    }
    return null;
  },

  demote: (ctx) => {
    const { piece, playerColor } = ctx as any;
    if (!piece || piece.color !== playerColor) {
      return { cardMsg: 'Click YOUR piece to demote!', finishCard: true };
    }
    return null;
  },

  promotehim: (ctx) => {
    const { piece, opp } = ctx as any;
    if (!piece || piece.color !== opp) {
      return { cardMsg: 'Click ENEMY piece to promote!', finishCard: true };
    }
    return null;
  },

  demotehim: (ctx) => {
    const { piece, opp } = ctx as any;
    if (!piece || piece.color !== opp) {
      return { cardMsg: 'Click ENEMY piece to demote!', finishCard: true };
    }
    return null;
  },

  lavaground: () => null,
  blackhole: () => null,
  fortress: () => null,
  fog_village: () => null,
  invisible: (ctx) => {
    const { piece, playerColor } = ctx as any;
    if (!piece || piece.color !== playerColor || piece.type === 'king') {
      return { cardMsg: 'Click YOUR piece (not king) to make invisible!', finishCard: true };
    }
    return null;
  },
  unabomber: () => null,
  halffuse: () => null,
  fullfusion: () => null,
  reverse: () => null,
  undo: () => null,
  mirror: () => null,
  gambler: () => null,
  radar: () => null,
  cheater: () => null,
  joker: () => null,
  doublemove_diff: () => null,
  doublemove_same: () => null,
};

function step(ctx: Record<string, unknown>): number {
  return (ctx.step as number) ?? 1;
}
