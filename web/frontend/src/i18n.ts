/** The server selects the locale before rendering. Language changes navigate
 * through the server, so public scripts and Vue always share one locale. */
export const locale: 'zh-CN' | 'en' = typeof document !== 'undefined' && /^en(?:-|$)/i.test(document.documentElement.lang) ? 'en' : 'zh-CN'
// Chinese pages do not download the English catalogue. Module evaluation waits
// only when English is selected, keeping t synchronous for every caller. If the
// optional chunk cannot load, source messages keep controls usable until reload.
const messages: Readonly<Record<string, string>> = locale === 'en'
  ? await import('../../locales/en.json').then(module => module.default).catch(() => ({}))
  : {}
export type MessageValues = Record<string, string | number>

/** Translate interface copy only. Callers still use textContent or framework
 * escaping for interpolated values; article and profile content stays intact. */
export function t(source: string, values?: MessageValues): string {
  const message = locale === 'en' && Object.hasOwn(messages, source) ? messages[source]! : source
  if (!values) return message
  return message.replace(/\{([A-Za-z][A-Za-z0-9_]*)\}/g, (placeholder, key: string) => Object.hasOwn(values, key) ? String(values[key]) : placeholder)
}

export function formatDate(value: Date | string | number, options: Intl.DateTimeFormatOptions = {year: 'numeric', month: 'short', day: 'numeric'}): string {
  const date = value instanceof Date ? value : new Date(value)
  return Number.isNaN(date.valueOf()) ? String(value) : new Intl.DateTimeFormat(locale, {timeZone: 'Asia/Shanghai', ...options}).format(date)
}

export function formatNumber(value: number, options?: Intl.NumberFormatOptions): string {
  return new Intl.NumberFormat(locale, options).format(value)
}

/** The server cannot see URL fragments. Refresh the ordinary language links
 * before navigation, including links mounted later by the account interface.
 * Native navigation keeps the browser's unsaved-changes protection intact. */
export function initLocaleLinks(): void {
  if (document.documentElement.hasAttribute('data-locale-links-ready')) return
  document.documentElement.setAttribute('data-locale-links-ready', '')
  function update(link: HTMLAnchorElement): void {
    const target = new URL(link.href, location.href)
    if (target.origin !== location.origin || target.pathname !== '/locale') return
    const current = new URL(location.href)
    current.searchParams.delete('lang')
    target.searchParams.set('next', current.pathname + current.search + current.hash)
    link.setAttribute('href', target.pathname + target.search + target.hash)
  }
  const refresh = () => document.querySelectorAll<HTMLAnchorElement>('a[data-locale-switch]').forEach(update)
  const beforeNavigation = (event: Event) => {
    const link = event.target instanceof Element ? event.target.closest<HTMLAnchorElement>('a[data-locale-switch]') : null
    if (link) update(link)
  }
  refresh()
  addEventListener('hashchange', refresh)
  addEventListener('popstate', refresh)
  addEventListener('pageshow', refresh)
  document.addEventListener('pointerdown', beforeNavigation, true)
  document.addEventListener('focusin', beforeNavigation, true)
  document.addEventListener('click', beforeNavigation, true)
}
