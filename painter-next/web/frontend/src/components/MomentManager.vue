<script setup lang="ts">
import {computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, useId} from 'vue'
import {api, APIError, body, list, type Row} from '../api'
import {confirmAction} from '../confirm'
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
const rows = ref<Moment[]>([]), loading = ref(true), busy = ref(false)
const scope = ref(props.owner ? 'site' : 'mine'), status = ref('all')
const listError = ref(''), error = ref(''), notice = ref(''), conflict = ref(false), latest = ref<Moment|null>(null)
const maxLength = ref(500), maxImages = ref(9), maxImageBytes = ref(10*1024*1024), page = ref(0), cursors = ref(['']), nextCursor = ref('')
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
const statusOptions = [{value:'all',label:'全部状态'},{value:'draft',label:'草稿'},{value:'published',label:'已发布'},{value:'deleted',label:'已删除'}]
let sequence = 0, controller:AbortController|undefined, stopped = false
let createKey = randomID(), pendingCreate:{key:string;payload:MomentPayload}|null = null
let uploadRunning = false, uploadController:AbortController|undefined

function parseImages(value:unknown):MomentImage[] {
  if (!Array.isArray(value) || value.some(image => !image || typeof image.media_id !== 'string' || !image.media_id || typeof image.url !== 'string' || typeof image.alt !== 'string')) {
    throw new Error('服务器图片数据不完整，当前内容已保留。')
  }
  return value.map(image => ({media_id:image.media_id,url:image.url,alt:image.alt}))
}
function selectedImage(image:MomentImage):SelectedImage {
  return {...image,localID:randomID(),state:'ready',key:'',preview:'',error:'',retryable:false,descriptionOpen:false}
}
function releasePreview(image:SelectedImage) {
  if (image.preview) { URL.revokeObjectURL(image.preview); image.preview = '' }
}
function clearImages() { images.value.forEach(releasePreview); images.value = [] }
function fileProblem(file:File) {
  if (!['image/jpeg','image/png','image/webp','image/gif'].includes(file.type)) return '请选择 JPEG、PNG、WebP 或 GIF 图片。'
  if (file.size > maxImageBytes.value) return `图片超过 ${imageLimitLabel.value} 上限，请压缩后重新选择。`
  if (!file.size) return '图片文件为空，请重新选择。'
  return ''
}
const imageLimitLabel = computed(() => `${Number((maxImageBytes.value / 1048576).toFixed(1))} MiB`)
async function processUploads() {
  if (uploadRunning || stopped) return
  uploadRunning = true
  try {
    while (!stopped) {
      const image = images.value.find(item => item.state === 'queued')
      if (!image || !image.file) break
      image.state = 'uploading'; image.error = ''
      uploadController = new AbortController()
      try {
        const data = new FormData()
        data.append('file',image.file,image.file.name)
        const result = await api('/media',{method:'POST',body:data,key:image.key,signal:uploadController.signal})
        if (stopped) return
        if (result.state !== 'ready' || typeof result.media_id !== 'string' || !result.media_id) throw new Error('服务器尚未确认图片可用，请重试。')
        if (images.value.some(item => item !== image && item.state === 'ready' && item.media_id === result.media_id)) {
          image.retryable = false
          throw new Error('这张图片已经添加，请移除重复项。')
        }
        image.media_id = result.media_id
        image.url = result.private_preview_url || '/media/' + result.media_id
        image.state = 'ready'
        image.file = undefined
        await nextTick()
        releasePreview(image)
      } catch (cause) {
        if (stopped) return
        image.state = 'failed'
        image.error = (cause as Error).message || '网络未确认上传结果，请重试。'
      } finally { uploadController = undefined }
    }
  } finally { uploadRunning = false }
}
function pickImages(event:Event) {
  const input = event.target as HTMLInputElement, files = Array.from(input.files || [])
  input.value = ''
  if (locked.value || deleted.value || !files.length) return
  error.value = ''; notice.value = ''
  if (images.value.length + files.length > maxImages.value) {
    error.value = `最多添加 ${maxImages.value} 张图片，当前还可添加 ${Math.max(0,maxImages.value-images.value.length)} 张。本次所选文件未添加，请重新选择。`
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
    throw new Error('服务器动态数据不完整，当前输入已保留。')
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
  return Number.isNaN(date.valueOf()) ? '—' : date.toLocaleString('zh-CN',{year:'numeric',month:'2-digit',day:'2-digit',hour:'2-digit',minute:'2-digit',hour12:false})
}
function author(row:Moment) { return row.author?.display_name || row.author?.username || '作者' }
function rowStatus(row:Moment) { return row.status === 'deleted' || row.deleted_at ? '已删除' : row.published ? '已发布' : '草稿' }
function mutationError(cause:unknown) {
  error.value = cause instanceof Error ? cause.message : '操作失败，请重试。'
  if (form.id && cause instanceof APIError && [409,412].includes(cause.status)) {
    conflict.value = true
    latest.value = null
    error.value = '这条动态已在其他位置更新。你的输入已保留，请先读取最新版本。'
  }
}
async function load() {
  const at = ++sequence
  controller?.abort()
  controller = new AbortController()
  loading.value = true
  listError.value = ''
  rows.value = []
  nextCursor.value = ''
  try {
    const query = new URLSearchParams({scope:scope.value,status:status.value,limit:'20'})
    if (cursors.value[page.value]) query.set('cursor',cursors.value[page.value])
    const data = await api('/me/moments?' + query,{signal:controller.signal})
    if (stopped || at !== sequence) return
    rows.value = list(data).map(parseMoment)
    nextCursor.value = data.has_more === false ? '' : data.next_cursor || ''
    const limit = Number(data.max_text_length || data.max_length)
    if (Number.isSafeInteger(limit) && limit > 0) maxLength.value = limit
    const imageCount = Number(data.max_images), imageBytes = Number(data.max_image_bytes)
    if (Number.isSafeInteger(imageCount) && imageCount > 0) maxImages.value = imageCount
    if (Number.isSafeInteger(imageBytes) && imageBytes > 0) maxImageBytes.value = imageBytes
  } catch (cause) {
    if (at === sequence && (cause as Error).name !== 'AbortError') listError.value = (cause as Error).message
  } finally { if (!stopped && at === sequence) loading.value = false }
}
function filter() { page.value = 0; cursors.value = ['']; void load() }
function changeScope(value:string) { if (locked.value || loading.value) return; scope.value = value; filter() }
function changeStatus(value:string|number) { if (locked.value || loading.value) return; status.value = String(value); filter() }
function paginate(step:number) {
  if (loading.value || locked.value || (step < 0 && !page.value) || (step > 0 && !nextCursor.value)) return
  if (step > 0) cursors.value[page.value + 1] = nextCursor.value
  page.value += step
  void load()
}
async function edit(row?:Moment) {
  if (locked.value) return
  if (row && row.id === form.id) { textarea.value?.focus(); return }
  busy.value = true
  try {
    if (dirty.value && !await confirmAction('放弃未保存的动态？','切换前，请确认当前文字和图片已经保存。','放弃修改')) return
    const result = row ? parseMoment(await api('/studio/moments/' + encodeURIComponent(row.id))) : undefined
    if (stopped) return
    fill(result)
    error.value = ''; notice.value = ''
    await nextTick()
    textarea.value?.focus()
  } catch (cause) { error.value = (cause as Error).message }
  finally { busy.value = false }
}
async function save() {
  if (locked.value || conflict.value || deleted.value || !valid.value || (form.id && !dirty.value)) return
  busy.value = true; error.value = ''; notice.value = ''
  try {
    let result:Moment
    if (form.id) {
      result = parseMoment(await api('/studio/moments/' + encodeURIComponent(form.id) + '/draft',{method:'PUT',version:form.version,body:body(currentPayload())}))
    } else {
      // An uncertain create is retried with its original key and payload, even if
      // the user has continued typing. Once acknowledged, newer input stays dirty.
      pendingCreate ||= {key:createKey,payload:currentPayload()}
      result = parseMoment(await api('/moments',{method:'POST',key:pendingCreate.key,body:body(pendingCreate.payload)}))
      pendingCreate = null
    }
    if (stopped) return
    const keepInput = snapshot({text:result.text,images:imageInputs(result.images)}) !== snapshot(currentPayload())
    fill(result,keepInput)
    notice.value = keepInput ? '已确认保存的草稿。当前新修改仍未保存，请再次保存。' : result.published ? '草稿已保存，公开内容保持原样。发布更新后才会替换公开内容。' : '草稿已保存，发布后才会公开。'
    await load()
  } catch (cause) { if(!form.id&&cause instanceof APIError&&[400,413,422].includes(cause.status)){pendingCreate=null;createKey=randomID()} mutationError(cause) }
  finally { busy.value = false }
}
async function publish() {
  if (locked.value || !form.id || dirty.value || conflict.value || deleted.value || !valid.value || !publishChanged.value) return
  busy.value = true; error.value = ''; notice.value = ''
  try {
    const result = parseMoment(await api('/studio/moments/' + encodeURIComponent(form.id) + '/publish',{method:'POST',version:form.version}))
    if (stopped) return
    fill(result); notice.value = '动态已发布。'
    await load()
  } catch (cause) { mutationError(cause) }
  finally { busy.value = false }
}
async function withdraw() {
  if (locked.value || !form.id || !form.published || dirty.value || conflict.value || deleted.value) return
  busy.value = true
  try {
    if (!await confirmAction('撤回这条动态？','公开内容将被撤下，已保存的草稿会保留。','撤回动态')) return
    error.value = ''; notice.value = ''
    const result = parseMoment(await api('/studio/moments/' + encodeURIComponent(form.id) + '/publication',{method:'DELETE',version:form.version}))
    if (stopped) return
    fill(result); notice.value = '动态已撤回，草稿已保留。'
    await load()
  } catch (cause) { mutationError(cause) }
  finally { busy.value = false }
}
async function remove(row:Moment) {
  if (locked.value || row.status === 'deleted' || row.deleted_at) return
  busy.value = true
  try {
    const selected = form.id === row.id
    if (!await confirmAction('删除这条动态？',selected && dirty.value ? '当前未保存的文字、图片顺序和说明也会放弃；公开内容将被移除。此操作不能撤销。' : '动态会从公开页面移除，此操作不能撤销。','删除动态')) return
    error.value = ''; notice.value = ''
    await api('/studio/moments/' + encodeURIComponent(row.id),{method:'DELETE',version:selected ? form.version : row.version})
    if (stopped) return
    if (selected) fill()
    rows.value = rows.value.filter(item => item.id !== row.id)
    notice.value = '动态已删除。'
    await load()
  } catch (cause) {
    if (form.id === row.id) mutationError(cause)
    else error.value = (cause as Error).message + ' 请刷新列表后重试。'
  } finally { busy.value = false }
}
async function readLatest() {
  if (locked.value || !form.id) return
  busy.value = true; error.value = ''
  try {
    const result = parseMoment(await api('/studio/moments/' + encodeURIComponent(form.id)))
    if (!stopped) latest.value = result
  } catch (cause) { error.value = '无法读取最新版本，当前输入已保留。' + (cause as Error).message }
  finally { busy.value = false }
}
function keepInput() {
  if (locked.value || !latest.value || latest.value.status === 'deleted' || latest.value.deleted_at) return
  fill(latest.value,true)
  error.value = ''; notice.value = '已采用最新版本号，文字和图片已保留。检查内容后再次保存草稿。'
}
async function useLatest() {
  if (locked.value || !latest.value) return
  busy.value = true
  try {
    if (dirty.value && !await confirmAction('使用服务器最新内容？','当前文字、图片顺序和说明将被最新草稿替换。','使用最新内容')) return
    fill(latest.value); error.value = ''; notice.value = '已载入服务器最新内容。'
  } finally { busy.value = false }
}
function beforeLeave(event:BeforeUnloadEvent) { if (dirty.value || uploading.value) { event.preventDefault(); event.returnValue = '' } }
onMounted(() => { void load(); if(new URLSearchParams(location.search).get('compose')==='1')void nextTick(()=>textarea.value?.focus()); window.addEventListener('beforeunload',beforeLeave) })
onBeforeUnmount(() => { stopped = true; sequence++; controller?.abort(); uploadController?.abort(); clearImages(); window.removeEventListener('beforeunload',beforeLeave) })
</script>

<template>
 <div v-if="notice" class="moment-feedback" role="status">{{notice}}</div>
 <div class="moment-management">
  <form id="moment-compose" class="moment-composer" aria-label="动态编辑" @submit.prevent="save">
   <div class="moment-composer-heading"><h2>{{deleted?'查看动态':form.id?'编辑动态':'写一条动态'}}</h2><button v-if="form.id||dirty" type="button" class="quiet" :disabled="locked" @click="edit()">新建动态</button></div>
   <p class="moment-composer-state" role="status">{{deleted?'这条动态已删除，仅供查看。':dirty?'有未保存的修改':form.id?(form.published?'已保存 · 已发布':'已保存 · 草稿'):'先保存草稿，再发布。'}}</p>
   <div v-if="error" class="moment-feedback is-error" role="alert">{{error}}</div>
   <label :for="uid+'-text'">动态内容</label>
   <textarea :id="uid+'-text'" ref="textarea" v-model="form.text" rows="9" :disabled="locked" :readonly="deleted" :aria-describedby="uid+'-length'" :aria-invalid="length>maxLength||undefined" placeholder="此刻，有什么想记下来？"></textarea>
   <div :id="uid+'-length'" class="moment-length"><span>{{length>maxLength?'内容超出上限，请删减后保存。':'支持换行，也可以只发图片。'}}</span><span :class="{danger:length>maxLength}">{{length}} / {{maxLength}} 字符</span></div>
   <fieldset class="moment-image-editor">
    <legend>动态图片 <span>{{images.length}} / {{maxImages}}</span></legend>
    <div v-if="!deleted" class="moment-image-upload"><button :id="uid+'-image-add'" type="button" :disabled="locked||images.length>=maxImages" @click="fileInput?.click()"><Icon name="image" :size="16"/>添加图片</button><input ref="fileInput" class="sr-only" tabindex="-1" type="file" multiple accept="image/jpeg,image/png,image/webp,image/gif" :disabled="locked||images.length>=maxImages" aria-label="选择动态图片" @change="pickImages"><p>JPEG、PNG、WebP 或 GIF，每张最多 {{imageLimitLabel}}。最多 {{maxImages}} 张，按当前顺序展示。</p></div>
    <ol v-if="images.length" class="moment-image-list">
     <li v-for="(image,index) in images" :id="uid+'-image-'+image.localID" :key="image.localID" class="moment-image-item" tabindex="-1" :data-upload-state="image.state">
      <div class="moment-image-item-top"><div class="moment-image-thumb"><img v-if="image.url||image.preview" :src="image.url||image.preview" :alt="image.alt||`第 ${index+1} 张图片预览`" loading="lazy"><Icon v-else name="image" :size="24"/></div><div class="moment-image-file"><strong>图片 {{index+1}}</strong><span v-if="image.file" class="moment-image-filename">{{image.file.name}}</span><span class="moment-image-upload-state" role="status">{{image.state==='queued'?'等待上传':image.state==='uploading'?'正在上传…':image.state==='failed'?'上传失败':deleted?'已保存的图片':'图片已就绪'}}</span><progress v-if="image.state==='uploading'" :aria-label="`正在上传第 ${index+1} 张图片`"></progress></div></div>
      <div v-if="image.state==='failed'" class="moment-image-error" role="alert"><p>{{image.error}} 文件仍保留在当前页面。</p><button v-if="image.retryable" type="button" :aria-label="`重试上传第 ${index+1} 张图片`" :disabled="locked" @click="retryImage(image)">重试上传</button></div>
      <details class="moment-image-description" :open="image.descriptionOpen||Array.from(image.alt).length>200" @toggle="toggleDescription(image,$event)"><summary><span>{{image.alt||(deleted?'没有图片说明':'添加图片说明（选填）')}}</span><small :class="{danger:Array.from(image.alt).length>200}">{{Array.from(image.alt).length}} / 200</small><Icon name="chevron" :size="16"/></summary><label :for="uid+'-alt-'+image.localID">图片 {{index+1}} 说明 <span>选填</span></label><textarea :id="uid+'-alt-'+image.localID" v-model="image.alt" rows="2" :disabled="locked" :readonly="deleted" :aria-describedby="uid+'-alt-count-'+image.localID" :aria-invalid="Array.from(image.alt).length>200||undefined" placeholder="描述图片中的内容，帮助读者理解"></textarea><p :id="uid+'-alt-count-'+image.localID" class="moment-image-alt-count" :class="{danger:Array.from(image.alt).length>200}">{{Array.from(image.alt).length}} / 200 字符{{Array.from(image.alt).length>200?'，请删减后保存。':''}}</p></details>
      <div v-if="!deleted" class="moment-image-actions"><button type="button" :aria-label="`将第 ${index+1} 张图片前移`" :disabled="locked||index===0" @click="moveImage(index,-1)">前移</button><button type="button" :aria-label="`将第 ${index+1} 张图片后移`" :disabled="locked||index===images.length-1" @click="moveImage(index,1)">后移</button><button type="button" class="quiet danger" :aria-label="image.state==='queued'||image.state==='failed'?`取消第 ${index+1} 张图片上传`:`移除第 ${index+1} 张图片`" :disabled="busy||image.state==='uploading'" @click="removeImage(index)">{{image.state==='queued'||image.state==='failed'?'取消上传':'移除'}}</button></div>
     </li>
    </ol>
    <p v-else-if="deleted" class="moment-image-hint">这条动态没有图片。</p>
    <p v-if="!deleted" class="moment-image-hint">{{uploading?'图片正在上传，请等待完成后保存。':pendingImages?'有图片未上传成功，请重试或取消后再保存。':'图片上传后仍需保存草稿；发布后读者才可见。'}} 移除图片只取消这条动态的引用，不删除媒体文件。</p>
   </fieldset>
   <section v-if="conflict" class="moment-conflict" aria-label="版本冲突">
    <h3>当前输入已保留</h3><p>先读取服务器的最新草稿，再决定如何继续。</p>
    <button v-if="!latest" type="button" :disabled="locked" @click="readLatest">读取最新版本</button>
    <template v-else><p>{{latest.status==='deleted'?'这条动态已删除。请先保留需要的内容，再新建动态。':'服务器最新草稿：'}}</p><pre class="moment-conflict-text">{{latest.text||'（没有文字）'}}</pre><ol v-if="latest.images.length" class="moment-conflict-images" tabindex="0" aria-label="服务器最新草稿图片及说明"><li v-for="(image,index) in latest.images" :key="image.media_id"><img :src="image.url" :alt="image.alt||`服务器第 ${index+1} 张图片`" loading="lazy"><p>{{index+1}} · {{image.alt||'没有图片说明'}}</p></li></ol><p v-else>服务器草稿没有图片。</p><div class="moment-conflict-actions"><button v-if="latest.status!=='deleted'&&!latest.deleted_at" type="button" :disabled="locked" @click="keepInput">保留我的输入，继续编辑</button><button type="button" class="quiet" :disabled="locked" @click="useLatest">使用服务器内容</button></div></template>
   </section>
   <div v-if="!deleted" class="moment-composer-actions"><button type="submit" :disabled="locked||conflict||!valid||(!!form.id&&!dirty)">{{locked?'正在处理…':'保存草稿'}}</button><button type="button" class="primary" :disabled="locked||!form.id||dirty||conflict||!valid||!publishChanged" @click="publish">{{form.published?(publishChanged?'发布更新':'已发布'):'发布动态'}}</button></div>
   <p v-if="!deleted" class="moment-composer-help">{{form.published?'保存只会更新草稿，公开内容要在发布更新后才会改变。':'保存草稿后，点击发布动态，读者才能看到。'}}{{dirty&&form.id?' 请先保存当前修改，再发布或撤回。':''}}</p>
   <div v-if="form.id&&!deleted" class="moment-composer-secondary"><button v-if="form.published" type="button" class="quiet" :disabled="locked||dirty||conflict" @click="withdraw">撤回动态</button><button type="button" class="quiet danger" :disabled="locked" @click="remove(form)">删除动态</button></div>
  </form>

  <section class="moment-library" aria-label="动态列表">
   <div class="moment-library-tools">
    <div v-if="owner" class="moment-scope" role="group" aria-label="动态范围"><button type="button" :aria-pressed="scope==='site'" :disabled="locked||loading" @click="changeScope('site')">全站</button><button type="button" :aria-pressed="scope==='mine'" :disabled="locked||loading" @click="changeScope('mine')">我的</button></div>
    <SelectField :model-value="status" label="动态状态" :options="statusOptions" :disabled="locked||loading" @update:model-value="changeStatus"/>
   </div>
   <div v-if="listError" class="moment-feedback is-error" role="alert">{{listError}}<br><button type="button" :disabled="loading||locked" @click="load">重新加载列表</button></div>
   <p v-if="loading" class="moment-list-loading" role="status">正在载入动态…</p>
   <div class="moment-list" :aria-busy="loading">
    <article v-for="row in rows" :key="row.id" class="moment-list-row" :class="{'is-editing':form.id===row.id}">
     <div class="moment-row-meta"><strong>{{author(row)}}</strong><span class="moment-row-status" :class="{'is-published':row.published}">{{rowStatus(row)}}</span><time :datetime="row.updated_at">{{timestamp(row.updated_at)}}</time></div>
     <p v-if="row.text" class="moment-row-excerpt">{{row.text}}</p>
     <div v-if="row.images.length" class="moment-manager-thumbs" :aria-label="`${row.images.length} 张草稿图片`"><img v-for="(image,index) in row.images" :key="image.media_id" :src="image.url" :alt="image.alt||`第 ${index+1} 张图片`" width="72" height="72" loading="lazy"></div>
     <p v-if="row.published&&row.has_unpublished_changes" class="moment-row-note">有未发布的修改</p>
     <div class="moment-row-actions"><button type="button" :disabled="locked" :aria-current="form.id===row.id?'true':undefined" @click="edit(row)"><Icon :name="row.status==='deleted'?'posts':'pen'" :size="16"/>{{row.status==='deleted'?'查看':form.id===row.id?'正在编辑':'编辑'}}</button><button v-if="row.status!=='deleted'&&!row.deleted_at" type="button" class="quiet danger" :disabled="locked||loading" @click="remove(row)">删除</button></div>
    </article>
   </div>
   <div v-if="!loading&&!listError&&!rows.length" class="moment-library-empty"><h2>{{status==='deleted'?'没有已删除的动态':status==='draft'?'没有草稿':status==='published'?'还没有公开动态':'这里还没有动态'}}</h2><p>{{status==='deleted'?'删除的动态会显示在这里，仅供查看。':'写下此刻的想法，保存后再决定何时公开。'}}</p></div>
   <div class="moment-pagination"><span>{{loading?'正在更新…':listError?'列表未载入':`第 ${page+1} 页 · 本页 ${rows.length} 条`}}</span><div><button type="button" :disabled="page===0||loading||locked" @click="paginate(-1)">上一页</button><button type="button" :disabled="!nextCursor||loading||locked" @click="paginate(1)">下一页</button></div></div>
  </section>
 </div>
</template>
