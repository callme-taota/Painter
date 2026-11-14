/** Load before the reader reaches the end, including lists inside scrollable pickers. */
export function observeInfinite(element:HTMLElement, load:()=>void){
 let stopped=false,frame=0
 let root:HTMLElement|null=element.parentElement
 while(root&&!/(auto|scroll)/.test(getComputedStyle(root).overflowY))root=root.parentElement
 const check=()=>{
  if(stopped||!element.getClientRects().length)return
  const rect=element.getBoundingClientRect(),bounds=root?.getBoundingClientRect()
  if(rect.bottom>=(bounds?.top??0)&&rect.top<=(bounds?.bottom??innerHeight)+1000)load()
 }
 const schedule=()=>{cancelAnimationFrame(frame);frame=requestAnimationFrame(check)}
 const observer=new IntersectionObserver(entries=>{if(entries.some(entry=>entry.isIntersecting))schedule()},{root,rootMargin:'1000px 0px'})
 observer.observe(element)
 addEventListener('resize',schedule,{passive:true});(root||window).addEventListener('scroll',schedule,{passive:true})
 schedule()
 return {check:schedule,stop(){stopped=true;cancelAnimationFrame(frame);observer.disconnect();removeEventListener('resize',schedule);(root||window).removeEventListener('scroll',schedule)}}
}
