import assert from 'node:assert/strict';
import { existsSync } from 'node:fs';
import puppeteer from 'puppeteer';

const BASE_URL = process.env.PICKER_URL ?? 'http://localhost:4300';
const WIDTHS = [320, 375, 768, 1280];
const CORS = {
  'access-control-allow-origin': BASE_URL,
  'access-control-allow-credentials': 'true',
  'access-control-allow-headers': 'authorization,content-type',
  'access-control-allow-methods': 'GET,POST,PUT,DELETE,OPTIONS',
  'content-type': 'application/json',
};

const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

function fakeApi(request) {
  const url = new URL(request.url());
  const headers = { ...CORS, origin: BASE_URL };
  if (request.method() === 'OPTIONS') {
    return request.respond({ status: 204, headers });
  }
  if (url.pathname === '/auth/refresh') {
    return request.respond({
      status: 200,
      headers,
      body: JSON.stringify({
        accessToken: 'e2e-access-token',
        refreshToken: 'e2e-refresh-token',
        expiresIn: 3600,
      }),
    });
  }
  if (url.pathname === '/projects/demo/slide-preview') {
    const body = JSON.parse(request.postData() ?? '{}');
    if (!String(body.text ?? '').includes('What is 2+2?')) {
      return request.respond({ status: 400, headers, body: '{}' });
    }
    return request.respond({
      status: 200,
      headers: { ...headers, 'content-type': 'image/png' },
      body: Buffer.from(
        'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=',
        'base64',
      ),
    });
  }
  if (url.pathname === '/shorts/limits') {
    return request.respond({
      status: 200,
      headers,
      body: JSON.stringify({
        maxDurationSeconds: 3600,
        maxDurationMinutes: 60,
        maxUploadBytes: 2147483648,
        maxUploadMb: 2048,
        maxClipCount: 3,
        minClipSeconds: 30,
        maxClipSeconds: 90,
        acceptedFileTypes: ['mp3', 'mp4', 'mov', 'wav'],
      }),
    });
  }
  return request.respond({ status: 404, headers, body: '{}' });
}

async function keyboardFocus(page, formControlName) {
  await page.evaluate(() => document.activeElement?.blur());
  for (let i = 0; i < 80; i++) {
    await page.keyboard.press('Tab');
    const found = await page.evaluate(
      (name) => document.activeElement?.getAttribute('formcontrolname') === name || document.activeElement?.getAttribute('name') === name,
      formControlName,
    );
    if (found) return true;
  }
  return false;
}

const chrome = process.env.CHROME_PATH ??
  ['/usr/bin/google-chrome', '/usr/bin/chromium', '/snap/bin/chromium'].find(existsSync);
const browser = await puppeteer.launch({
  headless: true,
  ...(chrome ? { executablePath: chrome } : {}),
  args: ['--no-sandbox', '--disable-dev-shm-usage'],
});
try {
  const page = await browser.newPage();
  await page.evaluateOnNewDocument(() => localStorage.setItem('indifferent.refreshToken', 'e2e-refresh-token'));
  await page.setRequestInterception(true);
  page.on('request', (request) => {
    const url = new URL(request.url());
    if (url.origin === 'http://localhost:3000') return void fakeApi(request);
    return void request.continue();
  });

  for (const width of WIDTHS) {
    await page.setViewport({ width, height: 900, deviceScaleFactor: 1 });
    await page.goto(`${BASE_URL}/projects/new`, { waitUntil: 'networkidle0' });
    await page.waitForSelector('input[formcontrolname="template"]', { timeout: 20000 });
    await sleep(100);

    const metrics = await page.evaluate(() => ({
      scrollWidth: document.documentElement.scrollWidth,
      innerWidth: window.innerWidth,
      bodyScrollWidth: document.body.scrollWidth,
    }));
    assert.ok(
      metrics.scrollWidth <= metrics.innerWidth + 1,
      `${width}px: page scrolls sideways (${metrics.scrollWidth} > ${metrics.innerWidth})`,
    );
    assert.ok(
      metrics.bodyScrollWidth <= metrics.innerWidth + 1,
      `${width}px: body scrolls sideways (${metrics.bodyScrollWidth} > ${metrics.innerWidth})`,
    );

    const shortTargets = await page.evaluate(() => {
      const targets = [
        ...document.querySelectorAll('fieldset label:has(input), [role="radio"][tabindex="0"]'),
      ];
      return targets
        .map((el) => ({
          label: (el.textContent ?? '').trim().replace(/\s+/g, ' ').slice(0, 50),
          height: el.getBoundingClientRect().height,
        }))
        .filter((t) => t.height < 44);
    });
    assert.deepEqual(shortTargets, [], `${width}px: controls below 44px: ${JSON.stringify(shortTargets)}`);
  }

  await page.setViewport({ width: 1280, height: 900 });
  await page.goto(`${BASE_URL}/projects/new`, { waitUntil: 'networkidle0' });
  await page.waitForSelector('input[formcontrolname="template"]');

  assert.ok(await keyboardFocus(page, 'template'), 'keyboard: cannot tab to the design picker');
  const ring = await page.evaluate(() => {
    const span = document.activeElement?.nextElementSibling;
    return span ? getComputedStyle(span).boxShadow : 'none';
  });
  assert.notEqual(ring, 'none', 'keyboard: focused design has no visible focus ring');

  const before = await page.evaluate(() => document.querySelector('input[formcontrolname="template"]:checked')?.value);
  await page.keyboard.press('ArrowRight');
  await sleep(100);
  const after = await page.evaluate(() => ({
    value: document.querySelector('input[formcontrolname="template"]:checked')?.value,
    selectedCopy: [...document.querySelectorAll('fieldset label')].some((label) =>
      label.textContent?.includes('Selected') && label.querySelector('input:checked'),
    ),
  }));
  assert.notEqual(after.value, before, 'keyboard: ArrowRight did not move the design selection');
  assert.ok(after.selectedCopy, 'keyboard: selected design is not conveyed in text');

  assert.ok(await keyboardFocus(page, 'textSize'), 'keyboard: cannot tab to the text-size picker');
  const textBefore = await page.evaluate(() => document.querySelector('input[formcontrolname="textSize"]:checked')?.value);
  await page.keyboard.press('ArrowRight');
  await sleep(50);
  const textAfter = await page.evaluate(() => document.querySelector('input[formcontrolname="textSize"]:checked')?.value);
  assert.notEqual(textAfter, textBefore, 'keyboard: ArrowRight did not move the text-size selection');

  assert.ok(await keyboardFocus(page, 'showQuestionNumber'), 'keyboard: cannot tab to Show question number');
  const checkedBefore = await page.evaluate(() => document.querySelector('input[formcontrolname="showQuestionNumber"]').checked);
  await page.keyboard.press('Space');
  await sleep(50);
  const checkedAfter = await page.evaluate(() => document.querySelector('input[formcontrolname="showQuestionNumber"]').checked);
  assert.notEqual(checkedAfter, checkedBefore, 'keyboard: Space did not toggle Show question number');

  await page.screenshot({ path: '/tmp/picker-1280.png', fullPage: true });

  await page.goto(`${BASE_URL}/projects/demo/upload`, { waitUntil: 'networkidle0' });
  await page.waitForSelector('input[type="file"]');
  const fileInput = await page.$('input[type="file"]');
  await fileInput.uploadFile('/tmp/picker-sample.txt');
  await page.waitForFunction(() => {
    const button = [...document.querySelectorAll('button')].find((b) =>
      b.textContent?.includes('Preview first question'),
    );
    return button && !button.disabled;
  });
  await page.evaluate(() => {
    [...document.querySelectorAll('button')]
      .find((b) => b.textContent?.includes('Preview first question'))
      ?.click();
  });
  await page.waitForSelector('img[alt="Preview of the first question rendered with this project\'s design"]');
  const preview = await page.evaluate(() => {
    const img = document.querySelector('img[alt^="Preview of the first question"]');
    return { src: img?.getAttribute('src') ?? '', width: img?.naturalWidth ?? 0 };
  });
  assert.ok(preview.src.startsWith('blob:'), 'upload preview did not use the API PNG blob');
  assert.ok(preview.width > 0, 'upload preview image failed to decode');

  console.log(
    `picker verification passed: widths ${WIDTHS.join(', ')}; keyboard selection, 44px targets, focus rings and TXT preview OK`,
  );
} finally {
  await browser.close();
}
