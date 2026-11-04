import './homeControlGlass.css'

type Surface = {left:number;right:number;top:number;bottom:number;radius:number;alpha:number}
const clamp=(v:number,min=0,max=1)=>Math.max(min,Math.min(max,v))

// Integrate the visible image behind this small, fixed rail. A visible chapter
// is not enough: its opening mask may still leave the rail over ordinary paper.
function coverage(rail:DOMRect,surfaces:Surface[]):number {
  if(!rail.width||!rail.height)return 0
  let area=0
  const slices=24
  for(let i=0;i<slices;i++){
    const y=rail.top+(i+.5)*rail.height/slices
    const spans:Array<[number,number,number]>=[]
    for(const s of surfaces){
      if(y<s.top||y>s.bottom)continue
      let inset=0
      if(s.radius){
        const edge=Math.min(y-s.top,s.bottom-y)
        if(edge<s.radius)inset=s.radius-Math.sqrt(Math.max(0,s.radius*s.radius-(s.radius-edge)**2))
      }
      const left=Math.max(rail.left,s.left+inset),right=Math.min(rail.right,s.right-inset)
      if(right>left)spans.push([left,right,s.alpha])
    }
    const edges=Array.from(new Set(spans.flatMap(([left,right])=>[left,right]))).sort((a,b)=>a-b)
    let width=0
    for(let edge=1;edge<edges.length;edge++){
      const left=edges[edge-1]!,right=edges[edge]!,middle=(left+right)/2
      let alpha=0
      for(const [start,end,strength]of spans)if(middle>=start&&middle<=end)alpha=Math.max(alpha,strength)
      width+=(right-left)*alpha
    }
    area+=width
  }
  return clamp(area/(rail.width*slices))
}

export function initHomeControlGlass(){
  if(document.body.dataset.pageKind!=='home')return
  const control=document.querySelector<HTMLElement>('.floating-controls')
  const media=Array.from(document.querySelectorAll<HTMLElement>('[data-game-window],.home-record-stage,.point-record-surface'))
  if(!control||!media.length||control.dataset.mediaGlassReady==='true')return
  const rail:HTMLElement=control
  if(!CSS.supports('background','color-mix(in srgb,white 50%,transparent)'))return
  if(!CSS.supports('backdrop-filter','blur(1px)')&&!CSS.supports('-webkit-backdrop-filter','blur(1px)'))return
  rail.dataset.mediaGlassReady='true'
  rail.classList.add('has-media-glass')
  let queued=0,last=-1,near=true
  const visible=new Set<Element>()
  function render(){
    queued=0
    if(document.hidden)return
    const surfaces:Surface[]=[]
    const styles=new Map<HTMLElement,CSSStyleDeclaration>(),opacities=new Map<HTMLElement,number>()
    const style=(element:HTMLElement)=>{let value=styles.get(element);if(!value){value=getComputedStyle(element);styles.set(element,value)}return value}
    const opacity=(element:HTMLElement):number=>{
      const cached=opacities.get(element);if(cached!==undefined)return cached
      const computed=style(element)
      const own=element.hidden||computed.display==='none'||computed.visibility==='hidden'||computed.visibility==='collapse'?0:clamp(Number.parseFloat(computed.opacity)||0)
      const value=own*(element.parentElement?opacity(element.parentElement):1)
      opacities.set(element,value);return value
    }
    const railBox=rail.getBoundingClientRect()
    for(const element of media){
      const rect=element.getBoundingClientRect()
      if(!rect.width||!rect.height||rect.bottom<0||rect.top>innerHeight)continue
      let alpha=opacity(element)
      if(alpha<=.001)continue
      const cover=element.classList.contains('point-record-surface')
      if(cover){
        const image=element.querySelector<HTMLImageElement>('img')
        if(!image||!image.complete||!image.naturalWidth)continue
        alpha=opacity(image)
        if(alpha<=.001)continue
      }else{
        // The native collection is underneath the opaque pinned work stage
        // until its real cover sheet takes over. Geometry alone cannot see it.
        const collection=element.closest('.has-work-record-bridge')
        const previous=collection?.previousElementSibling
        const work=previous?.querySelector<HTMLElement>('.home-work-stage')
        if(work&&opacity(work)>.999){
          const box=work.getBoundingClientRect()
          if(box.left<=railBox.left&&box.right>=railBox.right&&box.top<=railBox.top&&box.bottom>=railBox.bottom)continue
        }
      }
      let left=0,right=0,top=0,bottom=0,radius=0
      if(element.hasAttribute('data-game-window')&&element.closest('.home-game-story.is-enhanced')){
        const inset=(side:string,axis:string,size:number)=>clamp(parseFloat(element.style.getPropertyValue('--game-mask-'+side)||element.style.getPropertyValue('--game-mask-'+axis))||0,0,100)*size/100
        left=inset('left','x',rect.width);right=inset('right','x',rect.width)
        top=inset('top','y',rect.height);bottom=inset('bottom','y',rect.height)
        radius=parseFloat(element.style.getPropertyValue('--game-mask-radius'))||0
      }
      if(cover){
        const corner=style(element).borderTopLeftRadius.split(/[ /]+/).map(value=>parseFloat(value)||0)
        radius=Math.min((corner[0]||0)*rect.width/Math.max(1,element.offsetWidth),(corner[1]??corner[0]??0)*rect.height/Math.max(1,element.offsetHeight))
      }
      surfaces.push({left:rect.left+left,right:rect.right-right,top:rect.top+top,bottom:rect.bottom-bottom,radius:Math.max(0,Math.min(radius,(rect.width-left-right)/2,(rect.height-top-bottom)/2)),alpha})
    }
    const value=coverage(railBox,surfaces)
    if(Math.abs(value-last)<.002)return
    last=value
    rail.style.setProperty('--painter-media-glass',value.toFixed(4))
    rail.dataset.mediaGlassAmount=value.toFixed(4)
  }
  function schedule(){if(!queued&&!document.hidden)queued=requestAnimationFrame(render)}
  addEventListener('scroll',()=>{if(near)schedule()},{passive:true})
  addEventListener('resize',schedule,{passive:true})
  addEventListener('pageshow',schedule)
  // A first-arrival observer may update the aperture after our scroll frame.
  // Reconcile that completed layout even if the user has stopped scrolling.
  addEventListener('painter:home-media-layout',schedule)
  for(const element of media)if(element.classList.contains('point-record-surface')){
    const image=element.querySelector('img');image?.addEventListener('load',schedule);image?.addEventListener('error',schedule)
  }
  document.addEventListener('visibilitychange',()=>{if(document.hidden){cancelAnimationFrame(queued);queued=0}else schedule()})
  if('IntersectionObserver'in window){
    const observer=new IntersectionObserver(entries=>{for(const entry of entries){if(entry.isIntersecting)visible.add(entry.target);else visible.delete(entry.target)}near=visible.size>0;schedule()},{rootMargin:'100% 0px'})
    media.forEach(element=>observer.observe(element))
  }
  render()
}
