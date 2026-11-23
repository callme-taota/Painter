<script setup lang="ts">
import {t, formatDate} from "../i18n"

import InfiniteList from './InfiniteList.vue'
import {computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, useId} from 'vue'
import {APIError, type Row} from '../api'
import {moments} from '../contentClient'
import {imageFileIssue,releaseUploadPreview} from '../mediaUpload'
import {useMediaUploadQueue} from '../useMediaUploadQueue'
import {useCursorList} from '../useCursorList'
import {confirmAction} from '../confirm'
import {toast} from '../toast'
import {randomID} from '../random'
import Icon from './Icon.vue'
import SelectField from './SelectField.vue'

type MomentImage = {media_id:string;url:string;alt:string}
type ImageInput = {media_id:string;alt:string}
type MomentPayload = {text:string;images:ImageInput[]}
type SelectedImage = MomentImage & {localID:string;state:'ready'|'queued'|'uploading'|'failed';file?:File;key:string;preview:string;error:string;retryable:boolean;descriptionOpen:boolean}
type Moment = Row & {id:string;version:number;text:string;images:MomentImage[];public_text:string|null;public_images:MomentImage[];published:boolean;status:string}
const props = defineProps<{owner:boolean}>()
const uid = useId(), textarea = ref<HTMLTextAreaElement>(), fileInput = ref<HTMLInputElement>()
const busy = ref(false)
const scope = ref(props.owner ? 'site' : 'mine'), status = ref('all')
const error = ref(''), conflict = ref(false), latest = ref<Moment|null>(null)
const maxLength = ref(500), maxImages = ref(9), maxImageBytes = ref(10*1024*1024)
const form = reactive<Moment>({id:'',version:0,text:'',images:[],public_text:null,public_images:[],published:false,status:'draft'})
const images = ref<SelectedImage[]>([])
const imageInputs = (value:Array<MomentImage|SelectedImage>):ImageInput[] => value.map(image => ({media_id:image.media_id,alt:image.alt}))
const snapshot = (value:MomentPayload) => JSON.stringify(value)
const currentPayload = ():MomentPayload => ({text:form.text,images:imageInputs(images.value)})
const initial = ref(snapshot(currentPayload()))
const pendingImages = computed(() => images.value.some(image => image.state !== 'ready'))
const uploading = computed(() => images.value.some(image => image.state === 'queued' || image.state === 'uploading'))
const locked = computed(() => busy.value || uploading.value)
const dirty = computed(() => pendingImages.value || snapshot(currentPayload()) !== initial.value)
const length = computed(() => Array.from(form.text).length)
const deleted = computed(() => form.status === 'deleted' || !!form.deleted_at)
const valid = computed(() => (!!form.text.trim() || images.value.length > 0) && length.value <= maxLength.value && images.value.length <= maxImages.value && !pendingImages.value && images.value.every(image => Array.from(image.alt).length <= 200))
const publishChanged = computed(() => !form.published || snapshot(currentPayload()) !== snapshot({text:form.public_text || '',images:imageInputs(form.public_images)}))
const statusOptions = [{value:'all',label:t("全部状态")},{value:'draft',label:t("草稿")},{value:'published',label:t("已发布")},{value:'deleted',label:t("已删除")}]
let stopped = false
let createKey = randomID(), pendingCreate:{key:string;payload:MomentPayload}|null = null


function parseImages(value:unknown):MomentImage[] {
  if (!Array.isArray(value) || value.some(image => !image || typeof image.media_id !== 'string' || !image.media_id || typeof image.url !== 'string' || typeof image.alt !== 'string')) {
    throw new Error(t("服务器图片数据不完整，当前内容已保留。"))
  }
  return value.map(image => ({media_id:image.media_id,url:image.url,alt:image.alt}))
}
function selectedImage(image:MomentImage):SelectedImage {
  return {...image,localID:randomID(),state:'ready',key:'',preview:'',error:'',retryable:false,descriptionOpen:false}
}
const releasePreview=releaseUploadPreview
function clearImages() { images.value.forEach(releasePreview); images.value = [] }
function fileProblem(file:File) {
  const issue=imageFileIssue(file,maxImageBytes.value)
  if (issue==='type') return t("请选择 JPEG、PNG、WebP 或 GIF 图片。")
  if (issue==='size') return t("图片超过 {value1} 上限，请压缩后重新选择。", {value1:imageLimitLabel.value})
  if (issue==='empty') return t("图片文件为空，请重新选择。")
  return ''
}
const imageLimitLabel = computed(() => `${Number((maxImageBytes.value / 1048576).toFixed(1))} MiB`)
const {run:processUploads}=useMediaUploadQueue<SelectedImage>({
  next:()=>images.value.find(image=>image.state==='queued'),
  file:image=>image.file,
  key:image=>image.key,
  async uploaded(image,result){
    if (images.value.some(item=>item!==image&&item.state==='ready'&&item.media_id===result.media_id)) {
      image.retryable=false
      throw new Error(t("这张图片已经添加，请移除重复项。"))
    }
    image.media_id=result.media_id
    image.url=result.private_preview_url||'/media/'+result.media_id
    image.state='ready';image.file=undefined
    await nextTick()
    releasePreview(image)
  },
})
function pickImages(event:Event) {
  const input = event.target as HTMLInputElement, files = Array.from(input.files || [])
  input.value = ''
  if (locked.value || deleted.value || !files.length) return
  error.value = ''
  if (images.value.length + files.length > maxImages.value) {
    error.value = t("最多添加 {value1} 张图片，当前还可添加 {value2} 张。本次所选文件未添加，请重新选择。", {value1:maxImages.value,value2:Math.max(0,maxImages.value-images.value.length)})
    return
  }
  for (const file of files) {
    const problem = fileProblem(file)
    images.value.push({media_id:'',url:'',alt:'',localID:randomID(),state:problem?'failed':'queued',file,key:randomID(),preview:problem?'':URL.createObjectURL(file),error:problem,retryable:!problem,descriptionOpen:false})
  }
  void processUploads()
}
function retryImage(image:SelectedImage) {
  if (locked.value || !image.file || image.state !== 'failed' || !image.retryable) return
  image.error = fileProblem(image.file)
  if (image.error) { image.retryable = false; return }
  image.state = 'queued'
  void processUploads()
}
async function removeImage(index:number) {
  const image = images.value[index]
  if (busy.value || !image || image.state === 'uploading') return
  releasePreview(image)
  images.value.splice(index,1)
  await nextTick()
  document.getElementById(images.value[Math.min(index,images.value.length-1)] ? uid + '-image-' + images.value[Math.min(index,images.value.length-1)].localID : uid + '-image-add')?.focus()
}
async function moveImage(index:number,step:number) {
  const target = index + step
  if (locked.value || deleted.value || target < 0 || target >= images.value.length) return
  const image = images.value[index]
  images.value.splice(index,1)
  images.value.splice(target,0,image)
  await nextTick()
  document.getElementById(uid + '-image-' + image.localID)?.focus()
}
function toggleDescription(image:SelectedImage,event:Event) {
  const details = event.target as HTMLDetailsElement
  if (Array.from(image.alt).length > 200 && !details.open) details.open = true
  image.descriptionOpen = details.open
}

function parseMoment(data:Row):Moment {
  if (!data || !String(data.id || '') || !Number.isSafeInteger(Number(data.version)) || Number(data.version) < 1 || typeof data.text !== 'string' || typeof data.published !== 'boolean') {
    throw new Error(t("服务器动态数据不完整，当前输入已保留。"))
  }
  return {...data,id:String(data.id),version:Number(data.version),images:parseImages(data.images),public_images:parseImages(data.public_images),public_text:typeof data.public_text === 'string' ? data.public_text : null,status:data.deleted_at ? 'deleted' : data.status || (data.published ? 'published' : 'draft')} as Moment
}
function fill(row?:Moment, preserveInput = false) {
  const input = form.text, inputImages = images.value
  if (!preserveInput) clearImages()
  Object.keys(form).forEach(key => delete form[key])
  Object.assign(form, row || {id:'',version:0,text:'',images:[],public_text:null,public_images:[],published:false,status:'draft'})
  initial.value = snapshot({text:form.text,images:imageInputs(form.images)})
  images.value = preserveInput ? inputImages : form.images.map(selectedImage)
  if (preserveInput) form.text = input
  conflict.value = false
  latest.value = null
  if (!row) { createKey = randomID(); pendingCreate = null }
}
function timestamp(value:unknown) {
  if (!value) return '—'
  const date = new Date(String(value))
  return Number.isNaN(date.valueOf()) ? '—' : formatDate(date,{year:'numeric',month:'2-digit',day:'2-digit',hour:'2-digit',minute:'2-digit',hour12:false})
}
function author(row:Moment) { return row.author?.display_name || row.author?.username || t("作者") }
function rowStatus(row:Moment) { return row.status === 'deleted' || row.deleted_at ? t("已删除") : row.published ? t("已发布") : t("草稿") }
function mutationError(cause:unknown) {
  if (form.id && cause instanceof APIError && [409,412].includes(cause.status)) {
    conflict.value = true
    latest.value = null
    error.value = t("这条动态已在其他位置更新。你的输入已保留，请先读取最新版本。")
  } else if(cause instanceof APIError&&cause.code==='VALIDATION_FAILED')error.value=cause.message
  else toast.error(cause instanceof Error ? cause.message : t("操作失败，请重试。"))
}
const {items:rows,loading,error:listError,cursor:nextCursor,load} = useCursorList<Moment>({
  request:(cursor,signal)=>moments.list(new URLSearchParams({scope:scope.value,status:status.value,limit:'20'}),cursor,{signal}),
  disabled:()=>locked.value,
  parse:parseMoment,
  received(data) {
    const limit = Number(data.max_text_length || data.max_length)
    if (Number.isSafeInteger(limit) && limit > 0) maxLength.value = limit
    const imageCount = Number(data.max_images), imageBytes = Number(data.max_image_bytes)
    if (Number.isSafeInteger(imageCount) && imageCount > 0) maxImages.value = imageCount
    if (Number.isSafeInteger(imageBytes) && imageBytes > 0) maxImageBytes.value = imageBytes
  },
})
function filter() { void load() }
function changeScope(value:string) { if (locked.value || loading.value) return; scope.value = value; filter() }
function changeStatus(value:string|number) { if (locked.value || loading.value) return; status.value = String(value); filter() }

async function edit(row?:Moment) {
  if (locked.value) return
  if (row && row.id === form.id) { textarea.value?.focus(); return }
  busy.value = true
  try {
    if (dirty.value && !await confirmAction(t("放弃未保存的动态？"),t("切换前，请确认当前文字和图片已经保存。"),t("放弃修改"))) return
    const result = row ? parseMoment(await moments.read(row.id)) : undefined
    if (stopped) return
    fill(result)
    error.value = ''
    await nextTick()
    textarea.value?.focus()
  } catch (cause) { error.value = (cause as Error).message }
  finally { busy.value = false }
}
async function save() {
  if (locked.value || conflict.value || deleted.value || !valid.value || (form.id && !dirty.value)) return
  busy.value = true; error.value = ''
  try {
    let result:Moment
    if (form.id) {
      result = parseMoment(await moments.saveDraft(form.id,currentPayload(),{version:form.version}))
    } else {
      // An uncertain create is retried with its original key and payload, even if
      // the user has continued typing. Once acknowledged, newer input stays dirty.
      pendingCreate ||= {key:createKey,payload:currentPayload()}
      result = parseMoment(await moments.create(pendingCreate.payload,pendingCreate.key))
      pendingCreate = null
    }
    if (stopped) return
    const keepInput = snapshot({text:result.text,images:imageInputs(result.images)}) !== snapshot(currentPayload())
    fill(result,keepInput)
    if(keepInput)toast.warning(t("已确认保存的草稿。当前新修改仍未保存，请再次保存。"));else toast.success(result.published?t("草稿已保存，公开内容保持原样。发布更新后才会替换公开内容。"):t("草稿已保存，发布后才会公开。"))
    await load()
  } catch (cause) { if(!form.id&&cause instanceof APIError&&[400,413,422].includes(cause.status)){pendingCreate=null;createKey=randomID()} mutationError(cause) }
  finally { busy.value = false }
}
async function publish() {
  if (locked.value || !form.id || dirty.value || conflict.value || deleted.value || !valid.value || !publishChanged.value) return
  busy.value = true; error.value = ''
  try {
    const result = parseMoment(await moments.publish(form.id,{version:form.version}))
    if (stopped) return
    fill(result); toast.success(t("动态已发布。"))
    await load()
  } catch (cause) { mutationError(cause) }
  finally { busy.value = false }
}
async function withdraw() {
  if (locked.value || !form.id || !form.published || dirty.value || conflict.value || deleted.value) return
  busy.value = true
  try {
    if (!await confirmAction(t("撤回这条动态？"),t("公开内容将被撤下，已保存的草稿会保留。"),t("撤回动态"))) return
    error.value = ''
    const result = parseMoment(await moments.withdraw(form.id,{version:form.version}))
    if (stopped) return
    fill(result); toast.success(t("动态已撤回，草稿已保留。"))
    await load()
  } catch (cause) { mutationError(cause) }
  finally { busy.value = false }
}
async function remove(row:Moment) {
  if (locked.value || row.status === 'deleted' || row.deleted_at) return
  busy.value = true
  try {
    const selected = form.id === row.id
    if (!await confirmAction(t("删除这条动态？"),selected && dirty.value ? t("当前未保存的文字、图片顺序和说明也会放弃；公开内容将被移除。此操作不能撤销。") : t("动态会从公开页面移除，此操作不能撤销。"),t("删除动态"))) return
    error.value = ''
    await moments.remove(row.id,{version:selected ? form.version : row.version})
    if (stopped) return
    if (selected) fill()
    rows.value = rows.value.filter(item => item.id !== row.id)
    toast.success(t("动态已删除。"))
    await load()
  } catch (cause) {
    if (form.id === row.id) mutationError(cause)
    else toast.error(t("{value1} 请刷新列表后重试。", {value1:(cause as Error).message}))
  } finally { busy.value = false }
}
async function readLatest() {
  if (locked.value || !form.id) return
  busy.value = true; error.value = ''
  try {
    const result = parseMoment(await moments.read(form.id))
    if (!stopped) latest.value = result
  } catch (cause) { error.value = t("无法读取最新版本，当前输入已保留。{value1}", {value1:(cause as Error).message}) }
  finally { busy.value = false }
}
function keepInput() {
  if (locked.value || !latest.value || latest.value.status === 'deleted' || latest.value.deleted_at) return
  fill(latest.value,true)
  error.value = ''; toast.info(t("已采用最新版本号，文字和图片已保留。检查内容后再次保存草稿。"))
}
async function useLatest() {
  if (locked.value || !latest.value) return
  busy.value = true
  try {
    if (dirty.value && !await confirmAction(t("使用服务器最新内容？"),t("当前文字、图片顺序和说明将被最新草稿替换。"),t("使用最新内容"))) return
    fill(latest.value); error.value = ''; toast.info(t("已载入服务器最新内容。"))
  } finally { busy.value = false }
}
function beforeLeave(event:BeforeUnloadEvent) { if (dirty.value || uploading.value) { event.preventDefault(); event.returnValue = '' } }
onMounted(() => { void load(); if(new URLSearchParams(location.search).get('compose')==='1')void nextTick(()=>textarea.value?.focus()); window.addEventListener('beforeunload',beforeLeave) })
onBeforeUnmount(() => { stopped = true; clearImages(); window.removeEventListener('beforeunload',beforeLeave) })
</script>

<template>

 <div class="moment-management">
  <form id="moment-compose" class="moment-composer" :aria-label="t('动态编辑')" @submit.prevent="save">
   <div class="moment-composer-heading"><h2>{{deleted?t("查看动态"):form.id?t("编辑动态"):t("写一条动态")}}</h2><button v-if="form.id||dirty" type="button" class="quiet" :disabled="locked" @click="edit()">{{t("新建动态")}}</button></div>
   <p class="moment-composer-state" role="status">{{deleted?t("这条动态已删除，仅供查看。"):dirty?t("有未保存的修改"):form.id?(form.published?t("已保存 · 已发布"):t("已保存 · 草稿")):t("先保存草稿，再发布。")}}</p>
   <div v-if="error" class="moment-feedback is-error" role="alert">{{error}}</div>
   <label :for="uid+'-text'">{{t("动态内容")}}</label>
   <textarea :id="uid+'-text'" ref="textarea" v-model="form.text" rows="9" :disabled="locked" :readonly="deleted" :aria-describedby="uid+'-length'" :aria-invalid="length>maxLength||undefined" :placeholder="t('此刻，有什么想记下来？')"></textarea>
   <div :id="uid+'-length'" class="moment-length"><span>{{length>maxLength?t("内容超出上限，请删减后保存。"):t("支持换行，也可以只发图片。")}}</span><span :class="{danger:length>maxLength}">{{t("{value1} / {value2} 字符", {value1:(length),value2:(maxLength)})}}</span></div>
   <fieldset class="moment-image-editor">
    <legend>{{t("动态图片")}} <span>{{images.length}} / {{maxImages}}</span></legend>
    <div v-if="!deleted" class="moment-image-upload"><button :id="uid+'-image-add'" type="button" :disabled="locked||images.length>=maxImages" @click="fileInput?.click()"><Icon name="image" :size="16"/>{{t("添加图片")}}</button><input ref="fileInput" class="sr-only" tabindex="-1" type="file" multiple accept="image/jpeg,image/png,image/webp,image/gif" :disabled="locked||images.length>=maxImages" :aria-label="t('选择动态图片')" @change="pickImages"><p>{{t("JPEG、PNG、WebP 或 GIF，每张最多 {value1}。最多 {value2} 张，按当前顺序展示。", {value1:(imageLimitLabel),value2:(maxImages)})}}</p></div>
    <ol v-if="images.length" class="moment-image-list">
     <li v-for="(image,index) in images" :id="uid+'-image-'+image.localID" :key="image.localID" class="moment-image-item" tabindex="-1" :data-upload-state="image.state">
      <div class="moment-image-item-top"><div class="moment-image-thumb"><img v-if="image.url||image.preview" :src="image.url||image.preview" :alt="image.alt||t('第 {value1} 张图片预览', {value1:index+1})" loading="lazy"><Icon v-else name="image" :size="24"/></div><div class="moment-image-file"><strong>{{t("图片 {value1}", {value1:(index+1)})}}</strong><span v-if="image.file" class="moment-image-filename">{{image.file.name}}</span><span class="moment-image-upload-state" role="status">{{image.state==='queued'?t("等待上传"):image.state==='uploading'?t("正在上传…"):image.state==='failed'?t("上传失败"):deleted?t("已保存的图片"):t("图片已就绪")}}</span><progress v-if="image.state==='uploading'" :aria-label="t('正在上传第 {value1} 张图片', {value1:index+1})"></progress></div></div>
      <div v-if="image.state==='failed'" class="moment-image-error" role="alert"><p>{{t("{value1} 文件仍保留在当前页面。", {value1:(image.error)})}}</p><button v-if="image.retryable" type="button" :aria-label="t('重试上传第 {value1} 张图片', {value1:index+1})" :disabled="locked" @click="retryImage(image)">{{t("重试上传")}}</button></div>
      <details class="moment-image-description" :open="image.descriptionOpen||Array.from(image.alt).length>200" @toggle="toggleDescription(image,$event)"><summary><span>{{image.alt||(deleted?t("没有图片说明"):t("添加图片说明（选填）"))}}</span><small :class="{danger:Array.from(image.alt).length>200}">{{Array.from(image.alt).length}} / 200</small><Icon name="chevron" :size="16"/></summary><label :for="uid+'-alt-'+image.localID">{{t("图片 {value1} 说明", {value1:(index+1)})}} <span>{{t("选填")}}</span></label><textarea :id="uid+'-alt-'+image.localID" v-model="image.alt" rows="2" :disabled="locked" :readonly="deleted" :aria-describedby="uid+'-alt-count-'+image.localID" :aria-invalid="Array.from(image.alt).length>200||undefined" :placeholder="t('描述图片中的内容，帮助读者理解')"></textarea><p :id="uid+'-alt-count-'+image.localID" class="moment-image-alt-count" :class="{danger:Array.from(image.alt).length>200}">{{t("{value1} / 200 字符{value2}", {value1:(Array.from(image.alt).length),value2:(Array.from(image.alt).length>200?t("，请删减后保存。"):'')})}}</p></details>
      <div v-if="!deleted" class="moment-image-actions"><button type="button" :aria-label="t('将第 {value1} 张图片前移', {value1:index+1})" :disabled="locked||index===0" @click="moveImage(index,-1)">{{t("前移")}}</button><button type="button" :aria-label="t('将第 {value1} 张图片后移', {value1:index+1})" :disabled="locked||index===images.length-1" @click="moveImage(index,1)">{{t("后移")}}</button><button type="button" class="quiet danger" :aria-label="image.state==='queued'||image.state==='failed'?t('取消第 {value1} 张图片上传', {value1:index+1}):t('移除第 {value1} 张图片', {value1:index+1})" :disabled="busy||image.state==='uploading'" @click="removeImage(index)">{{image.state==='queued'||image.state==='failed'?t("取消上传"):t("移除")}}</button></div>
     </li>
    </ol>
    <p v-else-if="deleted" class="moment-image-hint">{{t("这条动态没有图片。")}}</p>
    <p v-if="!deleted" class="moment-image-hint">{{t("{value1} 移除图片只取消这条动态的引用，不删除媒体文件。", {value1:(uploading?t("图片正在上传，请等待完成后保存。"):pendingImages?t("有图片未上传成功，请重试或取消后再保存。"):t("图片上传后仍需保存草稿；发布后读者才可见。"))})}}</p>
   </fieldset>
   <section v-if="conflict" class="moment-conflict" :aria-label="t('版本冲突')">
    <h3>{{t("当前输入已保留")}}</h3><p>{{t("先读取服务器的最新草稿，再决定如何继续。")}}</p>
    <button v-if="!latest" type="button" :disabled="locked" @click="readLatest">{{t("读取最新版本")}}</button>
    <template v-else><p>{{latest.status==='deleted'?t("这条动态已删除。请先保留需要的内容，再新建动态。"):t("服务器最新草稿：")}}</p><pre class="moment-conflict-text">{{latest.text||t("（没有文字）")}}</pre><ol v-if="latest.images.length" class="moment-conflict-images" tabindex="0" :aria-label="t('服务器最新草稿图片及说明')"><li v-for="(image,index) in latest.images" :key="image.media_id"><img :src="image.url" :alt="image.alt||t('服务器第 {value1} 张图片', {value1:index+1})" loading="lazy"><p>{{index+1}} · {{image.alt||t("没有图片说明")}}</p></li></ol><p v-else>{{t("服务器草稿没有图片。")}}</p><div class="moment-conflict-actions"><button v-if="latest.status!=='deleted'&&!latest.deleted_at" type="button" :disabled="locked" @click="keepInput">{{t("保留我的输入，继续编辑")}}</button><button type="button" class="quiet" :disabled="locked" @click="useLatest">{{t("使用服务器内容")}}</button></div></template>
   </section>
   <div v-if="!deleted" class="moment-composer-actions"><button type="submit" :disabled="locked||conflict||!valid||(!!form.id&&!dirty)">{{locked?t("正在处理…"):t("保存草稿")}}</button><button type="button" class="primary" :disabled="locked||!form.id||dirty||conflict||!valid||!publishChanged" @click="publish">{{form.published?(publishChanged?t("发布更新"):t("已发布")):t("发布动态")}}</button></div>
   <p v-if="!deleted" class="moment-composer-help">{{form.published?t("保存只会更新草稿，公开内容要在发布更新后才会改变。"):t("保存草稿后，点击发布动态，读者才能看到。")}}{{dirty&&form.id?t(" 请先保存当前修改，再发布或撤回。"):''}}</p>
   <div v-if="form.id&&!deleted" class="moment-composer-secondary"><button v-if="form.published" type="button" class="quiet" :disabled="locked||dirty||conflict" @click="withdraw">{{t("撤回动态")}}</button><button type="button" class="quiet danger" :disabled="locked" @click="remove(form)">{{t("删除动态")}}</button></div>
  </form>

  <section class="moment-library" :aria-label="t('动态列表')">
   <div class="moment-library-tools">
    <div v-if="owner" class="moment-scope" role="group" :aria-label="t('动态范围')"><button type="button" :aria-pressed="scope==='site'" :disabled="locked||loading" @click="changeScope('site')">{{t("全站")}}</button><button type="button" :aria-pressed="scope==='mine'" :disabled="locked||loading" @click="changeScope('mine')">{{t("我的")}}</button></div>
    <SelectField :model-value="status" :label="t('动态状态')" :options="statusOptions" :disabled="locked||loading" @update:model-value="changeStatus"/>
   </div>
   <div v-if="listError" class="moment-feedback is-error" role="alert">{{listError}}<br><button type="button" :disabled="loading||locked" @click="load()">{{t("重新加载列表")}}</button></div>
   <p v-if="loading" class="moment-list-loading" role="status">{{t("正在载入动态…")}}</p>
   <div class="moment-list" :aria-busy="loading">
    <article v-for="row in rows" :key="row.id" class="moment-list-row" :class="{'is-editing':form.id===row.id}">
     <div class="moment-row-meta"><strong>{{author(row)}}</strong><span class="moment-row-status" :class="{'is-published':row.published}">{{rowStatus(row)}}</span><time :datetime="row.updated_at">{{timestamp(row.updated_at)}}</time></div>
     <p v-if="row.text" class="moment-row-excerpt">{{row.text}}</p>
     <div v-if="row.images.length" class="moment-manager-thumbs" :aria-label="t('{value1} 张草稿图片', {value1:row.images.length})"><img v-for="(image,index) in row.images" :key="image.media_id" :src="image.url" :alt="image.alt||t('第 {value1} 张图片', {value1:index+1})" width="72" height="72" loading="lazy"></div>
     <p v-if="row.published&&row.has_unpublished_changes" class="moment-row-note">{{t("有未发布的修改")}}</p>
     <div class="moment-row-actions"><button type="button" :disabled="locked" :aria-current="form.id===row.id?'true':undefined" @click="edit(row)"><Icon :name="row.status==='deleted'?'posts':'pen'" :size="16"/>{{row.status==='deleted'?t("查看"):form.id===row.id?t("正在编辑"):t("编辑")}}</button><button v-if="row.status!=='deleted'&&!row.deleted_at" type="button" class="quiet danger" :disabled="locked||loading" @click="remove(row)">{{t("删除")}}</button></div>
    </article>
   </div>
   <div v-if="!loading&&!listError&&!rows.length" class="moment-library-empty"><h2>{{status==='deleted'?t("没有已删除的动态"):status==='draft'?t("没有草稿"):status==='published'?t("还没有公开动态"):t("这里还没有动态")}}</h2><p>{{status==='deleted'?t("删除的动态会显示在这里，仅供查看。"):t("写下此刻的想法，保存后再决定何时公开。")}}</p></div>
   <InfiniteList :has-more="!!nextCursor" :loading="loading" :disabled="locked" :error="listError" @load="load(true)"/><p class="manager-caption">{{t("已显示 {value1} 条动态", {value1:(rows.length)})}}</p>
  </section>
 </div>
</template>
