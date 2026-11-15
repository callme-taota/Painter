import {t,locale} from './i18n'
import {observeInfinite} from './infiniteScroll'
export function initPublicInfinite(){
 const foundSentinel=document.querySelector<HTMLElement>('[data-infinite-next]'),foundContainer=document.querySelector<HTMLElement>('[data-infinite-items]')
 if(!foundSentinel||!foundContainer)return
 const sentinel=foundSentinel,container=foundContainer
 let next=sentinel.dataset.infiniteNext||'',busy=false,failed=false,changed=false
 const seen=new Set<string>(),abort=new AbortController()
 const observer=observeInfinite(sentinel,()=>void load())
 async function load(retry=false){
  if(!next||busy||(failed&&!retry)||document.hidden)return
  const url=new URL(next,location.href)
  if(url.origin!==location.origin||url.pathname!==location.pathname||seen.has(url.href)){next='';return}
  busy=true;failed=false;sentinel.textContent=t('正在载入…');sentinel.setAttribute('aria-busy','true')
  try{
   const response=await fetch(url,{signal:abort.signal,headers:{Accept:'text/html','Accept-Language':locale}})
   if(!response.ok){changed=response.status===409;throw Error('load failed')}
   const doc=new DOMParser().parseFromString(await response.text(),'text/html'),incoming=doc.querySelector('[data-infinite-items]')
   if(!incoming||doc.body.dataset.pageKind!==document.body.dataset.pageKind)throw Error('invalid list')
   seen.add(url.href)
   const children=Array.from(incoming.children)
   const key=(child:Element)=>child.getAttribute('data-list-key')||child.id||child.querySelector<HTMLAnchorElement>('h2 a')?.getAttribute('href')||''
   const keys=new Set(Array.from(container.children).map(key).filter(Boolean))
   container.append(...children.filter(child=>{const id=key(child);if(id&&keys.has(id))return false;if(id)keys.add(id);return true}).map(child=>document.importNode(child,true)))
   const count=document.querySelector('[data-list-count]');if(count)count.textContent=String(container.children.length)
   next=doc.querySelector<HTMLElement>('[data-infinite-next]')?.dataset.infiniteNext||''
   sentinel.textContent=next?'':t('已显示全部内容');sentinel.dataset.infiniteNext=next
   document.dispatchEvent(new CustomEvent('painter:list-appended',{detail:{container}}))
  }catch(error){
   if(abort.signal.aborted)return
   failed=true;sentinel.textContent=t('后续内容未能载入，已显示的内容仍然保留。 ')
   const button=document.createElement('button');button.type='button';button.className='quiet';button.textContent=changed?t('内容已更新，重新载入列表'):t('重试载入');button.addEventListener('click',()=>{if(changed){const start=new URL(location.href);start.searchParams.delete('cursor');location.assign(start.href)}else void load(true)});sentinel.append(button)
  }finally{busy=false;sentinel.setAttribute('aria-busy','false');if(!failed&&next)observer.check()}
 }
 addEventListener('pagehide',event=>{if(!event.persisted){abort.abort();observer.stop()}})
 addEventListener('pageshow',()=>observer.check())
}
