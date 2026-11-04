import { randomID } from './random'
import { reactive } from 'vue'
export type Row = Record<string, any>
export const session = reactive<Row>({ loading:true, authenticated:false, user:null, csrf_token:'' })
export class APIError extends Error { constructor(public status:number, public code:string,message:string,public details:Row={}){super(message)} }
export async function api<T=Row>(path:string, options:RequestInit & {version?:number|string;idempotent?:boolean;key?:string}={}):Promise<T>{
 const headers = new Headers(options.headers)
 if(options.body && !(options.body instanceof FormData)) headers.set('Content-Type','application/json')
 if(options.method && !['GET','HEAD'].includes(options.method.toUpperCase())) {
  if(session.csrf_token) headers.set('X-CSRF-Token',session.csrf_token)
  if(options.version!==undefined) headers.set('If-Match',`"${options.version}"`)
  if(options.key) headers.set('Idempotency-Key',options.key)
  else if(!headers.has('Idempotency-Key')) headers.set('Idempotency-Key',randomID())
 }
 const res=await fetch('/api/v1'+path,{...options,headers,credentials:'same-origin'})
 const data=res.status===204?{}:await res.json().catch(()=>({}))
 if(!res.ok) throw new APIError(res.status,data.error?.code||'REQUEST_FAILED',data.error?.message||'请求失败，请稍后重试',data.error?.details)
 return data as T
}
export async function loadSession(){ try{Object.assign(session,await api('/session'))}finally{session.loading=false} }
export function list(data:any):Row[]{return Array.isArray(data)?data:(data?.items||[])}
export function date(value:any){ if(!value)return '—';const d=new Date(typeof value==='number'?value:value);return Number.isNaN(d.valueOf())?String(value):d.toLocaleDateString('zh-CN',{year:'numeric',month:'short',day:'numeric'}) }
export function download(name:string,text:string){const a=document.createElement('a');a.href=URL.createObjectURL(new Blob([text],{type:'text/markdown;charset=utf-8'}));a.download=name;a.click();setTimeout(()=>URL.revokeObjectURL(a.href),500)}
export const body=(value:any)=>JSON.stringify(value)
export async function allPages(path:string):Promise<Row[]>{let cursor='',out:Row[]=[];do{const data=await api(path+(cursor?(path.includes('?')?'&':'?')+'cursor='+encodeURIComponent(cursor):''));out.push(...list(data));if(!data.has_more)break;cursor=data.next_cursor||''}while(cursor);return out}
