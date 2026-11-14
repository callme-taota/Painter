type TOCLink = {nav: HTMLElement; link: HTMLAnchorElement}
type TOCEntry = {heading: HTMLElement; links: TOCLink[]}

const initialized = new WeakSet<HTMLElement>()

/** Follow the article body without changing its native hash navigation. */
export function initArticleTOC() {
 if (document.body.dataset.pageKind !== 'article') return
 const aside = document.querySelector<HTMLElement>('[data-article-toc]')
 const nav = aside?.querySelector<HTMLElement>('[data-article-toc-scroll]')
 const prose = document.querySelector<HTMLElement>('.reading-paper > .prose')
 if (!aside || !nav || !prose || initialized.has(aside)) return
 const mobileNav = document.querySelector<HTMLElement>('#mobile-more [data-mobile-article-toc-scroll]')
 const navs = mobileNav ? [nav, mobileNav] : [nav]

 const links = new Map<string, TOCLink[]>()
 for (const nav of navs) for (const link of nav.querySelectorAll<HTMLAnchorElement>('a[data-toc-link]')) {
  const href = link.getAttribute('href')
  if (!href?.startsWith('#')) continue
  try {
   const id = decodeURIComponent(href.slice(1))
   const targets = links.get(id) || []
   targets.push({nav, link})
   links.set(id, targets)
  } catch { /* A malformed fragment must not disable the remaining entries. */ }
 }
 const entries: TOCEntry[] = []
 for (const heading of prose.querySelectorAll<HTMLElement>('h1[id],h2[id],h3[id],h4[id],h5[id],h6[id]')) {
  const targets = links.get(heading.id)
  if (targets) entries.push({heading, links: targets})
 }
 if (!entries.length) return
 initialized.add(aside)
 aside.dataset.tocReady = 'true'

 let positions: number[] = []
 let offset = 128
 let maxScroll = 0
 let active = -1
 let frame = 0
 let needsMeasure = true
 const manualNavUntil = new WeakMap<HTMLElement, number>()

 const keepVisible = (entry: TOCEntry) => { for (const {nav, link} of entry.links) {
  const focused = document.activeElement
  const keyboardFocus = focused instanceof HTMLElement && nav.contains(focused) && focused.matches(':focus-visible')
  if (nav.clientHeight === 0 || !nav.getClientRects().length || nav.scrollHeight <= nav.clientHeight ||
      keyboardFocus || performance.now() < (manualNavUntil.get(nav) || 0)) continue
  const viewport = nav.getBoundingClientRect()
  const item = link.getBoundingClientRect()
  const top = viewport.top + nav.clientTop + 8
  const bottom = viewport.top + nav.clientTop + nav.clientHeight - 8
  // Only this scroll container moves; scrollIntoView could also move the page.
  if (item.top < top) nav.scrollTop += item.top - top
  else if (item.bottom > bottom) nav.scrollTop += item.bottom - bottom
 } }

 const update = () => {
  frame = 0
  const measured = needsMeasure
  if (needsMeasure) {
   needsMeasure = false
   const scrollY = window.scrollY
   positions = entries.map(({heading}) => heading.getBoundingClientRect().top + scrollY)
   const cssOffset = Number.parseFloat(getComputedStyle(document.documentElement).getPropertyValue('--article-toc-offset'))
   offset = Number.isFinite(cssOffset) && cssOffset >= 0 ? cssOffset : 128
   maxScroll = Math.max(0, document.documentElement.scrollHeight - window.innerHeight)
  }
  const activationLine = window.scrollY + offset + 1
  let low = 0
  let high = positions.length
  while (low < high) {
   const middle = (low + high) >>> 1
   if (positions[middle]! <= activationLine) low = middle + 1
   else high = middle
  }
  // A short final section may never reach the activation line at page bottom.
  const next = maxScroll > 0 && window.scrollY >= maxScroll - 2
   ? entries.length - 1 : Math.max(0, low - 1)
  if (next === active) {
   if (measured) keepVisible(entries[active]!)
   return
  }
  if (active >= 0) {
   for (const {link} of entries[active]!.links) {
    link.removeAttribute('aria-current')
    delete link.dataset.tocActive
   }
  }
  active = next
  for (const {link} of entries[active]!.links) {
   link.setAttribute('aria-current', 'location')
   link.dataset.tocActive = 'true'
  }
  keepVisible(entries[active]!)
 }

 const schedule = () => { if (!frame) frame = requestAnimationFrame(update) }
 const invalidate = () => { needsMeasure = true; schedule() }
 window.addEventListener('scroll', schedule, {passive: true})
 window.addEventListener('resize', invalidate, {passive: true})
 window.addEventListener('pageshow', invalidate)
 window.addEventListener('hashchange', invalidate)
 window.addEventListener('load', invalidate, {once: true})
 for (const nav of navs) {
  const manualNav = () => { manualNavUntil.set(nav, performance.now() + 700) }
  for (const event of ['wheel', 'touchstart', 'touchmove', 'pointerdown', 'keydown']) {
   nav.addEventListener(event, manualNav, {passive: true})
  }
 }
 if (mobileNav) {
  const dialog = mobileNav.closest<HTMLDialogElement>('dialog')
  const details = mobileNav.closest<HTMLDetailsElement>('details')
  const reveal = (event?: Event) => {
   if (event && event.target !== dialog && event.target !== details) return
   if (dialog?.open && details?.open) manualNavUntil.delete(mobileNav)
   invalidate()
  }
  dialog?.addEventListener('toggle', reveal, true)
  dialog?.addEventListener('close', reveal, true)
  document.querySelector('[data-mobile-more-open]')?.addEventListener('click', () => reveal())
 }
 prose.addEventListener('load', invalidate, true)
 prose.addEventListener('error', invalidate, true)
 if ('ResizeObserver' in window) {
  const observer = new ResizeObserver(invalidate)
  observer.observe(document.body)
  observer.observe(prose)
 }
 if (document.fonts) {
  void document.fonts.ready.then(invalidate)
  document.fonts.addEventListener('loadingdone', invalidate)
 }
 schedule()
}
