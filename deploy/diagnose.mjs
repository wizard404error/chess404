import { createRequire } from 'module';
const require = createRequire(import.meta.url);
const { chromium } = require('playwright');

const WEB_URL = 'https://web-production-ddc27.up.railway.app';

async function runPlayer(browser, name, donePromise) {
  const context = await browser.newContext({ viewport: { width: 1280, height: 720 } });
  const page = await context.newPage();
  const errors = [];
  const logs = [];

  page.on('console', msg => {
    const text = msg.text();
    logs.push(`[${msg.type()}] ${text}`);
    if (msg.type() === 'error' || text.includes('500') || text.includes('WebSocket')) {
      console.log(`  [${name}] ${text}`);
    }
  });
  page.on('pageerror', err => { errors.push(err.message); console.log(`  [${name}] PAGE_ERROR: ${err.message}`); });
  page.on('response', resp => {
    if (resp.status() >= 400) console.log(`  [${name}] HTTP ${resp.status()}: ${resp.url().substring(0, 120)}`);
  });
  page.on('websocket', ws => {
    console.log(`\n  [${name}] >>> WebSocket OPEN: ${ws.url()}`);
    ws.on('close', (code, reason) => console.log(`  [${name}] >>> WebSocket CLOSED: code=${code}`));
    ws.on('framesent', frame => console.log(`  [${name}] WS SEND: ${frame.payload?.substring(0, 100)}`));
    ws.on('framereceived', frame => console.log(`  [${name}] WS RECV: ${frame.payload?.substring(0, 100)}`));
  });

  // Load
  await page.goto(`${WEB_URL}/play`, { waitUntil: 'networkidle', timeout: 60000 });
  await page.waitForTimeout(3000);

  // Wait for spinner to disappear
  for (let i = 0; i < 10; i++) {
    const spinner = await page.$('div[style*="animation:loadingSlide"]');
    if (!spinner) break;
    await page.waitForTimeout(1000);
  }

  // Click "Join Casual - Open Cards"
  const joinBtn = await page.$('button:has-text("Join Casual")');
  if (joinBtn && await joinBtn.isVisible()) {
    console.log(`  [${name}] Clicking "Join Casual"`);
    await joinBtn.click();
  } else {
    console.log(`  [${name}] ERROR: Join Casual button not found`);
    return { errors, logs, wsCount: 0 };
  }

  // Wait for either WebSocket or "waiting" state
  let wsCount = 0;
  for (let i = 0; i < 60; i++) {
    if (page.isClosed()) break;
    const body = await page.textContent('body');
    // Check for match found
    if (body.includes('You are') && (body.includes('white') || body.includes('black'))) {
      console.log(`  [${name}] MATCH FOUND!`);
    }
    await page.waitForTimeout(1000);
    wsCount = page._page?.websockets?.length || 0;
    if (i % 10 === 9) {
      const preview = body.substring(0, 200);
      console.log(`  [${name}] State at ${i + 1}s: ${preview.replace(/\n/g, ' ')}`);
    }
  }

  // Wait for done signal
  try {
    await donePromise;
  } catch {}

  await page.close();
  await context.close();
  return { errors, logs, wsCount };
}

async function diagnose() {
  const browser = await chromium.launch({ headless: true });

  // Start both players with a shared promise for coordination
  let resolveDone;
  const donePromise = new Promise(resolve => { resolveDone = resolve; });

  console.log('Starting Player 1 (white)...');
  const p1Promise = runPlayer(browser, 'P1', donePromise);

  await new Promise(r => setTimeout(r, 3000));

  console.log('\nStarting Player 2 (black)...');
  const p2Promise = runPlayer(browser, 'P2', donePromise);

  // Wait for both to complete (60s timeout)
  const results = await Promise.all([
    p1Promise,
    p2Promise,
  ]);

  resolveDone();

  console.log('\n\n========== FINAL RESULTS ==========');
  for (let i = 0; i < results.length; i++) {
    const r = results[i];
    console.log(`\n--- Player ${i + 1} ---`);
    console.log(`WebSocket count: ${r.wsCount}`);
    console.log(`Errors: ${r.errors.length}`);
    for (const e of r.errors) console.log(`  Error: ${e}`);
  }

  await browser.close();
}

diagnose().catch(err => {
  console.error('Diagnose failed:', err);
  process.exit(1);
});
