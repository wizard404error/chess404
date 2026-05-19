'use client';

import React from 'react';
import PlayHubPage from '../../src/PlayHubPage';
import { usePlatform } from '../../src/contexts/PlatformContext';

export default function PlayRoute() {
  const platform = usePlatform();

  return (
    <PlayHubPage
      hostedRuntime={platform.hostedRuntime}
      whiteProfile={platform.whiteProfile}
      blackProfile={platform.blackProfile}
      preferredQueue={platform.queueLaunchIntent?.queue}
      preferredModeId={platform.queueLaunchIntent?.modeId}
      displayName={platform.whiteProfile?.displayName ?? null}
      identity={{
        guestId: platform.readStoredGuestIdentity('white').guestId,
        sessionSecret: platform.readStoredGuestIdentity('white').sessionSecret,
        sessionToken: platform.readStoredGuestIdentity('white').sessionToken,
        accountId: platform.primaryAccountIdentity?.accountId,
        accountSessionToken: platform.primaryAccountIdentity?.sessionToken,
      }}
      activeMatchId={platform.authoritativeMatchId}
      activeMatchQueue={platform.activeMatchRoomMeta?.queue ?? null}
      activeMatchModeId={platform.activeMatchRoomMeta?.modeId ?? null}
      boardStatusLabel={platform.boardStatusLabel}
      viewerSeat={platform.viewerSeat}
      matchDestinationNotice={platform.matchDestinationNotice}
      onReturnToMatch={() => platform.setActivePage('Match')}
      onCopyMatchLink={platform.copyLiveMatchLink}
    />
  );
}
