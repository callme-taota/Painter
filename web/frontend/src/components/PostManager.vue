<script setup lang="ts">
import {t, formatDate} from "../i18n"

import InfiniteList from './InfiniteList.vue'
import {onMounted,ref} from 'vue'
import {type Row} from '../api'
import {posts} from '../contentClient'
import {useCursorList} from '../useCursorList'
import {confirmAction} from '../confirm'
import {toast} from '../toast'
import Icon from './Icon.vue'
import RowMenu from './RowMenu.vue'
const props=defineProps<{owner:boolean}>()
const scope=ref(props.owner?'site':'mine'),status=ref('all'),query=ref(''),appliedQuery=ref(''),busy=ref(false)
const {items:rows,loading,error,cursor:next,load}=useCursorList({
 request:(cursor,signal)=>posts.list(new URLSearchParams({scope:scope.value,status:status.value,limit:'20',q:appliedQuery.value}),cursor,{signal}),
 disabled:()=>busy.value,
})
const states=[{id:'all',name:t("全部")},{id:'published',name:t("已发布")},{id:'draft',name:t("草稿")},{id:'trashed',name:t("回收站")}]
function label(row:Row){return row.status==='published'?t("已发布"):row.status==='trashed'?t("回收站"):t("草稿")}
function timestamp(value:string){if(!value)return '—';return formatDate(value,{year:'numeric',month:'2-digit',day:'2-digit',hour:'2-digit',minute:'2-digit',hour12:false})}
function reset(){void load()}
function search(){appliedQuery.value=query.value.trim();reset()}
async function action(row:Row,kind:'trash'|'recover'|'withdraw'){
 if(busy.value)return
 const prompt=kind==='trash'?[t("移入回收站？"),t("文章将从公开页面和搜索中移除，草稿和历史版本会保留。"),t("移入回收站")]:kind==='withdraw'?[t("撤回文章？"),t("公开版本会被撤下，草稿和历史版本会保留。"),t("撤回文章")]:null
 if(prompt&&!await confirmAction(prompt[0],prompt[1],prompt[2]))return
 busy.value=true;error.value=''
 try{await (kind==='recover'?posts.recover(row.id,row.version):kind==='withdraw'?posts.withdraw(row.id,{version:row.version}):posts.remove(row.id,{version:row.version}));toast.success(kind==='recover'?t("文章已恢复为草稿。"):kind==='withdraw'?t("文章已撤回。"):t("文章已移入回收站。"));await load()}catch(e){toast.error((e as Error).message)}finally{busy.value=false}
}
onMounted(()=>{void load()})
</script>
<template>
<section class="content-manager" :aria-label="t('文章列表')">
 <div class="manager-toolbar"><form class="manager-search" @submit.prevent="search"><label class="sr-only" for="manager-post-query">{{t("按标题或摘要搜索文章")}}</label><Icon name="search" :size="18"/><input id="manager-post-query" v-model="query" type="search" maxlength="64" :placeholder="t('搜索标题或摘要')" :disabled="busy"><button :disabled="loading||busy">{{t("搜索")}}</button></form><div v-if="owner" class="manager-scope" :aria-label="t('文章范围')"><button :aria-pressed="scope==='site'" :disabled="loading||busy" @click="scope='site';reset()">{{t("全站")}}</button><button :aria-pressed="scope==='mine'" :disabled="loading||busy" @click="scope='mine';reset()">{{t("我的")}}</button></div></div>
 <div class="manager-status-tabs" :aria-label="t('文章状态')"><button v-for="s in states" :key="s.id" :aria-pressed="status===s.id" :disabled="loading||busy" @click="status=s.id;reset()">{{s.name}}</button></div>
 <p v-if="error" class="notice error" role="alert">{{error}} <button class="quiet" :disabled="loading" @click="load()">{{t("重新加载")}}</button></p>
 <p class="manager-table-hint">{{t("左右滑动，查看完整信息与操作")}}</p>
 <div class="manager-table-wrap" tabindex="0" :aria-label="t('文章列表')" :aria-busy="loading"><table role="table" class="manager-table posts-table"><caption class="sr-only">{{t("文章、作者、发布状态、更新时间与操作")}}</caption><thead role="rowgroup"><tr role="row"><th role="columnheader" scope="col">{{t("文章")}}</th><th role="columnheader" v-if="owner&&scope==='site'" scope="col">{{t("作者")}}</th><th role="columnheader" scope="col">{{t("状态")}}</th><th role="columnheader" scope="col">{{t("更新于")}}</th><th role="columnheader" scope="col">{{t("操作")}}</th></tr></thead><tbody role="rowgroup"><tr role="row" v-if="loading&&!rows.length"><td role="cell" :colspan="owner&&scope==='site'?5:4"><div class="manager-loading" role="status">{{t("正在载入文章…")}}</div></td></tr><tr role="row" v-for="row in rows" :key="row.id"><td role="cell" class="manager-title"><a :href="`/studio/posts/${row.id}`">{{row.title||t("未命名文章")}}</a><small v-if="row.summary">{{row.summary}}</small></td><td role="cell" v-if="owner&&scope==='site'" class="manager-author">{{row.author_name||t("作者 {value1}", {value1:row.author_id})}}</td><td role="cell"><span class="table-status" :class="row.status">{{label(row)}}</span></td><td role="cell" class="manager-date"><time :datetime="row.updated_at">{{timestamp(row.updated_at)}}</time></td><td role="cell"><div class="manager-row-actions"><a v-if="row.status!=='trashed'" class="button quiet" :href="`/studio/posts/${row.id}`">{{t("编辑")}}</a><button v-else class="quiet" :disabled="busy||loading" @click="action(row,'recover')">{{t("恢复草稿")}}</button><RowMenu v-if="row.status!=='trashed'" :label="t('更多操作：{value1}', {value1:row.title})"><a v-if="row.status==='published'" :href="`/posts/${row.id}`" target="_blank" rel="noopener">{{t("查看公开文章")}}</a><button v-if="row.status==='published'" :disabled="busy||loading" @click="action(row,'withdraw')">{{t("撤回文章")}}</button><button class="danger" :disabled="busy||loading" @click="action(row,'trash')">{{t("移入回收站")}}</button></RowMenu></div></td></tr><tr role="row" v-if="!loading&&!rows.length"><td role="cell" :colspan="owner&&scope==='site'?5:4"><div class="manager-empty"><strong>{{appliedQuery?t("没有匹配的文章"):status==='trashed'?t("回收站是空的"):t("这里还没有文章")}}</strong><p>{{appliedQuery?t("试试其他标题或摘要关键词。"):status==='trashed'?t("移入回收站的文章可以在这里恢复。"):t("新草稿和公开文章会显示在这里。")}}</p><button v-if="appliedQuery" @click="query='';appliedQuery='';reset()">{{t("清除搜索")}}</button><a v-else-if="status!=='trashed'" class="button" href="/studio/posts/new">{{t("写第一篇文章")}}</a></div></td></tr></tbody></table></div>
 <InfiniteList :has-more="!!next" :loading="loading" :disabled="busy" :error="error" @load="load(true)"/><p class="manager-caption">{{t("已显示 {value1} 篇", {value1:(rows.length)})}}</p>
</section>
</template>
