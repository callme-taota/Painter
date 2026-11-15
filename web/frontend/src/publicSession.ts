import {t,locale} from './i18n'
/** Session data belongs to this document only. No credentials are persisted. */
export type PublicSession = {authenticated:boolean;user?:Record<string,any>|null;csrf_token?:string;[key:string]:any}
let value:PublicSession|null=null,expires=0,pending:Promise<PublicSession>|null=null,generation=0
const listeners=new Set<(value:PublicSession|null)=>void>()
const lifetime=60_000
function notify(){for(const listener of listeners)listener(value)}
export function observePublicSession(listener:(value:PublicSession|null)=>void){listeners.add(listener);return()=>listeners.delete(listener)}
export function invalidatePublicSession(anonymous=false){generation++;pending=null;value=anonymous?{authenticated:false,user:null,csrf_token:''}:null;expires=anonymous?Date.now()+lifetime:0;notify()}
export async function loadPublicSession(force=false):Promise<PublicSession>{
 if(force)invalidatePublicSession()
 if(value&&Date.now()<expires)return value
 if(pending)return pending
 const ownGeneration=generation
 const request=(async()=>{
  const response=await fetch('/api/v1/session',{credentials:'same-origin',cache:'no-store',headers:{'Accept-Language':locale}})
  if(!response.ok)throw Object.assign(new Error(t('登录状态暂不可用，请重试。')),{status:response.status})
  const data=await response.json()
  if(typeof data.authenticated!=='boolean')throw new Error(t('登录状态未完整载入，请重试。'))
  // A response started before sign-out must not restore the old account.
  if(ownGeneration!==generation)return value||loadPublicSession()
  value=data.authenticated?data:{authenticated:false,user:null,csrf_token:''};expires=Date.now()+lifetime;notify()
  return value!
 })()
 pending=request
 try{return await request}finally{if(pending===request)pending=null}
}
if(typeof window!=='undefined'){
 window.addEventListener('pageshow',event=>{if(event.persisted)invalidatePublicSession()})
 // A background tab must re-check the account before its next interaction load.
 document.addEventListener('visibilitychange',()=>{if(document.visibilityState==='hidden')expires=0})
}
