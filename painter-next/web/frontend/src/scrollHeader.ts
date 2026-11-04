// The header keeps its place in the document; only its painted position changes.
// Browsers without inert retain the visible, server-rendered navigation.
export function initScrollHeader(){
 if(document.body.dataset.pageKind==='home'||!('inert' in HTMLElement.prototype))return
 let header:HTMLElement|null=null,frame=0,anchor=scrollPosition(),previous=anchor,hidden=false
 let originalAria:string|null=null,originalInert=false
 const position=()=>scrollPosition()
 function scrollPosition(){const root=document.scrollingElement||document.documentElement;return Math.min(Math.max(0,window.scrollY),Math.max(0,root.scrollHeight-window.innerHeight))}
 function restoreAccessibility(){if(!header)return;header.inert=originalInert;if(originalAria===null)header.removeAttribute('aria-hidden');else header.setAttribute('aria-hidden',originalAria)}
 function show(instant=false){
  if(!header)return
  if(instant){header.setAttribute('data-scroll-instant','');requestAnimationFrame(()=>header?.removeAttribute('data-scroll-instant'))}
  hidden=false;header.dataset.scrollHidden='false';restoreAccessibility();anchor=previous=position()
 }
 function protectedInteraction(){
  if(!header)return false
  if(header.contains(document.activeElement)||header.querySelector('details[open],[aria-expanded="true"]'))return true
  if(document.querySelector('dialog[open],[role="dialog"][aria-modal="true"]'))return true
  // Public theme menus and custom select menus can live outside the header.
  return Array.from(document.querySelectorAll<HTMLElement>('.floating-theme[open],[role="menu"],[role="listbox"]')).some(el=>el.getClientRects().length>0)
 }
 function attach(){
  const next=document.querySelector<HTMLElement>('.site-header,.studio-mobile-bar')
  if(next===header)return
  if(header){restoreAccessibility();header.removeAttribute('data-scroll-header');header.removeAttribute('data-scroll-hidden');header.removeAttribute('data-scroll-instant');header.style.removeProperty('--scroll-header-offset')}
  header=next
  if(!header)return
  originalAria=header.getAttribute('aria-hidden');originalInert=header.inert
  header.setAttribute('data-scroll-header','');measure();show(true)
 }
 function measure(){if(header){header.style.setProperty('--scroll-header-offset',`${Math.max(0,parseFloat(getComputedStyle(header).top)||0)}px`);document.body.style.setProperty('--scroll-header-height',`${header.getBoundingClientRect().height}px`)}}
 function update(){
  frame=0;attach();if(!header)return
  const y=position()
  if(y<=16||protectedInteraction()){show();return}
  if(y===previous)return
  previous=y
  if(hidden){anchor=Math.max(anchor,y);if(anchor-y>=12)show()}
  else{
   anchor=Math.min(anchor,y)
   if(y-anchor>=32&&y>header.offsetHeight+(parseFloat(getComputedStyle(header).top)||0)+8){
    hidden=true;anchor=y;header.inert=true;header.setAttribute('aria-hidden','true');header.dataset.scrollHidden='true'
   }
  }
 }
 function schedule(){if(!frame)frame=requestAnimationFrame(update)}
 function reset(){attach();measure();show(true)}
 window.addEventListener('scroll',schedule,{passive:true})
 window.addEventListener('resize',reset,{passive:true})
 // Restored pages and initial hash navigation start with useful navigation.
 window.addEventListener('pageshow',()=>{reset();requestAnimationFrame(reset)})
 document.addEventListener('keydown',event=>{if(event.key==='Tab')show(true)},true)
 document.addEventListener('focusin',()=>{if(header?.contains(document.activeElement))show(true)})
 document.addEventListener('focusout',schedule)
 document.addEventListener('toggle',schedule,true)
 document.addEventListener('close',schedule,true)
 // The mobile workbench bar is mounted by Vue after public.ts has loaded, and
 // recreated when its breakpoint changes. Dialog/menu attributes also reveal it.
 new MutationObserver(schedule).observe(document.body,{subtree:true,childList:true,attributes:true,attributeFilter:['open','aria-expanded','aria-modal','hidden']})
 attach()
}
