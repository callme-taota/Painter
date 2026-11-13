import {registerHomeSnapSource} from './homeSnap'

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
  const recordGrid = record?.querySelector<HTMLElement>('.home-record-grid')
  const gameTile = reverse ? record?.querySelector<HTMLElement>('[data-record-game-tile]') : undefined
  const gameTileArt = gameTile?.querySelector<HTMLImageElement>('[data-game-tile-hero]')
  const firstGameArt = frames[0]?.querySelector<HTMLImageElement>('[data-game-hero]')
  const collectionHeading = story?.querySelector<HTMLElement>('.home-game-chrome')
  let tile = {left:0, top:0, width:0, height:0}, recordTravel = 0
  let origin = {top:37.5,right:33.33,bottom:37.5,left:50}
  const reduced = matchMedia('(prefers-reduced-motion:reduce)')
  const phone = matchMedia('(max-width:720px)')
  const configured = document.body.dataset.homeMotion !== 'false'
  let queued = 0, near = true, viewport = 1, viewportWidth = 1, active = -1
  const notifyMediaLayout = () => { if (story || record) dispatchEvent(new Event('painter:home-media-layout')) }

  if (firstGameArt && !phone.matches) firstGameArt.loading = 'eager'
  if (gameTile && !gameTileArt) gameTile.classList.add('has-missing-art')
  document.querySelectorAll<HTMLImageElement>('[data-game-hero],[data-game-tile-hero],[data-game-logo],[data-library-cover]').forEach(img => {
    const fail = () => {
      // Both real surfaces must use the same fallback, including a failure that
      // occurs while the wall cell is growing into the first full game scene.
      const linked = img === firstGameArt || img === gameTileArt
      const images = linked ? [firstGameArt, gameTileArt].filter((image): image is HTMLImageElement => !!image) : [img]
      if (!img.hasAttribute('data-game-logo') && img.dataset.fallback && img.src !== new URL(img.dataset.fallback, location.href).href) {
        const fallback = img.dataset.fallback
        images.forEach(image => { image.src = fallback; delete image.dataset.fallback; image.hidden = false; image.closest('.home-game-frame')?.classList.add('has-fallback-art') })
        return
      }
      images.forEach(image => { image.hidden = true })
      if (linked) gameTile?.classList.add('has-missing-art')
      if (img.hasAttribute('data-game-logo')) img.closest('.game-identity')?.classList.add('has-missing-logo')
    }
    img.addEventListener('error', fail)
    if (img.complete && !img.naturalWidth) fail()
  })

  // Phones browse real covers in native horizontal shelves. Never consume many
  // vertical viewports to advance a collection, or intercept a vertical swipe.
  const enabled = () => !!story && !!stage && !!aperture && frames.length > 0 && configured && !reduced.matches && !phone.matches && CSS.supports('clip-path', 'inset(1% round 1px)')
  function reset() {
    story?.classList.remove('is-enhanced')
    journey?.classList.remove('is-game-journey')
    journey?.classList.remove('is-record-game-journey')
    story?.style.removeProperty('height')
    if (aperture) { aperture.inert = false; aperture.removeAttribute('style') }
    frames.forEach(frame => { frame.inert = false; frame.removeAttribute('aria-hidden'); frame.removeAttribute('style'); frame.classList.remove('is-current') })
    record?.style.removeProperty('--record-drift')
    record?.style.removeProperty('--record-caption-opacity')
    recordGrid?.style.removeProperty('--record-cell')
    recordGrid?.style.removeProperty('--record-columns')
    gameTile?.classList.remove('is-record-game-tile-enhanced')
    if (gameTile) {
      gameTile.inert = false
      for (const name of ['column','row','width','height','x','y','opacity','progress','shade']) gameTile.style.removeProperty('--record-game-' + name)
    }
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
      // A scroll distance of one viewport moves the actual square-cell sheet
      // upward by one viewport. No cover copies, stretching or pinned still.
      const recordUnits = record ? -record.getBoundingClientRect().top / viewport : 0
      record?.style.setProperty('--record-drift', `${-recordTravel * clamp(recordUnits)}px`)
      record?.style.setProperty('--record-caption-opacity', String(1 - smooth(0,.28,units)))
      for (const side of ['top','right','bottom','left'] as const) {
        const end = side === 'top' || side === 'bottom' ? 40 : 24
        aperture.style.setProperty('--game-mask-' + side, (origin[side] * (1 - enter) + end * exit).toFixed(3) + '%')
      }
      aperture.style.setProperty('--game-mask-radius', String(12 * (1 - enter) + 20 * exit) + 'px')
      // Until 1.2H, the only visible game image is the real grid cell itself.
      // At the handoff its image crop and shading exactly match the full frame.
      const handedOff = enter >= 1
      aperture.style.setProperty('--game-reveal-opacity', String(focused ? 1 : (handedOff ? 1 : 0) * (1 - smooth(.78,1,exit))))
      const ui = focused ? 1 : smooth(1.2,1.55,units) * (1 - smooth(frames.length+.7,frames.length+1,units))
      aperture.style.setProperty('--game-ui-opacity', String(ui))
      aperture.style.pointerEvents = handedOff || focused ? '' : 'none'
      aperture.inert = !focused && (!handedOff || ui < .1)
      if (gameTile) {
        gameTile.style.setProperty('--record-game-x', `${-tile.left * enter}px`)
        gameTile.style.setProperty('--record-game-y', `${-(tile.top - recordTravel) * enter}px`)
        gameTile.style.setProperty('--record-game-width', `${tile.width + (viewportWidth - tile.width) * enter}px`)
        gameTile.style.setProperty('--record-game-height', `${tile.height + (viewport - tile.height) * enter}px`)
        gameTile.style.setProperty('--record-game-opacity', handedOff ? '0' : '1')
        gameTile.style.setProperty('--record-game-progress', String(enter))
        gameTile.style.setProperty('--record-game-shade', String(.12 + .88 * enter))
        gameTile.inert = enter > 0
      }
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
    if (active !== current) { active = current; story.dataset.gameActive = String(current) }
    story.dataset.gamePhase = units < (reverse ? 1.2 : .45) ? 'enter' : exit > 0 ? 'exit' : 'scenes'
    if (record && !reverse) record.style.setProperty('--record-drift', (clamp((innerHeight - record.getBoundingClientRect().top) / (innerHeight * 2)) * -40).toFixed(2) + 'px')
    notifyMediaLayout()
  }
  function schedule(force = false) { if (phone.matches && !force) return; if (!queued && !document.hidden && (near || force)) queued = requestAnimationFrame(render) }
  function measure() {
    // Keep the phone collection heading outside its horizontal scroll area.
    // The same real node returns to the desktop scene when its layout changes.
    if (collectionHeading && story && aperture) {
      if (phone.matches && collectionHeading.parentElement !== story) story.insertBefore(collectionHeading, story.firstChild)
      else if (!phone.matches && collectionHeading.parentElement !== aperture) aperture.append(collectionHeading)
    }
    if (!enabled() || !story || !stage) { reset(); return }
    story.classList.add('is-enhanced')
    journey?.classList.add(reverse ? 'is-record-game-journey' : 'is-game-journey')
    viewport = Math.max(1, stage.clientHeight)
    viewportWidth = Math.max(1, stage.clientWidth)
    story.style.height = String(viewport * (frames.length + (reverse ? 3 : 2))) + 'px'
    if (reverse && recordStage && recordGrid && gameTile) {
      const box = recordStage.getBoundingClientRect()
      const preferredColumns = matchMedia('(max-width:720px)').matches ? 2 : 4
      const itemCount = recordGrid.children.length
      let columns = preferredColumns
      // A tall viewport needs fewer real, square-cover columns to supply a
      // full viewport of travel. Sparse collections never acquire fake tiles.
      while (columns > 1 && Math.ceil(itemCount / columns) * (box.width / columns) < viewport * 2) columns--
      const cell = box.width / columns
      const rows = Math.max(1, Math.ceil(itemCount / columns))
      recordGrid.style.setProperty('--record-columns', String(columns))
      recordGrid.style.setProperty('--record-cell', `${cell}px`)
      recordTravel = Math.min(viewport, Math.max(0, rows * cell - viewport))
      const row = clamp(Math.round((recordTravel + viewport * .5 - cell / 2) / cell), 0, rows - 1)
      const column = matchMedia('(max-width:720px)').matches ? 0 : Math.floor(columns / 2)
      gameTile.style.setProperty('--record-game-column', String(column + 1))
      gameTile.style.setProperty('--record-game-row', String(row + 1))
      tile = {left:column * cell, top:row * cell, width:cell, height:cell}
      const top = tile.top - recordTravel
      origin = {top:clamp(top / viewport) * 100, right:clamp((box.width - tile.left - cell) / box.width) * 100, bottom:clamp((viewport - top - cell) / viewport) * 100, left:clamp(tile.left / box.width) * 100}
      gameTile.classList.add('is-record-game-tile-enhanced')
    }
    render()
    // Work/point morph initializes first; its 25th surface must use this final
    // square-grid slot, not the pre-enhancement layout or a transformed rect.
    dispatchEvent(new Event('painter:home-work-measure'))
  }
  function jump(index: number) {
    if (!enabled() || !story) return
    scrollTo({top: scrollY + story.getBoundingClientRect().top + (index + (reverse ? 1.6 : .8)) * viewport, behavior: 'instant'})
    render()
  }
  if (story) registerHomeSnapSource(story, () => {
    if (!enabled()) return []
    const start = scrollY + story.getBoundingClientRect().top
    return frames.map((_, index) => {
      const offset = index + (reverse ? 1.4 : .6)
      return {key:`game-${index}`, top:start + (offset + .24) * viewport, from:start + (offset + .08) * viewport, to:start + (offset + .48) * viewport}
    })
  })
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
  phone.addEventListener('change', measure)
  document.fonts.ready.then(measure)
  document.addEventListener('visibilitychange', () => { if (document.hidden) { cancelAnimationFrame(queued); queued = 0 } else schedule() })
  measure()
}
