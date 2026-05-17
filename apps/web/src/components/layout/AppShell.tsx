import React from 'react';
import {
  AccountIcon,
  BrandCrestIcon,
  ReturnIcon,
  ToolsIcon,
} from './icons';

export type ShellNavItem = {
  key: string;
  label: string;
  icon: React.ReactNode;
  badge?: number | null;
};

export type ShellNavGroup = {
  label?: string;
  items: ShellNavItem[];
};

export type ShellPageMeta = {
  eyebrow?: string;
  title: string;
  description?: string;
};

interface AppShellProps {
  brandTitle: string;
  brandSubtitle: string;
  pageMeta: ShellPageMeta;
  primaryItems: ShellNavItem[];
  utilityGroups: ShellNavGroup[];
  accountLabel: string;
  activeKey: string;
  onNavigate: (key: string) => void;
  onOpenAccount: () => void;
  showReturnToMatch?: boolean;
  onReturnToMatch?: () => void;
  topNotice?: React.ReactNode;
  children: React.ReactNode;
}

function SidebarItem({
  item,
  active,
  onClick,
}: {
  item: ShellNavItem;
  active: boolean;
  onClick: () => void;
}): React.ReactElement {
  return (
    <button className={`app-shell__nav-item${active ? ' app-shell__nav-item--active' : ''}`} onClick={onClick}>
      <span className="app-shell__nav-main">
        <span className="app-shell__nav-icon">{item.icon}</span>
        <span className="app-shell__nav-text">{item.label}</span>
      </span>
      {item.badge ? <span className="app-shell__nav-badge">{item.badge}</span> : null}
    </button>
  );
}

export default function AppShell({
  brandTitle,
  brandSubtitle,
  pageMeta,
  primaryItems,
  utilityGroups,
  accountLabel,
  activeKey,
  onNavigate,
  onOpenAccount,
  showReturnToMatch = false,
  onReturnToMatch,
  topNotice = null,
  children,
}: AppShellProps): React.ReactElement {
  const [mobileToolsOpen, setMobileToolsOpen] = React.useState(false);

  React.useEffect(() => {
    setMobileToolsOpen(false);
  }, [activeKey]);

  const bottomNavItems = [...primaryItems, { key: '__account__', label: 'Account', icon: <AccountIcon /> }];

  return (
    <div className="app-root">
      <div className="app-shell">
        <aside className="app-shell__sidebar">
          <div className="app-shell__brand">
            <div className="app-shell__brand-mark">
              <BrandCrestIcon />
            </div>
            <div className="app-shell__brand-copy">
              <div className="app-shell__brand-title">{brandTitle}</div>
              <div className="app-shell__brand-subtitle">{brandSubtitle}</div>
            </div>
          </div>

          <div className="app-shell__nav-group">
            <div className="app-shell__nav-label">Core</div>
            {primaryItems.map((item) => (
              <SidebarItem
                key={item.key}
                item={item}
                active={activeKey === item.key}
                onClick={() => onNavigate(item.key)}
              />
            ))}
          </div>

          {utilityGroups.map((group, index) => (
            <div className="app-shell__nav-group" key={`${group.label ?? 'utility'}-${index}`}>
              {group.label ? <div className="app-shell__nav-label">{group.label}</div> : null}
              {group.items.map((item) => (
                <SidebarItem
                  key={item.key}
                  item={item}
                  active={activeKey === item.key}
                  onClick={() => onNavigate(item.key)}
                />
              ))}
            </div>
          ))}

          <div className="app-shell__sidebar-footer">
            <button className={`app-shell__nav-item${activeKey === 'Account' ? ' app-shell__nav-item--active' : ''}`} onClick={onOpenAccount}>
              <span className="app-shell__nav-main">
                <span className="app-shell__nav-icon"><AccountIcon /></span>
                <span className="app-shell__nav-text">{accountLabel}</span>
              </span>
            </button>
          </div>
        </aside>

        <div className="app-shell__content">
          <header className="app-shell__topbar">
            <div className="app-shell__topbar-meta">
              {pageMeta.eyebrow ? <div className="eyebrow">{pageMeta.eyebrow}</div> : null}
              <div className="app-shell__topbar-title">{pageMeta.title}</div>
              {pageMeta.description ? <div className="app-shell__topbar-description">{pageMeta.description}</div> : null}
            </div>
            <div className="app-shell__topbar-actions">
              <button className="btn-ghost app-shell__mobile-tools" onClick={() => setMobileToolsOpen((current) => !current)}>
                <span style={{ display: 'inline-flex', width: 16, height: 16 }}><ToolsIcon /></span>
                Tools
              </button>
              {showReturnToMatch ? (
                <button className="btn-secondary" onClick={onReturnToMatch}>
                  <span style={{ display: 'inline-flex', width: 16, height: 16 }}><ReturnIcon /></span>
                  Return to Match
                </button>
              ) : null}
              <button className="btn-primary" onClick={onOpenAccount}>{accountLabel}</button>
            </div>
          </header>

          {topNotice}

          <main className="app-shell__main">
            {children}
          </main>
        </div>

        {mobileToolsOpen ? (
          <div className="app-shell__utility-sheet">
            {utilityGroups.map((group, index) => (
              <div className="app-shell__nav-group" key={`${group.label ?? 'utility-mobile'}-${index}`}>
                {group.label ? <div className="app-shell__nav-label">{group.label}</div> : null}
                {group.items.map((item) => (
                  <SidebarItem
                    key={item.key}
                    item={item}
                    active={activeKey === item.key}
                    onClick={() => onNavigate(item.key)}
                  />
                ))}
              </div>
            ))}
          </div>
        ) : null}

        <nav className="app-shell__bottom-nav">
          {bottomNavItems.map((item) => {
            const isAccount = item.key === '__account__';
            const active = isAccount ? activeKey === 'Account' : activeKey === item.key;
            return (
              <button
                key={item.key}
                className={active ? 'is-active' : ''}
                onClick={() => {
                  if (isAccount) {
                    onOpenAccount();
                    return;
                  }
                  onNavigate(item.key);
                }}
              >
                <span className="app-shell__nav-icon">{item.icon}</span>
                <span>{item.label}</span>
              </button>
            );
          })}
        </nav>
      </div>
    </div>
  );
}
