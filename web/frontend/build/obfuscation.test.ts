import assert from 'node:assert/strict'
import { createHash } from 'node:crypto'
import { mkdtemp, mkdir, readFile, readdir, rm, writeFile } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { pathToFileURL } from 'node:url'
import { test } from 'node:test'
import { build, createServer, type Plugin } from 'vite'
import { applicationObfuscation } from './obfuscation.ts'

const vendorMarker = 'VENDOR_SHOULD_REMAIN_LITERAL'
const privateMarker = 'PERSONAL_SOURCE_MARKER_SHOULD_BE_OBSCURED'

async function fixture() {
  const root = await mkdtemp(join(tmpdir(), 'painter-obfuscation-'))
  await mkdir(join(root, 'src'), { recursive: true })
  await mkdir(join(root, 'node_modules/fixture-vendor'), { recursive: true })
  await writeFile(join(root, 'package.json'), JSON.stringify({ type: 'module' }))
  await writeFile(join(root, 'node_modules/fixture-vendor/package.json'), JSON.stringify({
    name: 'fixture-vendor', version: '1.0.0', type: 'module', exports: './index.js',
  }))
  await writeFile(join(root, 'node_modules/fixture-vendor/index.js'),
    `export function vendorRead() { return '${vendorMarker}' }`)
  await writeFile(join(root, 'src/shared+data.ts'), `
    export const apiContract = { article_id: 'post-41', api_endpoint: '/api/v1/posts' }
    export function originalBusinessName(input: number) { return input + 41 }
  `)
  await writeFile(join(root, 'src/lazy+data.ts'), `
    export { apiContract } from './shared+data.ts'
    export const lazyValue = 'lazy-result'
  `)
  await writeFile(join(root, 'src/lazy-view.css'), '.fixture-view { color: rgb(1, 2, 3) }')
  await writeFile(join(root, 'src/lazy-view.ts'), `
    import './lazy-view.css'
    export const viewName = 'view-with-css'
  `)
  await writeFile(join(root, 'src/main.ts'), `
    import { originalBusinessName } from './shared+data.ts'
    export { apiContract } from './shared+data.ts'
    export { vendorRead } from 'fixture-vendor'
    export const privateText = '${privateMarker}'
    export function evaluate(input: number) { return originalBusinessName(input) }
    export async function loadLazy() { return import('./lazy+data.ts') }
    export async function loadView() { return import('./lazy-view.ts') }
  `)
  return root
}

async function compile(root: string, plugin: Plugin = applicationObfuscation()) {
  await build({
    configFile: false,
    root,
    base: '/assets/',
    logLevel: 'silent',
    plugins: [plugin],
    build: {
      outDir: 'dist', emptyOutDir: true, sourcemap: false, minify: 'esbuild', manifest: true,
      rollupOptions: {
        input: join(root, 'src/main.ts'),
        preserveEntrySignatures: 'strict',
        output: { entryFileNames: '[name]-[hash].js', chunkFileNames: 'chunks/[name]-[hash].js' },
      },
    },
  })
  return JSON.parse(await readFile(join(root, 'dist/.vite/manifest.json'), 'utf8')) as Record<string, {
    file: string; src?: string; isEntry?: boolean; isDynamicEntry?: boolean; css?: string[];
  }>
}

async function outputDigests(root: string) {
  const files = await readdir(join(root, 'dist'), { recursive: true, withFileTypes: true })
  const digests: Record<string, string> = {}
  for (const file of files) {
    if (!file.isFile()) continue
    const absolute = join(file.parentPath, file.name)
    const relative = absolute.slice(join(root, 'dist').length + 1)
    digests[relative] = createHash('sha256').update(await readFile(absolute)).digest('hex')
  }
  return digests
}

test('production preserves ESM exports, dynamic imports, API properties and lazy CSS', async t => {
  const root = await fixture()
  t.after(() => rm(root, { recursive: true, force: true }))
  const plugin = applicationObfuscation()
  const originalTransform = plugin.transform
  assert.equal(typeof originalTransform, 'function')
  let sawVendor = false
  plugin.transform = function (code, id) {
    const result = (originalTransform as Function).call(this, code, id)
    if (id.includes('/node_modules/fixture-vendor/')) {
      sawVendor = true
      assert.equal(result, null, 'vendor code must pass through without obfuscation')
    }
    return result
  }
  const manifest = await compile(root, plugin)
  const entry = manifest['src/main.ts']
  assert.ok(entry?.isEntry)
  const module = await import(pathToFileURL(join(root, 'dist', entry.file)).href)
  assert.equal(module.evaluate(1), 42)
  assert.deepEqual(module.apiContract, { article_id: 'post-41', api_endpoint: '/api/v1/posts' })
  assert.equal(module.privateText, privateMarker)
  assert.equal(module.vendorRead(), vendorMarker)
  const lazy = await module.loadLazy()
  assert.equal(lazy.lazyValue, 'lazy-result')
  assert.deepEqual(lazy.apiContract, module.apiContract)
  assert.equal(sawVendor, true)

  const lazyView = manifest['src/lazy-view.ts']
  assert.ok(lazyView?.isDynamicEntry)
  assert.equal(lazyView.css?.length, 1, 'lazy view keeps its CSS association')
  const mainCode = await readFile(join(root, 'dist', entry.file), 'utf8')
  assert.ok(mainCode.includes(lazyView.css![0]), 'preload retains the hashed lazy CSS URL')
  assert.ok(!mainCode.includes(privateMarker), 'personal source strings are encoded in release assets')
  const output = await outputDigests(root)
  for (const file of Object.keys(output).filter(file => file.endsWith('.js'))) {
    const code = await readFile(join(root, 'dist', file), 'utf8')
    assert.doesNotMatch(code, /\beval\s*\(|\bnew\s+Function\s*\(/)
    assert.doesNotMatch(code, /__VITE_|sourceMappingURL|!~\{/)
  }
  assert.equal(Object.keys(output).some(file => file.endsWith('.map')), false)
})

test('release files are stable across builds and checkout paths; changes invalidate linked hashes', async t => {
  const first = await fixture()
  const second = await fixture()
  t.after(() => Promise.all([first, second].map(root => rm(root, { recursive: true, force: true }))))
  const before = await compile(first)
  const beforeDigests = await outputDigests(first)
  await compile(first)
  assert.deepEqual(await outputDigests(first), beforeDigests)
  await compile(second)
  assert.deepEqual(await outputDigests(second), beforeDigests, 'absolute checkout paths must not enter seeds')
  const source = join(first, 'src/lazy+data.ts')
  await writeFile(source, (await readFile(source, 'utf8')).replace('lazy-result', 'lazy-result-changed'))
  const after = await compile(first)
  assert.notEqual(after['src/lazy+data.ts'].file, before['src/lazy+data.ts'].file)
  assert.notEqual(after['src/main.ts'].file, before['src/main.ts'].file)
})

test('development remains readable and does not activate the release obfuscator', async t => {
  const root = await fixture()
  t.after(() => rm(root, { recursive: true, force: true }))
  const server = await createServer({
    configFile: false, root, logLevel: 'silent', plugins: [applicationObfuscation()],
    server: { middlewareMode: true, watch: null },
    optimizeDeps: { noDiscovery: true, include: [] },
  })
  t.after(() => server.close())
  assert.equal(server.config.plugins.some(plugin => plugin.name === 'painter-application-obfuscation'), false)
  const result = await server.transformRequest('/src/main.ts')
  assert.ok(result?.code.includes(privateMarker))
})

test('release refuses source maps and builds containing no protected application module', async t => {
  const root = await fixture()
  t.after(() => rm(root, { recursive: true, force: true }))
  await assert.rejects(build({
    configFile: false, root, logLevel: 'silent', plugins: [applicationObfuscation()],
    build: { sourcemap: true, rollupOptions: { input: join(root, 'src/main.ts') } },
  }), /source maps to stay disabled/)
  await assert.rejects(build({
    configFile: false, root, logLevel: 'silent', plugins: [applicationObfuscation()],
    build: { write: false, sourcemap: false, rollupOptions: { input: join(root, 'src/main.ts'), output: { sourcemap: true } } },
  }), /source maps to stay disabled/)
  await writeFile(join(root, 'outside-src.js'), 'export const unprotected = true')
  await assert.rejects(build({
    configFile: false, root, logLevel: 'silent', plugins: [applicationObfuscation()],
    build: { write: false, sourcemap: false, rollupOptions: { input: join(root, 'outside-src.js') } },
  }), /No application modules were obfuscated/)
})
