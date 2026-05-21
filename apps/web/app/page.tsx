import { redirect } from 'next/navigation';

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

  redirect('/play');
}
