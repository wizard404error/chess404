'use client';
import React from 'react';
import { useRouter } from 'next/navigation';
import AdminModerationPage from '../../src/AdminModerationPage';
import { usePlatform } from '../../src/contexts/PlatformContext';

export default function AdminRoute() {
  const router = useRouter();
  const p = usePlatform();

  React.useEffect(() => {
    if (!p.primaryAccountIdentity?.accountId) {
      router.replace('/play');
    }
  }, [p.primaryAccountIdentity?.accountId, router]);

  if (!p.primaryAccountIdentity?.accountId) {
    return null;
  }

  return (
    <AdminModerationPage
      accountId={p.primaryAccountIdentity.accountId}
      sessionToken={p.primaryAccountIdentity.sessionToken ?? null}
      onOpenProfile={p.openProfileHandle}
    />
  );
}
