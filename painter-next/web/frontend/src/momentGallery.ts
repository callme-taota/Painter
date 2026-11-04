import './momentGallery.css'

type ImageGalleryOptions = {linkSelector?: string; title?: string}
type ImageViewer = {open(gallery: HTMLElement, links: HTMLAnchorElement[], opener: HTMLAnchorElement, fallbackTitle: string): void}
const boundImageLinks = new WeakSet<HTMLAnchorElement>()
let sharedViewer: ImageViewer | undefined

/** One viewer serves moments, home photos and ordinary article images. */
function createImageViewer(): ImageViewer {
  const dialog = document.createElement('dialog')
  dialog.className = 'moment-lightbox'
  dialog.setAttribute('aria-labelledby', 'moment-lightbox-title')
  dialog.setAttribute('aria-describedby', 'moment-lightbox-position')
  const title = document.createElement('h2')
  title.id = 'moment-lightbox-title'
  title.className = 'sr-only'
  title.textContent = '图片预览'
  const position = document.createElement('span')
  position.className = 'moment-lightbox-position sr-only'
  position.id = 'moment-lightbox-position'
  position.setAttribute('aria-live', 'polite')
  position.setAttribute('aria-atomic', 'true')
  function iconButton(name: string, label: string, path: string) {
    const button = document.createElement('button')
    button.type = 'button'
    button.className = 'moment-lightbox-' + name
    button.setAttribute('aria-label', label)
    const svg = document.createElementNS('http://www.w3.org/2000/svg', 'svg')
    svg.setAttribute('class', 'icon')
    svg.setAttribute('viewBox', '0 0 24 24')
    svg.setAttribute('aria-hidden', 'true')
    svg.setAttribute('focusable', 'false')
    const shape = document.createElementNS('http://www.w3.org/2000/svg', 'path')
    shape.setAttribute('d', path)
    svg.append(shape)
    button.append(svg)
    return button
  }
  const close = iconButton('close', '关闭图片预览', 'M6 6l12 12M6 18 18 6')
  const previous = iconButton('previous', '上一张图片', 'm15 6-6 6 6 6')
  const next = iconButton('next', '下一张图片', 'm9 6 6 6-6 6')
  const stage = document.createElement('div')
  stage.className = 'moment-lightbox-stage'
  let picture = document.createElement('img')
  const status = document.createElement('p')
  status.className = 'moment-lightbox-status'
  status.setAttribute('role', 'status')
  const original = document.createElement('a')
  original.className = 'moment-lightbox-original'
  original.textContent = '打开原图'
  original.target = '_blank'
  original.rel = 'noopener'
  original.hidden = true
  stage.append(picture, status, original)
  dialog.append(title, position, stage, close, previous, next)
  document.body.append(dialog)

  let items: HTMLAnchorElement[] = [], index = 0, opener: HTMLAnchorElement | undefined, request = 0
  function show(at: number) {
    index = Math.max(0, Math.min(items.length - 1, at))
    const link = items[index]
    if (!link) return
    const sequence = ++request
    const current = document.createElement('img')
    current.decoding = 'async'
    current.draggable = false
    current.hidden = true
    current.alt = link.dataset.imageAlt || `${title.textContent} ${index + 1}`
    // Each selection owns its image and handlers; late requests cannot replace it.
    current.addEventListener('load', () => {
      if (sequence !== request || !dialog.open) return
      current.hidden = false
      status.hidden = true
      original.hidden = true
      dialog.dataset.imageState = 'ready'
    })
    current.addEventListener('error', () => {
      if (sequence !== request || !dialog.open) return
      current.hidden = true
      status.hidden = false
      status.textContent = '图片暂时无法加载。'
      original.hidden = false
      dialog.dataset.imageState = 'error'
    })
    picture.replaceWith(current)
    picture = current
    if (document.activeElement === original) close.focus({preventScroll: true})
    status.hidden = false
    status.textContent = '正在加载图片…'
    original.hidden = true
    original.href = link.href
    dialog.dataset.imageState = 'loading'
    position.textContent = `${index + 1} / ${items.length}`
    previous.disabled = index === 0
    next.disabled = index === items.length - 1
    previous.hidden = next.hidden = items.length < 2
    if (document.activeElement === previous && previous.disabled) (next.hidden || next.disabled ? close : next).focus({preventScroll: true})
    if (document.activeElement === next && next.disabled) (previous.hidden || previous.disabled ? close : previous).focus({preventScroll: true})
    current.src = link.href
  }
  previous.addEventListener('click', () => show(index - 1))
  next.addEventListener('click', () => show(index + 1))
  close.addEventListener('click', () => dialog.close())
  dialog.addEventListener('close', () => {
    request++
    picture.removeAttribute('src')
    if (opener?.isConnected) opener.focus({preventScroll: true})
  })
  dialog.addEventListener('keydown', event => {
    if (event.altKey || event.ctrlKey || event.metaKey || event.defaultPrevented) return
    let at: number | undefined
    if (event.key === 'ArrowLeft') at = index - 1
    if (event.key === 'ArrowRight') at = index + 1
    if (event.key === 'Home') at = 0
    if (event.key === 'End') at = items.length - 1
    if (at !== undefined) { event.preventDefault(); show(at) }
  })
  let backdropPress = false
  const onBackdrop = (target: EventTarget | null) => target === dialog || target === stage
  dialog.addEventListener('pointerdown', event => { backdropPress = event.button === 0 && onBackdrop(event.target) })
  dialog.addEventListener('pointercancel', () => { backdropPress = false })
  dialog.addEventListener('click', event => {
    if (backdropPress && onBackdrop(event.target)) dialog.close()
    backdropPress = false
  })
  return {
    open(gallery, links, link, fallbackTitle) {
      items = links
      opener = link
      title.textContent = gallery.dataset.galleryTitle || fallbackTitle
      show(links.indexOf(link))
      if (!dialog.open) dialog.showModal()
      close.focus({preventScroll: true})
    }
  }
}

/** Idempotent scans also support newly inserted galleries without new dialogs. */
export function initImageGalleries(galleries: Iterable<HTMLElement>, options: ImageGalleryOptions = {}) {
  if (typeof HTMLDialogElement === 'undefined' || !HTMLDialogElement.prototype.showModal) return
  const selector = options.linkSelector || '[data-moment-image]'
  for (const gallery of galleries) {
    for (const link of gallery.querySelectorAll<HTMLAnchorElement>(selector)) {
      if (boundImageLinks.has(link)) continue
      boundImageLinks.add(link)
      sharedViewer ||= createImageViewer()
      link.addEventListener('click', event => {
        if (event.defaultPrevented || event.button !== 0 || event.ctrlKey || event.metaKey || event.shiftKey || event.altKey) return
        const links = Array.from(gallery.querySelectorAll<HTMLAnchorElement>(selector))
        if (!links.includes(link)) return
        event.preventDefault()
        sharedViewer!.open(gallery, links, link, options.title || '图片')
      })
    }
  }
}

/** Native image links remain the no-JavaScript fallback. */
export function initMomentGalleries() {
  initImageGalleries(document.querySelectorAll<HTMLElement>('[data-moment-gallery]'), {title: '动态图片'})
}
