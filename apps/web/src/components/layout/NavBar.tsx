'use client';

import React from 'react';
import type { AppPage } from '../../App';

interface NavItem {
  key: string;
  label: string;
  badge?: number | string | null;
}

interface NavBarProps {
  activePage: string;
  primaryNavItems: readonly NavItem[];
  secondaryNavItems: readonly NavItem[];
  showReturnToMatch: boolean;
  hasPrimaryAccountSession: boolean;
  activeSecondaryNav: unknown;
  secondaryMenuOpen: boolean;
  setSecondaryMenuOpen: (v: boolean | ((prev: boolean) => boolean)) => void;
  setActivePage: (page: AppPage) => void;
  accountLabel: string;
}

export function NavBar({
  activePage, primaryNavItems, secondaryNavItems,
  showReturnToMatch, hasPrimaryAccountSession,
  activeSecondaryNav, secondaryMenuOpen, setSecondaryMenuOpen,
  setActivePage, accountLabel,
}: NavBarProps) {
  return (
    <nav style={{
      display: 'flex', alignItems: 'center', justifyContent: 'space-between',
      padding: '0 28px', minHeight: '62px', flexShrink: 0,
      background: 'rgba(8,4,20,0.82)',
      backdropFilter: 'blur(20px)',
      WebkitBackdropFilter: 'blur(20px)',
      borderBottom: '1px solid rgba(255,165,40,0.25)',
      boxShadow: '0 4px 32px rgba(0,0,0,0.5), inset 0 -1px 0 rgba(255,140,0,0.1)',
      position: 'relative', zIndex: 100,
      gap: '18px', flexWrap: 'wrap',
    }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: '12px', minWidth: '180px' }}>
        <div style={{
          width: '38px', height: '38px', borderRadius: '8px',
          background: 'linear-gradient(135deg, #c8860a 0%, #8b5e0a 100%)',
          display: 'flex', alignItems: 'center', justifyContent: 'center',
          fontSize: '20px', boxShadow: '0 0 18px rgba(200,134,10,0.6)',
          border: '1px solid rgba(255,180,60,0.5)',
        }}>♛</div>
        <span style={{ fontSize: '22px', fontWeight: 800, letterSpacing: '1px', color: '#fff1c7' }}>CardChess</span>
      </div>
      <div style={{ display: 'flex', alignItems: 'center', gap: '6px', flex: '1 1 auto', flexWrap: 'wrap' }}>
        {primaryNavItems.map((item, i) => (
          <button key={item.key} onClick={() => setActivePage(item.key as AppPage)} style={{
            padding: '8px 16px', fontSize: '13px', fontWeight: i === 0 ? 700 : 600,
            background: activePage === item.key ? 'linear-gradient(180deg, rgba(200,134,10,0.35) 0%, rgba(139,94,10,0.4) 100%)' : 'transparent',
            color: activePage === item.key ? '#ffd700' : 'rgba(200,185,140,0.8)',
            border: activePage === item.key ? '1px solid rgba(200,134,10,0.6)' : '1px solid transparent',
            borderRadius: '6px', cursor: 'pointer',
            borderBottom: activePage === item.key ? '2px solid #c8860a' : '2px solid transparent',
            transition: 'all 0.15s ease',
            display: 'flex', alignItems: 'center', gap: '8px',
          }}
            onMouseEnter={e => { if (activePage !== item.key) (e.target as HTMLButtonElement).style.color = '#ffd700'; }}
            onMouseLeave={e => { if (activePage !== item.key) (e.target as HTMLButtonElement).style.color = 'rgba(200,185,140,0.8)'; }}
          >
            <span>{item.label}</span>
          </button>
        ))}
      </div>
      <div style={{ display: 'flex', gap: '10px', minWidth: '180px', justifyContent: 'flex-end', alignItems: 'center', marginLeft: 'auto' }}>
        {showReturnToMatch ? (
          <button onClick={() => setActivePage('Match')} style={{
            padding: '8px 16px', fontSize: '12px', fontWeight: 800,
            background: activePage === 'Match' ? 'linear-gradient(180deg, rgba(58,110,210,0.9) 0%, rgba(28,54,112,0.95) 100%)' : 'rgba(58,110,210,0.12)',
            color: '#eff6ff', border: '1px solid rgba(122,166,255,0.34)', borderRadius: '8px', cursor: 'pointer',
          }}>
            Return To Match
          </button>
        ) : null}
        <button onClick={() => setSecondaryMenuOpen(current => !current)} style={{
          padding: '8px 14px', fontSize: '12px', fontWeight: 700,
          background: activeSecondaryNav || secondaryMenuOpen ? 'rgba(255,255,255,0.08)' : 'transparent',
          color: 'rgba(220,200,150,0.9)', border: '1px solid rgba(180,130,60,0.3)', borderRadius: '8px', cursor: 'pointer',
        }}>
          More
        </button>
        <button onClick={() => setActivePage('Account')} style={{
          padding: '8px 18px', fontSize: '13px', fontWeight: 700,
          background: 'linear-gradient(180deg, #c8860a 0%, #7a5008 100%)', color: '#fff8e0',
          border: '1px solid rgba(255,180,60,0.5)', borderRadius: '8px', cursor: 'pointer',
          boxShadow: '0 2px 14px rgba(200,134,10,0.5)',
        }}>{accountLabel}</button>
      </div>
      {secondaryMenuOpen ? (
        <div style={{
          position: 'absolute', top: 'calc(100% + 10px)', right: '28px',
          width: 'min(320px, calc(100vw - 32px))', padding: '12px', borderRadius: '16px',
          background: 'linear-gradient(180deg, rgba(14,18,30,0.98) 0%, rgba(9,12,20,0.99) 100%)',
          border: '1px solid rgba(255,165,40,0.18)',
          boxShadow: '0 18px 48px rgba(0,0,0,0.35)', display: 'grid', gap: '8px',
        }}>
          <div style={{ color: '#ffcf72', fontSize: '11px', fontWeight: 800, letterSpacing: '1.2px', textTransform: 'uppercase', padding: '4px 6px 8px' }}>
            Secondary Surfaces
          </div>
          {secondaryNavItems.map((item) => (
            <button key={item.key} onClick={() => setActivePage(item.key as AppPage)} style={{
              padding: '10px 12px', borderRadius: '10px',
              border: activePage === item.key ? '1px solid rgba(255,165,40,0.24)' : '1px solid rgba(255,255,255,0.06)',
              background: activePage === item.key ? 'rgba(200,134,10,0.14)' : 'rgba(255,255,255,0.03)',
              color: activePage === item.key ? '#fff2c8' : 'rgba(244,232,200,0.82)',
              display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '10px',
              cursor: 'pointer', fontWeight: 700, fontSize: '12px', textAlign: 'left',
            }}>
              <span>{item.label}</span>
              {item.badge ? (
                <span style={{
                  minWidth: '18px', padding: '1px 6px', borderRadius: '999px',
                  background: 'rgba(255,215,0,0.18)', border: '1px solid rgba(255,215,0,0.22)',
                  color: '#fff3cf', fontSize: '11px', fontWeight: 800, lineHeight: 1.4,
                }}>{item.badge}</span>
              ) : null}
            </button>
          ))}
        </div>
      ) : null}
    </nav>
  );
}
