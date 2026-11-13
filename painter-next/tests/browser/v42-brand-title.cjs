// Anonymous, read-only branding regression against the V42 isolated candidate.
// The origin is deliberately fixed; no credentials or production data are read.
const fs = require('fs');
const path = require('path');
const crypto = require('crypto');
const {chromium} = require('/tmp/painter-browser/node_modules/playwright');
const base = 'http://127.0.0.1:18086';
const out = path.resolve('tests/artifacts/v42');
const oldIconHash = 'd4d342f8b1700a4808fdd62dfccf865cf0fc0f7fbe5f4b005100ae7d8dad49b7';
const oldLogoHash = '9e34ce25e135426dddfe9b1f0e9fb00d539454423607d4b1c80f666128768f59';
const imageFixture = fs.readFileSync('tests/browser/fixtures/avatar.png');
const report = {
  origin: base, database: 'painter_v42_branding_20261112', read_only: true,
  checks: [], page_titles: [], icons: [], screenshots: [], csp_violations: [],
  page_errors: [], resource_errors: [], blocked_writes: [], external_image_fixtures: [],
  limitations: [
    'Anonymous candidate pages only; no login, credentials, or records are created.',
    'External images use a local PNG fixture; provider availability is outside this test.',
    'Chrome desktop and emulated phone viewports; no physical-device or cross-engine claim.'
  ]
};
let browser, currentStep = 'startup';
const hash = body => crypto.createHash('sha256').update(body).digest('hex');
function check(name, passed, detail) {
  report.checks.push({step: currentStep, name, passed: !!passed, ...(detail === undefined ? {} : {detail})});
  console.log((passed ? 'PASS ' : 'FAIL ') + name);
  if (!passed) throw Error(name);
}
function save() {
  fs.mkdirSync(out, {recursive: true});
  fs.writeFileSync(path.join(out, 'browser-brand-title.json'), JSON.stringify(report, null, 2));
}
async function createContext(width, theme) {
  const context = await browser.newContext({
    viewport: {width, height: width < 720 ? 844 : 1000}, colorScheme: theme,
    isMobile: width < 720, hasTouch: width < 720, locale: 'zh-CN', serviceWorkers: 'block'
  });
  await context.exposeBinding('__v42CspViolation', (_, event) => report.csp_violations.push({...event, step: currentStep}));
  await context.addInitScript(theme => {
    // Chrome also invokes init scripts for its opaque initial about:blank page.
    try {
      localStorage.setItem('painter-theme', theme);
      localStorage.setItem('painter-analytics', 'off');
    } catch {}
    document.addEventListener('securitypolicyviolation', event => window.__v42CspViolation({
      path: location.pathname, effectiveDirective: event.effectiveDirective,
      blockedURI: event.blockedURI, disposition: event.disposition
    }));
  }, theme);
  await context.route('**/*', async route => {
    const request = route.request(), url = new URL(request.url());
    if (!['GET', 'HEAD'].includes(request.method())) {
      report.blocked_writes.push({method: request.method(), path: url.pathname});
      return route.fulfill({status: 204});
    }
    if (url.origin !== base) {
      if (request.resourceType() === 'image' && url.protocol === 'https:') {
        if (!report.external_image_fixtures.includes(url.hostname)) report.external_image_fixtures.push(url.hostname);
        return route.fulfill({status: 200, contentType: 'image/png', body: imageFixture});
      }
      return route.abort();
    }
    return route.continue();
  });
  context.on('page', page => {
    page.setDefaultTimeout(12000);
    page.on('pageerror', error => report.page_errors.push({step: currentStep, message: error.message}));
    page.on('response', response => {
      const url = new URL(response.url());
      if (url.origin === base && response.status() >= 400) report.resource_errors.push({step: currentStep, path: url.pathname, status: response.status()});
    });
  });
  return context;
}
async function ready(page) {
  await page.waitForFunction(() => document.querySelector('#site-search')?.dataset.raycastSearchReady === 'true');
  await page.evaluate(() => document.fonts.ready);
}
async function inspect(page, expectedTitle, label) {
  await ready(page);
  const title = await page.title();
  report.page_titles.push({step: currentStep, path: new URL(page.url()).pathname, title});
  check(label + ' document title', title === expectedTitle, {expected: expectedTitle, actual: title});
  const brand = page.locator('.site-header .painter-logo');
  await brand.evaluate(async image => { await image.decode(); });
  const logo = await brand.evaluate(image => {
    const rect = image.getBoundingClientRect(), style = getComputedStyle(image);
    return {width: rect.width, height: rect.height, naturalWidth: image.naturalWidth, naturalHeight: image.naturalHeight,
      source: image.getAttribute('src'), display: style.display, visibility: style.visibility, opacity: style.opacity,
      inViewport: rect.top >= 0 && rect.bottom <= innerHeight && rect.left >= 0 && rect.right <= innerWidth};
  });
  check(label + ' original 512px navigation logo is visible', logo.source === '/assets/images/painter-logo.png'
    && logo.naturalWidth === 512 && logo.naturalHeight === 512 && logo.width > 0 && logo.height > 0
    && logo.display !== 'none' && logo.visibility === 'visible' && Number(logo.opacity) > 0 && logo.inViewport, logo);
  const icons = await page.evaluate(async () => {
    const icon = document.head.querySelector('link[rel="icon"]'), apple = document.head.querySelector('link[rel="apple-touch-icon"]');
    async function decode(element) {
      if (!element) return null;
      const image = new Image(); image.src = element.href; await image.decode();
      return {href: element.href, type: element.type, width: image.naturalWidth, height: image.naturalHeight};
    }
    return {icon: await decode(icon), apple: await decode(apple), titleCount: document.head.querySelectorAll('title').length};
  });
  report.icons.push({step: currentStep, path: new URL(page.url()).pathname, ...icons});
  check(label + ' has exactly one title and decodable original favicon', icons.titleCount === 1
    && icons.icon?.href === base + '/assets/images/painter-favicon-d4d342f8b170.ico'
    && icons.icon.type === 'image/x-icon' && icons.icon.width === 128 && icons.icon.height === 128, icons);
  check(label + ' original apple icon decodes', icons.apple?.href === base + '/assets/images/painter-logo.png'
    && icons.apple.width === 512 && icons.apple.height === 512);
}
async function navigate(page, target, expectedTitle, label) {
  const response = await page.goto(base + target);
  check(label + ' returns HTML with nonce CSP', response?.status() === 200
    && /script-src 'self' 'nonce-[^']+'/.test(response.headers()['content-security-policy'] || ''));
  await inspect(page, expectedTitle, label);
}
async function run() {
  fs.mkdirSync(out, {recursive: true});
  browser = await chromium.launch({headless: true, executablePath: '/usr/bin/google-chrome', args: ['--no-sandbox', '--disable-dev-shm-usage']});
  let posts;
  for (const width of [1440, 390]) for (const theme of ['light', 'dark']) {
    const context = await createContext(width, theme), page = await context.newPage();
    const label = `${width}/${theme}`;
    currentStep = label + '/assets';
    if (!posts) {
      const response = await context.request.get(base + '/api/v1/posts?limit=2');
      check('public posts GET is available', response.status() === 200);
      posts = (await response.json()).items.slice(0, 2);
      check('two distinct published candidate articles available', posts.length === 2 && posts[0].id !== posts[1].id);
      for (const [asset, expectedHash, type] of [
        ['/favicon.ico', oldIconHash, ['image/x-icon', 'image/vnd.microsoft.icon']],
        ['/assets/images/painter-favicon-d4d342f8b170.ico', oldIconHash, ['image/x-icon', 'image/vnd.microsoft.icon']],
        ['/assets/images/painter-logo.png', oldLogoHash, ['image/png']]
      ]) {
        const response = await context.request.get(base + asset), actualHash = hash(await response.body());
        check(asset + ' returns unchanged old brand image', response.status() === 200
          && type.some(value => response.headers()['content-type'].startsWith(value)) && actualHash === expectedHash, {sha256: actualHash});
      }
    }
    currentStep = label + '/home';
    await navigate(page, '/', 'Painter', label + ' home');
    check(label + ' selected theme remains active', await page.locator('html').getAttribute('data-theme') === theme);
    if (width === 390 && theme === 'dark') {
      const file = 'mobile-dark-home.png'; await page.screenshot({path: path.join(out, file)}); report.screenshots.push(file);
    }
    for (let i = 0; i < posts.length; i++) {
      const post = posts[i]; currentStep = label + '/article-' + post.id;
      await navigate(page, '/posts/' + post.id, post.title + ' · Painter', label + ' article ' + post.id);
      check(label + ' article ' + post.id + ' heading matches published title', await page.locator('.article-heading h1').innerText() === post.title);
      if (width === 1440 && theme === 'light' && i === 0) {
        const file = 'desktop-light-article.png'; await page.screenshot({path: path.join(out, file)}); report.screenshots.push(file);
      }
    }
    currentStep = label + '/history-back';
    await page.goBack(); await inspect(page, posts[0].title + ' · Painter', label + ' back to article one');
    currentStep = label + '/history-forward';
    await page.goForward(); await inspect(page, posts[1].title + ' · Painter', label + ' forward to article two');
    currentStep = label + '/refresh';
    await page.reload(); await inspect(page, posts[1].title + ' · Painter', label + ' refreshed article two');
    await context.close();
  }
  currentStep = 'final';
  check('no CSP violations', report.csp_violations.length === 0, report.csp_violations);
  check('no page errors', report.page_errors.length === 0, report.page_errors);
  check('no same-origin resource failures', report.resource_errors.length === 0, report.resource_errors);
  check('no content or analytics writes attempted', report.blocked_writes.length === 0, report.blocked_writes);
  report.passed = true; save();
  console.log(JSON.stringify({passed: true, checks: report.checks.length, screenshots: report.screenshots.length, writes: 0}));
}
run().catch(error => {report.passed = false; report.failure = {step: currentStep, message: String(error)}; save(); console.error(error); process.exitCode = 1;})
  .finally(async () => {await browser?.close();});
