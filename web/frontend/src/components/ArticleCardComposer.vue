<script setup lang="ts">
import InfiniteList from './InfiniteList.vue'
import {computed,nextTick,onBeforeUnmount,onMounted,reactive,ref,watch} from 'vue'
import {appendUnique,api,body,type Row} from '../api'
import {articleCardKinds,articleCardMarkdown,articleCardTitle,isArticlePanel,isInternalArticleCard,internalArticleCardURL,parseArticleCard,type ArticleCard,type ArticleCardType,type ArticleReferenceType,type ArticleCardMetric,type ArticleCardEntry,type ArticleLayoutCard,type ArticleLayoutNode,type ArticleLayoutText,type ArticleLayoutLink,type ArticleLayoutImage,type ArticleLayoutStack,type ArticleTextRole} from '../articleCards'
import SelectField from './SelectField.vue'
import Icon from './Icon.vue'
import {initArticleCards} from '../contentCards'
import {toast} from '../toast'

const props=defineProps<{initial?:ArticleCard|null;label?:string;inline?:boolean;kind?:ArticleCardType;editing?:boolean;disabled?:boolean}>()
const emit=defineEmits<{close:[];apply:[value:{card:ArticleCard;inline:boolean;label:string}]}>()
const initial=props.initial
let nextRow=0
const metricRow=(item?:ArticleCardMetric)=>({key:nextRow++,label:item?.label||'',value:item?.value||'',detail:item?.detail||'',source_label:item?.source?.label||'',source_url:item?.source?.url||''})
const summaryRow=(item?:ArticleCardEntry)=>({key:nextRow++,title:item?.title||'',body:item?.body||''})
const form=reactive({type:initial?.type||props.kind||'github',url:initial?.v===1?initial.url:'',title:initial&&initial.v!==3?initial.title||'':'',description:initial?.v===1?initial.description||'':'',label:props.label||'',emoji:'',inline:props.inline??(initial?.type||props.kind)==='link',tone:initial?.v===2&&initial.type==='callout'?initial.tone:'note' as 'note'|'insight'|'warning',body:initial?.v===2&&initial.type==='callout'?initial.body:'',metrics:initial?.v===2&&initial.type==='metrics'?initial.metrics.map(metricRow):[metricRow()],entries:initial?.v===2&&initial.type==='summary'?initial.entries.map(summaryRow):[summaryRow()]})
const snapshot=ref<ArticleCard|null>(initial||null),busy=ref(false),error=ref(''),preview=ref(''),urlInput=ref<HTMLInputElement>(),titleInput=ref<HTMLInputElement>(),previewRoot=ref<HTMLElement>(),referenceFilter=ref<HTMLInputElement>()
const references=ref<Row[]>([]),referenceQuery=ref(''),referencesBusy=ref(false),referencesError=ref(''),nextCursor=ref(''),hasMore=ref(false)
const isLink=computed(()=>form.type==='link'),isInternal=computed(()=>isInternalArticleCard(form.type)),internalLabel=computed(()=>form.type==='moment'?'动态':'片语')
const isPanel=computed(()=>isArticlePanel(form.type))
const isLayout=computed(()=>form.type==='layout')
const roleLabels:Record<ArticleTextRole,string>={title:'标题',label:'名称或小标题',value:'数值',body:'正文',detail:'补充说明'}
const layoutTemplates=[{value:'custom',label:'当前组合'},{value:'note',label:'要点说明'},{value:'metrics',label:'指标对照'},{value:'summary',label:'摘要并列'},{value:'image',label:'图文说明'}]
function layoutTemplateCard(kind:string):ArticleLayoutCard {
 const text=(role:ArticleTextRole,text:string):ArticleLayoutText=>({kind:'text',role,text})
 if(kind==='metrics')return {v:3,type:'layout',root:{kind:'panel',children:[text('title','指标对照'),{kind:'grid',children:[1,2].map(index=>({kind:'stack',children:[text('label','指标 '+index),text('value','')]}))}]}}
 if(kind==='summary')return {v:3,type:'layout',root:{kind:'panel',children:[text('title','摘要要点'),{kind:'grid',children:[1,2].map(index=>({kind:'stack',children:[text('label','要点 '+index),text('body','')]}))}]}}
 if(kind==='image')return {v:3,type:'layout',root:{kind:'panel',children:[text('title','图文说明'),{kind:'image',src:'',alt:''},text('body','')]}}
 return {v:3,type:'layout',root:{kind:'panel',tone:'note',children:[text('title','要点说明'),text('body','')]}}
}
const layout=ref<ArticleLayoutCard>(initial?.v===3?JSON.parse(JSON.stringify(initial)):layoutTemplateCard('note'))
const layoutTemplate=ref(initial?.v===3?'custom':'note'),layoutMode=ref<'fields'|'json'>('fields'),layoutJSON=ref(''),layoutFieldsRoot=ref<HTMLElement>()
type LayoutLeaf=ArticleLayoutText|ArticleLayoutLink|ArticleLayoutImage
type LayoutField={node:LayoutLeaf;parent:ArticleLayoutNode[];path:string}
type LayoutGroup={title:string;path:string;fields:LayoutField[];grid?:{kind:'grid';children:ArticleLayoutStack[]};item?:ArticleLayoutStack}
function leafFields(nodes:ArticleLayoutNode[],path:string):LayoutField[]{return nodes.flatMap((node,index)=>'children'in node?leafFields(node.children,path+'.'+index):[{node,parent:nodes,path:path+'.'+index}])}
const layoutGroups=computed(()=>{
 const groups:LayoutGroup[]=[]
 let fields:LayoutField[]=[]
 const flush=()=>{if(fields.length){groups.push({title:'内容',path:fields[0].path,fields});fields=[]}}
 layout.value.root.children.forEach((node,index)=>{
  if(node.kind==='grid'){flush();node.children.forEach((item,itemIndex)=>groups.push({title:'第 '+(itemIndex+1)+' 项',path:'grid.'+index+'.'+itemIndex,fields:leafFields(item.children,'grid.'+index+'.'+itemIndex),grid:node,item}))}
  else if(node.kind==='stack')fields.push(...leafFields(node.children,'stack.'+index))
  else fields.push({node,parent:layout.value.root.children,path:'root.'+index})
 })
 flush()
 return groups
})
function layoutCounts(items:ArticleLayoutNode[]){let nodes=0,images=0;const walk=(items:ArticleLayoutNode[])=>{for(const node of items){nodes++;if(node.kind==='image')images++;if('children'in node)walk(node.children)}};walk(items);return {nodes,images}}
const layoutStats=computed(()=>{const result=layoutCounts(layout.value.root.children);return {...result,nodes:result.nodes+1}})
const parsedLayout=computed(()=>parseArticleCard(layoutMode.value==='json'?layoutJSON.value:JSON.stringify(layout.value)))
const layoutReady=computed(()=>parsedLayout.value?.v===3)
const ready=computed(()=>isLayout.value?layoutReady.value:isPanel.value?!!parseArticleCard(JSON.stringify(candidate())):isInternal.value?snapshot.value?.v===1&&!!snapshot.value.ref_id&&snapshot.value.type===form.type:!!form.url.trim()&&!!form.title.trim()&&(isLink.value||!!snapshot.value))
const filteredReferences=computed(()=>{const query=referenceQuery.value.trim().toLocaleLowerCase();return query?references.value.filter(row=>[row.text,row.source,row.author?.display_name].some(value=>typeof value==='string'&&value.toLocaleLowerCase().includes(query))):references.value})
let controller:AbortController|undefined,referenceController:AbortController|undefined,request=0,referenceRequest=0
watch(()=>[form.type,form.url],()=>{if(snapshot.value&&(form.type!==snapshot.value.type||snapshot.value.v===1&&form.url!==snapshot.value.url))snapshot.value=null;preview.value='';error.value=''})
function focusPrimary(){if(isInternal.value)referenceFilter.value?.focus();else if(isLayout.value)layoutFieldsRoot.value?.querySelector<HTMLElement>('input,textarea')?.focus();else if(isPanel.value)titleInput.value?.focus();else urlInput.value?.focus()}
watch(()=>form.type,(value,previous)=>{if(value===previous)return;request++;controller?.abort();busy.value=false;form.inline=value==='link';snapshot.value=null;form.url='';form.title='';form.description='';form.body='';form.tone='note';form.metrics=[metricRow()];form.entries=[summaryRow()];layout.value=layoutTemplateCard('note');layoutTemplate.value='note';layoutMode.value='fields';referenceQuery.value='';references.value=[];referenceRequest++;referenceController?.abort();referencesError.value='';referencesBusy.value=false;if(isInternalArticleCard(value))void loadReferences(true);void nextTick(focusPrimary)})
watch(()=>props.disabled,value=>{if(value)close()})
watch(()=>[form.title,form.description,form.tone,form.body,form.metrics,form.entries],()=>{preview.value=''},{deep:true})
watch(layoutTemplate,value=>{if(value!=='custom'){layout.value=layoutTemplateCard(value);layoutMode.value='fields';void nextTick(focusPrimary)}})
watch([layout,layoutJSON],()=>{if(!isLayout.value)return;request++;controller?.abort();busy.value=false;preview.value='';error.value=''},{deep:true})
onMounted(()=>{void nextTick(focusPrimary);if(isInternal.value)void loadReferences(true);if(initial)void showPreview(initial).catch(e=>{error.value=(e as Error).message})})
onBeforeUnmount(()=>{request++;referenceRequest++;controller?.abort();referenceController?.abort()})
function close(){request++;referenceRequest++;controller?.abort();referenceController?.abort();emit('close')}
function candidate():ArticleCard {
 if(isLayout.value){if(parsedLayout.value?.v!==3)throw new Error('请补全组合内容，或修正高级 JSON 后再预览。');return parsedLayout.value}
 const title=form.title.trim()?{title:form.title.trim()}:{}
 if(form.type==='callout')return {v:2,type:'callout',...title,tone:form.tone,body:form.body}
 if(form.type==='metrics')return {v:2,type:'metrics',...title,metrics:form.metrics.map(item=>({label:item.label.trim(),value:item.value.trim(),...(item.detail?{detail:item.detail}:{}),...(item.source_label||item.source_url?{source:{label:item.source_label.trim(),url:item.source_url.trim()}}:{})}))}
 if(form.type==='summary')return {v:2,type:'summary',...title,entries:form.entries.map(item=>({title:item.title.trim(),body:item.body}))}
 if(isInternal.value&&snapshot.value?.v===1)return {v:1,type:snapshot.value.type,url:snapshot.value.url,ref_id:snapshot.value.ref_id}
 const base=snapshot.value?.v===1?snapshot.value:{v:1 as const,type:form.type as ArticleReferenceType,url:form.url.trim(),title:form.title.trim(),metadata_source:'author' as const}
 const card={...base,title:form.title.trim(),description:form.description.trim()}
 if(card.title!==(snapshot.value?.v===1?snapshot.value.title:'')||card.description!==(snapshot.value?.v===1?snapshot.value.description||'':''))card.metadata_source='author'
 return card
}
function layoutEditMode(mode:'fields'|'json'){
 if(mode===layoutMode.value)return
 if(mode==='json')layoutJSON.value=JSON.stringify(layout.value,null,2)
 else{const card=parseArticleCard(layoutJSON.value);if(card?.v!==3){error.value='请先修正 JSON，才能返回表单编辑。输入内容已保留。';return}layout.value=card;layoutTemplate.value='custom'}
 layoutMode.value=mode;error.value=''
}
function removeLayoutField(field:LayoutField){if(field.parent.length>1){const at=field.parent.indexOf(field.node);if(at>=0)field.parent.splice(at,1)}}
function removeLayoutItem(group:LayoutGroup){if(group.grid&&group.item&&group.grid.children.length>1){const at=group.grid.children.indexOf(group.item);if(at>=0)group.grid.children.splice(at,1)}}
function canAddLayoutItem(group:LayoutGroup){if(!group.grid||!group.item||group.grid.children.length>=6)return false;const added=layoutCounts([group.item]);return layoutStats.value.nodes+added.nodes<=64&&layoutStats.value.images+added.images<=6}
function addLayoutItem(group:LayoutGroup){if(!canAddLayoutItem(group)||!group.grid||!group.item)return;const item=JSON.parse(JSON.stringify(group.item)) as ArticleLayoutStack;for(const field of leafFields(item.children,'')){if(field.node.kind==='text')field.node.text=field.node.role==='label'?'新项目':'';else if(field.node.kind==='link'){field.node.text='来源';field.node.url=''}else{field.node.src='';field.node.alt=''}};group.grid.children.push(item)}
function addLayoutField(kind:'text'|'link'|'image'){
 if(layout.value.root.children.length>=12||layoutStats.value.nodes>=64||kind==='image'&&layoutStats.value.images>=6)return
 layout.value.root.children.push(kind==='text'?{kind:'text',role:'body',text:''}:kind==='link'?{kind:'link',text:'来源',url:''}:{kind:'image',src:'',alt:''})
 void nextTick(()=>Array.from(layoutFieldsRoot.value?.querySelectorAll<HTMLElement>('input,textarea')||[]).at(-1)?.focus())
}
async function uploadLayoutImage(node:ArticleLayoutImage,event:Event){
 const input=event.target as HTMLInputElement,file=input.files?.[0];input.value='';if(!file||props.disabled||busy.value)return
 if(file.size>10*1024*1024){error.value='图片超过 10 MiB，请压缩后重试。';toast.error(error.value);return}
 const at=++request;controller?.abort();controller=new AbortController();busy.value=true;error.value=''
 try{const data=new FormData();data.append('file',file);const result=await api<{media_id:string;state?:string}>('/media',{method:'POST',body:data,signal:controller.signal});if(at!==request)return;if(typeof result.media_id!=='string'||!/^[/]media[/][a-fA-F0-9-]{16,128}$/.test('/media/'+result.media_id)||result.state&&result.state!=='ready')throw new Error('图片尚未上传完成，请重试。');node.src='/media/'+result.media_id;if(!node.alt)node.alt=Array.from(file.name).slice(0,200).join('');toast.success('图片已上传到卡片，插入草稿后仍需保存。')}catch(e){if(at===request&&(e as Error).name!=='AbortError'){error.value='图片上传失败：'+(e as Error).message;toast.error(error.value)}}finally{if(at===request)busy.value=false}
}
async function showPreview(card:ArticleCard,signal?:AbortSignal){const at=request;const result=await api<{sanitized_html:string}>('/editor/preview',{method:'POST',body:body({markdown:articleCardMarkdown(card),seq:0}),signal});if(at===request&&!signal?.aborted){preview.value=result.sanitized_html;await nextTick();if(previewRoot.value)initArticleCards(previewRoot.value)}}
async function loadReferences(reset=false){
 if(!isInternal.value||props.disabled||referencesBusy.value&&!reset)return
 const type=form.type,at=++referenceRequest;referenceController?.abort();referenceController=new AbortController();referencesBusy.value=true;referencesError.value=''
 const cursor=reset?'':nextCursor.value
 try{const data=await api<{items:Row[];next_cursor?:string|null;has_more:boolean}>((type==='moment'?'/moments':'/phrases')+'?limit=30'+(cursor?'&cursor='+encodeURIComponent(cursor):''),{signal:referenceController.signal});if(at!==referenceRequest||type!==form.type)return;if(!Array.isArray(data.items))throw new Error('内容列表暂不可用，请重试');references.value=reset?data.items:appendUnique(references.value,data.items);nextCursor.value=typeof data.next_cursor==='string'?data.next_cursor:'';hasMore.value=data.has_more===true}catch(e){if(at===referenceRequest&&(e as Error).name!=='AbortError')referencesError.value=(e as Error).message}finally{if(at===referenceRequest)referencesBusy.value=false}
}
async function resolve(input:Row){
 if(props.disabled||busy.value)return
 const at=++request;controller?.abort();controller=new AbortController();busy.value=true;error.value=''
 try{const result=await api<{card:ArticleCard}>('/editor/cards/resolve',{method:'POST',body:body(input),signal:controller.signal});if(at!==request)return;if(result.card.v!==1)throw new Error('卡片信息不完整，请重新读取。');form.url=result.card.url;form.title=result.card.title||'';form.description=result.card.description||'';await nextTick();snapshot.value=result.card;await showPreview(result.card,controller.signal);if(at===request)toast.success('卡片信息已读取，请确认后插入草稿。')}catch(e){if(at===request&&(e as Error).name!=='AbortError'){error.value=(e as Error).message;toast.error(error.value)}}finally{if(at===request)busy.value=false}
}
function readMetadata(){void resolve({type:form.type,url:form.url.trim()})}
function chooseReference(row:Row){void resolve({type:form.type,ref_id:String(row.id),url:internalArticleCardURL(form.type,String(row.id))})}
function referenceExcerpt(row:Row){const text=String(row.text||'');const points=Array.from(text);return points.length>130?points.slice(0,130).join('')+'…':text||'图片动态'}
async function apply(){
 if(props.disabled||busy.value||!ready.value)return
 const at=++request;controller?.abort();controller=new AbortController();busy.value=true;error.value=''
 try{const card=candidate();await showPreview(card,controller.signal);if(at!==request)return;const label=[form.emoji.trim(),form.label.trim()||articleCardTitle(card)].filter(Boolean).join(' ');emit('apply',{card,inline:isLink.value&&form.inline,label})}catch(e){if(at===request&&(e as Error).name!=='AbortError'){error.value=(e as Error).message;toast.error(error.value)}}finally{if(at===request)busy.value=false}
}
async function previewCard(){if(!ready.value||busy.value||props.disabled)return;const at=++request;controller?.abort();controller=new AbortController();busy.value=true;error.value='';try{await showPreview(candidate(),controller.signal);if(at===request)toast.info('卡片预览已更新，尚未应用到草稿。')}catch(e){if(at===request&&(e as Error).name!=='AbortError'){error.value=(e as Error).message;toast.error(error.value)}}finally{if(at===request)busy.value=false}}
</script>

<template>
 <form class="article-card-panel" aria-label="内容卡片" data-card-composer @submit.prevent="apply" @keydown.esc.stop.prevent="close">
  <div class="article-card-panel__heading"><h3>{{editing?'编辑内容卡片':'插入内容卡片'}}</h3><button type="button" class="quiet" aria-label="关闭内容卡片" @click="close"><Icon name="close" :size="16"/></button></div>
  <div class="article-card-panel__fields">
   <SelectField v-model="form.type" :disabled="busy||disabled" label="卡片类型" :options="[...articleCardKinds]"/>
   <template v-if="isInternal">
    <label>筛选已加载的{{internalLabel}}<input ref="referenceFilter" v-model="referenceQuery" :disabled="disabled" type="search" :placeholder="form.type==='moment'?'内容或作者':'内容或来源'" autocomplete="off" data-card-reference-filter @keydown.enter.prevent></label>
    <div class="article-card-panel__wide article-card-picker" :aria-busy="referencesBusy" :aria-label="'选择公开'+internalLabel">
     <button v-for="row in filteredReferences" :key="row.id" type="button" class="article-card-picker__item" :aria-pressed="snapshot?.v===1&&snapshot.ref_id===String(row.id)" :disabled="busy||disabled" :data-card-reference="row.id" @click="chooseReference(row)">
      <span class="article-card-picker__identity">{{form.type==='moment'?row.author?.display_name:(row.kind==='word'?'词语':'句子')}}<span v-if="form.type==='phrase'&&row.source">{{row.source}}</span><span v-if="snapshot?.v===1&&snapshot.ref_id===String(row.id)">已选择</span></span>
      <span class="article-card-picker__text">{{referenceExcerpt(row)}}</span>
      <span v-if="form.type==='moment'&&row.images?.length" class="article-card-picker__images">{{row.images.length}}张图片</span>
     </button>
     <p v-if="!referencesBusy&&!filteredReferences.length&&!referencesError" class="article-card-picker__empty">{{referenceQuery?'已加载内容中没有匹配项。':'还没有公开的'+internalLabel+'。先在内容管理中发布，再来引用。'}}</p>
     <p v-if="referencesBusy" class="article-card-picker__empty" role="status">正在读取公开{{internalLabel}}…</p>
     <InfiniteList :has-more="hasMore" :loading="referencesBusy" :disabled="disabled" :error="referencesError" @load="loadReferences()"/>
    </div>
    <div class="article-card-panel__wide article-card-panel__actions"><button type="button" class="quiet" :disabled="referencesBusy||disabled" @click="loadReferences(true)">刷新公开列表</button><button type="button" class="quiet" :disabled="busy||disabled||!ready" data-card-preview @click="previewCard">预览卡片</button></div>
    <p v-if="referencesError" class="article-card-panel__wide article-card-panel__error" role="alert">{{referencesError}} <button type="button" class="quiet" :disabled="referencesBusy" @click="loadReferences(!references.length)">重新读取</button></p>
   </template>
   <template v-else-if="isLayout">
    <div data-layout-template><SelectField v-model="layoutTemplate" :disabled="busy||disabled" label="组合模板" :options="layoutTemplates"/></div>
    <div class="article-card-panel__wide article-card-layout-modes" role="group" aria-label="组合卡片编辑方式"><button type="button" class="quiet" :aria-pressed="layoutMode==='fields'" :disabled="busy||disabled" data-layout-mode-fields @click="layoutEditMode('fields')">表单编辑</button><button type="button" class="quiet" :aria-pressed="layoutMode==='json'" :disabled="busy||disabled" data-layout-mode-json @click="layoutEditMode('json')">高级 JSON</button></div>
    <template v-if="layoutMode==='fields'">
     <SelectField :model-value="layout.root.tone||'plain'" :disabled="busy||disabled" label="呈现用途" :options="[{value:'plain',label:'普通内容'},{value:'note',label:'说明'},{value:'insight',label:'判断'},{value:'warning',label:'注意'}]" @update:model-value="value=>{if(value==='plain')delete layout.root.tone;else layout.root.tone=value as 'note'|'insight'|'warning'}"/>
     <div ref="layoutFieldsRoot" class="article-card-panel__wide article-card-panel__rows" data-layout-fields>
      <fieldset v-for="group in layoutGroups" :key="group.path" :disabled="busy||disabled" class="article-card-panel__row" data-layout-group>
       <legend>{{group.title}}</legend>
       <div v-for="field in group.fields" :key="field.path" class="article-card-layout-field">
        <template v-if="field.node.kind==='text'">
         <label v-if="field.node.role==='body'||field.node.role==='detail'">{{roleLabels[field.node.role]}}<textarea v-model="field.node.text" :maxlength="field.node.role==='body'?2000:500" rows="3" :data-layout-text="field.node.role" placeholder="填写这部分内容，支持换行"></textarea></label>
         <label v-else>{{roleLabels[field.node.role]}}<input v-model="field.node.text" :maxlength="field.node.role==='title'?200:field.node.role==='label'?120:100" :data-layout-text="field.node.role" :placeholder="field.node.role==='value'?'保留原始数值与单位':'填写'+roleLabels[field.node.role]"></label>
        </template>
        <template v-else-if="field.node.kind==='link'"><label>来源名称<input v-model="field.node.text" maxlength="120" data-layout-link-text></label><label>来源网址<input v-model="field.node.url" type="url" maxlength="2048" placeholder="https://" data-layout-link-url></label></template>
        <template v-else><label>图片<input type="file" accept="image/*" data-layout-image-file @change="uploadLayoutImage(field.node,$event)"></label><p class="article-card-panel__copy">{{field.node.src?'已上传图片，可重新选择替换。':'选择图片并上传后，才能预览和插入这张卡片。'}}</p><label>图片说明（可选）<input v-model="field.node.alt" maxlength="200" placeholder="描述图片内容，供无法查看图片的读者使用" data-layout-image-alt></label></template>
        <button type="button" class="quiet article-card-layout-remove" :disabled="field.parent.length<=1" aria-label="移除这部分内容" data-layout-remove-field @click="removeLayoutField(field)">移除此部分</button>
       </div>
       <div v-if="group.grid" class="article-card-panel__actions"><button type="button" class="quiet" :disabled="group.grid.children.length<=1" data-layout-remove-item @click="removeLayoutItem(group)">移除此项</button><button v-if="group.item===group.grid.children.at(-1)" type="button" class="quiet" :disabled="!canAddLayoutItem(group)" data-layout-add-item @click="addLayoutItem(group)">添加一项</button></div>
      </fieldset>
      <div class="article-card-panel__actions"><button type="button" class="quiet" :disabled="busy||disabled||layout.root.children.length>=12||layoutStats.nodes>=64" data-layout-add-text @click="addLayoutField('text')">添加正文</button><button type="button" class="quiet" :disabled="busy||disabled||layout.root.children.length>=12||layoutStats.nodes>=64" data-layout-add-link @click="addLayoutField('link')">添加来源</button><button type="button" class="quiet" :disabled="busy||disabled||layout.root.children.length>=12||layoutStats.nodes>=64||layoutStats.images>=6" data-layout-add-image @click="addLayoutField('image')">添加图片</button></div>
     </div>
    </template>
    <div v-else class="article-card-panel__wide"><label>组合卡片 JSON<textarea v-model="layoutJSON" class="article-card-layout-json" :disabled="busy||disabled" spellcheck="false" rows="14" aria-describedby="layout-json-help" data-layout-json></textarea></label><p id="layout-json-help" class="article-card-panel__copy">可调整文字、来源、图片和分组。仅支持受控组件，最多 8 KiB；不接受 HTML、脚本或自定义样式。</p><p v-if="!layoutReady" class="article-card-panel__error" role="alert" data-layout-json-error>JSON 格式或组件内容无效。请检查必填文字、图片和 HTTPS 来源；当前输入已保留。</p><button type="button" class="quiet" :disabled="busy||disabled" @click="layoutMode='fields';error=''">放弃 JSON 修改并返回表单</button></div>
    <div class="article-card-panel__wide article-card-panel__actions"><button type="button" class="quiet" :disabled="busy||disabled||!ready" data-card-preview @click="previewCard">预览卡片</button><span v-if="busy" role="status">正在上传或校验…</span></div>
    <p v-if="!ready&&layoutMode==='fields'" class="article-card-panel__wide article-card-panel__copy">请补全各部分内容。文字不能为空，来源需填写名称和 HTTPS 网址，图片需先上传。</p>
   </template>
   <template v-else-if="isPanel">
    <label>标题（可选）<input ref="titleInput" v-model="form.title" :disabled="busy||disabled" maxlength="200" placeholder="为这一组内容命名" data-card-title></label>
    <template v-if="form.type==='callout'">
     <SelectField v-model="form.tone" :disabled="busy||disabled" label="提示用途" :options="[{value:'note',label:'说明'},{value:'insight',label:'判断'},{value:'warning',label:'注意'}]"/>
     <label class="article-card-panel__wide">正文<textarea v-model="form.body" :disabled="busy||disabled" maxlength="2000" rows="4" placeholder="填写需要突出的一段说明，支持换行" data-card-body></textarea></label>
    </template>
    <div v-else-if="form.type==='metrics'" class="article-card-panel__wide article-card-panel__rows">
     <fieldset v-for="(item,index) in form.metrics" :key="item.key" :disabled="busy||disabled" class="article-card-panel__row">
      <legend>指标 {{index+1}}</legend>
      <div class="article-card-panel__fields"><label>名称<input v-model="item.label" maxlength="120" placeholder="例如 稳定币规模" data-card-metric-label></label><label>数值<input v-model="item.value" maxlength="100" placeholder="保留原始数值与单位" data-card-metric-value></label><label class="article-card-panel__wide">说明（可选）<textarea v-model="item.detail" maxlength="500" rows="2" placeholder="统计窗口、口径或补充说明" data-card-metric-detail></textarea></label><label>来源名称（可选）<input v-model="item.source_label" maxlength="120" placeholder="来源编号或名称" data-card-source-label></label><label>来源网址<input v-model="item.source_url" type="url" maxlength="2048" placeholder="https://" data-card-source-url></label></div>
      <p v-if="!!item.source_label.trim()!==!!item.source_url.trim()" class="article-card-panel__copy">来源名称和 HTTPS 网址需要一起填写。</p>
      <button type="button" class="quiet" :disabled="busy||disabled||form.metrics.length===1" :aria-label="'移除指标 '+(index+1)" @click="form.metrics.splice(index,1)">移除此项</button>
     </fieldset>
     <button type="button" class="quiet" :disabled="busy||disabled||form.metrics.length>=6" @click="form.metrics.push(metricRow())">添加指标（{{form.metrics.length}} / 6）</button>
    </div>
    <div v-else class="article-card-panel__wide article-card-panel__rows">
     <fieldset v-for="(item,index) in form.entries" :key="item.key" :disabled="busy||disabled" class="article-card-panel__row">
      <legend>摘要 {{index+1}}</legend>
      <label>小标题<input v-model="item.title" maxlength="200" placeholder="这一项的要点" data-card-entry-title></label><label>正文<textarea v-model="item.body" maxlength="2000" rows="3" placeholder="补充判断、依据或结论" data-card-entry-body></textarea></label>
      <button type="button" class="quiet" :disabled="busy||disabled||form.entries.length===1" :aria-label="'移除摘要 '+(index+1)" @click="form.entries.splice(index,1)">移除此项</button>
     </fieldset>
     <button type="button" class="quiet" :disabled="busy||disabled||form.entries.length>=6" @click="form.entries.push(summaryRow())">添加摘要（{{form.entries.length}} / 6）</button>
    </div>
    <div class="article-card-panel__wide article-card-panel__actions"><button type="button" class="quiet" :disabled="busy||disabled||!ready" data-card-preview @click="previewCard">预览卡片</button><span v-if="busy" role="status">正在校验…</span></div>
   </template>
   <template v-else>
    <label>网址<input ref="urlInput" v-model="form.url" :disabled="busy||disabled" type="url" placeholder="https://" maxlength="2048" autocomplete="off" data-card-url></label>
    <div class="article-card-panel__wide article-card-panel__actions"><button type="button" :disabled="busy||disabled||!form.url.trim()" data-card-resolve @click="readMetadata">读取网页信息</button><button type="button" class="quiet" :disabled="busy||disabled||!ready" data-card-preview @click="previewCard">预览卡片</button><span v-if="busy" role="status">正在读取或校验…</span></div>
    <label class="article-card-panel__wide">标题<input v-model="form.title" :disabled="busy||disabled" maxlength="200" :placeholder="isLink?'也可手动填写网页标题':'读取信息后可调整标题'" data-card-title></label>
    <label class="article-card-panel__wide">简介（可选）<textarea v-model="form.description" :disabled="busy||disabled" maxlength="1000" rows="2" placeholder="补充这条参考与正文的关系" data-card-description></textarea></label>
    <template v-if="isLink"><div class="article-card-panel__inline article-card-panel__wide"><label><input v-model="form.inline" :disabled="busy||disabled" type="checkbox">行内文字引用</label></div><template v-if="form.inline"><label>Emoji（可选）<input v-model="form.emoji" :disabled="busy||disabled" maxlength="16" placeholder="例如 🔎" aria-label="引用 Emoji"></label><label>正文链接文字<input v-model="form.label" :disabled="busy||disabled" maxlength="200" :placeholder="form.title||'默认使用网页标题'" data-card-label></label></template></template>
   </template>
  </div>
  <p class="article-card-panel__copy">{{isPanel||isLayout?'文字与数值按填写内容保存，不会自动核验或更新。请在说明中保留统计时间、口径与存档状态。':isInternal?'仅引用当前公开的'+internalLabel+'。保存的是站内引用；来源更新后卡片随之更新，撤回或删除后不再展示原文与图片。':isLink?'网页参考可读取公开网页，也可手动填写标题。行内引用支持鼠标悬停和键盘聚焦查看来源。':'读取公开信息后插入，封面、简介与统计随文章保存。修改卡片不会自动更新已发布的文章。'}}</p>
  <div v-if="preview" ref="previewRoot" class="article-card-panel__preview"><p>文章中的卡片</p><div v-html="preview"></div></div>
  <p v-if="error" class="article-card-panel__error" role="alert" data-card-error>{{error}}</p>
  <div class="article-card-panel__actions"><button type="submit" class="primary" :disabled="busy||disabled||!ready" data-card-apply>{{busy?'正在校验…':editing?'保存卡片':'插入卡片'}}</button><button type="button" class="quiet" @click="close">取消</button></div>
 </form>
</template>
