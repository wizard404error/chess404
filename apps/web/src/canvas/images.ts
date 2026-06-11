import type { PieceColor, PieceType } from '../types';

// Preload all piece images once
export const PIECE_IMAGES: Partial<Record<string, HTMLImageElement>> = {};
export const PIECE_KEYS = (['white','black'] as PieceColor[]).flatMap(color =>
  (['king','queen','rook','bishop','knight','pawn'] as PieceType[]).map(type => `${color}_${type}`)
);
if (typeof Image !== 'undefined') {
  PIECE_KEYS.forEach(key => {
    const img = new Image();
    img.onerror = () => {
      delete PIECE_IMAGES[key];
    };
    img.src = `/pieces/${key}.svg`;
    PIECE_IMAGES[key] = img;
  });
}

// ─── Fused piece images ───────────────────────────────────────────────────────
// Key format: `${color}_${typeA}_${typeB}` where typeA < typeB alphabetically
// so knight+rook and rook+knight both resolve to the same image key
export const FUSED_IMAGES: Partial<Record<string, HTMLImageElement>> = {};
export const KNOWN_FUSED_COMBOS: [PieceType, PieceType][] = [
  ['knight', 'rook'],
  // add more here as sprites are added e.g. ['knight', 'bishop'], ['pawn', 'rook']
];
if (typeof Image !== 'undefined') {
  (['white','black'] as PieceColor[]).forEach(color => {
    KNOWN_FUSED_COMBOS.forEach(([a, b]) => {
      const key = `${color}_${a}_${b}`; // always alphabetical: a < b
      const img = new Image();
      img.onerror = () => {
        delete FUSED_IMAGES[key];
      };
      img.src = `/pieces/${key}.png`;
      FUSED_IMAGES[key] = img;
    });
  });
}

export function isUsableImage(img: HTMLImageElement | null | undefined): img is HTMLImageElement {
  return Boolean(img && img.complete && img.naturalWidth > 0 && img.naturalHeight > 0);
}

// Returns the canonical fused image key (alphabetical type order) or null if not loaded
export function getFusedImage(color: PieceColor, typeA: PieceType, typeB: PieceType): HTMLImageElement | null {
  const [t1, t2] = [typeA, typeB].sort() as [PieceType, PieceType];
  const key = `${color}_${t1}_${t2}`;
  const img = FUSED_IMAGES[key];
  return isUsableImage(img) ? img : null;
}
