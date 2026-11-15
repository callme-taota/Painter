import {randomID} from './random'
import {toast} from './toast'
import {invalidatePublicSession,loadPublicSession,observePublicSession,type PublicSession} from './publicSession'
export type LikeState={postId:string;liked:boolean;bookmarked:boolean;count:number;busy:boolean;session:PublicSession|null;loaded:boolean;loadedAt:number;error:string}
const states=new Map<string,LikeState>(),loads=new Map<string,Promise<LikeState>>()
function announce(state:LikeState){window.dispatchEvent(new CustomEvent('painter:like',{detail:{...state}}))}
export function clearLikeSessions(){loads.clear();for(const state of states.values()){state.session={authenticated:false};state.liked=false;state.bookmarked=false;state.loaded=false;state.error='';announce(state)}}
observePublicSession(value=>{if(value?.authenticated===false)clearLikeSessions()})
export function getLike(id:string){if(!states.has(id))states.set(id,{postId:id,liked:false,bookmarked:false,count:0,busy:false,session:null,loaded:false,loadedAt:0,error:''});return states.get(id)!}
async function read(path:string){const response=await fetch('/api/v1'+path,{credentials:'same-origin',cache:'no-store'});const data=await response.json().catch(()=>({}));if(!response.ok){if(response.status===401)invalidatePublicSession(true);throw new Error(data.error?.message||'互动状态暂不可用，请重试。')}return data}
export function setBookmarked(id:string,bookmarked:boolean){const state=getLike(id);state.bookmarked=bookmarked;announce(state)}
export async function loadLike(id:string):Promise<LikeState>{
 if(loads.has(id))return loads.get(id)!
 const state=getLike(id)
 const request=(async()=>{try{
  const current=await loadPublicSession()
  if(!current.authenticated){state.session=current;state.loaded=true;state.liked=false;state.bookmarked=false;state.error='';return state}
  if(state.loaded&&String(state.session?.user?.id)===String(current.user?.id)&&Date.now()-state.loadedAt<60_000){state.session=current;return state}
  state.session=current
  const [metrics,relations]=await Promise.all([read(`/posts/${id}/metrics`),read(`/me/post-states?ids=${encodeURIComponent(id)}`)])
  // A concurrent 401/sign-out can clear this state while the requests settle.
  if(state.session!==current||!state.session.authenticated)return state
  state.count=Number(metrics.like_count??metrics.likes??0);state.liked=!!relations.items?.[0]?.liked;state.bookmarked=!!relations.items?.[0]?.bookmarked;state.loaded=true;state.loadedAt=Date.now();state.error=''
 }catch(e){state.error=(e as Error).message;state.loaded=false}finally{announce(state)}return state})()
 loads.set(id,request)
 try{return await request}finally{if(loads.get(id)===request)loads.delete(id)}
}
export async function toggleLike(id:string){
 const state=await loadLike(id);if(!state.loaded||state.busy||!state.session?.authenticated){if(state.error)toast.error(state.error,{id:'post-like:'+id});return state}
 state.busy=true;state.error='';announce(state)
 try{
  const response=await fetch(`/api/v1/posts/${id}/like`,{method:state.liked?'DELETE':'PUT',credentials:'same-origin',headers:{'X-CSRF-Token':state.session.csrf_token||'','Idempotency-Key':randomID()}})
  if(!response.ok){if(response.status===401){invalidatePublicSession(true);toast.error('登录已失效，请重新登录。',{id:'session-expired'});return state}const data=await response.json().catch(()=>({}));throw new Error(data.error?.message||'点赞未能保存，请重试。')}
  state.liked=!state.liked;state.count=Math.max(0,state.count+(state.liked?1:-1));announce(state)
  try{const metrics=await read(`/posts/${id}/metrics`);state.count=Number(metrics.like_count??metrics.likes??0);state.loadedAt=Date.now()}catch{toast.warning('点赞状态已更新，计数暂时无法刷新。',{id:'post-like:'+id})}
 }catch(e){state.error='';toast.error((e as Error).message,{id:'post-like:'+id})}finally{state.busy=false;announce(state)}return state
}
