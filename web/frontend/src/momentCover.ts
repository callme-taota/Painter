/** Thumbnail presentation never alters the original file. Enhancement stays local:
 * one tiny canvas per loaded cover, no additional request or scroll-time work. */
const initialized = new WeakSet<HTMLImageElement>()
const previewRatio = 4 / 5

function enhance(image: HTMLImageElement) {
  const preview = image.closest<HTMLElement>('.moment-preview')
  if (!preview || !image.isConnected || !image.naturalWidth || !image.naturalHeight) return
  const width = image.naturalWidth, height = image.naturalHeight
  const long = height > width * 2
  preview.dataset.coverShape = long ? 'long' : width > height ? 'landscape' : 'portrait'
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
    preview.style.setProperty('--moment-color-left', left)
    preview.style.setProperty('--moment-color-right', right)
    preview.dataset.coverColor = 'ready'
  } catch { /* Canvas/CORS/decoding restrictions keep the neutral CSS fallback. */ }
}

export function initMomentCovers() {
  for (const image of document.querySelectorAll<HTMLImageElement>('.moment-cover')) {
    if (initialized.has(image)) continue
    initialized.add(image)
    const failed = () => image.closest('.moment-preview')?.setAttribute('data-cover-failed', '')
    const loaded = () => {
      // Classify immediately; optional sampling is scheduled away from interaction.
      const preview = image.closest<HTMLElement>('.moment-preview')
      if (preview && image.naturalWidth && image.naturalHeight) {
        preview.removeAttribute('data-cover-failed')
        preview.dataset.coverShape = image.naturalHeight > image.naturalWidth * 2 ? 'long' : image.naturalWidth > image.naturalHeight ? 'landscape' : 'portrait'
      }
      if (typeof window.requestIdleCallback === 'function') window.requestIdleCallback(() => enhance(image), {timeout: 600})
      else setTimeout(() => enhance(image), 0)
    }
    image.addEventListener('error', failed, {once: true})
    image.addEventListener('load', loaded, {once: true})
    if (image.complete) {
      if (image.naturalWidth) { image.removeEventListener('load', loaded); loaded() }
      else failed()
    }
  }
}
