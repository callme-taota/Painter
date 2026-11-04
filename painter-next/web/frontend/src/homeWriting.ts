// The public HTML remains a native horizontal shelf until enhanced. Desktop
// vertical scrolling drives the shelf; touch and reduced-motion retain scrolling.
export function initHomeWriting(){
 const section=document.querySelector<HTMLElement>('[data-home-writing]'),stage=section?.querySelector<HTMLElement>('.home-writing-stage'),viewport=section?.querySelector<HTMLElement>('.home-writing-viewport'),track=section?.querySelector<HTMLElement>('.home-writing-track')
 if(!section||!stage||!track||!viewport||track.children.length<2)return
 const media=matchMedia('(min-width:1000px) and (prefers-reduced-motion:no-preference)'),configuredMotion=document.body.dataset.homeMotion!=='false';let distance=0,queued=false
 function update(){queued=false;if(!media.matches||!configuredMotion)return;const p=Math.max(0,Math.min(distance,-section!.getBoundingClientRect().top));track!.style.transform=`translate3d(${-p}px,0,0)`}
 function queue(){if(!queued){queued=true;requestAnimationFrame(update)}}
 function measure(){section!.classList.toggle('is-scroll-shelf',media.matches&&configuredMotion);track!.style.transform='';section!.style.height='';if(media.matches&&configuredMotion){viewport!.scrollLeft=0;distance=Math.max(0,track!.scrollWidth-viewport!.clientWidth);section!.style.height=(stage!.offsetHeight+distance)+'px';update()}}
 const observer=new ResizeObserver(measure);observer.observe(viewport);observer.observe(stage)
 track.addEventListener('focusin',e=>{if(!media.matches||!configuredMotion)return;const item=(e.target as HTMLElement).closest<HTMLElement>('.home-writing-entry');if(item){const target=Math.min(distance,item.offsetLeft);scrollTo({top:section!.getBoundingClientRect().top+scrollY+target,behavior:'instant'})}})
 media.addEventListener('change',measure);window.addEventListener('scroll',queue,{passive:true});measure()
}
