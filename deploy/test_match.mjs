import { chromium } from 'playwright';
const browser = await chromium.launch({ headless: true });
const page = await browser.newPage();

page.on('console', msg => { if (msg.type() === 'error') console.log('  ERROR:', msg.text().substring(0, 200)); });
page.on('pageerror', err => console.log('  PAGE_ERROR:', err.message));
page.on('response', resp => { if (resp.status() >= 400) console.log('  HTTP', resp.status(), resp.url().substring(0, 120)); });

// Navigate to a fake match page to see if it hydrates
await page.goto('https://web-production-ddc27.up.railway.app/match/test123', { waitUntil: 'load', timeout: 30000 });
console.log('  LOADED');

for (let i = 0; i < 20; i++) {
  const btns = await page.$$('button');
  if (btns.length > 0) { console.log('  Hydrated after', i+1, 's, buttons:', btns.length); break; }
  await page.waitForTimeout(1000);
}

const body = await page.textContent('body');
console.log('  Body preview:', body.substring(0, 300).replace(/\n/g, ' '));

await browser.close();
