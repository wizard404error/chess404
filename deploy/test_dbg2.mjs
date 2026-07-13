import { chromium } from 'playwright';

async function test(desc, setup) {
  const browser = await chromium.launch({ headless: true });
  const context = await browser.newContext({ viewport: { width: 1280, height: 720 } });
  const page = await context.newPage();
  
  await setup(page);
  
  await page.goto('https://web-production-ddc27.up.railway.app/play', { waitUntil: 'load', timeout: 30000 });
  await page.waitForTimeout(10000);
  
  const loc = page.locator('button').filter({ hasText: 'Join Casual' }).first();
  const found = await loc.count() > 0;
  console.log(desc, found ? 'FOUND' : 'NOT FOUND');
  if (found) console.log('  Text:', await loc.textContent());
  
  await browser.close();
}

// Test 1: No handlers
await test('No handlers', async (page) => {});

// Test 2: Just console handler
await test('Console handler', async (page) => {
  page.on('console', msg => {});
});

// Test 3: Websocket with framesent listener
await test('Websocket + framesent', async (page) => {
  page.on('websocket', ws => { ws.on('framesent', ()=>{}); });
});

// Test 4: Hydration loop polling
await test('Hydration loop', async (page) => {
  for (let i = 0; i < 5; i++) {
    const btns = await page.$$('button');
    if (btns.length > 0) break;
    await page.waitForTimeout(1000);
  }
});

// Test 5: Hydration loop + websocket
await test('Hydration + websocket', async (page) => {
  page.on('websocket', ws => { ws.on('framesent', ()=>{}); });
  for (let i = 0; i < 5; i++) {
    const btns = await page.$$('button');
    if (btns.length > 0) break;
    await page.waitForTimeout(1000);
  }
});
