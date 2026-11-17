/** Thumbnail presentation never alters the original file. Enhancement stays local:
 * one tiny canvas per presentation ratio, no request or scroll-time work. */
const initialized = new WeakSet<HTMLImageElement>()
const sampled = new WeakMap<HTMLImageElement, Map<number, {left: string, right: string}>>()
let breakpointBound = false

function schedule(image: HTMLImageElement) {
  if (typeof window.requestIdleCallback === 'function') window.requestIdleCallback(() => enhance(image), {timeout: 600})
  else setTimeout(() => enhance(image), 0)
}

function enhance(image: HTMLImageElement) {
  const preview = image.closest<HTMLElement>('.moment-preview')
  if (!preview || !image.isConnected || !image.naturalWidth || !image.naturalHeight) return
  const width = image.naturalWidth, height = image.naturalHeight
  const long = preview.dataset.coverShape === 'long'
  // SSR reserves the frame before the image arrives. Never resize it on load.
  const parts = getComputedStyle(preview).aspectRatio.split('/').map(Number)
  const previewRatio = parts[0]! / (parts[1] || 1)
  if (!Number.isFinite(previewRatio) || previewRatio <= 0) return
  let colors = sampled.get(image)
  if (!colors) { colors = new Map(); sampled.set(image, colors) }
  function apply(left: string, right: string) {
    preview!.style.setProperty('--moment-color-left', left)
    preview!.style.setProperty('--moment-color-right', right)
    preview!.dataset.coverColor = 'ready'
  }
  const cached = colors.get(previewRatio)
  if (cached) { apply(cached.left, cached.right); return }
  // Sample only the visible crop's lower part so the halo agrees with the card.
  const cropWidth = Math.min(width, height * previewRatio)
  const cropHeight = Math.min(height, width / previewRatio)
  const x = (width - cropWidth) / 2, y = long ? 0 : (height - cropHeight) / 2
  try {
    if (new URL(image.currentSrc || image.src, location.href).origin !== location.origin) return
    const canvas = document.createElement('canvas')
    canvas.width = 24; canvas.height = 8
    const context = canvas.getContext('2d', {willReadFrequently: true})
    if (!context) return
    context.drawImage(image, x, y + cropHeight * .62, cropWidth, cropHeight * .38, 0, 0, 24, 8)
    const pixels = context.getImageData(0, 0, 24, 8).data
    function average(start: number, end: number) {
      const channels = [0, 0, 0]
      let weight = 0
      for (let row = 0; row < 8; row++) for (let col = start; col < end; col++) {
        const at = (row * 24 + col) * 4, alpha = pixels[at + 3]! / 255
        weight += alpha
        channels.forEach((_, channel) => { channels[channel]! += pixels[at + channel]! * alpha })
      }
      return weight > 0 ? channels.map(value => Math.round(value / weight)).join(' ') : null
    }
    const left = average(0, 12), right = average(12, 24)
    if (!left || !right) return
    colors.set(previewRatio, {left, right})
    apply(left, right)
  } catch { /* Canvas/CORS/decoding restrictions keep the neutral CSS fallback. */ }
}

export function initMomentCovers() {
  if (!breakpointBound) {
    breakpointBound = true
    // Desktop/mobile can crop wide photos differently. Reuse both cached colors
    // on subsequent switches; no ordinary resize or scroll listener is needed.
    matchMedia('(max-width:720px)').addEventListener('change', () => {
      for (const image of document.querySelectorAll<HTMLImageElement>('.moment-cover')) {
        if (image.complete && image.naturalWidth) schedule(image)
      }
    })
  }
  for (const image of document.querySelectorAll<HTMLImageElement>('.moment-cover')) {
    if (initialized.has(image)) continue
    initialized.add(image)
    const failed = () => image.closest('.moment-preview')?.setAttribute('data-cover-failed', '')
    const loaded = () => {
      // Optional color enhancement stays away from interaction.
      const preview = image.closest<HTMLElement>('.moment-preview')
      if (preview && image.naturalWidth && image.naturalHeight) {
        preview.removeAttribute('data-cover-failed')
      }
      schedule(image)
    }
    image.addEventListener('error', failed, {once: true})
    image.addEventListener('load', loaded, {once: true})
    if (image.complete) {
      if (image.naturalWidth) { image.removeEventListener('load', loaded); loaded() }
      else failed()
    }
  }
}
