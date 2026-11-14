export type HomeSnapPoint = {key: string; top: number; from: number; to: number}
type Source = () => HomeSnapPoint[]
type Direction = -1 | 0 | 1
type Intent = {direction: Direction; lastY: number; observedScroll: boolean; waitingUntil: number}

const sources = new Map<HTMLElement, Source>()
const sign = (value: number): Direction => value > 0 ? 1 : value < 0 ? -1 : 0

// Owners opt in only their complete, stable frames, using absolute scrollY.
export function registerHomeSnapSource(owner: HTMLElement, read: Source): () => void {
  sources.set(owner, read)
  return () => { if (sources.get(owner) === read) sources.delete(owner) }
}

let initialized = false
export function initHomeSnap() {
  if (initialized || document.body.dataset.pageKind !== 'home') return
  initialized = true
  const reduced = matchMedia('(prefers-reduced-motion:reduce)')
  const phone = matchMedia('(max-width:720px)')
  const interactive = 'a,button,input,textarea,select,summary,[contenteditable],[tabindex],[role="slider"],[role="listbox"],[role="menuitem"],[role="dialog"]'
  let timer = 0, frame = 0, intent: Intent | undefined, pageY = scrollY, animatedY: number | undefined
  let touch: {x: number; y: number; blocked: boolean} | undefined

  function allowed() {
    if (document.hidden || reduced.matches || phone.matches || document.body.dataset.homeMotion === 'false') return false
    const active = document.activeElement
    if (active instanceof Element && active !== document.body && active !== document.documentElement
      && (active.matches(interactive) || active.closest('[contenteditable="true"]'))) return false
    return !Array.from(document.querySelectorAll<HTMLElement>('dialog[open],[role="dialog"][aria-modal="true"]'))
      .some(dialog => dialog.getClientRects().length > 0 && getComputedStyle(dialog).visibility !== 'hidden')
  }
  function cancel() {
    clearTimeout(timer)
    cancelAnimationFrame(frame)
    timer = 0
    frame = 0
    intent = undefined
    animatedY = undefined
    delete document.body.dataset.homeSnapping
  }
  function nested(target: EventTarget | null) {
    for (let node = target instanceof Element ? target : null; node && node !== document.body && node !== document.documentElement; node = node.parentElement) {
      const style = getComputedStyle(node)
      if ((/^(auto|scroll)$/.test(style.overflowX) && node.scrollWidth > node.clientWidth + 2)
        || (/^(auto|scroll)$/.test(style.overflowY) && node.scrollHeight > node.clientHeight + 2)) return true
    }
    return false
  }
  function settle() {
    clearTimeout(timer)
    if (touch || !intent?.observedScroll) return
    timer = window.setTimeout(snap, 220)
  }
  function snap() {
    timer = 0
    const gesture = intent
    intent = undefined // A gesture is consumed even when there is no suitable frame.
    if (!gesture?.observedScroll || !gesture.direction || touch || !allowed()) return
    const y = scrollY, limit = Math.max(0, document.documentElement.scrollHeight - innerHeight)
    const maximum = Math.min(innerHeight * .12, 120)
    if (y < 0 || y > limit || Math.abs(y - gesture.lastY) > 1) return // Native motion still wins.
    const candidates: {owner: HTMLElement; read: Source; point: HomeSnapPoint; distance: number}[] = []
    for (const [owner, read] of sources) {
      if (!owner.isConnected) continue
      for (const point of read()) {
        if (![point.top, point.from, point.to].every(Number.isFinite) || point.from > point.to
          || point.top < point.from || point.top > point.to || y < point.from || y > point.to
          || point.top < 0 || point.top > limit) continue
        const distance = point.top - y
        if (Math.abs(distance) < 2 || Math.abs(distance) > maximum || sign(distance) !== gesture.direction) continue
        candidates.push({owner, read, point, distance})
      }
    }
    const target = candidates.sort((a, b) => Math.abs(a.distance) - Math.abs(b.distance))[0]
    if (!target) return
    const started = performance.now(), duration = 180 + Math.min(40, Math.abs(target.distance) * .35)
    animatedY = y
    document.body.dataset.homeSnapping = target.point.key
    function step(now: number) {
      if (touch || !allowed() || !target.owner.isConnected || sources.get(target.owner) !== target.read
        || (animatedY !== undefined && Math.abs(scrollY - animatedY) > 2)) { cancel(); return }
      const progress = Math.max(0, Math.min(1, (now - started) / duration))
      animatedY = y + target.distance * (1 - Math.pow(1 - progress, 3))
      scrollTo({top: animatedY, behavior: 'instant'})
      if (progress < 1) frame = requestAnimationFrame(step)
      else { frame = 0; animatedY = undefined; delete document.body.dataset.homeSnapping }
    }
    frame = requestAnimationFrame(step)
  }
  addEventListener('wheel', event => {
    cancel()
    if (!event.isTrusted || touch || !allowed() || event.ctrlKey || event.metaKey || event.altKey || event.shiftKey
      || Math.abs(event.deltaY) <= Math.abs(event.deltaX) || nested(event.target)) return
    // The wheel delta is not the direction: wait for an actual page increment.
    // A passive wheel can run after compositor scrolling, before its scroll
    // event. Keep the last observed page position rather than reading it here.
    intent = {direction: 0, lastY: pageY, observedScroll: false, waitingUntil: performance.now() + 160}
  }, {passive: true})
  addEventListener('touchstart', event => {
    cancel()
    const point = event.touches[0]
    touch = point ? {x: point.clientX, y: point.clientY, blocked: !event.isTrusted || event.touches.length !== 1
      || !allowed() || nested(event.target) || (event.target instanceof Element && !!event.target.closest(interactive))} : undefined
  }, {passive: true})
  addEventListener('touchmove', event => {
    if (!touch) return
    const point = event.touches[0]
    if (!point || !event.isTrusted || event.touches.length !== 1 || !allowed() || nested(event.target)) touch.blocked = true
    if (!point) { cancel(); return }
    const x = Math.abs(point.clientX - touch.x), y = Math.abs(point.clientY - touch.y)
    if (x > y && x > 6) touch.blocked = true
    if (touch.blocked) { cancel(); return }
    if (y > x && y > 6 && !intent) {
      intent = {direction: 0, lastY: pageY, observedScroll: false, waitingUntil: performance.now() + 160}
    }
  }, {passive: true})
  addEventListener('touchend', event => {
    if (event.touches.length) { if (touch) touch.blocked = true; cancel(); return }
    touch = undefined
    settle() // Native inertia keeps resetting this timer through scroll events.
  }, {passive: true})
  addEventListener('touchcancel', () => { touch = undefined; cancel() }, {passive: true})
  addEventListener('scroll', () => {
    pageY = scrollY
    if (frame) {
      if (animatedY !== undefined && Math.abs(pageY - animatedY) > 2) cancel()
      return
    }
    if (!intent) return
    if (!allowed() || (touch?.blocked)) { cancel(); return }
    const y = scrollY, delta = y - intent.lastY
    if (Math.abs(delta) < .5) return
    if (!intent.observedScroll && performance.now() > intent.waitingUntil) { cancel(); return }
    intent.lastY = y
    intent.direction = sign(delta)
    intent.observedScroll = true
    settle()
  }, {passive: true})
  addEventListener('pointerdown', event => { if (event.pointerType !== 'touch') cancel() }, {passive: true})
  addEventListener('keydown', cancel)
  addEventListener('hashchange', cancel)
  addEventListener('popstate', cancel)
  addEventListener('resize', cancel)
  addEventListener('blur', cancel)
  document.addEventListener('focusin', cancel)
  document.addEventListener('visibilitychange', cancel)
  reduced.addEventListener('change', cancel)
  phone.addEventListener('change', cancel)
}
