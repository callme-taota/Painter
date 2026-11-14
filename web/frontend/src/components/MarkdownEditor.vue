<script setup lang="ts">
import { randomID } from '../random'
import '../editor-v16.css'
import {onMounted,onBeforeUnmount,reactive,ref,watch,computed,nextTick} from 'vue'
import {Compartment,EditorState} from '@codemirror/state'
import {EditorView,keymap,drawSelection,highlightActiveLine,lineNumbers} from '@codemirror/view'
import {defaultKeymap,history,historyKeymap,indentWithTab,undo,redo} from '@codemirror/commands'
import {markdown} from '@codemirror/lang-markdown'
import {HighlightStyle,syntaxHighlighting} from '@codemirror/language'
import {tags as syntaxTags} from '@lezer/highlight'
import {searchKeymap,highlightSelectionMatches,openSearchPanel} from '@codemirror/search'
import {api,body,session,onSessionEnd,APIError,list,download,date,allPages,type Row} from '../api'
import {track} from '../telemetry'
import Icon from './Icon.vue'
import SelectField from './SelectField.vue'
import {confirmAction} from '../confirm'
import DocumentEditor from './DocumentEditor.vue'
import ArticleCardComposer from './ArticleCardComposer.vue'
import {articleCardMarkdown,articleReferenceMarkdown,type ArticleCard} from '../articleCards'
import {initArticleCards} from '../contentCards'
import {markdownCardAt} from '../editorMarkdown'
import TagPicker from './TagPicker.vue'
import {recoveryGet,recoveryPut,recoveryDelete,uncreatedRecoveries} from '../storage'
const props=defineProps<{postId?:string}>()
const host=ref<HTMLElement>(),loading=ref(true),error=ref(''),notice=ref(''),saving=ref(false),state=ref('尚未保存'),viewMode=ref<'document'|'source'>('document'),sourceView=ref('edit'),preview=ref(''),previewBusy=ref(false),revisionList=ref<Row[]>([]),showHistory=ref(false),conflict=ref<Row|null>(null),recovered=ref<Row|null>(null),tags=ref<Row[]>([]),uploadCount=ref(0),working=ref(false)
const draft=reactive<Row>({id:props.postId||'',title:'',summary:'',markdown:'',tag_ids:[],cover:defaultCover(),date_display:defaultDates(),version:1,draft_version:1,publication:null})
const titleInput=ref<HTMLTextAreaElement>()
let titleObserver:ResizeObserver|undefined,titleWidth=0
function resizeTitle(){const input=titleInput.value;if(!input)return;input.style.height='auto';const style=getComputedStyle(input);input.style.height=`${input.scrollHeight+parseFloat(style.borderTopWidth)+parseFloat(style.borderBottomWidth)}px`}
function updateTitle(event:Event){const input=event.target as HTMLTextAreaElement,value=input.value.replace(/[\r\n]+/g,' ');if(input.value!==value)input.value=value;draft.title=value;resizeTitle()}
function titleEnter(event:KeyboardEvent){if(!event.isComposing)event.preventDefault()}
watch(()=>draft.title,resizeTitle,{flush:'post'})
onMounted(()=>{if(titleInput.value){titleObserver=new ResizeObserver(entries=>{const width=entries[0]?.contentRect.width||0;if(Math.abs(width-titleWidth)<.5)return;titleWidth=width;resizeTitle()});titleObserver.observe(titleInput.value)}void nextTick(resizeTitle);void document.fonts.ready.then(resizeTitle)})
onBeforeUnmount(()=>titleObserver?.disconnect())
const initialized=ref(false),initializationError=ref(''),restoring=ref(false),restoreNeedsReload=ref(false),endingSession=ref(false),mergePending=ref(false),conflictLoading=ref(false)
const editorLocked=computed(()=>loading.value||!initialized.value||restoring.value||restoreNeedsReload.value||endingSession.value)
const sourceAccess=new Compartment()
watch(editorLocked,locked=>editor?.dispatch({effects:sourceAccess.reconfigure([EditorState.readOnly.of(locked),EditorView.editable.of(!locked)])}),{flush:'sync'})
const propertiesQuery=matchMedia('(min-width:1360px)'),propertiesOpen=ref(propertiesQuery.matches)
const editorPhoneQuery=matchMedia('(max-width:720px)'),editorPhone=ref(editorPhoneQuery.matches),phoneEditorStep=ref<'writing'|'details'>('writing')
function resizeEditorPhone(){editorPhone.value=editorPhoneQuery.matches;if(editorPhone.value&&sourceView.value==='split')sourceView.value='edit'}
function changePhoneEditorStep(step:'writing'|'details'){if(composing()){notice.value='请先完成当前输入，再切换编辑步骤。';return}documentEditor.value?.flush();phoneEditorStep.value=step;closeMore();if(document.activeElement instanceof HTMLElement&&document.activeElement.matches('input,textarea,[contenteditable=true]'))document.activeElement.blur();void nextTick(()=>document.getElementById('editor-step-'+step)?.scrollIntoView({block:'start',behavior:'auto'}))}
function editorStepKey(e:KeyboardEvent){if(!['ArrowRight','ArrowLeft','Home','End'].includes(e.key))return;e.preventDefault();const step=e.key==='Home'?'writing':e.key==='End'?'details':phoneEditorStep.value==='writing'?'details':'writing';changePhoneEditorStep(step);void nextTick(()=>document.querySelector<HTMLButtonElement>(`#editor-step-${step}`)?.focus())}
function publishOrReview(){if(editorPhone.value&&phoneEditorStep.value==='writing'){changePhoneEditorStep('details');return}void command('publish')}
onMounted(()=>editorPhoneQuery.addEventListener('change',resizeEditorPhone))
onBeforeUnmount(()=>editorPhoneQuery.removeEventListener('change',resizeEditorPhone))
function resizeProperties(){propertiesOpen.value=propertiesQuery.matches}
onMounted(()=>propertiesQuery.addEventListener('change',resizeProperties))
onBeforeUnmount(()=>propertiesQuery.removeEventListener('change',resizeProperties))
let editor:EditorView|undefined,seq=0,ackSeq=0,saveTimer:ReturnType<typeof setTimeout>|undefined,previewTimer:ReturnType<typeof setTimeout>|undefined,persistTimer:ReturnType<typeof setTimeout>|undefined,previewAbort:AbortController|undefined,queue=Promise.resolve(),pendingSave:{key:string;snapshot:Row;seq:number}|null=null,stopped=false,lastSavedAt=0
const clientID=ref('')
const documentEditor=ref<InstanceType<typeof DocumentEditor>>(),documentIssue=ref(''),documentComposing=ref(false),uploadInput=ref<HTMLInputElement>()
const sourceCardOpen=ref(false)
const sourceCardInitial=ref<ArticleCard|null>(null)
const sourcePreview=ref<HTMLElement>()
watch(preview,()=>void nextTick(()=>{if(sourcePreview.value)initArticleCards(sourcePreview.value)}))
let sourceCardRange:{from:number;to:number}|undefined
function openSourceCard(){if(editorLocked.value||!editor)return;const at=markdownCardAt(editor.state.doc.toString(),editor.state.selection.main.from);sourceCardInitial.value=at?.card||null;sourceCardRange=at?{from:at.from,to:at.to}:{from:editor.state.selection.main.from,to:editor.state.selection.main.to};sourceCardOpen.value=true}
function sourceCardApply(value:{card:ArticleCard;inline:boolean;label:string}){if(!editor||!sourceCardRange||editorLocked.value)return;const text=value.inline?articleReferenceMarkdown(value.card,value.label):'\n\n'+articleCardMarkdown(value.card)+'\n\n';editor.dispatch({changes:{...sourceCardRange,insert:text},selection:{anchor:sourceCardRange.from+text.length}});sourceCardOpen.value=false;editor.focus()}
let syncingSource=false
function composing():boolean{return !!editor?.composing||documentComposing.value||!!documentEditor.value?.isComposing()}
function documentChanged(value:string){if(editorLocked.value||value===draft.markdown)return;draft.markdown=value;syncingSource=true;try{editor?.dispatch({changes:{from:0,to:editor.state.doc.length,insert:value}})}finally{syncingSource=false}changed()}
function documentCompatibility(issue:string){documentIssue.value=issue;if(issue&&viewMode.value==='document')viewMode.value='source'}
function documentComposition(value:boolean){documentComposing.value=value;if(!value){setTimeout(()=>{if(seq!==ackSeq)void autoSave();schedulePreview()},40)}}
const moreActions=ref<HTMLDetailsElement>()
function closeMore(focus=false){const menu=moreActions.value;if(!menu)return;menu.open=false;if(focus)menu.querySelector('summary')?.focus()}
function dismissMore(event:PointerEvent){if(!moreActions.value?.contains(event.target as Node))closeMore()}
function moreFocusOut(event:FocusEvent){if(event.relatedTarget&&!moreActions.value?.contains(event.relatedTarget as Node))closeMore()}
onMounted(()=>document.addEventListener('pointerdown',dismissMore))
onBeforeUnmount(()=>document.removeEventListener('pointerdown',dismissMore))
function switchMode(mode:'document'|'source'){if(editorLocked.value)return;if(composing()){notice.value='请先完成当前输入，再切换编辑方式。';return}if(mode==='document'&&documentIssue.value){notice.value='这篇文章包含文档编辑暂不支持的语法，请继续在 Markdown 源码中编辑。';return}documentEditor.value?.flush();viewMode.value=mode;closeMore();requestAnimationFrame(()=>{if(mode==='source'){editor?.requestMeasure();if(sourceView.value!=='preview')editor?.focus()}else documentEditor.value?.focus()})}
function exportMarkdown(){closeMore();download((draft.title||'未命名文章')+'.md',draft.markdown)}
function sourcePaste(value:string){if(editorLocked.value)return;viewMode.value='source';notice.value='粘贴内容含有文档模式暂不支持的语法，已完整放入 Markdown 源码。';documentChanged(value);requestAnimationFrame(()=>{editor?.requestMeasure();editor?.focus()})}
const localNewDrafts=ref<Row[]>([])
let resolveNewMapping=false
const editorHighlight=HighlightStyle.define([
 {tag:[syntaxTags.keyword,syntaxTags.heading,syntaxTags.link,syntaxTags.atom,syntaxTags.number],color:'var(--accent)'},
 {tag:[syntaxTags.string,syntaxTags.regexp],color:'var(--success)'},
 {tag:[syntaxTags.comment,syntaxTags.meta,syntaxTags.punctuation],color:'var(--muted)'},
 {tag:syntaxTags.strong,fontWeight:'700'},
 {tag:syntaxTags.emphasis,fontStyle:'italic'},
 {tag:syntaxTags.strikethrough,textDecoration:'line-through'},
 {tag:syntaxTags.invalid,color:'var(--danger)'}
])
const recoveryActorID=String(session.user?.id||'')
const recoveryKey=computed(()=>`painter:${location.origin}:${recoveryActorID}:${draft.id||clientID.value}`)
const recoveryWrites=new Set<Promise<boolean>>()
const removeSessionEnd=onSessionEnd(async reason=>{if(reason==='expired'){try{documentEditor.value?.flush()}catch{/* Preserve the last serialised Markdown if the document view cannot flush. */}}endingSession.value=true;clearTimeout(saveTimer);clearTimeout(persistTimer);clearTimeout(previewTimer);clearTimeout(coverTimer);previewAbort?.abort();coverAbort?.abort();if(reason==='expired')await persist(true);await Promise.allSettled([...recoveryWrites])})
const isDirty=computed(()=>state.value!=='已保存')
function defaultDates(value:Row={}){return {created:typeof value.created==='boolean'?value.created:true,updated:typeof value.updated==='boolean'?value.updated:true,published:typeof value.published==='boolean'?value.published:true}}
function defaultCover(value:Row={}){return {mode:value.mode||'auto',template:value.template||'editorial',color:value.color||'ink',...((value.mode==='upload'&&value.media_id)?{media_id:String(value.media_id)}:{})}}
const coverPreview=ref(''),coverBusy=ref(false)
let coverAbort:AbortController|undefined,coverTimer:ReturnType<typeof setTimeout>|undefined,coverObjectURL=''
function snapshot(){return {title:draft.title||'未命名文章',summary:draft.summary,markdown:draft.markdown,tag_ids:[...draft.tag_ids],cover:defaultCover(draft.cover),date_display:defaultDates(draft.date_display)}}
function scheduleCover(){clearTimeout(coverTimer);coverTimer=setTimeout(()=>void renderCover(),400)}
async function renderCover(){coverAbort?.abort();const cover=defaultCover(draft.cover);if(cover.mode==='none'){coverPreview.value='';return}if(cover.mode==='upload'){coverPreview.value=cover.media_id?'/media/'+cover.media_id:'';return}if(!navigator.onLine)return;const controller=new AbortController();coverAbort=controller;coverBusy.value=true;try{const res=await fetch('/api/v1/editor/cover-preview',{method:'POST',credentials:'same-origin',headers:{'Content-Type':'application/json','X-CSRF-Token':session.csrf_token},body:body({title:draft.title||'未命名文章',cover}),signal:controller.signal});if(!res.ok)throw new Error('封面预览暂不可用');const blob=await res.blob();if(controller.signal.aborted)return;if(coverObjectURL)URL.revokeObjectURL(coverObjectURL);coverObjectURL=URL.createObjectURL(blob);coverPreview.value=coverObjectURL}catch(e){if((e as Error).name!=='AbortError')notice.value='封面预览暂不可用，封面选择仍会随草稿保存。'}finally{if(coverAbort===controller)coverBusy.value=false}}
async function coverUpload(event:Event){const input=event.target as HTMLInputElement,file=input.files?.[0];input.value='';if(!file||editorLocked.value)return;if(file.size>10*1024*1024){error.value='封面超过 10 MiB，请压缩后重试。';return}uploadCount.value++;error.value='';try{const form=new FormData();form.append('file',file);const result=await api<Row>('/media',{method:'POST',body:form});draft.cover={...defaultCover(draft.cover),mode:'upload',media_id:result.media_id};notice.value='封面已上传，随草稿保存后生效。'}catch(e){error.value='封面上传失败：'+(e as Error).message}finally{uploadCount.value--}}
function coverName(value:Row={}){const cover=defaultCover(value);return cover.mode==='none'?'不显示封面':cover.mode==='upload'?'上传图片 · '+(cover.media_id||'尚未上传'):({editorial:'深邃渐变',geometric:'流彩渐变',minimal:'柔光渐变'} as Row)[cover.template]+' · '+({ink:'墨蓝',moss:'苔绿',clay:'陶土',plum:'梅紫'} as Row)[cover.color]}
function changed(){if(editorLocked.value)return;seq++;state.value='有未保存的修改';error.value='';clearTimeout(persistTimer);persistTimer=setTimeout(()=>void persist(),250);clearTimeout(saveTimer);if(!conflict.value&&!mergePending.value&&!working.value&&!composing())saveTimer=setTimeout(()=>void autoSave(),1000);schedulePreview()}
async function persist(finalSnapshot=false){if((endingSession.value&&!finalSnapshot)||!recoveryActorID||!initialized.value||loading.value||!clientID.value||(!draft.id&&!seq))return false;const write=(async()=>{try{await recoveryPut(recoveryKey.value,{...snapshot(),post_id:draft.id||null,base_version:draft.version,base_draft_version:draft.draft_version,manual_merge_required:mergePending.value,conflict_snapshot:mergePending.value?JSON.parse(JSON.stringify(conflict.value)):null,seq,ack_seq:ackSeq,client_draft_id:clientID.value,saved_at:Date.now()});return true}catch{notice.value='浏览器无法保存恢复点。请导出 Markdown，避免离线内容丢失。';return false}})();recoveryWrites.add(write);try{return await write}finally{recoveryWrites.delete(write)}}
async function finishMapping(oldKey:string){if(await persist()){try{await recoveryDelete(oldKey)}catch{notice.value='文章已保存。旧本地恢复点尚未清理，下次会先核对服务器映射。'}}}
function schedulePreview(){clearTimeout(previewTimer);if(composing())return;previewTimer=setTimeout(()=>void renderPreview(),700)}
async function renderPreview(){if(!navigator.onLine){notice.value='当前离线，预览保留上一次联机结果。';return}previewAbort?.abort();const controller=new AbortController();previewAbort=controller;const at=seq,text=draft.markdown;previewBusy.value=true;try{const data=await api('/editor/preview',{method:'POST',body:body({markdown:text,seq:at}),signal:controller.signal});if(seq===at&&draft.markdown===text)preview.value=data.sanitized_html||data.html||''}catch(e){if((e as Error).name!=='AbortError')notice.value='预览暂不可用；正文和本地修改仍然保留。'}finally{if(previewAbort===controller)previewBusy.value=false}}
function applyServer(data:Row){Object.assign(draft,data.draft?{...data,...data.draft}:data);draft.tag_ids=(data.tag_ids||data.tags?.map((x:Row)=>x.id)||draft.tag_ids||[]).map(String);draft.cover=defaultCover(data.cover||data.draft?.cover);draft.date_display=defaultDates(data.date_display||data.draft?.date_display);draft.publication=data.publication||null}
function validServerDraft(data:Row|null,id:string){const d=data?.draft?{...data,...data.draft}:data;return !!d&&String(data?.id||data?.post_id)===String(id)&&Number.isSafeInteger(Number(data?.version))&&Number(data?.version)>0&&Number.isSafeInteger(Number(d.draft_version))&&Number(d.draft_version)>0&&typeof d.title==='string'&&typeof d.summary==='string'&&typeof d.markdown==='string'&&Array.isArray(d.tag_ids)}
function requireServerDraft(data:Row,id:string){if(!validServerDraft(data,id))throw new Error('服务器草稿数据不完整，请重新载入。')}
async function performSave(manual=false,restoreFlush=false){if(endingSession.value||!initialized.value||loading.value||restoreNeedsReload.value||(restoring.value&&!restoreFlush)||stopped||(conflict.value&&!mergePending.value)||(mergePending.value&&!manual))return;if(composing()){state.value='正在输入，组合结束后保存';return}if(!navigator.onLine){state.value='离线 · 已保留本地修改';await persist();return}if(seq===ackSeq&&draft.id)return;if(draft.cover.mode==='upload'&&!draft.cover.media_id){state.value='封面待选图片 · 已保留本地修改';error.value='';await persist();return}
 saving.value=true;error.value='';state.value='正在保存…'
 const command=pendingSave||{key:randomID(),snapshot:snapshot(),seq};pendingSave=command
 try{let res:Row|undefined;const oldKey=!draft.id?recoveryKey.value:''
  if(!draft.id&&resolveNewMapping){try{res=await api<Row>(`/me/posts/by-client-id/${encodeURIComponent(clientID.value)}`);draft.id=String(res.id||res.post_id);draft.version=res.version;draft.draft_version=res.draft_version;draft.publication=res.publication||null;historyReplace(draft.id);sessionStorage.removeItem('painter-new-draft');resolveNewMapping=false;if(comparable(res)!==comparable(command.snapshot)){conflict.value=res;pendingSave=null;state.value='存在编辑冲突';notice.value='已找回这篇新稿的服务器记录。请比较本地内容与服务器版本后再保存。';await finishMapping(oldKey);return}}catch(e){if((e as APIError).status!==404)throw e;resolveNewMapping=false}}
  if(!res){if(!draft.id){res=await api<Row>('/posts',{method:'POST',key:command.key,body:body({...command.snapshot,client_draft_id:clientID.value})});draft.id=String(res.id||res.post_id);historyReplace(draft.id);sessionStorage.removeItem('painter-new-draft')}
   else res=await api<Row>(`/studio/posts/${draft.id}/draft`,{method:'PUT',version:draft.version,key:command.key,body:body({...command.snapshot,expected_draft_version:draft.draft_version})})}
  draft.version=res.version;draft.draft_version=res.draft_version;ackSeq=command.seq;pendingSave=null;lastSavedAt=Date.now();if(mergePending.value&&ackSeq===seq){mergePending.value=false;conflict.value=null;notice.value='合并后的内容已手动保存。'}state.value=ackSeq===seq?'已保存':mergePending.value?'合并中 · 需手动保存':'有未保存的修改';if(oldKey)await finishMapping(oldKey);else await persist();
 }catch(e){const err=e as APIError;if(err.status&&err.status<500&&err.status!==408&&err.status!==429)pendingSave=null;state.value=err.status===409?'存在编辑冲突':'保存失败 · 本地内容已保留';error.value=err.message;track('editor_error.v1',{...(draft.id?{post_id:draft.id}:{}),operation_id:command.key,props:{editor_operation:'save',error_code:['NETWORK_ERROR','FETCH_FAILED','VERSION_CONFLICT','VALIDATION_FAILED','SESSION_EXPIRED','UNAUTHENTICATED','PERMISSION_DENIED','CSRF_FAILED','NOT_FOUND','IDEMPOTENCY_KEY_REUSED','PAYLOAD_TOO_LARGE','MEDIA_NOT_READY','STORAGE_UNAVAILABLE','RATE_LIMITED','WRITE_BUSY','INTERNAL_ERROR','PRECONDITION_REQUIRED','REQUEST_FAILED'].includes(err.code)?err.code:'UNKNOWN'}});if(err.status===409){mergePending.value=false;await refreshConflict()}if(err.status===401)notice.value='会话已失效。请先导出，重新登录后再比较服务器版本。';await persist();throw e
 }finally{saving.value=false}
}
function historyReplace(id:string){window.history.replaceState({},'',`/studio/posts/${id}`)}
function enqueueSave(manual:boolean){if(editorLocked.value||(!manual&&(working.value||mergePending.value||conflict.value)))return Promise.resolve();clearTimeout(saveTimer);queue=queue.catch(()=>{}).then(()=>performSave(manual));return queue.catch(()=>{})}
function save(){return enqueueSave(true)}
function autoSave(){return enqueueSave(false)}
async function flush(restoreFlush=false){if(endingSession.value||!initialized.value||loading.value||restoreNeedsReload.value)throw new Error('编辑器尚未安全加载，请先重新加载。');clearTimeout(saveTimer);await queue.catch(()=>{});if(uploadCount.value)throw new Error('图片仍在上传，请稍后发布。');if(composing())throw new Error('请先完成中文输入，再继续操作。');if(conflict.value||mergePending.value)throw new Error('请先比较并合并内容，再手动保存草稿。');if(/\[图片上传(?:失败|中)|blob:/.test(draft.markdown))throw new Error('请先处理正文中的图片上传占位，再发布。');if(draft.cover.mode==='upload'&&!draft.cover.media_id)throw new Error('请先上传封面，或选择自动封面。');if(seq!==ackSeq||!draft.id)await performSave(false,restoreFlush);if(seq!==ackSeq||!draft.id)throw new Error('修改尚未保存。请联网后重试。')}
async function command(action:string){if(editorLocked.value||working.value||saving.value)return;documentEditor.value?.flush();working.value=true;error.value='';try{await flush();const res=await api<Row>(`/studio/posts/${draft.id}/${action}`,{method:action==='publication'?'DELETE':'POST',version:draft.version,idempotent:true,body:body({expected_draft_version:draft.draft_version})});if(res.version)draft.version=res.version;if(res.draft_version)draft.draft_version=res.draft_version;if(action==='publish'){draft.publication={revision_id:res.revision_id};notice.value='文章已发布。后续草稿修改不会改变当前公开版本。'}else if(action==='publication'){draft.publication=null;notice.value='文章已撤回，草稿和历史修订保留。'}else{notice.value='已创建不可变检查点。';await loadRevisions()}}catch(e){error.value=(e as Error).message}finally{working.value=false;if(seq!==ackSeq&&!conflict.value&&!mergePending.value)saveTimer=setTimeout(()=>void autoSave(),1000)}}
function insert(before:string,after=''){if(editorLocked.value)return;if(viewMode.value==='document'){documentEditor.value?.insertMarkdown(before+after);return}if(!editor)return;const {from,to}=editor.state.selection.main;const selected=editor.state.sliceDoc(from,to);editor.dispatch({changes:{from,to,insert:before+selected+after},selection:{anchor:from+before.length,head:from+before.length+selected.length}});editor.focus()}
async function upload(file:File){if(editorLocked.value)return;if(file.size>10*1024*1024){error.value='图片超过 10 MiB，请压缩后重试。';return}const token=`[图片上传中 ${randomID()}]`;insert(`\n\n${token}\n\n`);uploadCount.value++;const form=new FormData();form.append('file',file);try{const res=await api<Row>('/media',{method:'POST',body:form,idempotent:true});replaceUpload(token,`![${file.name.replace(/[\[\]\\]/g,'')}](/media/${res.media_id})`)}catch(e){replaceUpload(token,`[图片上传失败：${file.name.replace(/[\[\]\\]/g,'')}]`);error.value=`图片上传失败：${(e as Error).message}。请删除上传占位后重试。`}finally{uploadCount.value--}}
function replaceUpload(token:string,replacement:string){if(viewMode.value==='document'&&documentEditor.value?.replaceText(token,replacement))return;if(!editor)return;const text=editor.state.doc.toString(),at=text.indexOf(token);if(at>=0)editor.dispatch({changes:{from:at,to:at+token.length,insert:replacement}})}
function pickFile(event:Event){const file=(event.target as HTMLInputElement).files?.[0];if(file)void upload(file);(event.target as HTMLInputElement).value=''}
async function loadRevisions(){if(!draft.id)return;revisionList.value=list(await api(`/studio/posts/${draft.id}/revisions`))}
async function reloadAfterRestore(){
 if(!restoreNeedsReload.value||restoring.value)return
 restoring.value=true;error.value=''
 try{await applyRestoredDraft()}catch(e){error.value='无法确认恢复后的草稿：'+(e as Error).message}finally{restoring.value=false}
}
async function applyRestoredDraft(){
 const current=await api(`/studio/posts/${draft.id}`)
 if(stopped)return
 requireServerDraft(current,draft.id)
 loading.value=true
 try{applyServer(current);pendingSave=null;conflict.value=null;mergePending.value=false;editor?.dispatch({changes:{from:0,to:editor.state.doc.length,insert:draft.markdown}});seq++;ackSeq=seq;state.value='已保存';restoreNeedsReload.value=false}finally{loading.value=false}
 await persist();notice.value='已载入服务器当前草稿。恢复操作不会自动发布。';void renderPreview();void renderCover()
}
async function restore(rid:string){
 if(editorLocked.value||working.value||saving.value)return
 if(!await confirmAction('恢复这个版本？','将先保存当前内容并创建检查点，再恢复此版本为草稿。公开版本不会随之改变。','恢复为草稿'))return
 if(editorLocked.value||working.value||saving.value)return
 documentEditor.value?.flush();restoring.value=true;working.value=true;error.value='';clearTimeout(saveTimer)
 try{
  await flush(true);await persist()
  // Once a restore is sent, do not reopen stale inputs until a fresh GET succeeds.
  restoreNeedsReload.value=true;state.value='正在恢复历史版本…'
  await api<Row>(`/studio/posts/${draft.id}/restore`,{method:'POST',version:draft.version,idempotent:true,body:body({revision_id:rid})})
  await applyRestoredDraft();notice.value='历史版本已恢复为草稿，尚未发布。'
 }catch(e){error.value=(e as Error).message;if(restoreNeedsReload.value)state.value='恢复状态待确认 · 编辑已暂停'}finally{working.value=false;restoring.value=false}
}
function comparable(value:Row){return JSON.stringify({title:value.title||'未命名文章',summary:value.summary||'',markdown:value.markdown||'',tag_ids:(value.tag_ids||[]).map(String).sort(),cover:defaultCover(value.cover),date_display:defaultDates(value.date_display)})}
function recoveryDiffers(local:Row){return comparable(local)!==comparable(snapshot())}
async function restoreLocalNew(local:Row){if(editorLocked.value||saving.value||working.value||draft.id)return;working.value=true;clearTimeout(saveTimer);clearTimeout(persistTimer);try{await queue.catch(()=>{});if(draft.id)return;await persist();loading.value=true;clientID.value=local.client_draft_id;sessionStorage.setItem('painter-new-draft',clientID.value);Object.assign(draft,{id:'',title:local.title||'',summary:local.summary||'',markdown:local.markdown||'',tag_ids:local.tag_ids||[],cover:defaultCover(local.cover),date_display:defaultDates(local.date_display),version:local.base_version||1,draft_version:local.base_draft_version||1,publication:null});editor?.dispatch({changes:{from:0,to:editor.state.doc.length,insert:draft.markdown}});seq=Number(local.seq)||1;ackSeq=Number(local.ack_seq)||0;pendingSave=null;conflict.value=null;mergePending.value=false;resolveNewMapping=true;loading.value=false;state.value='本地新稿已恢复，等待保存';notice.value='已加载这篇本地新稿。请检查后保存；保存前会核对它是否已有服务器记录。';localNewDrafts.value=localNewDrafts.value.filter(x=>x.key!==local.key);await persist();schedulePreview();scheduleCover()}catch(e){error.value=(e as Error).message}finally{loading.value=false;working.value=false}}
function dateNames(value:Row={}){const d=defaultDates(value);return [d.created?'创建':null,d.updated?'最后编辑':null,d.published?'发布':null].filter(Boolean).join('、')||'不显示时间'}
function tagNames(ids:any[]=[]){return ids.map(id=>tags.value.find(x=>String(x.id)===String(id))?.name||'已归档标签').join('、')||'无标签'}
async function useRecovery(){
 if(editorLocked.value||saving.value||working.value||!recovered.value)return
 const local=recovered.value
 if(String(local.base_version)!==String(draft.version)||String(local.base_draft_version)!==String(draft.draft_version))conflict.value={...draft}
 else if(local.manual_merge_required){conflict.value=local.conflict_snapshot||{...draft};mergePending.value=true}
 loading.value=true
 try{Object.assign(draft,{title:local.title,summary:local.summary,markdown:local.markdown,tag_ids:local.tag_ids||[],cover:defaultCover(local.cover),date_display:defaultDates(local.date_display)});editor?.dispatch({changes:{from:0,to:editor.state.doc.length,insert:local.markdown}})}finally{loading.value=false}
 recovered.value=null;changed();notice.value=mergePending.value?'已恢复合并中的本地内容，请检查后手动保存。':'已加载本地恢复点，请检查后保存。'
}
const validConflict=computed(()=>validServerDraft(conflict.value,draft.id))
async function refreshConflict(){
 conflictLoading.value=true
 try{const current=await api(`/studio/posts/${draft.id}`);requireServerDraft(current,draft.id);conflict.value=current}catch{conflict.value={};notice.value='无法载入服务器版本。本地内容已保留，请重试载入后再合并。'}finally{conflictLoading.value=false}
}
async function mergeConflict(){
 if(editorLocked.value||saving.value||working.value||!validConflict.value||mergePending.value)return
 const server=conflict.value!;clearTimeout(saveTimer);draft.version=server.version;draft.draft_version=server.draft_version||server.draft?.draft_version;mergePending.value=true;pendingSave=null;seq++;state.value='合并中 · 需手动保存';notice.value='请在编辑区合并两份内容，确认后点击“保存草稿”。合并期间自动保存已暂停。';await persist()
}
function beforeUnload(e:BeforeUnloadEvent){if(seq!==ackSeq||uploadCount.value||composing()){e.preventDefault();e.returnValue=''}}
const interval=setInterval(()=>{if(!loading.value&&seq!==ackSeq&&!saving.value&&!working.value&&!conflict.value&&Date.now()-lastSavedAt>10000)void autoSave()},10000)
async function initializeEditor(){
 if(initialized.value||stopped)return
 loading.value=true;initializationError.value='';error.value='';state.value='正在载入…'
 try{
  if(!clientID.value){clientID.value=props.postId||randomID();if(!props.postId)sessionStorage.setItem('painter-new-draft',clientID.value)}
  const values=await Promise.all([allPages('/tags?limit=50'),props.postId?api(`/studio/posts/${props.postId}`):Promise.resolve(null)])
  if(stopped)return
  if(props.postId)requireServerDraft(values[1] as Row,props.postId)
  tags.value=list(values[0]);if(values[1])applyServer(values[1]);editor?.destroy();editor=new EditorView({parent:host.value,state:EditorState.create({doc:draft.markdown,extensions:[sourceAccess.of([EditorState.readOnly.of(true),EditorView.editable.of(false)]),lineNumbers(),history(),drawSelection(),highlightActiveLine(),markdown(),syntaxHighlighting(editorHighlight),highlightSelectionMatches(),keymap.of([...defaultKeymap,...historyKeymap,...searchKeymap,indentWithTab,{key:'Mod-s',run:()=>{void save();return true}}]),EditorView.lineWrapping,EditorView.contentAttributes.of({'aria-label':'Markdown 正文'}),EditorView.theme({'&':{backgroundColor:'var(--paper)',color:'var(--ink)'},'.cm-cursor':{borderLeftColor:'var(--accent)'},'.cm-activeLine':{backgroundColor:'var(--accent-soft)'},'.cm-selectionBackground':{background:'var(--accent-soft)'}}),EditorView.updateListener.of(update=>{if(update.docChanged&&sourceCardOpen.value&&sourceCardRange){const range=sourceCardRange,from=update.changes.mapPos(range.from,1);sourceCardRange={from,to:range.from===range.to?from:Math.max(from,update.changes.mapPos(range.to,-1))}}if(update.docChanged&&!syncingSource&&!editorLocked.value){draft.markdown=update.state.doc.toString();changed()}}),EditorView.domEventHandlers({compositionend:()=>{setTimeout(()=>{if(seq!==ackSeq)void autoSave();schedulePreview()},30);return false},paste:(e)=>{const file=Array.from(e.clipboardData?.files||[]).find(f=>f.type.startsWith('image/'));if(file){void upload(file);return true}return false},drop:(e)=>{const file=Array.from(e.dataTransfer?.files||[]).find(f=>f.type.startsWith('image/'));if(file){e.preventDefault();void upload(file);return true}return false}})]})})
  initialized.value=true;loading.value=false;state.value=draft.id?'已保存':'尚未保存'
  track('editor_open.v1',draft.id?{post_id:draft.id}:{})
  const local=await recoveryGet(recoveryKey.value).catch(()=>null)
  if(stopped)return
  if(props.postId&&local&&local.seq>local.ack_seq&&(local.manual_merge_required||recoveryDiffers(local)))recovered.value=local
  if(!props.postId)localNewDrafts.value=await uncreatedRecoveries(recoveryActorID).catch(()=>[])
  void renderPreview();void renderCover()
 }catch(e){initialized.value=false;initializationError.value=(e as Error).message;state.value='载入失败 · 编辑已暂停'}finally{loading.value=false}
}
onMounted(()=>{window.addEventListener('beforeunload',beforeUnload);window.addEventListener('online',autoSave);void initializeEditor()})
watch(()=>[draft.title,draft.summary,JSON.stringify(draft.tag_ids),JSON.stringify(draft.cover),JSON.stringify(draft.date_display)],changed,{flush:'sync'})
watch(()=>[draft.title,JSON.stringify(draft.cover)],()=>{if(initialized.value&&!loading.value)scheduleCover()})
onBeforeUnmount(()=>{removeSessionEnd();stopped=true;clearInterval(interval);clearTimeout(saveTimer);clearTimeout(previewTimer);clearTimeout(persistTimer);previewAbort?.abort();coverAbort?.abort();clearTimeout(coverTimer);if(coverObjectURL)URL.revokeObjectURL(coverObjectURL);void persist();editor?.destroy();window.removeEventListener('beforeunload',beforeUnload);window.removeEventListener('online',autoSave)})
</script>
<template>
 <section class="editor-page editor-unified" :class="{'is-source':viewMode==='source','is-phone-editor':editorPhone}" :data-phone-step="phoneEditorStep">
 <header class="editor-bar"><div class="editor-heading"><h1>{{draft.id?'编辑文章':'写一篇新文章'}}</h1><span class="save-state" :class="{saved:state==='已保存'}" aria-live="polite">{{state}}{{uploadCount?' · 图片上传中':''}}</span></div><div class="toolbar"><a class="button quiet editor-return" href="/studio/posts">返回文章</a><button :disabled="saving||working||editorLocked" @click="save">保存草稿</button><button v-if="!editorPhone" :disabled="working||editorLocked||!!conflict" @click="command('checkpoints')">留检查点</button><button v-if="draft.publication&&!editorPhone" :disabled="working||editorLocked" @click="command('publication')">撤回</button><details ref="moreActions" class="editor-more" @keydown.esc.prevent.stop="closeMore(true)" @focusout="moreFocusOut"><summary aria-label="更多编辑操作">更多<Icon name="chevron" :size="15"/></summary><div class="editor-more-menu"><button v-if="viewMode==='document'" :disabled="editorLocked" @click="switchMode('source')">编辑 Markdown 源码</button><button v-else :disabled="editorLocked||!!documentIssue" @click="switchMode('document')">返回文档编辑</button><button :disabled="!initialized" @click="exportMarkdown"><Icon name="export" :size="16"/>导出 Markdown</button><button v-if="editorPhone" :disabled="working||editorLocked||!!conflict" @click="closeMore();command('checkpoints')">保存检查点</button><button v-if="editorPhone&&draft.publication" :disabled="working||editorLocked" @click="closeMore();command('publication')">撤回公开文章</button></div></details><button class="primary" :disabled="working||editorLocked||!!conflict||uploadCount>0" @click="publishOrReview">{{working?'处理中…':editorPhone&&phoneEditorStep==='writing'?'下一步':'发布文章'}}</button></div></header>
 <div v-if="initializationError" class="notice error editor-init-error" role="alert"><strong>编辑器未能安全加载</strong><p>{{initializationError}}。正文和文章信息尚未解锁，请重新加载后继续。</p><button :disabled="loading" @click="initializeEditor">{{loading?'正在重新加载…':'重新加载编辑器'}}</button></div><div v-if="restoreNeedsReload&&!restoring" class="notice error" role="alert"><strong>需要确认恢复后的服务器草稿</strong><p>编辑已暂停，当前本地内容仍保留。请重新载入后继续。</p><button @click="reloadAfterRestore">重新载入当前草稿</button><button class="quiet" @click="download('恢复前本地副本.md',draft.markdown)">导出本地副本</button></div><div v-if="restoring" class="notice" role="status">正在恢复历史版本，编辑暂时锁定…</div><div v-if="error" class="notice error" role="alert">{{error}}</div><div v-if="notice" class="notice" role="status">{{notice}}</div>
 <section v-if="!draft.id&&localNewDrafts.length" class="panel" aria-labelledby="new-recovery-title"><h2 id="new-recovery-title">找回尚未同步的新稿</h2><p class="muted">这些内容只保存在当前浏览器、当前账号下。选择前会先保留正在编辑的本地内容，恢复不会直接发布。</p><div v-for="local in localNewDrafts" :key="local.key" class="user-row"><div class="user-info"><strong>{{local.title||'未命名文章'}}</strong><p class="muted">{{local.summary||String(local.markdown||'').slice(0,100)}}</p><small>{{date(local.saved_at)}}</small></div><div class="actions"><button :disabled="saving||working||editorLocked" @click="restoreLocalNew(local)">恢复这篇新稿</button><button class="quiet" @click="download((local.title||'本地新稿')+'.md',local.markdown)">导出新稿</button></div></div></section>
 <div v-if="recovered" class="notice"><strong>发现尚未同步的本地内容</strong><p>恢复点来自 {{date(recovered.saved_at)}}。服务器内容保持不变。</p><button :disabled="editorLocked||saving||working" @click="useRecovery">加载恢复点</button> <button class="quiet" @click="download('本地恢复.md',recovered.markdown)">导出恢复点</button> <button class="quiet" @click="recovered=null">稍后处理</button></div>
 <nav v-if="editorPhone" class="phone-task-tabs editor-phone-steps" role="tablist" aria-label="文章编辑步骤" @keydown="editorStepKey"><button id="editor-step-writing" type="button" role="tab" :tabindex="phoneEditorStep==='writing'?0:-1" :aria-selected="phoneEditorStep==='writing'" aria-controls="editor-phone-writing" @click="changePhoneEditorStep('writing')">正文</button><button id="editor-step-details" type="button" role="tab" :tabindex="phoneEditorStep==='details'?0:-1" :aria-selected="phoneEditorStep==='details'" aria-controls="editor-phone-details" @click="changePhoneEditorStep('details')">文章信息</button></nav>
 <div class="editor-workspace">
 <div class="editor-title-area"><textarea ref="titleInput" class="editor-title" :disabled="editorLocked" :value="draft.title" @input="updateTitle" @keydown.enter="titleEnter" maxlength="200" rows="1" aria-label="文章标题" placeholder="给文章一个标题"></textarea></div>
 <details id="editor-phone-details" class="editor-properties" v-show="!editorPhone||phoneEditorStep==='details'" :role="editorPhone?'tabpanel':undefined" :aria-labelledby="editorPhone?'editor-step-details':undefined" :open="editorPhone?phoneEditorStep==='details':propertiesOpen" @toggle="!editorPhone&&(propertiesOpen=($event.target as HTMLDetailsElement).open)"><summary><span>文章信息与展示</span><span class="property-summary">{{draft.tag_ids.length}} 个标签 · {{coverName(draft.cover)}}</span><Icon name="chevron" :size="16"/></summary><div class="editor-metadata"><label class="full">摘要<textarea :disabled="editorLocked" v-model="draft.summary" maxlength="500" rows="2" placeholder="用一两句话介绍这篇文章"></textarea></label><TagPicker class="full" v-model="draft.tag_ids" :tags="tags" :loading="loading" :disabled="editorLocked"/><fieldset class="full date-display-options"><legend>文章时间</legend><div class="date-display-choices"><label class="check"><input type="checkbox" v-model="draft.date_display.created" :disabled="editorLocked">显示创建时间</label><label class="check"><input type="checkbox" v-model="draft.date_display.updated" :disabled="editorLocked">显示最后编辑时间</label><label class="check"><input type="checkbox" v-model="draft.date_display.published" :disabled="editorLocked">显示发布时间</label></div><p class="muted">时间选项随发布生效，最后编辑时间对应当前公开版本。时间按北京时间显示。</p></fieldset><fieldset class="full cover-editor"><legend>文章封面</legend><div class="cover-options"><SelectField v-model="draft.cover.mode" :disabled="editorLocked" label="封面来源" :options="[{value:'auto',label:'自动设计'},{value:'upload',label:'上传图片'},{value:'none',label:'不显示封面'}]"/><template v-if="draft.cover.mode==='auto'"><SelectField :disabled="editorLocked" v-model="draft.cover.template" label="版式" :options="[{value:'editorial',label:'深邃渐变'},{value:'geometric',label:'流彩渐变'},{value:'minimal',label:'柔光渐变'}]"/><SelectField :disabled="editorLocked" v-model="draft.cover.color" label="配色" :options="[{value:'ink',label:'墨蓝'},{value:'moss',label:'苔绿'},{value:'clay',label:'陶土'},{value:'plum',label:'梅紫'}]"/></template><label v-if="draft.cover.mode==='upload'">选择封面图片<input type="file" :disabled="editorLocked" accept="image/jpeg,image/png,image/webp,image/gif" @change="coverUpload"><small class="muted">JPEG、PNG、WebP 或 GIF，最大 10 MiB。</small></label></div><div class="cover-preview" v-if="draft.cover.mode!=='none'"><img v-if="coverPreview" :src="coverPreview" alt="文章封面预览" width="1200" height="675"><p v-else class="muted">{{draft.cover.mode==='upload'?'选择一张图片作为封面':'正在准备封面…'}}</p><span v-if="coverBusy" class="cover-busy" role="status">更新预览…</span></div><p v-else class="muted">文章将以标题和正文呈现。</p></fieldset></div></details>
 <section id="editor-phone-writing" class="editor-writing" v-show="!editorPhone||phoneEditorStep==='writing'" :role="editorPhone?'tabpanel':undefined" :aria-labelledby="editorPhone?'editor-step-writing':undefined" aria-label="文章正文与编辑工具">
 <div v-if="viewMode==='source'" class="editor-source-context"><span>Markdown 源码</span><button class="quiet" :disabled="editorLocked||!!documentIssue" @click="switchMode('document')">返回文档编辑</button></div>
 <input ref="uploadInput" id="editor-upload" class="sr-only" type="file" :disabled="editorLocked" accept="image/jpeg,image/png,image/webp,image/gif" @change="pickFile">
 <div v-if="documentIssue" class="notice document-compatibility" role="status"><span>正文包含{{documentIssue}}，已保留全部原文。请使用 Markdown 源码编辑。</span><button v-if="viewMode!=='source'" @click="switchMode('source')">前往 Markdown 源码</button></div>
 <DocumentEditor v-if="initialized" v-show="viewMode==='document'" ref="documentEditor" :model-value="draft.markdown" :active="viewMode==='document'" :disabled="editorLocked" @update:model-value="documentChanged" @save="save" @pick-image="uploadInput?.click()" @image="upload" @compatibility="documentCompatibility" @composition="documentComposition" @source-paste="sourcePaste"/>
 <div v-if="!initialized" class="document-loading" role="status">{{loading?'正在载入正文…':'重新加载编辑器后，即可继续写作。'}}</div>
 <fieldset :disabled="editorLocked" v-show="viewMode==='source'" class="editor-tools" aria-label="Markdown 工具栏"><div class="tool-group" role="group" aria-label="文字格式"><button title="标题" @click="insert('## ')">标题</button><button title="加粗" aria-label="加粗" @click="insert('**','**')"><b>B</b></button><button title="斜体" aria-label="斜体" @click="insert('*','*')"><i>I</i></button><button @click="insert('- ')">列表</button><button @click="insert('> ')">引用</button><button @click="insert('```\n','\n```')">代码</button><button @click="insert('[','](https://)')">链接</button><button data-open-source-cards @click="openSourceCard">卡片</button></div><div class="tool-group" role="group" aria-label="编辑操作"><button title="撤销" aria-label="撤销" @click="editor&&undo(editor)"><Icon name="undo" :size="18"/></button><button title="重做" aria-label="重做" @click="editor&&redo(editor)"><Icon name="redo" :size="18"/></button><button title="查找" aria-label="查找" @click="editor&&openSearchPanel(editor)"><Icon name="search" :size="18"/></button><label class="button quiet" for="editor-upload" title="插入图片"><Icon name="image" :size="18"/><span class="sr-only">插入图片</span></label></div><div class="view-switch" role="group" aria-label="编辑器视图"><button :aria-pressed="sourceView==='edit'" @click="sourceView='edit'">编辑</button><button :aria-pressed="sourceView==='preview'" @click="sourceView='preview'">预览</button><button v-if="!editorPhone" :aria-pressed="sourceView==='split'" @click="sourceView='split'">分屏</button></div></fieldset>
 <ArticleCardComposer v-if="sourceCardOpen&&viewMode==='source'" :initial="sourceCardInitial" :editing="!!sourceCardInitial" :disabled="editorLocked" @close="sourceCardOpen=false;editor?.focus()" @apply="sourceCardApply"/>
 <div v-show="viewMode==='source'" class="editor-panes" :data-view="sourceView"><div class="editor-source" ref="host" aria-label="Markdown 正文编辑器"></div><div ref="sourcePreview" class="editor-preview prose"><p v-if="previewBusy" class="muted" role="status">正在生成规范预览…</p><div v-html="preview"></div><p v-if="!preview&&!previewBusy" class="muted">开始写作，预览会在这里呈现。</p></div></div>
 </section>
 </div>
 <div v-if="conflict" class="panel conflict-panel"><h2>另一处修改已先保存</h2><p>{{mergePending?'自动保存已暂停。请在编辑区合并两份内容，确认后点击“保存草稿”。':'你的内容仍在编辑区。先比较，再把需要的部分合并到当前草稿。'}}</p><div v-if="validConflict" class="conflict-panes"><div><h3>当前本地内容</h3><dl class="conflict-meta"><dt>标题</dt><dd>{{draft.title}}</dd><dt>摘要</dt><dd>{{draft.summary||'无摘要'}}</dd><dt>标签</dt><dd>{{tagNames(draft.tag_ids)}}</dd><dt>封面</dt><dd>{{coverName(draft.cover)}}</dd><dt>时间展示</dt><dd>{{dateNames(draft.date_display)}}</dd></dl><pre>{{draft.markdown}}</pre></div><div><h3>服务器最新内容</h3><dl class="conflict-meta"><dt>标题</dt><dd>{{conflict.title||conflict.draft?.title}}</dd><dt>摘要</dt><dd>{{conflict.summary||conflict.draft?.summary||'无摘要'}}</dd><dt>标签</dt><dd>{{tagNames(conflict.tag_ids||conflict.draft?.tag_ids)}}</dd><dt>封面</dt><dd>{{coverName(conflict.cover||conflict.draft?.cover)}}</dd><dt>时间展示</dt><dd>{{dateNames(conflict.date_display||conflict.draft?.date_display)}}</dd></dl><pre>{{conflict.markdown||conflict.draft?.markdown}}</pre></div></div><button v-if="!validConflict" :disabled="conflictLoading||editorLocked" @click="refreshConflict">{{conflictLoading?'正在载入…':'重新载入服务器版本'}}</button><button v-else :disabled="mergePending||saving||working||editorLocked" @click="mergeConflict">{{mergePending?'合并中，请检查后保存草稿':'以服务器最新版本为基准，继续合并'}}</button> <button @click="download('冲突本地副本.md',draft.markdown)">导出本地副本</button></div>
 <div class="form-actions"><button :disabled="editorLocked" @click="showHistory=!showHistory;showHistory&&loadRevisions()">{{showHistory?'收起':'查看'}}修订记录</button><a v-if="draft.publication" :href="`/posts/${draft.id}`" target="_blank" rel="noopener" class="button quiet">查看公开文章</a></div>
 <div v-if="showHistory" class="panel"><h2>不可变修订</h2><div v-if="!revisionList.length" class="muted">手动保存检查点或发布后，修订会保留在这里。</div><div v-for="r in revisionList" :key="r.id" class="user-row"><div class="user-info"><strong>{{r.title||r.reason}}</strong><div class="meta">{{date(r.created_at)}} · {{r.reason}} · {{r.renderer_version}}</div></div><button :disabled="working||editorLocked" @click="restore(r.id||r.revision_id)">恢复为草稿</button></div></div>
 </section>
</template>

<style scoped>
.document-compatibility{display:flex;align-items:center;justify-content:space-between;flex-wrap:wrap;gap:8px;font-size:.8rem}.document-compatibility button{font-size:.78rem}.document-loading{min-height:540px;padding:36px;background:var(--paper);border:1px solid var(--line);border-radius:var(--radius-surface);color:var(--muted)}
</style>

<style scoped>
.date-display-options legend{color:var(--ink);font-size:.9rem;margin-bottom:12px}.date-display-choices{display:flex;flex-wrap:wrap;gap:8px 26px}.date-display-choices .check{min-height:var(--control-height);font-size:var(--control-font);line-height:var(--control-leading)}.date-display-options p{font-size:.78rem;line-height:1.8;margin:8px 0 0}
</style>
