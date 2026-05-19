"""
Targeted fix: The board surface outer <div> wrapping everything is missing its closing tag.
Structure should be:
  ) : showBoardSurface ? (
    <div style={{...flex...}}>        <- opens here (line 6187)
      {/* Left col */}
      <div>...</div>
      {/* Board col */}
      <div>...</div>
      {/* Right panel */}
      <div>
        <GamePanel ... />
        {/* ELO Stakes */} ...
        {/* Game Controls */} ...
      </div>
    </div>                            <- needs to close here
  ) : null}
"""

with open('src/App.tsx', 'r', encoding='utf-8') as f:
    text = f.read()

# The problem: the } that closes the showBoardSurface ternary reads:
#   {/* ELO Stakes */}
#   ...
#       ) : null}
# But there are several unclosed divs because the right-panel closing div
# from the old code was removed by the GamePanel patch.
#
# The new right panel structure (from GamePanel injection) ends with:
#   />
#   {/* ELO Stakes */}
# (no closing </div> for the right panel wrapper div)
# Then immediately: ) : null}
#
# Fix: after the GamePanel closing /> and ELO Stakes block, ensure we have:
#   </div>   <- closes the right panel column div
#   </div>   <- closes the outer showBoardSurface flex div
# before ) : null}

# Step 1: Find the right panel div opening
# It opens with: <div style={{ width:'340px' ... (or similar)
# We need to find where this column starts
# From the original code, right panel starts with:
# {/* ── Right panel ── */}
# <div style={{ width:'340px'...

RIGHT_PANEL_COMMENT = "{/* ── Right panel ── */}"
rp_pos = text.find(RIGHT_PANEL_COMMENT)
if rp_pos != -1:
    print(f"Found right panel marker")
    # Right panel div opens right after this comment
    rp_div_start = text.find("<div style=", rp_pos)
    rp_div_end = text.find(">", rp_div_start) + 1
    print(f"Right panel div: {repr(text[rp_div_start:rp_div_end][:80])}")
else:
    print("WARNING: Could not find right panel comment")

# Step 2: Find the end of the showBoardSurface block
# It ends with ) : null} somewhere before </AppShell>
BS_END = "      ) : null}\n      </AppShell>"
bs_pos = text.find(BS_END)
if bs_pos == -1:
    # Try alternate indentation
    BS_END = "      ) : null}\n      </AppShell>"
    bs_pos = text.rfind(") : null}")
    if bs_pos != -1:
        # Find the actual ) : null} before </AppShell>
        appshell_close = text.find("</AppShell>", bs_pos)
        print(f"Found ) : null}} at char {bs_pos}, </AppShell> at char {appshell_close}")
        bs_end_line = text[max(0, bs_pos-100):bs_pos+20]
        print(f"Context: {repr(bs_end_line)}")

# Step 3: The actual fix - find the GamePanel closing /> followed by ELO Stakes
# and ensure there are proper closing divs before ) : null}

GAMEPANEL_END_MARKER = "          />\n          {/* ELO Stakes */}"
gp_end = text.find(GAMEPANEL_END_MARKER)
if gp_end == -1:
    # Try with different spacing
    GAMEPANEL_END_MARKER = "/>\n          {/* ELO Stakes */}"
    gp_end = text.rfind(GAMEPANEL_END_MARKER)

if gp_end != -1:
    print(f"Found GamePanel end marker at char {gp_end}")
    # Show context around it
    ctx = text[gp_end-100:gp_end+200]
    print(f"Context: {repr(ctx[:300])}")
else:
    print("WARNING: Could not find GamePanel end marker")

# Step 4: Find what comes between GamePanel close and ) : null}
# We need to count what divs are open
null_pos = text.rfind(") : null}")
print(f"\n) : null}} at char {null_pos}")
context_before_null = text[null_pos-500:null_pos+10]
print(f"\nContext before ) : null}}:")
for i, line in enumerate(context_before_null.split('\n')):
    print(f"  {i}: {repr(line)}")
