<script setup lang="ts">
import {computed,nextTick,onBeforeUnmount,onMounted,ref,useId,watch} from 'vue'
import type {Row} from '../api'
import Icon from './Icon.vue'

const props=withDefaults(defineProps<{
 modelValue:string[]
 tags:Row[]
 loading?:boolean
 disabled?:boolean
}>(),{loading:false,disabled:false})
const emit=defineEmits<{'update:modelValue':[value:string[]]}>()
const maxTags=20
const id=useId(),root=ref<HTMLElement>(),input=ref<HTMLInputElement>()
const open=ref(false),query=ref(''),activeIndex=ref(-1),composing=ref(false)
let compositionEndedAt=0
const blocked=computed(()=>props.disabled||props.loading)
const selectedIds=computed(()=>props.modelValue.map(String))
const selectedSet=computed(()=>new Set(selectedIds.value))
const atLimit=computed(()=>selectedIds.value.length>=maxTags)
const options=computed(()=>Array.from(new Map(props.tags.filter(tag=>tag.id!=null).map(tag=>{
 const id=String(tag.id)
 return [id,{id,name:String(tag.name||`标签 ${id}`)}] as const
})).values()))
const names=computed(()=>new Map(options.value.map(tag=>[tag.id,tag.name])))
const selectedTags=computed(()=>selectedIds.value.map(id=>({
 id,name:names.value.get(id)||`不可用标签（${id}）`,unavailable:!names.value.has(id)
})))
const hasUnavailable=computed(()=>selectedTags.value.some(tag=>tag.unavailable))
const normalize=(value:string)=>value.normalize('NFKC').toLocaleLowerCase().trim()
const filteredOptions=computed(()=>{
 const search=normalize(query.value)
 return search?options.value.filter(tag=>normalize(tag.name).includes(search)):options.value
})
const activeOption=computed(()=>open.value?filteredOptions.value[activeIndex.value]:undefined)
const hint=computed(()=>{
 if(props.loading)return '正在载入标签…'
 if(atLimit.value)return '已达到 20 个标签上限，移除一个后可继续添加。'
 if(!options.value.length)return '站点还没有可选标签，可以先继续写作。'
 return '可选择多个标签，最多 20 个。'
})

function canToggle(id:string){return selectedSet.value.has(id)||!atLimit.value}
function firstOption(){return filteredOptions.value.findIndex(tag=>canToggle(tag.id))}
function openDropdown(){if(blocked.value)return;open.value=true;if(activeIndex.value<0)activeIndex.value=firstOption()}
function closeDropdown(){open.value=false;query.value='';activeIndex.value=-1}
function focusInput(){if(blocked.value)return;input.value?.focus();openDropdown()}
function toggleDropdown(){
 if(blocked.value)return
 const wasOpen=open.value
 input.value?.focus()
 if(wasOpen)closeDropdown();else openDropdown()
}
function removeTag(id:string){
 if(blocked.value)return
 emit('update:modelValue',selectedIds.value.filter(value=>value!==id))
 focusInput()
}
function toggleTag(id:string){
 if(blocked.value)return
 if(!canToggle(id)){focusInput();return}
 emit('update:modelValue',selectedSet.value.has(id)?selectedIds.value.filter(value=>value!==id):[...selectedIds.value,id])
 query.value=''
 focusInput()
}
function scrollToActive(){
 void nextTick(()=>root.value?.querySelector<HTMLElement>(`[data-option-index="${activeIndex.value}"]`)?.scrollIntoView({block:'nearest'}))
}
function moveActive(direction:number){
 const available=filteredOptions.value.map((tag,index)=>canToggle(tag.id)?index:-1).filter(index=>index>=0)
 if(!available.length){activeIndex.value=-1;return}
 const current=available.indexOf(activeIndex.value)
 activeIndex.value=available[current<0?(direction>0?0:available.length-1):(current+direction+available.length)%available.length]!
 scrollToActive()
}
function keydown(event:KeyboardEvent){
 if(blocked.value)return
 if(event.isComposing||composing.value||event.keyCode===229)return
 if(event.key==='Enter'){
  event.preventDefault()
  event.stopPropagation()
  if(Date.now()-compositionEndedAt<50)return
  if(!open.value){openDropdown();return}
  if(activeOption.value)toggleTag(activeOption.value.id)
 }else if(event.key==='ArrowDown'||event.key==='ArrowUp'){
  event.preventDefault()
  event.stopPropagation()
  const wasOpen=open.value
  openDropdown()
  if(wasOpen)moveActive(event.key==='ArrowDown'?1:-1)
  else if(event.key==='ArrowUp'){activeIndex.value=-1;moveActive(-1)}
  else scrollToActive()
 }else if(event.key==='Escape'&&(open.value||query.value)){
  event.preventDefault()
  event.stopPropagation()
  closeDropdown()
 }
}
function endComposition(){composing.value=false;compositionEndedAt=Date.now()}
function leave(event:FocusEvent){
 if(!(event.relatedTarget instanceof Node)||!root.value?.contains(event.relatedTarget))closeDropdown()
}
function outside(event:PointerEvent){
 if(event.target instanceof Node&&!root.value?.contains(event.target))closeDropdown()
}
watch(query,()=>{activeIndex.value=firstOption()})
watch(options,()=>{activeIndex.value=firstOption()})
watch(atLimit,()=>{if(activeOption.value&&!canToggle(activeOption.value.id))activeIndex.value=firstOption()})
watch(blocked,value=>{if(value)closeDropdown()})
onMounted(()=>document.addEventListener('pointerdown',outside,true))
onBeforeUnmount(()=>document.removeEventListener('pointerdown',outside,true))
</script>

<template>
 <fieldset ref="root" class="tag-picker" :disabled="blocked" :aria-busy="loading" @focusout="leave">
  <legend><label :for="`${id}-input`">标签</label><span class="tag-count">已选 {{selectedIds.length}} / 20</span></legend>
  <div class="tag-picker-body">
   <div class="tag-control" :class="{'is-disabled':blocked}" @click="focusInput">
    <button v-for="tag in selectedTags" :key="tag.id" class="selected-tag" type="button" :disabled="blocked" :aria-label="`移除标签：${tag.name}`" :title="tag.unavailable?'这个标签已归档或暂不可用，当前选择已保留。':tag.name" @click.stop="removeTag(tag.id)">
     <span>{{tag.name}}</span><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.65" stroke-linecap="round" aria-hidden="true"><path d="m6 6 12 12M18 6 6 18"/></svg>
    </button>
    <div class="tag-input-row">
     <input :id="`${id}-input`" ref="input" v-model="query" class="tag-search" type="text" role="combobox" :disabled="blocked" autocomplete="off" :spellcheck="false" placeholder="搜索并选择标签" :aria-expanded="open" aria-autocomplete="list" aria-haspopup="listbox" :aria-controls="open?`${id}-list`:undefined" :aria-activedescendant="activeOption?`${id}-option-${activeIndex}`:undefined" :aria-describedby="`${id}-hint${hasUnavailable?` ${id}-unavailable`:''}`" @focus="openDropdown" @input="openDropdown" @keydown="keydown" @compositionstart="composing=true" @compositionend="endComposition">
     <button class="tag-toggle" type="button" :disabled="blocked" :aria-label="open?'收起标签选项':'展开标签选项'" :aria-expanded="open" :aria-controls="open?`${id}-list`:undefined" tabindex="-1" @click.stop="toggleDropdown"><Icon name="chevron" :size="18"/></button>
    </div>
   </div>
   <div v-if="open" class="tag-dropdown">
    <div v-if="filteredOptions.length" :id="`${id}-list`" class="tag-options" role="listbox" aria-label="可选标签" aria-multiselectable="true">
     <button v-for="(tag,index) in filteredOptions" :id="`${id}-option-${index}`" :key="tag.id" class="tag-option" :class="{'is-active':index===activeIndex}" type="button" role="option" :aria-selected="selectedSet.has(tag.id)" :aria-disabled="!canToggle(tag.id)" :data-option-index="index" tabindex="-1" @mousedown.prevent @pointermove="event=>{if(event.pointerType==='mouse'&&canToggle(tag.id))activeIndex=index}" @click="toggleTag(tag.id)">
      <span>{{tag.name}}</span><span v-if="selectedSet.has(tag.id)" class="tag-selected-label">已选</span>
     </button>
    </div>
    <div v-else :id="`${id}-list`" class="tag-empty" role="listbox" aria-label="可选标签" aria-multiselectable="true"><p>{{options.length?'没有匹配的标签':'暂无可选标签'}}</p><span>{{options.length?'试试其他关键词。':'可以先继续写作。'}}</span></div>
   </div>
  </div>
  <p :id="`${id}-hint`" class="tag-hint" aria-live="polite">{{hint}}</p>
  <p v-if="hasUnavailable" :id="`${id}-unavailable`" class="tag-hint">不可用的标签已保留，移除后无法在这里重新选择。</p>
  <span class="sr-only" role="status">{{open?`找到 ${filteredOptions.length} 个标签，已选择 ${selectedIds.length} 个。`:''}}</span>
 </fieldset>
</template>

<style scoped>
.tag-picker{min-width:0}.tag-picker legend{width:100%;padding-bottom:10px;font-size:.85rem;color:var(--ink)}.tag-picker legend label{display:inline;font-size:inherit;font-weight:550}.tag-count{margin-left:12px;color:var(--muted);font-size:.75rem;font-variant-numeric:tabular-nums}.tag-picker-body{position:relative}.tag-control{display:flex;flex-wrap:wrap;align-items:center;gap:4px;padding:3px 6px;min-height:var(--control-height);border:1px solid var(--line);border-radius:var(--radius-control);background:var(--paper);cursor:text}.tag-control:hover{border-color:var(--muted)}.tag-control:focus-within{border-color:var(--accent);outline:2px solid var(--accent);outline-offset:2px}.tag-control.is-disabled{cursor:not-allowed;background:var(--subtle)}.tag-control.is-disabled:hover{border-color:var(--line)}.selected-tag{gap:6px;min-width:0;max-width:100%;min-height:var(--control-inset-height);padding:4px 8px;border:0;border-radius:var(--radius-item);background:var(--accent-soft);color:var(--accent);font-size:var(--control-font);font-weight:500;white-space:normal;text-align:left;line-height:var(--control-leading)}.selected-tag span{min-width:0;overflow-wrap:anywhere;line-height:var(--control-leading)}.selected-tag svg{flex:none;line-height:var(--control-leading)}.selected-tag:hover{background:var(--accent-soft);color:var(--accent);text-decoration:underline;text-underline-offset:3px;line-height:var(--control-leading)}.tag-input-row{display:flex;align-items:center;flex:1 1 100px;min-width:0}.tag-search{min-width:0;min-height:var(--control-inset-height);padding:4px 8px;border:0;border-radius:0;background:transparent;font-size:var(--control-font);outline:0;line-height:var(--control-leading)}.tag-search:focus-visible{outline:0;line-height:var(--control-leading)}.tag-toggle{flex:none;min-width:var(--control-inset-height);min-height:var(--control-inset-height);padding:4px 8px;border:0;border-radius:var(--radius-control);background:transparent;color:var(--muted);line-height:var(--control-leading)}.tag-toggle[aria-expanded=true] .ui-icon{transform:rotate(180deg);line-height:var(--control-leading)}.tag-dropdown{position:absolute;inset:calc(100% + 6px) 0 auto;z-index:12;padding:6px;border-radius:var(--radius-menu);background:var(--paper);box-shadow:var(--shadow)}.tag-options{max-height:min(280px,42dvh);overflow-y:auto;overscroll-behavior:contain;scrollbar-width:thin;scrollbar-color:var(--line) var(--paper);line-height:var(--control-leading)}.tag-option{display:flex;justify-content:space-between;gap:16px;width:100%;min-height:var(--control-height);padding:var(--option-pad-y) 10px;border:0;border-radius:var(--radius-item);background:transparent;color:var(--ink);font-size:var(--control-font);font-weight:400;white-space:normal;text-align:left;line-height:var(--control-leading)}.tag-option>span:first-child{min-width:0;overflow-wrap:anywhere;line-height:var(--control-leading)}.tag-option.is-active,.tag-option:hover{background:var(--accent-soft);color:var(--accent);line-height:var(--control-leading)}.tag-option[aria-disabled=true]{color:var(--muted);cursor:not-allowed;line-height:var(--control-leading)}.tag-option[aria-disabled=true]:hover{background:transparent;line-height:var(--control-leading)}.tag-selected-label{flex:none;color:var(--accent);font-size:.75rem;font-weight:550}.tag-empty{padding:16px 10px;color:var(--muted);font-size:.82rem;line-height:1.8}.tag-empty p{margin:0 0 4px;color:var(--ink)}.tag-empty span{font-size:.78rem}.tag-hint{margin:8px 0 0;color:var(--muted);font-size:.78rem;line-height:1.8}.tag-search::selection{color:var(--ink);background:var(--accent-soft);line-height:var(--control-leading)}
@media(max-width:720px){.tag-search{font-size:var(--control-font);line-height:var(--control-leading)}.tag-count{font-size:.72rem}.tag-option{font-size:var(--control-font);line-height:var(--control-leading)}.tag-hint{font-size:.75rem}}
</style>
