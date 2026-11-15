<script setup lang="ts">
import {computed,onBeforeUnmount,onMounted,reactive,ref} from 'vue'
import {APIError,api,body,session,type Row} from '../api'
import {toast} from '../toast'
import {confirmAction} from '../confirm'
import Icon from './Icon.vue'

const props=defineProps<{profile:Row}>()
const ready=ref(false),busy=ref(false),error=ref(''),conflict=ref(false),baseline=ref('')
const form=reactive({background_preset:'paper',background_media_id:null as string|null,version:1})
const palettes=[{id:'paper',name:'纸色'},{id:'mist',name:'青雾'},{id:'ink',name:'夜蓝'},{id:'rose',name:'暮色'}]
const snapshot=()=>JSON.stringify({background_preset:form.background_preset,background_media_id:form.background_media_id})
const dirty=computed(()=>ready.value&&snapshot()!==baseline.value)
const avatar=computed(()=>props.profile.avatar_media_id?'/media/'+props.profile.avatar_media_id:props.profile.avatar_url)
const image=computed(()=>form.background_media_id?'/media/'+form.background_media_id:'')
function apply(value:Row){
 if(!palettes.some(p=>p.id===value.background_preset)||!Number.isSafeInteger(Number(value.version))||Number(value.version)<1||!(value.background_media_id===null||typeof value.background_media_id==='string'))throw new Error('主页设置未完整载入，请重试。')
 form.background_preset=value.background_preset;form.background_media_id=value.background_media_id;form.version=Number(value.version);baseline.value=snapshot();ready.value=true;conflict.value=false
}
async function load(){
 if(busy.value)return
 if(dirty.value&&!await confirmAction('重新读取主页设置？','这会放弃尚未保存的背景修改。','重新读取'))return
 busy.value=true;error.value=''
 try{apply(await api('/me/author-page'))}catch(e){error.value=(e as Error).message}finally{busy.value=false}
}
async function save(){
 if(busy.value||!ready.value||conflict.value)return
 busy.value=true;error.value=''
 try{apply(await api('/me/author-page',{method:'PUT',version:form.version,body:body({background_preset:form.background_preset,background_media_id:form.background_media_id})}));toast.success('主页背景已保存。')}catch(e){if(e instanceof APIError&&e.code==='VERSION_CONFLICT'){conflict.value=true;error.value=e.message}else toast.error((e as Error).message)}finally{busy.value=false}
}
async function upload(event:Event){
 const input=event.target as HTMLInputElement,file=input.files?.[0]
 if(!file||busy.value||!ready.value){input.value='';return}
 error.value=''
 if(!['image/png','image/jpeg','image/webp','image/gif'].includes(file.type)){error.value='请选择 PNG、JPEG、WebP 或 GIF 图片。';input.value='';return}
 if(file.size>10*1024*1024){error.value='图片不能超过 10 MB。';input.value='';return}
 busy.value=true
 try{const data=new FormData();data.append('file',file);const result=await api('/media',{method:'POST',body:data});if(result.state!=='ready'||typeof result.media_id!=='string')throw new Error('图片尚未上传完成，请稍后重试。');form.background_media_id=result.media_id;toast.success('图片已上传，保存主页背景后生效。')}catch(e){toast.error((e as Error).message)}finally{busy.value=false;input.value=''}
}
function leave(event:BeforeUnloadEvent){if(dirty.value){event.preventDefault();event.returnValue=''}}
onMounted(()=>{window.addEventListener('beforeunload',leave);void load()})
onBeforeUnmount(()=>window.removeEventListener('beforeunload',leave))
</script>

<template>
 <section class="author-page-settings panel" aria-labelledby="author-settings-title" :aria-busy="busy">
  <div class="author-settings-heading"><div><h2 id="author-settings-title">公开主页外观</h2><p>选择背景，让公开资料更有自己的样子。</p></div><a v-if="session.user?.id" class="text-link" :href="`/users/${session.user.id}`" target="_blank" rel="noopener">查看主页<Icon name="arrow" :size="16"/></a></div>
  <div v-if="error" class="notice error" role="alert"><p>{{error}}</p><button v-if="!ready||conflict" type="button" :disabled="busy" @click="load">{{conflict?'重新读取设置':'重试'}}</button></div>

  <p v-if="!ready&&!error" class="muted" role="status">正在读取主页设置…</p>
  <form v-if="ready" class="author-settings-grid" @submit.prevent="save">
   <fieldset class="author-settings-controls" :disabled="busy||conflict"><legend class="sr-only">主页背景设置</legend><fieldset class="author-palette-picker"><legend>背景配色</legend><div class="author-palette-options"><label v-for="palette in palettes" :key="palette.id" :class="{'is-selected':form.background_preset===palette.id}"><input v-model="form.background_preset" type="radio" name="author-palette" :value="palette.id" @click="form.background_media_id=null" @change="form.background_media_id=null"><span class="author-background-swatch" :data-author-preset="palette.id" aria-hidden="true"></span><span>{{palette.name}}</span></label></div></fieldset><div class="author-upload-control"><label class="button" for="author-background-file"><Icon name="image" :size="18"/>{{image?'替换背景图片':'上传背景图片'}}</label><input id="author-background-file" class="sr-only" type="file" accept="image/png,image/jpeg,image/webp,image/gif" @change="upload"><button v-if="image" type="button" class="quiet" @click="form.background_media_id=null">移除图片</button><p>建议使用宽幅图片，最大 10 MB。图片会显示在公开主页顶部。</p></div><div class="form-actions"><button class="primary" :disabled="busy||conflict||!dirty">{{busy?'正在处理…':'保存主页背景'}}</button><span v-if="dirty" class="author-settings-dirty" role="status">有未保存的修改</span></div></fieldset>
   <div class="author-settings-preview" aria-label="主页背景预览"><div class="author-backdrop" :data-author-preset="form.background_preset"><img v-if="image" :key="image" :src="image" alt="已选择的主页背景" @error="event=>(event.target as HTMLImageElement).style.display='none'"></div><div class="author-preview-identity"><img v-if="avatar" class="avatar" :src="avatar" width="44" height="44" alt=""><span v-else class="avatar">{{(profile.display_name||'?').slice(0,1)}}</span><strong>{{profile.display_name||'公开昵称'}}</strong></div><p v-if="profile.bio" class="author-preview-bio">{{profile.bio}}</p></div>
  </form>
 </section>
</template>

<style scoped>
.author-page-settings{margin-top:28px}.author-settings-heading{display:flex;justify-content:space-between;align-items:start;gap:24px;margin-bottom:28px}.author-settings-heading h2{font:600 1.2rem/1.5 var(--font-ui);margin:0 0 8px}.author-settings-heading p{margin:0;color:var(--muted);font-size:.85rem}.author-settings-heading>.text-link{flex:none;min-height:44px;font-size:.8rem}.author-page-settings .notice>p{margin:0 0 12px}.author-settings-grid{display:grid;grid-template-columns:minmax(0,1fr) minmax(260px,.85fr);gap:36px;align-items:start}.author-page-settings fieldset{padding:0;border:0;margin:0;min-width:0}.author-settings-controls{display:grid;gap:28px}.author-palette-picker>legend{font-size:.85rem;font-weight:550;margin-bottom:12px}.author-upload-control{display:flex;flex-wrap:wrap;align-items:center;gap:10px}.author-upload-control .button{font-size:.8rem;cursor:pointer;color:var(--ink)}.author-upload-control:has(input:focus-visible)>label{outline:2px solid var(--accent);outline-offset:3px}.author-upload-control>p{flex-basis:100%;font-size:.75rem;color:var(--muted);line-height:1.8;margin:2px 0 0}.author-settings-grid .form-actions{margin:0;gap:12px;flex-wrap:wrap}.author-settings-dirty{font-size:.75rem;color:var(--muted)}.author-settings-preview{background:var(--paper);border:1px solid var(--line);border-radius:var(--radius-surface);overflow:hidden;min-width:0}.author-settings-preview .author-backdrop{height:120px}.author-preview-identity{display:flex;align-items:center;gap:14px;padding:20px 20px 0}.author-preview-identity .avatar{flex:none}.author-preview-identity strong{font:500 1.2rem/1.5 var(--font-display);overflow-wrap:anywhere}.author-preview-bio{margin:14px 20px 20px;color:var(--muted);font-size:.78rem;line-height:1.8;white-space:pre-line;overflow-wrap:anywhere}.author-preview-identity:last-child{padding-bottom:20px}
@media(max-width:1100px){.author-settings-grid{grid-template-columns:minmax(0,1fr);gap:28px}.author-settings-preview{max-width:520px;grid-row:1}}
@media(max-width:600px){.author-settings-heading{gap:12px;flex-wrap:wrap;margin-bottom:24px}.author-settings-heading>.text-link{min-height:44px}.author-settings-dirty{font-size:.7rem}}

/* A palette is a compact choice; the adjacent preview carries the large artwork. */
.author-palette-options{display:grid;grid-template-columns:repeat(auto-fit,minmax(94px,1fr));gap:8px;max-width:480px}
.author-palette-options label{display:flex;align-items:center;gap:6px;min-height:var(--control-height);padding:6px 8px;border:1px solid var(--line);border-radius:var(--radius-control);font-size:var(--control-font);line-height:var(--control-leading)}
.author-palette-options label:hover{background:var(--subtle)}
.author-palette-options .is-selected{border-color:var(--accent);background:var(--accent-soft)}
.author-palette-options input{order:3;margin:0 0 0 auto;width:var(--choice-mark);height:var(--choice-mark);min-height:0}
.author-palette-options label>span:last-child{white-space:nowrap}
.author-palette-options .author-background-swatch{flex:none;width:18px;height:18px;border-radius:var(--radius-item)}
.author-palette-options .is-selected .author-background-swatch{outline:0}
.author-palette-options label:has(input:focus-visible){outline:2px solid var(--accent);outline-offset:2px}
.author-palette-options input:focus-visible{outline:0}
@media(max-width:600px){.author-palette-options{grid-template-columns:repeat(2,minmax(0,1fr))}}
</style>
