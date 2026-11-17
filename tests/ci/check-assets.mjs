// Validate both the checked-in offline bundle and the freshly built CI bundle.
import assert from 'node:assert/strict'
import { existsSync, readFileSync, statSync } from 'node:fs'
import { resolve, relative, isAbsolute } from 'node:path'
import { fileURLToPath } from 'node:url'

const assets = fileURLToPath(new URL('../../web/assets/', import.meta.url))
const manifest = JSON.parse(readFileSync(resolve(assets, '.vite/manifest.json'), 'utf8'))
function checkFile(name) {
  assert.equal(typeof name, 'string', 'Asset paths must be strings')
  const path = resolve(assets, name)
  const local = relative(assets, path)
  assert(!local.startsWith('..') && !isAbsolute(local), `Asset escapes bundle: ${name}`)
  assert(existsSync(path) && statSync(path).isFile(), `Missing embedded asset: ${name}`)
}
for (const entry of ['src/public.ts', 'src/studio.ts', 'src/setup.ts', 'src/sharing.css']) {
  assert(manifest[entry], `Missing application entry: ${entry}`)
}
for (const [key, entry] of Object.entries(manifest)) {
  checkFile(entry.file)
  for (const file of [...(entry.css || []), ...(entry.assets || [])]) checkFile(file)
  for (const dependency of [...(entry.imports || []), ...(entry.dynamicImports || [])]) {
    assert(manifest[dependency], `Unknown dependency ${dependency} in ${key}`)
  }
}
for (const license of ['EDITOR-THIRD-PARTY.txt', 'Geist-OFL.txt', 'Newsreader-OFL.txt']) {
  checkFile(`licenses/${license}`)
}
console.log(`Verified ${Object.keys(manifest).length} manifest entries and font/editor licenses`)
