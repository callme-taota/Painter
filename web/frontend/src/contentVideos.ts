import {t} from './i18n'
import {bilibiliPlayerURL} from './articleEmbeds'

const enhanced = new WeakSet<HTMLElement>()
let active: {card: HTMLElement, reset: () => void} | undefined
let pageBound = false

export function initContentVideos(root: ParentNode = document) {
  if (active && !active.card.isConnected) active.reset()
  if (!pageBound) { pageBound = true; addEventListener('pagehide', () => active?.reset()) }
  const cards = Array.from(root.querySelectorAll<HTMLElement>('[data-content-video="bilibili"]'))
  if (root instanceof HTMLElement && root.matches('[data-content-video="bilibili"]')) cards.unshift(root)
  for (const card of cards) {
    // In the writing surface an atom is selected/edited, never played.
    if (enhanced.has(card) || card.closest('.ProseMirror,[data-painter-card]')) continue
    const source = card.querySelector<HTMLAnchorElement>('a.content-video__source')
    const stage = card.querySelector<HTMLElement>('.content-video__stage')
    const load = card.querySelector<HTMLButtonElement>('button[data-video-load]')
    const details = card.querySelector<HTMLElement>('.content-video__details')
    const player = bilibiliPlayerURL(source?.getAttribute('href') || '')
    if (!source || !stage || !load || !details || !player) continue
    enhanced.add(card)
    load.hidden = false
    load.textContent = t('加载B站播放器')
    const destination = source.querySelector('.content-video__destination')
    if (destination) destination.textContent = t('在B站打开')
    let frame: HTMLIFrameElement | undefined, controls: HTMLElement | undefined, timeout: ReturnType<typeof setTimeout> | undefined
    function reset() {
      clearTimeout(timeout)
      frame?.remove(); frame = undefined
      controls?.remove(); controls = undefined
      card.dataset.videoState = 'idle'
      stage!.removeAttribute('aria-busy')
      load!.hidden = false
      if (active?.card === card) active = undefined
    }
    function play() {
      if (frame || !card.isConnected) return
      const currentPlayer = bilibiliPlayerURL(source!.getAttribute('href') || '')
      if (!currentPlayer) { load!.hidden = true; return }
      active?.reset()
      active = {card, reset}
      load!.hidden = true
      card.dataset.videoState = 'loading'
      stage!.setAttribute('aria-busy', 'true')
      const actions = document.createElement('div'); actions.className = 'content-video__controls'
      const status = document.createElement('p'); status.className = 'content-video__status'; status.setAttribute('role', 'status'); status.textContent = t('正在加载B站播放器…')
      const close = document.createElement('button'); close.type = 'button'; close.className = 'content-video__close'; close.textContent = t('关闭播放器')
      const retry = document.createElement('button'); retry.type = 'button'; retry.className = 'content-video__retry'; retry.textContent = t('重试加载'); retry.hidden = true
      close.addEventListener('click', () => { reset(); load!.focus() })
      retry.addEventListener('click', () => { reset(); play() })
      actions.append(status, retry, close); details!.append(actions); controls = actions
      const iframe = document.createElement('iframe')
      iframe.className = 'content-video__frame'
      iframe.title = t('B站视频：{title}', {title: card.querySelector('.content-video__title')?.textContent || t('哔哩哔哩视频')})
      iframe.setAttribute('sandbox', 'allow-scripts allow-same-origin allow-presentation')
      iframe.allow = 'fullscreen; picture-in-picture'
      iframe.allowFullscreen = true
      iframe.referrerPolicy = 'no-referrer'
      iframe.src = currentPlayer
      iframe.addEventListener('load', () => {
        if (frame !== iframe) return
        clearTimeout(timeout); stage!.removeAttribute('aria-busy'); card.dataset.videoState = 'loaded'
        status.textContent = t('视频无法播放时，可在B站打开')
      }, {once: true})
      iframe.addEventListener('error', () => {
        if (frame !== iframe) return
        clearTimeout(timeout); stage!.removeAttribute('aria-busy')
        status.textContent = t('播放器暂时无法加载，可重试或在B站打开'); retry.hidden = false
      }, {once: true})
      timeout = setTimeout(() => {
        if (frame !== iframe) return
        stage!.removeAttribute('aria-busy')
        status.textContent = t('播放器加载较慢，可重试或在B站打开'); retry.hidden = false
      }, 12000)
      frame = iframe; stage!.append(iframe)
      iframe.focus()
    }
    load.addEventListener('click', play)
  }
}
