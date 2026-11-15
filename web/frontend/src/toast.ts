import {t,locale} from './i18n'
import './toast.css'

export type ToastKind='success'|'error'|'info'|'warning'
export type ToastOptions={id?:string;duration?:number}
type Entry={id:string;kind:ToastKind;message:string;element:HTMLLIElement;text:HTMLParagraphElement;icon:SVGSVGElement;close:HTMLButtonElement;origin:HTMLElement|null;duration:number;remaining:number;started:number;timer?:number;paused:Set<string>}
type ToastService={show:(kind:ToastKind,message:string,options:ToastOptions)=>string;dismiss:(id?:string)=>void}
const singleton=Symbol.for('painter.toast.service')
const labels:Record<ToastKind,string>={success:t('成功'),error:t('错误'),info:t('提示'),warning:t('注意')}
const paths:Record<ToastKind,string[]>={success:['M20 6 9 17l-5-5'],error:['m8 8 8 8m0-8-8 8'],info:['M12 11v6','M12 7h.01'],warning:['M12 8v5','M12 17h.01']}

// Callers provide human-facing copy, never raw responses or credentials.
function safeMessage(message:string){
 const clean=message.replace(/[\u0000-\u0008\u000b\u000c\u000e-\u001f\u007f]/g,'')
  .replace(/\bBearer\s+[A-Za-z0-9._~+/=-]+/gi,t('Bearer [已隐藏]'))
  .replace(/\beyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\b/g,t('[已隐藏的凭据]'))
  .replace(/\b(access_token|refresh_token|id_token|csrf_token|client_secret|password|authorization|api_key|token)(["']?\s*[:=]\s*["']?)[^\s,;"']+/gi,t('$1$2[已隐藏]'))
  .trim()
 const points=Array.from(clean)
 return points.slice(0,4000).join('')+(points.length>4000?'…':'')
}
function duration(kind:ToastKind,message:string,value?:number){
 if(value!==undefined&&Number.isFinite(value))return value<=0?0:Math.min(value,2147483647)
 const length=Array.from(message).length
 if(kind==='error'&&length>240)return 0
 return Math.max(kind==='error'?10000:kind==='warning'?8000:5000,Math.min(30000,length*60))
}
function svg(kind:ToastKind|'close'){
 const node=document.createElementNS('http://www.w3.org/2000/svg','svg')
 for(const [key,value] of Object.entries({viewBox:'0 0 24 24',fill:'none',stroke:'currentColor','stroke-width':'1.8','stroke-linecap':'round','stroke-linejoin':'round','aria-hidden':'true'}))node.setAttribute(key,value)
 if(kind!=='close'){const circle=document.createElementNS(node.namespaceURI,'circle');circle.setAttribute('cx','12');circle.setAttribute('cy','12');circle.setAttribute('r','10');node.append(circle)}
 for(const d of kind==='close'?['m6 6 12 12M18 6 6 18']:paths[kind]){const path=document.createElementNS(node.namespaceURI,'path');path.setAttribute('d',d);node.append(path)}
 return node
}
function createService():ToastService {
 const host=document.createElement('div'),list=document.createElement('ol'),live=document.createElement('div')
 host.className='painter-toast-host';host.dataset.toastHost='';host.hidden=true
 list.className='painter-toast-list';list.setAttribute('aria-label',t('通知'))
 live.className='painter-toast-live';live.setAttribute('role','status');live.setAttribute('aria-live','polite');live.setAttribute('aria-atomic','true')
 host.append(list,live)
 const popover=typeof host.showPopover==='function'
 if(popover)host.setAttribute('popover','manual')
 const entries=new Map<string,Entry>(),dialogs:HTMLDialogElement[]=[]
 let sequence=0,frame=0,announcementTimer=0
 let announcements:{id:string;text:string}[]=[]
 const clearTimer=(entry:Entry)=>{if(entry.timer!==undefined){clearTimeout(entry.timer);entry.timer=undefined;entry.remaining=Math.max(0,entry.remaining-(performance.now()-entry.started))}}
 const arm=(entry:Entry)=>{if(entries.get(entry.id)!==entry||entry.duration===0||entry.paused.size||document.hidden||entry.timer!==undefined)return;if(entry.remaining<=0){dismiss(entry.id);return}entry.started=performance.now();entry.timer=window.setTimeout(()=>dismiss(entry.id),entry.remaining)}
 const pause=(entry:Entry,reason:string)=>{clearTimer(entry);entry.paused.add(reason)}
 const resume=(entry:Entry,reason:string)=>{entry.paused.delete(reason);arm(entry)}
 const closePopover=()=>{if(popover){try{if(host.matches(':popover-open'))host.hidePopover()}catch{}}}
 const route=(promote=false)=>{
  if(!document.body)return
  const opened=Array.from(document.querySelectorAll<HTMLDialogElement>('dialog[open]')).filter(dialog=>{try{return dialog.matches(':modal')}catch{return true}})
  for(let i=dialogs.length-1;i>=0;i--)if(!opened.includes(dialogs[i]!))dialogs.splice(i,1)
  for(const dialog of opened)if(!dialogs.includes(dialog))dialogs.push(dialog)
  const parent=dialogs.at(-1)||document.body
  const focused=host.contains(document.activeElement)?document.activeElement as HTMLElement:null
  if(host.parentElement!==parent){closePopover();parent.append(host);promote=true}
  if(!entries.size){closePopover();host.hidden=true;return}
  host.hidden=false
  if(popover){try{if(promote)closePopover();if(!host.matches(':popover-open'))host.showPopover()}catch{host.removeAttribute('popover')}}
  if(focused&&document.activeElement!==focused)focused.focus({preventScroll:true})
  const viewport=window.visualViewport
  host.style.setProperty('--toast-viewport-top',(viewport?.offsetTop||0)+'px')
  host.style.setProperty('--toast-viewport-height',(viewport?.height||innerHeight)+'px')
  host.style.setProperty('--toast-viewport-bottom',Math.max(0,innerHeight-(viewport?viewport.offsetTop+viewport.height:innerHeight))+'px')
  for(const entry of entries.values())entry.text.tabIndex=entry.text.scrollHeight>entry.text.clientHeight+1?0:-1
 }
 const schedule=()=>{if(!frame)frame=requestAnimationFrame(()=>{frame=0;route()})}
 const announce=(entry:Entry)=>{
  announcements=announcements.filter(item=>item.id!==entry.id)
  announcements.push({id:entry.id,text:t('{kind}：{message}',{kind:labels[entry.kind],message:entry.message})})
  announcements=announcements.slice(-3);clearTimeout(announcementTimer);live.textContent=''
  announcementTimer=window.setTimeout(()=>{live.textContent=announcements.filter(item=>entries.has(item.id)).map(item=>item.text).join(locale==='en'?'. ':'。');announcements=[]},100)
 }
 function dismiss(id?:string){
  const targets=id===undefined?Array.from(entries.values()):entries.has(id)?[entries.get(id)!]:[]
  for(const entry of targets){clearTimer(entry);entries.delete(entry.id);entry.element.remove()}
  route()
 }
 function show(kind:ToastKind,input:string,options:ToastOptions){
  if(typeof input!=='string')return ''
  const message=safeMessage(input);if(!message)return ''
  let entry=options.id?entries.get(options.id):Array.from(entries.values()).find(item=>item.kind===kind&&item.message===message)
  const same=entry?.kind===kind&&entry.message===message
  if(!entry){
   if(entries.size>=3){const oldest=Array.from(entries.values()).find(item=>!item.element.contains(document.activeElement));if(oldest)dismiss(oldest.id)}
   const element=document.createElement('li'),text=document.createElement('p'),icon=svg(kind),close=document.createElement('button')
   element.className='painter-toast';text.className='painter-toast-message';text.dir='auto';icon.classList.add('painter-toast-icon');close.className='painter-toast-close';close.type='button';close.setAttribute('aria-label',t('关闭通知'));close.append(svg('close'))
   let id=options.id||'toast-'+(++sequence)
   while(!options.id&&entries.has(id))id='toast-'+(++sequence)
   entry={id,kind,message,element,text,icon,close,origin:document.activeElement instanceof HTMLElement?document.activeElement:null,duration:0,remaining:0,started:0,paused:new Set()}
   const current=entry
   close.addEventListener('click',()=>{const focused=document.activeElement===close;dismiss(current.id);if(focused&&current.origin?.isConnected&&!host.contains(current.origin))current.origin.focus({preventScroll:true})})
   element.addEventListener('pointerenter',()=>pause(current,'hover'))
   element.addEventListener('pointerleave',()=>resume(current,'hover'))
   element.addEventListener('focusin',()=>pause(current,'focus'))
   element.addEventListener('focusout',event=>{if(!element.contains(event.relatedTarget as Node|null))resume(current,'focus')})
   element.append(icon,text,close);list.append(element);entries.set(entry.id,entry)
  }else{clearTimer(entry);if(entry.kind!==kind){const icon=svg(kind);icon.classList.add('painter-toast-icon');entry.icon.replaceWith(icon);entry.icon=icon}}
  entry.kind=kind;entry.message=message;entry.element.dataset.toastId=entry.id;entry.element.dataset.toastKind=kind;entry.text.textContent=message;entry.duration=duration(kind,message,options.duration);entry.remaining=entry.duration
  entry.element.setAttribute('aria-label',labels[kind]);route(true);schedule();arm(entry)
  if(!same)announce(entry)
  return entry.id
 }
 document.addEventListener('visibilitychange',()=>{for(const entry of entries.values()){if(document.hidden)pause(entry,'hidden');else resume(entry,'hidden')}})
 document.addEventListener('toggle',event=>{if(event.target instanceof HTMLDialogElement)schedule()},true)
 document.addEventListener('close',schedule,true)
 new MutationObserver(records=>{if(!entries.size)return;if(!host.isConnected||records.some(record=>record.type==='attributes'||Array.from(record.addedNodes).some(node=>node instanceof HTMLDialogElement||node instanceof Element&&!!node.querySelector('dialog[open]'))))schedule()}).observe(document.documentElement,{subtree:true,childList:true,attributes:true,attributeFilter:['open']})
 window.addEventListener('resize',schedule,{passive:true})
 window.visualViewport?.addEventListener('resize',schedule,{passive:true})
 window.visualViewport?.addEventListener('scroll',schedule,{passive:true})
 if(document.body)route();else document.addEventListener('DOMContentLoaded',schedule,{once:true})
 return {show,dismiss}
}
function service(){
 if(typeof document==='undefined')return null
 const scope=globalThis as typeof globalThis&{[key:symbol]:ToastService|undefined}
 return scope[singleton]||(scope[singleton]=createService())
}
export function initToast(){service()}
const show=(kind:ToastKind,message:string,options:ToastOptions={})=>service()?.show(kind,message,options)||''
export const toast={success:(message:string,options?:ToastOptions)=>show('success',message,options),error:(message:string,options?:ToastOptions)=>show('error',message,options),info:(message:string,options?:ToastOptions)=>show('info',message,options),warning:(message:string,options?:ToastOptions)=>show('warning',message,options),dismiss:(id?:string)=>service()?.dismiss(id)}
