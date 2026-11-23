import {api} from './api'
import {t} from './i18n'

export const imageTypes = ['image/jpeg','image/png','image/webp','image/gif'] as const
export type ImageFileIssue = 'type'|'size'|'empty'
export function imageFileIssue(file:File,maxBytes=10*1024*1024):ImageFileIssue|undefined {
  if (!(imageTypes as readonly string[]).includes(file.type)) return 'type'
  if (file.size > maxBytes) return 'size'
  if (!file.size) return 'empty'
}

export type UploadedMedia = {state:'ready';media_id:string;private_preview_url?:string}
// A transport acknowledgement is usable only after the media state machine
// confirms readiness. Uploading a file never saves or publishes its references.
export async function uploadMedia(file:File,options:{key?:string;signal?:AbortSignal}={}):Promise<UploadedMedia> {
  const data = new FormData()
  data.append('file',file,file.name)
  const result = await api<UploadedMedia>('/media',{method:'POST',body:data,...options})
  if (!result || result.state !== 'ready' || typeof result.media_id !== 'string' || !result.media_id) {
    throw new Error(t('服务器尚未确认图片可用，请重试。'))
  }
  return result
}

export function releaseUploadPreview(upload:{preview:string}|null|undefined) {
  if (upload?.preview) { URL.revokeObjectURL(upload.preview); upload.preview = '' }
}
