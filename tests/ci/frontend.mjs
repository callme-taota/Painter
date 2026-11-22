// Explicit suites only: historical business scripts may require private local
// accounts or write to a running application and must never be auto-discovered.
import { spawnSync } from 'node:child_process'
import { fileURLToPath } from 'node:url'
import { resolve } from 'node:path'

const root = fileURLToPath(new URL('../../', import.meta.url))
const suites = {
  unit: [
    'tests/browser/v24-cards-unit.mjs',
    'tests/browser/v39-cards-unit.mjs',
    'tests/browser/content-panels-unit.mjs',
    'tests/browser/content-layout-unit.mjs',
    'tests/browser/article-links-unit.mjs',
    'tests/browser/article-social-video-unit.mjs',
    'tests/browser/v49-request-coordination.cjs',
    'tests/browser/content-state-unit.cjs',
  ],
  browser: ['tests/browser/toast-core.cjs', 'tests/browser/comment-replies.cjs', 'tests/browser/following-feed.cjs', 'tests/browser/content-managers.cjs'],
}
const group = process.argv[2]
if (!Object.hasOwn(suites, group)) {
  throw new Error('Usage: node tests/ci/frontend.mjs unit|browser')
}
const env = { ...process.env }
if (group === 'browser') {
  const { chromium } = await import('playwright')
  env.PLAYWRIGHT_MODULE = fileURLToPath(new URL('./node_modules/playwright/index.js', import.meta.url))
  env.CHROMIUM_EXECUTABLE ||= chromium.executablePath()
  env.ARTIFACT_DIR ||= resolve(root, 'tests/artifacts/ci/browser')
}
for (const script of suites[group]) {
  console.log(`\nRunning ${script}`)
  const result = spawnSync(process.execPath, [
    '--import', resolve(root, 'tests/ci/register-hooks.mjs'), script,
  ], { cwd: root, env, stdio: 'inherit' })
  if (result.error) throw result.error
  if (result.status !== 0) process.exit(result.status || 1)
}
