<script setup lang="ts">
import {computed,nextTick,onBeforeUnmount,onMounted,reactive,ref,watch} from 'vue'
import {api,body,type Row} from '../api'
import {articleCardKinds,articleCardMarkdown,isInternalArticleCard,internalArticleCardURL,type ArticleCard,type ArticleCardType} from '../articleCards'
import SelectField from './SelectField.vue'
import Icon from './Icon.vue'
import {initArticleCards} from '../contentCards'

const props=defineProps<{initial?:ArticleCard|null;label?:string;inline?:boolean;kind?:ArticleCardType;editing?:boolean;disabled?:boolean}>()
const emit=defineEmits<{close:[];apply:[value:{card:ArticleCard;inline:boolean;label:string}]}>()
const initial=props.initial
const form=reactive({type:initial?.type||props.kind||'github',url:initial?.url||'',title:initial?.title||'',description:initial?.description||'',label:props.label||'',emoji:'',inline:props.inline??(initial?.type||props.kind)==='link'})
const snapshot=ref<ArticleCard|null>(initial||null),busy=ref(false),error=ref(''),preview=ref(''),urlInput=ref<HTMLInputElement>(),previewRoot=ref<HTMLElement>(),referenceFilter=ref<HTMLInputElement>()
const references=ref<Row[]>([]),referenceQuery=ref(''),referencesBusy=ref(false),referencesError=ref(''),nextCursor=ref(''),hasMore=ref(false)
const isLink=computed(()=>form.type==='link'),isInternal=computed(()=>isInternalArticleCard(form.type)),internalLabel=computed(()=>form.type==='moment'?'动态':'片语')
const ready=computed(()=>isInternal.value?!!snapshot.value?.ref_id&&snapshot.value.type===form.type:!!form.url.trim()&&!!form.title.trim()&&(isLink.value||!!snapshot.value))
const filteredReferences=computed(()=>{const query=referenceQuery.value.trim().toLocaleLowerCase();return query?references.value.filter(row=>[row.text,row.source,row.author?.display_name].some(value=>typeof value==='string'&&value.toLocaleLowerCase().includes(query))):references.value})
let controller:AbortController|undefined,referenceController:AbortController|undefined,request=0,referenceRequest=0
watch(()=>[form.type,form.url],()=>{if(snapshot.value&&(form.type!==snapshot.value.type||form.url!==snapshot.value.url))snapshot.value=null;preview.value='';error.value=''})
watch(()=>form.type,(value,previous)=>{if(value===previous)return;request++;controller?.abort();busy.value=false;form.inline=value==='link';snapshot.value=null;form.url='';form.title='';form.description='';referenceQuery.value='';references.value=[];referenceRequest++;referenceController?.abort();referencesError.value='';referencesBusy.value=false;if(isInternalArticleCard(value))void loadReferences(true);void nextTick(()=>isInternalArticleCard(value)?referenceFilter.value?.focus():urlInput.value?.focus())})
watch(()=>props.disabled,value=>{if(value)close()})
watch(()=>[form.title,form.description],()=>{preview.value=''})
onMounted(()=>{void nextTick(()=>isInternal.value?referenceFilter.value?.focus():urlInput.value?.focus());if(isInternal.value)void loadReferences(true);if(initial)void showPreview(initial).catch(e=>{error.value=(e as Error).message})})
onBeforeUnmount(()=>{request++;referenceRequest++;controller?.abort();referenceController?.abort()})
function close(){request++;referenceRequest++;controller?.abort();referenceController?.abort();emit('close')}
function candidate():ArticleCard {
 if(isInternal.value&&snapshot.value)return {v:1,type:snapshot.value.type,url:snapshot.value.url,ref_id:snapshot.value.ref_id}
 const base=snapshot.value||{v:1 as const,type:form.type as ArticleCardType,url:form.url.trim(),title:form.title.trim(),metadata_source:'author' as const}
 const card={...base,title:form.title.trim(),description:form.description.trim()}
 if(card.title!==snapshot.value?.title||card.description!==(snapshot.value?.description||''))card.metadata_source='author'
 return card
}
async function showPreview(card:ArticleCard,signal?:AbortSignal){const at=request;const result=await api<{sanitized_html:string}>('/editor/preview',{method:'POST',body:body({markdown:articleCardMarkdown(card),seq:0}),signal});if(at===request&&!signal?.aborted){preview.value=result.sanitized_html;await nextTick();if(previewRoot.value)initArticleCards(previewRoot.value)}}
async function loadReferences(reset=false){
 if(!isInternal.value||props.disabled||referencesBusy.value&&!reset)return
 const type=form.type,at=++referenceRequest;referenceController?.abort();referenceController=new AbortController();referencesBusy.value=true;referencesError.value=''
 const cursor=reset?'':nextCursor.value
 try{const data=await api<{items:Row[];next_cursor?:string|null;has_more:boolean}>((type==='moment'?'/moments':'/phrases')+'?limit=30'+(cursor?'&cursor='+encodeURIComponent(cursor):''),{signal:referenceController.signal});if(at!==referenceRequest||type!==form.type)return;if(!Array.isArray(data.items))throw new Error('内容列表暂不可用，请重试');references.value=reset?data.items:[...references.value,...data.items];nextCursor.value=typeof data.next_cursor==='string'?data.next_cursor:'';hasMore.value=data.has_more===true}catch(e){if(at===referenceRequest&&(e as Error).name!=='AbortError')referencesError.value=(e as Error).message}finally{if(at===referenceRequest)referencesBusy.value=false}
}
async function resolve(input:Row){
 if(props.disabled||busy.value)return
 const at=++request;controller?.abort();controller=new AbortController();busy.value=true;error.value=''
 try{const result=await api<{card:ArticleCard}>('/editor/cards/resolve',{method:'POST',body:body(input),signal:controller.signal});if(at!==request)return;form.url=result.card.url;form.title=result.card.title||'';form.description=result.card.description||'';await nextTick();snapshot.value=result.card;await showPreview(result.card,controller.signal)}catch(e){if(at===request&&(e as Error).name!=='AbortError')error.value=(e as Error).message}finally{if(at===request)busy.value=false}
}
function readMetadata(){void resolve({type:form.type,url:form.url.trim()})}
function chooseReference(row:Row){void resolve({type:form.type,ref_id:String(row.id),url:internalArticleCardURL(form.type,String(row.id))})}
function referenceExcerpt(row:Row){const text=String(row.text||'');const points=Array.from(text);return points.length>130?points.slice(0,130).join('')+'…':text||'图片动态'}
async function apply(){
 if(props.disabled||busy.value||!ready.value)return
 const at=++request;controller?.abort();controller=new AbortController();busy.value=true;error.value=''
 try{const card=candidate();await showPreview(card,controller.signal);if(at!==request)return;const label=[form.emoji.trim(),form.label.trim()||card.title||internalLabel.value].filter(Boolean).join(' ');emit('apply',{card,inline:isLink.value&&form.inline,label})}catch(e){if(at===request&&(e as Error).name!=='AbortError')error.value=(e as Error).message}finally{if(at===request)busy.value=false}
}
async function previewCard(){if(!ready.value||busy.value||props.disabled)return;const at=++request;controller?.abort();controller=new AbortController();busy.value=true;error.value='';try{await showPreview(candidate(),controller.signal)}catch(e){if(at===request&&(e as Error).name!=='AbortError')error.value=(e as Error).message}finally{if(at===request)busy.value=false}}
</script>

<template>
 <form class="article-card-panel" aria-label="内容卡片" data-card-composer @submit.prevent="apply" @keydown.esc.stop.prevent="close">
  <div class="article-card-panel__heading"><h3>{{editing?'编辑内容卡片':'插入内容卡片'}}</h3><button type="button" class="quiet" aria-label="关闭内容卡片" @click="close"><Icon name="close" :size="16"/></button></div>
  <div class="article-card-panel__fields">
   <SelectField v-model="form.type" :disabled="busy||disabled" label="卡片类型" :options="[...articleCardKinds]"/>
   <template v-if="isInternal">
    <label>筛选已加载的{{internalLabel}}<input ref="referenceFilter" v-model="referenceQuery" :disabled="disabled" type="search" :placeholder="form.type==='moment'?'内容或作者':'内容或来源'" autocomplete="off" data-card-reference-filter @keydown.enter.prevent></label>
    <div class="article-card-panel__wide article-card-picker" :aria-busy="referencesBusy" :aria-label="'选择公开'+internalLabel">
     <button v-for="row in filteredReferences" :key="row.id" type="button" class="article-card-picker__item" :aria-pressed="snapshot?.ref_id===String(row.id)" :disabled="busy||disabled" :data-card-reference="row.id" @click="chooseReference(row)">
      <span class="article-card-picker__identity">{{form.type==='moment'?row.author?.display_name:(row.kind==='word'?'词语':'句子')}}<span v-if="form.type==='phrase'&&row.source">{{row.source}}</span><span v-if="snapshot?.ref_id===String(row.id)">已选择</span></span>
      <span class="article-card-picker__text">{{referenceExcerpt(row)}}</span>
      <span v-if="form.type==='moment'&&row.images?.length" class="article-card-picker__images">{{row.images.length}}张图片</span>
     </button>
     <p v-if="!referencesBusy&&!filteredReferences.length&&!referencesError" class="article-card-picker__empty">{{referenceQuery?'已加载内容中没有匹配项。':'还没有公开的'+internalLabel+'。先在内容管理中发布，再来引用。'}}</p>
     <p v-if="referencesBusy" class="article-card-picker__empty" role="status">正在读取公开{{internalLabel}}…</p>
    </div>
    <div class="article-card-panel__wide article-card-panel__actions"><button v-if="hasMore" type="button" :disabled="referencesBusy||disabled" @click="loadReferences()">加载更多{{internalLabel}}</button><button type="button" class="quiet" :disabled="referencesBusy||disabled" @click="loadReferences(true)">刷新公开列表</button><button type="button" class="quiet" :disabled="busy||disabled||!ready" data-card-preview @click="previewCard">预览卡片</button></div>
    <p v-if="referencesError" class="article-card-panel__wide article-card-panel__error" role="alert">{{referencesError}} <button type="button" class="quiet" :disabled="referencesBusy" @click="loadReferences(!references.length)">重新读取</button></p>
   </template>
   <template v-else>
    <label>网址<input ref="urlInput" v-model="form.url" :disabled="busy||disabled" type="url" placeholder="https://" maxlength="2048" autocomplete="off" data-card-url></label>
    <div class="article-card-panel__wide article-card-panel__actions"><button type="button" :disabled="busy||disabled||!form.url.trim()" data-card-resolve @click="readMetadata">读取网页信息</button><button type="button" class="quiet" :disabled="busy||disabled||!ready" data-card-preview @click="previewCard">预览卡片</button><span v-if="busy" role="status">正在读取或校验…</span></div>
    <label class="article-card-panel__wide">标题<input v-model="form.title" :disabled="busy||disabled" maxlength="200" :placeholder="isLink?'也可手动填写网页标题':'读取信息后可调整标题'" data-card-title></label>
    <label class="article-card-panel__wide">简介（可选）<textarea v-model="form.description" :disabled="busy||disabled" maxlength="1000" rows="2" placeholder="补充这条参考与正文的关系" data-card-description></textarea></label>
    <template v-if="isLink"><div class="article-card-panel__inline article-card-panel__wide"><label><input v-model="form.inline" :disabled="busy||disabled" type="checkbox">行内文字引用</label></div><template v-if="form.inline"><label>Emoji（可选）<input v-model="form.emoji" :disabled="busy||disabled" maxlength="16" placeholder="例如 🔎" aria-label="引用 Emoji"></label><label>正文链接文字<input v-model="form.label" :disabled="busy||disabled" maxlength="200" :placeholder="form.title||'默认使用网页标题'" data-card-label></label></template></template>
   </template>
  </div>
  <p class="article-card-panel__copy">{{isInternal?'仅引用当前公开的'+internalLabel+'。保存的是站内引用；来源更新后卡片随之更新，撤回或删除后不再展示原文与图片。':isLink?'网页参考可读取公开网页，也可手动填写标题。行内引用支持鼠标悬停和键盘聚焦查看来源。':'读取公开信息后插入，封面、简介与统计随文章保存。修改卡片不会自动更新已发布的文章。'}}</p>
  <div v-if="preview" ref="previewRoot" class="article-card-panel__preview"><p>文章中的卡片</p><div v-html="preview"></div></div>
  <p v-if="error" class="article-card-panel__error" role="alert" data-card-error>{{error}}</p>
  <div class="article-card-panel__actions"><button type="submit" class="primary" :disabled="busy||disabled||!ready" data-card-apply>{{busy?'正在校验…':editing?'保存卡片':'插入卡片'}}</button><button type="button" class="quiet" @click="close">取消</button></div>
 </form>
</template>
