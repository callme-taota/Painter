/** Keep the small-screen disclosure and existing tool popovers inside the viewport. */
export function initDock(){
 const dock=document.querySelector<HTMLElement>('[data-site-dock]')
 if(!dock)return
 const menus=Array.from(dock.querySelectorAll<HTMLDetailsElement>('details'))
 function fit(menu:HTMLDetailsElement){
  const popover=menu.querySelector<HTMLElement>('.theme-popover,.create-popover,.dock-popover')
  if(!popover||!menu.open)return
  popover.style.removeProperty('--dock-popover-offset')
  const box=popover.getBoundingClientRect()
  const shift=box.top<12?12-box.top:box.bottom>innerHeight-12?innerHeight-12-box.bottom:0
  if(shift)popover.style.setProperty('--dock-popover-offset',`${shift}px`)
 }
 menus.forEach(menu=>menu.addEventListener('toggle',()=>{
  if(!menu.open)return
  menus.forEach(other=>{if(other!==menu)other.open=false})
  fit(menu)
 }))
 addEventListener('resize',()=>menus.forEach(fit),{passive:true})
 const browse=dock.querySelector<HTMLDetailsElement>('.dock-browse')
 if(!browse)return
 const summary=browse.querySelector<HTMLElement>('summary')!
 const items=Array.from(browse.querySelectorAll<HTMLAnchorElement>('.dock-popover a'))
 summary.addEventListener('keydown',event=>{
  if(!['ArrowDown','ArrowUp'].includes(event.key))return
  event.preventDefault();browse.open=true;fit(browse)
  items[event.key==='ArrowDown'?0:items.length-1]?.focus()
 })
 browse.addEventListener('keydown',event=>{
  if(event.key==='Escape'&&browse.open){event.preventDefault();browse.open=false;summary.focus();return}
  // The summary's opener already moved focus; do not handle that key twice.
  if(event.target===summary)return
  const current=items.indexOf(document.activeElement as HTMLAnchorElement)
  if(current<0)return
  let next=current
  if(event.key==='ArrowDown')next=(current+1)%items.length
  else if(event.key==='ArrowUp')next=(current+items.length-1)%items.length
  else if(event.key==='Home')next=0
  else if(event.key==='End')next=items.length-1
  else return
  event.preventDefault();items[next]?.focus()
 })
 document.addEventListener('pointerdown',event=>{if(browse.open&&!browse.contains(event.target as Node))browse.open=false})
 browse.addEventListener('focusout',event=>{if(event.relatedTarget instanceof Node&&!browse.contains(event.relatedTarget))browse.open=false})
}
