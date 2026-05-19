"""
Atomic patch for App.tsx:
1. Add imports: PlatformContext, useRouter, usePathname, PlayerBar, GamePanel, MatchLayout, CardHand
2. Add children prop to App signature
3. Add useRouter / usePathname bridge effect after activePage state
4. Wrap entire return JSX in PlatformContext.Provider
5. Replace renderPlayerCard body with <PlayerBar /> component
6. Swap Move-History+Engine column + Chat column for <GamePanel />
7. Inject children-aware render logic
"""

import re

SRC = "src/App.tsx"

with open(SRC, "r", encoding="utf-8") as f:
    text = f.read()

# ── PATCH 1: Add imports after "import React from 'react';" ─────────────────
NEW_IMPORTS = """\
import { PlatformContext } from './contexts/PlatformContext';
import { usePathname, useRouter } from 'next/navigation';
import { PlayerBar } from './components/match/PlayerBar';
import { GamePanel } from './components/match/GamePanel';
"""
text = text.replace(
    "import React from 'react';",
    "import React from 'react';\n" + NEW_IMPORTS,
    1,
)

# ── PATCH 2: Add `children` prop to App function signature ───────────────────
text = text.replace(
    "export default function App({ runtimeConfig }: { runtimeConfig?: { matchServiceHttpBase?: string; matchServiceWsBase?: string } })",
    "export default function App({ runtimeConfig, children }: { runtimeConfig?: { matchServiceHttpBase?: string; matchServiceWsBase?: string }, children?: React.ReactNode })",
    1,
)

# ── PATCH 3: Add router bridge after activePage useState ────────────────────
ROUTER_BRIDGE = """
  // App Router pathname → activePage bridge
  const router = useRouter();
  const pathname = usePathname();
  React.useEffect(() => {
    if (!pathname) return;
    if (pathname === '/' || pathname === '/play') setActivePage('Play');
    else if (pathname === '/watch') setActivePage('Watch');
    else if (pathname === '/history') setActivePage('History');
    else if (pathname === '/friends') setActivePage('Friends');
    else if (pathname === '/inbox') setActivePage('Inbox');
    else if (pathname === '/profiles') setActivePage('Profiles');
    else if (pathname === '/cards') setActivePage('Cards');
    else if (pathname === '/rankings') setActivePage('Rankings');
    else if (pathname === '/community') setActivePage('Community');
    else if (pathname === '/status') setActivePage('Status');
    else if (pathname === '/account') setActivePage('Account');
    else if (pathname === '/admin') setActivePage('Admin');
    else if (pathname.startsWith('/match/')) setActivePage('Match');
  }, [pathname]);
"""
text = text.replace(
    "  const [activePage, setActivePage] = React.useState<AppPage>('Play');",
    "  const [activePage, setActivePage] = React.useState<AppPage>('Play');" + ROUTER_BRIDGE,
    1,
)

# ── PATCH 4: Replace renderPlayerCard body with <PlayerBar /> ────────────────
OLD_RENDER_PC = """\
    return (
      <div style={{
        background: isWhiteSeat ? 'rgba(8,45,18,0.50)' : 'rgba(35,12,58,0.52)',
        backdropFilter:'blur(16px)', WebkitBackdropFilter:'blur(16px)',
        border: isWhiteSeat ? '1px solid rgba(60,220,110,0.45)' : '1px solid rgba(180,110,255,0.35)',
        borderRadius:'16px', padding:'12px 16px',
        display:'flex', alignItems:'center', gap:'12px',
        boxShadow: isWhiteSeat
          ? '0 8px 32px rgba(0,0,0,0.35), inset 0 1px 0 rgba(80,240,130,0.2), 0 0 30px rgba(30,180,70,0.2)'
          : '0 8px 32px rgba(0,0,0,0.35), inset 0 1px 0 rgba(180,110,255,0.12), 0 0 30px rgba(120,70,200,0.18)',
      }}>"""

NEW_RENDER_PC = """\
    return (
      <PlayerBar
        seat={seat}
        playerName={seatName}
        rating={seatRating}
        timeMs={seatTime * 1000}
        isClockActive={seatTicking}
        seatBadge={seatBadge}
      />
    );
  };
  // END renderPlayerCard — DEAD CODE BELOW (replaced by component above)
  const _oldRenderPlayerCard_DEAD = () => {
    return (
      <div style={{
        background: 'rgba(0,0,0,0)',"""

# We do a targeted replacement: find the renderPlayerCard function return and replace it
# Find the exact block
pc_start = text.find(OLD_RENDER_PC)
if pc_start == -1:
    print("WARNING: Could not find renderPlayerCard return block")
else:
    # Find the closing of this function: }; after the last </div>
    search_from = pc_start + len(OLD_RENDER_PC)
    func_end = text.find("\n  };\n", search_from)
    if func_end == -1:
        print("WARNING: Could not find end of renderPlayerCard")
    else:
        func_end += len("\n  };\n")
        # Replace the entire function body return statement
        old_body = text[pc_start:func_end]
        new_body = """\
    return (
      <PlayerBar
        seat={seat}
        playerName={seatName}
        rating={seatRating}
        timeMs={seatTime * 1000}
        isClockActive={seatTicking}
        seatBadge={seatBadge}
      />
    );
  };
"""
        text = text[:pc_start] + new_body + text[func_end:]
        print(f"Replaced renderPlayerCard body (saved {len(old_body) - len(new_body)} chars)")

# ── PATCH 5: Replace Move/Engine + Chat columns with <GamePanel /> ───────────
# Target: the div wrapping Move History + Engine analysis (flex:2)
# AND the Chat div below it.
# We'll identify them by their distinctive markers.

MOVE_ENGINE_MARKER = "          <div style={{ display:'flex', gap:'8px', flex:2, minHeight:0 }}>\n            {/* Move History */}"
CHAT_START_MARKER = "          {/* Chat */}\n          <div style={{ display:'flex', flexDirection:'column', flex:1"

# Find where the Move/Engine block starts
me_start = text.find(MOVE_ENGINE_MARKER)
if me_start == -1:
    print("WARNING: Could not find Move/Engine block")
else:
    # Find the ELO Stakes marker that comes after it (to know where Move/Engine ends)
    elo_marker = "          {/* ELO Stakes */}"
    elo_pos = text.find(elo_marker, me_start)
    if elo_pos == -1:
        print("WARNING: Could not find ELO Stakes marker")
    else:
        # Replace Move/Engine block with GamePanel
        old_me_block = text[me_start:elo_pos]
        
        # Now find where Chat starts and ends (after ELO stakes)
        chat_start_pos = text.find(CHAT_START_MARKER, elo_pos)
        if chat_start_pos != -1:
            # Find the end of the right panel column (the triple closing divs)
            # Chat closes with </div></div></div> at same indent level
            # We look for "        </div>\n      </div>\n" after chat_start
            chat_end_marker = "\n        </div>\n      </div>\n"
            chat_end_pos = text.find(chat_end_marker, chat_start_pos)
            if chat_end_pos != -1:
                # Include the closing tags
                chat_end_pos += len(chat_end_marker)
                
                old_right_panel = text[me_start:chat_end_pos]
                
                new_right_panel = """          <GamePanel
            chatMessages={chatMessages}
            onSendMessage={sendChatMessage}
            isChatDisabled={!chatConnected}
            movHist={movHist}
            engineNode={
              engineOn && ev ? (
                <div style={{ padding: '12px', fontFamily: 'monospace' }}>
                  <div style={{ fontSize: '22px', fontWeight: 'bold', color: ev.score > 0 ? '#2ecc71' : ev.score < 0 ? '#e74c3c' : '#ecf0f1', textAlign: 'center', marginBottom: '8px' }}>
                    {ev.mate != null ? (ev.mate === 0 ? 'Mate' : `M${Math.abs(ev.mate)}`) : (ev.score / 100).toFixed(2)}
                  </div>
                  {ev.best && (
                    <div style={{ color: '#f39c12', textAlign: 'center', fontSize: '13px' }}>
                      Best: {uciToSan(ev.best, reviewIdx >= 0 ? (reviewBoard ?? board) : board)} <span style={{ color: '#7f8c8d', fontSize: '10px' }}>({ev.best})</span>
                    </div>
                  )}
                  {ev.pv && ev.pv.length > 0 && (
                    <div style={{ color: '#bdc3c7', fontSize: '10px', marginTop: '6px', wordBreak: 'break-all' }}>
                      {ev.pv.slice(0, 5).join(' ')}
                    </div>
                  )}
                </div>
              ) : (
                <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', height: '100%', gap: '8px' }}>
                  <div style={{ color: 'rgba(255,255,255,0.3)', fontSize: '12px' }}>Engine {engineOn ? 'calculating...' : 'off'}</div>
                  {over && (
                    <button onClick={() => setEngineOn(v => !v)} style={{ padding: '6px 14px', fontSize: '11px', background: engineOn ? 'linear-gradient(180deg,#1a6fc4,#0d4a8a)' : 'rgba(60,70,90,0.6)', color: '#fff', border: 'none', borderRadius: '6px', cursor: 'pointer', fontWeight: 'bold' }}>
                      {engineOn ? 'ENGINE ON' : 'ENGINE OFF'}
                    </button>
                  )}
                </div>
              )
            }
          />
          {/* ELO Stakes */}
"""
                text = text[:me_start] + new_right_panel + text[chat_end_pos:]
                print(f"Replaced Move/Engine+Chat with GamePanel (saved {len(old_right_panel) - len(new_right_panel)} chars)")
            else:
                print("WARNING: Could not find chat end marker")
        else:
            print("WARNING: Could not find Chat start marker")

# ── PATCH 6: Wrap return JSX in PlatformContext.Provider ────────────────────
PLATFORM_VALUE = """    <PlatformContext.Provider value={{
      hostedRuntime, setHostedRuntime,
      whiteProfile, blackProfile,
      queueLaunchIntent,
      primaryAccountIdentity,
      authoritativeMatchId, setAuthoritativeMatchId,
      activeMatchRoomMeta,
      boardStatusLabel,
      viewerSeat,
      matchDestinationNotice,
      setActivePage,
      openLiveMatch,
      openReplayMatch,
      openProfileHandle,
      openGuestHistory,
      historyFocusMatchId, setHistoryFocusMatchId,
      historyFocusGuestId, setHistoryFocusGuestId,
      communityFocusGuestId, setCommunityFocusGuestId,
      socialLiveToken,
      setInboxUnreadCount,
      profileFocusHandle,
      shellAccountNotice,
      hasPrimaryAccountSession,
      accountActionQueryDetected,
      handlePrimaryShellAuthenticated,
      handleSeatAuthenticated,
      syncPrimaryAccountIdentity,
      writeStoredActiveMatchId,
      clearRequestedMatchQuery,
      requestedMatchIdRef,
      readStoredGuestIdentity,
      copyLiveMatchLink: (matchId: string) => { void copyLiveMatchLink(matchId); },
    }}>
"""

# Inject PlatformContext.Provider after the opening fragment in return
RETURN_OPEN = "  return (\n    <>\n"
if RETURN_OPEN in text:
    text = text.replace(RETURN_OPEN, RETURN_OPEN + PLATFORM_VALUE, 1)
    print("Wrapped return JSX in PlatformContext.Provider")
else:
    print("WARNING: Could not find return open")

# Close it before the closing fragment
RETURN_CLOSE = "\n    </>\n  );\n}"
if RETURN_CLOSE in text:
    text = text.replace(RETURN_CLOSE, "\n    </PlatformContext.Provider>\n    </>\n  );\n}", 1)
    print("Closed PlatformContext.Provider before </> ")
else:
    print("WARNING: Could not find closing fragment")

# ── PATCH 7: Children-aware rendering ────────────────────────────────────────
SHOW_PLAY_HUB = "      {showPlayHub ? ("
CHILDREN_RENDER = "      {(children && activePage !== 'Match') ? children : showPlayHub ? ("
if SHOW_PLAY_HUB in text and CHILDREN_RENDER not in text:
    text = text.replace(SHOW_PLAY_HUB, CHILDREN_RENDER, 1)
    print("Added children-aware rendering")
elif CHILDREN_RENDER in text:
    print("Children-aware rendering already present")

with open(SRC, "w", encoding="utf-8") as f:
    f.write(text)

print(f"\nAll patches applied. Final size: {len(text)} chars")
