'use client';
import React from 'react';
import AdminModerationPage from '../../src/AdminModerationPage';
import { usePlatform } from '../../src/contexts/PlatformContext';

export default function AdminRoute() {
  const p = usePlatform();
  return (
    <AdminModerationPage
      accountId={p.primaryAccountIdentity?.accountId ?? null}
      sessionToken={p.primaryAccountIdentity?.sessionToken ?? null}
      onOpenProfile={p.openProfileHandle}
    />
  );
}
