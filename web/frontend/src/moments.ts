import {initMomentGalleries} from './momentGallery'
import {t, locale} from './i18n'

const readers = new WeakSet<HTMLElement>()
const covers = new WeakSet<HTMLImageElement>()
const returnKey = 'painter-moment-return'
let viewerInitialized = false

function ordinaryClick(event: MouseEvent) {
  return !event.defaultPrevented && event.button === 0 && !event.ctrlKey && !event.metaKey && !event.shiftKey && !event.altKey
}

function initReaders() {
  for (const reader of document.querySelectorAll<HTMLElement>('[data-moment-reader]')) {
    if (readers.has(reader)) continue
    readers.add(reader)
    const gallery = reader.querySelector<HTMLElement>('.moment-slides')
    if (!gallery) continue
    const slides = Array.from(gallery.querySelectorAll<HTMLAnchorElement>('[data-moment-image]'))
    const previous = reader.querySelector<HTMLButtonElement>('[data-moment-previous]')
    const next = reader.querySelector<HTMLButtonElement>('[data-moment-next]')
    const position = reader.querySelector<HTMLOutputElement>('[data-moment-position]')
    let index = 0, frame = 0
    const update = () => {
      frame = 0
      index = Math.max(0, Math.min(slides.length - 1, Math.round(gallery.scrollLeft / Math.max(1, gallery.clientWidth))))
      if (previous) previous.disabled = index === 0
      if (next) next.disabled = index === slides.length - 1
      if (position) position.textContent = `${index + 1} / ${slides.length}`
    }
    const show = (at: number) => {
      const target = Math.max(0, Math.min(slides.length - 1, at))
      gallery.scrollTo({left: target * gallery.clientWidth, behavior: matchMedia('(prefers-reduced-motion:reduce)').matches ? 'instant' : 'smooth'})
    }
    previous?.addEventListener('click', () => show(index - 1))
    next?.addEventListener('click', () => show(index + 1))
    if (previous) previous.hidden = false
    if (next) next.hidden = false
    gallery.addEventListener('scroll', () => { if (!frame) frame = requestAnimationFrame(update) }, {passive: true})
    gallery.addEventListener('keydown', event => {
      if (event.ctrlKey || event.metaKey || event.altKey || !['ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(event.key)) return
      event.preventDefault()
      show(event.key === 'Home' ? 0 : event.key === 'End' ? slides.length - 1 : index + (event.key === 'ArrowRight' ? 1 : -1))
    })
    for (const slide of slides) {
      const image = slide.querySelector('img')
      const failed = () => slide.setAttribute('data-image-failed', '')
      image?.addEventListener('error', failed, {once: true})
      if (image?.complete && !image.naturalWidth) failed()
    }
    update()
  }
  initMomentGalleries()
}

function validListURL(value: string) {
  const url = new URL(value, location.href)
  return url.origin === location.origin && (url.pathname === '/moments' || /^\/users\/[1-9]\d{0,18}$/.test(url.pathname))
}

function initBackLink() {
  const back = document.querySelector<HTMLAnchorElement>('[data-moment-back]')
  if (!back) return
  try {
    const entry = JSON.parse(sessionStorage.getItem(returnKey) || 'null') as {from: string; to: string; at: number} | null
    if (!entry || Date.now() - entry.at > 1800000 || entry.to !== location.pathname || !validListURL(entry.from)) return
    const referrer = new URL(document.referrer)
    const from = new URL(entry.from, location.href)
    if (referrer.origin !== location.origin || referrer.pathname !== from.pathname) return
    back.href = from.pathname + from.search + from.hash
    back.addEventListener('click', event => { if (ordinaryClick(event) && history.length > 1) { event.preventDefault(); history.back() } })
  } catch { /* The real /moments link also works without session storage. */ }
}

/** Desktop reading is an enhancement over the same public, server-rendered URL.
 * Each opening rechecks visibility anonymously; no private endpoint or cache is used. */
function initViewer() {
  if (viewerInitialized || !document.querySelector('[data-moment-open]')) return
  viewerInitialized = true
  const viewerKey = `moment-${Date.now()}-${Math.random().toString(36).slice(2)}`
  const listTitle = document.title
  const originalMetadata = Array.from(document.querySelectorAll<HTMLMetaElement | HTMLLinkElement>('link[rel=canonical],meta[property="og:title"],meta[property="og:description"],meta[property="og:url"],meta[name=description]'))
    .map(element => ({element, attribute: element instanceof HTMLLinkElement ? 'href' : 'content', value: element.getAttribute(element instanceof HTMLLinkElement ? 'href' : 'content') || ''}))
  const dialog = document.createElement('dialog')
  dialog.className = 'moment-viewer'
  dialog.setAttribute('aria-label', t('动态详情'))
  const close = document.createElement('button')
  close.type = 'button'
  close.className = 'moment-viewer-close'
  close.setAttribute('aria-label', t('关闭动态'))
  close.innerHTML = '<svg class="icon" viewBox="0 0 24 24" aria-hidden="true"><path d="m6 6 12 12M18 6 6 18"/></svg>'
  const body = document.createElement('div')
  body.className = 'moment-viewer-body'
  dialog.append(close, body)
  document.body.append(dialog)
  let opener: HTMLAnchorElement | null = null, currentID = '', controller: AbortController | null = null, sequence = 0, active = false
  const state = () => history.state && typeof history.state === 'object' ? history.state : {}
  const owns = () => state().painterMoment?.viewer === viewerKey
  const restoreMetadata = () => {
    document.title = listTitle
    for (const {element, attribute, value} of originalMetadata) element.setAttribute(attribute, value)
  }
  function closeReaderLayers() {
    // These layers can only be reached through the active, modal reader.
    // Close them before removing their opener or restoring the list URL.
    document.querySelectorAll<HTMLDialogElement>('.sharing-dialog[open],.moment-lightbox[open]').forEach(layer => layer.close())
  }
  function hide() {
    if (!active) return
    closeReaderLayers()
    active = false
    currentID = ''
    sequence++
    controller?.abort()
    document.documentElement.classList.remove('moment-viewer-open')
    if (dialog.open) dialog.close()
    body.replaceChildren()
    restoreMetadata()
    opener?.isConnected && opener.focus({preventScroll: true})
    document.dispatchEvent(new Event('painter:moment-viewer-closed'))
  }
  function dismiss() {
    if (owns()) history.back()
    else hide()
  }
  function status(message: string, retry: boolean) {
    body.replaceChildren()
    const wrap = document.createElement('div')
    wrap.className = 'moment-viewer-status'
    const text = document.createElement('p')
    text.setAttribute('role', 'status')
    text.textContent = message
    wrap.append(text)
    if (retry) {
      const button = document.createElement('button')
      button.type = 'button'
      button.textContent = t('重试')
      button.addEventListener('click', () => void load(currentID))
      wrap.append(button)
    }
    const link = document.createElement('a')
    link.href = `/moments/${currentID}?lang=${locale}`
    link.textContent = t('在独立页面打开')
    wrap.append(link)
    body.append(wrap)
  }
  async function load(id: string) {
    closeReaderLayers()
    controller?.abort()
    controller = new AbortController()
    const request = ++sequence
    currentID = id
    restoreMetadata()
    status(t('正在读取动态…'), false)
    body.setAttribute('aria-busy', 'true')
    try {
      const response = await fetch(`/moments/${id}?lang=${locale}`, {credentials: 'omit', cache: 'no-store', redirect: 'error', signal: controller.signal, headers: {Accept: 'text/html', 'Accept-Language': locale}})
      if (!response.ok) {
        if (request !== sequence || !active) return
        status(response.status === 404 ? t('这条动态已撤回、删除或尚未公开。') : t('动态暂时无法加载，请重试。'), response.status !== 404)
        return
      }
      const doc = new DOMParser().parseFromString(await response.text(), 'text/html')
      const reader = doc.querySelector<HTMLElement>('[data-moment-reader]')
      if (doc.body.dataset.pageKind !== 'moment' || !reader || reader.dataset.momentId !== id) throw Error('Invalid public moment')
      if (request !== sequence || !active) return
      body.replaceChildren(document.importNode(reader, true))
      document.title = doc.title
      for (const {element, attribute} of originalMetadata) {
        const selector = element instanceof HTMLLinkElement ? 'link[rel=canonical]' : element.hasAttribute('property') ? `meta[property="${element.getAttribute('property')}"]` : 'meta[name=description]'
        const value = doc.querySelector(selector)?.getAttribute(attribute)
        if (value) element.setAttribute(attribute, value)
      }
      initReaders()
    } catch {
      if (request === sequence && active) status(t('动态暂时无法加载，请重试。'), true)
    } finally {
      if (request === sequence) body.removeAttribute('aria-busy')
    }
  }
  function open(id: string, trigger?: HTMLAnchorElement) {
    opener = trigger || document.querySelector<HTMLAnchorElement>(`[data-moment-open][href="/moments/${id}"]`)
    active = true
    document.documentElement.classList.add('moment-viewer-open')
    if (!dialog.open) dialog.showModal()
    close.focus({preventScroll: true})
    void load(id)
  }
  document.addEventListener('click', event => {
    const link = event.target instanceof Element ? event.target.closest<HTMLAnchorElement>('a[data-moment-open]') : null
    if (!link || !ordinaryClick(event) || link.target || link.hasAttribute('download')) return
    const url = new URL(link.href, location.href)
    const match = url.origin === location.origin && /^\/moments\/([1-9]\d{0,18})$/.exec(url.pathname)
    if (!match) return
    if (!matchMedia('(min-width:721px)').matches || !dialog.showModal) {
      try { sessionStorage.setItem(returnKey, JSON.stringify({from: location.href, to: url.pathname, at: Date.now()})) } catch { /* Optional return hint. */ }
      return
    }
    event.preventDefault()
    history.pushState({...state(), painterMoment: {viewer: viewerKey, id: match[1]}}, '', url.pathname + (locale === 'en' ? '?lang=en' : ''))
    open(match[1]!, link)
  })
  close.addEventListener('click', dismiss)
  dialog.addEventListener('cancel', event => { event.preventDefault(); dismiss() })
  dialog.addEventListener('close', () => { if (active) dismiss() })
  let backdropPress = false
  dialog.addEventListener('pointerdown', event => { backdropPress = event.target === dialog && event.button === 0 })
  dialog.addEventListener('pointercancel', () => { backdropPress = false })
  dialog.addEventListener('click', event => {
    const rect = dialog.getBoundingClientRect()
    if (backdropPress && event.target === dialog && (event.clientX < rect.left || event.clientX > rect.right || event.clientY < rect.top || event.clientY > rect.bottom)) dismiss()
    backdropPress = false
  })
  addEventListener('popstate', () => {
    const value = state().painterMoment
    if (value?.viewer === viewerKey && typeof value.id === 'string' && /^[1-9]\d{0,18}$/.test(value.id)) open(value.id)
    else hide()
  })
  // Never retain a published snapshot inside the browser's back/forward cache.
  addEventListener('pagehide', () => {
    if (!active) return
    closeReaderLayers()
    controller?.abort()
    sequence++
    body.replaceChildren()
  })
  addEventListener('pageshow', event => {
    if (!event.persisted) return
    const value = state().painterMoment
    if (owns() && typeof value.id === 'string' && /^[1-9]\d{0,18}$/.test(value.id)) open(value.id)
    else hide()
  })
}

export function initMoments() {
  for (const image of document.querySelectorAll<HTMLImageElement>('.moment-cover')) {
    if (covers.has(image)) continue
    covers.add(image)
    const failed = () => image.closest('.moment-preview')?.setAttribute('data-cover-failed', '')
    image.addEventListener('error', failed, {once: true})
    if (image.complete && !image.naturalWidth) failed()
  }
  initReaders()
  initViewer()
  initBackLink()
}
