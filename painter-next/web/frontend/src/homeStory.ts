const clamp = (value: number) => Math.max(0, Math.min(1, value))
const smooth = (a: number, b: number, value: number) => { const t = clamp((value - a) / (b - a)); return t * t * (3 - 2 * t) }

// Native scroll drives the existing paper planes. A home without an intro still
// has its configured surface transition, without an idle render loop.
export function initHomeStory() {
  const story = document.querySelector<HTMLElement>('[data-home-story]')
  if(story?.closest('[data-work-managed]'))return
  const stage = story?.querySelector<HTMLElement>('.story-stage')
  const pages = story?.querySelector<HTMLElement>('[data-story-pages]')
  const scenes = Array.from(story?.querySelectorAll<HTMLElement>('[data-story-scene]') || [])
  const reduced = matchMedia('(prefers-reduced-motion:reduce)')
  const configuredMotion = document.body.dataset.homeMotion !== 'false'
  const configuredWarmth = document.body.dataset.homeWarmth !== 'false'
  const canAnimate = !!story && !!stage && !!pages && scenes.length > 1 && configuredMotion
  let queued = 0, last = -1, inView = true
  document.querySelectorAll<HTMLImageElement>('[data-home-cover]').forEach(img => {
    img.addEventListener('error', () => { img.hidden = true })
    if (img.complete && !img.naturalWidth) img.hidden = true
  })
  function staticScenes() {
    for (const scene of scenes) {
      scene.inert = false
      scene.removeAttribute('aria-hidden')
      scene.style.removeProperty('--scene-opacity')
      scene.style.removeProperty('--scene-y')
    }
    pages?.style.removeProperty('transform')
    story?.style.removeProperty('--page-spread')
    story?.style.removeProperty('--home-story-height')
    last = -1
  }
  function warmth(progress: number) {
    document.body.style.setProperty('--home-warmth', configuredWarmth && !reduced.matches ? smooth(0, 1, progress).toFixed(4) : '0')
  }
  function render() {
    queued = 0
    if (document.hidden) return
    const animated = canAnimate && !reduced.matches
    document.documentElement.classList.toggle('story-ready', animated)
    if (!animated || !story || !stage || !pages) {
      staticScenes()
      if (story) { story.dataset.motion = 'static'; story.dataset.renderer = 'dom' }
      warmth(clamp(scrollY / Math.max(1, document.documentElement.scrollHeight - innerHeight)))
      return
    }
    if (scenes.length !== 3) story.style.setProperty('--home-story-height', String(scenes.length * 95) + 'svh')
    const box = story.getBoundingClientRect(), progress = clamp(-box.top / Math.max(1, box.height - stage.clientHeight))
    story.dataset.motion = 'scroll'
    story.dataset.renderer = 'dom'
    warmth(progress)
    if (progress === last) return
    last = progress
    const focusedIndex = scenes.findIndex(scene => scene.contains(document.activeElement))
    const weights = focusedIndex >= 0 ? scenes.map((_, index) => index === focusedIndex ? 1 : 0) : scenes.length === 3
      ? [1 - smooth(.18, .32, progress), smooth(.2, .34, progress) * (1 - smooth(.54, .68, progress)), smooth(.57, .73, progress)]
      : [1 - smooth(.35, .65, progress), smooth(.35, .65, progress)]
    story.dataset.chapter = scenes[weights.indexOf(Math.max(...weights))]?.dataset.storyScene || 'hello'
    story.style.setProperty('--story-progress', progress.toFixed(4))
    scenes.forEach((scene, index) => {
      const alpha = weights[index] ?? 0
      const hasFocus = scene.contains(document.activeElement)
      const travel = hasFocus ? 0 : scenes.length === 3
        ? index === 0 ? -smooth(0, .34, progress) * 28 : index === 1 ? (1 - smooth(.2, .36, progress)) * 28 - smooth(.54, .7, progress) * 28 : (1 - smooth(.57, .75, progress)) * 28
        : (1 - alpha) * (index === 0 ? -28 : 28)
      scene.style.setProperty('--scene-opacity', alpha.toFixed(4))
      scene.style.setProperty('--scene-y', String(travel) + 'px')
      // A focused native link remains available while its scene leaves the
      // viewport. Scrolling must never make the active element inert.
      scene.inert = alpha < .15 && !hasFocus
      scene.setAttribute('aria-hidden', String(alpha < .02 && !hasFocus))
    })
    pages.style.transform = 'translate3d(0,' + (-progress * 24) + 'px,0) rotateX(' + (9 - progress * 7) + 'deg) rotateY(' + (-16 + progress * 22) + 'deg) rotateZ(' + (7 - progress * 12) + 'deg)'
    story.style.setProperty('--page-spread', (1 + smooth(.2, .8, progress) * 1.25).toFixed(3))
  }
  function schedule(force = false) {
    // Motion-off homes can still have the configured whole-page surface warmth.
    const globalWarmth = !canAnimate && configuredWarmth && !reduced.matches
    if (!queued && !document.hidden && (inView || force || globalWarmth)) queued = requestAnimationFrame(render)
  }
  addEventListener('scroll', () => schedule(), {passive: true})
  addEventListener('resize', () => { last = -1; schedule(true) }, {passive: true})
  reduced.addEventListener('change', () => schedule(true))
  document.addEventListener('visibilitychange', () => {
    if (document.hidden) { cancelAnimationFrame(queued); queued = 0 } else schedule(true)
  })
  story?.addEventListener('focusin', event => {
    const scene = (event.target as HTMLElement).closest<HTMLElement>('[data-story-scene]')
    if (!scene || !stage || !document.documentElement.classList.contains('story-ready')) return
    last = -1
    schedule(true)
    const index = scenes.indexOf(scene), at = scenes.length === 3 ? [0, .44, .86][index] || 0 : index * .86
    scrollTo({top: story.getBoundingClientRect().top + scrollY + (story.offsetHeight - stage.clientHeight) * at, behavior: 'instant'})
  })
  story?.addEventListener('focusout', () => { last = -1; schedule(true) })
  if (story && 'IntersectionObserver' in window) {
    new IntersectionObserver(entries => {
      inView = entries[0]?.isIntersecting ?? false
      // Render the boundary once, then do no story work while it is offscreen.
      schedule(true)
    }, {rootMargin: '80px 0px'}).observe(story)
  }
  render()
}
