import { createHash } from 'node:crypto'
import { resolve } from 'node:path'
import JavaScriptObfuscator from 'javascript-obfuscator'
import { normalizePath, type Plugin } from 'vite'

// Keep module specifiers literal: Rollup and Vite must still discover imports,
// attach lazy CSS and generate hashes from the final transformed module graph.
function importSpecifiers(ast: unknown): string[] {
  const values = new Set<string>()
  function visit(value: unknown) {
    if (!value || typeof value !== 'object') return
    if (Array.isArray(value)) { value.forEach(visit); return }
    const node = value as Record<string, unknown>
    if (['ImportDeclaration', 'ExportNamedDeclaration', 'ExportAllDeclaration', 'ImportExpression'].includes(String(node.type))) {
      const source = node.source as { value?: unknown } | undefined
      if (typeof source?.value === 'string') values.add(source.value)
    }
    Object.values(node).forEach(visit)
  }
  visit(ast)
  return [...values].map(value => '^' + value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&') + '$')
}

export function applicationObfuscation(): Plugin {
  let sourceRoot = ''
  const protectedModules = new Set<string>()
  return {
    name: 'painter-application-obfuscation',
    apply: 'build',
    enforce: 'post',
    configResolved(config) {
      if (config.build.sourcemap) throw Error('Painter obfuscated builds require source maps to stay disabled')
      sourceRoot = normalizePath(resolve(config.root, 'src')) + '/'
    },
    outputOptions(options) {
      if (options.sourcemap) throw Error('Painter obfuscated builds require source maps to stay disabled')
    },
    buildStart() { protectedModules.clear() },
    transform(code, rawID) {
      const id = normalizePath(rawID)
      const [file, query = ''] = id.split('?', 2)
      if (!file.startsWith(sourceRoot) || file.includes('/node_modules/') || !/\.(?:[cm]?js|[cm]?ts|vue)$/.test(file) || file.endsWith('.d.ts')) return null
      const params = new URLSearchParams(query)
      if (params.get('type') === 'style' || ['raw', 'url', 'worker', 'sharedworker'].some(key => params.has(key))) return null
      const relativeID = id.slice(sourceRoot.length)
      const digest = createHash('sha256').update(relativeID).digest('hex')
      const result = JavaScriptObfuscator.obfuscate(code, {
        advertisement: false,
        target: 'browser-no-eval',
        compact: true,
        seed: (Number.parseInt(digest.slice(0, 8), 16) & 0x7fffffff) || 1,
        identifiersPrefix: 'p' + digest.slice(0, 10) + '_',
        identifierNamesGenerator: 'hexadecimal',
        reservedNames: ['^__VITE_', '^__vite_'],
        reservedStrings: importSpecifiers(this.parse(code)),
        ignoreImports: true,
        renameGlobals: false,
        renameProperties: false,
        transformObjectKeys: false,
        stringArray: true,
        stringArrayThreshold: 1,
        stringArrayEncoding: ['base64'],
        stringArrayRotate: false,
        stringArrayShuffle: true,
        stringArrayCallsTransform: false,
        stringArrayWrappersCount: 1,
        stringArrayWrappersChainedCalls: false,
        splitStrings: false,
        controlFlowFlattening: false,
        deadCodeInjection: false,
        debugProtection: false,
        selfDefending: false,
        disableConsoleOutput: false,
        sourceMap: false,
        log: false,
      })
      protectedModules.add(relativeID)
      return { code: result.getObfuscatedCode(), map: null }
    },
    generateBundle() {
      if (protectedModules.size === 0) this.error('No application modules were obfuscated; refusing an unprotected release')
      this.info(`Protected ${protectedModules.size} application modules; third-party dependencies are unchanged`)
    },
  }
}
