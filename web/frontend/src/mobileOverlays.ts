/** Native dialogs share one phone contract: Back dismisses the layer, not its page. */
const registrations=new WeakMap<HTMLDialogElement,()=>void>()
const phone=()=>matchMedia('(max-width:720px)').matches
function syncScrollLock(){document.documentElement.classList.toggle('mobile-overlay-open',phone()&&!!document.querySelector('dialog[open]'))}
/** Finish history dismissal before a link or GET form navigates away. */
export function dismissMobileOverlay(dialog:HTMLDialogElement,value=''):Promise<void>{
 if(!dialog.open)return Promise.resolve()
 if(!dialog.dataset.mobileOverlayToken||history.state?.painterMobileOverlay!==dialog.dataset.mobileOverlayToken){dialog.close(value);return Promise.resolve()}
 return new Promise(resolve=>{
  const done=()=>{clearTimeout(timeout);window.removeEventListener('popstate',done);resolve()}
  const timeout=window.setTimeout(done,700)
  window.addEventListener('popstate',done,{once:true});dialog.close(value)
 })
}
export function registerMobileOverlay(dialog:HTMLDialogElement,restore?:()=>void):()=>void{
 const existing=registrations.get(dialog);if(existing)return existing
 let token:string|null=null,disposed=false
 const ownedTokens=new Set<string>()
 const marker='painterMobileOverlay'
 const state=()=>history.state&&typeof history.state==='object'?history.state:{}
 function opened(){
  syncScrollLock()
  if(!dialog.open||!phone()||token)return
  token=crypto.randomUUID?.()||`layer-${Date.now()}-${Math.random()}`
  ownedTokens.add(token);dialog.dataset.mobileOverlayToken=token
  history.pushState({...state(),[marker]:token},'',location.href)
 }
 function closed(){
  syncScrollLock()
  if(!token)return
  const current=token;token=null;delete dialog.dataset.mobileOverlayToken
  if(state()[marker]===current)history.back()
 }
 function pop(){
  const entry=state()[marker]
  if(token&&entry!==token){token=null;delete dialog.dataset.mobileOverlayToken;if(dialog.open)dialog.close('back')}
  else if(!token&&typeof entry==='string'&&ownedTokens.has(entry)){
   if(!phone()){history.back();return}
   token=entry;dialog.dataset.mobileOverlayToken=token
   if(restore)restore();else{dialog.returnValue='';dialog.showModal()}
  }
  syncScrollLock()
 }
 const click=(event:MouseEvent)=>{
  if(!token||event.defaultPrevented||event.button!==0||event.metaKey||event.ctrlKey||event.shiftKey||event.altKey)return
  const link=(event.target as HTMLElement).closest<HTMLAnchorElement>('a[href]')
  if(!link||link.target||link.hasAttribute('download'))return
  const url=new URL(link.href,location.href);if(url.origin!==location.origin)return
  event.preventDefault();void dismissMobileOverlay(dialog).then(()=>location.assign(url.href))
 }
 const submit=(event:SubmitEvent)=>{
  if(!token||event.defaultPrevented||!(event.target instanceof HTMLFormElement))return
  const form=event.target,button=event.submitter
  event.preventDefault();void dismissMobileOverlay(dialog,'apply').then(()=>form.requestSubmit(button instanceof HTMLButtonElement||button instanceof HTMLInputElement?button:undefined))
 }
 const observer=new MutationObserver(()=>{if(dialog.open)opened();else closed()})
 observer.observe(dialog,{attributes:true,attributeFilter:['open']})
 dialog.addEventListener('close',closed);dialog.addEventListener('click',click);dialog.addEventListener('submit',submit);window.addEventListener('popstate',pop)
 const resize=()=>{syncScrollLock();if(dialog.open)opened()};window.addEventListener('resize',resize,{passive:true})
 const cleanup=()=>{if(disposed)return;disposed=true;if(token)closed();observer.disconnect();dialog.removeEventListener('close',closed);dialog.removeEventListener('click',click);dialog.removeEventListener('submit',submit);window.removeEventListener('popstate',pop);window.removeEventListener('resize',resize);registrations.delete(dialog);syncScrollLock()}
 registrations.set(dialog,cleanup);if(dialog.open)opened();return cleanup
}
export function initMobileViewport(){
 // A reload cannot restore an interrupted temporary task's in-memory draft.
 if(history.state?.painterMobileOverlay){const state={...history.state};delete state.painterMobileOverlay;history.replaceState(state,'',location.href)}
 const update=()=>{
  const viewport=visualViewport
  document.documentElement.style.setProperty('--mobile-viewport-height',`${viewport?.height||innerHeight}px`)
  document.documentElement.style.setProperty('--mobile-viewport-top',`${viewport?.offsetTop||0}px`)
  const editable=document.activeElement?.matches('input,textarea,[contenteditable=true],.cm-content')
  document.documentElement.toggleAttribute('data-mobile-keyboard',phone()&&!!editable&&!!viewport&&innerHeight-viewport.height>120)
 }
 visualViewport?.addEventListener('resize',update,{passive:true});visualViewport?.addEventListener('scroll',update,{passive:true});window.addEventListener('resize',update,{passive:true});document.addEventListener('focusin',update);document.addEventListener('focusout',()=>requestAnimationFrame(update));update()
}
