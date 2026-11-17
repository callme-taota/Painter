import {t} from './i18n'

// Match the complete input, not a hostname substring or an author-provided
// player URL. Keep legacy av IDs as strings (they can exceed Number's range).
export function canonicalBilibiliURL(raw: unknown): string | null {
  if (typeof raw !== 'string') return null
  if (new TextEncoder().encode(raw).length > 2048 || /[\s\\\u0000-\u001f\u007f#]/.test(raw)) return null
  const at = raw.indexOf('?'), base = at < 0 ? raw : raw.slice(0, at), query = at < 0 ? undefined : raw.slice(at + 1)
  if (query === '') return null
  const match = /^https:\/\/www\.bilibili\.com\/video\/(BV[A-Za-z0-9]{10}|av[1-9][0-9]{0,17})\/?$/.exec(base!)
  if (!match) return null
  const seen = new Set<string>()
  let page = '1'
  if (query !== undefined) for (const pair of query.split('&')) {
    const at = pair.indexOf('='), key = pair.slice(0, at), value = pair.slice(at + 1)
    if (at < 1 || seen.has(key) || !['p', 'spm_id_from', 'vd_source'].includes(key)) return null
    seen.add(key)
    if (key === 'p') { if (!/^[1-9][0-9]{0,3}$/.test(value) || Number(value) > 1000) return null; page = value }
    else { try { if (new TextEncoder().encode(decodeURIComponent(value.replace(/\+/g, ' '))).length > 512) return null } catch { return null } }
  }
  return `https://www.bilibili.com/video/${match[1]}/` + (page !== '1' ? `?p=${page}` : '')
}

export function bilibiliPlayerURL(raw: string): string | null {
  if (canonicalBilibiliURL(raw) !== raw) return null
  const url = new URL(raw), id = url.pathname.split('/')[2]!
  const query = new URLSearchParams({[id.startsWith('BV') ? 'bvid' : 'aid']: id.startsWith('BV') ? id : id.slice(2), p: url.searchParams.get('p') || '1', autoplay: '0', danmaku: '0', refer: '0'})
  return 'https://player.bilibili.com/player.html?' + query.toString()
}

export function socialPlatform(raw: string): string {
  const host = new URL(raw).hostname.toLowerCase()
  if (['x.com','twitter.com'].some(domain=>host===domain||host.endsWith('.'+domain))) return 'X'
  if (host === 'weibo.com' || host.endsWith('.weibo.com') || host === 'weibo.cn' || host.endsWith('.weibo.cn')) return t('微博')
  if (host === 'xiaohongshu.com' || host.endsWith('.xiaohongshu.com') || host === 'xhslink.com' || host.endsWith('.xhslink.com')) return t('小红书')
  if (host === 'douyin.com' || host.endsWith('.douyin.com')) return t('抖音')
  if (host === 'bilibili.com' || host.endsWith('.bilibili.com') || host === 'b23.tv' || host.endsWith('.b23.tv')) return t('B站')
  return t('分享')
}
