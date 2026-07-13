import { chromium } from 'playwright';
const browser = await chromium.launch({ headless: true });

// Exact same setup as diagnose.mjs runPlayer
const name = 'TEST';
const context = await browser.newContext({ viewport: { width: 1280, height: 720 } });
const page = await context.newPage();
const errors = [];
const logs = [];

page.on('console', msg => {
  const text = msg.text();
  logs.push(`[${msg.type()}] ${text}`);
  if (!text.includes('AudioContext') && (msg.type() === 'error' || text.includes('500') || text.includes('WebSocket') || text.includes('Failed'))) {
    console.log(`  [${name}] ${text}`);
  }
});
page.on('pageerror', err => { errors.push(err.message); console.log(`  [${name}] PAGE_ERROR: ${err.message}`); });
page.on('response', resp => {
  if (resp.status() >= 400) console.log(`  [${name}] HTTP ${resp.status()}: ${resp.url().substring(0, 120)}`);
});
page.on('websocket', ws => {
  console.log(`\n  [${name}] >>> WebSocket OPEN: ${ws.url()}`);
  ws.on('close', (code) => console.log(`  [${name}] >>> WebSocket CLOSED: code=${code}`));
  ws.on('framesent', frame => console.log(`  [${name}] WS SEND: ${frame.payload?.substring(0, 100)}`));
  ws.on('framereceived', frame => console.log(`  [${name}] WS RECV: ${frame.payload?.substring(0, 100)}`));
});

await page.goto('https://web-production-ddc27.up.railway.app/play', { waitUntil: 'load', timeout: 30000 });

for (let i = 0; i < 20; i++) {
  if (page.isClosed()) break;
  const btns = await page.$$('button').catch(() => []);
  if (btns.length > 0) { console.log(`  [${name}] Hydrated at ${i + 1}s, buttons: ${btns.length}`); break; }
  await page.waitForTimeout(1000);
}

// Use same locator approach as diagnose
const loc = page.locator('button').filter({ hasText: 'Join Casual' }).first();
const foundCount = await loc.count();
console.log(`  [${name}] Found: ${foundCount > 0}`);
if (foundCount > 0) {
  console.log(`  [${name}] Text: "${await loc.textContent()}"`);
} else {
  // Dump all buttons
  const allBtns = await page.$$('button');
  const texts = await Promise.all(allBtns.slice(0, 20).map(b => b.textContent()));
  console.log(`  [${name}] No Join Casual. First 20 buttons: ${texts.map(t => `"${t?.substring(0,30)}"`).join(', ')}`);
  // Try case-insensitive search
  for (const btn of allBtns) {
    const t = await btn.textContent();
    if (t && t.toLowerCase().includes('join') && t.toLowerCase().includes('casual')) {
      console.log(`  [${name}] Found by JS: "${t}"`);
    }
  }
}

await browser.close();
