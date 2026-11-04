import './homePointMorph.css'

export type PointMorphRect = {left: number; top: number; width: number; height: number}

type DayFrame = PointMorphRect & {element: HTMLElement; color: string; shadow: string}
type CoverFrame = {element: HTMLElement; source: DayFrame; target: PointMorphRect}

const clamp = (value: number) => Math.max(0, Math.min(1, value))
const smooth = (from: number, to: number, value: number) => {
  const t = clamp((value - from) / (to - from))
  return t * t * (3 - 2 * t)
}
const mix = (from: number, to: number, progress: number) => from + (to - from) * progress
const localRect = (box: DOMRect, origin: DOMRect): PointMorphRect => ({
  left: box.left - origin.left,
  top: box.top - origin.top,
  width: box.width,
  height: box.height,
})
const center = (box: PointMorphRect) => ({x: box.left + box.width / 2, y: box.top + box.height / 2})

function sample<T>(items: T[], count: number): T[] {
  const size = Math.min(items.length, Math.max(0, Math.floor(count)))
  return Array.from({length: size}, (_, index) => items[Math.floor((index + .5) * items.length / size)]!)
}

// Geometry is read only during measure(). Scroll frames update a few shared
// calendar scalars and the 24 saved cover surfaces; no new activity is authored.
export function createPointMorph(stage: HTMLElement, activity: HTMLElement, record?: HTMLElement) {
  const calendar = activity.querySelector<HTMLElement>('[data-activity-calendar]')
  const scroll = activity.querySelector<HTMLElement>('.activity-scroll')
  const days = Array.from(calendar?.querySelectorAll<HTMLElement>('[data-date]') || [])
  const recordStage = record?.querySelector<HTMLElement>('.home-record-stage')
  const tiles = Array.from(record?.querySelectorAll<HTMLElement>('.home-record-tile') || []).slice(0, 24)
  const layer = document.createElement('div')
  layer.className = 'point-record-layer'
  layer.setAttribute('aria-hidden', 'true')
  layer.inert = true
  layer.hidden = true
  stage.append(layer)

  const surfaces = tiles.map(tile => {
    const surface = document.createElement('div')
    surface.className = 'point-record-surface'
    const source = tile.querySelector<HTMLImageElement>('img')
    if (source) {
      const image = source.cloneNode(false) as HTMLImageElement
      image.removeAttribute('id')
      image.removeAttribute('data-home-cover')
      image.alt = ''
      image.loading = 'eager'
      image.decoding = 'async'
      image.addEventListener('error', () => { image.hidden = true })
      surface.append(image)
    }
    layer.append(surface)
    return {tile, surface}
  })

  let frames: DayFrame[] = [], visible: DayFrame[] = [], covers: CoverFrame[] = []
  let chosenCards: HTMLElement[] = [], chosenSources = new Set<HTMLElement>()
  let savedScrollLeft = scroll?.scrollLeft || 0
  let moving = false, calendarProgress = 0, recordProgress = 0
  let sourcesHidden = false
  let measured = false

  function setMoving(next: boolean) {
    if (next === moving) return
    moving = next
    if (next) {
      // Overflow must open while real cells travel outside the calendar. Keep
      // its native horizontal position when it temporarily stops scrolling.
      activity.style.setProperty('--point-scroll-left', `${savedScrollLeft}px`)
      activity.classList.add('point-calendar-moving')
    } else {
      activity.classList.remove('point-calendar-moving')
      if (scroll) scroll.scrollLeft = savedScrollLeft
    }
  }

  function cardTargets(count: number) {
    const real = visible.filter(frame => !frame.element.classList.contains('is-future'))
    const pool = real.length ? real : visible
    const published = pool.filter(frame => Number(frame.element.dataset.count) > 0)
    const selected = published.length >= count
      ? sample(published, count)
      : [...published, ...sample(pool.filter(frame => !published.includes(frame)), count - published.length)]
    chosenCards.forEach(day => day.classList.remove('point-card-landing'))
    chosenCards = selected.map(frame => frame.element)
    return chosenCards.slice()
  }

  function measure() {
    const restoreCalendar = calendarProgress, restoreRecords = recordProgress
    if (!moving && scroll) savedScrollLeft = scroll.scrollLeft
    setMoving(false)
    activity.classList.add('point-calendar-active')
    activity.style.setProperty('--point-gather', '0')
    activity.style.setProperty('--point-resolve', '0')
    activity.style.setProperty('--point-day-alpha', '1')

    const stageBox = stage.getBoundingClientRect()
    const viewport = (scroll || activity).getBoundingClientRect()
    const cards = Array.from(stage.querySelectorAll<HTMLElement>('.project-card-link')).map(card => center(localRect(card.getBoundingClientRect(), stageBox)))
    const fallback = {x: stageBox.width / 2, y: stageBox.height / 2}
    frames = days.map(element => {
      const style = getComputedStyle(element)
      return {element, ...localRect(element.getBoundingClientRect(), stageBox), color: style.backgroundColor, shadow: style.boxShadow}
    }).filter(frame => frame.width > 0 && frame.height > 0)
    visible = frames.filter(frame => {
      const x = frame.left + stageBox.left
      return x >= viewport.left && x + frame.width <= viewport.right
    })
    if (!visible.length) visible = frames.filter(frame => {
      const x = frame.left + stageBox.left
      return x + frame.width > viewport.left && x < viewport.right
    })

    const past = visible.filter(frame => !frame.element.classList.contains('is-future'))
    const sourcePool = past.length >= surfaces.length ? past : visible
    const sources = sample(sourcePool, surfaces.length)
    chosenSources.forEach(day => day.classList.remove('point-record-origin'))
    chosenSources = new Set(sources.map(frame => frame.element))
    sourcesHidden = false
    const recordBox = recordStage?.getBoundingClientRect()
    covers = []
    surfaces.forEach(({tile, surface}, index) => {
      const source = sources[index]
      const tileBox = tile.getBoundingClientRect()
      if (!source || !recordBox || tileBox.width <= 0 || tileBox.height <= 0) {
        surface.hidden = true
        return
      }
      const target = localRect(tileBox, recordBox)
      surface.hidden = false
      surface.dataset.pointSourceDate = source.element.dataset.date || ''
      surface.dataset.pointRecordIndex = String(index)
      surface.style.left = `${target.left}px`
      surface.style.top = `${target.top}px`
      surface.style.width = `${target.width}px`
      surface.style.height = `${target.height}px`
      surface.style.backgroundColor = source.color
      surface.style.boxShadow = source.shadow
      covers.push({element: surface, source, target})
    })

    frames.forEach((frame, index) => {
      const point = center(frame)
      const origin = cards[index % cards.length] || fallback
      const nearest = sources.reduce<DayFrame | undefined>((best, candidate) => {
        if (!best) return candidate
        const a = center(candidate), b = center(best)
        return (a.x - point.x) ** 2 + (a.y - point.y) ** 2 < (b.x - point.x) ** 2 + (b.y - point.y) ** 2 ? candidate : best
      }, undefined)
      const destination = nearest ? center(nearest) : point
      frame.element.classList.add('point-calendar-day')
      frame.element.style.setProperty('--point-gather-x', `${origin.x - point.x}px`)
      frame.element.style.setProperty('--point-gather-y', `${origin.y - point.y}px`)
      frame.element.style.setProperty('--point-resolve-x', `${destination.x - point.x}px`)
      frame.element.style.setProperty('--point-resolve-y', `${destination.y - point.y}px`)
    })
    measured = true
    render(restoreCalendar, restoreRecords)
  }

  function render(toCalendar: number, toRecords: number) {
    calendarProgress = clamp(toCalendar)
    recordProgress = clamp(toRecords)
    if (!measured) return
    activity.classList.add('point-calendar-active')
    const gather = 1 - smooth(0, 1, calendarProgress)
    const resolve = smooth(0, .62, recordProgress)
    activity.style.setProperty('--point-gather', gather.toFixed(5))
    activity.style.setProperty('--point-resolve', resolve.toFixed(5))
    activity.style.setProperty('--point-day-alpha', (1 - smooth(.05, .62, recordProgress)).toFixed(5))
    setMoving((calendarProgress > 0 && calendarProgress < 1) || (recordProgress > 0 && recordProgress < 1))

    // Native days take over from the actual project cards, and then pass their
    // exact positions and colors into saved album art. Their labels stay native.
    chosenCards.forEach(day => day.classList.toggle('point-card-landing', calendarProgress > 0 && calendarProgress < .985 && recordProgress === 0))
    if (sourcesHidden !== (recordProgress > 0)) {
      sourcesHidden = recordProgress > 0
      chosenSources.forEach(day => day.classList.toggle('point-record-origin', sourcesHidden))
    }
    layer.hidden = recordProgress <= 0 || !covers.length
    layer.classList.toggle('is-moving', recordProgress > 0 && recordProgress < 1)
    layer.style.setProperty('--point-cover-alpha', smooth(.12, .9, recordProgress).toFixed(5))
    const travel = smooth(0, 1, recordProgress)
    covers.forEach(({element, source, target}) => {
      const scaleX = mix(source.width / target.width, 1, travel)
      const scaleY = mix(source.height / target.height, 1, travel)
      const x = (source.left - target.left) * (1 - travel)
      const y = (source.top - target.top) * (1 - travel)
      const radius = 3 * (1 - travel)
      element.style.transform = recordProgress === 1 ? 'none' : `translate3d(${x.toFixed(3)}px,${y.toFixed(3)}px,0) scale(${scaleX.toFixed(6)},${scaleY.toFixed(6)})`
      element.style.borderRadius = `${(radius / scaleX).toFixed(3)}px / ${(radius / scaleY).toFixed(3)}px`
      const stroke = 1 - smooth(0, .65, recordProgress)
      let length = 0
      element.style.boxShadow = !stroke || source.shadow === 'none' ? 'none' : source.shadow
        .replace(/(?:rgba?|color)\([^)]*\)/g, color => `color-mix(in srgb,${color} ${(stroke * 100).toFixed(4)}%,transparent)`)
        .replace(/(-?[\d.]+)px/g, (_, value: string) => `${(Number(value) / (length++ === 0 ? scaleX : length === 2 ? scaleY : Math.sqrt(scaleX * scaleY))).toFixed(4)}px`)
    })
  }

  function reset() {
    setMoving(false)
    calendarProgress = 0
    recordProgress = 0
    sourcesHidden = false
    activity.classList.remove('point-calendar-active')
    for (const property of ['--point-gather', '--point-resolve', '--point-day-alpha', '--point-scroll-left']) activity.style.removeProperty(property)
    days.forEach(day => day.classList.remove('point-card-landing', 'point-record-origin'))
    layer.hidden = true
    layer.classList.remove('is-moving')
  }

  function destroy() {
    reset()
    days.forEach(day => {
      day.classList.remove('point-calendar-day')
      for (const property of ['--point-gather-x', '--point-gather-y', '--point-resolve-x', '--point-resolve-y']) day.style.removeProperty(property)
    })
    layer.remove()
  }

  return {
    measure,
    render,
    reset,
    destroy,
    cardTargets,
    dayRect(day: HTMLElement): PointMorphRect | undefined {
      const frame = frames.find(item => item.element === day)
      return frame && {left: frame.left, top: frame.top, width: frame.width, height: frame.height}
    },
    get recordCount() { return covers.length },
  }
}
