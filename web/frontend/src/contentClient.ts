import {api,body,type Row} from './api'
import {withCursor} from './pagination'

type Version = number|string
type WriteOptions = {version:Version;key?:string}
type ReadOptions = {signal?:AbortSignal}

// Articles and moments share a draft/publication lifecycle. Keep their wire
// payloads and domain-specific capabilities explicit at the call site.
function contentClient(kind:'posts'|'moments') {
  const resource = (id:string)=>`/studio/${kind}/${encodeURIComponent(id)}`
  const write = (id:string,suffix:string,method:string,options:WriteOptions,payload?:Row)=>
    api(resource(id)+suffix,{...options,method,...(payload===undefined?{}:{body:body(payload)})})
  return {
    list:(query:URLSearchParams,cursor='',options:ReadOptions={})=>api(withCursor(`/me/${kind}?${query}`,cursor),options),
    read:(id:string,options:ReadOptions={})=>api(resource(id),options),
    create:(payload:Row,key:string)=>api(`/${kind}`,{method:'POST',key,body:body(payload)}),
    saveDraft:(id:string,payload:Row,options:WriteOptions)=>write(id,'/draft','PUT',options,payload),
    publish:(id:string,options:WriteOptions,payload?:Row)=>write(id,'/publish','POST',options,payload),
    withdraw:(id:string,options:WriteOptions,payload?:Row)=>write(id,'/publication','DELETE',options,payload),
    remove:(id:string,options:WriteOptions)=>write(id,'','DELETE',options),
  }
}

export const moments = contentClient('moments')
export const posts = {
  ...contentClient('posts'),
  byClientID:(id:string)=>api(`/me/posts/by-client-id/${encodeURIComponent(id)}`),
  recover:(id:string,version:Version)=>api(`/studio/posts/${encodeURIComponent(id)}/recover`,{method:'POST',version}),
  revisions:(id:string)=>api(`/studio/posts/${encodeURIComponent(id)}/revisions`),
  checkpoint:(id:string,version:Version,payload:Row)=>api(`/studio/posts/${encodeURIComponent(id)}/checkpoints`,{method:'POST',version,body:body(payload)}),
  restore:(id:string,version:Version,revisionID:string)=>api(`/studio/posts/${encodeURIComponent(id)}/restore`,{method:'POST',version,body:body({revision_id:revisionID})}),
}
