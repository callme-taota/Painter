/** Enhance the optional, server-authorized right-rail menu without extra requests. */
export function initCreationControls() {
  document.querySelectorAll<HTMLDetailsElement>('.floating-create').forEach(menu => {
    const summary = menu.querySelector<HTMLElement>('summary')
    const items = Array.from(menu.querySelectorAll<HTMLAnchorElement>('.create-popover a'))
    if (!summary || !items.length) return
    const close = (restoreFocus = false) => {
      menu.open = false
      if (restoreFocus) summary.focus()
    }
    summary.addEventListener('keydown', event => {
      if (event.key !== 'ArrowDown' && event.key !== 'ArrowUp') return
      event.preventDefault()
      menu.open = true
      items[event.key === 'ArrowDown' ? 0 : items.length - 1]?.focus()
    })
    menu.addEventListener('keydown', event => {
      if (event.key === 'Escape' && menu.open) {
        event.preventDefault()
        close(true)
        return
      }
      if (event.target === summary) return
      const index = items.indexOf(document.activeElement as HTMLAnchorElement)
      if (index < 0) return
      let target: number | undefined
      if (event.key === 'ArrowDown') target = (index + 1) % items.length
      if (event.key === 'ArrowUp') target = (index - 1 + items.length) % items.length
      if (event.key === 'Home') target = 0
      if (event.key === 'End') target = items.length - 1
      if (target !== undefined) { event.preventDefault(); items[target]?.focus() }
    })
    menu.addEventListener('toggle', () => {
      if (!menu.open) return
      document.querySelectorAll<HTMLDetailsElement>('.floating-controls details[open]').forEach(other => {
        if (other !== menu && !other.contains(menu) && !menu.contains(other)) other.open = false
      })
    })
    document.addEventListener('click', event => {
      if (menu.open && event.target instanceof Node && !menu.contains(event.target)) close()
    })
    menu.addEventListener('focusout', event => {
      if (event.relatedTarget instanceof Node) {
        if (!menu.contains(event.relatedTarget)) close()
        return
      }
      // Pointer focus can temporarily pass through body between blur and focus.
      setTimeout(() => {
        if (menu.open && !menu.contains(document.activeElement)) close()
      }, 0)
    })
  })
}
