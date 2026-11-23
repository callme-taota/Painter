import {onScopeDispose} from 'vue'
import {uploadMedia,type UploadedMedia} from './mediaUpload'

type UploadState = {state:'queued'|'uploading'|'failed'|'ready';error:string}
type Options<T extends UploadState> = {
  next:()=>T|undefined
  file:(upload:T)=>File|undefined
  key:(upload:T)=>string
  uploaded:(upload:T,result:UploadedMedia)=>void|Promise<void>
  failed?:(upload:T,cause:unknown)=>void
}

// The page owns its records, selection limits and attachment semantics. One
// queue owns at most one in-flight request; retries reuse the record's file/key.
export function useMediaUploadQueue<T extends UploadState>(options:Options<T>) {
  let running=false,disposed=false,controller:AbortController|undefined
  async function transfer(upload:T):Promise<boolean> {
    const file=options.file(upload)
    if(!file)return false
    upload.state='uploading';upload.error=''
    controller=new AbortController()
    try {
      const result=await uploadMedia(file,{key:options.key(upload),signal:controller.signal})
      if(disposed)return false
      await options.uploaded(upload,result)
      if(disposed)return false
      upload.state='ready'
      return true
    } catch(cause) {
      if(!disposed){upload.state='failed';upload.error=(cause as Error).message;options.failed?.(upload,cause)}
      return false
    } finally {controller=undefined}
  }
  async function send(upload:T):Promise<boolean> {
    if(running||disposed)return false
    running=true
    try{return await transfer(upload)}finally{running=false}
  }
  async function run():Promise<void> {
    if(running||disposed)return
    running=true
    try {
      while(!disposed){const upload=options.next();if(!upload||!options.file(upload))break;await transfer(upload)}
    } finally {running=false}
  }
  onScopeDispose(()=>{disposed=true;controller?.abort()})
  return {run,send}
}
