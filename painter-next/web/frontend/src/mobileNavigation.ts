import {registerMobileOverlay} from './mobileOverlays'
export function initMobileNavigation(){
 const dialog=document.querySelector<HTMLDialogElement>('#mobile-more')
 const dock=document.querySelector<HTMLElement>('[data-site-dock]')
 const nav=document.querySelector<HTMLElement>('[data-mobile-nav]')
 if(!dialog||!dock||!nav)return
 const slot=dialog.querySelector<HTMLElement>('[data-mobile-tools]')!,tools=dock.querySelector<HTMLElement>('.dock-tools')
 const placeholder=document.createComment('desktop tools position')
 if(tools)tools.before(placeholder)
 const query=matchMedia('(max-width:720px)')
 const trigger=nav.querySelector<HTMLElement>('[data-mobile-more-open]')!
 registerMobileOverlay(dialog)
 document.documentElement.classList.add('mobile-chrome-ready')
 function layout(){
  if(query.matches){if(tools)slot.append(tools)}
  else{if(dialog!.open)dialog!.close();if(tools)placeholder.after(tools)}
 }
 trigger.addEventListener('click',event=>{event.preventDefault();if(!dialog.open){dialog.returnValue='';dialog.showModal()}})
 dialog.querySelector('[data-mobile-more-close]')?.addEventListener('click',()=>dialog.close())
 dialog.addEventListener('click',event=>{
  if(event.target===dialog){const box=dialog.getBoundingClientRect();const e=event as MouseEvent;if(e.clientX<box.left||e.clientX>box.right||e.clientY<box.top||e.clientY>box.bottom)dialog.close()}
  if((event.target as HTMLElement).closest('[data-floating-action=top],[data-floating-action=bottom]'))dialog.close()
 })
 const reading=document.querySelector<HTMLElement>('.article-aside')
 if(reading){
  const wrapper=document.createElement('details');wrapper.className='mobile-contents'
  const summary=document.createElement('summary');summary.textContent='在这篇文章中'
  const links=document.createElement('nav');links.setAttribute('aria-label','文章目录')
  for(const item of reading.querySelectorAll<HTMLAnchorElement>('nav a')){const link=item.cloneNode(true) as HTMLAnchorElement;links.append(link)}
  wrapper.append(summary,links);slot.before(wrapper)
 }
 query.addEventListener('change',layout);layout()
}
