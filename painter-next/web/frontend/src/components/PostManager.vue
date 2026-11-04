<script setup lang="ts">
import {onMounted,ref,onBeforeUnmount} from 'vue'
import {api,list,type Row} from '../api'
import {confirmAction} from '../confirm'
import Icon from './Icon.vue'
import RowMenu from './RowMenu.vue'
const props=defineProps<{owner:boolean}>()
const scope=ref(props.owner?'site':'mine'),status=ref('all'),query=ref(''),appliedQuery=ref(''),rows=ref<Row[]>([]),loading=ref(true),busy=ref(false),error=ref(''),notice=ref(''),next=ref(''),page=ref(0)
const cursors=ref(['']);let seq=0,controller:AbortController|undefined
const states=[{id:'all',name:'全部'},{id:'published',name:'已发布'},{id:'draft',name:'草稿'},{id:'trashed',name:'回收站'}]
function label(row:Row){return row.status==='published'?'已发布':row.status==='trashed'?'回收站':'草稿'}
function timestamp(value:string){if(!value)return '—';return new Date(value).toLocaleString('zh-CN',{year:'numeric',month:'2-digit',day:'2-digit',hour:'2-digit',minute:'2-digit',hour12:false})}
async function load(){const at=++seq;controller?.abort();controller=new AbortController();loading.value=true;error.value='';try{const q=new URLSearchParams({scope:scope.value,status:status.value,limit:'20',q:appliedQuery.value});if(cursors.value[page.value])q.set('cursor',cursors.value[page.value]);const data=await api('/me/posts?'+q,{signal:controller.signal});if(at!==seq)return;rows.value=list(data);next.value=data.next_cursor||''}catch(e){if(at===seq&&(e as Error).name!=='AbortError')error.value=(e as Error).message}finally{if(at===seq)loading.value=false}}
function reset(){page.value=0;cursors.value=[''];void load()}
function search(){appliedQuery.value=query.value.trim();reset()}
function paginate(step:number){if(step>0){if(!next.value)return;cursors.value[page.value+1]=next.value}page.value+=step;void load()}
async function action(row:Row,kind:'trash'|'recover'|'withdraw'){
 if(busy.value)return
 const prompt=kind==='trash'?['移入回收站？','文章将从公开页面和搜索中移除，草稿和历史版本会保留。','移入回收站']:kind==='withdraw'?['撤回文章？','公开版本会被撤下，草稿和历史版本会保留。','撤回文章']:null
 if(prompt&&!await confirmAction(prompt[0],prompt[1],prompt[2]))return
 busy.value=true;error.value='';notice.value=''
 try{await api(`/studio/posts/${row.id}`+(kind==='recover'?'/recover':kind==='withdraw'?'/publication':''),{method:kind==='recover'?'POST':'DELETE',version:row.version});notice.value=kind==='recover'?'文章已恢复为草稿。':kind==='withdraw'?'文章已撤回。':'文章已移入回收站。';await load()}catch(e){error.value=(e as Error).message}finally{busy.value=false}
}
onMounted(load);onBeforeUnmount(()=>{seq++;controller?.abort()})
</script>
<template>
<section class="content-manager" aria-label="文章列表">
 <div class="manager-toolbar"><form class="manager-search" @submit.prevent="search"><label class="sr-only" for="manager-post-query">按标题或摘要搜索文章</label><Icon name="search" :size="18"/><input id="manager-post-query" v-model="query" type="search" maxlength="64" placeholder="搜索标题或摘要" :disabled="busy"><button :disabled="loading||busy">搜索</button></form><div v-if="owner" class="manager-scope" aria-label="文章范围"><button :aria-pressed="scope==='site'" :disabled="loading||busy" @click="scope='site';reset()">全站</button><button :aria-pressed="scope==='mine'" :disabled="loading||busy" @click="scope='mine';reset()">我的</button></div></div>
 <div class="manager-status-tabs" aria-label="文章状态"><button v-for="s in states" :key="s.id" :aria-pressed="status===s.id" :disabled="loading||busy" @click="status=s.id;reset()">{{s.name}}</button></div>
 <p v-if="error" class="notice error" role="alert">{{error}} <button class="quiet" :disabled="loading" @click="load">重新加载</button></p><p v-if="notice" class="notice" role="status">{{notice}}</p>
 <p class="manager-table-hint">左右滑动，查看完整信息与操作</p>
 <div class="manager-table-wrap" tabindex="0" aria-label="文章表格，可横向滚动" :aria-busy="loading"><table class="manager-table posts-table"><caption class="sr-only">文章、作者、发布状态、更新时间与操作</caption><thead><tr><th scope="col">文章</th><th v-if="owner&&scope==='site'" scope="col">作者</th><th scope="col">状态</th><th scope="col">更新于</th><th scope="col">操作</th></tr></thead><tbody><tr v-if="loading&&!rows.length"><td :colspan="owner&&scope==='site'?5:4"><div class="manager-loading" role="status">正在载入文章…</div></td></tr><tr v-for="row in rows" :key="row.id"><td class="manager-title"><a :href="`/studio/posts/${row.id}`">{{row.title||'未命名文章'}}</a><small v-if="row.summary">{{row.summary}}</small></td><td v-if="owner&&scope==='site'" class="manager-author">{{row.author_name||'作者 '+row.author_id}}</td><td><span class="table-status" :class="row.status">{{label(row)}}</span></td><td class="manager-date"><time :datetime="row.updated_at">{{timestamp(row.updated_at)}}</time></td><td><div class="manager-row-actions"><a v-if="row.status!=='trashed'" class="button quiet" :href="`/studio/posts/${row.id}`">编辑</a><button v-else class="quiet" :disabled="busy||loading" @click="action(row,'recover')">恢复草稿</button><RowMenu v-if="row.status!=='trashed'" :label="'更多操作：'+row.title"><a v-if="row.status==='published'" :href="`/posts/${row.id}`" target="_blank" rel="noopener">查看公开文章</a><button v-if="row.status==='published'" :disabled="busy||loading" @click="action(row,'withdraw')">撤回文章</button><button class="danger" :disabled="busy||loading" @click="action(row,'trash')">移入回收站</button></RowMenu></div></td></tr><tr v-if="!loading&&!rows.length"><td :colspan="owner&&scope==='site'?5:4"><div class="manager-empty"><strong>{{appliedQuery?'没有匹配的文章':status==='trashed'?'回收站是空的':'这里还没有文章'}}</strong><p>{{appliedQuery?'试试其他标题或摘要关键词。':status==='trashed'?'移入回收站的文章可以在这里恢复。':'新草稿和公开文章会显示在这里。'}}</p><button v-if="appliedQuery" @click="query='';appliedQuery='';reset()">清除搜索</button><a v-else-if="status!=='trashed'" class="button" href="/studio/posts/new">写第一篇文章</a></div></td></tr></tbody></table></div>
 <div class="manager-pagination"><span>{{loading?'正在更新…':`第 ${page+1} 页 · 本页 ${rows.length} 篇`}}</span><div><button :disabled="page===0||loading||busy" @click="paginate(-1)">上一页</button><button :disabled="!next||loading||busy" @click="paginate(1)">下一页</button></div></div>
</section>
</template>
