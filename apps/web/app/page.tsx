import { redirect } from 'next/navigation';
import Link from 'next/link';

function firstParam(value: string | string[] | undefined): string {
  if (Array.isArray(value)) {
    return value[0]?.trim() ?? '';
  }
  return value?.trim() ?? '';
}

export default async function HomePage({
  searchParams,
}: {
  searchParams: Promise<Record<string, string | string[] | undefined>>;
}) {
  const params = await searchParams;
  const requestedMatchId = firstParam(params.match);
  const requestedReplayMatchId = firstParam(params.replay);
  const requestedGuestId = firstParam(params.guest);
  const requestedProfileHandle = firstParam(params.profile).toLowerCase();
  const requestedAuthAction = firstParam(params.auth);
  const requestedAuthToken = firstParam(params.token);
  const requestedAccountId = firstParam(params.account);

  if (requestedMatchId) {
    redirect(`/match/${encodeURIComponent(requestedMatchId)}`);
  }

  if (requestedReplayMatchId || requestedGuestId) {
    const historyParams = new URLSearchParams();
    if (requestedReplayMatchId) {
      historyParams.set('replay', requestedReplayMatchId);
    }
    if (requestedGuestId) {
      historyParams.set('guest', requestedGuestId);
    }
    redirect(`/history${historyParams.size ? `?${historyParams.toString()}` : ''}`);
  }

  if (requestedProfileHandle) {
    const profileParams = new URLSearchParams({ profile: requestedProfileHandle });
    redirect(`/profiles?${profileParams.toString()}`);
  }

  if (
    (requestedAuthAction === 'verify-email' || requestedAuthAction === 'reset-password') &&
    requestedAuthToken
  ) {
    const accountParams = new URLSearchParams({
      auth: requestedAuthAction,
      token: requestedAuthToken,
    });
    if (requestedAccountId) {
      accountParams.set('account', requestedAccountId);
    }
    redirect(`/account?${accountParams.toString()}`);
  }

  return (
    <div className="landing-page">
      <section className="hero">
        <h1 className="hero-title">Chess404</h1>
        <p className="hero-subtitle">
          Competitive online chess with curated card powers. Outplay, outwit, outshine.
        </p>
        <div className="hero-actions">
          <Link href="/play" className="btn-primary">Play Now</Link>
          <Link href="/watch" className="btn-secondary">Watch Games</Link>
        </div>
      </section>
      <section className="features">
        <div className="feature-card">
          <h3>Chess + Cards</h3>
          <p>Every game combines classic chess with tactical card abilities. Freeze enemy pieces, teleport across the board, or shield your king.</p>
        </div>
        <div className="feature-card">
          <h3>Ranked Play</h3>
          <p>Climb the leaderboard with competitive matchmaking. Time controls, draws, and resignations all supported.</p>
        </div>
        <div className="feature-card">
          <h3>Guest or Account</h3>
          <p>Jump in as a guest instantly or create an account to save your progress, stats, and history.</p>
        </div>
      </section>
    </div>
  );
}
