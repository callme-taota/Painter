<script setup lang="ts">
import {ref,nextTick,onBeforeUnmount} from 'vue'
const props=defineProps<{label:string}>()
const trigger=ref<HTMLButtonElement>(),panel=ref<HTMLElement>(),open=ref(false),position=ref<Record<string,string>>({})
function close(focus=false){open.value=false;document.removeEventListener('pointerdown',outside);window.removeEventListener('resize',place);window.removeEventListener('scroll',place,true);if(focus)trigger.value?.focus()}
function place(){if(!trigger.value||!panel.value)return;const r=trigger.value.getBoundingClientRect(),h=panel.value.offsetHeight,w=180;position.value={left:Math.max(8,Math.min(r.right-w,innerWidth-w-8))+'px',top:Math.max(8,Math.min(r.bottom+6,innerHeight-h-8))+'px'}}
async function toggle(){if(open.value){close();return}open.value=true;await nextTick();place();document.addEventListener('pointerdown',outside);window.addEventListener('resize',place);window.addEventListener('scroll',place,true);panel.value?.querySelector<HTMLElement>('a,button')?.focus()}
function outside(e:PointerEvent){if(!trigger.value?.contains(e.target as Node)&&!panel.value?.contains(e.target as Node))close()}
function key(e:KeyboardEvent){if(e.key==='Escape'){e.preventDefault();close(true)}else if(e.key==='Tab'){const items=[...panel.value!.querySelectorAll<HTMLElement>('a,button')];if((!e.shiftKey&&e.target===items.at(-1))||(e.shiftKey&&e.target===items[0])){e.preventDefault();close(true)}}}
onBeforeUnmount(()=>close())
</script>
<template><button ref="trigger" type="button" class="quiet row-menu-trigger" :aria-label="props.label" :aria-expanded="open" @click="toggle"><svg class="icon" viewBox="0 0 24 24" aria-hidden="true"><path d="M5 12h.01M12 12h.01M19 12h.01" stroke-width="3"/></svg></button><Teleport to="body"><div v-if="open" ref="panel" class="row-menu-panel" role="group" :aria-label="label" :style="position" @keydown="key" @click="close(true)"><slot/></div></Teleport></template>
