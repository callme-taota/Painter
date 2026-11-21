<script setup lang="ts">
import {computed,nextTick,onBeforeUnmount,onMounted,reactive,ref} from 'vue'
import {api,appendUnique,date,onSessionEnd,type Row} from '../api'
import {loadPublicSession,observePublicSession} from '../publicSession'
import {locale,t} from '../i18n'
import {toast} from '../toast'
import InfiniteList from './InfiniteList.vue'
import Icon from './Icon.vue'

const props=defineProps<{initial:Row;viewerId:string}>()
type FeedType='all'|'post'|'moment'
const types:{id:FeedType;label:string}[]=[{id:'all',label:t('全部')},{id:'post',label:t('文章')},{id:'moment',label:t('动态')}]
const resolveType=(value:unknown):FeedType=>value==='post'||value==='moment'?value:'all'
const kind=ref<FeedType>(resolveType(props.initial.type))
const items=ref<Row[]>(Array.isArray(props.initial.items)?props.initial.items:[])
const followingCount=ref(Number(props.initial.following_count)||0)
const cursor=ref(String(props.initial.next_cursor||'')),hasMore=ref(props.initial.has_more===true)
const loading=ref(false),verified=ref(false),verifying=ref(true),error=ref(''),appendError=ref('')
const failedImages=reactive(new Set<string>())
const root=document.querySelector<HTMLElement>('[data-following-feed]')
const rawLimit=new URL(location.href).searchParams.get('limit')
const pageSize=rawLimit&&/^\d+$/.test(rawLimit)?Math.max(1,Math.min(Number(rawLimit),50)):20
let requestVersion=0,verificationVersion=0,controller:AbortController|null=null,disposed=false,redirecting=false

function stopRequest(){requestVersion++;controller?.abort();controller=null;loading.value=false}
function clear(){stopRequest();items.value=[];cursor.value='';hasMore.value=false;followingCount.value=0;failedImages.clear();verified.value=false;error.value='';appendError.value=''}
function conceal(){verificationVersion++;stopRequest();verified.value=false;if(root)root.hidden=true}
function signIn(){
 if(disposed||redirecting)return
 redirecting=true;verificationVersion++;clear()
 if(root)root.hidden=true
 location.replace('/login?return_to='+encodeURIComponent('/following'))
}
const stopSessionEnd=onSessionEnd(()=>signIn())
const stopSessionObservation=observePublicSession(value=>{
 if(value&&(!value.authenticated||String(value.user?.id||'')!==props.viewerId))signIn()
})

function filterHref(value:FeedType){
 const query=new URLSearchParams({type:value})
 if(rawLimit)query.set('limit',String(pageSize))
 if(locale==='en')query.set('lang','en')
 return '/following?'+query.toString()
}
function resetCursorURL(replace=false){
 const url=filterHref(kind.value)
 if(location.pathname+location.search!==url)history[replace?'replaceState':'pushState']({...history.state},'',url)
}
function isOrdinaryClick(event:MouseEvent){return event.button===0&&!event.ctrlKey&&!event.metaKey&&!event.altKey&&!event.shiftKey}
function filter(event:MouseEvent,value:FeedType){
 if(!isOrdinaryClick(event)||!verified.value||verifying.value)return
 event.preventDefault()
 if(value===kind.value&&!error.value)return
 stopRequest();kind.value=value;items.value=[];cursor.value='';hasMore.value=false;failedImages.clear()
 resetCursorURL();void loadFeed()
}
function currentAccount(data:Row){return typeof data.viewer_id==='string'&&data.viewer_id===props.viewerId}
async function loadFeed(append=false,manual=false){
 if(disposed||redirecting||!verified.value||document.hidden||(append&&(loading.value||!hasMore.value||!cursor.value)))return
 stopRequest()
 const version=requestVersion,requestedType=kind.value,ownController=new AbortController()
 controller=ownController;loading.value=true;error.value='';appendError.value=''
 const query=new URLSearchParams({type:requestedType,limit:String(pageSize)})
 if(append)query.set('cursor',cursor.value)
 try{
  const data=await api<Row>('/me/feed?'+query.toString(),{signal:ownController.signal,cache:'no-store'})
  if(disposed||redirecting||version!==requestVersion)return
  if(!currentAccount(data)){signIn();return}
  if(data.type!==requestedType||!Array.isArray(data.items))throw new Error(t('关注更新未能载入，请重试。'))
  items.value=append?appendUnique(items.value,data.items):data.items
  followingCount.value=Number(data.following_count)||0
  cursor.value=String(data.next_cursor||'');hasMore.value=data.has_more===true&&!!cursor.value
  if(!append)failedImages.clear()
  if(manual)toast.success(t('关注更新已刷新。'),{id:'following-refresh'})
 }catch(failure){
  if(disposed||redirecting||version!==requestVersion||ownController.signal.aborted)return
  if((failure as {status?:number}).status===401){signIn();return}
  const message=t('关注更新未能载入，请重试。')
  if(append)appendError.value=message
  else error.value=message
  if(manual)toast.error(message,{id:'following-refresh'})
 }finally{
  if(version===requestVersion){controller=null;loading.value=false}
 }
}
async function verifyAccount(refresh=false){
 if(disposed||redirecting||document.hidden)return
 conceal();const version=verificationVersion
 verifying.value=true;error.value=''
 try{
  // Flush the private list out of the DOM while its wrapper is still hidden.
  // Only the heading and validation state can be revealed before authentication.
  await nextTick()
  if(disposed||redirecting||version!==verificationVersion)return
  if(root)root.hidden=false
  const account=await loadPublicSession(true)
  if(disposed||redirecting||version!==verificationVersion)return
  if(!account.authenticated||String(account.user?.id||'')!==props.viewerId){signIn();return}
  verified.value=true
  if(refresh){items.value=[];cursor.value='';hasMore.value=false;resetCursorURL(true);await loadFeed()}
 }catch{
  if(disposed||redirecting||version!==verificationVersion)return
  clear();error.value=t('登录状态暂不可用，请重试。')
 }finally{
  if(!disposed&&!redirecting&&version===verificationVersion){verifying.value=false;if(root)root.hidden=false}
 }
}
function refresh(){if(!verified.value){void verifyAccount(true);return}resetCursorURL(true);void loadFeed(false,true)}
function pageHidden(){conceal()}
function pageShown(event:PageTransitionEvent){if(event.persisted)void verifyAccount(true)}
function visibilityChanged(){if(document.hidden)conceal();else void verifyAccount(true)}
function historyChanged(){
 kind.value=resolveType(new URL(location.href).searchParams.get('type'))
 // A history entry may include an SSR cursor. Reloading verifies both its
 // account and pagination context without combining it with another list.
 location.reload()
}
const emptyTitle=computed(()=>followingCount.value===0?t('还没有关注作者'):kind.value==='all'?t('还没有可阅读的更新'):t('暂无这类更新'))

onMounted(()=>{
 window.addEventListener('pagehide',pageHidden)
 window.addEventListener('pageshow',pageShown)
 window.addEventListener('popstate',historyChanged)
 document.addEventListener('visibilitychange',visibilityChanged)
 if(props.initial.viewer_id!==props.viewerId){signIn();return}
 void verifyAccount()
})
onBeforeUnmount(()=>{
 disposed=true;verificationVersion++;clear();stopSessionEnd();stopSessionObservation()
 window.removeEventListener('pagehide',pageHidden)
 window.removeEventListener('pageshow',pageShown)
 window.removeEventListener('popstate',historyChanged)
 document.removeEventListener('visibilitychange',visibilityChanged)
})
</script>

<template>
 <header class="following-heading"><h1>{{t('关注更新')}}</h1></header>
 <div class="following-toolbar">
  <nav class="following-filters" :aria-label="t('内容类型')"><a v-for="option in types" :key="option.id" :href="filterHref(option.id)" :aria-current="kind===option.id?'page':undefined" @click="filter($event,option.id)">{{option.label}}</a></nav>
  <div class="following-actions"><a href="/me/following">{{t('管理关注')}}</a><button type="button" class="quiet following-refresh" :disabled="loading||verifying" @click="refresh"><svg class="icon" viewBox="0 0 24 24" aria-hidden="true"><path d="M20 11A8 8 0 0 0 6 6L3 9m0-6v6h6M4 13a8 8 0 0 0 14 5l3-3m0 6v-6h-6"/></svg>{{t('刷新')}}</button></div>
 </div>
 <p v-if="verifying" class="following-status" role="status">{{t('正在确认登录状态…')}}</p>
 <div v-if="error" class="following-error" role="alert"><span>{{error}}</span><button type="button" class="quiet" :disabled="loading||verifying" @click="verified?loadFeed():verifyAccount(true)">{{t('重试')}}</button></div>
 <template v-if="verified&&!verifying">
  <p v-if="loading&&!items.length" class="following-status" role="status">{{t('正在载入…')}}</p>
  <div v-else-if="!items.length&&!error" class="following-empty"><h2>{{emptyTitle}}</h2><p>{{followingCount===0?t('去站内认识作者，关注后在这里阅读他们的文章与动态。'):t('作者新公开的文章与动态会出现在这里。')}}</p><a v-if="followingCount===0" class="text-link" href="/discover">{{t('发现作者')}}<Icon name="arrow" :size="16"/></a><a v-else-if="kind!=='all'" class="text-link" :href="filterHref('all')" @click="filter($event,'all')">{{t('查看全部更新')}}<Icon name="arrow" :size="16"/></a></div>
  <div class="following-items">
   <article v-for="item in items" :key="item.id" class="following-item" :class="'following-item-'+item.type" :data-list-key="item.id">
    <div class="following-byline"><a class="following-author" :href="'/users/'+item.author.id"><img v-if="item.author.avatar_url" :src="item.author.avatar_url" width="28" height="28" alt="" loading="lazy"><span v-else class="following-avatar" aria-hidden="true">{{String(item.author.display_name||'?').slice(0,1)}}</span><span>{{item.author.display_name}}</span></a><span class="following-item-type">{{item.type==='post'?t('文章'):t('动态')}}</span><time :datetime="item.published_at">{{date(item.published_at)}}</time></div>
    <div v-if="item.type==='post'" class="following-post-content" :class="{'has-cover':item.cover_url&&!failedImages.has(item.id)}"><div class="following-copy"><h2><a :href="item.url">{{item.title}}</a></h2><p v-if="item.summary">{{item.summary}}</p></div><a v-if="item.cover_url&&!failedImages.has(item.id)" class="following-post-cover" :href="item.url" aria-hidden="true" tabindex="-1"><img :src="item.cover_url" alt="" loading="lazy" decoding="async" @error="failedImages.add(item.id)"></a></div>
    <template v-else><a v-if="item.summary" class="following-moment-text" :href="item.url"><p>{{item.summary}}</p></a><a v-if="item.cover_url&&!failedImages.has(item.id)" class="following-moment-cover" :href="item.url" :aria-label="t('查看动态')"><img :src="item.cover_url" :alt="t('动态图片')" loading="lazy" decoding="async" @error="failedImages.add(item.id)"><span v-if="item.image_count>1" class="following-image-count">{{t('共 {count} 张图片',{count:item.image_count})}}</span></a><a v-else-if="item.cover_url" class="following-image-failed" :href="item.url">{{t('图片暂时无法加载。')}} {{t('查看动态')}}</a></template>
   </article>
  </div>
  <InfiniteList :has-more="hasMore" :loading="loading" :disabled="!verified||verifying" :error="appendError" @load="loadFeed(true)"/>
  <p v-if="items.length&&!hasMore&&!loading" class="following-end" role="status">{{t('已显示全部内容')}}</p>
 </template>
</template>
