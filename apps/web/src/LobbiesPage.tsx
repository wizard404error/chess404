import React from 'react';
import { DEFAULT_MATCH_MODE_ID, OFFICIAL_MATCH_MODES, type MatchModeId, type PieceColor } from '@chess404/contracts';
import { createPrivateMatch, type PrivateMatchIdentity } from './lib/private-match-service';
import { writeStoredRoomMeta } from './lib/match-service';

interface LobbiesPageProps {
  identity: PrivateMatchIdentity | null;
  displayName?: string | null;
  hostedRuntime: boolean;
}

export default function LobbiesPage({ identity, displayName, hostedRuntime }: LobbiesPageProps): React.ReactElement {
  const [modeId, setModeId] = React.useState<MatchModeId>(DEFAULT_MATCH_MODE_ID);
  const [preferredSeat, setPreferredSeat] = React.useState<PieceColor>('white');
  const [creating, setCreating] = React.useState(false);
  const [error, setError] = React.useState('');
  const [created, setCreated] = React.useState<{ matchId: string; inviteUrl: string; waitingForOpponent: boolean } | null>(null);

  const handleCreate = React.useCallback(async () => {
    if (!identity?.guestId) {
      setError('Your hosted player session is still loading.');
      return;
    }
    setCreating(true);
    setError('');
    try {
      const result = await createPrivateMatch({
        identity,
        modeId,
        preferredSeat,
        clockSeconds: 600,
      });
      const inviteUrl = `${window.location.origin}/?match=${encodeURIComponent(result.matchId)}`;
      writeStoredRoomMeta(result.matchId, {
        queue: 'direct',
        modeId,
        clockSeconds: 600,
        viewerSeat: result.seatColor,
        whiteGuestId: result.snapshot.match.whiteGuestId,
        blackGuestId: result.snapshot.match.blackGuestId,
        whiteAccountId: result.snapshot.match.whiteAccountId,
        blackAccountId: result.snapshot.match.blackAccountId,
        whiteName: result.snapshot.match.whiteName,
        blackName: result.snapshot.match.blackName,
        whitePlayerSecret: result.seatColor === 'white' ? result.claim?.playerSecret : undefined,
        blackPlayerSecret: result.seatColor === 'black' ? result.claim?.playerSecret : undefined,
        whiteClaimToken: result.seatColor === 'white' ? result.claim?.claimToken : undefined,
        blackClaimToken: result.seatColor === 'black' ? result.claim?.claimToken : undefined,
        whiteClaimExpiresAt: result.seatColor === 'white' ? result.claim?.expiresAt : undefined,
        blackClaimExpiresAt: result.seatColor === 'black' ? result.claim?.expiresAt : undefined,
      });
      setCreated({
        matchId: result.matchId,
        inviteUrl,
        waitingForOpponent: result.waitingForOpponent,
      });
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create private room.');
    } finally {
      setCreating(false);
    }
  }, [identity, modeId, preferredSeat]);

  const copyInviteLink = React.useCallback(async () => {
    if (!created?.inviteUrl) return;
    await navigator.clipboard.writeText(created.inviteUrl);
  }, [created?.inviteUrl]);

  const openRoom = React.useCallback(() => {
    if (!created?.matchId) return;
    window.location.href = `/?match=${encodeURIComponent(created.matchId)}`;
  }, [created?.matchId]);

  return (
    <div style={{ maxWidth: '980px', margin: '0 auto', display: 'grid', gap: '18px' }}>
      <div style={{ padding: '22px', borderRadius: '18px', background: 'linear-gradient(180deg, rgba(14,20,38,0.92) 0%, rgba(7,11,22,0.98) 100%)', border: '1px solid rgba(120,150,255,0.18)', boxShadow: '0 18px 48px rgba(0,0,0,0.28)' }}>
        <div style={{ color: '#f3f6ff', fontSize: '28px', fontWeight: 900 }}>Private Lobbies</div>
        <div style={{ marginTop: '8px', color: 'rgba(214,224,255,0.76)', fontSize: '14px', lineHeight: 1.65 }}>
          Create a private invite room, share the link, and let the second device auto-join the empty seat like a real quick-pair platform should.
        </div>
      </div>

      <div style={{ display: 'grid', gridTemplateColumns: 'minmax(0, 1.15fr) minmax(320px, 0.85fr)', gap: '18px' }}>
        <div style={{ padding: '20px', borderRadius: '18px', background: 'rgba(10,14,28,0.92)', border: '1px solid rgba(120,150,255,0.14)' }}>
          <div style={{ color: '#ffffff', fontSize: '16px', fontWeight: 800 }}>Create Invite Match</div>
          <div style={{ marginTop: '6px', color: 'rgba(214,224,255,0.7)', fontSize: '12px' }}>
            Current player: {displayName ?? identity?.guestId ?? 'Loading...'}
          </div>

          <div style={{ marginTop: '18px', display: 'grid', gap: '14px' }}>
            <label style={{ display: 'grid', gap: '8px' }}>
              <span style={{ color: '#dbe8ff', fontSize: '12px', fontWeight: 700, textTransform: 'uppercase', letterSpacing: '0.8px' }}>Mode</span>
              <select value={modeId} onChange={event => setModeId(event.target.value as MatchModeId)} style={selectStyle}>
                {OFFICIAL_MATCH_MODES.map(mode => (
                  <option key={mode.id} value={mode.id}>{mode.label}</option>
                ))}
              </select>
            </label>

            <label style={{ display: 'grid', gap: '8px' }}>
              <span style={{ color: '#dbe8ff', fontSize: '12px', fontWeight: 700, textTransform: 'uppercase', letterSpacing: '0.8px' }}>Your Seat</span>
              <div style={{ display: 'flex', gap: '10px', flexWrap: 'wrap' }}>
                {(['white', 'black'] as PieceColor[]).map(color => (
                  <button
                    key={color}
                    onClick={() => setPreferredSeat(color)}
                    style={{
                      padding: '10px 16px',
                      borderRadius: '999px',
                      border: preferredSeat === color ? '1px solid rgba(110,200,255,0.55)' : '1px solid rgba(255,255,255,0.10)',
                      background: preferredSeat === color ? 'rgba(54,116,255,0.22)' : 'rgba(255,255,255,0.04)',
                      color: preferredSeat === color ? '#dbe8ff' : 'rgba(214,224,255,0.68)',
                      cursor: 'pointer',
                      fontWeight: 800,
                      fontSize: '12px',
                      textTransform: 'capitalize',
                    }}
                  >
                    {color}
                  </button>
                ))}
              </div>
            </label>
          </div>

          {error && (
            <div style={{ marginTop: '16px', padding: '10px 12px', borderRadius: '10px', background: 'rgba(164,30,30,0.16)', border: '1px solid rgba(220,80,80,0.34)', color: '#ffb3b3', fontSize: '12px' }}>
              {error}
            </div>
          )}

          <button
            onClick={() => { void handleCreate(); }}
            disabled={creating || !identity?.guestId}
            style={{
              marginTop: '18px',
              width: '100%',
              padding: '13px 16px',
              borderRadius: '12px',
              border: '1px solid rgba(122,166,255,0.34)',
              background: creating ? 'rgba(100,100,120,0.2)' : 'linear-gradient(135deg, rgba(52,110,255,0.92), rgba(68,160,255,0.92))',
              color: '#f7fbff',
              fontSize: '13px',
              fontWeight: 900,
              cursor: creating || !identity?.guestId ? 'not-allowed' : 'pointer',
              boxShadow: creating ? 'none' : '0 16px 36px rgba(30,100,255,0.24)',
            }}
          >
            {creating ? 'Creating lobby...' : 'Create Private Invite Match'}
          </button>
        </div>

        <div style={{ padding: '20px', borderRadius: '18px', background: 'rgba(10,14,28,0.92)', border: '1px solid rgba(120,150,255,0.14)' }}>
          <div style={{ color: '#ffffff', fontSize: '16px', fontWeight: 800 }}>Invite Flow</div>
          <div style={{ marginTop: '8px', color: 'rgba(214,224,255,0.7)', fontSize: '13px', lineHeight: 1.6 }}>
            Share the room link with a friend. The first browser already owns one seat. The second device opens the link and automatically takes the empty seat.
          </div>

          {created ? (
            <div style={{ marginTop: '18px', display: 'grid', gap: '12px' }}>
              <div style={{ padding: '12px 14px', borderRadius: '12px', background: 'rgba(75,150,255,0.10)', border: '1px solid rgba(95,165,255,0.24)' }}>
                <div style={{ color: '#9ed0ff', fontSize: '11px', fontWeight: 800, textTransform: 'uppercase', letterSpacing: '0.8px' }}>Invite Link</div>
                <div style={{ marginTop: '8px', color: '#eef4ff', fontSize: '12px', lineHeight: 1.5, wordBreak: 'break-all' }}>{created.inviteUrl}</div>
              </div>
              <div style={{ display: 'flex', gap: '10px', flexWrap: 'wrap' }}>
                <button onClick={() => { void copyInviteLink(); }} style={secondaryButtonStyle}>Copy Invite Link</button>
                <button onClick={openRoom} style={primaryButtonStyle}>Open Waiting Room</button>
              </div>
              <div style={{ color: created.waitingForOpponent ? '#9de4b0' : '#ffd487', fontSize: '12px', lineHeight: 1.6 }}>
                {created.waitingForOpponent
                  ? 'Room is live and waiting for the second seat to join.'
                  : 'Both seats are already claimed.'}
              </div>
            </div>
          ) : (
            <div style={{ marginTop: '18px', padding: '14px 16px', borderRadius: '12px', background: 'rgba(255,255,255,0.04)', border: '1px dashed rgba(255,255,255,0.12)', color: 'rgba(214,224,255,0.62)', fontSize: '12px', lineHeight: 1.6 }}>
              Create a lobby first, then copy the invite link to your phone or another browser.
              {!hostedRuntime && <div style={{ marginTop: '8px', color: '#ffd487' }}>Localhost still keeps its dev behavior, but hosted private lobbies are now the real platform path.</div>}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

const selectStyle: React.CSSProperties = {
  padding: '12px 14px',
  borderRadius: '12px',
  border: '1px solid rgba(255,255,255,0.12)',
  background: 'rgba(255,255,255,0.05)',
  color: '#eef4ff',
  fontSize: '13px',
  outline: 'none',
};

const primaryButtonStyle: React.CSSProperties = {
  padding: '11px 14px',
  borderRadius: '10px',
  border: '1px solid rgba(122,166,255,0.34)',
  background: 'linear-gradient(135deg, rgba(52,110,255,0.92), rgba(68,160,255,0.92))',
  color: '#f7fbff',
  fontWeight: 900,
  fontSize: '12px',
  cursor: 'pointer',
};

const secondaryButtonStyle: React.CSSProperties = {
  ...primaryButtonStyle,
  background: 'rgba(255,255,255,0.06)',
  border: '1px solid rgba(255,255,255,0.14)',
};
