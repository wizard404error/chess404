import type { MatchModeId, PieceColor } from '@chess404/contracts';
import type { PrivateMatchAccessResponse, PrivateMatchIdentity } from './private-match-service';
import { DEFAULT_MATCH_MODE_ID } from '@chess404/contracts';

export interface DirectChallengeLaunchResponse {
  challengeId: string;
  modeId?: MatchModeId;
  match: PrivateMatchAccessResponse;
}

export async function sendDirectChallenge(input: {
  identity: PrivateMatchIdentity;
  targetAccountId: string;
  modeId?: MatchModeId;
  clockSeconds?: number;
  preferredSeat?: PieceColor;
}): Promise<DirectChallengeLaunchResponse> {
  const response = await fetch('/api/gateway/challenges', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({
      guest: {
        guestId: input.identity.guestId,
        sessionSecret: input.identity.sessionSecret,
        sessionToken: input.identity.sessionToken,
      },
      account: input.identity.accountId ? {
        accountId: input.identity.accountId,
        sessionToken: input.identity.accountSessionToken,
      } : undefined,
      targetAccountId: input.targetAccountId,
      modeId: input.modeId ?? DEFAULT_MATCH_MODE_ID,
      clockSeconds: input.clockSeconds ?? 600,
      preferredSeat: input.preferredSeat ?? 'white',
    }),
  });

  return unwrapResponse<DirectChallengeLaunchResponse>(response);
}

export async function acceptDirectChallenge(input: {
  challengeId: string;
  identity: PrivateMatchIdentity;
}): Promise<DirectChallengeLaunchResponse> {
  const response = await fetch(`/api/gateway/challenges/${encodeURIComponent(input.challengeId)}/accept`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({
      guest: {
        guestId: input.identity.guestId,
        sessionSecret: input.identity.sessionSecret,
        sessionToken: input.identity.sessionToken,
      },
      account: input.identity.accountId ? {
        accountId: input.identity.accountId,
        sessionToken: input.identity.accountSessionToken,
      } : undefined,
    }),
  });

  return unwrapResponse<DirectChallengeLaunchResponse>(response);
}

async function unwrapResponse<T>(response: Response): Promise<T> {
  if (!response.ok) {
    let message = `Request failed with ${response.status}`;
    try {
      const payload = (await response.json()) as { error?: string };
      if (payload?.error) {
        message = payload.error;
      }
    } catch {
      // Keep fallback message.
    }
    throw new Error(message);
  }

  return response.json() as Promise<T>;
}
