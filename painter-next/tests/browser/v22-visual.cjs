// V22 isolated candidate visual acceptance. Does not write application data.
const fs = require('fs');
const path = require('path');
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || '/tmp/painter-browser/node_modules/playwright');
const origin = process.env.BASE_URL || 'http://192.168.2.209:18081';
const parsed = new URL(origin);
if (parsed.port !== '18081' || !['192.168.2.209', '127.0.0.1', 'localhost'].includes(parsed.hostname)) throw new Error('Visual review is restricted to candidate port 18081');
if (process.env.V22_DATA_STABLE !== '1') throw new Error('Wait for root to confirm data stability, then set V22_DATA_STABLE=1');
const out = path.resolve('tests/artifacts/v22/visual');
const ownerState = '/tmp/painter-next-v16-owner-state.json';
const fixture = JSON.parse(fs.readFileSync('/tmp/painter-v22-fixtures.json', 'utf8'));
const checkedReview = process.env.V22_CHECKED_REVIEW === '1';
const choiceOnly = process.env.V22_CHOICE_ONLY === '1';
const report = { origin, started_at: new Date().toISOString(), browser: '/usr/bin/google-chrome', mode: choiceOnly ? 'read-only, no screenshots; temporary checkbox indeterminate property only; forced-colors emulation' : 'read-only; native viewport captures; no CSS/DOM presentation overrides', scope: choiceOnly ? 'DOM-only supplemental choice state verification' : checkedReview ? 'three affected admin content captures after checked-state fix' : '17-capture V22 matrix', captures: [], checks: [], page_errors: [], console_errors: [], network_failures: [], blocked_writes: [], failures: [] };
const reportFile = choiceOnly ? 'report-choice-states.json' : checkedReview ? 'report-checked-fix.json' : 'report.json';
let browser;
function check(name, passed, evidence) { report.checks.push({ name, passed: !!passed, ...(evidence === undefined ? {} : { evidence }) }); }
async function frames(page) { await page.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve)))); }
async function scroll(page, selector, inset = 100) {
  await page.locator(selector).first().evaluate((el, offset) => window.scrollTo({ top: Math.max(0, el.getBoundingClientRect().top + window.scrollY - offset), behavior: 'instant' }), inset);
  await frames(page);
}
async function settle(page) { await page.evaluate(async () => { await document.fonts.ready; }); await frames(page); }
async function visibleDecode(page) {
  return page.evaluate(async () => {
    const visible = [...document.images].filter(img => { const r = img.getBoundingClientRect(); return r.width > 0 && r.height > 0 && r.left < innerWidth && r.right > 0 && r.top < innerHeight && r.bottom > 0; });
    await Promise.all(visible.map(async img => {
      if (!img.complete) await new Promise(resolve => { let timer; const done = () => { clearTimeout(timer); img.removeEventListener('load', done); img.removeEventListener('error', done); resolve(); }; img.addEventListener('load', done, { once: true }); img.addEventListener('error', done, { once: true }); timer = setTimeout(done, 16000); });
      if (img.complete && img.naturalWidth > 0) try { await img.decode(); } catch {}
    }));
    return visible.map(img => ({ src: img.currentSrc || img.src, natural_width: img.naturalWidth, natural_height: img.naturalHeight, complete: img.complete, alt: img.alt }));
  });
}
async function prepareWall(page, kind) {
  const selector = `[data-home-section="${kind}"]`;
  await page.locator(selector).waitFor();
  const pictures = page.locator(`${selector} img`);
  // Trigger the section's native lazy images without rewriting their loading/src attributes.
  for (let i = 0; i < await pictures.count(); i++) {
    await pictures.nth(i).evaluate(el => el.scrollIntoView({ block: 'center', behavior: 'instant' }));
    await frames(page);
  }
  await scroll(page, selector);
  await visibleDecode(page);
  await settle(page);
}
async function context(width, theme, owner = false) {
  const context = await browser.newContext({ viewport: { width, height: width >= 1000 ? 1000 : 900 }, deviceScaleFactor: 1, colorScheme: theme, locale: 'zh-CN', timezoneId: 'Asia/Shanghai', isMobile: width < 720, hasTouch: width < 720, ...(owner ? { storageState: ownerState } : {}) });
  await context.addInitScript(value => { try { localStorage.setItem('painter-theme', value); } catch {} }, theme);
  await context.route('**/*', route => {
    const request = route.request(), url = new URL(request.url());
    if (url.origin === origin && url.pathname === '/api/v1/analytics/events') return route.fulfill({ status: 200, contentType: 'application/json', body: '{"accepted":0}' });
    if (!['GET', 'HEAD', 'OPTIONS'].includes(request.method())) { report.blocked_writes.push({ method: request.method(), url: request.url() }); return route.abort(); }
    return route.continue();
  });
  const page = await context.newPage();
  page.setDefaultTimeout(12000);
  const errors = [], consoleErrors = [];
  page.on('pageerror', error => { const value = { width, theme, owner, url: page.url(), message: error.message }; errors.push(value); report.page_errors.push(value); });
  page.on('console', message => { if (message.type() === 'error') { const value = { width, theme, owner, url: page.url(), message: message.text() }; consoleErrors.push(value); report.console_errors.push(value); } });
  page.on('requestfailed', request => report.network_failures.push({ width, theme, url: request.url(), error: request.failure()?.errorText }));
  page.on('response', response => { if (response.status() >= 400) report.network_failures.push({ width, theme, url: response.url(), status: response.status() }); });
  return { context, page, width, theme, owner, errors, consoleErrors };
}
async function capture(surface, name, target, fullPage = false) {
  const { page, width, theme } = surface;
  if (checkedReview) name += '-after-checked-fix';
  await settle(page);
  const visible = await visibleDecode(page);
  await frames(page);
  const geometry = await page.evaluate(() => ({ viewport: { width: innerWidth, height: innerHeight }, document_width: document.documentElement.scrollWidth, body_width: document.body.scrollWidth, scroll_y: scrollY, theme: document.documentElement.dataset.theme, horizontal_offenders: [...document.querySelectorAll('main *,dialog[open] *')].filter(el => { const r = el.getBoundingClientRect(), style = getComputedStyle(el); return r.width > 0 && style.visibility !== 'hidden' && (r.left < -1 || r.right > innerWidth + 1) && !el.closest('.homepage-table-scroll,.home-writing-viewport'); }).slice(0, 12).map(el => ({ tag: el.tagName, class: el.className, text: (el.textContent || '').trim().slice(0, 90) })) }));
  const images = target ? await page.locator(target).evaluate(el => [...el.querySelectorAll('img')].map(img => ({ src: img.currentSrc || img.src, natural_width: img.naturalWidth, natural_height: img.naturalHeight, complete: img.complete, hidden: img.hidden, alt: img.alt }))) : visible;
  const file = name + '.png';
  await page.screenshot({ path: path.join(out, file), fullPage });
  check(`${name}: no document horizontal overflow`, geometry.document_width <= width + 1 && geometry.body_width <= width + 1, geometry);
  check(`${name}: correct theme`, geometry.theme === theme, geometry.theme);
  check(`${name}: no runtime errors`, surface.errors.length === 0, surface.errors);
  const broken = images.filter(img => img.complete && img.natural_width === 0);
  check(`${name}: captured image sources loaded`, broken.length === 0, broken);
  report.captures.push({ file, width, height: width >= 1000 ? 1000 : 900, theme, full_page: fullPage, url: page.url(), geometry, images, visible_images: visible, console_errors: [...surface.consoleErrors] });
  console.log(JSON.stringify({ capture: file, page_overflow: geometry.document_width > width + 1, broken_images: broken.length, runtime_errors: surface.errors.length }));
}
async function checkedStateReview(surface) {
  const { page, width, theme } = surface;
  const styles = await page.locator('.homepage-sections-table input[type="checkbox"]').evaluateAll(nodes => nodes.map(input => {
    const style = getComputedStyle(input), mark = getComputedStyle(input, '::after');
    return { label: input.closest('tr').querySelector('th').textContent, checked: input.checked, ink: getComputedStyle(document.body).color, background: style.backgroundColor, mark_content: mark.content, mark_width: mark.width, mark_height: mark.height, mark_border_color: mark.borderBottomColor, mark_border_width: mark.borderBottomWidth, mark_border_style: mark.borderBottomStyle };
  }));
  check(`${width}/${theme}: all six checked states use ink background`, styles.length === 6 && styles.every(value => value.checked && value.background === value.ink), styles);
  check(`${width}/${theme}: checked marks visibly contrast with their background`, styles.every(value => value.mark_content !== 'none' && parseFloat(value.mark_width) > 0 && parseFloat(value.mark_height) > 0 && parseFloat(value.mark_border_width) > 0 && value.mark_border_style === 'solid' && value.mark_border_color !== value.background), styles);
  const checkbox = page.locator('.homepage-sections-table input[type="checkbox"]').first();
  await checkbox.focus();
  await page.keyboard.press('Tab');
  await page.keyboard.press('Shift+Tab');
  const focus = await checkbox.evaluate(input => { const style = getComputedStyle(input); return { focused: input === document.activeElement, visible: input.matches(':focus-visible'), outline_style: style.outlineStyle, outline_width: style.outlineWidth, outline_color: style.outlineColor, background: style.backgroundColor }; });
  check(`${width}/${theme}: checkbox has visible keyboard focus`, focus.focused && focus.visible && focus.outline_style !== 'none' && parseFloat(focus.outline_width) >= 2 && focus.outline_color !== focus.background, focus);
  const disabled = await page.getByRole('button', { name: '将个人介绍上移', exact: true }).evaluate(button => { const style = getComputedStyle(button); return { disabled: button.disabled, opacity: style.opacity, cursor: style.cursor }; });
  check(`${width}/${theme}: unavailable move action is disabled and visually distinct`, disabled.disabled && parseFloat(disabled.opacity) < 1 && disabled.cursor === 'not-allowed', disabled);
}
async function choiceStateReview(theme) {
  const surface = await context(1440, theme, true);
  const { page } = surface;
  try {
    const response = await page.goto(origin + '/studio/home', { waitUntil: 'domcontentloaded' });
    check(`${theme}: supplemental admin HTTP 200`, response.status() === 200, response.status());
    const checkbox = page.locator('.homepage-sections-table input[type="checkbox"]').first();
    await checkbox.waitFor();
    await checkbox.evaluate(input => { input.indeterminate = true; });
    await frames(page);
    const mixed = await checkbox.evaluate(input => {
      const style = getComputedStyle(input), mark = getComputedStyle(input, '::after');
      const canvas = document.createElement('canvas').getContext('2d');
      const normalize = value => { canvas.fillStyle = value; return canvas.fillStyle; };
      return { indeterminate: input.indeterminate, matches_indeterminate: input.matches(':indeterminate'), checked: input.checked, background: style.backgroundColor, accent: style.accentColor, primary_ink: style.getPropertyValue('--primary-ink').trim(), normalized_primary_ink: normalize(style.getPropertyValue('--primary-ink').trim()), mark_content: mark.content, mark_width: mark.width, mark_height: mark.height, mark_border_width: mark.borderWidth, mark_background: mark.backgroundColor, normalized_mark_background: normalize(mark.backgroundColor), mark_transform: mark.transform };
    });
    check(`${theme}: mixed state uses accent background`, mixed.indeterminate && mixed.matches_indeterminate && mixed.background === mixed.accent, mixed);
    check(`${theme}: mixed state shows primary-ink horizontal line`, mixed.mark_content !== 'none' && mixed.mark_width === '8px' && mixed.mark_height === '2px' && mixed.mark_border_width === '0px' && mixed.normalized_mark_background === mixed.normalized_primary_ink && mixed.mark_transform === 'none', mixed);
    await checkbox.evaluate(input => { input.indeterminate = false; });
    await page.emulateMedia({ forcedColors: 'active' });
    await frames(page);
    const forced = await checkbox.evaluate(input => { const style = getComputedStyle(input), mark = getComputedStyle(input, '::after'); return { active: matchMedia('(forced-colors: active)').matches, indeterminate: input.indeterminate, checked: input.checked, appearance: style.appearance, mark_content: mark.content }; });
    check(`${theme}: forced-colors restores native appearance without custom mark`, forced.active && !forced.indeterminate && forced.checked && forced.appearance === 'auto' && forced.mark_content === 'none', forced);
    await page.emulateMedia({ forcedColors: 'none' });
  } finally { await surface.context.close(); }
}
async function publicViews(width, theme, kinds, viewer = false) {
  const surface = await context(width, theme);
  try {
    const response = await surface.page.goto(origin + '/', { waitUntil: 'domcontentloaded' });
    check(`${width}/${theme}: homepage HTTP 200`, response.status() === 200, response.status());
    await surface.page.locator('[data-home-section="steam"] .home-game').first().waitFor();
    check(`${width}/${theme}: fixture wall cardinality`, await surface.page.locator('.home-game').count() === 3 && await surface.page.locator('.home-record').count() === 3 && await surface.page.locator('.home-photo').count() === 3);
    check(`${width}/${theme}: configured article count`, await surface.page.locator('.home-writing-entry').count() === 3);
    for (const kind of kinds) { await prepareWall(surface.page, kind); await capture(surface, `${width}-${theme}-${kind}`, `[data-home-section="${kind}"]`); }
    if (viewer) {
      await prepareWall(surface.page, 'photos');
      await surface.page.locator('.home-photo [data-moment-image]').first().click();
      await surface.page.locator('.moment-lightbox[open]').waitFor();
      await surface.page.waitForFunction(() => { const image = document.querySelector('.moment-lightbox-stage img'); return image && !image.hidden && image.complete && image.naturalWidth > 0; });
      check(`${width}/${theme}: photo viewer shows actual source`, await surface.page.locator('.moment-lightbox-stage img').getAttribute('src') === origin + '/media/' + fixture.items.photos[0].media_id);
      await capture(surface, `${width}-${theme}-viewer`, '.moment-lightbox');
      await surface.page.getByRole('button', { name: '关闭图片预览', exact: true }).click();
    }
  } finally { await surface.context.close(); }
}
async function adminViews(width, theme, views) {
  const surface = await context(width, theme, true);
  try {
    const response = await surface.page.goto(origin + '/studio/home', { waitUntil: 'domcontentloaded' });
    check(`${width}/${theme}: admin HTTP 200`, response.status() === 200, response.status());
    await surface.page.locator('.homepage-manager').waitFor();
    await surface.page.getByRole('tab', { name: '页面内容', exact: true }).waitFor();
    await surface.page.locator('.homepage-sections-table tbody tr').first().waitFor();
    for (const view of views) {
      const label = { content: '页面内容', walls: '展示墙', steam: 'Steam 设置' }[view];
      await surface.page.getByRole('tab', { name: label, exact: true }).click();
      if (view === 'content') {
        await surface.page.getByRole('button', { name: '编辑最近文章内容', exact: true }).click();
        const count = surface.page.locator('.homepage-section-editor input[type="number"]');
        check(`${width}/${theme}: article count control reflects saved three`, await count.inputValue() === '3');
        if (width < 720) {
          const wrapper = surface.page.locator('.homepage-sections-table').locator('..');
          await wrapper.evaluate(el => { el.scrollLeft = el.scrollWidth; });
          check(`${width}/${theme}: narrow table actions reachable locally`, await surface.page.getByRole('button', { name: '编辑最近文章内容', exact: true }).evaluate(el => { const r = el.getBoundingClientRect(), container = el.closest('.homepage-table-scroll').getBoundingClientRect(); return r.left >= container.left - 1 && r.right <= container.right + 1; }));
          await wrapper.evaluate(el => { el.scrollLeft = 0; });
        }
      }
      if (view === 'walls') {
        if (width < 720) { await surface.page.getByRole('combobox', { name: '展示墙', exact: true }).click(); await surface.page.getByRole('option', { name: '个人照片', exact: true }).click(); }
        await surface.page.waitForFunction(() => document.querySelector('.homepage-table-scroll[aria-busy]')?.getAttribute('aria-busy') === 'false');
        await surface.page.locator('.homepage-items-table tbody tr').first().getByRole('button', { name: '编辑', exact: true }).click();
      }
      await surface.page.evaluate(() => window.scrollTo({ top: 0, behavior: 'instant' }));
      await frames(surface.page);
      await capture(surface, `${width}-${theme}-admin-${view}`, '.homepage-manager', true);
      if (checkedReview && view === 'content') await checkedStateReview(surface);
    }
  } finally { await surface.context.close(); }
}
(async () => {
  fs.mkdirSync(out, { recursive: true });
  browser = await chromium.launch({ executablePath: '/usr/bin/google-chrome', headless: true, args: ['--no-sandbox'] });
  const jobs = choiceOnly ? [
    () => choiceStateReview('light'),
    () => choiceStateReview('dark')
  ] : checkedReview ? [
    () => adminViews(1440, 'light', ['content']),
    () => adminViews(1440, 'dark', ['content']),
    () => adminViews(390, 'light', ['content'])
  ] : [
    () => publicViews(1440, 'light', ['steam', 'music', 'photos']),
    () => adminViews(1440, 'light', ['content', 'walls', 'steam']),
    () => publicViews(1440, 'dark', ['music']),
    () => adminViews(1440, 'dark', ['content']),
    () => publicViews(390, 'light', ['steam', 'music', 'photos']),
    () => adminViews(390, 'light', ['content']),
    () => publicViews(390, 'dark', ['music'], true),
    () => adminViews(390, 'dark', ['walls']),
    () => publicViews(320, 'light', ['music']),
    () => adminViews(320, 'light', ['content'])
  ];
  for (let i = 0; i < jobs.length; i++) try { await jobs[i](); } catch (error) { report.failures.push({ job: i + 1, error: String(error) }); console.error(JSON.stringify(report.failures.at(-1))); }
  const captureCount = choiceOnly ? 0 : checkedReview ? 3 : 17;
  check(choiceOnly ? 'no additional screenshots captured' : `all ${captureCount} requested screenshots captured`, report.captures.length === captureCount, report.captures.length);
  check('no application write attempted', report.blocked_writes.length === 0, report.blocked_writes);
  check('no browser runtime errors', report.page_errors.length === 0, report.page_errors);
  report.finished_at = new Date().toISOString();
  report.passed = report.failures.length === 0 && report.checks.every(item => item.passed);
  fs.writeFileSync(path.join(out, reportFile), JSON.stringify(report, null, 2));
  console.log(JSON.stringify({ passed: report.passed, captures: report.captures.length, checks: report.checks.length, console_errors: report.console_errors.length, network_failures: report.network_failures.length, failed_checks: report.checks.filter(item => !item.passed).map(item => item.name) }));
  await browser.close();
  if (!report.passed) process.exitCode = 1;
})().catch(async error => { report.failures.push({ error: String(error) }); fs.mkdirSync(out, { recursive: true }); fs.writeFileSync(path.join(out, reportFile), JSON.stringify(report, null, 2)); await browser?.close(); console.error(String(error)); process.exitCode = 1; });
