import {t,locale} from './i18n'
import './raycast-search.css'

type Suggestion={kind:'post'|'tag'|'series'|'moment'|'phrase';label:string;url:string}
type SearchItem=Suggestion|{kind:'command';label:string;url:string;detail:string;icon:'post'|'image'|'quote'}
const commands:SearchItem[]=[
  {kind:'command',label:t('浏览文章'),url:'/posts',detail:t('文章列表'),icon:'post'},
  {kind:'command',label:t('查看动态'),url:'/moments',detail:t('日常与图片'),icon:'image'},
  {kind:'command',label:t('读一句片语'),url:'/phrases',detail:t('句子与词语'),icon:'quote'},
]
const paths={search:'<circle cx="10.5" cy="10.5" r="6.5"/><path d="m16 16 4 4"/>',post:'<rect x="5" y="3" width="14" height="18" rx="2"/><path d="M8 8h8M8 12h8M8 16h5"/>',tag:'<path d="M20 13 13 20a2 2 0 0 1-3 0l-8-8V3h9l9 8a2 2 0 0 1 0 2Z"/><circle cx="7" cy="7" r="1"/>',image:'<rect x="3" y="4" width="18" height="16" rx="3"/><path d="m4 17 5-5 4 3 3-5 5 6"/><circle cx="8" cy="9" r="1"/>',quote:'<path d="M10 7H4v6h6V7Zm10 0h-6v6h6V7ZM4 13c0 4 2 5 4 5m6-5c0 4 2 5 4 5"/>',arrow:'<path d="M5 12h14m-5-5 5 5-5 5"/>'}
function icon(name:keyof typeof paths){const el=document.createElementNS('http://www.w3.org/2000/svg','svg');el.classList.add('raycast-search-icon');el.setAttribute('viewBox','0 0 24 24');el.setAttribute('aria-hidden','true');el.innerHTML=paths[name];return el}
function safeSuggestion(value:unknown):Suggestion|undefined{
  if(!value||typeof value!=='object')return
  const item=value as Partial<Suggestion>
  if(!['post','tag','series','moment','phrase'].includes(String(item.kind))||typeof item.label!=='string'||typeof item.url!=='string'||!item.url.startsWith('/')||item.url.startsWith('//'))return
  try{const url=new URL(item.url,location.origin);if(url.origin===location.origin)return{kind:item.kind as Suggestion['kind'],label:item.label,url:url.pathname+url.search+url.hash}}catch{return}
}
export function initSearch(){
  const found=document.querySelector<HTMLDialogElement>('#site-search')
  if(!found?.showModal||found.dataset.raycastSearchReady)return
  const foundInput=found.querySelector<HTMLInputElement>('input[name=q]'),foundList=found.querySelector<HTMLElement>('[role=listbox]'),foundStatus=found.querySelector<HTMLElement>('[data-search-status]'),foundForm=found.querySelector<HTMLFormElement>('form')
  if(!foundInput||!foundList||!foundStatus||!foundForm)return
  const dialog=found,input=foundInput,list=foundList,status=foundStatus,form=foundForm
  dialog.dataset.raycastSearchReady='true';dialog.classList.add('raycast-search')
  // Progressive enhancement keeps the original SSR form and established selectors.
  const row=input.closest('.quick-search-row')
  if(row){const lens=icon('search');lens.classList.add('raycast-search-input-icon');row.prepend(lens);const submit=row.querySelector<HTMLButtonElement>('button');if(submit){submit.setAttribute('aria-label',t('查看完整搜索结果'));submit.title=t('查看完整搜索结果');submit.replaceChildren(icon('arrow'));submit.classList.add('raycast-search-submit')}const close=dialog.querySelector<HTMLButtonElement>('[data-search-close]');if(close){close.type='button';close.title=t('关闭搜索（Esc）');row.append(close)}}
  input.placeholder=t('搜索站内内容、标签，或跳转…');input.setAttribute('aria-describedby','raycast-search-status');status.id='raycast-search-status';status.setAttribute('aria-live','polite')
  const footer=dialog.querySelector<HTMLElement>('.search-dialog-foot'),fullSearch=footer?.querySelector<HTMLAnchorElement>('a')
  if(footer){const hints=document.createElement('div');hints.className='raycast-search-keys';for(const [label,keys,css] of [[t('选择'),['↑','↓'],''],[t('打开'),['Enter'],''],[t('关闭'),['Esc'],'raycast-search-key-close']] as const){const group=document.createElement('span');group.className=css;const text=document.createElement('span');text.textContent=label;group.append(text);for(const key of keys){const kbd=document.createElement('kbd');kbd.textContent=key;group.append(kbd)}hints.append(group)}footer.replaceChildren(hints);if(fullSearch){fullSearch.replaceChildren(document.createTextNode(t('完整搜索')),icon('arrow'));footer.append(fullSearch)}}
  let items:SearchItem[]=[],options:HTMLElement[]=[],active=-1,timer:ReturnType<typeof setTimeout>|undefined,abort:AbortController|undefined,seq=0,composing=false,opener:HTMLElement|null=null
  function select(index:number){active=index;options.forEach((el,i)=>el.setAttribute('aria-selected',String(i===index)));if(index<0)input.removeAttribute('aria-activedescendant');else{input.setAttribute('aria-activedescendant','search-option-'+index);options[index]?.scrollIntoView({block:'nearest'})}}
  function clear(){items=[];options=[];active=-1;list.replaceChildren();input.removeAttribute('aria-activedescendant')}
  function showStatus(text:string,quiet=false){if(matchMedia('(max-width:720px)').matches)text=text.replaceAll(t('按 Enter'),t('点击箭头'));status.textContent=text;status.classList.toggle('raycast-search-status-quiet',quiet)}
  function busy(value:boolean){dialog.toggleAttribute('data-search-loading',value);list.setAttribute('aria-busy',String(value))}
  function cancelPending(){clearTimeout(timer);abort?.abort();seq++}
  function render(suggestions:Suggestion[],q:string){
    clear();items=q?suggestions:[...suggestions,...commands];let previousGroup='';const fragment=document.createDocumentFragment()
    for(const [index,item] of items.entries()){
      const group=item.kind==='command'?t('站内入口'):item.kind==='tag'?t('标签'):item.kind==='series'?t('专栏'):item.kind==='moment'?t('动态'):item.kind==='phrase'?t('片语'):q?t('匹配文章'):t('最近文章')
      if(group!==previousGroup){const heading=document.createElement('li');heading.role='presentation';heading.className='raycast-search-group';heading.textContent=group;fragment.append(heading);previousGroup=group}
      const option=document.createElement('li');option.id='search-option-'+index;option.role='option';option.setAttribute('aria-selected','false');option.dataset.searchKind=item.kind
      const link=document.createElement('a');link.href=item.url;link.tabIndex=-1
      const mark=document.createElement('span');mark.className='raycast-search-item-icon';mark.append(icon(item.kind==='command'?item.icon:item.kind==='tag'?'tag':item.kind==='moment'?'image':item.kind==='phrase'?'quote':'post'))
      const label=document.createElement('span');label.className='raycast-search-label';label.textContent=item.label
      const detail=document.createElement('small');detail.className='suggestion-kind';detail.textContent=item.kind==='command'?item.detail:item.kind==='tag'?t('标签'):item.kind==='series'?t('专栏'):item.kind==='moment'?t('动态'):item.kind==='phrase'?t('片语'):t('文章')
      const action=icon('arrow');action.classList.add('raycast-search-item-action');link.append(mark,label,detail,action);option.append(link)
      option.addEventListener('pointermove',event=>{if(event.pointerType==='mouse'&&!event.buttons)select(index)});options.push(option);fragment.append(option)
    }
    list.append(fragment);if(fullSearch)fullSearch.href=q?'/search?q='+encodeURIComponent(q):'/search'
  }
  async function suggest(){
    abort?.abort();const controller=new AbortController();abort=controller;const at=++seq,q=input.value.trim();clear();busy(true);showStatus(t('正在查找…'))
    try{const response=await fetch('/api/v1/search/suggestions?q='+encodeURIComponent(q),{signal:controller.signal,headers:{'Accept-Language':locale}});if(!response.ok)throw new Error('suggestions unavailable');const data=await response.json();if(at!==seq||input.value.trim()!==q||!dialog.open||composing)return;const suggestions:Suggestion[]=Array.isArray(data.items)?data.items.map(safeSuggestion).filter((x:Suggestion|undefined):x is Suggestion=>!!x):[];render(suggestions,q);showStatus(suggestions.length?(q?t('搜索建议'):t('最近的文章与标签')):(q?t('没有建议，按 Enter 查看完整搜索。'):t('从站内入口开始，或输入关键词。')),suggestions.length>0)}
    catch(error){if((error as Error).name!=='AbortError'&&at===seq&&dialog.open&&!composing){render([],q);showStatus(q?t('建议暂不可用，仍可按 Enter 搜索。'):t('建议暂不可用，可先打开站内页面。'))}}finally{if(at===seq)busy(false)}
  }
  function schedule(){cancelPending();clear();if(fullSearch)fullSearch.href=input.value.trim()?'/search?q='+encodeURIComponent(input.value.trim()):'/search';if(composing){busy(false);showStatus(t('输入完成后查找。'));return}busy(true);showStatus(t('正在查找…'));timer=setTimeout(()=>void suggest(),180)}
  function open(event?:Event){event?.preventDefault();if(dialog.open)return;const close=dialog.querySelector<HTMLElement>('[data-search-close]');if(close)close.textContent=matchMedia('(max-width:720px)').matches?t('取消'):'Esc';opener=document.activeElement instanceof HTMLElement?document.activeElement:null;dialog.showModal();input.focus();input.setAttribute('aria-expanded','true');void suggest()}
  document.querySelectorAll('[data-search-open]').forEach(el=>el.addEventListener('click',open))
  dialog.addEventListener('close',()=>{cancelPending();busy(false);input.setAttribute('aria-expanded','false');if(opener?.isConnected)opener.focus({preventScroll:true})})
  dialog.addEventListener('cancel',event=>{if(composing)event.preventDefault()})
  dialog.querySelector('[data-search-close]')?.addEventListener('click',()=>dialog.close())
  dialog.addEventListener('click',event=>{if(event.target!==dialog)return;const box=dialog.getBoundingClientRect();if(event.clientX<box.left||event.clientX>box.right||event.clientY<box.top||event.clientY>box.bottom)dialog.close()})
  input.addEventListener('input',schedule);input.addEventListener('compositionstart',()=>{composing=true;schedule()});input.addEventListener('compositionend',()=>{composing=false;schedule()})
  input.addEventListener('keydown',event=>{if(event.isComposing||composing)return;if(event.key==='Escape'){event.preventDefault();dialog.close();return}if(event.key==='ArrowDown'||event.key==='ArrowUp'){event.preventDefault();if(items.length)select(active<0?(event.key==='ArrowDown'?0:items.length-1):(active+(event.key==='ArrowDown'?1:-1)+items.length)%items.length)}else if(event.key==='Enter'&&active>=0){event.preventDefault();options[active]?.querySelector<HTMLAnchorElement>('a')?.click()}})
  form.addEventListener('submit',event=>{if(composing||!input.value.trim())event.preventDefault()})
  document.addEventListener('keydown',event=>{if(event.isComposing||event.repeat)return;const editing=(event.target as HTMLElement)?.closest('input,textarea,select,[contenteditable]:not([contenteditable=false]),.cm-editor');if((event.ctrlKey||event.metaKey)&&event.key.toLowerCase()==='k'){if(editing&&!dialog.open)return;event.preventDefault();open()}else if(event.key==='/'&&!editing&&!event.ctrlKey&&!event.metaKey&&!event.altKey){event.preventDefault();open()}})
}
