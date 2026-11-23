<script setup lang="ts">
import {uploadMedia} from '../mediaUpload'
import {t} from "../i18n"

import InfiniteList from './InfiniteList.vue'
import {ref,computed,onMounted,onBeforeUnmount} from 'vue'
import {api,date,type Row} from '../api'
import {useCursorList} from '../useCursorList'
import {withCursor} from '../pagination'
import {posts} from '../contentClient'
import {useMutationAttempt} from '../useMutationAttempt'
import PendingSaveNotice from './PendingSaveNotice.vue'
import SelectField from './SelectField.vue'
import Icon from './Icon.vue'
import {confirmAction} from '../confirm'
import {toast} from '../toast'
import '../series.css'
const selected=ref<Row|null>(null),chapters=ref<Row[]>([]),busy=ref(false),error=ref(''),query=ref(''),baseline=ref('')
const postQuery=ref(''),picker=ref(false)
const saveAttempt=useMutationAttempt(),{pending:pendingSave}=saveAttempt
let detailVersion=0
const payload=()=>selected.value?{title:selected.value.title,slug:selected.value.slug||'',description:selected.value.description||'',cover_media_id:selected.value.cover_media_id||null,status:selected.value.status,state:selected.value.state,chapter_post_ids:chapters.value.map(c=>String(c.post_id||c.id))}:null
const dirty=computed(()=>!!selected.value&&JSON.stringify(payload())!==baseline.value)
const selectedIDs=computed(()=>new Set(chapters.value.map(c=>String(c.post_id||c.id))))
const deletedChapters=computed(()=>chapters.value.filter(chapter=>chapter.status==='deleted').length)
const stateOptions=[{value:'ongoing',label:t("连载中")},{value:'completed',label:t("已完结")}],visibilityOptions=[{value:'draft',label:t("草稿 · 仅自己可见")},{value:'published',label:t("公开 · 进入专栏列表")}]
function report(e:unknown){error.value=e instanceof Error?e.message:t("操作失败，请稍后重试")}
function mutationError(e:unknown){if(['VERSION_CONFLICT','VALIDATION_FAILED'].includes((e as {code?:string})?.code||''))report(e);else toast.error(e instanceof Error?e.message:t("操作失败，请稍后重试"))}
const {items,loading,hasMore:more,load}=useCursorList({
 request:(cursor,signal)=>api(withCursor('/me/series?'+new URLSearchParams({limit:'20',q:query.value}),cursor),{signal}),
 disabled:()=>busy.value,
 error,
})
const {items:candidates,loading:postLoading,hasMore:postMore,load:findPosts}=useCursorList({
 request:async(cursor,signal)=>{
  const data=await posts.list(new URLSearchParams({limit:'20',q:postQuery.value}),cursor,{signal})
  return {...data,items:(data.items||[]).filter((post:Row)=>!['deleted','trashed'].includes(post.status))}
 },
 error,
})
async function mayLeave(){return (!dirty.value&&!pendingSave.value)||await confirmAction(t("放弃未保存的专栏修改？"),pendingSave.value?t("有一次保存尚未确认。切换前建议重试确认保存；继续切换会保留服务器已收到的内容。"):t("章节顺序和资料尚未保存。"),t("放弃修改"))}
function assign(data:Row){saveAttempt.clear();selected.value={...data};chapters.value=(data.chapters||[]).map((c:Row)=>({...c}));baseline.value=JSON.stringify(payload());candidates.value=[];picker.value=false}
async function open(row:Row){if(busy.value||!await mayLeave())return;const generation=++detailVersion;busy.value=true;error.value='';try{const data=await api('/studio/series/'+row.id);if(generation===detailVersion)assign(data)}catch(e){if(generation===detailVersion)report(e)}finally{if(generation===detailVersion)busy.value=false}}
async function back(){if(busy.value||!await mayLeave())return;saveAttempt.clear();selected.value=null;chapters.value=[];error.value=''}
async function create(){if(busy.value||!await mayLeave())return;detailVersion++;assign({title:'',slug:'',description:'',cover_media_id:null,status:'draft',state:'ongoing',chapters:[]});error.value=''}
async function save(){
 if(!selected.value||busy.value)return
 busy.value=true;error.value=''
 try{
  const attempt=saveAttempt.begin(()=>({path:selected.value!.id?'/studio/series/'+selected.value!.id:'/series',method:selected.value!.id?'PUT':'POST',version:selected.value!.id?selected.value!.version:undefined,payload:payload()}))
  const data=await api(attempt.path,{method:attempt.method,version:attempt.version,key:attempt.key,body:attempt.body})
  if(!data||typeof data.id!=='string'||!Number.isSafeInteger(data.version)||data.version<1||['title','slug','description','status','state'].some(key=>typeof data[key]!=='string')||!Array.isArray(data.chapters))throw new Error(t("保存结果不完整，请刷新列表确认；当前输入已保留。"))
  const local={...selected.value},localChapters=chapters.value.map(chapter=>({...chapter})),preserve=JSON.stringify(payload())!==attempt.body
  assign(data)
  if(preserve){Object.assign(selected.value!,local,{id:data.id,version:data.version});chapters.value=localChapters;toast.warning(t("上次保存已确认，当前新修改仍未保存。"))}
  else toast.success(t("专栏和章节顺序已保存"))
  await load()
 }catch(e){saveAttempt.failed(e);mutationError(e)}finally{busy.value=false}
}
async function remove(){if(!selected.value?.id||busy.value||pendingSave.value)return;if(!await confirmAction(t("删除这个专栏？"),t("只删除专栏及章节引用，文章正文仍然保留。"),t("删除专栏")))return;busy.value=true;error.value='';try{await api('/studio/series/'+selected.value.id,{method:'DELETE',version:selected.value.version});selected.value=null;chapters.value=[];toast.success(t("专栏已删除，文章仍然保留"));await load()}catch(e){mutationError(e)}finally{busy.value=false}}
function showPicker(){picker.value=!picker.value;if(picker.value&&!candidates.value.length)void findPosts()}
function add(post:Row){const id=String(post.id);if(selectedIDs.value.has(id)||chapters.value.length>=300)return;chapters.value.push({id,post_id:id,title:post.title,status:post.status})}
function move(index:number,delta:number){const target=index+delta;if(target<0||target>=chapters.value.length)return;const copy=[...chapters.value];[copy[index],copy[target]]=[copy[target]!,copy[index]!];chapters.value=copy}
async function upload(event:Event){const input=event.target as HTMLInputElement,file=input.files?.[0];if(!file||!selected.value||busy.value)return;busy.value=true;error.value='';try{const value=await uploadMedia(file);selected.value.cover_media_id=value.media_id;selected.value.cover_url='/media/'+value.media_id;toast.success(t("封面已上传，保存专栏后生效"))}catch(e){mutationError(e)}finally{busy.value=false;input.value=''}}
function beforeUnload(event:BeforeUnloadEvent){if(dirty.value||pendingSave.value){event.preventDefault();event.returnValue=''}}
onMounted(()=>{void load();addEventListener('beforeunload',beforeUnload)})
onBeforeUnmount(()=>{detailVersion++;removeEventListener('beforeunload',beforeUnload)})
</script>
<template>
 <section class="series-manager" :class="{'has-selection':!!selected}" :aria-label="t('专栏管理')">
  <PendingSaveNotice v-if="pendingSave&&!busy" :busy="busy" @retry="save"/>
  <p v-if="error" class="notice error" role="alert">{{error}}</p>
  <div class="series-manager-grid">
   <section class="series-index" aria-labelledby="my-series-title"><div class="series-panel-head"><h2 id="my-series-title">{{t("我的专栏")}}</h2><button type="button" class="primary" :disabled="busy" @click="create">{{t("新建专栏")}}</button></div>
    <form class="series-search" @submit.prevent="load()"><label class="sr-only" for="series-query">{{t("查找专栏")}}</label><input id="series-query" v-model="query" type="search" maxlength="64" :placeholder="t('查找专栏')"><button :disabled="loading">{{t("搜索")}}</button></form>
    <p v-if="loading&&!items.length" class="muted" role="status">{{t("正在载入专栏…")}}</p>
    <div v-else-if="!items.length" class="series-empty"><h3>{{query?t("没有找到专栏"):t("把文章写成一段连续的旅程")}}</h3><p>{{query?t("换个关键词试试。"):t("创建专栏，再把已有文章按章节编排。文章仍可独立阅读。")}}</p></div>
    <div v-else class="series-table-wrap"><table class="series-table"><thead><tr><th>{{t("专栏")}}</th><th>{{t("章节")}}</th><th>{{t("状态")}}</th></tr></thead><tbody><tr v-for="row in items" :key="row.id" :data-selected="selected?.id===row.id"><td><button type="button" class="series-row-title" :disabled="busy" @click="open(row)">{{row.title}}</button><small>{{date(row.updated_at)}}</small></td><td>{{row.total_chapter_count}}</td><td><span>{{row.status==='published'?t("公开"):t("草稿")}}</span><small>{{row.state==='completed'?t("已完结"):t("连载中")}}</small></td></tr></tbody></table></div>
    <InfiniteList :has-more="more" :loading="loading" :disabled="busy" :error="error" @load="load(true)"/>
   </section>
   <form v-if="selected" class="series-editor" @submit.prevent="save"><button type="button" class="quiet series-mobile-back" :disabled="busy" @click="back"><Icon name="arrow" :size="16"/>{{t("返回专栏列表")}}</button><div class="series-panel-head"><div><h2>{{selected.id?t("编辑专栏"):t("新建专栏")}}</h2><span class="muted">{{dirty?t("有未保存的修改"):t("已保存")}}</span></div><button class="primary" :disabled="busy||!selected.title?.trim()||deletedChapters>0">{{busy?t("处理中…"):t("保存专栏")}}</button></div>
    <fieldset :disabled="busy" class="series-fields"><label>{{t("专栏名称")}}<input v-model="selected.title" required maxlength="120" :placeholder="t('例如：从零搭建个人博客')"></label><label>{{t("简介")}}<textarea v-model="selected.description" maxlength="1200" rows="3" :placeholder="t('这组连载讲什么，适合谁阅读')"></textarea></label>
    <div class="series-choices"><SelectField v-model="selected.state" :label="t('连载状态')" :options="stateOptions" :disabled="busy"/><SelectField v-model="selected.status" :label="t('可见范围')" :options="visibilityOptions" :disabled="busy"/></div>
    <details class="series-advanced"><summary>{{t("专栏封面")}}</summary><div class="series-cover-tools"><img v-if="selected.cover_media_id" :src="'/media/'+selected.cover_media_id" width="120" height="68" :alt="t('专栏封面预览')"><label class="button">{{t("上传封面")}}<input class="sr-only" type="file" accept="image/png,image/jpeg,image/webp,image/gif" @change="upload"></label><button v-if="selected.cover_media_id" type="button" class="quiet" @click="selected.cover_media_id=null">{{t("移除封面")}}</button></div></details>
    <div class="series-panel-head series-chapter-head"><div><h3>{{t("章节目录")}}</h3><span class="muted">{{t("{value1} / 300 章 · 未公开文章只在这里可见", {value1:(chapters.length)})}}</span></div><button type="button" :aria-expanded="picker" aria-controls="series-post-picker" @click="showPicker">{{picker?t("收起文章"):t("添加文章")}}</button></div>
    <div v-if="picker" id="series-post-picker" class="series-post-picker"><div class="series-search"><label class="sr-only" for="series-post-query">{{t("搜索我的文章")}}</label><input id="series-post-query" v-model="postQuery" type="search" maxlength="64" :placeholder="t('搜索我的文章')" @keydown.enter.prevent="findPosts()"><button type="button" :disabled="postLoading" @click="findPosts()">{{t("搜索")}}</button></div><p v-if="postLoading" role="status">{{t("正在查找文章…")}}</p><ul v-show="!postLoading||candidates.length" class="series-post-options"><li v-for="post in candidates" :key="post.id"><span>{{post.title||t("未命名文章")}}</span><button type="button" :disabled="selectedIDs.has(String(post.id))||chapters.length>=300" @click="add(post)">{{selectedIDs.has(String(post.id))?t("已加入"):t("加入")}}</button></li><li v-if="!candidates.length" class="muted">{{t("没有可添加的文章，先创建文章或换个关键词。")}}</li><li class="series-picker-continuation"><InfiniteList :has-more="postMore" :loading="postLoading" :error="error" @load="findPosts(true)"/></li></ul></div>
    <p v-if="deletedChapters" class="notice error" role="alert">{{t("有 {value1} 篇文章已删除，请先从目录移除再保存。", {value1:(deletedChapters)})}}</p>
    <ol v-if="chapters.length" class="series-chapter-editor"><li v-for="(chapter,index) in chapters" :key="chapter.post_id||chapter.id"><span class="series-chapter-number">{{index+1}}</span><div class="series-chapter-name"><strong>{{chapter.title||t("未命名文章")}}</strong><small>{{chapter.status==='deleted'?t("文章已删除 · 请移除"):chapter.status==='published'?t("已公开"):t("未公开 · 读者暂不可见")}}</small></div><div class="series-chapter-actions"><button type="button" class="quiet" :disabled="index===0" :aria-label="t('上移第{value1}章', {value1:(index+1)})" @click="move(index,-1)"><Icon name="chevron" :size="16" class="series-move-up"/></button><button type="button" class="quiet" :disabled="index===chapters.length-1" :aria-label="t('下移第{value1}章', {value1:(index+1)})" @click="move(index,1)"><Icon name="chevron" :size="16"/></button><button type="button" class="quiet" :aria-label="t('移除第{value1}章', {value1:(index+1)})" @click="chapters.splice(index,1)">{{t("移除")}}</button></div></li></ol><p v-else class="series-empty">{{t("还没有章节，添加文章后按阅读顺序编排。")}}</p>
    </fieldset><div class="series-editor-footer"><a v-if="selected.id&&selected.status==='published'" :href="'/series/'+selected.id" target="_blank" rel="noopener">{{t("查看公开专栏")}}</a><button v-if="selected.id" type="button" class="quiet danger" :disabled="busy||!!pendingSave" @click="remove">{{t("删除专栏")}}</button></div>
   </form><section v-else class="series-editor-placeholder"><Icon name="posts" :size="28"/><h2>{{t("选择一个专栏开始编排")}}</h2><p>{{t("将已有文章排列成章节，让读者顺着目录继续阅读。")}}</p></section>
  </div>
 </section>
</template>
