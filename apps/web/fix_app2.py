"""
Fix App.tsx:
1. Ensure PlatformContext.Provider wraps the AppShell (and all content)
2. Remove stray/duplicate PlatformContext.Provider from the previous bad injection
3. Add useRouter / usePathname usage inside the App function
4. Ensure children prop is typed properly
"""

with open('src/App.tsx', 'r', encoding='utf-8') as f:
    content = f.read()

# -----------------------------------------------------------------
# Step 1: Find and remove the broken PlatformContext.Provider block
# that was injected *before* AppShell (added by previous Python script)
# It starts with: "      <PlatformContext.Provider value={{\n"
# and ends with "      }}>\n" (then the <AppShell follows)
# -----------------------------------------------------------------

import re

# Remove the big PlatformContext.Provider value block before AppShell 
# (the one that was injected incorrectly before AppShell)
broken_provider_pattern = r"      <PlatformContext\.Provider value=\{\{[\s\S]*?\}\}>\n      <AppShell"
fixed_content = re.sub(broken_provider_pattern, "      <AppShell", content, count=1)

if fixed_content == content:
    print("WARNING: Could not find/remove broken PlatformContext.Provider before AppShell")
else:
    print("Removed broken PlatformContext.Provider before AppShell")
    content = fixed_content

# -----------------------------------------------------------------  
# Step 2: Wrap the return JSX with PlatformContext.Provider
# Find: "  return (\n    <>\n    <div style={{" 
# Add PlatformContext.Provider after the <>
# -----------------------------------------------------------------

# Build the provider value string
provider_value = '''    <PlatformContext.Provider value={{
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
'''

# Find the main return JSX starting point
# It's:  "  return (\n    <>\n    <div style={{"
target = "  return (\n    <>\n    <div style={{"
replacement = "  return (\n    <>\n" + provider_value + "    <div style={{"

if target in content:
    content = content.replace(target, replacement, 1)
    print("Injected PlatformContext.Provider at top of return JSX")
else:
    print("WARNING: Could not find target for PlatformContext.Provider injection")

# -----------------------------------------------------------------
# Step 3: Close PlatformContext.Provider before the closing </>
# Current end:
#   </AppShell>
#   </PlatformContext.Provider>  <- this was the duplicate we removed
#   </div>
#   </>
#
# New end should be:
#   </AppShell>
#   </div>
#   </PlatformContext.Provider>
#   </>
# -----------------------------------------------------------------

# The current (broken) end has </PlatformContext.Provider> right after </AppShell>
# We need to move it to just before </> 

old_end = "      </AppShell>\n      </PlatformContext.Provider>\n    </div>\n    </>\n  );\n}"
new_end = "      </AppShell>\n    </div>\n    </PlatformContext.Provider>\n    </>\n  );\n}"

if old_end in content:
    content = content.replace(old_end, new_end, 1)
    print("Fixed PlatformContext.Provider closing position")
else:
    # Try alternate - maybe whitespace differs
    print("WARNING: Could not find PlatformContext.Provider closing - checking manually")
    # Find end of file
    end_idx = content.rfind('</PlatformContext.Provider>')
    appl_idx = content.rfind('</AppShell>')
    div_idx = content.rfind('</div>', end_idx)
    print(f"  </AppShell> at char {appl_idx}")
    print(f"  </PlatformContext.Provider> at char {end_idx}")
    print(f"  Last </div> after at char {div_idx}")

# -----------------------------------------------------------------
# Step 4: Add useRouter/usePathname usage if missing
# -----------------------------------------------------------------

if 'const router = useRouter()' not in content and 'useRouter' in content:
    # Inject after: "  const [activePage, setActivePage] = React.useState<AppPage>('Play');"
    old_state = "  const [activePage, setActivePage] = React.useState<AppPage>('Play');"
    new_state = """  const [activePage, setActivePage] = React.useState<AppPage>('Play');
  const router = useRouter();
  const pathname = usePathname();
  React.useEffect(() => {
    if (!pathname) return;
    if (pathname === '/play' || pathname === '/') setActivePage('Play');
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
  }, [pathname]);"""
    if old_state in content:
        content = content.replace(old_state, new_state, 1)
        print("Added useRouter/usePathname usage")

# -----------------------------------------------------------------
# Step 5: Fix children prop signature
# -----------------------------------------------------------------
old_sig = "export default function App({ runtimeConfig }: { runtimeConfig?: { matchServiceHttpBase?: string; matchServiceWsBase?: string } })"
new_sig = "export default function App({ runtimeConfig, children }: { runtimeConfig?: { matchServiceHttpBase?: string; matchServiceWsBase?: string }, children?: React.ReactNode })"
if old_sig in content:
    content = content.replace(old_sig, new_sig, 1)
    print("Fixed children prop signature")
elif 'children' in content and 'export default function App(' in content:
    print("children prop already present")

# -----------------------------------------------------------------
# Step 6: Fix children rendering
# Replace "{showPlayHub ? (" with the conditional children version
# -----------------------------------------------------------------
old_render = "      {showPlayHub ? ("
new_render = "      {(children && activePage !== 'Match') ? children : showPlayHub ? ("
if old_render in content and new_render not in content:
    content = content.replace(old_render, new_render, 1)
    print("Fixed children rendering")
elif new_render in content:
    print("Children rendering already fixed")
else:
    print("WARNING: Could not fix children rendering")

with open('src/App.tsx', 'w', encoding='utf-8') as f:
    f.write(content)

print(f"\nDone. Total chars: {len(content)}")
