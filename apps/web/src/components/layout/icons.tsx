import React from 'react';

type IconProps = {
  className?: string;
};

function icon(className: string | undefined, path: React.ReactNode): React.ReactElement {
  return (
    <svg
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.8"
      strokeLinecap="round"
      strokeLinejoin="round"
      className={className}
      aria-hidden="true"
    >
      {path}
    </svg>
  );
}

export function BrandCrestIcon({ className }: IconProps): React.ReactElement {
  return icon(className, (
    <>
      <path d="M6 18h12" />
      <path d="M8 18 9.2 10.5" />
      <path d="M16 18 14.8 10.5" />
      <path d="M7 7.5 9 10.5l3-4 3 4 2-3" />
      <path d="M8 6a1 1 0 1 0 0-.01" />
      <path d="M12 4a1 1 0 1 0 0-.01" />
      <path d="M16 6a1 1 0 1 0 0-.01" />
    </>
  ));
}

export function PlayIcon({ className }: IconProps): React.ReactElement {
  return icon(className, (
    <>
      <path d="m8 6 10 6-10 6z" />
      <path d="M4 6v12" />
    </>
  ));
}

export function WatchIcon({ className }: IconProps): React.ReactElement {
  return icon(className, (
    <>
      <path d="M2 12s3.5-6 10-6 10 6 10 6-3.5 6-10 6S2 12 2 12Z" />
      <circle cx="12" cy="12" r="3" />
    </>
  ));
}

export function TrophyIcon({ className }: IconProps): React.ReactElement {
  return icon(className, (
    <>
      <path d="M8 4h8v4a4 4 0 0 1-8 0Z" />
      <path d="M9 20h6" />
      <path d="M12 16v4" />
      <path d="M8 6H5a2 2 0 0 0 0 4h1" />
      <path d="M16 6h3a2 2 0 0 1 0 4h-1" />
    </>
  ));
}

export function ProfileIcon({ className }: IconProps): React.ReactElement {
  return icon(className, (
    <>
      <circle cx="12" cy="8" r="3.2" />
      <path d="M5.5 19a6.5 6.5 0 0 1 13 0" />
    </>
  ));
}

export function HistoryIcon({ className }: IconProps): React.ReactElement {
  return icon(className, (
    <>
      <path d="M3 12a9 9 0 1 0 3-6.7" />
      <path d="M3 4v5h5" />
      <path d="M12 7v5l3 2" />
    </>
  ));
}

export function CardsIcon({ className }: IconProps): React.ReactElement {
  return icon(className, (
    <>
      <rect x="6" y="4" width="11" height="16" rx="2" />
      <path d="M9 8h5" />
      <path d="M9 12h5" />
      <path d="M4 7v10a2 2 0 0 0 2 2" />
    </>
  ));
}

export function CommunityIcon({ className }: IconProps): React.ReactElement {
  return icon(className, (
    <>
      <circle cx="8" cy="9" r="2" />
      <circle cx="16" cy="9" r="2" />
      <path d="M4.5 18a4 4 0 0 1 7 0" />
      <path d="M12.5 18a4 4 0 0 1 7 0" />
    </>
  ));
}

export function FriendsIcon({ className }: IconProps): React.ReactElement {
  return icon(className, (
    <>
      <circle cx="9" cy="9" r="2.5" />
      <path d="M4.5 18a5 5 0 0 1 9 0" />
      <path d="M17 8v6" />
      <path d="M14 11h6" />
    </>
  ));
}

export function InboxIcon({ className }: IconProps): React.ReactElement {
  return icon(className, (
    <>
      <path d="M4 6h16v12H4z" />
      <path d="m4 12 4-3h8l4 3" />
    </>
  ));
}

export function AccountIcon({ className }: IconProps): React.ReactElement {
  return icon(className, (
    <>
      <circle cx="12" cy="8" r="3" />
      <path d="M6 19c1.5-3 10.5-3 12 0" />
    </>
  ));
}

export function AdminIcon({ className }: IconProps): React.ReactElement {
  return icon(className, (
    <>
      <path d="M12 3 5 6v6c0 4 3 7 7 9 4-2 7-5 7-9V6Z" />
      <path d="M9.5 12.5 11 14l3.5-4" />
    </>
  ));
}

export function StatusIcon({ className }: IconProps): React.ReactElement {
  return icon(className, (
    <>
      <path d="M4 18h16" />
      <path d="M7 15V9" />
      <path d="M12 15V6" />
      <path d="M17 15v-3" />
    </>
  ));
}

export function ToolsIcon({ className }: IconProps): React.ReactElement {
  return icon(className, (
    <>
      <path d="M4 7h16" />
      <path d="M4 12h16" />
      <path d="M4 17h16" />
    </>
  ));
}

export function ReturnIcon({ className }: IconProps): React.ReactElement {
  return icon(className, (
    <>
      <path d="M10 7 5 12l5 5" />
      <path d="M19 12H5" />
    </>
  ));
}
