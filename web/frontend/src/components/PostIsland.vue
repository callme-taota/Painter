<script setup lang="ts">
import {t} from '../i18n'
import {computed,nextTick,onMounted,onBeforeUnmount,reactive,ref,watch} from 'vue'
import InfiniteList from './InfiniteList.vue'
import DiscussionComment from './DiscussionComment.vue'
import CommentReplyComposer from './CommentReplyComposer.vue'
import '../comments-v16.css'
import {confirmAction} from '../confirm'
import {appendUnique,api,APIError,body,loadSession,onSessionEnd,session,list,type Row} from '../api'
import {clearLikeSessions,loadLike,setBookmarked,toggleLike,type LikeState} from '../postLike'
import {randomID} from '../random'
import {toast} from '../toast'

const props=defineProps<{postId:string;revisionId?:string;receipt?:string}>()
const features=ref<Row>({comments_enabled:true,collections_enabled:true})
const comments=ref<Row[]>([]),metrics=ref<Row>({}),liked=ref(false),bookmarked=ref(false),text=ref(''),error=ref(''),loading=ref(true),commentsLoading=ref(false),busy=ref(false),next=ref<string|null>(null),more=ref(false)
type Thread={open:boolean;items:Row[];loaded:boolean;loading:boolean;next:string;more:boolean;error:string;generation:number}
const threads=reactive<Record<string,Thread>>({}),replyDrafts=reactive<Record<string,string>>({})
const replyTarget=ref<{id:string;rootID:string;name:string}|null>(null)
const replyText=computed({get:()=>replyTarget.value?replyDrafts[replyTarget.value.id]||'':'',set:value=>{if(replyTarget.value)replyDrafts[replyTarget.value.id]=value}})
const canInteract=computed(()=>!session.loading&&session.authenticated)
const actionDisabled=computed(()=>busy.value||loading.value||commentsLoading.value)
const threadReads=new Map<string,AbortController>(),replyMutations=new Map<string,{text:string;key:string}>()
const focused=ref(/^[1-9][0-9]*$/.test(new URL(location.href).searchParams.get('comment')||''))
let disposed=false,rootsGeneration=0,rootRead:AbortController|undefined,rootMutation:{text:string;key:string}|undefined,lastActor=''
function thread(id:string):Thread{return threads[id]||(threads[id]={open:false,items:[],loaded:false,loading:false,next:'',more:false,error:'',generation:0})}
function cancelThreadRead(id:string){const read=threadReads.get(id);if(!read)return;thread(id).generation++;read.abort();threadReads.delete(id);thread(id).loading=false}
function receiveLike(e:Event){const state=(e as CustomEvent<LikeState>).detail;if(state.postId===props.postId){liked.value=state.liked;bookmarked.value=state.bookmarked;metrics.value.like_count=state.count;if(state.session?.authenticated===false){session.authenticated=false;session.user=null;session.csrf_token=''}if(state.error)error.value=state.error}}
function report(e:unknown,interactive=false){if((e as Error).name==='AbortError'||disposed)return;if(e instanceof APIError&&e.status===401){session.authenticated=false;session.user=null;session.csrf_token='';clearLikeSessions();if(interactive)toast.error(t('登录已失效，请重新登录。'),{id:'session-expired'});return}if(interactive)toast.error((e as Error).message,{id:'post-operation:'+props.postId});else error.value=(e as Error).message}
function clearDrafts(){text.value='';replyTarget.value=null;for(const id of Object.keys(replyDrafts))delete replyDrafts[id];replyMutations.clear();rootMutation=undefined}
const stopSessionEnd=onSessionEnd(reason=>{if(reason==='signout')clearDrafts()})
watch(()=>session.user?.id,id=>{if(id){if(lastActor&&lastActor!==String(id))clearDrafts();lastActor=String(id)}})

async function loadComments(append=false){
 const generation=++rootsGeneration;rootRead?.abort();const controller=new AbortController();rootRead=controller
 let data:Row
 try{data=await api(`/posts/${props.postId}/comments${append&&next.value?'?cursor='+encodeURIComponent(next.value):''}`,{signal:controller.signal})}
 catch(e){if(e instanceof APIError&&e.status===404&&generation===rootsGeneration&&!disposed){comments.value=[];next.value=null;more.value=false;for(const read of threadReads.values())read.abort();threadReads.clear();for(const id of Object.keys(threads))delete threads[id]}throw e}
 if(disposed||generation!==rootsGeneration)return
 comments.value=append?appendUnique(comments.value,list(data)):list(data);next.value=data.next_cursor;more.value=data.has_more
 if(!append){const known=new Set(comments.value.map(c=>String(c.id)));for(const id of Object.keys(threads))if(!known.has(id)){threadReads.get(id)?.abort();threadReads.delete(id);delete threads[id]}}
}
async function moreComments(){if(actionDisabled.value)return;commentsLoading.value=true;error.value='';try{await loadComments(true)}catch(e){report(e)}finally{commentsLoading.value=false}}
async function loadReplies(root:Row,append=false){
 const state=thread(String(root.id));if(state.loading||append&&(!state.loaded||!state.more))return false
 state.loading=true;state.error='';const generation=++state.generation,controller=new AbortController();threadReads.set(String(root.id),controller)
 try{
  const data=await api(`/comments/${root.id}/replies${append&&state.next?'?cursor='+encodeURIComponent(state.next):''}`,{signal:controller.signal})
  if(disposed||threads[root.id]!==state||generation!==state.generation)return false
  state.items=append?appendUnique(state.items,list(data)):list(data);if(state.items[0]?.thread_reply_count!==undefined)root.reply_count=Number(state.items[0].thread_reply_count);state.next=data.next_cursor||'';state.more=!!data.has_more;state.loaded=true;return true
 }catch(e){if(!disposed&&(e as Error).name!=='AbortError'){if(e instanceof APIError&&e.status===404){state.items=[];state.loaded=false;state.more=false;state.next=''}state.error=e instanceof APIError&&e.status===404?t('这段讨论已不可用，请刷新讨论。'):t('回复未能载入，请重试。')}return false}
 finally{if(generation===state.generation)state.loading=false;if(threadReads.get(String(root.id))===controller)threadReads.delete(String(root.id))}
}
function toggleReplies(root:Row){const state=thread(String(root.id));state.open=!state.open;if(state.open&&!state.loaded)void loadReplies(root)}
function startReply(c:Row,root:Row){if(!canInteract.value||!features.value.comments_enabled||actionDisabled.value)return;replyTarget.value={id:String(c.id),rootID:String(root.id),name:String(c.user?.display_name||'')};if(c.parent_id)thread(String(root.id)).open=true}
function cancelReply(){const id=replyTarget.value?.id;replyTarget.value=null;if(id)requestAnimationFrame(()=>document.querySelector<HTMLButtonElement>(`#comment-${CSS.escape(id)} [data-comment-reply]`)?.focus({preventScroll:true}))}
function mutationFor(target:string,value:string){const pending=replyMutations.get(target);if(pending?.text===value)return pending.key;const key=randomID();replyMutations.set(target,{text:value,key});return key}
async function submitReply(){
 const target=replyTarget.value,value=replyText.value;if(!target||!canInteract.value||!features.value.comments_enabled||actionDisabled.value||!value.trim()||Array.from(value).length>5000)return
 const root=comments.value.find(c=>String(c.id)===target.rootID);if(!root)return
 // A read started before this write may omit the new reply. Invalidate it and
 // reload after the write, or append to a fully loaded thread we already have.
 cancelThreadRead(target.rootID)
 busy.value=true
 try{
  const created=await api(`/posts/${props.postId}/comments`,{method:'POST',key:mutationFor(target.id,value),body:body({body:value,reply_to_id:target.id})})
  if(disposed)return
  replyMutations.delete(target.id);replyDrafts[target.id]='';replyTarget.value=null
  root.reply_count=Number(created.thread_reply_count??Number(root.reply_count||0)+1);const state=thread(target.rootID);state.open=true
  if(state.loaded&&!state.more){state.items=appendUnique(state.items,[created]);toast.success(t('回复已发表'),{id:'post-operation:'+props.postId})}
  else if(state.loaded){toast.success(t('回复已发表，后续回复会随滚动载入。'),{id:'post-operation:'+props.postId})}
  else if(await loadReplies(root)){toast.success(t('回复已发表'),{id:'post-operation:'+props.postId})}
  else toast.warning(t('回复已发表，但回复列表未能更新，请重试载入。'),{id:'post-operation:'+props.postId})
 }catch(e){if(e instanceof APIError&&e.status===404)toast.error(t('这条评论已不可用，回复内容已保留。'),{id:'post-operation:'+props.postId});else report(e,true)}finally{busy.value=false}
}
async function relation(kind:'like'|'bookmark'){if(!canInteract.value)return;if(kind==='like'){await toggleLike(props.postId);return}if(busy.value)return;busy.value=true;error.value='';const before=bookmarked.value;bookmarked.value=!before;try{await api(`/posts/${props.postId}/${kind}`,{method:before?'DELETE':'PUT'});setBookmarked(props.postId,!before);toast.success(before?t('已取消收藏'):t('文章已收藏'),{id:'post-bookmark:'+props.postId})}catch(e){bookmarked.value=before;report(e,true)}finally{busy.value=false}}
async function refreshDiscussion(message:string){try{if(focused.value)exitFocused();await loadComments();toast.success(message,{id:'post-operation:'+props.postId})}catch{error.value=t('{value1}，但讨论列表未能更新，请重新加载。',{value1:message});toast.warning(error.value,{id:'post-operation:'+props.postId})}}
async function submit(){if(!canInteract.value||!features.value.comments_enabled||actionDisabled.value||!text.value.trim())return;busy.value=true;error.value='';const value=text.value;if(rootMutation?.text!==value)rootMutation={text:value,key:randomID()};try{await api(`/posts/${props.postId}/comments`,{method:'POST',key:rootMutation.key,body:body({body:value})});text.value='';rootMutation=undefined;await refreshDiscussion(t('评论已发表'))}catch(e){report(e,true)}finally{busy.value=false}}
async function remove(c:Row,root:Row){
 if(!canInteract.value||actionDisabled.value)return
 const detail=!c.parent_id&&Number(c.reply_count)>0?t('删除后，这条评论和其下回复将不再公开展示。'):t('删除后无法恢复。')
 if(!await confirmAction(t('删除这条评论？'),detail,t('删除评论'))||actionDisabled.value)return
 cancelThreadRead(String(root.id))
 busy.value=true;error.value=''
 try{
  await api(`/comments/${c.id}`,{method:'DELETE'})
  if(c.parent_id){const state=thread(String(root.id));state.items=state.items.filter(item=>String(item.id)!==String(c.id));for(const item of state.items)if(String(item.reply_to?.id)===String(c.id)){item.reply_to=null;item.reply_to_deleted=true}root.reply_count=Math.max(0,Number(root.reply_count||0)-1);if(replyTarget.value?.id===String(c.id))replyTarget.value=null;toast.success(t('评论已删除'),{id:'post-operation:'+props.postId})}
  else{comments.value=comments.value.filter(item=>String(item.id)!==String(root.id));delete threads[String(root.id)];if(replyTarget.value?.rootID===String(root.id))replyTarget.value=null;await refreshDiscussion(t('评论已删除'))}
 }catch(e){report(e,true)}finally{busy.value=false}
}
async function commentLike(c:Row){if(!canInteract.value||actionDisabled.value)return;busy.value=true;const before=c.liked;try{await api(`/comments/${c.id}/like`,{method:before?'DELETE':'PUT'});c.liked=!before;c.like_count=Number(c.like_count||0)+(before?-1:1)}catch(e){report(e,true)}finally{busy.value=false}}
async function loadFocused(){
 const id=new URL(location.href).searchParams.get('comment'),generation=++rootsGeneration
 rootRead?.abort();const controller=new AbortController();rootRead=controller
 const data=await api(`/posts/${props.postId}/comments/${id}/context`,{signal:controller.signal,cache:'no-store'})
 if(disposed||generation!==rootsGeneration)return
 if(!data.root||!data.target||String(data.target.id)!==id||data.root.parent_id||data.target.parent_id&&String(data.target.parent_id)!==String(data.root.id))throw new Error(t('讨论未能完整载入，请重试。'))
 for(const read of threadReads.values())read.abort();threadReads.clear();for(const key of Object.keys(threads))delete threads[key]
 comments.value=[data.root];next.value=null;more.value=false
 if(data.target.parent_id){const state=thread(String(data.root.id));state.open=true;state.loaded=true;state.items=[data.target]}
 await nextTick();if(!disposed)document.getElementById('comment-'+id)?.scrollIntoView({block:'center'})
}
function exitFocused(){
 focused.value=false
 const url=new URL(location.href);url.searchParams.delete('comment');url.hash='comments';history.replaceState(history.state,'',url.pathname+url.search+url.hash)
 for(const read of threadReads.values())read.abort();threadReads.clear();for(const key of Object.keys(threads))delete threads[key]
 replyTarget.value=null
}
async function allComments(){exitFocused();comments.value=[];await initialize()}
async function initialize(){loading.value=true;error.value='';try{await Promise.all([loadSession(),api('/site').then(data=>{features.value=data}),focused.value?loadFocused():loadComments()]);if(canInteract.value){const state=await loadLike(props.postId);liked.value=state.liked;bookmarked.value=state.bookmarked;metrics.value.like_count=state.count}}catch(e){report(e)}finally{loading.value=false}}
onMounted(()=>{window.addEventListener('painter:like',receiveLike);void initialize()})
onBeforeUnmount(()=>{disposed=true;rootRead?.abort();for(const controller of threadReads.values())controller.abort();stopSessionEnd();window.removeEventListener('painter:like',receiveLike)})
</script>
<template>
 <div class="article-community">
  <div v-if="canInteract" class="interaction-bar"><button :aria-pressed="liked" :disabled="busy||loading" @click="relation('like')"><svg class="icon" viewBox="0 0 24 24" aria-hidden="true"><path d="M7 21H4V10h3m0 11h10a2 2 0 0 0 2-1.7l1.5-9A2 2 0 0 0 18.5 8H14l1-5-2-1-6 8v11Z"/></svg>{{liked?t('已点赞'):t('点赞')}} {{metrics.like_count??metrics.likes??''}}</button><button v-if="features.collections_enabled||bookmarked" :aria-pressed="bookmarked" :disabled="busy||loading" @click="relation('bookmark')"><svg class="icon" viewBox="0 0 24 24" aria-hidden="true"><path d="M6 3h12v18l-6-4-6 4V3Z"/></svg>{{bookmarked?t('已收藏'):t('收藏')}}</button></div>
  <section class="comment-area discussion-surface" aria-labelledby="comments-title">
   <header class="discussion-heading"><h2 id="comments-title">{{t('一起聊聊')}}</h2><span v-if="!loading&&!error" class="discussion-count">{{comments.length?t('已载入 {value1} 条讨论',{value1:comments.length}):t('等一个新想法')}}</span></header>
   <div v-if="error" class="notice error" role="alert">{{error}} <button type="button" class="quiet" :disabled="actionDisabled" @click="initialize">{{t('重新加载讨论')}}</button></div>
   <p v-if="!features.comments_enabled&&canInteract" class="notice">{{t('站点目前关闭了新评论，已有讨论仍然保留。')}}</p>
   <form v-if="canInteract&&features.comments_enabled" class="discussion-composer" @submit.prevent="submit"><label for="comment-body">{{t('分享你的想法')}}<textarea id="comment-body" v-model="text" maxlength="5000" rows="3" required :placeholder="t('留下你的思考，让讨论继续。')" :disabled="busy"></textarea></label><div><small>{{t('友善交流，言之有物。')}}</small><button class="primary" :disabled="actionDisabled||!text.trim()">{{busy?t('提交中…'):t('发表评论')}}</button></div></form>
   <div v-if="loading" class="discussion-loading" role="status">{{t('正在载入讨论…')}}</div>
   <p v-if="focused" class="discussion-focus-note">{{t("当前显示通知对应的讨论。")}} <button type="button" class="quiet" :disabled="actionDisabled" @click="allComments">{{t("查看完整讨论")}}</button></p>
   <div class="discussion-list">
    <div v-for="c in comments" :key="c.id" class="discussion-thread" :data-thread-id="c.id">
     <DiscussionComment :comment="c" :authenticated="canInteract" :reply-enabled="features.comments_enabled" :disabled="actionDisabled" :owner="session.user?.role==='owner'" @reply="startReply($event,c)" @remove="remove($event,c)" @like="commentLike"/>
     <CommentReplyComposer v-if="canInteract&&features.comments_enabled&&replyTarget?.id===String(c.id)" :key="replyTarget.id" v-model="replyText" :target-id="replyTarget.id" :name="replyTarget.name" :busy="busy" :disabled="actionDisabled" @submit="submitReply" @cancel="cancelReply"/>
     <button v-if="Number(c.reply_count)>0" type="button" class="quiet discussion-replies-toggle" :aria-expanded="!!threads[c.id]?.open" :aria-controls="'replies-'+c.id" @click="toggleReplies(c)"><svg class="icon" viewBox="0 0 24 24" aria-hidden="true"><path d="m8 10 4 4 4-4"/></svg>{{focused&&threads[c.id]?.loaded?(threads[c.id]?.open?t('收起通知回复'):t('展开通知回复')):threads[c.id]?.open?t('收起回复'):t('查看 {count} 条回复',{count:c.reply_count})}}</button>
     <div v-if="threads[c.id]?.open" :id="'replies-'+c.id" class="discussion-replies" :aria-label="t('评论回复')" :aria-busy="threads[c.id].loading">
      <template v-for="reply in threads[c.id].items" :key="reply.id">
       <DiscussionComment :comment="reply" reply :authenticated="canInteract" :reply-enabled="features.comments_enabled" :disabled="actionDisabled" :owner="session.user?.role==='owner'" @reply="startReply($event,c)" @remove="remove($event,c)" @like="commentLike"/>
       <CommentReplyComposer v-if="canInteract&&features.comments_enabled&&replyTarget?.id===String(reply.id)" :key="replyTarget.id" v-model="replyText" :target-id="replyTarget.id" :name="replyTarget.name" :busy="busy" :disabled="actionDisabled" @submit="submitReply" @cancel="cancelReply"/>
      </template>
      <p v-if="threads[c.id].loading&&!threads[c.id].loaded" class="discussion-loading" role="status">{{t('正在载入回复…')}}</p>
      <div v-if="threads[c.id].error&&!threads[c.id].loaded" class="discussion-replies-error" role="alert">{{threads[c.id].error}} <button type="button" class="quiet" :disabled="threads[c.id].loading" @click="loadReplies(c)">{{t('重试载入')}}</button></div>
      <p v-if="threads[c.id].loaded&&!threads[c.id].loading&&!threads[c.id].items.length" class="discussion-replies-empty">{{t('暂无可展示的回复')}}</p>
      <InfiniteList v-if="threads[c.id].loaded" :has-more="threads[c.id].more" :loading="threads[c.id].loading" :disabled="busy||!threads[c.id].open" :error="threads[c.id].error" @load="loadReplies(c,true)"/>
     </div>
    </div>
   </div>
   <p v-if="!loading&&!error&&!comments.length" class="discussion-empty">{{t('还没有评论。')}}</p>
   <InfiniteList :has-more="more" :loading="loading||commentsLoading" :disabled="busy" :error="error" @load="moreComments"/>
  </section>
 </div>
</template>
