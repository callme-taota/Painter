// Keep chronological DOM order while packing mixed portrait/landscape cards.
// Only content/width changes schedule work; scrolling never measures the grid.
const grids = new WeakMap<HTMLElement, () => void>()

export function initMomentGrids() {
  if (typeof ResizeObserver !== 'function') return
  for (const grid of document.querySelectorAll<HTMLElement>('.moments-items,.author-writing .author-moments')) {
    const existing = grids.get(grid)
    if (existing) { existing(); continue }
    let queued = false, lastColumns = 0
    const observed = new WeakSet<Element>()
    const sizes = new WeakMap<Element, string>()
    const observer = new ResizeObserver(entries => {
      let changed = false
      for (const entry of entries) {
        const box = entry.contentRect
        // Container height follows our placements; only its width is an input.
        const size = entry.target === grid ? `${box.width}` : `${box.width},${box.height}`
        if (sizes.get(entry.target) !== size) { sizes.set(entry.target, size); changed = true }
      }
      if (changed) schedule()
    })
    function layout() {
      queued = false
      if (!grid.isConnected || !grid.clientWidth) return
      const cards = Array.from(grid.children).filter((node): node is HTMLElement => node instanceof HTMLElement && node.matches('.moment-card'))
      grid.setAttribute('data-moment-grid', '')
      // Old explicit columns would create implicit tracks after a breakpoint
      // shrinks the grid. Clear them before reading the new track widths.
      const style = getComputedStyle(grid)
      const columns = Number(style.getPropertyValue('--moment-columns')) || 1
      if (columns !== lastColumns) {
        cards.forEach(card => card.style.removeProperty('grid-area'))
        lastColumns = columns
      }
      const gap = Math.ceil((parseFloat(style.getPropertyValue('--moment-row-gap')) || 0) / 2)
      const heights = Array<number>(columns).fill(0)
      // Read together before writing placements to avoid per-card layout flushes.
      const measured = cards.map(card => Math.ceil(card.getBoundingClientRect().height / 2))
      cards.forEach((card, index) => {
        const column = heights.indexOf(Math.min(...heights))
        const start = heights[column]! + 1, span = Math.max(1, measured[index]!)
        const area = `${start} / ${column + 1} / span ${span} / span 1`
        if (card.style.gridArea !== area) card.style.gridArea = area
        heights[column] = start - 1 + span + gap
        if (!observed.has(card)) { observed.add(card); observer.observe(card) }
      })
    }
    function schedule() {
      if (!queued) { queued = true; requestAnimationFrame(layout) }
    }
    grids.set(grid, schedule)
    observer.observe(grid)
    void document.fonts.ready.then(schedule)
    schedule()
  }
}
