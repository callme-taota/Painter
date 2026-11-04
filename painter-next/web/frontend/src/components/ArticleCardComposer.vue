<script setup lang="ts">
import {computed,nextTick,onBeforeUnmount,onMounted,reactive,ref,watch} from 'vue'
import {api,body} from '../api'
import {articleCardKinds,articleCardMarkdown,type ArticleCard,type ArticleCardType} from '../articleCards'
import SelectField from './SelectField.vue'
import Icon from './Icon.vue'
import {initArticleCards} from '../contentCards'

const props=defineProps<{initial?:ArticleCard|null;label?:string;inline?:boolean;kind?:ArticleCardType;editing?:boolean;disabled?:boolean}>()
const emit=defineEmits<{close:[];apply:[value:{card:ArticleCard;inline:boolean;label:string}]}>()
const initial=props.initial
const form=reactive({type:initial?.type||props.kind||'github',url:initial?.url||'',title:initial?.title||'',description:initial?.description||'',label:props.label||'',emoji:'',inline:props.inline??(initial?.type||props.kind)==='link'})
const snapshot=ref<ArticleCard|null>(initial||null),busy=ref(false),error=ref(''),preview=ref(''),urlInput=ref<HTMLInputElement>(),previewRoot=ref<HTMLElement>()
const isLink=computed(()=>form.type==='link'),ready=computed(()=>!!form.url.trim()&&!!form.title.trim()&&(isLink.value||!!snapshot.value))
let controller:AbortController|undefined,request=0
watch(()=>[form.type,form.url],()=>{if(snapshot.value&&(form.type!==snapshot.value.type||form.url!==snapshot.value.url))snapshot.value=null;preview.value='';error.value=''})
watch(()=>form.type,(value,previous)=>{if(value!==previous)form.inline=value==='link'})
watch(()=>props.disabled,value=>{if(value)close()})
watch(()=>[form.title,form.description],()=>{preview.value=''})
onMounted(()=>{void nextTick(()=>urlInput.value?.focus());if(initial)void showPreview(initial).catch(e=>{error.value=(e as Error).message})})
onBeforeUnmount(()=>{request++;controller?.abort()})
function close(){request++;controller?.abort();emit('close')}
function candidate():ArticleCard {
 const base=snapshot.value||{v:1 as const,type:form.type as ArticleCardType,url:form.url.trim(),title:form.title.trim(),metadata_source:'author' as const}
 const card={...base,title:form.title.trim(),description:form.description.trim()}
 if(card.title!==snapshot.value?.title||card.description!==(snapshot.value?.description||''))card.metadata_source='author'
 return card
}
async function showPreview(card:ArticleCard,signal?:AbortSignal){const at=request;const result=await api<{sanitized_html:string}>('/editor/preview',{method:'POST',body:body({markdown:articleCardMarkdown(card),seq:0}),signal});if(at===request&&!signal?.aborted){preview.value=result.sanitized_html;await nextTick();if(previewRoot.value)initArticleCards(previewRoot.value)}}
async function readMetadata(){
 if(props.disabled||busy.value)return
 const at=++request;controller?.abort();controller=new AbortController();busy.value=true;error.value=''
 try{const result=await api<{card:ArticleCard}>('/editor/cards/resolve',{method:'POST',body:body({type:form.type,url:form.url.trim()}),signal:controller.signal});if(at!==request)return;form.url=result.card.url;form.title=result.card.title;form.description=result.card.description||'';await nextTick();snapshot.value=result.card;await showPreview(result.card,controller.signal)}catch(e){if(at===request&&(e as Error).name!=='AbortError')error.value=(e as Error).message}finally{if(at===request)busy.value=false}
}
async function apply(){
 if(props.disabled||busy.value||!ready.value)return
 const at=++request;controller?.abort();controller=new AbortController();busy.value=true;error.value=''
 try{const card=candidate();await showPreview(card,controller.signal);if(at!==request)return;const label=[form.emoji.trim(),form.label.trim()||card.title].filter(Boolean).join(' ');emit('apply',{card,inline:isLink.value&&form.inline,label})}catch(e){if(at===request&&(e as Error).name!=='AbortError')error.value=(e as Error).message}finally{if(at===request)busy.value=false}
}
async function previewCard(){if(!ready.value||busy.value||props.disabled)return;const at=++request;controller?.abort();controller=new AbortController();busy.value=true;error.value='';try{await showPreview(candidate(),controller.signal)}catch(e){if(at===request&&(e as Error).name!=='AbortError')error.value=(e as Error).message}finally{if(at===request)busy.value=false}}
</script>

<template>
 <form class="article-card-panel" aria-label="内容卡片" data-card-composer @submit.prevent="apply" @keydown.esc.stop.prevent="close">
  <div class="article-card-panel__heading"><h3>{{editing?'编辑内容卡片':'插入内容卡片'}}</h3><button type="button" class="quiet" aria-label="关闭内容卡片" @click="close"><Icon name="close" :size="16"/></button></div>
  <div class="article-card-panel__fields">
   <SelectField v-model="form.type" :disabled="busy||disabled" label="卡片类型" :options="[...articleCardKinds]"/>
   <label>网址<input ref="urlInput" v-model="form.url" :disabled="busy||disabled" type="url" placeholder="https://" maxlength="2048" autocomplete="off" data-card-url></label>
   <div class="article-card-panel__wide article-card-panel__actions"><button type="button" :disabled="busy||disabled||!form.url.trim()" data-card-resolve @click="readMetadata">读取网页信息</button><button type="button" class="quiet" :disabled="busy||disabled||!ready" data-card-preview @click="previewCard">预览卡片</button><span v-if="busy" role="status">正在读取或校验…</span></div>
   <label class="article-card-panel__wide">标题<input v-model="form.title" :disabled="busy||disabled" maxlength="200" :placeholder="isLink?'也可手动填写网页标题':'读取信息后可调整标题'" data-card-title></label>
   <label class="article-card-panel__wide">简介（可选）<textarea v-model="form.description" :disabled="busy||disabled" maxlength="1000" rows="2" placeholder="补充这条参考与正文的关系" data-card-description></textarea></label>
   <template v-if="isLink">
    <div class="article-card-panel__inline article-card-panel__wide"><label><input v-model="form.inline" :disabled="busy||disabled" type="checkbox">行内文字引用</label></div>
    <template v-if="form.inline"><label>Emoji（可选）<input v-model="form.emoji" :disabled="busy||disabled" maxlength="16" placeholder="例如 🔎" aria-label="引用 Emoji"></label><label>正文链接文字<input v-model="form.label" :disabled="busy||disabled" maxlength="200" :placeholder="form.title||'默认使用网页标题'" data-card-label></label></template>
   </template>
  </div>
  <p class="article-card-panel__copy">{{isLink?'网页参考可读取公开网页，也可手动填写标题。行内引用支持鼠标悬停和键盘聚焦查看来源。':'读取公开信息后插入，封面、简介与统计随文章保存。修改卡片不会自动更新已发布的文章。'}}</p>
  <div v-if="preview" ref="previewRoot" class="article-card-panel__preview"><p>文章中的卡片</p><div v-html="preview"></div></div>
  <p v-if="error" class="article-card-panel__error" role="alert" data-card-error>{{error}}</p>
  <div class="article-card-panel__actions"><button type="submit" class="primary" :disabled="busy||disabled||!ready" data-card-apply>{{busy?'正在校验…':editing?'保存卡片':'插入卡片'}}</button><button type="button" class="quiet" @click="close">取消</button></div>
 </form>
</template>
