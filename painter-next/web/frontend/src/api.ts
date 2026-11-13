import { randomID } from './random'
import { reactive } from 'vue'
import {invalidatePublicSession,loadPublicSession,observePublicSession} from './publicSession'
export type Row = Record<string, any>
export const session = reactive<Row>({ loading:true, authenticated:false, user:null, csrf_token:'' })
const pendingGets=new Map<string,Promise<any>>()
export type SessionEndReason='signout'|'expired'
const sessionEndHandlers=new Set<(reason:SessionEndReason)=>void|Promise<void>>()
export function onSessionEnd(handler:(reason:SessionEndReason)=>void|Promise<void>){sessionEndHandlers.add(handler);return()=>sessionEndHandlers.delete(handler)}
let expiry:Promise<void>|null=null,expired=false,expiryNotified=false,signOutRequests=0,explicitlySignedOut=false
function notifyExpiry(){if(expired&&!expiryNotified&&!signOutRequests&&!explicitlySignedOut){expiryNotified=true;window.dispatchEvent(new CustomEvent('painter:session-expired'))}}
async function finishLocalSession(reason:SessionEndReason){await Promise.allSettled([...sessionEndHandlers].map(handler=>Promise.resolve().then(()=>handler(reason))))}
async function expireSession(ownSignOut=false){
 if(expired){notifyExpiry();return}
 if(expiry)return expiry
 if(signOutRequests&&!ownSignOut)return
 const operation=(async()=>{await finishLocalSession('expired');expired=true;invalidatePublicSession(true);notifyExpiry()})()
 expiry=operation
 try{await operation}finally{if(expiry===operation)expiry=null}
}
observePublicSession(value=>{if(!value)return;if(!value.authenticated&&session.authenticated&&!expired&&!explicitlySignedOut){void expireSession();return}Object.assign(session,{user:null,csrf_token:''},value)})
export class APIError extends Error { constructor(public status:number, public code:string,message:string,public details:Row={}){super(message)} }
export async function api<T=Row>(path:string, options:RequestInit & {version?:number|string;idempotent?:boolean;key?:string}={}):Promise<T>{
 // Only default GETs share a pending request. A caller's AbortSignal, cache,
 // headers or other Fetch options always keep their original semantics.
 const shared=Object.keys(options).length===0
 if(shared&&pendingGets.has(path))return pendingGets.get(path)!
 const request=perform<T>(path,options)
 if(!shared)return request
 pendingGets.set(path,request)
 try{return await request}finally{if(pendingGets.get(path)===request)pendingGets.delete(path)}
}
async function perform<T>(path:string,options:RequestInit & {version?:number|string;idempotent?:boolean;key?:string}):Promise<T>{
 const method=(options.method||'GET').toUpperCase(),signOut=path==='/session'&&method==='DELETE'
 if(signOut)signOutRequests++
 try{
  const headers=new Headers(options.headers)
  if(options.body&&!(options.body instanceof FormData))headers.set('Content-Type','application/json')
  if(!['GET','HEAD'].includes(method)){
   if(session.csrf_token)headers.set('X-CSRF-Token',session.csrf_token)
   if(options.version!==undefined)headers.set('If-Match',`"${options.version}"`)
   if(options.key)headers.set('Idempotency-Key',options.key)
   else if(!headers.has('Idempotency-Key'))headers.set('Idempotency-Key',randomID())
  }
  const res=await fetch('/api/v1'+path,{...options,headers,credentials:'same-origin'})
  const data=res.status===204?{}:await res.json().catch(()=>({}))
  if(!res.ok){if(res.status===401)await expireSession(signOut);throw new APIError(res.status,data.error?.code||'REQUEST_FAILED',data.error?.message||'请求失败，请稍后重试',data.error?.details)}
  if(signOut){await finishLocalSession('signout');explicitlySignedOut=true}
  // A post-write read must not join a pre-write read.
  if(!['GET','HEAD'].includes(method))pendingGets.clear()
  if(!['GET','HEAD'].includes(method)&&(path==='/session'||path==='/me'||path.startsWith('/me/mfa/')||path==='/me/password'))invalidatePublicSession(signOut)
  return data as T
 }finally{if(signOut){signOutRequests--;notifyExpiry()}}
}
export async function loadSession(force=false){try{const value=await loadPublicSession(force);if(!value.authenticated&&session.authenticated){await expireSession();return}if(value.authenticated){expired=false;expiryNotified=false;explicitlySignedOut=false}Object.assign(session,{user:null,csrf_token:''},value)}finally{session.loading=false}}
export function list(data:any):Row[]{return Array.isArray(data)?data:(data?.items||[])}
export function date(value:any){ if(!value)return '—';const d=new Date(typeof value==='number'?value:value);return Number.isNaN(d.valueOf())?String(value):d.toLocaleDateString('zh-CN',{year:'numeric',month:'short',day:'numeric'}) }
export function download(name:string,text:string){const a=document.createElement('a');a.href=URL.createObjectURL(new Blob([text],{type:'text/markdown;charset=utf-8'}));a.download=name;a.click();setTimeout(()=>URL.revokeObjectURL(a.href),500)}
export const body=(value:any)=>JSON.stringify(value)
export async function allPages(path:string):Promise<Row[]>{let cursor='',out:Row[]=[];do{const data=await api(path+(cursor?(path.includes('?')?'&':'?')+'cursor='+encodeURIComponent(cursor):''));out.push(...list(data));if(!data.has_more)break;cursor=data.next_cursor||''}while(cursor);return out}
