import './homePointMorph.css'

export type PointMorphRect = {left: number; top: number; width: number; height: number}

type DayFrame = PointMorphRect & {element: HTMLElement; level: number}
type CoverFrame = {element: HTMLElement; source: DayFrame; target: PointMorphRect}

const activityColors = ['var(--activity-0)', 'var(--activity-1)', 'var(--activity-2)', 'var(--activity-3)', 'var(--activity-4)'] as const
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

// A complete cartesian block, never the diamond produced by radial sampling.
// Keep the same visual (row, column) at both ends so every in-between frame is
// a regular grid too, including when the game occupies an earlier visual slot.
function centralBlock(pool: DayFrame[], columns: number, rows: number): DayFrame[] {
  if (!pool.length || !columns || !rows) return []
  const coordinate = (value: number) => Math.round(value * 100) / 100
  const xs = [...new Set(pool.map(frame => coordinate(frame.left)))].sort((a, b) => a - b)
  const ys = [...new Set(pool.map(frame => coordinate(frame.top)))].sort((a, b) => a - b)
  const cells = new Map(pool.map(frame => [`${coordinate(frame.left)}:${coordinate(frame.top)}`, frame]))
  const middle = {
    x: (Math.min(...pool.map(frame => frame.left)) + Math.max(...pool.map(frame => frame.left + frame.width))) / 2,
    y: (Math.min(...pool.map(frame => frame.top)) + Math.max(...pool.map(frame => frame.top + frame.height))) / 2,
  }
  let best: DayFrame[] = [], score = Infinity
  for (let row = 0; row <= ys.length - rows; row++) {
    for (let column = 0; column <= xs.length - columns; column++) {
      const block: DayFrame[] = []
      for (let y = row; y < row + rows; y++) for (let x = column; x < column + columns; x++) {
        const cell = cells.get(`${xs[x]}:${ys[y]}`)
        if (cell) block.push(cell)
      }
      if (block.length !== columns * rows) continue
      const first = block[0]!, last = block[block.length - 1]!
      const x = (first.left + last.left + last.width) / 2
      const y = (first.top + last.top + last.height) / 2
      const distance = (x - middle.x) ** 2 + (y - middle.y) ** 2
      if (distance < score) { best = block; score = distance }
    }
  }
  return best
}

// Geometry is read only during measure(). Scroll frames update a few shared
// calendar scalars and saved cover surfaces; no new activity is authored.
export function createPointMorph(stage: HTMLElement, activity: HTMLElement, record?: HTMLElement) {
  const calendar = activity.querySelector<HTMLElement>('[data-activity-calendar]')
  const scroll = activity.querySelector<HTMLElement>('.activity-scroll')
  const days = Array.from(calendar?.querySelectorAll<HTMLElement>('[data-date]') || [])
  const recordStage = record?.querySelector<HTMLElement>('.home-record-stage')
  const tiles = Array.from(record?.querySelectorAll<HTMLElement>('.home-record-tile') || [])
  const layer = document.createElement('div')
  layer.className = 'point-record-layer'
  layer.setAttribute('aria-hidden', 'true')
  layer.inert = true
  layer.hidden = true
  stage.append(layer)

  const sourceImages = new Map<HTMLImageElement, HTMLImageElement>()
  function syncImage(source: HTMLImageElement, image: HTMLImageElement) {
    for (const name of ['src', 'srcset', 'sizes']) {
      const value = source.getAttribute(name)
      if (value === null) image.removeAttribute(name)
      else if (image.getAttribute(name) !== value) image.setAttribute(name, value)
    }
    image.hidden = source.hidden
  }
  const imageChanges = new MutationObserver(changes => {
    for (const source of new Set(changes.map(change => change.target as HTMLImageElement))) {
      const image = sourceImages.get(source)
      if (image) syncImage(source, image)
    }
  })
  const surfaces = tiles.map(tile => {
    const surface = document.createElement('div')
    surface.className = 'point-record-surface'
    if (tile.hasAttribute('data-record-game-tile')) surface.dataset.pointGame = 'true'
    const source = tile.querySelector<HTMLImageElement>('img')
    if (source) {
      const image = source.cloneNode(false) as HTMLImageElement
      // The projection follows its real image; it must not be registered as
      // another native game hero or acquire its independent fallback handler.
      for (const name of ['id', 'data-home-cover', 'data-game-tile-hero', 'data-game-hero', 'data-fallback']) image.removeAttribute(name)
      image.alt = ''
      image.loading = 'eager'
      image.decoding = 'async'
      image.addEventListener('error', () => { image.hidden = true })
      image.addEventListener('load', () => { image.hidden = source.hidden })
      sourceImages.set(source, image)
      imageChanges.observe(source, {attributes: true, attributeFilter: ['src', 'srcset', 'sizes', 'hidden']})
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
      return {element, ...localRect(element.getBoundingClientRect(), stageBox), level: Math.max(0, Math.min(4, Math.round(Number(element.dataset.level) || 0)))}
    }).filter(frame => frame.width > 0 && frame.height > 0)
    visible = frames.filter(frame => {
      const x = frame.left + stageBox.left
      return x >= viewport.left && x + frame.width <= viewport.right
    })
    if (!visible.length) visible = frames.filter(frame => {
      const x = frame.left + stageBox.left
      return x + frame.width > viewport.left && x < viewport.right
    })

    const recordBox = recordStage?.getBoundingClientRect()
    const grid = record?.querySelector<HTMLElement>('.home-record-grid')
    const cell = Number.parseFloat(grid?.style.getPropertyValue('--record-cell') || '')
    const squareGrid = record?.closest('.is-record-game-journey') && Number.isFinite(cell) && cell > 0
    const targets = surfaces.flatMap(({tile, surface}, index) => {
      surface.hidden = true
      const box = tile.getBoundingClientRect()
      if (!recordBox || box.width <= 0 || box.height <= 0) return []
      // Native offsets survive wall scrolling and expansion of its game cell.
      const target = squareGrid ? {left: tile.offsetLeft, top: tile.offsetTop, width: cell, height: cell} : localRect(box, recordBox)
      return [{tile, surface, index, target}]
    }).sort((a, b) => a.target.top - b.target.top || a.target.left - b.target.left)
    const columns = targets.filter(item => Math.abs(item.target.top - (targets[0]?.target.top || 0)) < .5).length
    const nativeRows = [...new Set(targets.map(item => item.target.top))].sort((a, b) => a - b)
    const selectedTargets: typeof targets = []
    let rows = 0
    for (const top of nativeRows) {
      if (!recordBox || top >= recordBox.height || rows >= 7) break
      const row = targets.filter(item => Math.abs(item.target.top - top) < .5)
      if (row.length !== columns) break
      selectedTargets.push(...row)
      rows++
    }
    // Only the complete first-screen rectangle participates. The rest of the
    // actual wall is below the viewport and enters through its native scroll.
    const past = visible.filter(frame => !frame.element.classList.contains('is-future'))
    let sources = centralBlock(past, columns, rows)
    if (!sources.length) sources = centralBlock(visible, columns, rows)
    chosenSources.forEach(day => day.classList.remove('point-record-origin'))
    chosenSources = new Set(sources.map(frame => frame.element))
    sourcesHidden = false
    layer.dataset.pointColumns = String(columns)
    layer.dataset.pointRows = String(rows)
    covers = []
    selectedTargets.forEach(({tile, surface, index, target}, position) => {
      const source = sources[position]
      if (!source) return
      surface.hidden = false
      surface.dataset.pointSourceDate = source.element.dataset.date || ''
      surface.dataset.pointRecordIndex = String(index)
      surface.dataset.pointRow = String(Math.floor(position / columns))
      surface.dataset.pointColumn = String(position % columns)
      surface.style.left = `${target.left}px`
      surface.style.top = `${target.top}px`
      surface.style.width = `${target.width}px`
      surface.style.height = `${target.height}px`
      surface.style.backgroundColor = activityColors[source.level]!
      surface.style.boxShadow = source.level === 0 ? 'inset 0 0 0 1px color-mix(in srgb,var(--ink) 13%,transparent)' : 'none'
      if (tile.hasAttribute('data-record-game-tile')) {
        const art = tile.querySelector<HTMLImageElement>('img'), image = surface.querySelector<HTMLImageElement>('img')
        if (art && image) {
          image.style.position = 'absolute'
          image.style.inset = '-10% 0 auto'
          image.style.height = '120%'
          image.style.objectPosition = getComputedStyle(art).objectPosition
        }
      }
      covers.push({element: surface, source, target})
    })

    frames.forEach((frame, index) => {
      const point = center(frame)
      const origin = cards[index % cards.length] || fallback
      frame.element.classList.add('point-calendar-day')
      frame.element.style.setProperty('--point-gather-x', `${origin.x - point.x}px`)
      frame.element.style.setProperty('--point-gather-y', `${origin.y - point.y}px`)
      frame.element.style.setProperty('--point-resolve-x', '0px')
      frame.element.style.setProperty('--point-resolve-y', '0px')
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
    activity.style.setProperty('--point-gather', gather.toFixed(5))
    // Unselected days fade in place; only the central cover surfaces grow.
    activity.style.setProperty('--point-resolve', '0')
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
      element.style.boxShadow = !stroke || source.level !== 0 ? 'none' : `inset 0 0 0 ${(1 / Math.sqrt(scaleX * scaleY)).toFixed(4)}px color-mix(in srgb,var(--ink) ${(stroke * 13).toFixed(4)}%,transparent)`
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
    imageChanges.disconnect()
    sourceImages.clear()
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
