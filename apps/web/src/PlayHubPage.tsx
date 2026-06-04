'use client';

import React from 'react';
import { type MatchModeId, type PieceColor } from '@chess404/contracts';
import type { GuestProfile } from './lib/platform-service';
import type { QueueName, QueueTicket } from './lib/matchmaking-service';
import type { PrivateMatchIdentity } from './lib/private-match-service';
import { modeLabel, queueLabel } from './lib/match-labels';
import QueuePage from './QueuePage';
import LobbiesPage from './LobbiesPage';

interface PlayHubPageProps {
  hostedRuntime: boolean | null;
  whiteProfile: GuestProfile | null;
  blackProfile: GuestProfile | null;
  preferredQueue?: QueueName | null;
  preferredModeId?: MatchModeId | null;
  queueRecovery?: {
    white: QueueTicket | null;
    black: QueueTicket | null;
  } | null;
  identity: PrivateMatchIdentity | null;
  displayName?: string | null;
  activeMatchId?: string | null;
  activeMatchQueue?: QueueName | 'direct' | null;
  activeMatchModeId?: MatchModeId | null;
  boardStatusLabel: string;
  viewerSeat?: PieceColor | null;
  matchDestinationNotice?: string | null;
  onReturnToMatch?: () => void;
  onCopyMatchLink?: (matchId: string) => void;
}


function viewerRoleLabel(viewerSeat?: PieceColor | null): string {
  if (viewerSeat === 'white') {
    return 'Playing as White';
  }
  if (viewerSeat === 'black') {
    return 'Playing as Black';
  }
  return 'Spectating read-only';
}

export default function PlayHubPage({
  hostedRuntime,
  whiteProfile,
  blackProfile,
  preferredQueue = null,
  preferredModeId = null,
  queueRecovery = null,
  identity,
  displayName = null,
  activeMatchId = null,
  activeMatchQueue = null,
  activeMatchModeId = null,
  boardStatusLabel,
  viewerSeat = null,
  matchDestinationNotice = null,
  onReturnToMatch,
  onCopyMatchLink,
}: PlayHubPageProps): React.ReactElement {
  return (
    <div style={{ flex: 1, minHeight: 0, overflowY: 'auto', padding: '24px 28px 32px', color: '#f4e8c8' }}>
      <div style={{ maxWidth: '1380px', margin: '0 auto', display: 'grid', gap: '18px' }}>
        <div style={{
          padding: '22px 24px',
          borderRadius: '18px',
          background: 'linear-gradient(180deg, rgba(14,18,30,0.98) 0%, rgba(10,14,24,0.96) 100%)',
          border: '1px solid rgba(255,165,40,0.16)',
          boxShadow: '0 18px 50px rgba(0,0,0,0.28)',
        }}>
          <div style={{ color: '#ffcf72', fontSize: '12px', fontWeight: 800, letterSpacing: '1.4px', textTransform: 'uppercase', marginBottom: '8px' }}>
            Play
          </div>
          <div style={{ color: '#fff4d6', fontSize: '30px', fontWeight: 900 }}>
            Queue into a real match or create a private invite
          </div>
          <div style={{ color: 'rgba(244,232,200,0.72)', fontSize: '14px', lineHeight: 1.7, marginTop: '8px', maxWidth: '820px' }}>
            Choose an official mode, enter casual or rated quick pair, or open one clean invite room for a friend. The live board only opens when a real match is ready.
          </div>
        </div>

        {activeMatchId ? (
          <div style={{
            padding: '18px 20px',
            borderRadius: '18px',
            background: 'linear-gradient(180deg, rgba(18,30,48,0.96) 0%, rgba(10,18,30,0.98) 100%)',
            border: '1px solid rgba(110,170,255,0.28)',
            boxShadow: '0 16px 42px rgba(0,0,0,0.24)',
          }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', gap: '16px', alignItems: 'flex-start', flexWrap: 'wrap' }}>
              <div style={{ minWidth: 0 }}>
                <div style={{ color: '#d9e7ff', fontSize: '13px', fontWeight: 800, letterSpacing: '1.2px', textTransform: 'uppercase' }}>
                  Active Match
                </div>
                <div style={{ color: '#f4f7ff', fontSize: '24px', fontWeight: 900, marginTop: '6px' }}>
                  {boardStatusLabel}
                </div>
                <div style={{ color: 'rgba(220,230,255,0.72)', fontSize: '13px', lineHeight: 1.6, marginTop: '8px', maxWidth: '760px' }}>
                  Your live game stays separate from the play hub so you can queue again later, reopen the board instantly, or share the public match destination.
                </div>
              </div>

              <div style={{ display: 'flex', gap: '10px', flexWrap: 'wrap' }}>
                <button onClick={onReturnToMatch} style={primaryActionStyle}>
                  Return To Match
                </button>
                <button onClick={() => onCopyMatchLink?.(activeMatchId)} style={secondaryActionStyle}>
                  Share Match Link
                </button>
              </div>
            </div>

            <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(220px, 1fr))', gap: '12px', marginTop: '16px' }}>
              <MetaTile label="Lane" value={queueLabel(activeMatchQueue)} />
              <MetaTile label="Mode" value={modeLabel(activeMatchModeId)} />
              <MetaTile label="Role" value={viewerRoleLabel(viewerSeat)} />
            </div>

            {matchDestinationNotice ? (
              <div style={{
                marginTop: '14px',
                padding: '11px 13px',
                borderRadius: '12px',
                background: 'rgba(255,255,255,0.04)',
                border: '1px solid rgba(255,255,255,0.08)',
                color: 'rgba(244,232,200,0.84)',
                fontSize: '12px',
                lineHeight: 1.55,
              }}>
                {matchDestinationNotice}
              </div>
            ) : null}
          </div>
        ) : (
          <div style={{
            display: 'grid',
            gridTemplateColumns: 'repeat(auto-fit, minmax(260px, 1fr))',
            gap: '14px',
          }}>
            <LaunchTile
              eyebrow="Quick Pair"
              title="Enter the official queue lanes"
              detail="Pick the mode and choose casual or rated from one place. Once an opponent is assigned, the live board opens automatically."
            />
            <LaunchTile
              eyebrow="Play A Friend"
              title="Create one room and one clean invite"
              detail="Open a private waiting room, share the link, and let the second device claim the empty seat without extra setup."
            />
          </div>
        )}

        <div style={{ display: 'grid', gap: '18px' }}>
          <div style={{
            padding: '18px',
            borderRadius: '18px',
            background: 'linear-gradient(180deg, rgba(12,18,28,0.94) 0%, rgba(9,12,20,0.98) 100%)',
            border: '1px solid rgba(255,165,40,0.14)',
            boxShadow: '0 14px 38px rgba(0,0,0,0.22)',
          }}>
            <div style={{ marginBottom: '14px' }}>
              <div style={{ color: '#ffcf72', fontSize: '12px', fontWeight: 800, letterSpacing: '1.2px', textTransform: 'uppercase' }}>
                Quick Pair
              </div>
              <div style={{ color: '#fff4d6', fontSize: '18px', fontWeight: 800, marginTop: '6px' }}>
                Official modes and queue lane selection stay together
              </div>
              <div style={{ color: 'rgba(244,232,200,0.68)', fontSize: '13px', lineHeight: 1.6, marginTop: '6px' }}>
                Pick the official mode and the competitive lane from one surface, then wait here until a real opponent is assigned.
              </div>
            </div>

            <QueuePage
              embedded
              whiteProfile={whiteProfile}
              blackProfile={blackProfile}
              preferredQueue={preferredQueue}
              preferredModeId={preferredModeId}
              recoveredWhiteTicket={queueRecovery?.white ?? null}
              recoveredBlackTicket={queueRecovery?.black ?? null}
              recoveryReady={queueRecovery !== null}
            />
          </div>

          <div style={{
            padding: '18px',
            borderRadius: '18px',
            background: 'linear-gradient(180deg, rgba(12,18,28,0.94) 0%, rgba(9,12,20,0.98) 100%)',
            border: '1px solid rgba(110,170,255,0.14)',
            boxShadow: '0 14px 38px rgba(0,0,0,0.22)',
          }}>
            <div style={{ marginBottom: '14px' }}>
              <div style={{ color: '#9ed0ff', fontSize: '12px', fontWeight: 800, letterSpacing: '1.2px', textTransform: 'uppercase' }}>
                Play A Friend
              </div>
              <div style={{ color: '#f4f7ff', fontSize: '18px', fontWeight: 800, marginTop: '6px' }}>
                Create one real room and share one clean invite link
              </div>
              <div style={{ color: 'rgba(220,230,255,0.7)', fontSize: '13px', lineHeight: 1.6, marginTop: '6px' }}>
                The first browser owns one seat, the second device claims the other seat, and the waiting room becomes the match destination.
              </div>
            </div>

            <LobbiesPage
              embedded
              hostedRuntime={hostedRuntime}
              displayName={displayName}
              identity={identity}
            />
          </div>
        </div>
      </div>
    </div>
  );
}

function MetaTile(props: { label: string; value: string }): React.ReactElement {
  return (
    <div style={{
      padding: '12px 14px',
      borderRadius: '12px',
      background: 'rgba(255,255,255,0.03)',
      border: '1px solid rgba(255,255,255,0.07)',
    }}>
      <div style={{ color: 'rgba(244,232,200,0.56)', fontSize: '10px', fontWeight: 800, letterSpacing: '1px', textTransform: 'uppercase' }}>
        {props.label}
      </div>
      <div style={{ color: '#fff2c8', fontSize: '14px', fontWeight: 800, marginTop: '6px' }}>
        {props.value}
      </div>
    </div>
  );
}

function LaunchTile(props: { eyebrow: string; title: string; detail: string }): React.ReactElement {
  return (
    <div style={{
      padding: '18px',
      borderRadius: '18px',
      background: 'linear-gradient(180deg, rgba(12,18,28,0.94) 0%, rgba(9,12,20,0.98) 100%)',
      border: '1px solid rgba(255,165,40,0.14)',
      boxShadow: '0 14px 38px rgba(0,0,0,0.22)',
    }}>
      <div style={{ color: '#ffcf72', fontSize: '11px', fontWeight: 800, letterSpacing: '1.1px', textTransform: 'uppercase' }}>
        {props.eyebrow}
      </div>
      <div style={{ color: '#fff4d6', fontSize: '18px', fontWeight: 800, marginTop: '8px' }}>
        {props.title}
      </div>
      <div style={{ color: 'rgba(244,232,200,0.68)', fontSize: '13px', lineHeight: 1.6, marginTop: '8px' }}>
        {props.detail}
      </div>
    </div>
  );
}

const primaryActionStyle: React.CSSProperties = {
  padding: '11px 16px',
  borderRadius: '10px',
  border: '1px solid rgba(122,166,255,0.36)',
  background: 'linear-gradient(180deg, rgba(58,110,210,0.95) 0%, rgba(28,54,112,0.98) 100%)',
  color: '#f7fbff',
  fontWeight: 800,
  fontSize: '12px',
  cursor: 'pointer',
};

const secondaryActionStyle: React.CSSProperties = {
  ...primaryActionStyle,
  background: 'rgba(255,255,255,0.05)',
  border: '1px solid rgba(255,255,255,0.12)',
  color: 'rgba(244,247,255,0.88)',
};
