import {initMomentGalleries} from './momentGallery'

type MomentLayout = 'list' | 'two' | 'grid'

const storageKey = 'painter-moment-layout'
const layouts: MomentLayout[] = ['list', 'two', 'grid']
const initialized = new WeakSet<HTMLElement>()

function isLayout(value: unknown): value is MomentLayout {
  return typeof value === 'string' && layouts.includes(value as MomentLayout)
}

/** The server-rendered cards remain readable before this preference is applied. */
export function initMoments() {
  initMomentGalleries()
  document.querySelectorAll<HTMLElement>('[data-moment-feed]').forEach(feed => {
    if (initialized.has(feed)) return
    initialized.add(feed)

    let saved: string | null = null
    try { saved = localStorage.getItem(storageKey) } catch { /* Storage is optional. */ }
    let layout: MomentLayout = isLayout(saved) ? saved : 'two'
    const controls = Array.from(feed.querySelectorAll<HTMLButtonElement>('button[data-moment-layout]'))

    function apply(value: MomentLayout, persist = false) {
      layout = value
      feed.dataset.layout = layout
      controls.forEach(button => button.setAttribute('aria-pressed', String(button.dataset.momentLayout === layout)))
      if (persist) {
        try { localStorage.setItem(storageKey, layout) } catch { /* Keep the in-page preference. */ }
      }
    }

    controls.forEach(button => {
      button.type = 'button'
      button.addEventListener('click', () => {
        const value = button.dataset.momentLayout
        if (isLayout(value)) apply(value, true)
      })
    })
    apply(layout)
    const group = feed.querySelector<HTMLElement>('.moments-layout')
    if (group) group.hidden = false
  })
}
