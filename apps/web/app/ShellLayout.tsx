'use client';

import React from 'react';
import { usePathname, useRouter } from 'next/navigation';
import AppShell, { ShellNavGroup, ShellNavItem } from '../src/components/layout/AppShell';
import {
  AdminIcon,
  CardsIcon,
  CommunityIcon,
  FriendsIcon,
  HistoryIcon,
  InboxIcon,
  PlayIcon,
  ProfileIcon,
  StatusIcon,
  WatchIcon,
} from '../src/components/layout/icons';

const primaryItems: ShellNavItem[] = [
  { key: '/play', label: 'Play', icon: <PlayIcon /> },
  { key: '/watch', label: 'Watch', icon: <WatchIcon /> },
];

const utilityGroups: ShellNavGroup[] = [
  {
    label: 'Social',
    items: [
      { key: '/rankings', label: 'Rankings', icon: <ProfileIcon /> },
      { key: '/profiles', label: 'Profiles', icon: <ProfileIcon /> },
      { key: '/community', label: 'Community', icon: <CommunityIcon /> },
    ],
  },
  {
    label: 'Library',
    items: [
      { key: '/cards', label: 'Card Index', icon: <CardsIcon /> },
      { key: '/history', label: 'History', icon: <HistoryIcon /> },
    ],
  },
];

export function ShellLayout({ children }: { children: React.ReactNode }) {
  const router = useRouter();
  const pathname = usePathname();
  const [accountOpen, setAccountOpen] = React.useState(false);

  // In a real implementation, pageMeta would be dynamic based on pathname
  const pageMeta = {
    title: pathname === '/' ? 'Home' : pathname.slice(1).charAt(0).toUpperCase() + pathname.slice(2),
  };

  const handleNavigate = (key: string) => {
    router.push(key);
  };

  return (
    <AppShell
      brandTitle="Chess404"
      brandSubtitle="Arcane Chess Platform"
      pageMeta={pageMeta}
      primaryItems={primaryItems}
      utilityGroups={utilityGroups}
      accountLabel="Account"
      activeKey={pathname}
      onNavigate={handleNavigate}
      onOpenAccount={() => setAccountOpen(true)}
    >
      {children}
      {/* 
        In full implementation, the Auth/Account modal would be rendered here 
        when accountOpen is true 
      */}
    </AppShell>
  );
}
