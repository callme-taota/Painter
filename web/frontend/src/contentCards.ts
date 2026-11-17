import {t,locale,formatDate,formatNumber} from './i18n'
import './articleCards.css'
import './researchCards.css'
import {initImageGalleries} from './momentGallery'
import {initContentVideos} from './contentVideos'
import {socialPlatform} from './articleEmbeds'

type LinkPreview = {
  wrapper: HTMLElement
  anchor: HTMLAnchorElement
  preview: HTMLElement
  hint: HTMLElement
  touchArmed: boolean
  pointerType: string
  closeTimer?: number
}
const previews = new WeakMap<HTMLElement, LinkPreview>()
const internalRequests = new WeakSet<HTMLElement>()
let active: LinkPreview | undefined
let globalHandlers = false
let positionFrame = 0
let previewID = 0

function dismiss(state: LinkPreview) {
  if (state.closeTimer) clearTimeout(state.closeTimer)
  state.closeTimer = undefined
  state.wrapper.removeAttribute('data-preview-open')
  state.wrapper.dataset.previewDismissed = 'true'
  state.anchor.setAttribute('aria-expanded', 'false')
  state.preview.setAttribute('aria-hidden', 'true')
  state.hint.hidden = true
  state.touchArmed = false
  if (active === state) active = undefined
}

function positionPreview() {
  positionFrame = 0
  if (!active) return
  const {wrapper, preview, anchor} = active
  if (!wrapper.isConnected) { dismiss(active); return }
  preview.style.setProperty('--content-preview-shift', '0px')
  preview.style.setProperty('--content-preview-shift-y', '0px')
  wrapper.removeAttribute('data-preview-below')
  let box = preview.getBoundingClientRect()
  const link = anchor.getBoundingClientRect()
  // On mobile, an overflowing absolute child can enlarge window.innerWidth.
  // The document's client viewport remains the actual CSS viewing boundary.
  const width = document.documentElement.clientWidth || innerWidth
  const height = document.documentElement.clientHeight || innerHeight
  if (link.bottom < 0 || link.top > height || link.right < 0 || link.left > width) { dismiss(active); return }
  if (box.top < 16 && height - link.bottom > link.top) {
    wrapper.dataset.previewBelow = 'true'
    box = preview.getBoundingClientRect()
  }
  const x = box.left < 16 ? 16 - box.left : box.right > width - 16 ? width - 16 - box.right : 0
  const y = box.top < 16 ? 16 - box.top : box.bottom > height - 16 ? height - 16 - box.bottom : 0
  preview.style.setProperty('--content-preview-shift', `${Math.round(x)}px`)
  preview.style.setProperty('--content-preview-shift-y', `${Math.round(y)}px`)
}
function schedulePosition() {
  if (active && !positionFrame) positionFrame = requestAnimationFrame(positionPreview)
}
function reveal(state: LinkPreview) {
  if (state.closeTimer) clearTimeout(state.closeTimer)
  state.closeTimer = undefined
  if (active && active !== state) dismiss(active)
  active = state
  state.wrapper.removeAttribute('data-preview-dismissed')
  state.wrapper.dataset.previewOpen = 'true'
  state.anchor.setAttribute('aria-expanded', 'true')
  state.preview.setAttribute('aria-hidden', 'false')
  positionPreview()
}
function installGlobalHandlers() {
  if (globalHandlers) return
  globalHandlers = true
  document.addEventListener('pointerdown', event => {
    if (active && !event.composedPath().includes(active.wrapper)) dismiss(active)
  }, true)
  document.addEventListener('keydown', event => {
    if (event.key !== 'Escape' || event.defaultPrevented || !active) return
    if (!active.anchor.isContentEditable) event.preventDefault()
    dismiss(active)
  })
  document.addEventListener('scroll', schedulePosition, {capture: true, passive: true})
  window.addEventListener('resize', schedulePosition, {passive: true})
  window.addEventListener('blur', () => { if (active) dismiss(active) })
}
function enhanceInlinePreview(wrapper: HTMLElement) {
  if (previews.has(wrapper)) return
  const anchor = wrapper.querySelector<HTMLAnchorElement>(':scope > a.content-link__anchor')
  const preview = wrapper.querySelector<HTMLElement>(':scope > .content-link__preview')
  if (!anchor || !preview) return
  installGlobalHandlers()
  const oldPreviewID = preview.id
  if (!preview.id || document.getElementById(preview.id) !== preview) {
    let id: string
    do { id = `content-link-preview-${++previewID}` } while (document.getElementById(id))
    preview.id = id
  }
  preview.setAttribute('role', 'tooltip')
  preview.setAttribute('aria-hidden', 'true')
  // Horizontal transforms retain the original overflow extent on mobile.
  // Move the actual inset so an open popup cannot widen the visual viewport.
  preview.style.insetInlineStart = 'auto'
  preview.style.left = 'var(--content-preview-shift, 0px)'
  preview.style.transform = 'translateY(var(--content-preview-shift-y, 0px))'
  anchor.setAttribute('aria-controls', preview.id)
  anchor.setAttribute('aria-expanded', 'false')
  const describedBy = (anchor.getAttribute('aria-describedby') || '').split(/\s+/)
    .filter(id => id && (id !== oldPreviewID || oldPreviewID === preview.id))
  if (!describedBy.includes(preview.id)) describedBy.push(preview.id)
  anchor.setAttribute('aria-describedby', describedBy.join(' '))
  wrapper.removeAttribute('data-preview-open')
  wrapper.removeAttribute('data-preview-below')
  wrapper.dataset.previewDismissed = 'true'
  const hint = preview.querySelector<HTMLElement>(':scope > .content-link__hint') || document.createElement('span')
  hint.className = 'content-link__hint'
  hint.textContent = t('再次点按链接可打开')
  hint.hidden = true
  if (hint.parentElement !== preview) preview.append(hint)
  const state: LinkPreview = {wrapper, anchor, preview, hint, touchArmed: false, pointerType: ''}
  previews.set(wrapper, state)
  wrapper.addEventListener('pointerenter', event => {
    state.pointerType = event.pointerType
    if (event.pointerType !== 'touch') reveal(state)
  })
  wrapper.addEventListener('pointerleave', event => {
    if (event.pointerType !== 'touch' && !wrapper.contains(document.activeElement)) {
      // A scrollable preview clips its CSS hover bridge. Give the pointer time
      // to cross the ten-pixel gap without dropping the preview mid-gesture.
      if (state.closeTimer) clearTimeout(state.closeTimer)
      state.closeTimer = window.setTimeout(() => {
        if (!wrapper.matches(':hover') && !wrapper.contains(document.activeElement)) dismiss(state)
      }, 200)
    }
  })
  wrapper.addEventListener('focusin', () => reveal(state))
  wrapper.addEventListener('focusout', event => {
    if (event.relatedTarget instanceof Node && wrapper.contains(event.relatedTarget)) return
    if (state.pointerType !== 'touch' && wrapper.matches(':hover')) return
    dismiss(state)
  })
  anchor.addEventListener('pointerdown', event => { state.pointerType = event.pointerType })
  anchor.addEventListener('click', event => {
    if (event.defaultPrevented || event.button !== 0 || event.ctrlKey || event.metaKey || event.shiftKey || event.altKey || event.detail === 0 || anchor.isContentEditable) return
    const pointer = (event as PointerEvent).pointerType || state.pointerType
    const touch = pointer === 'touch' || (!pointer && matchMedia('(hover: none)').matches)
    if (!touch) return
    if (state.touchArmed && active === state && wrapper.dataset.previewOpen === 'true') return
    event.preventDefault()
    reveal(state)
    state.touchArmed = true
    hint.hidden = false
    schedulePosition()
  })
}

function enhanceArticleImages(root: ParentNode) {
  const articles = within(root, 'article.article-reading .prose')
    .filter(article => article.closest('body')?.getAttribute('data-page-kind') === 'article' && !article.isContentEditable)
  for (const article of articles) {
    for (const image of article.querySelectorAll<HTMLImageElement>('img')) {
      if (image.closest('.content-card,.content-link,.content-social,.content-video,button,[contenteditable="true"]')) continue
      const source = image.getAttribute('src')
      if (!source) continue
      let target: URL
      try { target = new URL(source, document.baseURI) } catch { continue }
      if (target.protocol !== 'https:' && target.protocol !== 'http:') continue
      let link = image.closest<HTMLAnchorElement>('a')
      // Linked illustrations may be navigation or provider cards; retain that
      // destination unless the author explicitly linked this exact image.
      if (link && link.href !== target.href) continue
      if (!link) {
        link = document.createElement('a')
        link.href = target.href
        image.before(link)
        link.append(image)
      }
      link.classList.add('article-image-link')
      link.dataset.articleImage = ''
      link.dataset.imageAlt = image.alt
      if (!image.alt && !link.getAttribute('aria-label')) link.setAttribute('aria-label', t('查看文章图片'))
    }
  }
  initImageGalleries(articles, {linkSelector: 'a[data-article-image]', title: t('文章图片')})
}

function within(root: ParentNode, selector: string): HTMLElement[] {
  const elements = Array.from(root.querySelectorAll<HTMLElement>(selector))
  if (root instanceof HTMLElement && root.matches(selector)) elements.unshift(root)
  return elements
}

// Internal references are fetched only from this fixed same-origin public API.
// No snapshot body, provider request, session permission or cross-render cache
// can make a withdrawn moment/phrase remain visible in the document editor.
export function initInternalArticleCards(root: ParentNode, signal?: AbortSignal) {
  for (const anchor of within(root, 'a.content-card--moment,a.content-card--phrase')) {
    if (!anchor.isConnected) continue
    if (['ready','error'].includes(anchor.dataset.internalCardState || '') || internalRequests.has(anchor)) continue
    const href = anchor.getAttribute('href') || ''
    const match = /^\/(moments|phrases)\/([1-9][0-9]{0,18})$/.exec(href)
    if (!match || BigInt(match[2]) > 9223372036854775807n) continue
    const kind = match[1] === 'moments' ? 'moment' : 'phrase'
    if (!anchor.classList.contains('content-card--' + kind)) continue
    internalRequests.add(anchor)
    anchor.setAttribute('aria-busy', 'true')
    void fetch('/api/v1/content-cards/' + kind + '/' + match[2], {credentials:'omit', cache:'no-store', redirect:'error', signal, headers:{'Accept-Language':locale}})
      .then(async response => {
        if (signal?.aborted || !anchor.isConnected) return
        if (response.status === 404) { internalCardUnavailable(anchor, kind, false); return }
        if (!response.ok) throw new Error('unavailable')
        const value = await response.json()
        if (signal?.aborted || !anchor.isConnected) return
        const reference = value?.reference
        if (!reference || reference.id !== match[2] || reference.type !== kind || reference.url !== href || typeof reference.text !== 'string') throw new Error('invalid reference')
        const body = document.createElement('span'); body.className = 'content-card__body'
        const add = (name:string, text:string) => { const node = document.createElement('span'); node.className = name; node.textContent = text; body.append(node); return node }
        if (kind === 'phrase') {
          add('content-card__kind', reference.kind === 'word' ? t('词语') : t('句子'))
          add('content-card__quote', reference.text)
          if (typeof reference.source === 'string' && reference.source) add('content-card__attribution', reference.source)
        } else {
          const identity = document.createElement('span'); identity.className = 'content-card__identity'
          const author = reference.author || {}
          if (typeof author.avatar_url === 'string' && /^\/media\/[a-fA-F0-9-]{16,128}$/.test(author.avatar_url)) {
            const avatar = document.createElement('img'); avatar.className = 'content-card__avatar'; avatar.src = author.avatar_url; avatar.alt = ''; avatar.loading = 'lazy'; identity.append(avatar)
          }
          const name = document.createElement('span'); name.className = 'content-card__author'; name.textContent = typeof author.display_name === 'string' ? author.display_name : ''; identity.append(name)
          if (typeof reference.published_at === 'string' && !Number.isNaN(Date.parse(reference.published_at))) {
            const date = document.createElement('time'); date.className = 'content-card__date'; date.dateTime = reference.published_at; date.textContent = formatDate(reference.published_at); identity.append(date)
          }
          body.append(identity)
          if (reference.text) add('content-card__text', reference.text)
          const images = Array.isArray(reference.images) ? reference.images.filter((item:unknown):item is {url:string;alt?:string} => !!item && typeof item === 'object' && 'url' in item && typeof item.url === 'string' && /^\/media\/[a-fA-F0-9-]{16,128}$/.test(item.url)).slice(0,9) : []
          if (images.length) {
            const photos = document.createElement('span'); photos.className = 'content-card__photos'
            for (const item of images.slice(0,3)) { const image = document.createElement('img'); image.className = 'content-card__photo'; image.src = item.url; image.alt = typeof item.alt === 'string' ? item.alt : ''; image.loading = 'lazy'; photos.append(image) }
            body.append(photos)
            if (images.length > 3) add('content-card__photo-count', t('共{count}张图片',{count:formatNumber(images.length)}))
          }
        }
        add('content-card__destination', kind === 'moment' ? t('查看动态') : t('查看片语'))
        anchor.replaceChildren(body)
        anchor.dataset.internalCardState = 'ready'
        anchor.removeAttribute('aria-busy')
      }).catch(error => {
        if (signal?.aborted || (error as Error).name === 'AbortError' || !anchor.isConnected) return
        internalCardUnavailable(anchor, kind, true)
      }).finally(() => { if (signal?.aborted || !anchor.isConnected) internalRequests.delete(anchor) })
  }
}

function internalCardUnavailable(anchor:HTMLElement, kind:string, retryable:boolean) {
  const replacement = document.createElement(retryable ? 'a' : 'span')
  if (anchor.id) replacement.id = anchor.id
  replacement.className = anchor.className + ' content-card--unavailable'
  replacement.dataset.internalCardState = 'error'
  if (replacement instanceof HTMLAnchorElement) replacement.href = anchor.getAttribute('href') || ''
  else replacement.setAttribute('role','note')
  const body = document.createElement('span'); body.className = 'content-card__body'
  const title = document.createElement('span'); title.className = 'content-card__title'; title.textContent = t(kind === 'moment' ? (retryable ? '此动态暂时无法读取' : '此动态暂不可用') : (retryable ? '此片语暂时无法读取' : '此片语暂不可用'))
  const description = document.createElement('span'); description.className = 'content-card__description'; description.textContent = retryable ? t('可打开原内容重试，或稍后重新预览') : t('内容可能已撤回或删除')
  body.append(title,description); replacement.append(body); anchor.replaceWith(replacement)
}

/** Safe to rescan after editor/preview updates; global listeners are shared. */
export function initArticleCards(root: ParentNode = document) {
  initContentVideos(root)
  for (const card of within(root, 'a.content-social')) {
    const platform = card.querySelector('.content-social__platform')
    if (platform) { try { platform.textContent = socialPlatform(card.getAttribute('href') || '') } catch { /* Static source remains readable. */ } }
  }
  if (active && !active.wrapper.isConnected) dismiss(active)
  for (const wrapper of within(root, '.content-link')) enhanceInlinePreview(wrapper)
  initInternalArticleCards(root)
  enhanceArticleImages(root)
}
