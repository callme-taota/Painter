// Synthetic metadata rendered by the production Go consent template. No live
// account, password, session or client credentials are used by this visual QA.
const fs = require('fs');
const path = require('path');
const http = require('http');
const {chromium} = require('/tmp/painter-browser/node_modules/playwright');
const output = path.resolve('tests/artifacts/v43/identity-consent');
const source = process.env.PAINTER_IDENTITY_PREVIEW_DIR;
if (!source) throw Error('PAINTER_IDENTITY_PREVIEW_DIR must name the synthetic Go fixture directory');
const report = {fixture: 'production Go template, synthetic client metadata', checks: [], screenshots: [], errors: []};
const assert = (name, condition) => { report.checks.push({name, passed: !!condition}); if (!condition) throw Error(name); };
(async () => {
  fs.mkdirSync(output, {recursive: true});
  const browser = await chromium.launch({headless: true, executablePath: '/usr/bin/google-chrome', args: ['--no-sandbox', '--disable-dev-shm-usage']});
  try {
    for (const theme of ['light', 'dark']) for (const [viewport, width, height] of [['desktop', 1440, 1000], ['mobile', 390, 844]]) {
      const context = await browser.newContext({viewport: {width, height}, colorScheme: theme});
      const page = await context.newPage();
      page.on('pageerror', error => report.errors.push(String(error)));
      await page.route('**/*', async route => {
        const url = new URL(route.request().url());
        if (url.pathname === '/assets/images/painter-logo.png') return route.fulfill({contentType: 'image/png', body: fs.readFileSync('web/assets/images/painter-logo.png')});
        if (url.pathname === '/oidc/consent' && route.request().method() === 'GET') return route.fulfill({contentType: 'text/html; charset=utf-8', body: fs.readFileSync(path.join(source, 'identity-consent-' + (url.searchParams.get('state') || 'allow') + '.html'))});
        return route.abort();
      });
      for (const state of ['allow', 'reauth']) {
        await page.goto('http://identity-preview.test/oidc/consent?state=' + state);
        await page.locator('main').waitFor();
        await page.locator('.brand img').evaluate(img => img.decode());
        assert(`${theme}/${viewport}/${state}: no horizontal overflow`, await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth));
        assert(`${theme}/${viewport}/${state}: identity and return origin visible`, await page.getByText('Taota', {exact: true}).isVisible() && await page.getByText(/workspace\.example/).isVisible());
        assert(`${theme}/${viewport}/${state}: primary action visible`, await page.getByRole('button', {name: state === 'allow' ? '允许并登录' : '重新登录'}).isVisible());
        await page.getByRole('button', {name: '拒绝', exact: true}).focus();
        assert(`${theme}/${viewport}/${state}: keyboard focus visible`, await page.getByRole('button', {name: '拒绝', exact: true}).evaluate(el => getComputedStyle(el).outlineStyle !== 'none'));
        const shot = path.join(output, `${theme}-${viewport}-${state}.png`);
        await page.screenshot({path: shot, fullPage: true}); report.screenshots.push(shot);
      }
      await context.close();
    }
    // Chromium applies form-action to the entire HTTP redirect chain. Exercise
    // the actual Go continuation template under the unchanged production CSP.
    for (const action of ['allow', 'deny']) {
      const context = await browser.newContext();
      const page = await context.newPage();
      const violations = [];
      page.on('console', message => { if (message.type() === 'error') violations.push(message.text()); });
      const csp = "default-src 'self'; form-action 'self'; script-src 'nonce-preview-only-nonce'; object-src 'none'; base-uri 'none'";
      const clientServer = http.createServer((request, response) => response.end('application callback reached'));
      await new Promise(resolve => clientServer.listen(0, '127.0.0.1', resolve));
      const clientOrigin = 'http://127.0.0.1:' + clientServer.address().port;
      let identityOrigin;
      let formOrigin;
      const identityServer = http.createServer((request, response) => {
        response.setHeader('Content-Security-Policy', csp);
        response.setHeader('Content-Type', 'text/html; charset=utf-8');
        if (request.url.startsWith('/oidc/authorize/callback')) { response.writeHead(302, {Location: clientOrigin + '/callback?code=preview-only-code'}); response.end(); return; }
        if (request.url === '/form' && request.method === 'POST') { formOrigin=request.headers.origin; response.setHeader('Referrer-Policy','no-referrer'); response.end(fs.readFileSync(path.join(source, `identity-continue-${action}.html`), 'utf8').replaceAll('http://identity-preview.test', identityOrigin).replaceAll('http://client-preview.test', clientOrigin)); return; }
        response.setHeader('Referrer-Policy','same-origin');
        response.end('<form action="/form" method="post"><button>continue</button></form>');
      });
      await new Promise(resolve => identityServer.listen(0, '127.0.0.1', resolve));
      identityOrigin = 'http://127.0.0.1:' + identityServer.address().port;
      try {
        await page.goto(identityOrigin + '/');
        await Promise.all([page.waitForURL(clientOrigin + '/**'), page.getByRole('button').click()]);
        assert(`CSP ${action}: callback navigation completes`, page.url().startsWith(clientOrigin + '/callback'));
        assert(`CSRF ${action}: native form preserves same-origin Origin`, formOrigin === identityOrigin);
        assert(`CSP ${action}: no policy violations`, violations.length === 0);
      } finally {
        await context.close();
        await new Promise(resolve => identityServer.close(resolve));
        await new Promise(resolve => clientServer.close(resolve));
      }
    }
    assert('no JavaScript errors', report.errors.length === 0);
  } finally {
    await browser.close();
    fs.writeFileSync(path.join(output, 'report.json'), JSON.stringify(report, null, 2));
  }
  console.log(JSON.stringify({checks: report.checks.length, screenshots: report.screenshots.length, errors: report.errors.length}));
})().catch(error => { console.error(error.message); process.exit(1); });
