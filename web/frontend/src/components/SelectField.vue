<script setup lang="ts">
import {t} from "../i18n"

import {computed,nextTick,onBeforeUnmount,ref,useId,watch} from 'vue'
import Icon from './Icon.vue'
type Option={value:string|number;label:string;disabled?:boolean}
const props=defineProps<{modelValue:string|number;options:Option[];label:string;disabled?:boolean}>()
const emit=defineEmits<{'update:modelValue':[string|number]}>()
const id=useId(),trigger=ref<HTMLButtonElement>(),menu=ref<HTMLElement>(),open=ref(false),active=ref(0),position=ref<Record<string,string>>({})
const selected=computed(()=>props.options.find(o=>o.value===props.modelValue))
let buffer='',timer:ReturnType<typeof setTimeout>|undefined
function place(){
 const r=trigger.value?.getBoundingClientRect();if(!r)return
 const rowHeight=parseFloat(getComputedStyle(trigger.value!).getPropertyValue('--control-height'))||36
 const css=menu.value?getComputedStyle(menu.value):null,edges=css?parseFloat(css.borderTopWidth)+parseFloat(css.borderBottomWidth):0
 const natural=menu.value?menu.value.scrollHeight+edges:props.options.length*rowHeight+8
 const below=Math.max(0,innerHeight-r.bottom-14),aboveSpace=Math.max(0,r.top-14)
 const above=natural>below&&aboveSpace>below,maxHeight=Math.min(276,above?aboveSpace:below),height=Math.min(natural,maxHeight)
 position.value={left:Math.max(8,Math.min(r.left,innerWidth-Math.max(r.width,180)-8))+'px',top:Math.max(8,Math.min(above?r.top-height-6:r.bottom+6,innerHeight-height-8))+'px',width:Math.min(Math.max(r.width,180),innerWidth-16)+'px',maxHeight:maxHeight+'px'}
}
async function reveal(){if(props.disabled)return;open.value=true;active.value=Math.max(0,props.options.findIndex(o=>o.value===props.modelValue));if(props.options[active.value]?.disabled)move(1);await nextTick();place();scroll()}
function close(focus=false){open.value=false;if(focus)trigger.value?.focus()}
function scroll(){menu.value?.querySelector<HTMLElement>('[data-active=true]')?.scrollIntoView({block:'nearest'})}
function move(step:number){for(let i=1;i<=props.options.length;i++){const next=(active.value+step*i+props.options.length)%props.options.length;if(!props.options[next].disabled){active.value=next;void nextTick(scroll);return}}}
function choose(i:number){if(props.options[i]?.disabled)return;emit('update:modelValue',props.options[i].value);close(true)}
function key(e:KeyboardEvent){if(['ArrowDown','ArrowUp','Home','End','Enter',' '].includes(e.key)){e.preventDefault();if(!open.value){void reveal();return}if(e.key==='ArrowDown')move(1);else if(e.key==='ArrowUp')move(-1);else if(e.key==='Home'||e.key==='End'){const indexes=props.options.map((o,i)=>o.disabled?-1:i).filter(i=>i>=0);active.value=e.key==='Home'?indexes[0]:indexes[indexes.length-1];void nextTick(scroll)}else choose(active.value)}else if(e.key==='Escape'&&open.value){e.preventDefault();e.stopPropagation();close(true)}else if(e.key==='Tab')close();else if(e.key.length===1&&!e.ctrlKey&&!e.metaKey&&!e.altKey){buffer+=e.key.toLocaleLowerCase();clearTimeout(timer);timer=setTimeout(()=>buffer='',500);const index=props.options.findIndex(o=>!o.disabled&&o.label.toLocaleLowerCase().startsWith(buffer));if(index>=0){e.preventDefault();if(!open.value)void reveal();active.value=index;void nextTick(scroll)}}}
function outside(e:PointerEvent){if(!trigger.value?.contains(e.target as Node)&&!menu.value?.contains(e.target as Node))close()}
watch(open,value=>{if(value){document.addEventListener('pointerdown',outside);window.addEventListener('resize',place);window.addEventListener('scroll',place,true)}else{document.removeEventListener('pointerdown',outside);window.removeEventListener('resize',place);window.removeEventListener('scroll',place,true)}})
watch(()=>props.disabled,v=>{if(v)close()})
onBeforeUnmount(()=>{clearTimeout(timer);document.removeEventListener('pointerdown',outside);window.removeEventListener('resize',place);window.removeEventListener('scroll',place,true)})
</script>
<template>
 <div class="select-field"><span class="select-label">{{label}}</span><button ref="trigger" type="button" class="select-trigger" role="combobox" aria-haspopup="listbox" :aria-label="label" :aria-expanded="open" :aria-controls="open?id:undefined" :aria-activedescendant="open?id+'-'+active:undefined" :disabled="disabled" @click="open?close():reveal()" @keydown="key"><span>{{selected?.label||t("请选择")}}</span><Icon name="chevron" :size="16"/></button></div>
 <Teleport :to="trigger?.closest('dialog')||'body'"><ul v-if="open" :id="id" ref="menu" class="select-menu" role="listbox" :aria-label="label" :style="position"><li v-for="(option,i) in options" :id="id+'-'+i" :key="option.value" role="option" :aria-selected="option.value===modelValue" :aria-disabled="option.disabled||undefined" :data-active="i===active" @pointermove="!option.disabled&&(active=i)" @pointerdown.prevent @click="choose(i)"><span>{{option.label}}</span><svg v-if="option.value===modelValue" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" aria-hidden="true"><path d="m5 12 4 4L19 6"/></svg></li></ul></Teleport>
</template>
