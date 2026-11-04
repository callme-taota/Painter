const clamp = (v: number, lo = 0, hi = 1) => Math.max(lo, Math.min(hi, v))
const smooth = (a: number, b: number, v: number) => { const t = clamp((v - a) / (b - a)); return t * t * (3 - 2 * t) }

// A native document first: only the bounded visible chapter gets a scroll frame.
export function initHomeShowcase() {
  const story = document.querySelector<HTMLElement>('[data-game-story]')
  const stage = story?.querySelector<HTMLElement>('[data-game-stage]')
  const aperture = story?.querySelector<HTMLElement>('[data-game-window]')
  const frames = Array.from(story?.querySelectorAll<HTMLElement>('[data-game-frame]') || [])
  const journey = story?.closest<HTMLElement>('[data-game-music-journey]')
  const record = document.querySelector<HTMLElement>('[data-record-wall]')
  const reverse = journey?.dataset.collectionDirection === 'music-games'
  const recordStage = record?.querySelector<HTMLElement>('.home-record-stage')
  let origin = {top:37.5,right:33.33,bottom:37.5,left:50}
  const nav = story?.querySelector<HTMLElement>('[data-game-nav]')
  const jumps = Array.from(nav?.querySelectorAll<HTMLButtonElement>('[data-game-jump]') || [])
  const reduced = matchMedia('(prefers-reduced-motion:reduce)')
  const configured = document.body.dataset.homeMotion !== 'false'
  let queued = 0, near = true, viewport = 1, active = -1
  const notifyMediaLayout = () => { if (story || record) dispatchEvent(new Event('painter:home-media-layout')) }

  document.querySelectorAll<HTMLImageElement>('[data-game-hero],[data-game-logo],[data-library-cover]').forEach(img => {
    const fail = () => {
      if ((img.hasAttribute('data-game-hero') || img.hasAttribute('data-library-cover')) && img.dataset.fallback && img.src !== new URL(img.dataset.fallback, location.href).href) {
        img.src = img.dataset.fallback
        delete img.dataset.fallback
        img.closest('.home-game-frame')?.classList.add('has-fallback-art')
        return
      }
      img.hidden = true
      if (img.hasAttribute('data-game-logo')) img.closest('.game-identity')?.classList.add('has-missing-logo')
    }
    img.addEventListener('error', fail)
    if (img.complete && !img.naturalWidth) fail()
  })

  const enabled = () => !!story && !!stage && !!aperture && frames.length > 0 && configured && !reduced.matches && CSS.supports('clip-path', 'inset(1% round 1px)')
  function reset() {
    story?.classList.remove('is-enhanced')
    journey?.classList.remove('is-game-journey')
    journey?.classList.remove('is-record-game-journey')
    story?.style.removeProperty('height')
    if (aperture) { aperture.inert = false; aperture.removeAttribute('style') }
    frames.forEach(frame => { frame.inert = false; frame.removeAttribute('aria-hidden'); frame.removeAttribute('style'); frame.classList.remove('is-current') })
    if (nav) nav.hidden = true
    record?.style.removeProperty('--record-drift')
    record?.querySelector('[data-record-game-origin]')?.removeAttribute('data-record-game-origin')
    active = -1
    if (story) delete story.dataset.gameFocusRetained
    notifyMediaLayout()
  }
  function render() {
    queued = 0
    if (document.hidden) return
    if (!enabled() || !story || !stage || !aperture) { reset(); return }
    const units = -story.getBoundingClientRect().top / viewport
    const focused = aperture.contains(document.activeElement)
    const focusedFrame = focused ? frames.findIndex(frame => frame.contains(document.activeElement)) : -1
    const enter = focused ? 1 : reverse ? smooth(0, 1.2, units) : smooth(-.8, .45, units)
    const exit = focused ? 0 : reverse ? smooth(frames.length + 1, frames.length + 2, units) : smooth(frames.length, frames.length + 1, units)
    story.dataset.gameFocusRetained = String(focused)
    const opening = enter * (1 - exit)
    if (reverse) {
      for (const side of ['top','right','bottom','left'] as const) {
        const end = side === 'top' || side === 'bottom' ? 40 : 24
        aperture.style.setProperty('--game-mask-' + side, (origin[side] * (1 - enter) + end * exit).toFixed(3) + '%')
      }
      aperture.style.setProperty('--game-mask-radius', String(12 * (1 - enter) + 20 * exit) + 'px')
      aperture.style.setProperty('--game-reveal-opacity', String(focused ? 1 : smooth(0,.12,units) * (1 - smooth(.78,1,exit))))
      // Finish the identity/control fade before the aperture begins closing.
      const ui = focused ? 1 : smooth(.55,1,enter) * (1 - smooth(frames.length+.7,frames.length+1,units))
      aperture.style.setProperty('--game-ui-opacity', String(ui))
      aperture.inert = !focused && (enter < .65 || ui < .1)
    } else {
      aperture.style.setProperty('--game-mask-y', ((1 - opening) * 50).toFixed(3) + '%')
      aperture.style.setProperty('--game-mask-x', ((1 - opening) * 12).toFixed(3) + '%')
      aperture.style.setProperty('--game-mask-radius', ((1 - opening) * 32).toFixed(2) + 'px')
      aperture.style.setProperty('--game-ui-opacity', String(clamp(opening * 2)))
      aperture.inert = !focused && opening < .08
    }
    const scene = focusedFrame >= 0 ? focusedFrame : clamp(units - (reverse ? 1.4 : .6), 0, frames.length - 1), at = Math.floor(scene)
    const mix = focusedFrame >= 0 ? 0 : at < frames.length - 1 ? smooth(.68, 1, scene - at) : 0
    const current = mix >= .5 ? at + 1 : at
    frames.forEach((frame, i) => {
      const opacity = i === at ? 1 - mix : i === at + 1 ? mix : 0
      frame.style.setProperty('--game-scene-opacity', opacity.toFixed(4))
      frame.style.setProperty('--game-image-y', (clamp(scene - i, -1.5, 1.5) * -7).toFixed(3) + 'vh')
      frame.style.setProperty('--game-copy-y', (clamp(scene - i, -1, 1) * -16).toFixed(2) + 'px')
      frame.classList.toggle('is-current', i === current)
      frame.inert = i !== current
      frame.setAttribute('aria-hidden', String(i !== current))
    })
    if (active !== current) { active = current; jumps.forEach((button, i) => button.setAttribute('aria-pressed', String(i === current))); story.dataset.gameActive = String(current) }
    story.dataset.gamePhase = units < (reverse ? 1.2 : .45) ? 'enter' : exit > 0 ? 'exit' : 'scenes'
    if (record && !reverse) record.style.setProperty('--record-drift', (clamp((innerHeight - record.getBoundingClientRect().top) / (innerHeight * 2)) * -40).toFixed(2) + 'px')
    notifyMediaLayout()
  }
  function schedule(force = false) { if (!queued && !document.hidden && (near || force)) queued = requestAnimationFrame(render) }
  function measure() {
    if (!enabled() || !story || !stage) { reset(); return }
    story.classList.add('is-enhanced')
    journey?.classList.add(reverse ? 'is-record-game-journey' : 'is-game-journey')
    viewport = Math.max(1, stage.clientHeight)
    story.style.height = String(viewport * (frames.length + (reverse ? 3 : 2))) + 'px'
    if (reverse && recordStage) {
      const box = recordStage.getBoundingClientRect()
      const tiles = Array.from(recordStage.querySelectorAll<HTMLElement>('.home-record-tile'))
      const tile = tiles.reduce<HTMLElement | undefined>((best,item) => {
        const score = (node:HTMLElement) => { const r=node.getBoundingClientRect();return Math.hypot((r.left+r.width/2-box.left)/box.width-.52,(r.top+r.height/2-box.top)/box.height-.46) }
        return !best || score(item)<score(best) ? item : best
      },undefined)
      if (tile && box.width && box.height) {
        record?.querySelector('[data-record-game-origin]')?.removeAttribute('data-record-game-origin')
        tile.dataset.recordGameOrigin = 'true'
        const r=tile.getBoundingClientRect()
        origin={top:clamp((r.top-box.top)/box.height)*100,right:clamp((box.right-r.right)/box.width)*100,bottom:clamp((box.bottom-r.bottom)/box.height)*100,left:clamp((r.left-box.left)/box.width)*100}
      }
    }
    if (nav) nav.hidden = false
    render()
  }
  function jump(index: number) {
    if (!enabled() || !story) return
    scrollTo({top: scrollY + story.getBoundingClientRect().top + (index + (reverse ? 1.6 : .8)) * viewport, behavior: 'instant'})
    render()
  }
  jumps.forEach((button, i) => button.addEventListener('click', () => jump(i)))
  story?.addEventListener('focusin', event => {
    const frame = (event.target as HTMLElement).closest<HTMLElement>('[data-game-frame]')
    if (frame && enabled()) jump(frames.indexOf(frame))
  })
  story?.addEventListener('focusout', () => schedule(true))
  if ('IntersectionObserver' in window && story) {
    const observer = new IntersectionObserver(entries => { near = entries[0]?.isIntersecting ?? true; if (near) schedule() }, {rootMargin: '100% 0px'})
    observer.observe(journey || story)
  }
  addEventListener('scroll', () => schedule(), {passive: true})
  addEventListener('resize', measure, {passive: true})
  reduced.addEventListener('change', measure)
  document.addEventListener('visibilitychange', () => { if (document.hidden) { cancelAnimationFrame(queued); queued = 0 } else schedule() })
  measure()
}
