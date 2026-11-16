// Public UI regression against a disposable, pre-seeded loopback candidate.
// No credentials are read and every browser request is restricted to same-origin GET.
// PAINTER_BROWSER_BASE_URL defaults to http://127.0.0.1:18082.
// PAINTER_MOMENT_FIXTURES points to prepare-fixtures.py's public fixture index.
// Set PLAYWRIGHT_MODULE, CHROMIUM_EXECUTABLE and ARTIFACT_DIR as needed.
// PAINTER_MOMENT_SCENARIOS optionally selects visual,history,touch,failures,persisted,pagination,author.
// Photographs must use the existing CC0 fixture; other images are labelled diagnostics.
const fs = require('node:fs');
const path = require('node:path');
const assert = require('node:assert/strict');
const {chromium} = require(process.env.PLAYWRIGHT_MODULE || '/tmp/painter-browser/node_modules/playwright');

const base = new URL(process.env.PAINTER_BROWSER_BASE_URL || 'http://127.0.0.1:18082');
assert(['localhost', '127.0.0.1', '[::1]'].includes(base.hostname), 'Use an explicitly disposable loopback candidate');
assert(!base.username && !base.password && base.pathname === '/' && !base.search && !base.hash, 'Base URL must be an origin');
const origin = base.origin;
const fixturePath = process.env.PAINTER_MOMENT_FIXTURES || '/tmp/painter-moments-qa-20261117/fixtures.json';
const fixture = JSON.parse(fs.readFileSync(fixturePath, 'utf8'));
assert(fixture.only_disposable_database === true && new URL(fixture.origin).origin === origin, 'Fixture index must identify this disposable candidate');
assert(Array.isArray(fixture.moments) && fixture.moments.length > 20, 'Prepare more than one page of public moments first');
const out = path.resolve(process.env.ARTIFACT_DIR || 'tests/artifacts/moments-reader');
fs.mkdirSync(out, {recursive: true});
const report = {origin, fixture_index: fixturePath, production_writes: 0, same_origin_get_only: true,
  simulated_failures: [], checks: [], errors: [], blocked: [], screenshots: [], scenarios: []};
let browser, closingBrowser = false;
const publicIds = new Set(fixture.moments.map(item => String(item.id)));
const item = (name, author = 'author') => {
  const value = fixture.moments.find(value => value.case === name && value.author_kind === author);
  assert(value, `Missing fixture case ${author}/${name}`);
  return value;
};
function check(scenario, name, passed, detail) {
  report.checks.push({scenario, name, passed: !!passed, ...(detail === undefined ? {} : {detail})});
  assert.ok(passed, `${scenario}: ${name}${detail === undefined ? '' : ' ' + JSON.stringify(detail)}`);
}
function deferred() {
  let resolve;
  const promise = new Promise(value => { resolve = value; });
  return {promise, resolve};
}
async function deadline(promise, label) {
  let timeout;
  try { return await Promise.race([promise, new Promise((_, reject) => { timeout = setTimeout(() => reject(new Error(label)), 12000); })]); }
  finally { clearTimeout(timeout); }
}
const frames = page => page.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))));
async function dispose(s) {
  // Cleanup failures are reported and fail the final check. They must not mask
  // the original navigation/assertion error thrown inside the scenario.
  try { await s.context.close(); }
  catch (error) { report.errors.push({scenario: s.name, message: 'Browser context cleanup failed: ' + error.message}); }
}
async function setup(name, {width = 1440, height = width < 721 ? 844 : 1000, lang = 'zh-CN', theme = 'light', js = true, handler} = {}) {
  const context = await browser.newContext({viewport: {width, height}, locale: lang === 'en' ? 'en-US' : 'zh-CN',
    colorScheme: theme, reducedMotion: 'reduce', isMobile: width < 721, hasTouch: width < 721,
    javaScriptEnabled: js, serviceWorkers: 'block'});
  if (js) await context.addInitScript(value => localStorage.setItem('painter-theme', value), theme);
  const requests = [];
  await context.route('**/*', async route => {
    const request = route.request(), url = new URL(request.url());
    if (request.method() !== 'GET' || url.origin !== origin) {
      report.blocked.push({scenario: name, method: request.method(), origin: url.origin, path: url.pathname});
      return route.abort();
    }
    requests.push({path: url.pathname, query: url.search, navigation: request.isNavigationRequest()});
    if (handler && await handler(route, url, request)) return;
    return route.continue();
  });
  const page = await context.newPage();
  page.setDefaultTimeout(12000);
  page.setDefaultNavigationTimeout(20000);
  page.on('pageerror', error => report.errors.push({scenario: name, message: error.message}));
  page.on('crash', () => report.errors.push({scenario: name, message: 'Chromium renderer crashed'}));
  const navigate = async value => {
    const url = new URL(value, origin);
    assert.equal(url.origin, origin);
    if (!url.searchParams.has('lang')) url.searchParams.set('lang', lang);
    const response = await page.goto(url.href, {waitUntil: 'domcontentloaded'});
    check(name, 'public route responds successfully', response.status() === 200, {path: url.pathname});
    await page.evaluate(() => document.fonts.ready);
    if (js && url.pathname === '/moments') await page.locator('.moment-viewer').waitFor({state: 'attached'});
    return response;
  };
  const scenario = {name, page, context, requests, navigate, lang, width, js};
  report.scenarios.push({name, width, height, lang, theme, javascript: js});
  return scenario;
}
async function capture(s, name) {
  const file = path.join(out, name + '.png');
  await s.page.waitForFunction(() => Array.from(document.images).every(image => {
    const rect = image.getBoundingClientRect();
    return rect.bottom <= 0 || rect.top >= innerHeight || rect.right <= 0 || rect.left >= innerWidth || image.complete;
  }));
  await s.page.screenshot({path: file, animations: 'disabled'});
  report.screenshots.push(file);
}
async function safeContent(s) {
  const content = await s.page.locator('main').innerText();
  check(s.name, 'unpublished text and image edit never enter the public page',
    fixture.private_markers.every(marker => !content.includes(marker)) &&
    !(await s.page.locator('main').innerHTML()).includes(fixture.snapshot_case.private_image_id));
  check(s.name, 'literal HTML remains text', await s.page.evaluate(() => !window.__momentsQAInjected && !document.querySelector('script[data-moments-qa-xss]')));
}
async function noOverflow(s) {
  const sizes = await s.page.evaluate(() => ({viewport: innerWidth, document: document.documentElement.scrollWidth}));
  check(s.name, 'no document horizontal overflow', sizes.document <= sizes.viewport + 1, sizes);
}
async function feedState(s) {
  return s.page.locator('.moment-card').evaluateAll(cards => ({count: cards.length, ids: cards.map(card => card.id),
    columns: getComputedStyle(cards[0]?.parentElement || document.body).gridTemplateColumns.split(/\s+/).length}));
}
async function open(s, value) {
  const link = s.page.locator(`a[data-moment-open][href="${value.url}"]`).first();
  await link.scrollIntoViewIfNeeded();
  await link.click();
  await s.page.waitForURL(url => url.pathname === value.url);
  await s.page.locator(`.moment-viewer[open] [data-moment-reader][data-moment-id="${value.id}"]`).waitFor();
  await frames(s.page);
  return link;
}
async function close(s, method = 'escape') {
  if (method === 'back') await s.page.goBack();
  else if (method === 'button') await s.page.locator('.moment-viewer-close').click();
  else await s.page.keyboard.press('Escape');
  await s.page.waitForFunction(() => !document.querySelector('.moment-viewer')?.open);
  await frames(s.page);
}
async function lookAndRead() {
  for (const width of [1440, 390, 320]) for (const theme of ['light', 'dark']) {
    const lang = theme === 'dark' ? 'en' : 'zh-CN';
    const s = await setup(`visual-${width}-${theme}`, {width, lang, theme});
    try {
      await s.navigate('/moments');
      await noOverflow(s); await safeContent(s);
      const state = await feedState(s);
      check(s.name, 'appropriate column count', width < 721 ? state.columns === 2 : state.columns >= 3, state);
      if (width < 721) check(s.name, 'phone feed has its own 12px margins',
        Math.abs((await s.page.locator('.moments-items').boundingBox()).x - 12) <= 1);
      check(s.name, 'feed has no extra visible page heading or layout toolbar', await s.page.locator('.browse-heading,.moments-toolbar,.moments-layout').count() === 0);
      check(s.name, 'cards retain native detail links and actual fixture identities', await s.page.locator('[data-moment-open]').evaluateAll((links, ids) =>
        links.every(link => ids.includes(link.getAttribute('href')?.split('/').pop())), [...publicIds]));
      check(s.name, 'UI uses the requested locale', await s.page.locator('html').getAttribute('lang') === lang);
      await capture(s, `feed-${width}-${theme}`);
      if (width > 720) {
        await open(s, item('three'));
        if (lang === 'en') check(s.name, 'reader controls are translated independently of user content',
          await s.page.locator('.moment-viewer').getAttribute('aria-label') === 'Update details' &&
          await s.page.locator('.moment-viewer-close').getAttribute('aria-label') === 'Close update');
        const rects = await s.page.locator('.moment-viewer [data-moment-reader]').evaluate(reader => {
          const media = reader.querySelector('.moment-reader-media').getBoundingClientRect();
          const text = reader.querySelector('.moment-reader-content').getBoundingClientRect();
          return {media: media.toJSON(), text: text.toJSON()};
        });
        check(s.name, 'desktop reader separates image left and text right', rects.media.right <= rects.text.left + 2, rects);
        check(s.name, 'reader contains complete public text', await s.page.locator('.moment-viewer .moment-reader-content .moment-text').innerText() === item('three').text);
        await capture(s, `reader-image-${width}-${theme}`);
        await close(s);
        await open(s, item('long-text'));
        check(s.name, 'text-only reader has no empty image pane', await s.page.locator('.moment-viewer .moment-reader-media').count() === 0);
        check(s.name, 'full long text is retained', await s.page.locator('.moment-viewer .moment-reader-content .moment-text').innerText() === item('long-text').text);
        check(s.name, 'text-only reading column has a comfortable line length',
          (await s.page.locator('.moment-viewer .moment-reader-content').boundingBox()).width <= 680);
        await capture(s, `reader-text-${width}-${theme}`);
        await close(s);
      } else {
        const value = item('three');
        await s.page.locator(`[data-moment-open][href="${value.url}"]`).tap();
        await s.page.waitForURL(url => url.pathname === value.url);
        await s.page.locator('[data-moment-reader]').waitFor();
        check(s.name, 'phone opens a real standalone detail page', await s.page.locator('body').getAttribute('data-page-kind') === 'moment' && await s.page.locator('.moment-viewer[open]').count() === 0);
        check(s.name, 'phone has a direct return affordance', await s.page.locator('[data-moment-back]').isVisible());
        await noOverflow(s);
        await capture(s, `reader-phone-${width}-${theme}`);
        check(s.name, 'icon-only phone return has a localized accessible name',
          await s.page.getByRole('link', {name: lang === 'en' ? 'All updates' : '所有动态', exact: true}).count() === 1);
        const geometry = await s.page.evaluate(() => {
          const media = document.querySelector('.moment-reader-media').getBoundingClientRect();
          const center = selector => { const rect = document.querySelector(selector).getBoundingClientRect(); return rect.y + rect.height / 2; };
          return {viewport: innerWidth, media: media.toJSON(), centers: [center('[data-moment-back]'), center('.moment-reader-author .moment-author'), center('.moment-reader-author .moment-share')]};
        });
        check(s.name, 'phone image area spans the viewport', Math.abs(geometry.media.left) <= 1 && Math.abs(geometry.media.right - geometry.viewport) <= 1, geometry);
        check(s.name, 'phone back, author and share are visually aligned', Math.max(...geometry.centers) - Math.min(...geometry.centers) <= 2, geometry.centers);
        await s.page.goBack();
        await s.page.waitForURL(url => url.pathname === '/moments');
        check(s.name, 'Back returns to the mobile feed', await s.page.locator('[data-moment-feed]').count() === 1);
      }
    } finally { await dispose(s); }
  }
}
async function historyAndLayers() {
  const s = await setup('desktop-history-keyboard');
  try {
    await s.navigate('/moments');
    const value = item('nine');
    const link = s.page.locator(`[data-moment-open][href="${value.url}"]`);
    await link.scrollIntoViewIfNeeded(); await frames(s.page);
    const before = {scroll: await s.page.evaluate(() => scrollY), list: await feedState(s), title: await s.page.title(), canonical: await s.page.locator('link[rel=canonical]').getAttribute('href')};
    await link.focus(); await s.page.keyboard.press('Enter');
    await s.page.locator(`.moment-viewer[open] [data-moment-id="${value.id}"]`).waitFor();
    check(s.name, 'Enter opens the native card enhancement', new URL(s.page.url()).pathname === value.url);
    check(s.name, 'dialog locks background scrolling', await s.page.evaluate(() => getComputedStyle(document.documentElement).overflowY === 'hidden'));
    for (let count = 0; count < 12; count++) {
      await s.page.keyboard.press('Tab');
      check(s.name, `Tab ${count + 1} stays in the active reader`, await s.page.evaluate(() => document.querySelector('.moment-viewer').contains(document.activeElement)));
    }
    const gallery = s.page.locator('.moment-viewer .moment-slides');
    await gallery.locator('a').first().focus();
    // Tabbing through real image links may scroll the gallery natively.
    // Establish a known settled starting slide before testing relative arrows.
    await frames(s.page);
    await s.page.keyboard.press('Home');
    await s.page.waitForFunction(() => document.querySelector('.moment-viewer [data-moment-position]')?.textContent === '1 / 9');
    await s.page.keyboard.press('ArrowRight');
    await s.page.waitForFunction(() => document.querySelector('.moment-viewer [data-moment-position]')?.textContent === '2 / 9');
    check(s.name, 'keyboard changes the ordered image', await s.page.locator('.moment-viewer [data-moment-position]').innerText() === '2 / 9');
    await s.page.keyboard.press('End');
    await s.page.waitForFunction(() => document.querySelector('.moment-viewer [data-moment-position]')?.textContent === '9 / 9');
    check(s.name, 'last image disables next', await s.page.locator('.moment-viewer [data-moment-next]').isDisabled());
    await s.page.keyboard.press('Home');
    await s.page.waitForFunction(() => document.querySelector('.moment-viewer [data-moment-position]')?.textContent === '1 / 9');
    check(s.name, 'first image disables previous', await s.page.locator('.moment-viewer [data-moment-previous]').isDisabled());
    await close(s);
    const after = {scroll: await s.page.evaluate(() => scrollY), list: await feedState(s)};
    check(s.name, 'Escape restores opener focus', await link.evaluate(link => document.activeElement === link));
    // An already in-flight next page may finish while the dialog is open.
    // Closing must preserve every original card and its order, not forbid additions.
    check(s.name, 'Escape preserves the existing list and ordering',
      before.list.ids.every((id, index) => after.list.ids[index] === id), {before: before.list.count, after: after.list.count});
    check(s.name, 'Escape restores reading position', Math.abs(before.scroll - after.scroll) < 24, {before: before.scroll, after: after.scroll});
    check(s.name, 'list title and canonical recover after closing', await s.page.title() === before.title && await s.page.locator('link[rel=canonical]').getAttribute('href') === before.canonical);
    await s.page.goForward();
    await s.page.locator(`.moment-viewer[open] [data-moment-id="${value.id}"]`).waitFor();
    check(s.name, 'Forward reopens a freshly loaded reader', await s.page.locator('.moment-viewer[open]').count() === 1);
    await s.page.locator('.moment-viewer .moment-share').click();
    await s.page.locator('.sharing-dialog[open]').waitFor();
    await close(s, 'back');
    check(s.name, 'Back closes reader and nested sharing together', await s.page.locator('.sharing-dialog[open],.moment-viewer[open]').count() === 0);
    await s.page.goForward();
    await s.page.locator(`.moment-viewer[open] [data-moment-id="${value.id}"]`).waitFor();
    await s.page.locator('.moment-viewer .moment-photo').first().click();
    await s.page.locator('.moment-lightbox[open]').waitFor();
    await close(s, 'back');
    check(s.name, 'Back closes reader and nested image viewer together', await s.page.locator('.moment-lightbox[open],.moment-viewer[open]').count() === 0);
    const opened = s.context.waitForEvent('page');
    await link.click({modifiers: ['Control']});
    const nativePage = await opened;
    try {
      await nativePage.waitForLoadState('domcontentloaded');
      await nativePage.locator('[data-moment-reader]').waitFor();
      check(s.name, 'modified card click retains a native independent tab', new URL(nativePage.url()).pathname === value.url && await nativePage.locator('body').getAttribute('data-page-kind') === 'moment');
    } finally { await nativePage.close(); }
  } finally { await dispose(s); }
}
async function mobileTouch() {
  const s = await setup('phone-native-touch', {width: 390});
  try {
    await s.navigate('/moments');
    const value = item('nine');
    await s.page.locator(`[data-moment-open][href="${value.url}"]`).scrollIntoViewIfNeeded();
    const scroll = await s.page.evaluate(() => scrollY);
    await s.page.locator(`[data-moment-open][href="${value.url}"]`).tap();
    await s.page.waitForURL(url => url.pathname === value.url);
    await s.page.locator('[data-moment-next]:not([hidden])').waitFor();
    const box = await s.page.locator('.moment-slides').boundingBox();
    const cdp = await s.context.newCDPSession(s.page);
    try {
      const start = {x: box.x + box.width * .85, y: box.y + Math.min(box.height / 2, 220)};
      await cdp.send('Input.dispatchTouchEvent', {type: 'touchStart', touchPoints: [start]});
      for (let index = 1; index <= 9; index++) {
        await cdp.send('Input.dispatchTouchEvent', {type: 'touchMove', touchPoints: [{x: start.x - box.width * .7 * index / 9, y: start.y}]});
        await s.page.waitForTimeout(18);
      }
      await cdp.send('Input.dispatchTouchEvent', {type: 'touchEnd', touchPoints: []});
      await s.page.waitForFunction(() => (document.querySelector('[data-moment-position]')?.textContent || '').trim() !== '1 / 9');
      check(s.name, 'real Chromium touch swipe advances the image', await s.page.locator('[data-moment-position]').innerText() !== '1 / 9');
      await s.page.locator('[data-moment-next]').tap();
      check(s.name, 'touch controls retain a 44px target', (await s.page.locator('[data-moment-next]').boundingBox()).height >= 44);
    } finally { await cdp.detach(); }
    await s.page.locator('[data-moment-back]').tap();
    await s.page.waitForURL(url => url.pathname === '/moments');
    check(s.name, 'native return keeps feed position', Math.abs(await s.page.evaluate(() => scrollY) - scroll) < 32);
  } finally { await dispose(s); }
}
async function failuresAndLateResponses() {
  let mode = '', failed = false;
  const captured = deferred(), release = deferred(), delivered = deferred();
  const target = item('one');
  const s = await setup('reader-failure-recovery', {handler: async (route, url) => {
    if (url.pathname !== target.url) return false;
    if (mode === '503' && !failed) {
      failed = true; report.simulated_failures.push('detail503');
      await route.fulfill({status: 503, contentType: 'text/html', body: 'Diagnostic temporary failure'}); return true;
    }
    if (mode === '404') {
      report.simulated_failures.push('detail404-withdrawn');
      await route.fulfill({status: 404, contentType: 'text/html', body: 'Diagnostic withdrawn public snapshot'}); return true;
    }
    if (mode === 'late') {
      const response = await route.fetch({maxRedirects: 0});
      captured.resolve(); await release.promise;
      try { await route.fulfill({response}); } catch { /* The reader intentionally aborts a dismissed request. */ }
      delivered.resolve(); return true;
    }
    return false;
  }});
  try {
    await s.navigate('/moments');
    mode = '503';
    await s.page.locator(`[data-moment-open][href="${target.url}"]`).click();
    const retry = s.page.locator('.moment-viewer-status button');
    await retry.waitFor();
    check(s.name, 'temporary failure does not display an old snapshot', await s.page.locator('.moment-viewer [data-moment-reader]').count() === 0);
    await retry.click();
    await s.page.locator(`.moment-viewer [data-moment-id="${target.id}"]`).waitFor();
    check(s.name, 'retry displays the actual public record', await s.page.locator('.moment-viewer .moment-text').innerText() === target.text);
    await close(s);
    mode = '404';
    await s.page.locator(`[data-moment-open][href="${target.url}"]`).click();
    await s.page.locator('.moment-viewer-status').filter({hasText: '撤回'}).waitFor();
    check(s.name, 'reopening rechecks visibility and removes cached content', await s.page.locator('.moment-viewer [data-moment-reader]').count() === 0 && await s.page.locator('.moment-viewer-status button').count() === 0);
    check(s.name, 'withdrawn record retains a native independent URL', await s.page.locator('.moment-viewer-status a').getAttribute('href') === target.url + '?lang=zh-CN');
    await capture(s, 'reader-withdrawn-diagnostic');
    await close(s);
    mode = 'late';
    await s.page.locator(`[data-moment-open][href="${target.url}"]`).click();
    await deadline(captured.promise, 'Delayed public detail request was not captured');
    await close(s);
    const other = item('text-only');
    await open(s, other);
    release.resolve(); await deadline(delivered.promise, 'Delayed public detail response was not released'); await frames(s.page);
    check(s.name, 'late response cannot replace the new reader or reopen a closed one', await s.page.locator('.moment-viewer [data-moment-reader]').getAttribute('data-moment-id') === String(other.id) && new URL(s.page.url()).pathname === other.url);
    await close(s);
  } finally { release.resolve(); await dispose(s); }
}
async function persistedRevalidation() {
  let withdrawn = false;
  const target = item('one');
  const s = await setup('persisted-lifecycle-revalidation', {handler: async (route, url) => {
    if (!withdrawn || url.pathname !== target.url) return false;
    await route.fulfill({status: 404, contentType: 'text/html', body: 'Diagnostic withdrawn after cached navigation'}); return true;
  }});
  try {
    await s.navigate('/moments');
    await open(s, target);
    withdrawn = true;
    report.simulated_failures.push('persisted-pageshow404');
    // Explicitly simulated lifecycle: Playwright disables BFCache by default.
    // This exercises the persisted handler, without claiming a physical cache return.
    await s.page.evaluate(() => dispatchEvent(new PageTransitionEvent('pageshow', {persisted: true})));
    await s.page.locator('.moment-viewer-status').filter({hasText: '撤回'}).waitFor();
    check(s.name, 'persisted return rechecks public visibility instead of retaining a withdrawn snapshot',
      await s.page.locator('.moment-viewer [data-moment-reader]').count() === 0 && new URL(s.page.url()).pathname === target.url);
    await close(s);
  } finally { await dispose(s); }
}
async function pagination() {
  const s = await setup('pagination-after-reader-close', {height: 600});
  try {
    await s.navigate('/moments');
    const first = await feedState(s);
    check(s.name, 'fixture has a real next page', first.count < fixture.checks.public_count && await s.page.locator('[data-infinite-next]').count() === 1);
    const id = await s.page.locator('[data-moment-open]').first().getAttribute('href');
    await open(s, fixture.moments.find(value => value.url === id));
    const cursorRequests = () => s.requests.filter(request => request.path === '/moments' && new URLSearchParams(request.query).has('cursor')).length;
    const before = cursorRequests();
    await s.page.setViewportSize({width: 1440, height: 1700}); await frames(s.page);
    check(s.name, 'list pagination pauses while the reader owns a detail URL', cursorRequests() === before);
    await close(s);
    await s.page.waitForFunction(count => document.querySelectorAll('.moment-card').length === count, fixture.checks.public_count);
    const final = await feedState(s);
    check(s.name, 'closing resumes the original list cursor without dropping content', final.count === fixture.checks.public_count && new Set(final.ids).size === final.count, final);
    const value = item('one', 'owner');
    await open(s, value);
    check(s.name, 'appended card opens and preserves published images after a private edit', !(await s.page.locator('.moment-viewer').innerHTML()).includes(fixture.snapshot_case.private_image_id) && await s.page.locator('.moment-viewer .moment-text').innerText() === value.text);
    await close(s);
  } finally { await dispose(s); }
  let fail = true;
  const f = await setup('pagination-failure-retry', {height: 600, handler: async (route, url) => {
    if (url.pathname !== '/moments' || !url.searchParams.has('cursor') || !fail) return false;
    fail = false; report.simulated_failures.push('pagination503');
    await route.fulfill({status: 503, contentType: 'text/html', body: 'Diagnostic pagination failure'}); return true;
  }});
  try {
    await f.navigate('/moments');
    const count = (await feedState(f)).count;
    await f.page.locator('[data-infinite-next]').scrollIntoViewIfNeeded();
    const retry = f.page.locator('[data-infinite-next] button'); await retry.waitFor();
    check(f.name, 'failed next page preserves already rendered cards', (await feedState(f)).count === count);
    await retry.click();
    await f.page.waitForFunction(count => document.querySelectorAll('.moment-card').length === count, fixture.checks.public_count);
    const final = await feedState(f);
    check(f.name, 'explicit retry appends unique real cards', final.count === fixture.checks.public_count && new Set(final.ids).size === final.count);
  } finally { await dispose(f); }
}
async function authorsAndNativeLinks() {
  const s = await setup('author-reuse-and-private-scope');
  try {
    await s.navigate(fixture.author.profile_url);
    await s.page.locator('.moment-viewer').waitFor({state: 'attached'});
    const authors = await s.page.locator('.author-moments .moment-author').evaluateAll(links => links.map(link => link.getAttribute('href')));
    check(s.name, 'author feed reuses cards while remaining scoped to its creator', authors.length === fixture.checks.per_creator && authors.every(href => href === '/users/' + fixture.author.id));
    await safeContent(s); await noOverflow(s);
    await open(s, item('three'));
    await close(s, 'button');
    check(s.name, 'closing from author feed restores that profile and tab', new URL(s.page.url()).pathname === '/users/' + fixture.author.id && new URL(s.page.url()).searchParams.get('tab') === 'moments');
    for (const person of [fixture.owner, fixture.author]) {
      const response = await s.page.goto(origin + '/moments/' + person.draft_id, {waitUntil: 'domcontentloaded'});
      check(s.name, 'unpublished native detail stays unavailable', response.status() === 404 && await s.page.locator('[data-moment-reader]').count() === 0);
    }
    await s.navigate(item('literal-html').url);
    await safeContent(s);
  } finally { await dispose(s); }
  const n = await setup('no-javascript-native-links', {width: 390, js: false});
  try {
    await n.navigate('/moments');
    const value = item('three');
    await n.page.locator(`[data-moment-open][href="${value.url}"]`).click();
    await n.page.waitForURL(url => url.pathname === value.url);
    check(n.name, 'no-JS card uses the server-rendered detail URL', await n.page.locator('[data-moment-reader]').getAttribute('data-moment-id') === String(value.id));
    check(n.name, 'all original images and text remain in native HTML', await n.page.locator('.moment-photo').count() === value.image_count && await n.page.locator('.moment-reader-content .moment-text').innerText() === value.text);
    await n.page.locator('[data-moment-back]').click();
    await n.page.waitForURL(url => url.pathname === '/moments');
    check(n.name, 'no-JS return remains a valid list link', await n.page.locator('.moment-card').count() > 0);
  } finally { await dispose(n); }
}

(async () => {
  const scenarios = {visual: lookAndRead, history: historyAndLayers, touch: mobileTouch,
    failures: failuresAndLateResponses, persisted: persistedRevalidation, pagination, author: authorsAndNativeLinks};
  const selected = (process.env.PAINTER_MOMENT_SCENARIOS || Object.keys(scenarios).join(',')).split(',');
  assert(selected.length && selected.every(name => Object.hasOwn(scenarios, name)), 'Unknown moment scenario');
  browser = await chromium.launch({executablePath: process.env.CHROMIUM_EXECUTABLE || '/usr/bin/google-chrome', headless: true, args: ['--no-sandbox']});
  browser.on('disconnected', () => { if (!closingBrowser) report.errors.push({scenario: 'browser', message: 'Chromium disconnected unexpectedly'}); });
  try {
    for (const name of selected) await scenarios[name]();
  } finally {
    closingBrowser = true;
    try { await browser.close(); }
    catch (error) { report.errors.push({scenario: 'browser', message: 'Browser cleanup failed: ' + error.message}); }
  }
  check('complete', 'no application or browser runtime errors', report.errors.length === 0, report.errors);
  report.passed = true;
  fs.writeFileSync(path.join(out, 'report.json'), JSON.stringify(report, null, 2) + '\n');
  console.log(JSON.stringify({passed: true, checks: report.checks.length, screenshots: report.screenshots.length, actual_business_writes: 0}));
})().catch(error => {
  report.passed = false; report.failure = error.stack || String(error);
  fs.writeFileSync(path.join(out, 'report.json'), JSON.stringify(report, null, 2) + '\n');
  console.error(String(error)); process.exitCode = 1;
});
