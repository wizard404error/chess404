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
    if (!text.includes('AudioContext') && (msg.type() === 'error' || text.includes('500') || text.includes('WebSocket') || text.includes('Failed') || text.includes('[DEBUG]'))) {
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
  await page.goto(`${WEB_URL}/play`, { waitUntil: 'load', timeout: 30000 });
  console.log(`  [${name}] URL: ${page.url()}`);

  // Wait for hydration (buttons in DOM)
  for (let i = 0; i < 20; i++) {
    if (page.isClosed()) { console.log(`  [${name}] Page closed`); return { errors, logs, wsCount: 0 }; }
    const btns = await page.$$('button').catch(() => []);
    if (btns.length > 0) { break; }
    await page.waitForTimeout(1000);
  }

  // Wait for Join Casual button (profile must load first - bootstrap may take a few seconds)
  let joinBtn = null;
  for (let i = 0; i < 60; i++) {
    if (page.isClosed()) break;
    joinBtn = page.locator('button').filter({ hasText: 'Join Casual' }).first();
    if (await joinBtn.count() > 0 && await joinBtn.isVisible()) break;
    await page.waitForTimeout(1000);
  }
  if (!joinBtn || await joinBtn.count() === 0) {
    console.log(`  [${name}] ERROR: Join Casual button never appeared`);
    return { errors, logs, wsCount: 0 };
  }

  console.log(`  [${name}] Clicking "Join Casual"`);
  await joinBtn.click();
  await page.waitForTimeout(2000);
  console.log(`  [${name}] URL: ${page.url()}`);

  // Wait for match to be found (URL changes to /match/ or WebSocket appears)
  let wsCount = 0;
  for (let i = 0; i < 60; i++) {
    if (page.isClosed()) break;
    const url = page.url();
    if (url.includes('/match/')) {
      console.log(`  [${name}] NAVIGATED TO MATCH: ${url}`);
      // Wait for match page to hydrate
      for (let j = 0; j < 20; j++) {
        const btns = await page.$$('button').catch(() => []);
        if (btns.length > 5) { console.log(`  [${name}] Match page hydrated, buttons: ${btns.length}`); break; }
        await page.waitForTimeout(1000);
      }
      break;
    }
    await page.waitForTimeout(1000);
    wsCount = page._page?.websockets?.length || 0;
    if (i % 10 === 9) {
      console.log(`  [${name}] State at ${i + 1}s (URL: ${url.replace('https://web-production-ddc27.up.railway.app', '')})`);
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
  const p1ResultPromise = runPlayer(browser, 'P1', donePromise);

  await new Promise(r => setTimeout(r, 10000));

  console.log('\nStarting Player 2 (black)...');
  const p2ResultPromise = runPlayer(browser, 'P2', donePromise);

  const results = await Promise.all([p1ResultPromise, p2ResultPromise]);

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
