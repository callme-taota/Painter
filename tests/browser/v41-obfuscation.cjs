// Real application/CSP regression against the dedicated V41 candidate only.
// Never points at the formal service; only the explicitly allowed local draft
// and media mutations below can reach the isolated candidate.
const fs = require('fs');
const path = require('path');
const {chromium} = require('/tmp/painter-browser/node_modules/playwright');
const base = 'http://127.0.0.1:18086';
const out = path.resolve('tests/artifacts/v41');
const credentials = JSON.parse(fs.readFileSync(path.join(out, 'qa-credentials.json'), 'utf8'));
const image = fs.readFileSync('tests/browser/fixtures/avatar.png');
const report = {
  base, mode: 'isolated PostgreSQL candidate; real browser and local draft/media writes',
  checks: [], loaded_scripts: [], core_obfuscated_scripts: [], csp_violations: [], expected_probe_violations: [], page_errors: [],
  resource_errors: [], blocked_requests: [], external_image_fixtures: [],
  local_mutations: [], screenshots: [], limitations: [
    'External platform images are fulfilled with a local PNG fixture; provider availability is outside this test.',
    'HTTPS forwarding and redirects are covered separately by Go tests and public read probes.',
    'No draft is published; no formal service, production database, or external write endpoint is contacted.'
  ]
};
const redact = value => [credentials.username, credentials.password].filter(Boolean).reduce((text, secret) => text.split(secret).join('[redacted]'), String(value));
let browser, context, currentStep = 'startup';
const pendingScriptReads = [], seenScripts = new Set();
function check(name, passed, detail) {
  report.checks.push({step: currentStep, name, passed: !!passed, ...(detail === undefined ? {} : {detail})});
  console.log((passed ? 'PASS ' : 'FAIL ') + name);
  if (!passed) throw Error(name);
}
function saveReport() {
  fs.mkdirSync(out, {recursive:true});
  fs.writeFileSync(path.join(out, 'browser-obfuscation.json'), redact(JSON.stringify(report, null, 2)));
}
async function ready(page) {
  await page.waitForFunction(() => document.querySelector('#site-search')?.dataset.raycastSearchReady === 'true');
  await page.evaluate(() => document.fonts.ready);
}
async function capture(page, name) {
  const file = name + '.png';
  await page.screenshot({path:path.join(out, file)});
  report.screenshots.push(file);
}
async function navigate(page, target) {
  const response = await page.goto(base + target);
  check(target + ' responds with the candidate CSP', response.status() === 200 && /script-src 'self' 'nonce-[^']+'/.test(response.headers()['content-security-policy'] || ''));
  await ready(page);
  check(target + ' CSP forbids eval execution', !/unsafe-eval/.test(response.headers()['content-security-policy'] || ''));
  check(target + ' inline scripts match response nonce', await page.evaluate(() => [...document.querySelectorAll('script:not([src])')].every(script => /^[A-Za-z0-9+/]{32}$/.test(script.nonce))));
  return response;
}
async function properties(page) {
  const details = page.locator('.editor-properties');
  if (!await details.evaluate(element => element.open)) await details.locator('summary').click();
}
async function run() {
  fs.mkdirSync(out, {recursive:true});
  browser = await chromium.launch({headless:true, executablePath:'/usr/bin/google-chrome', args:['--no-sandbox','--disable-dev-shm-usage']});
  context = await browser.newContext({viewport:{width:1440,height:1000}, colorScheme:'light', locale:'zh-CN', serviceWorkers:'block'});
  await context.exposeBinding('__v41CaptureViolation', (_, item) => {
    (item.expected ? report.expected_probe_violations : report.csp_violations).push({...item, step:currentStep});
  });
  await context.addInitScript(() => {
    localStorage.setItem('painter-theme', 'dark');
    localStorage.setItem('painter-analytics', 'off');
    document.addEventListener('securitypolicyviolation', event => window.__v41CaptureViolation({
      path:location.pathname, effectiveDirective:event.effectiveDirective,
      blockedURI:event.blockedURI, disposition:event.disposition,
      expected:!!window.__v41ExpectedProbe
    }));
  });
  await context.route('**/*', async route => {
    const request = route.request(), url = new URL(request.url()), method = request.method();
    if (url.origin !== base) {
      if (method === 'GET' && request.resourceType() === 'image' && url.protocol === 'https:') {
        if (!report.external_image_fixtures.includes(url.hostname)) report.external_image_fixtures.push(url.hostname);
        return route.fulfill({status:200, contentType:'image/png', body:image});
      }
      report.blocked_requests.push({method, origin:url.origin, path:url.pathname, resource:request.resourceType()});
      return route.abort();
    }
    if (url.pathname === '/api/v1/analytics/events') return route.fulfill({status:204});
    if (!['GET','HEAD'].includes(method)) {
      const allowed = method === 'POST' && ['/api/v1/auth/login','/api/v1/editor/preview','/api/v1/editor/cover-preview','/api/v1/media','/api/v1/posts'].includes(url.pathname)
        || method === 'PUT' && /^\/api\/v1\/studio\/posts\/[^/]+\/draft$/.test(url.pathname);
      if (!allowed) {
        report.blocked_requests.push({method, path:url.pathname, resource:request.resourceType()});
        return route.abort();
      }
      report.local_mutations.push({method, path:url.pathname});
    }
    return route.continue();
  });
  const page = await context.newPage();
  page.setDefaultTimeout(12000);
  page.on('pageerror', error => report.page_errors.push({step:currentStep, message:redact(error.message)}));
  page.on('response', response => {
    const url = new URL(response.url());
    if (url.origin === base && response.status() >= 400) report.resource_errors.push({step:currentStep, status:response.status(), path:url.pathname});
    if (url.origin === base && url.pathname.endsWith('.js') && response.ok() && !seenScripts.has(url.pathname)) {
      seenScripts.add(url.pathname);
      pendingScriptReads.push(response.body().then(buffer => {
        const source = buffer.toString('utf8');
        report.loaded_scripts.push({path:url.pathname, bytes:buffer.length,
          has_string_decoder:source.includes('abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789+/=') && source.includes('decodeURIComponent'),
          encoded_array_blocks:(source.match(/\["[A-Za-z0-9+/=]{4,}"(?:,"[A-Za-z0-9+/=]*"){3,}/g) || []).length,
          direct_eval_or_function:/\b(?:eval|Function)\s*\(/.test(source)
        });
      }).catch(error => { report.resource_errors.push({path:url.pathname, kind:'script-body-read', message:redact(error.message)}); }));
    }
  });
  currentStep = 'public-home';
  await navigate(page, '/');
  check('nonced theme bootstrap executes before app enhancement', await page.locator('html').getAttribute('data-theme') === 'dark');
  check('header liquid glass is enhanced', await page.locator('.site-header').getAttribute('data-liquid-glass-ready') === 'true');
  await page.locator('[data-search-open]').click();
  await page.waitForFunction(() => document.querySelector('#site-search')?.open && document.querySelector('#site-search')?.dataset.liquidGlassReady === 'true');
  check('search liquid glass opens and accepts input', await page.locator('#quick-search').evaluate(element => element === document.activeElement));
  await page.locator('#quick-search').fill('数据');
  await page.waitForFunction(() => !document.querySelector('#site-search').hasAttribute('data-search-loading'));
  check('search suggestion request completes under same-origin policy', await page.locator('[data-search-status]').textContent().then(text => !!text.trim()));
  await capture(page, 'security-search-glass');
  await page.keyboard.press('Escape');
  await page.locator('.floating-theme summary').click();
  await page.locator('[data-theme-option="light"]').click();
  check('theme picker works with strict script policy', await page.locator('html').getAttribute('data-theme') === 'light');
  const publicPosts = await page.evaluate(async () => (await (await fetch('/api/v1/posts?limit=5')).json()).items);
  check('seeded public articles are available', Array.isArray(publicPosts) && publicPosts.length > 0);
  currentStep = 'public-article';
  await navigate(page, '/posts/' + publicPosts[0].id);
  check('article body and JSON-LD render', await page.locator('.article-reading .prose').count() > 0 && await page.locator('script[type="application/ld+json"]').count() === 1);
  currentStep = 'public-search';
  await navigate(page, '/search?q=' + encodeURIComponent('数据'));
  check('custom search filter surfaces initialize', await page.locator('[data-tag-filter] summary').getAttribute('data-liquid-glass-ready') === 'true');
  currentStep = 'local-login';
  await navigate(page, '/login?return_to=%2Fstudio%2Fposts%2Fnew');
  await page.getByRole('textbox', {name:'用户名', exact:true}).fill(credentials.username);
  await page.getByLabel('密码', {exact:true}).fill(credentials.password);
  await page.getByRole('button', {name:'登录', exact:true}).click();
  await page.waitForURL(base + '/studio/posts/new');
  await page.waitForFunction(() => document.querySelector('.editor-title')?.disabled === false);
  check('real isolated login reaches unlocked studio editor', await page.getByRole('textbox', {name:'文档正文', exact:true}).isVisible());
  currentStep = 'isolated-editor';
  await page.getByRole('textbox', {name:'文章标题', exact:true}).fill('V41 CSP 隔离回归草稿');
  const documentBody = page.getByRole('textbox', {name:'文档正文', exact:true});
  await documentBody.fill('V41 browser security regression body');
  await documentBody.press('Control+a');
  await page.locator('.document-toolbar button[aria-label="加粗"]').click();
  check('Tiptap editing and formatting work', await documentBody.locator('strong').textContent() === 'V41 browser security regression body');
  await properties(page);
  await page.waitForFunction(() => {
    const image = document.querySelector('.cover-preview img');
    return image?.src.startsWith('blob:') && image.complete && image.naturalWidth > 0;
  });
  check('automatic cover blob preview loads under CSP', true);
  await page.locator('.cover-editor .select-trigger').first().click();
  await page.getByRole('option', {name:'上传图片', exact:true}).click();
  await page.locator('.cover-editor input[type="file"]').setInputFiles({name:'v41-isolated-cover.png', mimeType:'image/png', buffer:image});
  await page.waitForFunction(() => {
    const image = document.querySelector('.cover-preview img');
    return image && new URL(image.src).pathname.startsWith('/media/') && image.complete && image.naturalWidth > 0;
  });
  check('authenticated upload and media cover preview load', true);
  await page.getByRole('button', {name:'保存草稿', exact:true}).click();
  await page.waitForFunction(() => document.querySelector('.save-state')?.textContent.trim() === '已保存' && /^\/studio\/posts\/[^/]+$/.test(location.pathname) && !location.pathname.endsWith('/new'));
  report.draft_path = new URL(page.url()).pathname;
  check('draft saves to the isolated PostgreSQL candidate', true);
  await page.locator('.editor-more summary').click();
  await page.getByRole('button', {name:'编辑 Markdown 源码', exact:true}).click();
  const source = page.getByRole('textbox', {name:'Markdown 正文', exact:true});
  await source.waitFor();
  check('Markdown mode retains document formatting', await source.textContent().then(text => text.includes('**V41 browser security regression body**')));
  await source.click();
  await source.press('Control+End');
  await source.press('End');
  await source.press('Enter');
  await source.press('Enter');
  await source.pressSequentially('Source mode stays functional under CSP.');
  await page.getByRole('button', {name:'保存草稿', exact:true}).click();
  await page.waitForFunction(() => document.querySelector('.save-state')?.textContent.trim() === '已保存');
  await page.locator('.editor-source-context').getByRole('button', {name:'返回文档编辑', exact:true}).click();
  check('returning to Tiptap retains saved Markdown edits', await documentBody.textContent().then(text => text.includes('Source mode stays functional under CSP.')));
  await capture(page, 'security-editor');
  await page.reload();
  await page.waitForFunction(() => document.querySelector('.editor-title')?.disabled === false);
  check('server reload retains text and cover metadata', await page.getByRole('textbox', {name:'文章标题', exact:true}).inputValue() === 'V41 CSP 隔离回归草稿' && await page.getByRole('textbox', {name:'文档正文', exact:true}).textContent().then(text => text.includes('Source mode stays functional under CSP.')));
  await properties(page);
  check('uploaded cover remains on saved draft', await page.locator('.cover-preview img').getAttribute('src').then(src => src.startsWith('/media/')));
  await Promise.all(pendingScriptReads);
  report.core_obfuscated_scripts = report.loaded_scripts.filter(script => /^\/assets\/(?:public-|studio-|chunks\/AuthForm-|chunks\/MarkdownEditor-)/.test(script.path));
  check('public, studio, auth and lazy editor chunks carry obfuscated string decoding', report.core_obfuscated_scripts.length === 4 && report.core_obfuscated_scripts.every(script => script.has_string_decoder && script.encoded_array_blocks > 0), report.core_obfuscated_scripts);
  check('loaded scripts contain no direct eval or Function execution', report.loaded_scripts.every(script => !script.direct_eval_or_function));
  currentStep = 'expected-script-block-probes';
  await page.evaluate(() => {
    window.__v41ExpectedProbe = true;
    const inline = document.createElement('script');
    inline.textContent = 'window.__v41ForbiddenInlineExecuted = true';
    document.head.appendChild(inline);
    const external = document.createElement('script');
    external.src = 'https://csp-probe.invalid/v41-external.js';
    document.head.appendChild(external);
  });
  await page.waitForTimeout(400);
  check('injected nonced-less inline and external scripts are blocked', report.expected_probe_violations.some(item => item.blockedURI === 'inline') && report.expected_probe_violations.some(item => item.blockedURI === 'https://csp-probe.invalid/v41-external.js') && await page.evaluate(() => !window.__v41ForbiddenInlineExecuted));
  check('normal functions emit no CSP violations', report.csp_violations.length === 0, report.csp_violations);
  check('normal functions emit no JavaScript page errors', report.page_errors.length === 0, report.page_errors);
  check('candidate resources have no HTTP errors', report.resource_errors.length === 0, report.resource_errors);
  check('request guard saw no formal/external mutation or unexpected request', report.blocked_requests.length === 0, report.blocked_requests);
  report.passed = true;
}
run().catch(error => {
  report.passed = false;
  report.failure = redact(error.stack || error.message);
  console.error(redact(error.message));
  process.exitCode = 1;
}).finally(async () => {
  saveReport();
  await context?.close();
  await browser?.close();
  console.log(JSON.stringify({passed:report.passed, checks:report.checks.length, report:path.join(out, 'browser-obfuscation.json')}));
});
