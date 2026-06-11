'use client';

import React from 'react';
import {
  fetchAccountAuthOverview,
  formatAccountRestrictionNotice,
  isAccountRestrictionError,
  loginAccountWithPassword,
  logoutAccountSession,
  registerAccountWithPassword,
  requestPasswordReset,
  resumeAccountSession,
  type AccountAuthOverview,
  type AccountSession,
  type GuestProfile,
  type GuestSession,
} from './lib/platform-service';
import { formatDateTime } from './lib/display';

const WHITE_GUEST_ID_STORAGE_KEY = 'chess404.guest.white';
const WHITE_GUEST_SECRET_STORAGE_KEY = 'chess404.guest.white.secret';
const WHITE_GUEST_TOKEN_STORAGE_KEY = 'chess404.guest.white.token';
const WHITE_GUEST_TOKEN_EXPIRY_STORAGE_KEY = 'chess404.guest.white.token.expiresAt';
const WHITE_ACCOUNT_ID_STORAGE_KEY = 'chess404.account.white.id';
const WHITE_ACCOUNT_TOKEN_STORAGE_KEY = 'chess404.account.white.token';
const WHITE_ACCOUNT_EXPIRY_STORAGE_KEY = 'chess404.account.white.expiresAt';

type StoredGuestIdentity = {
  guestId?: string;
  sessionSecret?: string;
  sessionToken?: string;
  sessionExpiresAt?: string;
};

type StoredAccountIdentity = {
  accountId?: string;
  sessionToken?: string;
  expiresAt?: string;
};

function readStoredGuestIdentity(): StoredGuestIdentity {
  if (typeof window === 'undefined') {
    return {};
  }
  return {
    guestId: window.localStorage.getItem(WHITE_GUEST_ID_STORAGE_KEY) ?? undefined,
    sessionSecret: window.localStorage.getItem(WHITE_GUEST_SECRET_STORAGE_KEY) ?? undefined,
    sessionToken: window.localStorage.getItem(WHITE_GUEST_TOKEN_STORAGE_KEY) ?? undefined,
    sessionExpiresAt: window.localStorage.getItem(WHITE_GUEST_TOKEN_EXPIRY_STORAGE_KEY) ?? undefined,
  };
}

function writeStoredGuestSession(session: GuestSession): void {
  if (typeof window === 'undefined') {
    return;
  }
  window.localStorage.setItem(WHITE_GUEST_ID_STORAGE_KEY, session.guest.guestId);
  window.localStorage.setItem(WHITE_GUEST_SECRET_STORAGE_KEY, session.sessionSecret);
  if ((session.sessionToken ?? '').trim()) {
    window.localStorage.setItem(WHITE_GUEST_TOKEN_STORAGE_KEY, session.sessionToken ?? '');
  } else {
    window.localStorage.removeItem(WHITE_GUEST_TOKEN_STORAGE_KEY);
  }
  if ((session.expiresAt ?? '').trim()) {
    window.localStorage.setItem(WHITE_GUEST_TOKEN_EXPIRY_STORAGE_KEY, session.expiresAt ?? '');
  } else {
    window.localStorage.removeItem(WHITE_GUEST_TOKEN_EXPIRY_STORAGE_KEY);
  }
}

function readStoredAccountIdentity(): StoredAccountIdentity {
  if (typeof window === 'undefined') {
    return {};
  }
  return {
    accountId: window.localStorage.getItem(WHITE_ACCOUNT_ID_STORAGE_KEY) ?? undefined,
    sessionToken: window.localStorage.getItem(WHITE_ACCOUNT_TOKEN_STORAGE_KEY) ?? undefined,
    expiresAt: window.localStorage.getItem(WHITE_ACCOUNT_EXPIRY_STORAGE_KEY) ?? undefined,
  };
}

function writeStoredAccountSession(session: AccountSession | null): void {
  if (typeof window === 'undefined') {
    return;
  }
  if (!session) {
    window.localStorage.removeItem(WHITE_ACCOUNT_ID_STORAGE_KEY);
    window.localStorage.removeItem(WHITE_ACCOUNT_TOKEN_STORAGE_KEY);
    window.localStorage.removeItem(WHITE_ACCOUNT_EXPIRY_STORAGE_KEY);
    return;
  }
  window.localStorage.setItem(WHITE_ACCOUNT_ID_STORAGE_KEY, session.account.accountId);
  window.localStorage.setItem(WHITE_ACCOUNT_TOKEN_STORAGE_KEY, session.sessionToken);
  window.localStorage.setItem(WHITE_ACCOUNT_EXPIRY_STORAGE_KEY, session.expiresAt);
}

function suggestHandle(seed: string): string {
  const normalized = seed
    .toLowerCase()
    .replace(/[^a-z0-9_-]+/g, '_')
    .replace(/^_+|_+$/g, '')
    .slice(0, 24);
  if (normalized.length >= 3) {
    return normalized;
  }
  return 'chess404_player';
}

interface AuthPageProps {
  hostedRuntime: boolean | null;
  guestProfile?: GuestProfile | null;
  externalNotice?: string | null;
  onAuthenticated?: (guestSession: GuestSession, accountSession: AccountSession) => void;
  onOpenAccount?: () => void;
  onContinue?: () => void;
  onAuthStateChange?: () => void;
}

export default function AuthPage({
  hostedRuntime,
  guestProfile = null,
  externalNotice = null,
  onAuthenticated,
  onOpenAccount,
  onContinue,
  onAuthStateChange,
}: AuthPageProps): React.ReactElement {
  const [activeTab, setActiveTab] = React.useState<'register' | 'login' | 'reset'>('register');
  const [guestIdentity, setGuestIdentity] = React.useState<StoredGuestIdentity>(() => readStoredGuestIdentity());
  const [accountSession, setAccountSession] = React.useState<AccountSession | null>(null);
  const [authOverview, setAuthOverview] = React.useState<AccountAuthOverview | null>(null);
  const [handle, setHandle] = React.useState('');
  const [email, setEmail] = React.useState('');
  const [password, setPassword] = React.useState('');
  const [loginIdentifier, setLoginIdentifier] = React.useState('');
  const [loginPassword, setLoginPassword] = React.useState('');
  const [resetIdentifier, setResetIdentifier] = React.useState('');
  const [loading, setLoading] = React.useState(true);
  const [busy, setBusy] = React.useState(false);
  const [error, setError] = React.useState('');
  const [notice, setNotice] = React.useState('');

  React.useEffect(() => {
    setGuestIdentity(readStoredGuestIdentity());
  }, [guestProfile?.guestId]);

  React.useEffect(() => {
    setHandle((current) => {
      if (current.trim()) {
        return current;
      }
      if (guestProfile?.displayName) {
        return suggestHandle(guestProfile.displayName);
      }
      if (guestIdentity.guestId) {
        return suggestHandle(guestIdentity.guestId);
      }
      return 'chess404_player';
    });
  }, [guestIdentity.guestId, guestProfile?.displayName]);

  const refreshStoredAccount = React.useCallback(async () => {
    setLoading(true);
    setError('');
    setGuestIdentity(readStoredGuestIdentity());
    const storedAccount = readStoredAccountIdentity();
    if (!storedAccount.accountId || !storedAccount.sessionToken) {
      setAccountSession(null);
      setAuthOverview(null);
      setLoading(false);
      return;
    }

    try {
      const session = await resumeAccountSession({
        accountId: storedAccount.accountId,
        sessionToken: storedAccount.sessionToken,
      });
      writeStoredAccountSession(session);
      setAccountSession(session);
      onAuthStateChange?.();
    } catch (err) {
      writeStoredAccountSession(null);
      setAccountSession(null);
      setAuthOverview(null);
      onAuthStateChange?.();
      if (isAccountRestrictionError(err)) {
        setNotice(formatAccountRestrictionNotice(err.restriction));
        setError('');
      } else {
        setError(err instanceof Error ? err.message : 'Failed to restore the active account session.');
      }
    } finally {
      setLoading(false);
    }
  }, [onAuthStateChange]);

  React.useEffect(() => {
    void refreshStoredAccount();
  }, [refreshStoredAccount]);

  React.useEffect(() => {
    if (!accountSession) {
      setAuthOverview(null);
      return;
    }

    let cancelled = false;
    void fetchAccountAuthOverview({
      accountId: accountSession.account.accountId,
      sessionToken: accountSession.sessionToken,
    })
      .then((overview) => {
        if (!cancelled) {
          setAuthOverview(overview);
          if (overview.email) {
            setEmail((current) => current.trim() ? current : overview.email ?? current);
          }
        }
      })
      .catch(() => {
        if (!cancelled) {
          setAuthOverview(null);
        }
      });

    return () => {
      cancelled = true;
    };
  }, [accountSession?.account.accountId, accountSession?.sessionToken]);

  React.useEffect(() => {
    if (!accountSession?.account.handle) {
      return;
    }
    setLoginIdentifier((current) => current.trim() ? current : accountSession.account.handle);
  }, [accountSession?.account.handle]);

  const completeAuth = React.useCallback((guestSession: GuestSession, nextAccountSession: AccountSession) => {
    writeStoredGuestSession(guestSession);
    writeStoredAccountSession(nextAccountSession);
    setGuestIdentity({
      guestId: guestSession.guest.guestId,
      sessionSecret: guestSession.sessionSecret,
      sessionToken: guestSession.sessionToken,
      sessionExpiresAt: guestSession.expiresAt,
    });
    setAccountSession(nextAccountSession);
    onAuthenticated?.(guestSession, nextAccountSession);
    onAuthStateChange?.();
  }, [onAuthenticated, onAuthStateChange]);

  const submitRegistration = React.useCallback(async () => {
    if (!handle.trim()) { setError('Handle is required'); setBusy(false); return; }
    if (handle.trim().length < 2) { setError('Handle must be at least 2 characters'); setBusy(false); return; }
    if (!email.trim()) { setError('Email is required'); setBusy(false); return; }
    if (!email.includes('@') || !email.includes('.')) { setError('Enter a valid email address'); setBusy(false); return; }
    if (!password) { setError('Password is required'); setBusy(false); return; }
    if (password.length < 8) { setError('Password must be at least 8 characters'); setBusy(false); return; }
    setBusy(true);
    setError('');
    setNotice('');
    try {
      const liveGuestIdentity = readStoredGuestIdentity();
      const result = await registerAccountWithPassword({
        handle,
        email,
        password,
        guestId: liveGuestIdentity.guestId,
        sessionSecret: liveGuestIdentity.sessionSecret,
        sessionToken: liveGuestIdentity.sessionToken,
      });
      completeAuth(result.guest, result.account);
      setAuthOverview(result.overview);
      setPassword('');
      setLoginPassword('');
      setLoginIdentifier(result.account.account.handle);
      setNotice(
        result.requestedVerification
          ? `Account created as @${result.account.account.handle}. Verification is heading to ${result.overview.email ?? email.trim().toLowerCase()}.`
          : `Account created as @${result.account.account.handle}.`
      );
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create the account.');
    } finally {
      setBusy(false);
    }
  }, [completeAuth, email, handle, password]);

  const submitLogin = React.useCallback(async () => {
    if (!loginIdentifier.trim()) { setError('Email or handle is required'); setBusy(false); return; }
    if (!loginPassword) { setError('Password is required'); setBusy(false); return; }
    setBusy(true);
    setError('');
    setNotice('');
    try {
      const result = await loginAccountWithPassword({
        identifier: loginIdentifier,
        password: loginPassword,
      });
      completeAuth(result.guest, result.account);
      setLoginPassword('');
      setLoginIdentifier(result.account.account.handle);
      setNotice(`Signed in as @${result.account.account.handle}.`);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to sign in.');
    } finally {
      setBusy(false);
    }
  }, [completeAuth, loginIdentifier, loginPassword]);

  const submitPasswordReset = React.useCallback(async () => {
    if (!resetIdentifier.trim()) { setError('Email or handle is required'); setBusy(false); return; }
    setBusy(true);
    setError('');
    setNotice('');
    try {
      const result = await requestPasswordReset({
        identifier: resetIdentifier,
      });
      const destination = result.email?.trim() ? result.email : 'the email tied to that account';
      setNotice(
        result.previewToken
          ? `Password reset preview generated for ${destination}.`
          : `Password reset instructions were queued for ${destination}.`
      );
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to request password reset.');
    } finally {
      setBusy(false);
    }
  }, [resetIdentifier]);

  const signOut = React.useCallback(async () => {
    if (!accountSession) {
      return;
    }
    setBusy(true);
    setError('');
    setNotice('');
    try {
      await logoutAccountSession({
        accountId: accountSession.account.accountId,
        sessionToken: accountSession.sessionToken,
      });
      writeStoredAccountSession(null);
      setAccountSession(null);
      setAuthOverview(null);
      onAuthStateChange?.();
      setNotice('Signed out on this device.');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to sign out.');
    } finally {
      setBusy(false);
    }
  }, [accountSession, onAuthStateChange]);

  const accountBadge = authOverview?.emailVerified
    ? 'Verified account'
    : authOverview?.pendingEmailVerification
      ? 'Verification pending'
      : 'Account ready';

  const accountStatus = authOverview?.emailVerified
    ? `Verified email${authOverview.email ? `: ${authOverview.email}` : ''}`
    : authOverview?.pendingEmailVerification
      ? `Verification queued${authOverview.verificationExpiresAt ? ` until ${formatDateTime(authOverview.verificationExpiresAt)}` : ''}`
      : authOverview?.passwordLoginEnabled
        ? 'Password sign-in is enabled'
        : 'Password sign-in still needs setup';

  return (
    <div style={{ display:'flex', flex:1, minHeight:0, alignItems:'center', justifyContent:'center', padding:'30px' }}>
      <div style={{
        width:'min(480px, 100%)',
      }}>
        <div className="stat-card">
          {loading ? (
            <div style={{ color:'rgba(255,232,184,0.76)', fontSize:'14px', lineHeight:1.7 }}>
              Restoring the last account session for this device...
            </div>
          ) : accountSession ? (
            <>
              <div style={{ display:'grid', gap:'10px' }}>
                <div style={{
                  width:'fit-content',
                  padding:'6px 11px',
                  borderRadius:'999px',
                  background:'rgba(255,215,128,0.10)',
                  border:'1px solid rgba(255,215,128,0.14)',
                  color:'#ffd98e',
                  fontSize:'11px',
                  fontWeight:800,
                  letterSpacing:'0.9px',
                  textTransform:'uppercase',
                }}>
                  {accountBadge}
                </div>
                <div style={{ color:'#fff4d8', fontSize:'28px', fontWeight:900 }}>
                  @{accountSession.account.handle}
                </div>
                <div style={{ color:'rgba(255,232,184,0.72)', fontSize:'14px', lineHeight:1.7 }}>
                  {accountStatus}
                </div>
              </div>

              <div style={{
                borderRadius:'18px',
                padding:'16px 17px',
                background:'rgba(255,255,255,0.03)',
                border:'1px solid rgba(255,255,255,0.07)',
                display:'grid',
                gap:'8px',
              }}>
                <div style={{ color:'#fff0c9', fontSize:'13px', fontWeight:800 }}>
                  Session restore is active
                </div>
                <div style={{ color:'rgba(255,232,184,0.72)', fontSize:'12px', lineHeight:1.65 }}>
                  This device is signed in and can restore your playable identity for queue, private lobbies, and live match ownership.
                </div>
                <div style={{ color:'rgba(255,232,184,0.58)', fontSize:'11px' }}>
                  Session expires: {formatDateTime(accountSession.expiresAt)}
                </div>
              </div>

              <div style={{ display:'grid', gap:'10px' }}>
                <button
                  className="btn-primary"
                  onClick={onContinue}
                  style={{ padding: '12px 16px', boxShadow:'0 8px 20px rgba(200,134,10,0.28)' }}
                >
                  {hostedRuntime ? 'Go To Play' : 'Open Board'}
                </button>
                <button
                  className="btn-ghost"
                  onClick={() => onOpenAccount?.()}
                  style={{ padding: '12px 16px' }}
                >
                  Open Account And Security
                </button>
                <button
                  onClick={() => { void signOut(); }}
                  disabled={busy}
                  style={{
                    padding:'12px 16px',
                    borderRadius:'12px',
                    border:'1px solid rgba(255,120,120,0.22)',
                    background:'rgba(80,18,18,0.38)',
                    color:'#ffd8d8',
                    fontSize:'12px',
                    fontWeight:800,
                    cursor: busy ? 'default' : 'pointer',
                    opacity: busy ? 0.7 : 1,
                  }}
                >
                  Sign Out On This Device
                </button>
              </div>
            </>
          ) : (
            <>
              <div style={{ display:'grid', gap:'10px' }}>
                <div style={{ color:'#fff4d8', fontSize:'28px', fontWeight:900 }}>
                  Create or restore your account
                </div>
                <div style={{ color:'rgba(255,232,184,0.72)', fontSize:'14px', lineHeight:1.7 }}>
                  Start with one Chess404 account for quick pair, private invites, rankings, replays, and future rated progress.
                </div>
              </div>

              <div style={{
                display:'grid',
                gridTemplateColumns:'repeat(3, minmax(0, 1fr))',
                gap:'8px',
                padding:'6px',
                borderRadius:'14px',
                background:'rgba(255,255,255,0.03)',
                border:'1px solid rgba(255,255,255,0.06)',
              }}>
                {[
                  { key: 'register' as const, label: 'Register' },
                  { key: 'login' as const, label: 'Sign In' },
                  { key: 'reset' as const, label: 'Recover' },
                ].map((tab) => (
                  <button
                    key={tab.key}
                    onClick={() => setActiveTab(tab.key)}
                    style={{
                      padding:'11px 10px',
                      borderRadius:'10px',
                      border: activeTab === tab.key ? '1px solid rgba(255,180,60,0.38)' : '1px solid transparent',
                      background: activeTab === tab.key ? 'rgba(200,134,10,0.18)' : 'transparent',
                      color: activeTab === tab.key ? '#fff2c7' : 'rgba(255,232,184,0.72)',
                      fontSize:'12px',
                      fontWeight:800,
                      cursor:'pointer',
                    }}
                  >
                    {tab.label}
                  </button>
                ))}
              </div>

              {activeTab === 'register' ? (
                <div style={{ display:'grid', gap:'12px' }}>
                  <label style={{ display:'grid', gap:'6px' }}>
                    <span style={{ color:'rgba(255,232,184,0.74)', fontSize:'12px', fontWeight:700 }}>Handle</span>
                    <input
                      className="input input-glow"
                      value={handle}
                      onChange={(event) => setHandle(event.target.value)}
                      placeholder="wizard404error"
                    />
                  </label>
                  <label style={{ display:'grid', gap:'6px' }}>
                    <span style={{ color:'rgba(255,232,184,0.74)', fontSize:'12px', fontWeight:700 }}>Email</span>
                    <input
                      className="input input-glow"
                      value={email}
                      onChange={(event) => setEmail(event.target.value)}
                      placeholder="you@example.com"
                      type="email"
                    />
                  </label>
                  <label style={{ display:'grid', gap:'6px' }}>
                    <span style={{ color:'rgba(255,232,184,0.74)', fontSize:'12px', fontWeight:700 }}>Password</span>
                    <input
                      className="input input-glow"
                      value={password}
                      onChange={(event) => setPassword(event.target.value)}
                      placeholder="Choose a strong password"
                      type="password"
                    />
                  </label>
                  <button
                    className="btn-primary"
                    onClick={() => { void submitRegistration(); }}
                    disabled={busy}
                    style={{ padding: '13px 16px' }}
                  >
                    {busy ? 'Creating account...' : 'Create Account'}
                  </button>
                </div>
              ) : activeTab === 'login' ? (
                <div style={{ display:'grid', gap:'12px' }}>
                  <label style={{ display:'grid', gap:'6px' }}>
                    <span style={{ color:'rgba(255,232,184,0.74)', fontSize:'12px', fontWeight:700 }}>Handle or email</span>
                    <input
                      className="input input-glow"
                      value={loginIdentifier}
                      onChange={(event) => setLoginIdentifier(event.target.value)}
                      placeholder="wizard404error or you@example.com"
                    />
                  </label>
                  <label style={{ display:'grid', gap:'6px' }}>
                    <span style={{ color:'rgba(255,232,184,0.74)', fontSize:'12px', fontWeight:700 }}>Password</span>
                    <input
                      className="input input-glow"
                      value={loginPassword}
                      onChange={(event) => setLoginPassword(event.target.value)}
                      placeholder="Your account password"
                      type="password"
                    />
                  </label>
                  <button
                    className="btn-primary"
                    onClick={() => { void submitLogin(); }}
                    disabled={busy}
                    style={{ padding: '13px 16px' }}
                  >
                    {busy ? 'Signing in...' : 'Sign In'}
                  </button>
                </div>
              ) : (
                <div style={{ display:'grid', gap:'12px' }}>
                  <label style={{ display:'grid', gap:'6px' }}>
                    <span style={{ color:'rgba(255,232,184,0.74)', fontSize:'12px', fontWeight:700 }}>Handle or email</span>
                    <input
                      className="input input-glow"
                      value={resetIdentifier}
                      onChange={(event) => setResetIdentifier(event.target.value)}
                      placeholder="wizard404error or you@example.com"
                    />
                  </label>
                  <button
                    className="btn-ghost"
                    onClick={() => { void submitPasswordReset(); }}
                    disabled={busy}
                    style={{ padding: '13px 16px' }}
                  >
                    {busy ? 'Requesting reset...' : 'Send Password Reset'}
                  </button>
                </div>
              )}
            </>
          )}

          {(externalNotice || notice) ? (
            <div style={{
              padding:'12px 14px',
              borderRadius:'14px',
              background:'rgba(48,108,74,0.22)',
              border:'1px solid rgba(94,234,162,0.18)',
              color:'#dbffe7',
              fontSize:'12px',
              lineHeight:1.6,
            }}>
              {externalNotice || notice}
            </div>
          ) : null}

          {error ? (
            <div style={{
              padding:'12px 14px',
              borderRadius:'14px',
              background:'rgba(108,48,48,0.24)',
              border:'1px solid rgba(255,130,130,0.18)',
              color:'#ffdcdc',
              fontSize:'12px',
              lineHeight:1.6,
            }}>
              {error}
            </div>
          ) : null}
        </div>
      </div>
    </div>
  );
}
