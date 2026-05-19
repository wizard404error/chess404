import React from 'react';
import { DEFAULT_MATCH_MODE_ID, OFFICIAL_MATCH_MODES, type MatchModeId, type PieceColor } from '@chess404/contracts';
import { acceptDirectChallenge, sendDirectChallenge, type DirectChallengeLaunchResponse } from './lib/direct-challenge-service';
import { modeLabel } from './lib/match-labels';
import {
  type AccountProfile,
  cancelDirectChallenge,
  declineDirectChallenge,
  fetchDirectChallengeOverview,
  fetchFriendOverview,
  removeFriend,
  respondToFriendRequest,
  sendFriendRequest,
  type DirectChallengeOverview,
  type DirectChallengeView,
  type FriendOverview,
  type FriendRequestView,
  type FriendshipView,
} from './lib/platform-service';
import { writeStoredRoomMeta } from './lib/match-service';
import type { PrivateMatchIdentity } from './lib/private-match-service';

interface FriendsPageProps {
  identity?: PrivateMatchIdentity | null;
  accountId?: string | null;
  sessionToken?: string | null;
  liveRefreshToken?: number;
  onOpenProfile?: (handle: string) => void;
  onOpenAccount?: () => void;
}

function formatDateTime(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return date.toLocaleString();
}


function describePresence(account: AccountProfile): {
  label: string;
  detail: string;
  border: string;
  background: string;
  color: string;
} {
  switch (account.presenceStatus) {
    case 'online':
      return {
        label: 'Online now',
        detail: 'Ready for live play',
        border: '1px solid rgba(86,204,120,0.28)',
        background: 'rgba(30,110,60,0.18)',
        color: '#d8ffe5',
      };
    case 'recently_active':
      return {
        label: 'Recently active',
        detail: account.lastActiveAt ? `Active ${formatDateTime(account.lastActiveAt)}` : 'Seen recently',
        border: '1px solid rgba(255,180,60,0.24)',
        background: 'rgba(255,180,60,0.08)',
        color: '#ffe7a9',
      };
    default:
      return {
        label: 'Offline',
        detail: `Last seen ${formatDateTime(account.lastSeenAt)}`,
        border: '1px solid rgba(255,255,255,0.12)',
        background: 'rgba(255,255,255,0.035)',
        color: 'rgba(255,232,180,0.72)',
      };
  }
}

function persistChallengeRoom(result: DirectChallengeLaunchResponse): void {
  writeStoredRoomMeta(result.match.matchId, {
    queue: 'direct',
    modeId: result.modeId ?? result.match.snapshot.match.modeId ?? DEFAULT_MATCH_MODE_ID,
    viewerSeat: result.match.seatColor,
    whiteGuestId: result.match.snapshot.match.whiteGuestId,
    blackGuestId: result.match.snapshot.match.blackGuestId,
    whiteAccountId: result.match.snapshot.match.whiteAccountId,
    blackAccountId: result.match.snapshot.match.blackAccountId,
    whiteName: result.match.snapshot.match.whiteName,
    blackName: result.match.snapshot.match.blackName,
    whitePlayerSecret: result.match.seatColor === 'white' ? result.match.claim?.playerSecret : undefined,
    blackPlayerSecret: result.match.seatColor === 'black' ? result.match.claim?.playerSecret : undefined,
    whiteClaimToken: result.match.seatColor === 'white' ? result.match.claim?.claimToken : undefined,
    blackClaimToken: result.match.seatColor === 'black' ? result.match.claim?.claimToken : undefined,
    whiteClaimExpiresAt: result.match.seatColor === 'white' ? result.match.claim?.expiresAt : undefined,
    blackClaimExpiresAt: result.match.seatColor === 'black' ? result.match.claim?.expiresAt : undefined,
  });
}

export default function FriendsPage({
  identity = null,
  accountId = null,
  sessionToken = null,
  liveRefreshToken = 0,
  onOpenProfile,
  onOpenAccount,
}: FriendsPageProps): React.ReactElement {
  const [overview, setOverview] = React.useState<FriendOverview | null>(null);
  const [challengeOverview, setChallengeOverview] = React.useState<DirectChallengeOverview | null>(null);
  const [targetHandle, setTargetHandle] = React.useState('');
  const [challengeModeId, setChallengeModeId] = React.useState<MatchModeId>(DEFAULT_MATCH_MODE_ID);
  const [challengeSeat, setChallengeSeat] = React.useState<PieceColor>('white');
  const [loading, setLoading] = React.useState(false);
  const [error, setError] = React.useState('');
  const [notice, setNotice] = React.useState('');
  const [busyRequestId, setBusyRequestId] = React.useState<string | null>(null);

  const loadOverview = React.useCallback(async () => {
    if (!accountId || !sessionToken) {
      setOverview(null);
      setChallengeOverview(null);
      return;
    }
    setLoading(true);
    setError('');
    try {
      const [nextFriends, nextChallenges] = await Promise.all([
        fetchFriendOverview({ accountId, sessionToken }),
        fetchDirectChallengeOverview({ accountId, sessionToken }),
      ]);
      setOverview(nextFriends);
      setChallengeOverview(nextChallenges);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load friends.');
    } finally {
      setLoading(false);
    }
  }, [accountId, sessionToken]);

  React.useEffect(() => {
    if (busyRequestId) {
      return;
    }
    void loadOverview();
  }, [busyRequestId, liveRefreshToken, loadOverview]);

  const mutateOverview = React.useCallback((next: FriendOverview, message?: string) => {
    setOverview(next);
    setError('');
    if (message) {
      setNotice(message);
    }
  }, []);

  const mutateChallenges = React.useCallback((next: DirectChallengeOverview, message?: string) => {
    setChallengeOverview(next);
    setError('');
    if (message) {
      setNotice(message);
    }
  }, []);

  const submitFriendRequest = React.useCallback(async () => {
    if (!accountId || !sessionToken) {
      setError('Sign in to send friend requests.');
      return;
    }
    const handle = targetHandle.trim().toLowerCase();
    if (!handle) {
      setError('Enter a handle to send a friend request.');
      return;
    }
    setBusyRequestId('send');
    setNotice('');
    setError('');
    try {
      const next = await sendFriendRequest({ accountId, sessionToken, targetHandle: handle });
      mutateOverview(next, `Friend request sent to @${handle}`);
      setTargetHandle('');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to send friend request.');
    } finally {
      setBusyRequestId(null);
    }
  }, [accountId, mutateOverview, sessionToken, targetHandle]);

  const handleRespond = React.useCallback(async (request: FriendRequestView, accept: boolean) => {
    if (!accountId || !sessionToken) {
      setError('Sign in to manage friend requests.');
      return;
    }
    setBusyRequestId(request.requestId);
    setNotice('');
    setError('');
    try {
      const next = await respondToFriendRequest({
        accountId,
        sessionToken,
        requestId: request.requestId,
        accept,
      });
      mutateOverview(next, accept ? `You are now friends with @${request.account.handle}` : `Declined @${request.account.handle}`);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to update friend request.');
    } finally {
      setBusyRequestId(null);
    }
  }, [accountId, mutateOverview, sessionToken]);

  const handleRemoveFriend = React.useCallback(async (friendship: FriendshipView) => {
    if (!accountId || !sessionToken) {
      setError('Sign in to manage friends.');
      return;
    }
    setBusyRequestId(friendship.friendshipId);
    setNotice('');
    setError('');
    try {
      const next = await removeFriend({
        accountId,
        sessionToken,
        friendAccountId: friendship.account.accountId,
      });
      mutateOverview(next, `Removed @${friendship.account.handle} from your friends list`);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to remove friend.');
    } finally {
      setBusyRequestId(null);
    }
  }, [accountId, mutateOverview, sessionToken]);

  const handleSendChallenge = React.useCallback(async (friendship: FriendshipView) => {
    if (!accountId || !sessionToken || !identity?.guestId) {
      setError('Sign in with an active player session to send direct challenges.');
      return;
    }
    setBusyRequestId(`challenge:${friendship.friendshipId}`);
    setNotice('');
    setError('');
    try {
      const result = await sendDirectChallenge({
        identity,
        targetAccountId: friendship.account.accountId,
        modeId: challengeModeId,
        preferredSeat: challengeSeat,
        clockSeconds: 600,
      });
      persistChallengeRoom(result);
      window.location.href = `/?match=${encodeURIComponent(result.match.matchId)}`;
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to send direct challenge.');
    } finally {
      setBusyRequestId(null);
    }
  }, [accountId, challengeModeId, challengeSeat, identity, sessionToken]);

  const handleAcceptChallenge = React.useCallback(async (challenge: DirectChallengeView) => {
    if (!accountId || !sessionToken || !identity?.guestId) {
      setError('Sign in with an active player session to accept direct challenges.');
      return;
    }
    setBusyRequestId(`accept:${challenge.challengeId}`);
    setNotice('');
    setError('');
    try {
      const result = await acceptDirectChallenge({
        challengeId: challenge.challengeId,
        identity,
      });
      persistChallengeRoom(result);
      window.location.href = `/?match=${encodeURIComponent(result.match.matchId)}`;
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to accept direct challenge.');
    } finally {
      setBusyRequestId(null);
    }
  }, [accountId, identity, sessionToken]);

  const handleDeclineChallenge = React.useCallback(async (challenge: DirectChallengeView) => {
    if (!accountId || !sessionToken) {
      setError('Sign in to manage direct challenges.');
      return;
    }
    setBusyRequestId(`decline:${challenge.challengeId}`);
    setNotice('');
    setError('');
    try {
      await declineDirectChallenge({
        accountId,
        sessionToken,
        challengeId: challenge.challengeId,
      });
      const next = await fetchDirectChallengeOverview({ accountId, sessionToken });
      mutateChallenges(next, `Declined @${challenge.account.handle}'s challenge.`);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to decline direct challenge.');
    } finally {
      setBusyRequestId(null);
    }
  }, [accountId, mutateChallenges, sessionToken]);

  const handleCancelChallenge = React.useCallback(async (challenge: DirectChallengeView) => {
    if (!accountId || !sessionToken) {
      setError('Sign in to manage direct challenges.');
      return;
    }
    setBusyRequestId(`cancel:${challenge.challengeId}`);
    setNotice('');
    setError('');
    try {
      await cancelDirectChallenge({
        accountId,
        sessionToken,
        challengeId: challenge.challengeId,
      });
      const next = await fetchDirectChallengeOverview({ accountId, sessionToken });
      mutateChallenges(next, `Cancelled your challenge to @${challenge.account.handle}.`);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to cancel direct challenge.');
    } finally {
      setBusyRequestId(null);
    }
  }, [accountId, mutateChallenges, sessionToken]);

  const renderProfileChip = React.useCallback((handle: string, label?: string) => (
    <button
      onClick={() => onOpenProfile?.(handle)}
      style={{
        padding: '7px 10px',
        borderRadius: '999px',
        border: '1px solid rgba(255,180,60,0.22)',
        background: 'rgba(255,180,60,0.08)',
        color: '#ffe7a9',
        fontSize: '11px',
        fontWeight: 800,
        cursor: 'pointer',
      }}
    >
      {label ?? `@${handle}`}
    </button>
  ), [onOpenProfile]);

  if (!accountId || !sessionToken) {
    return (
      <div style={{ display: 'flex', flex: 1, minHeight: 0, alignItems: 'center', justifyContent: 'center', padding: '32px' }}>
        <div style={{
          width: 'min(560px, 100%)',
          padding: '26px',
          borderRadius: '18px',
          border: '1px solid rgba(255,180,60,0.18)',
          background: 'linear-gradient(180deg, rgba(15,18,28,0.98) 0%, rgba(10,12,20,0.96) 100%)',
          boxShadow: '0 20px 60px rgba(0,0,0,0.35)',
          color: '#fff2c8',
        }}>
          <div style={{ fontSize: '13px', fontWeight: 800, letterSpacing: '1.2px', textTransform: 'uppercase', color: '#ffcf72' }}>Friends</div>
          <div style={{ marginTop: '10px', fontSize: '15px', lineHeight: 1.7, color: 'rgba(255,236,194,0.82)' }}>
            Friends are tied to real account sessions now. Sign into a claimed account to send requests, accept friends, and unlock future direct challenges and lobby invites.
          </div>
          <div style={{ display: 'flex', gap: '10px', marginTop: '18px', flexWrap: 'wrap' }}>
            <button
              onClick={onOpenAccount}
              style={{
                padding: '11px 16px',
                borderRadius: '10px',
                border: '1px solid rgba(255,180,60,0.36)',
                background: 'linear-gradient(180deg, rgba(200,134,10,0.36) 0%, rgba(122,79,8,0.44) 100%)',
                color: '#fff4d3',
                fontSize: '12px',
                fontWeight: 800,
                cursor: 'pointer',
              }}
            >
              Open Account
            </button>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div style={{ display: 'flex', flex: 1, minHeight: 0, padding: '22px 28px 26px', gap: '18px' }}>
      <div style={{
        width: '390px',
        flexShrink: 0,
        minWidth: 0,
        minHeight: 0,
        display: 'flex',
        flexDirection: 'column',
        background: 'linear-gradient(180deg, rgba(14,18,30,0.98) 0%, rgba(9,12,20,0.96) 100%)',
        border: '1px solid rgba(255,165,40,0.16)',
        borderRadius: '14px',
        boxShadow: '0 12px 40px rgba(0,0,0,0.35)',
        overflow: 'hidden',
      }}>
        <div style={{ padding: '18px 20px 14px', borderBottom: '1px solid rgba(255,165,40,0.12)' }}>
          <div style={{ color: '#ffcf72', fontSize: '13px', fontWeight: 800, letterSpacing: '1.2px', textTransform: 'uppercase' }}>Friends Network</div>
          <div style={{ color: 'rgba(255,232,180,0.72)', fontSize: '12px', marginTop: '4px', lineHeight: 1.5 }}>
            Send friend requests by handle, manage direct challenges, and keep a persistent social graph tied to your Chess404 account.
          </div>
          {overview && (
            <div style={{ display: 'flex', gap: '8px', alignItems: 'center', marginTop: '14px', flexWrap: 'wrap' }}>
              {renderProfileChip(overview.viewer.handle, `Signed in as @${overview.viewer.handle}`)}
              <div style={{ fontSize: '11px', color: 'rgba(255,232,180,0.64)' }}>
                {overview.friends.length} friends
              </div>
            </div>
          )}

          <div style={{ display: 'grid', gap: '10px', marginTop: '14px' }}>
            <div style={{ display: 'flex', gap: '8px' }}>
              <input
                value={targetHandle}
                onChange={(event) => setTargetHandle(event.target.value)}
                onKeyDown={(event) => {
                  if (event.key === 'Enter') {
                    void submitFriendRequest();
                  }
                }}
                placeholder="Send request to handle"
                style={{
                  flex: 1,
                  padding: '10px 12px',
                  borderRadius: '10px',
                  border: '1px solid rgba(255,180,60,0.24)',
                  background: 'rgba(255,255,255,0.04)',
                  color: '#fff2c8',
                  fontSize: '12px',
                  fontWeight: 700,
                  outline: 'none',
                }}
              />
              <button
                onClick={() => void submitFriendRequest()}
                disabled={busyRequestId === 'send'}
                style={{
                  padding: '10px 12px',
                  borderRadius: '10px',
                  border: '1px solid rgba(255,180,60,0.3)',
                  background: 'linear-gradient(180deg, rgba(200,134,10,0.28) 0%, rgba(122,79,8,0.38) 100%)',
                  color: '#fff2c8',
                  fontSize: '12px',
                  fontWeight: 800,
                  cursor: 'pointer',
                  opacity: busyRequestId === 'send' ? 0.7 : 1,
                }}
              >
                Send
              </button>
            </div>
            <button
              onClick={() => void loadOverview()}
              style={{
                justifySelf: 'start',
                padding: '8px 12px',
                borderRadius: '8px',
                border: '1px solid rgba(255,180,60,0.2)',
                background: 'rgba(255,255,255,0.04)',
                color: '#fff2c8',
                fontSize: '11px',
                fontWeight: 700,
                cursor: 'pointer',
              }}
            >
              Refresh overview
            </button>

            <div style={{ marginTop: '4px', padding: '12px 12px 10px', borderRadius: '12px', border: '1px solid rgba(255,180,60,0.12)', background: 'rgba(255,255,255,0.025)', display: 'grid', gap: '10px' }}>
              <div style={{ color: '#fff2c8', fontSize: '11px', fontWeight: 800, letterSpacing: '0.8px', textTransform: 'uppercase' }}>Challenge Defaults</div>
              <label style={{ display: 'grid', gap: '6px' }}>
                <span style={{ color: 'rgba(255,232,180,0.62)', fontSize: '11px', fontWeight: 700 }}>Mode</span>
                <select
                  value={challengeModeId}
                  onChange={(event) => setChallengeModeId(event.target.value as MatchModeId)}
                  style={{
                    padding: '10px 12px',
                    borderRadius: '10px',
                    border: '1px solid rgba(255,180,60,0.22)',
                    background: 'rgba(255,255,255,0.04)',
                    color: '#fff2c8',
                    fontSize: '12px',
                    fontWeight: 700,
                    outline: 'none',
                  }}
                >
                  {OFFICIAL_MATCH_MODES.map((mode) => (
                    <option key={mode.id} value={mode.id}>{mode.label}</option>
                  ))}
                </select>
              </label>
              <div style={{ display: 'grid', gap: '6px' }}>
                <span style={{ color: 'rgba(255,232,180,0.62)', fontSize: '11px', fontWeight: 700 }}>Your preferred seat</span>
                <div style={{ display: 'flex', gap: '8px', flexWrap: 'wrap' }}>
                  {(['white', 'black'] as PieceColor[]).map((color) => (
                    <button
                      key={color}
                      onClick={() => setChallengeSeat(color)}
                      style={{
                        padding: '8px 10px',
                        borderRadius: '999px',
                        border: challengeSeat === color ? '1px solid rgba(255,215,0,0.34)' : '1px solid rgba(255,180,60,0.16)',
                        background: challengeSeat === color ? 'rgba(255,180,60,0.16)' : 'rgba(255,255,255,0.03)',
                        color: challengeSeat === color ? '#fff2c8' : 'rgba(255,232,180,0.72)',
                        fontSize: '11px',
                        fontWeight: 800,
                        cursor: 'pointer',
                        textTransform: 'capitalize',
                      }}
                    >
                      {color}
                    </button>
                  ))}
                </div>
              </div>
            </div>
          </div>
        </div>

        <div style={{ flex: 1, minHeight: 0, overflowY: 'auto', padding: '18px 20px 22px', display: 'grid', gap: '14px' }}>
          {error && (
            <div style={{ padding: '12px 14px', borderRadius: '10px', background: 'rgba(120,20,20,0.22)', border: '1px solid rgba(231,76,60,0.32)', color: '#ffb1a7', fontSize: '12px', fontWeight: 700 }}>
              {error}
            </div>
          )}
          {notice && (
            <div style={{ padding: '12px 14px', borderRadius: '10px', background: 'rgba(20,90,50,0.22)', border: '1px solid rgba(80,190,120,0.28)', color: '#d7ffd7', fontSize: '12px', fontWeight: 700 }}>
              {notice}
            </div>
          )}

          <FriendSection title="Incoming Requests" emptyLabel="No incoming requests right now.">
            {overview?.incoming.map((request) => (
              <FriendRequestCard
                key={request.requestId}
                request={request}
                busy={busyRequestId === request.requestId}
                onOpenProfile={onOpenProfile}
                onAccept={() => void handleRespond(request, true)}
                onDecline={() => void handleRespond(request, false)}
              />
            ))}
          </FriendSection>

          <FriendSection title="Outgoing Requests" emptyLabel="No pending outgoing requests.">
            {overview?.outgoing.map((request) => (
              <FriendRequestCard
                key={request.requestId}
                request={request}
                busy={busyRequestId === request.requestId}
                onOpenProfile={onOpenProfile}
                readOnly
              />
            ))}
          </FriendSection>

          <FriendSection title="Incoming Challenges" emptyLabel="No incoming direct challenges.">
            {challengeOverview?.incoming.map((challenge) => (
              <DirectChallengeCard
                key={challenge.challengeId}
                challenge={challenge}
                busy={busyRequestId === `accept:${challenge.challengeId}` || busyRequestId === `decline:${challenge.challengeId}`}
                onOpenProfile={onOpenProfile}
                onAccept={() => void handleAcceptChallenge(challenge)}
                onDecline={() => void handleDeclineChallenge(challenge)}
              />
            ))}
          </FriendSection>

          <FriendSection title="Outgoing Challenges" emptyLabel="No pending direct challenges sent to friends.">
            {challengeOverview?.outgoing.map((challenge) => (
              <DirectChallengeCard
                key={challenge.challengeId}
                challenge={challenge}
                busy={busyRequestId === `cancel:${challenge.challengeId}`}
                onOpenProfile={onOpenProfile}
                onCancel={() => void handleCancelChallenge(challenge)}
                readOnly
              />
            ))}
          </FriendSection>
        </div>
      </div>

      <div style={{
        flex: 1,
        minWidth: 0,
        minHeight: 0,
        display: 'flex',
        flexDirection: 'column',
        background: 'linear-gradient(180deg, rgba(15,18,28,0.98) 0%, rgba(10,12,20,0.96) 100%)',
        border: '1px solid rgba(255,180,60,0.14)',
        borderRadius: '16px',
        boxShadow: '0 18px 52px rgba(0,0,0,0.35)',
        overflow: 'hidden',
      }}>
        <div style={{ padding: '20px 24px 14px', borderBottom: '1px solid rgba(255,180,60,0.12)' }}>
          <div style={{ color: '#ffcf72', fontSize: '13px', fontWeight: 800, letterSpacing: '1.2px', textTransform: 'uppercase' }}>Accepted Friends</div>
          <div style={{ color: 'rgba(255,232,180,0.72)', fontSize: '12px', marginTop: '4px', lineHeight: 1.5 }}>
            Friends are now the launch foundation for direct account-to-account play. Challenge a friend into a private room without falling back to manual copy-paste lobby links.
          </div>
        </div>

        <div style={{ flex: 1, minHeight: 0, overflowY: 'auto', padding: '22px 24px 26px' }}>
          {loading && !overview ? (
            <div style={{ color: 'rgba(255,232,180,0.72)', fontSize: '13px' }}>Loading friends...</div>
          ) : overview?.friends.length ? (
            <div style={{ display: 'grid', gap: '14px', gridTemplateColumns: 'repeat(auto-fit, minmax(240px, 1fr))' }}>
              {overview.friends.map((friendship) => (
                <div
                  key={friendship.friendshipId}
                  style={{
                    padding: '16px',
                    borderRadius: '14px',
                    border: '1px solid rgba(255,180,60,0.14)',
                    background: 'rgba(255,255,255,0.03)',
                    display: 'grid',
                    gap: '10px',
                  }}
                >
                  <div style={{ display: 'flex', justifyContent: 'space-between', gap: '10px', alignItems: 'center' }}>
                    <div>
                      <div style={{ color: '#fff2c8', fontSize: '15px', fontWeight: 800 }}>@{friendship.account.handle}</div>
                      <div style={{ color: 'rgba(255,232,180,0.64)', fontSize: '12px', marginTop: '3px' }}>
                        Rating {friendship.account.rating ?? 1200} | {describePresence(friendship.account).detail}
                      </div>
                    </div>
                    <button
                      onClick={() => void handleRemoveFriend(friendship)}
                      disabled={busyRequestId === friendship.friendshipId}
                      style={{
                        padding: '8px 10px',
                        borderRadius: '9px',
                        border: '1px solid rgba(231,76,60,0.32)',
                        background: 'rgba(120,20,20,0.18)',
                        color: '#ffd3ce',
                        fontSize: '11px',
                        fontWeight: 800,
                        cursor: 'pointer',
                        opacity: busyRequestId === friendship.friendshipId ? 0.7 : 1,
                      }}
                    >
                      Remove
                    </button>
                  </div>

                  <div style={{ color: 'rgba(255,232,180,0.78)', fontSize: '12px', lineHeight: 1.6 }}>
                    Friends since {formatDateTime(friendship.createdAt)}
                  </div>
                  <PresencePill account={friendship.account} />

                  <div style={{ display: 'flex', gap: '8px', flexWrap: 'wrap' }}>
                    {renderProfileChip(friendship.account.handle)}
                    <button
                      onClick={() => void handleSendChallenge(friendship)}
                      disabled={busyRequestId === `challenge:${friendship.friendshipId}`}
                      style={{
                        padding: '7px 10px',
                        borderRadius: '8px',
                        border: '1px solid rgba(86,204,120,0.3)',
                        background: 'rgba(30,110,60,0.2)',
                        color: '#d8ffe5',
                        fontSize: '11px',
                        fontWeight: 800,
                        cursor: 'pointer',
                        opacity: busyRequestId === `challenge:${friendship.friendshipId}` ? 0.7 : 1,
                      }}
                    >
                      Challenge
                    </button>
                  </div>
                </div>
              ))}
            </div>
          ) : (
            <div style={{
              padding: '18px 20px',
              borderRadius: '14px',
              border: '1px dashed rgba(255,180,60,0.18)',
              background: 'rgba(255,255,255,0.02)',
              color: 'rgba(255,232,180,0.72)',
              fontSize: '13px',
              lineHeight: 1.7,
            }}>
              No accepted friends yet. Start by sending a request to another claimed Chess404 handle.
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

function FriendSection({
  title,
  emptyLabel,
  children,
}: React.PropsWithChildren<{ title: string; emptyLabel: string }>): React.ReactElement {
  const items = React.Children.toArray(children);
  return (
    <section style={{ display: 'grid', gap: '10px' }}>
      <div style={{ color: '#fff2c8', fontSize: '12px', fontWeight: 800, letterSpacing: '0.8px', textTransform: 'uppercase' }}>{title}</div>
      {items.length ? items : (
        <div style={{ padding: '12px 14px', borderRadius: '10px', background: 'rgba(255,255,255,0.03)', border: '1px dashed rgba(255,180,60,0.12)', color: 'rgba(255,232,180,0.66)', fontSize: '12px', lineHeight: 1.6 }}>
          {emptyLabel}
        </div>
      )}
    </section>
  );
}

function FriendRequestCard({
  request,
  busy,
  onOpenProfile,
  onAccept,
  onDecline,
  readOnly = false,
}: {
  request: FriendRequestView;
  busy: boolean;
  onOpenProfile?: (handle: string) => void;
  onAccept?: () => void;
  onDecline?: () => void;
  readOnly?: boolean;
}): React.ReactElement {
  return (
    <div style={{ padding: '14px', borderRadius: '12px', border: '1px solid rgba(255,180,60,0.12)', background: 'rgba(255,255,255,0.03)', display: 'grid', gap: '9px' }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', gap: '10px', alignItems: 'center' }}>
        <div>
          <div style={{ color: '#fff2c8', fontSize: '14px', fontWeight: 800 }}>@{request.account.handle}</div>
          <div style={{ color: 'rgba(255,232,180,0.64)', fontSize: '11px', marginTop: '3px' }}>
            {request.account.rating ?? 1200} rating | {describePresence(request.account).detail}
          </div>
        </div>
        <button
          onClick={() => onOpenProfile?.(request.account.handle)}
          style={{
            padding: '7px 10px',
            borderRadius: '8px',
            border: '1px solid rgba(255,180,60,0.18)',
            background: 'rgba(255,180,60,0.06)',
            color: '#ffe7a9',
            fontSize: '11px',
            fontWeight: 800,
            cursor: 'pointer',
          }}
        >
          Profile
        </button>
      </div>
      <PresencePill account={request.account} />
      {!readOnly ? (
        <div style={{ display: 'flex', gap: '8px', flexWrap: 'wrap' }}>
          <button
            onClick={onAccept}
            disabled={busy}
            style={{
              padding: '8px 10px',
              borderRadius: '9px',
              border: '1px solid rgba(86,204,120,0.32)',
              background: 'rgba(30,110,60,0.2)',
              color: '#d8ffe5',
              fontSize: '11px',
              fontWeight: 800,
              cursor: 'pointer',
              opacity: busy ? 0.7 : 1,
            }}
          >
            Accept
          </button>
          <button
            onClick={onDecline}
            disabled={busy}
            style={{
              padding: '8px 10px',
              borderRadius: '9px',
              border: '1px solid rgba(231,76,60,0.26)',
              background: 'rgba(120,20,20,0.18)',
              color: '#ffd3ce',
              fontSize: '11px',
              fontWeight: 800,
              cursor: 'pointer',
              opacity: busy ? 0.7 : 1,
            }}
          >
            Decline
          </button>
        </div>
      ) : (
        <div style={{ color: 'rgba(255,232,180,0.62)', fontSize: '11px' }}>
          Pending since {formatDateTime(request.createdAt)}
        </div>
      )}
    </div>
  );
}

function DirectChallengeCard({
  challenge,
  busy,
  onOpenProfile,
  onAccept,
  onDecline,
  onCancel,
  readOnly = false,
}: {
  challenge: DirectChallengeView;
  busy: boolean;
  onOpenProfile?: (handle: string) => void;
  onAccept?: () => void;
  onDecline?: () => void;
  onCancel?: () => void;
  readOnly?: boolean;
}): React.ReactElement {
  const seatText = challenge.viewerSeat
    ? `You play ${challenge.viewerSeat}`
    : challenge.challengerSeat
      ? `Challenger prefers ${challenge.challengerSeat}`
      : 'Seat assigned on join';

  return (
    <div style={{ padding: '14px', borderRadius: '12px', border: '1px solid rgba(255,180,60,0.12)', background: 'rgba(255,255,255,0.03)', display: 'grid', gap: '9px' }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', gap: '10px', alignItems: 'center' }}>
        <div>
          <div style={{ color: '#fff2c8', fontSize: '14px', fontWeight: 800 }}>@{challenge.account.handle}</div>
          <div style={{ color: 'rgba(255,232,180,0.64)', fontSize: '11px', marginTop: '3px', lineHeight: 1.5 }}>
            {modeLabel(challenge.modeId)} | {seatText} | {describePresence(challenge.account).detail}
          </div>
        </div>
        <button
          onClick={() => onOpenProfile?.(challenge.account.handle)}
          style={{
            padding: '7px 10px',
            borderRadius: '8px',
            border: '1px solid rgba(255,180,60,0.18)',
            background: 'rgba(255,180,60,0.06)',
            color: '#ffe7a9',
            fontSize: '11px',
            fontWeight: 800,
            cursor: 'pointer',
          }}
        >
          Profile
        </button>
      </div>
      <PresencePill account={challenge.account} />
      <div style={{ color: 'rgba(255,232,180,0.72)', fontSize: '11px', lineHeight: 1.6 }}>
        Match room {challenge.matchId} | Created {formatDateTime(challenge.createdAt)}
      </div>
      {!readOnly ? (
        <div style={{ display: 'flex', gap: '8px', flexWrap: 'wrap' }}>
          <button
            onClick={onAccept}
            disabled={busy}
            style={{
              padding: '8px 10px',
              borderRadius: '9px',
              border: '1px solid rgba(86,204,120,0.32)',
              background: 'rgba(30,110,60,0.2)',
              color: '#d8ffe5',
              fontSize: '11px',
              fontWeight: 800,
              cursor: 'pointer',
              opacity: busy ? 0.7 : 1,
            }}
          >
            Accept Challenge
          </button>
          <button
            onClick={onDecline}
            disabled={busy}
            style={{
              padding: '8px 10px',
              borderRadius: '9px',
              border: '1px solid rgba(231,76,60,0.26)',
              background: 'rgba(120,20,20,0.18)',
              color: '#ffd3ce',
              fontSize: '11px',
              fontWeight: 800,
              cursor: 'pointer',
              opacity: busy ? 0.7 : 1,
            }}
          >
            Decline
          </button>
        </div>
      ) : (
        <div style={{ display: 'flex', gap: '8px', flexWrap: 'wrap' }}>
          <div style={{ color: 'rgba(255,232,180,0.62)', fontSize: '11px', alignSelf: 'center' }}>
            Waiting for your friend to accept.
          </div>
          <button
            onClick={onCancel}
            disabled={busy}
            style={{
              padding: '8px 10px',
              borderRadius: '9px',
              border: '1px solid rgba(231,76,60,0.26)',
              background: 'rgba(120,20,20,0.18)',
              color: '#ffd3ce',
              fontSize: '11px',
              fontWeight: 800,
              cursor: 'pointer',
              opacity: busy ? 0.7 : 1,
            }}
          >
            Cancel Challenge
          </button>
        </div>
      )}
    </div>
  );
}

function PresencePill({ account }: { account: AccountProfile }): React.ReactElement {
  const presence = describePresence(account);
  return (
    <div
      style={{
        display: 'inline-flex',
        alignItems: 'center',
        gap: '6px',
        padding: '6px 9px',
        borderRadius: '999px',
        border: presence.border,
        background: presence.background,
        color: presence.color,
        fontSize: '10px',
        fontWeight: 800,
        letterSpacing: '0.5px',
        textTransform: 'uppercase',
      }}
    >
      <span style={{
        width: '7px',
        height: '7px',
        borderRadius: '999px',
        background: account.online ? '#7cff9c' : account.recentlyActive ? '#ffd36f' : 'rgba(255,255,255,0.35)',
        boxShadow: account.online ? '0 0 12px rgba(124,255,156,0.65)' : 'none',
      }} />
      {presence.label}
    </div>
  );
}
