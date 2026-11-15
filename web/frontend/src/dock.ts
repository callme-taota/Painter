/** Two stable entry points, with nested tools that keep their own focus scope. */
export function initDock(){
 const dock=document.querySelector<HTMLElement>('[data-site-dock]')
 if(!dock)return
 const menus=Array.from(dock.querySelectorAll<HTMLDetailsElement>('details'))
 const roots=menus.filter(menu=>menu.parentElement===dock)
 const phone=matchMedia('(max-width:720px)')
 function fit(menu:HTMLDetailsElement){
  if(phone.matches||!menu.open||!roots.includes(menu))return
  const panel=menu.querySelector<HTMLElement>('.dock-popover')
  if(!panel)return
  panel.style.removeProperty('--dock-popover-offset')
  const box=panel.getBoundingClientRect()
  const shift=box.top<12?12-box.top:box.bottom>innerHeight-12?innerHeight-12-box.bottom:0
  if(shift)panel.style.setProperty('--dock-popover-offset',`${shift}px`)
 }
 menus.forEach(menu=>{
  menu.addEventListener('toggle',()=>{
   if(!menu.open)return
   for(const other of menus)if(other!==menu&&!other.contains(menu)&&!menu.contains(other))other.open=false
   const root=roots.find(candidate=>candidate===menu||candidate.contains(menu))
   if(root)requestAnimationFrame(()=>fit(root))
  })
  menu.addEventListener('keydown',event=>{
   if(event.defaultPrevented)return
   const summary=menu.querySelector<HTMLElement>(':scope>summary')
   if(!summary)return
   // After a nested disclosure closes, Escape belongs to the nearest still-open
   // ancestor, even while focus remains on the nested summary.
   if(event.key==='Escape'&&menu.open&&(event.target as Element).closest('details[open]')===menu){event.preventDefault();event.stopPropagation();menu.open=false;summary.focus();return}
   if((event.target as Element).closest('details')!==menu)return
   if(!roots.includes(menu)||!['ArrowDown','ArrowUp','Home','End'].includes(event.key))return
   const options=Array.from(menu.querySelectorAll<HTMLElement>('a[href],button:not(:disabled),summary')).filter(item=>item!==summary&&!item.hidden&&item.getClientRects().length>0)
   if(event.target===summary){
    if(!['ArrowDown','ArrowUp'].includes(event.key))return
    event.preventDefault();menu.open=true;fit(menu)
    const visible=Array.from(menu.querySelectorAll<HTMLElement>('a[href],button:not(:disabled),summary')).filter(item=>item!==summary&&!item.hidden&&item.getClientRects().length>0)
    visible[event.key==='ArrowDown'?0:visible.length-1]?.focus();return
   }
   const index=options.indexOf(document.activeElement as HTMLElement)
   if(index<0)return
   event.preventDefault()
   const next=event.key==='Home'?0:event.key==='End'?options.length-1:(index+(event.key==='ArrowDown'?1:-1)+options.length)%options.length
   options[next]?.focus()
  })
 })
 document.addEventListener('pointerdown',event=>{
  if(!(event.target instanceof Node))return
  for(const root of roots)if(root.open&&!root.contains(event.target))root.open=false
 })
 roots.forEach(root=>root.addEventListener('focusout',event=>{
  if(event.relatedTarget instanceof Node&&!root.contains(event.relatedTarget))root.open=false
 }))
 addEventListener('resize',()=>roots.forEach(fit),{passive:true})
 phone.addEventListener('change',()=>roots.forEach(menu=>menu.open=false))
}
