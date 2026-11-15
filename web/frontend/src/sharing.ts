import {registerMobileOverlay} from './mobileOverlays'
import {toast} from './toast'

type ShareMode='short'|'embed'|'share'
type ShareManifest={type:string;id:string;title:string;short_url:string;share_url:string;embed_url:string;canonical_url:string;embed_html:string}
const copyLabels:Record<ShareMode,string>={short:'复制短链接',embed:'复制 HTML',share:'复制分享页链接'}
const descriptions:Record<ShareMode,string>={short:'打开后直接前往站内原文。内容更新后，链接依然有效。',embed:'在支持 HTML 的网站中插入预览卡片，读者可点击查看原文。',share:'用独立、简洁的页面展示标题和摘要，再引导读者回到原文。'}

/** One delegated, on-demand reader. Opening a share panel never changes data. */
export function initSharing(){
 if(document.querySelector('[data-sharing-dialog]'))return
 const dialog=document.createElement('dialog')
 dialog.className='sharing-dialog';dialog.dataset.sharingDialog='';dialog.setAttribute('aria-labelledby','sharing-heading')
 dialog.innerHTML=`<div class="sharing-heading"><h2 id="sharing-heading">分享</h2><button type="button" class="sharing-close" aria-label="关闭分享"><svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" aria-hidden="true"><path d="m6 6 12 12M18 6 6 18"/></svg></button></div><p class="sharing-title"></p><div class="sharing-tabs" role="tablist" aria-label="分享方式"><button type="button" role="tab" id="sharing-tab-short" aria-controls="sharing-panel" data-sharing-mode="short" aria-selected="true">短链接</button><button type="button" role="tab" id="sharing-tab-embed" aria-controls="sharing-panel" data-sharing-mode="embed" aria-selected="false" tabindex="-1">内嵌 HTML</button><button type="button" role="tab" id="sharing-tab-share" aria-controls="sharing-panel" data-sharing-mode="share" aria-selected="false" tabindex="-1">专属分享</button></div><div id="sharing-panel" role="tabpanel" aria-labelledby="sharing-tab-short"><p class="sharing-description"></p><label class="sharing-field"><span class="sr-only">分享内容</span><textarea readonly spellcheck="false" rows="3" aria-label="分享内容"></textarea></label><p class="sharing-status" role="status" aria-live="polite"></p><div class="sharing-actions"><a class="sharing-preview" target="_blank" rel="noopener noreferrer" hidden>打开预览</a><button type="button" class="sharing-retry" hidden>重试</button><button type="button" class="sharing-copy" disabled>复制短链接</button></div></div>`
 document.body.append(dialog)
 const title=dialog.querySelector<HTMLElement>('.sharing-title')!,field=dialog.querySelector<HTMLTextAreaElement>('textarea')!,description=dialog.querySelector<HTMLElement>('.sharing-description')!,status=dialog.querySelector<HTMLElement>('.sharing-status')!,copy=dialog.querySelector<HTMLButtonElement>('.sharing-copy')!,preview=dialog.querySelector<HTMLAnchorElement>('.sharing-preview')!,retry=dialog.querySelector<HTMLButtonElement>('.sharing-retry')!,panel=dialog.querySelector<HTMLElement>('[role=tabpanel]')!
 const tabs=Array.from(dialog.querySelectorAll<HTMLButtonElement>('[role=tab]'))
 let mode:ShareMode='short',manifest:ShareManifest|null=null,trigger:HTMLElement|null=null,controller:AbortController|null=null,request=0,copyRequest=0
 const content=()=>manifest?(mode==='short'?manifest.short_url:mode==='embed'?manifest.embed_html:manifest.share_url):''
 function render(){
  tabs.forEach(tab=>{const active=tab.dataset.sharingMode===mode;tab.setAttribute('aria-selected',String(active));tab.tabIndex=active?0:-1})
  panel.setAttribute('aria-labelledby',`sharing-tab-${mode}`)
  description.textContent=descriptions[mode];field.value=content();field.rows=mode==='embed'?7:3
  copy.textContent=copyLabels[mode];copy.disabled=!manifest
  field.classList.toggle('sharing-code',mode==='embed')
  preview.hidden=!manifest
  if(manifest)preview.href=mode==='embed'?manifest.embed_url:mode==='share'?manifest.share_url:manifest.canonical_url
  else preview.removeAttribute('href')
 }
 function validManifest(value:unknown):value is ShareManifest{
  if(!value||typeof value!=='object')return false
  const v=value as Record<string,unknown>
  if(typeof v.title!=='string'||typeof v.embed_html!=='string'||v.embed_html.length>16000)return false
  return ['short_url','share_url','embed_url','canonical_url'].every(key=>{
   if(typeof v[key]!=='string')return false
   try{const url=new URL(v[key] as string);return ['http:','https:'].includes(url.protocol)&&!url.username&&!url.password}catch{return false}
  })
 }
 async function load(){
  const type=trigger?.dataset.shareType,id=trigger?.dataset.shareId
  if(!type||!['post','moment','phrase','series'].includes(type)||!id||!/^[1-9]\d{0,18}$/.test(id))return
  controller?.abort();controller=new AbortController();const current=++request
  manifest=null;title.textContent='';retry.hidden=true;status.textContent='正在读取分享内容…';field.placeholder='';render()
  try{
   const response=await fetch(`/api/v1/share?type=${encodeURIComponent(type)}&id=${encodeURIComponent(id)}`,{credentials:'omit',cache:'no-store',signal:controller.signal,headers:{Accept:'application/json'}})
   if(!response.ok)throw new Error(response.status===404?'内容已撤回、删除或尚未公开，暂时无法分享。':'未能读取分享内容，请重试。')
   const value:unknown=await response.json()
   if(!validManifest(value))throw new Error('分享内容暂时不可用，请重试。')
   if(current!==request||!dialog.open)return
   manifest=value;title.textContent=value.title;status.textContent='';render()
  }catch(error){
   if(current!==request||!dialog.open||(error instanceof DOMException&&error.name==='AbortError'))return
   status.textContent=error instanceof TypeError?'网络连接失败，请重试。':error instanceof Error?error.message:'网络连接失败，请重试。';retry.hidden=false;render()
  }
 }
 function open(button:HTMLElement,reset=true){
  trigger=button;if(reset)mode='short'
  if(!dialog.open)dialog.showModal()
  void load()
 }
 registerMobileOverlay(dialog,()=>{if(trigger)open(trigger,false)})
 document.addEventListener('click',event=>{
  const button=event.target instanceof Element?event.target.closest<HTMLElement>('[data-share-type][data-share-id]'):null
  if(!button||button instanceof HTMLButtonElement&&button.disabled)return
  event.preventDefault();open(button)
 })
 dialog.querySelector('.sharing-close')?.addEventListener('click',()=>dialog.close())
 dialog.addEventListener('click',event=>{if(event.target===dialog){const rect=dialog.getBoundingClientRect();if(event.clientX<rect.left||event.clientX>rect.right||event.clientY<rect.top||event.clientY>rect.bottom)dialog.close()}})
 dialog.addEventListener('close',()=>{controller?.abort();request++;copyRequest++;manifest=null;field.value='';preview.hidden=true;preview.removeAttribute('href');trigger?.focus({preventScroll:true})})
 retry.addEventListener('click',()=>void load())
 tabs.forEach((tab,index)=>{
  tab.addEventListener('click',()=>{mode=tab.dataset.sharingMode as ShareMode;copyRequest++;if(manifest)status.textContent='';render()})
  tab.addEventListener('keydown',event=>{
   let next=index
   if(event.key==='ArrowRight')next=(index+1)%tabs.length
   else if(event.key==='ArrowLeft')next=(index+tabs.length-1)%tabs.length
   else if(event.key==='Home')next=0
   else if(event.key==='End')next=tabs.length-1
   else return
   event.preventDefault();tabs[next]?.click();tabs[next]?.focus()
  })
 })
 copy.addEventListener('click',async()=>{
  const text=content();if(!text)return
  const current=++copyRequest
  try{
   if(!navigator.clipboard?.writeText)throw new Error('clipboard unavailable')
   await navigator.clipboard.writeText(text)
   if(dialog.open&&current===copyRequest){status.textContent='';toast.success(mode==='embed'?'HTML 已复制':'链接已复制',{id:'sharing-copy'})}
  }catch{
   if(!dialog.open||current!==copyRequest)return
   field.focus();field.select();field.setSelectionRange(0,field.value.length)
   status.textContent='';toast.warning('未能自动复制，已选中内容。请长按或使用复制快捷键。',{id:'sharing-copy'})
  }
 })
 render()
}
