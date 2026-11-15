<script setup lang="ts">
import {computed,nextTick,onBeforeUnmount,onMounted,ref,useId,watch} from 'vue'
import {api} from '../api'
import {initArticleCards} from '../contentCards'
import {toast} from '../toast'
import SelectField from './SelectField.vue'
import Icon from './Icon.vue'
import '../articleImport.css'

type ImportFormat={example:string;prompt:string;schema:string}
type ImportPreview={title:string;summary:string;markdown:string;block_count:number;card_count:number;sanitized_html:string}
type ImportResult={title:string;summary:string;markdown:string;mode:'append'|'replace';includeMetadata:boolean}
const props=defineProps<{disabled?:boolean;hasContent?:boolean;applyError?:string}>()
const emit=defineEmits<{close:[];apply:[result:ImportResult]}>()
const uid=useId(),limit=1<<20,dialog=ref<HTMLDialogElement>(),jsonInput=ref<HTMLTextAreaElement>(),fileInput=ref<HTMLInputElement>(),promptInput=ref<HTMLTextAreaElement>(),previewRoot=ref<HTMLElement>(),previewHeading=ref<HTMLElement>()
const json=ref(''),filename=ref(''),mode=ref<'append'|'replace'>('append'),includeMetadata=ref(false),preview=ref<ImportPreview|null>(null),validatedJSON=ref('')
const busy=ref(false),readingFile=ref(false),error=ref(''),message=ref(''),format=ref<ImportFormat|null>(null),formatBusy=ref(false),formatError=ref(''),showPrompt=ref(false)
const byteSize=computed(()=>new TextEncoder().encode(json.value).length)
const sizeLabel=computed(()=>byteSize.value<1024?byteSize.value+' B':(byteSize.value/1024).toFixed(1)+' KiB')
const canPreview=computed(()=>!props.disabled&&!busy.value&&!readingFile.value&&!!json.value.trim()&&byteSize.value<=limit)
const canApply=computed(()=>!props.disabled&&!busy.value&&!readingFile.value&&!!preview.value&&validatedJSON.value===json.value&&byteSize.value<=limit)
let previewController:AbortController|undefined,formatController:AbortController|undefined,request=0,fileRequest=0,formatRequest=0,closed=false

function invalidate(){request++;fileRequest++;previewController?.abort();previewController=undefined;busy.value=false;readingFile.value=false;preview.value=null;validatedJSON.value='';filename.value='';message.value='';error.value=byteSize.value>limit?'JSON 超过 1 MiB，请删减内容后再预览。':''}
watch(json,invalidate,{flush:'sync'})
watch(()=>props.disabled,disabled=>{if(disabled){request++;fileRequest++;previewController?.abort();busy.value=false;readingFile.value=false}})
function close(){if(closed)return;closed=true;request++;fileRequest++;formatRequest++;previewController?.abort();formatController?.abort();dialog.value?.close();emit('close')}
function backdrop(event:MouseEvent){if(event.target!==dialog.value)return;const box=dialog.value.getBoundingClientRect();if(event.clientX<box.left||event.clientX>box.right||event.clientY<box.top||event.clientY>box.bottom)close()}
onMounted(()=>{dialog.value?.showModal();void nextTick(()=>jsonInput.value?.focus());void loadFormat()})
onBeforeUnmount(()=>{closed=true;request++;fileRequest++;formatRequest++;previewController?.abort();formatController?.abort();dialog.value?.close()})

async function loadFormat(){
 if(closed||props.disabled||formatBusy.value)return
 const at=++formatRequest;formatController?.abort();const controller=new AbortController();formatController=controller;formatBusy.value=true;formatError.value=''
 try{const data=await api<ImportFormat>('/editor/import-format',{signal:controller.signal});if(at!==formatRequest||closed)return;if(!data||['example','prompt','schema'].some(key=>typeof data[key as keyof ImportFormat]!=='string'))throw new Error('导入格式未完整载入，请重试。');format.value=data}catch(e){if(at===formatRequest&&!closed&&(e as Error).name!=='AbortError')formatError.value=(e as Error).message||'导入格式暂不可用，请重试。'}finally{if(at===formatRequest)formatBusy.value=false}
}
async function copyPrompt(){
 if(!format.value||props.disabled)return
 message.value=''
 try{if(!navigator.clipboard?.writeText)throw new Error('clipboard unavailable');await navigator.clipboard.writeText(format.value.prompt);if(closed)return;showPrompt.value=false;toast.success('AI 提示词已复制。将它与原始内容一起交给 AI，生成文章 JSON。')}catch{if(closed)return;showPrompt.value=true;message.value='暂时无法自动复制，请选中下方提示词手动复制。';toast.warning('暂时无法自动复制，已展开提示词供手动复制。');await nextTick();promptInput.value?.focus();promptInput.value?.select()}
}
function downloadFormat(kind:'example'|'schema'){
 if(!format.value||props.disabled)return
 try{const url=URL.createObjectURL(new Blob([format.value[kind]],{type:'application/json;charset=utf-8'})),link=document.createElement('a')
 link.href=url;link.download=kind==='example'?'painter-article.example.json':'painter-article.schema.json';document.body.append(link);link.click();link.remove();setTimeout(()=>URL.revokeObjectURL(url),1000);toast.success(kind==='example'?'示例 JSON 已开始下载。':'格式规范已开始下载。')}catch{toast.error('下载未能开始，请重试。')}
}
function pasteJSON(event:ClipboardEvent){
 if(props.disabled)return
 const text=event.clipboardData?.getData('text/plain');if(text===undefined)return
 const input=event.target as HTMLTextAreaElement,next=json.value.slice(0,input.selectionStart)+text+json.value.slice(input.selectionEnd)
 if(new TextEncoder().encode(next).length>limit){event.preventDefault();error.value='粘贴后的 JSON 超过 1 MiB，当前内容已保留。'}
}
async function readFile(event:Event){
 const input=event.target as HTMLInputElement,file=input.files?.[0];input.value='';if(!file||props.disabled)return
 invalidate();const at=++fileRequest
 if(file.size>limit){error.value='文件超过 1 MiB，请选择较小的文章 JSON。';toast.error(error.value);return}
 readingFile.value=true;error.value=''
 try{const raw=await file.arrayBuffer();if(at!==fileRequest||closed||props.disabled)return;const decoded=new TextDecoder('utf-8',{fatal:true}).decode(raw);if(new TextEncoder().encode(decoded).length>limit)throw new Error('size');readingFile.value=false;json.value=decoded;filename.value=file.name;toast.info('文件已读取。请先预览，再应用到草稿。')}catch(e){if(at===fileRequest&&!closed){error.value=e instanceof TypeError?'文件不是有效的 UTF-8 文本，请导出为 UTF-8 JSON 后重试。':'文件未能读取，当前 JSON 已保留，请重新选择。';toast.error(error.value)}}finally{if(at===fileRequest)readingFile.value=false}
}
async function renderPreview(){
 if(!canPreview.value)return
 const source=json.value,at=++request;previewController?.abort();const controller=new AbortController();previewController=controller;busy.value=true;error.value='';message.value='';preview.value=null;validatedJSON.value=''
 try{const data=await api<ImportPreview>('/editor/import-preview',{method:'POST',body:source,signal:controller.signal});if(at!==request||closed||props.disabled||json.value!==source)return;if(!data||['title','summary','markdown','sanitized_html'].some(key=>typeof data[key as keyof ImportPreview]!=='string')||![data.block_count,data.card_count].every(value=>Number.isSafeInteger(value)&&value>=0))throw new Error('预览结果不完整，请重新预览。');preview.value=data;validatedJSON.value=source;await nextTick();if(at===request&&previewRoot.value){initArticleCards(previewRoot.value);previewHeading.value?.focus({preventScroll:true});if(matchMedia('(max-width:720px)').matches)previewHeading.value?.scrollIntoView({block:'start',behavior:'auto'})}}catch(e){if(at===request&&!closed&&(e as Error).name!=='AbortError'){error.value=(e as Error).message||'预览未完成，请检查 JSON 后重试。';toast.error(error.value)}}finally{if(at===request)busy.value=false}
}
function apply(){if(!canApply.value||!preview.value)return;const result=preview.value;emit('apply',{title:result.title,summary:result.summary,markdown:result.markdown,mode:mode.value,includeMetadata:includeMetadata.value})}
</script>

<template>
 <dialog ref="dialog" class="article-import-dialog" :aria-labelledby="uid+'-title'" :aria-describedby="uid+'-description'" data-import-dialog @cancel.prevent="close" @close="close" @click="backdrop">
  <header class="article-import-heading"><div><h2 :id="uid+'-title'">导入文章</h2><p :id="uid+'-description'">将整理好的文章 JSON 转为正文与卡片。预览通过后，再应用到当前草稿。</p></div><button type="button" class="quiet article-import-close" aria-label="关闭文章导入" data-import-close @click="close"><Icon name="close" :size="20"/></button></header>
  <div class="article-import-content">
   <section class="article-import-source" :aria-labelledby="uid+'-source-title'">
    <h3 :id="uid+'-source-title'">准备内容</h3>
    <div class="article-import-resources"><button type="button" class="quiet" :disabled="disabled||!format" data-import-prompt @click="copyPrompt">复制 AI 提示词</button><button type="button" class="quiet" :disabled="disabled||!format" data-import-example @click="downloadFormat('example')">下载示例 JSON</button><button type="button" class="quiet" :disabled="disabled||!format" data-import-schema @click="downloadFormat('schema')">下载格式规范</button></div>
    <p v-if="formatBusy" class="article-import-hint" role="status">正在读取导入格式…</p>
    <p v-if="formatError" class="article-import-error" role="alert" data-import-format-error>{{formatError}} <button type="button" class="quiet" :disabled="disabled||formatBusy" @click="loadFormat">重试</button></p>
    <label v-if="showPrompt" class="article-import-prompt">AI 提示词<textarea ref="promptInput" :value="format?.prompt" readonly rows="7" spellcheck="false" data-import-prompt-text @focus="event=>(event.target as HTMLTextAreaElement).select()"></textarea></label>
    <div class="article-import-file-row"><button type="button" :disabled="disabled||readingFile||busy" data-import-file-button @click="fileInput?.click()">选择 JSON 文件</button><input ref="fileInput" class="sr-only" type="file" accept=".json,application/json" tabindex="-1" :disabled="disabled||readingFile||busy" aria-label="选择文章 JSON 文件" data-import-file @change="readFile"><span v-if="filename" class="article-import-filename">{{filename}}</span></div>
    <label class="article-import-json" :for="uid+'-json'">文章 JSON<textarea :id="uid+'-json'" ref="jsonInput" v-model="json" :disabled="disabled" :aria-describedby="uid+'-size'" :aria-invalid="byteSize>limit||undefined" rows="13" spellcheck="false" placeholder="选择本地 JSON 文件，或将生成的文章 JSON 粘贴到这里" data-import-json @paste="pasteJSON"></textarea></label>
    <p :id="uid+'-size'" class="article-import-hint" :class="{'article-import-error':byteSize>limit}">{{sizeLabel}} / 1 MiB · UTF-8 JSON</p>
    <p v-if="readingFile" class="article-import-hint" role="status">正在读取文件…</p>
    <p v-if="message" class="article-import-hint" role="status" data-import-message>{{message}}</p>
    <p v-if="error" class="article-import-error" role="alert" data-import-error>{{error}}</p>
    <button type="button" :disabled="!canPreview" data-import-preview @click="renderPreview">{{busy?'正在预览…':'预览文章'}}</button>
   </section>
   <section class="article-import-preview" :aria-labelledby="uid+'-preview-title'" :aria-busy="busy">
    <div class="article-import-preview-heading"><h3 :id="uid+'-preview-title'" ref="previewHeading" tabindex="-1">导入预览</h3><span v-if="preview" class="article-import-hint" data-import-counts>{{preview.block_count}} 个内容块 · {{preview.card_count}} 张卡片</span></div>
    <div v-if="preview" ref="previewRoot" class="article-import-preview-content" data-import-result><h4 v-if="preview.title">{{preview.title}}</h4><p v-if="preview.summary" class="article-import-summary">{{preview.summary}}</p><div class="prose" v-html="preview.sanitized_html"></div></div>
    <p v-else class="article-import-empty">{{busy?'正在校验格式并生成预览…':'选择文件或粘贴 JSON，再点击“预览文章”。这里会显示转换后的正文与卡片。'}}</p>
   </section>
  </div>
  <footer class="article-import-footer"><div class="article-import-options"><div data-import-mode><SelectField v-model="mode" label="应用方式" :disabled="disabled||busy" :options="[{value:'append',label:'追加到正文末尾'},{value:'replace',label:'替换当前正文'}]"/></div><label class="check"><input v-model="includeMetadata" type="checkbox" :disabled="disabled||busy" data-import-metadata>使用文件中的标题与摘要</label></div><p v-if="hasContent&&mode==='replace'" class="article-import-replace" role="note" data-import-replace-warning>替换会覆盖当前正文，应用前需要再次确认。</p><p class="article-import-hint">应用后仍是草稿，沿用编辑器的保存与发布操作。</p><p v-if="applyError" class="article-import-error" role="alert" data-import-apply-error>{{applyError}}</p><div class="article-import-actions"><button type="button" class="quiet" data-import-cancel @click="close">取消</button><button type="button" class="primary" :disabled="!canApply" data-import-apply @click="apply">应用到草稿</button></div></footer>
 </dialog>
</template>
