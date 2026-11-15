<script setup lang="ts">
import {t} from "../i18n"

import {ref,onMounted,onBeforeUnmount,watch,nextTick} from 'vue'
import {observeInfinite} from '../infiniteScroll'
const props=defineProps<{hasMore:boolean;loading?:boolean;disabled?:boolean;error?:string}>()
const emit=defineEmits<{load:[]}>(),sentinel=ref<HTMLElement>()
let observer:ReturnType<typeof observeInfinite>|undefined,emitting=false
function load(retry=false){if(!props.hasMore||props.loading||props.disabled||emitting||(!retry&&props.error))return;emitting=true;emit('load');void nextTick(()=>{emitting=false})}
onMounted(()=>{if(sentinel.value)observer=observeInfinite(sentinel.value,()=>load())})
watch(()=>[props.hasMore,props.loading,props.disabled,props.error],()=>void nextTick(()=>observer?.check()))
onBeforeUnmount(()=>observer?.stop())
</script>
<template><div ref="sentinel" class="infinite-status" :aria-busy="!!loading" data-infinite-sentinel><span v-if="loading" role="status">{{t("正在载入…")}}</span><template v-else-if="hasMore&&error"><span>{{t("后续内容未能载入，已显示的内容仍然保留。")}}</span><button type="button" class="quiet" :disabled="disabled" @click="load(true)">{{t("重试载入")}}</button></template><span v-else-if="hasMore" class="sr-only">{{t("滚动时自动载入后续内容")}}</span></div></template>
