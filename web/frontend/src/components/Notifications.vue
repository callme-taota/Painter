<script setup lang="ts">
import {computed,onBeforeUnmount,onMounted,ref,watch} from 'vue'
import {api,APIError,date,onSessionEnd,session,type Row} from '../api'
import {loadPublicSession,observePublicSession} from '../publicSession'
import {useCursorList} from '../useCursorList'
import {withCursor} from '../pagination'
import {t} from '../i18n'
import {toast} from '../toast'
import InfiniteList from './InfiniteList.vue'
import '../notifications.css'

const props=defineProps<{viewerId:string}>()
const root=ref<HTMLElement>(),filter=ref<'all'|'unread'>('all'),unread=ref<number|null>(null),busy=ref(''),verified=ref(true),verificationError=ref('')
let disposed=false,epoch=0,redirecting=false
const numberID=(value:unknown)=>typeof value==='string'&&/^[1-9][0-9]*$/.test(value)
function parse(row:Row):Row {
 if(!row||!numberID(row.id)||!['comment','reply'].includes(row.kind)||typeof row.unread!=='boolean'||typeof row.available!=='boolean'||typeof row.created_at!=='string'||!Number.isFinite(Date.parse(row.created_at)))throw new Error(t('通知数据未完整载入，请重试。'))
 if(row.available&&(!numberID(row.actor?.id)||typeof row.actor.display_name!=='string'||typeof row.post_title!=='string'||typeof row.body!=='string'||!numberID(row.post_id)||!numberID(row.comment_id)))throw new Error(t('通知数据未完整载入，请重试。'))
 return row
}
const {items,loading,error,hasMore,load,reset}=useCursorList({
 request:async(cursor,signal)=>{
  const data=await api(withCursor('/me/notifications?filter='+filter.value,cursor),{signal,cache:'no-store'})
  if(data.viewer_id!==props.viewerId){signIn();throw new Error(t('账号已变化，请重新登录。'))}
  return data
 },
 disabled:()=>!!busy.value||!verified.value,
 parse,
 received(data){
  if(data.viewer_id!==props.viewerId){signIn();throw new Error(t('账号已变化，请重新登录。'))}
  if(data.filter!==filter.value||!Number.isSafeInteger(data.unread_count)||data.unread_count<0)throw new Error(t('通知数据未完整载入，请重试。'))
  unread.value=data.unread_count
 },
 failed(cause){if(cause instanceof APIError&&cause.status===401)conceal()},
})
const unreadLabel=computed(()=>unread.value===null?t('正在读取通知…'):t('{value1} 条未读',{value1:unread.value}))
function conceal(){epoch++;verified.value=false;if(root.value)root.value.hidden=true;reset();unread.value=null}
function signIn(){if(disposed||redirecting)return;redirecting=true;conceal();location.replace('/login?return_to='+encodeURIComponent('/me/notifications'))}
const stopSessionEnd=onSessionEnd(()=>conceal())
const stopObservation=observePublicSession(value=>{if(value&&(!value.authenticated||String(value.user?.id||'')!==props.viewerId))signIn()})
watch(()=>session.user?.id,id=>{if(id&&String(id)!==props.viewerId)signIn()})
async function verify(){
 conceal();const at=epoch;verificationError.value=''
 try{
  const account=await loadPublicSession(true)
  if(disposed||at!==epoch)return
  if(!account.authenticated||String(account.user?.id||'')!==props.viewerId){signIn();return}
  verified.value=true;if(root.value)root.value.hidden=false;await load()
 }catch{if(!disposed&&at===epoch){verificationError.value=t('暂时无法确认登录状态，请重试。');if(root.value)root.value.hidden=false}}
}
function visibility(){if(document.hidden)conceal();else void verify()}
function restored(event:PageTransitionEvent){if(event.persisted)void verify()}
function change(value:'all'|'unread'){if(value===filter.value||busy.value)return;filter.value=value;void load()}
async function markRead(row:Row){
 if(busy.value||!verified.value||!row.unread)return
 const at=epoch;busy.value=row.id
 try{
  await api('/me/notifications/'+row.id+'/read',{method:'PUT'})
  if(disposed||at!==epoch||!verified.value)return
  const current=items.value.find(item=>item.id===row.id)
  if(current?.unread){current.unread=false;if(unread.value!==null)unread.value=Math.max(0,unread.value-1)}
  if(filter.value==='unread')items.value=items.value.filter(item=>item.id!==row.id)
  toast.success(t('通知已标为已读。'))
  await load()
 }catch(cause){if(!disposed&&at===epoch)toast.error((cause as Error).message)}
 finally{busy.value=''}
}
function discussionURL(row:Row){return '/posts/'+row.post_id+'?comment='+row.comment_id+'#comment-'+row.comment_id}
onMounted(()=>{void load();addEventListener('pagehide',conceal);addEventListener('pageshow',restored);document.addEventListener('visibilitychange',visibility)})
onBeforeUnmount(()=>{disposed=true;conceal();stopSessionEnd();stopObservation();removeEventListener('pagehide',conceal);removeEventListener('pageshow',restored);document.removeEventListener('visibilitychange',visibility)})
</script>

<template>
 <section ref="root" class="notifications" :aria-label="t('通知列表')">
  <div v-if="verificationError" class="notice error" role="alert">{{verificationError}} <button type="button" class="quiet" @click="verify">{{t('重试')}}</button></div>
  <template v-if="verified">
   <div class="notifications-toolbar"><div class="tabs" role="group" :aria-label="t('通知筛选')"><button v-for="option in [{id:'all',label:t('全部')},{id:'unread',label:t('未读')}]" :key="option.id" type="button" class="quiet" :aria-pressed="filter===option.id" :disabled="!!busy" @click="change(option.id as 'all'|'unread')">{{option.label}}</button></div><span class="muted" role="status">{{unreadLabel}}</span><button type="button" class="quiet" :disabled="loading||!!busy" @click="load()">{{t('刷新通知')}}</button></div>
   <p v-if="error" class="notice error" role="alert">{{error}} <button type="button" class="quiet" :disabled="loading||!!busy" @click="load()">{{t('重试')}}</button></p>
   <div v-if="loading&&!items.length" class="notifications-loading" :aria-label="t('正在读取通知…')"><div class="loading-line"></div><div class="loading-line"></div></div>
   <ol v-if="items.length" class="notification-list" :aria-busy="loading"><li v-for="row in items" :key="row.id" class="notification-row" :data-notification-id="row.id">
    <div class="notification-meta"><span v-if="row.unread" class="notification-unread">{{t('未读')}}</span><span v-else class="muted">{{t('已读')}}</span><time :datetime="row.created_at">{{date(row.created_at)}}</time></div>
    <template v-if="row.available"><p class="notification-action"><strong>{{row.actor.display_name}}</strong> {{row.kind==='reply'?t('回复了你的评论'):t('评论了你的文章')}}</p><h2>{{row.post_title}}</h2><p class="notification-excerpt">{{row.body}}</p></template>
    <template v-else><h2>{{t('这条讨论已不可用')}}</h2><p class="notification-excerpt muted">{{t('相关内容已隐藏、删除或暂不可访问。')}}</p></template>
    <div class="notification-actions"><a v-if="row.available" :href="discussionURL(row)">{{t('查看讨论')}}</a><button v-if="row.unread" type="button" class="quiet" :disabled="!!busy" @click="markRead(row)">{{busy===row.id?t('正在处理…'):t('标为已读')}}</button></div>
   </li></ol>
   <div v-else-if="!loading&&!error" class="empty"><h2>{{filter==='unread'?t('没有未读通知'):t('还没有通知')}}</h2><p>{{filter==='unread'?t('新的评论与回复会出现在这里。'):t('有人评论你的文章或回复你的评论时，会在这里提醒你。')}}</p><button v-if="filter==='unread'" type="button" class="quiet" @click="change('all')">{{t('查看全部通知')}}</button><a v-else class="button quiet" href="/discover">{{t('浏览内容')}}</a></div>
   <InfiniteList :has-more="hasMore" :loading="loading" :disabled="!!busy" :error="error" @load="load(true)"/>
  </template>
 </section>
</template>
