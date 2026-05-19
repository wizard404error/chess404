import sys

with open('src/App.tsx', 'r', encoding='utf-8') as f:
    lines = f.readlines()

platform_import = "import { PlatformContext } from './contexts/PlatformContext';\n"
router_import = "import { usePathname, useRouter } from 'next/navigation';\n"

new_lines = []
seen_platform_close = False
injected_imports = False

for i, line in enumerate(lines):
    # Inject imports right after React import line
    if not injected_imports and "import React from 'react';" in line:
        new_lines.append(line)
        new_lines.append(platform_import)
        new_lines.append(router_import)
        injected_imports = True
        continue
    # Skip duplicate closing Provider tag
    if '</PlatformContext.Provider>' in line:
        if seen_platform_close:
            print(f'Skipping duplicate closing tag at line {i}')
            continue
        seen_platform_close = True
    new_lines.append(line)

with open('src/App.tsx', 'w', encoding='utf-8') as f:
    f.writelines(new_lines)
print(f'Done. Total lines: {len(new_lines)}')
