'use client';

import React from 'react';
import type { GameCard, PieceColor } from '../../types';
import { MAX_HAND_SIZE, RARITY_STYLE } from '../../constants';
import { useMatchCard } from '../../contexts/MatchCardContext';
import { useMatchState } from '../../contexts/MatchStateContext';

interface CardHandProps {
  hand: GameCard[];
  playerColor: PieceColor;
  position: 'top' | 'bottom';
}

export default function CardHand({ hand, playerColor, position }: CardHandProps) {
  const { selectedCard, setSelectedCard, cardUsedBy, dealPhase, canUseCard } = useMatchCard();
  const { radarActive } = useMatchState();

  const CW = 56, CH = 78;
  const isBottom = position === 'bottom';
  const xStep  = hand.length > 1 ? Math.min(52, 500 / hand.length) : 0;
  const spread = hand.length > 1 ? Math.min(18, 60 / hand.length)  : 0;

  return (
    <div style={{
      position:'relative', height:'100px', width:'580px',
      display:'flex', alignItems: isBottom ? 'flex-end' : 'flex-start',
      justifyContent:'center',
      marginTop: isBottom ? '4px' : 0, marginBottom: isBottom ? 0 : '4px',
      overflow:'visible', zIndex:0,
    }}>
      {hand.map((card, i) => {
        const mid   = (hand.length - 1) / 2;
        const angle = hand.length > 1 ? ((i - mid) / Math.max(hand.length - 1, 1)) * spread : 0;
        const yOff  = hand.length > 1 ? Math.min(12, Math.abs(i - mid) * 3) : 0;
        const xOff  = (i - mid) * xStep;
        const isSelected = selectedCard?.id === card.id;
        const isJokerCard = card.mechanic === 'joker';

        if (!isBottom) {
          if (radarActive) {
            return (
              <div key={card.id} style={{
                position:'absolute', top:`${yOff}px`,
                left:`calc(50% + ${xOff}px - ${CW/2}px)`,
                width:`${CW}px`, height:`${CH}px`,
                transform:`rotate(${-angle}deg)`, transformOrigin:'50% -20%',
                borderRadius:'7px',
                boxShadow:`0 6px 24px rgba(0,0,0,0.8), 0 0 16px rgba(96,165,250,0.5)`,
                background:`linear-gradient(160deg, ${card.color} 0%, color-mix(in srgb, ${card.color} 60%, #000) 100%)`,
                border:`2px solid #60a5fa`, overflow:'hidden', zIndex:i,
                pointerEvents:'none', animation:'radarReveal 0.4s cubic-bezier(0.34,1.56,0.64,1)',
              }}>
                <div style={{ position:'absolute', inset:0, background:'rgba(96,165,250,0.08)', zIndex:0 }} />
                <div style={{ width:'100%', height:'38px', background:`radial-gradient(ellipse at 50% 30%, ${card.accent}44 0%, transparent 70%)`, display:'flex', alignItems:'center', justifyContent:'center', fontSize:'18px', borderBottom:`1px solid ${card.accent}33` }}>{card.icon}</div>
                <div style={{ padding:'2px 3px', fontSize:'6px', fontWeight:700, color:'#fff', textAlign:'center', lineHeight:'1.2' }}>{card.name}</div>
                <div style={{ margin:'2px 4px 0', padding:'1px 3px', background:`${card.accent}33`, border:`1px solid ${card.accent}55`, borderRadius:'3px', fontSize:'5px', color:card.accent, textAlign:'center', fontWeight:700, textTransform:'uppercase' }}>{card.type}</div>
                <div style={{ margin:'1px 4px 0', padding:'1px 2px', border:`1px solid ${RARITY_STYLE[card.rarity].accent}88`, borderRadius:'3px', fontSize:'4.5px', color:RARITY_STYLE[card.rarity].accent, textAlign:'center', fontWeight:800, textTransform:'uppercase' }}>{RARITY_STYLE[card.rarity].label}</div>
                <div style={{ position:'absolute', top:'2px', left:'2px', fontSize:'7px', background:'rgba(96,165,250,0.9)', borderRadius:'3px', padding:'1px 3px', color:'#fff', fontWeight:800 }}>📡</div>
              </div>
            );
          }
          return (
            <div key={card.id} style={{
              position:'absolute', top:`${yOff}px`,
              left:`calc(50% + ${xOff}px - ${CW/2}px)`,
              width:`${CW}px`, height:`${CH}px`,
              transform:`rotate(${-angle}deg)`, transformOrigin:'50% -20%',
              borderRadius:'7px', boxShadow:'0 6px 18px rgba(0,0,0,0.7)',
              background:'linear-gradient(160deg, #1a1a3e 0%, #0d0d1f 100%)',
              border:'1px solid rgba(80,80,160,0.45)', overflow:'hidden', zIndex:i, pointerEvents:'none',
            }}>
              <div style={{ position:'absolute', inset:0, backgroundImage:'repeating-linear-gradient(45deg, rgba(60,60,120,0.12) 0px, rgba(60,60,120,0.12) 2px, transparent 2px, transparent 10px)' }} />
              <div style={{ position:'absolute', inset:0, display:'flex', alignItems:'center', justifyContent:'center', fontSize:'22px', opacity:0.35 }}>♛</div>
              <div style={{ position:'absolute', inset:'4px', borderRadius:'5px', border:'1px solid rgba(100,100,200,0.25)' }} />
            </div>
          );
        }

        const canUse = canUseCard(card, playerColor);
        const alreadyUsedThisTurn = cardUsedBy[playerColor];
        return (
          <div key={card.id}
            style={{
              position:'absolute', bottom:`${yOff}px`,
              left:`calc(50% + ${xOff}px - ${CW/2}px)`,
              width:`${CW}px`, height:`${CH}px`,
              transform:`rotate(${angle}deg)`, transformOrigin:'50% 120%',
              cursor: !canUse ? 'not-allowed' : 'pointer',
              transition:'transform 0.18s ease, filter 0.18s ease',
              zIndex: isSelected ? 99 : i + 1,
              filter: isSelected
                ? `brightness(1.3) drop-shadow(0 0 14px ${card.accent}cc)`
                : !canUse ? 'brightness(0.45) saturate(0.3)' : 'none',
              borderRadius:'7px',
              boxShadow: isJokerCard && canUse
                ? `0 6px 18px rgba(0,0,0,0.7), 0 0 20px rgba(245,158,11,0.5), inset 0 1px 0 rgba(255,255,255,0.12)`
                : `0 6px 18px rgba(0,0,0,0.7), inset 0 1px 0 rgba(255,255,255,0.12)`,
              background:`linear-gradient(160deg, ${card.color} 0%, color-mix(in srgb, ${card.color} 60%, #000) 100%)`,
              border: isJokerCard && canUse ? `1px solid ${card.accent}99` : `1px solid ${card.accent}55`,
              overflow:'visible',
              animation: isJokerCard && canUse ? 'jokerFloat 3s ease-in-out infinite' : 'none',
            }}
            onClick={() => {
              if (!canUse) return;
              setSelectedCard(isSelected ? null : card);
            }}
            onMouseEnter={e => {
              if (!canUse) return;
              const el = e.currentTarget as HTMLDivElement;
              el.style.transform = `rotate(${angle}deg) translateY(-20px) scale(1.08)`;
              el.style.zIndex = '99';
              const tip = el.querySelector('.card-tooltip') as HTMLElement;
              if (tip) tip.style.display = 'block';
            }}
            onMouseLeave={e => {
              const el = e.currentTarget as HTMLDivElement;
              el.style.transform = `rotate(${angle}deg)`;
              el.style.zIndex = String(isSelected ? 99 : i + 1);
              const tip = el.querySelector('.card-tooltip') as HTMLElement;
              if (tip) tip.style.display = 'none';
            }}
          >
            <div style={{ width:'100%', height:'38px', background:`radial-gradient(ellipse at 50% 30%, ${card.accent}44 0%, transparent 70%)`, display:'flex', alignItems:'center', justifyContent:'center', fontSize:'20px', borderBottom:`1px solid ${card.accent}33`, position:'relative' }}>
              {card.icon}
              {isJokerCard && canUse && (
                <>
                  {[0,1,2].map(j => (
                    <div key={j} style={{
                      position:'absolute', top:`${5+j*7}px`, left:`${8+j*15}px`,
                      width:'4px', height:'4px', borderRadius:'50%',
                      background:'#f59e0b',
                      animation:`jokerGlitter ${1.2+j*0.4}s ease-in-out infinite`,
                      animationDelay:`${j*0.35}s`, pointerEvents:'none',
                    }}/>
                  ))}
                </>
              )}
            </div>
            <div style={{ padding:'3px 4px 1px', fontSize:'6.5px', fontWeight:700, color:'#fff', textAlign:'center', lineHeight:'1.2' }}>{card.name}</div>
            <div style={{ margin:'2px 4px 0', padding:'1px 3px', background:`${card.accent}33`, border:`1px solid ${card.accent}55`, borderRadius:'3px', fontSize:'5.5px', color:card.accent, textAlign:'center', fontWeight:700, textTransform:'uppercase' }}>{card.type}</div>
            <div style={{
              margin:'1px 4px 0', padding:'1px 3px',
              border:`1px solid ${RARITY_STYLE[card.rarity].accent}88`,
              borderRadius:'3px', fontSize:'5px',
              color: RARITY_STYLE[card.rarity].accent,
              textAlign:'center', fontWeight:800, textTransform:'uppercase', letterSpacing:'0.3px',
              boxShadow: card.rarity === 'legendary' ? `0 0 6px ${RARITY_STYLE[card.rarity].glow}` : card.rarity === 'epic' ? `0 0 4px ${RARITY_STYLE[card.rarity].glow}` : 'none',
            }}>{RARITY_STYLE[card.rarity].label}</div>
            <div style={{ position:'absolute', inset:0, borderRadius:'7px', background:'linear-gradient(135deg, rgba(255,255,255,0.07) 0%, transparent 50%)', pointerEvents:'none' }} />
            <div style={{ position:'absolute', top:'3px', right:'3px', width:'6px', height:'6px', borderRadius:'50%', background:card.accent, boxShadow:`0 0 4px ${card.accent}` }} />
            {!canUse && (
              <div style={{ position:'absolute', inset:0, display:'flex', alignItems:'center', justifyContent:'center', borderRadius:'7px', background:'rgba(0,0,0,0.25)' }}>
                <span style={{ fontSize:'14px', opacity:0.7 }}>{alreadyUsedThisTurn ? '✓' : card.type === 'trap' ? '' : '🔒'}</span>
              </div>
            )}
            <div className="card-tooltip" style={{
              display:'none', position:'absolute', bottom:'calc(100% + 8px)', left:'50%',
              transform:'translateX(-50%)', minWidth:'180px', maxWidth:'240px',
              padding:'10px 12px', borderRadius:'10px', zIndex:999,
              background:'linear-gradient(180deg, rgba(10,14,26,0.98) 0%, rgba(6,8,16,0.99) 100%)',
              border:`1px solid ${RARITY_STYLE[card.rarity].accent}88`,
              boxShadow:`0 8px 32px rgba(0,0,0,0.7), 0 0 16px ${RARITY_STYLE[card.rarity].glow}`,
              pointerEvents:'none',
            }}>
              <div style={{ display:'flex', alignItems:'center', gap:'8px', marginBottom:'6px' }}>
                <span style={{ fontSize:'18px' }}>{card.icon}</span>
                <div style={{ fontWeight:800, fontSize:'12px', color:'#fff' }}>{card.name}</div>
              </div>
              <div style={{ display:'flex', gap:'4px', marginBottom:'5px', flexWrap:'wrap' }}>
                <span style={{ padding:'1px 6px', borderRadius:'3px', fontSize:'8px', fontWeight:800, color: RARITY_STYLE[card.rarity].accent, background:`${RARITY_STYLE[card.rarity].accent}22`, border:`1px solid ${RARITY_STYLE[card.rarity].accent}55`, textTransform:'uppercase', letterSpacing:'0.5px' }}>
                  {RARITY_STYLE[card.rarity].label}
                </span>
                <span style={{ padding:'1px 6px', borderRadius:'3px', fontSize:'8px', fontWeight:700, color:card.accent, background:`${card.accent}22`, border:`1px solid ${card.accent}55`, textTransform:'capitalize' }}>
                  {card.mechanic}
                </span>
              </div>
              <div style={{ fontSize:'10px', color:'rgba(200,210,230,0.9)', lineHeight:1.5, fontWeight:500 }}>
                {card.desc}
              </div>
            </div>
          </div>
        );
      })}
      {hand.length === 0 && dealPhase === 'done' && (
        <div style={{ color:'rgba(255,255,255,0.55)', fontSize:'11px', [isBottom ? 'marginBottom' : 'marginTop']:'28px' }}>
          No cards in hand
        </div>
      )}
      {isBottom && hand.length > 0 && (
        <div style={{
          position:'absolute', bottom:'-2px', right:'0',
          background: hand.length >= MAX_HAND_SIZE
            ? 'rgba(231,76,60,0.9)'
            : hand.length >= MAX_HAND_SIZE - 2 ? 'rgba(243,156,18,0.85)' : 'rgba(30,50,80,0.7)',
          color:'#fff', fontSize:'9px', fontWeight:800,
          padding:'2px 7px', borderRadius:'8px', border:'1px solid rgba(255,255,255,0.15)', zIndex:200,
        }}>
          {hand.length}/{MAX_HAND_SIZE}{hand.length >= MAX_HAND_SIZE ? ' 🔴 FULL' : ''}
        </div>
      )}
    </div>
  );
}
