import { chromium } from 'playwright';
const browser = await chromium.launch({ headless: true });
const context = await browser.newContext({ viewport: { width: 1280, height: 720 } });
const page = await context.newPage();

page.on('console', msg => {});
page.on('pageerror', err => {});
page.on('response', resp => {});
page.on('websocket', ws => { ws.on('framesent', ()=>{}); });

await page.goto('https://web-production-ddc27.up.railway.app/play', { waitUntil: 'load', timeout: 30000 });

// Hydration loop like diagnose
for (let i = 0; i < 10; i++) {
  const btns = await page.$$('button');
  if (btns.length > 0) { console.log('Hydrated at', i+1, 's, buttons:', btns.length); break; }
  await page.waitForTimeout(1000);
}

const loc = page.locator('button').filter({ hasText: 'Join Casual' }).first();
console.log('Found:', await loc.count() > 0);
if (await loc.count() > 0) console.log('Text:', await loc.textContent());

// Also check Play button
const playLoc = page.locator('button').filter({ hasText: 'Play' }).first();
console.log('Play button found:', await playLoc.count() > 0);

// Check visible/hidden buttons
const allVisible = page.locator('button:visible');
console.log('Visible buttons:', await allVisible.count());

await browser.close();
