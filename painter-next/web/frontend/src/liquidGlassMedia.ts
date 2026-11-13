/**
 * Aligned decorative image copies + ordinary SVG CSS filters, following the
 * cross-browser technique documented by https://github.com/gentpan/liquidglass.
 * This implementation does not capture DOM, read pixels, or fetch resources.
 */
const PRIVATE_PATH=/^\/(?:login|register|setup|studio|me|auth|oidc|api|account|settings|preview|drafts?|reset-password|forgot-password)(?:\/|$)/i
const PRIVATE_CONTENT='input[type="password"],input[type="file"],input[autocomplete="one-time-code"],textarea,[contenteditable]:not([contenteditable="false"]),[data-private],[data-glass-capture="off"]'
const MAX_IMAGES=6
export type MobileMediaGlassStatus='active'|'idle'|'pending'|'blocked'
export interface MobileMediaGlassOptions{filter:string;enabled?:boolean}
export interface MobileMediaGlassRenderer{
 update(options:MobileMediaGlassOptions):void
 refresh():void
 destroy():void
}
type Copy={wrapper:HTMLSpanElement;image:HTMLImageElement;url:string;overlays:HTMLSpanElement[]}
type Surface={target:HTMLElement;holder:HTMLElement;source:HTMLElement;options:MobileMediaGlassOptions;copies:Map<HTMLImageElement,Copy>;report:(status:MobileMediaGlassStatus)=>void;dead:boolean;observer:MutationObserver}
type Overlay={rect:DOMRect;background:string;opacity:number;borderRadius:string}
type Photo={image:HTMLImageElement;rect:DOMRect;visible:DOMRect;style:CSSStyleDeclaration;opacity:number;overlays:Overlay[]}
const surfaces=new Set<Surface>()
let frame=0,listening=false,followUntil=0
const intersect=(a:DOMRect,b:DOMRect)=>{
 const left=Math.max(a.left,b.left),top=Math.max(a.top,b.top),right=Math.min(a.right,b.right),bottom=Math.min(a.bottom,b.bottom)
 return new DOMRect(left,top,Math.max(0,right-left),Math.max(0,bottom-top))
}
const overlaps=(a:DOMRect,b:DOMRect)=>a.left<b.right&&a.right>b.left&&a.top<b.bottom&&a.bottom>b.top
const write=(element:HTMLElement,key:string,value:string)=>{if(element.style.getPropertyValue(key)!==value)element.style.setProperty(key,value)}
const px=(value:number)=>`${Math.round(value*100)/100}px`
const clips=(value:string)=>/^(?:hidden|clip|scroll|auto)$/.test(value)
function clipAxes(rect:DOMRect,bounds:DOMRect,x:boolean,y:boolean):DOMRect{
 const left=x?Math.max(rect.left,bounds.left):rect.left,right=x?Math.min(rect.right,bounds.right):rect.right
 const top=y?Math.max(rect.top,bounds.top):rect.top,bottom=y?Math.min(rect.bottom,bounds.bottom):rect.bottom
 return new DOMRect(left,top,Math.max(0,right-left),Math.max(0,bottom-top))
}
function path(rectangles:DOMRect[],lens:DOMRect):string{
 const n=(value:number)=>String(Math.round(value*100)/100)
 return `path("${rectangles.map(rect=>`M${n(rect.left-lens.left)} ${n(rect.top-lens.top)}H${n(rect.right-lens.left)}V${n(rect.bottom-lens.top)}H${n(rect.left-lens.left)}Z`).join('')||'M0 0Z'}")`
}
function permitted(surface:Surface):boolean{
 let pathname:string
 try{pathname=decodeURIComponent(location.pathname)}catch{return false}
 return surface.options.enabled!==false&&matchMedia('(max-width:720px)').matches
  &&!matchMedia('(prefers-reduced-transparency:reduce),(prefers-contrast:more),(forced-colors:active)').matches
  &&!document.documentElement.classList.contains('reduce-transparency')&&!document.body.classList.contains('reduce-transparency')
  &&!PRIVATE_PATH.test(pathname)&&!surface.source.matches(PRIVATE_CONTENT)&&!surface.source.querySelector(PRIVATE_CONTENT)
  &&surface.target.isConnected&&surface.holder.isConnected&&surface.source.isConnected
  &&!surface.source.contains(surface.target)&&!surface.source.contains(surface.holder)
  &&surface.target.matches('.site-header,.mobile-primary-nav,#site-search')
  &&CSS.supports('clip-path','path("M0 0H1V1H0Z")')
  &&/^url\(["']?#painter-liquid-glass-\d+["']?\)$/.test(surface.options.filter)
}
function clear(surface:Surface){
 for(const copy of surface.copies.values())copy.wrapper.remove()
 surface.copies.clear();write(surface.holder,'opacity','0')
}
function schedule(){if(!frame&&surfaces.size)frame=requestAnimationFrame(paint)}
function listen(){
 if(listening)return
 listening=true
 addEventListener('scroll',schedule,{passive:true,capture:true})
 addEventListener('resize',schedule,{passive:true})
}
function unlisten(){
 if(surfaces.size||!listening)return
 listening=false;cancelAnimationFrame(frame);frame=0;followUntil=0
 removeEventListener('scroll',schedule,true);removeEventListener('resize',schedule)
}
/** Read every geometry before changing decorative layers, shared by both bars. */
function paint(){
 frame=0
 const viewport=new DOMRect(0,0,innerWidth,innerHeight)
 const visible=new Map<Surface,DOMRect>(),photos=new Map<HTMLElement,Photo[]>()
 const styles=new WeakMap<Element,CSSStyleDeclaration>()
 const css=(element:Element)=>{let result=styles.get(element);if(!result){result=getComputedStyle(element);styles.set(element,result)}return result}
 for(const surface of surfaces){
  if(!permitted(surface))continue
  const rect=surface.target.getBoundingClientRect(),style=css(surface.target)
  if(rect.width&&rect.height&&overlaps(rect,viewport)&&style.visibility!=='hidden'&&Number(style.opacity)>.01)visible.set(surface,rect)
 }
 for(const surface of visible.keys()){
  if(photos.has(surface.source))continue
  const lenses=[...visible].filter(([other])=>other.source===surface.source).map(([,rect])=>rect)
  const matches:Photo[]=[]
  for(const image of surface.source.querySelectorAll<HTMLImageElement>('img')){
   if(!image.complete||!image.naturalWidth||image.closest('[data-private],[data-glass-capture="off"]'))continue
   const rect=image.getBoundingClientRect()
   if(!rect.width||!rect.height||!lenses.some(lens=>overlaps(rect,lens)))continue
   const style=css(image)
   if(style.visibility==='hidden'||style.display==='none')continue
   let clipped=rect,opacity=1,rotated=false,branchZ=0
   const overlays:Overlay[]=[]
   for(let element:HTMLElement|null=image;element&&element!==surface.source.parentElement;element=element.parentElement){
    const current=css(element);opacity*=Number(current.opacity)
    if(current.visibility==='hidden'||current.display==='none'){opacity=0;break}
    if(current.transform!=='none'){
     try{const matrix=new DOMMatrixReadOnly(current.transform);if(Math.abs(matrix.b)>.001||Math.abs(matrix.c)>.001||!matrix.is2D)rotated=true}catch{rotated=true}
    }
    if(element!==image){
     const bounds=element.getBoundingClientRect()
     if(clips(current.overflowX)||clips(current.overflowY))clipped=clipAxes(clipped,bounds,clips(current.overflowX),clips(current.overflowY))
     // Reuse full-inset, non-image CSS shading, such as the game's dark gradients.
     // A raised foreground branch (e.g. game logo) already paints above this shade.
     for(const pseudo of ['::after']){
      const shade=getComputedStyle(element,pseudo),shadeZ=Number.parseInt(shade.zIndex)||0
      if(branchZ>shadeZ||shade.content==='none'||shade.content==='normal'||shade.display==='none'||shade.visibility==='hidden'||Number(shade.opacity)<=.01
       ||shade.position!=='absolute'||![shade.top,shade.right,shade.bottom,shade.left].every(value=>value==='0px')
       ||shade.backgroundImage==='none'||/url\(/i.test(shade.backgroundImage))continue
      overlays.push({rect:bounds,background:shade.background,opacity:Number(shade.opacity),borderRadius:shade.borderRadius})
     }
    }
    branchZ=Math.max(branchZ,Number.parseInt(current.zIndex)||0)
   }
   if(opacity>.01&&!rotated&&lenses.some(lens=>overlaps(clipped,lens)))matches.push({image,rect,visible:clipped,style,opacity,overlays:overlays.slice(0,4)})
  }
  photos.set(surface.source,matches)
 }
 for(const surface of surfaces){
  const lens=visible.get(surface)
  if(!lens){clear(surface);surface.report(permitted(surface)?'idle':'blocked');continue}
  const chosen=(photos.get(surface.source)??[]).filter(photo=>overlaps(photo.visible,lens)).slice(-MAX_IMAGES)
  const retained=new Set(chosen.map(photo=>photo.image))
  for(const [image,copy]of surface.copies)if(!retained.has(image)){copy.wrapper.remove();surface.copies.delete(image)}
  let ready=0
  const coverage:DOMRect[]=[]
  for(const photo of chosen){
   let copy=surface.copies.get(photo.image)
   if(!copy){
    const wrapper=document.createElement('span'),image=document.createElement('img')
    wrapper.setAttribute('aria-hidden','true');image.alt='';image.setAttribute('aria-hidden','true')
    image.loading='eager';image.decoding='async';image.referrerPolicy=photo.image.referrerPolicy||'no-referrer'
    wrapper.style.cssText='display:block;position:absolute;inset:0;pointer-events:none;overflow:visible'
    image.style.cssText='position:absolute;display:block;max-width:none!important;max-height:none!important;margin:0;padding:0;border:0;pointer-events:none'
    image.addEventListener('load',schedule);image.addEventListener('error',schedule)
    wrapper.append(image);surface.holder.append(wrapper)
    copy={wrapper,image,url:'',overlays:[]};surface.copies.set(photo.image,copy)
   }
   const url=photo.image.currentSrc||photo.image.src
   if(url!==copy.url){copy.url=url;copy.image.src=url}
   const loaded=copy.image.complete&&copy.image.naturalWidth>0
   write(copy.wrapper,'opacity',loaded?String(photo.opacity):'0')
   write(copy.wrapper,'clip-path',`inset(${px(photo.visible.top-lens.top)} ${px(lens.right-photo.visible.right)} ${px(lens.bottom-photo.visible.bottom)} ${px(photo.visible.left-lens.left)})`)
   write(copy.image,'left',px(photo.rect.left-lens.left));write(copy.image,'top',px(photo.rect.top-lens.top))
   write(copy.image,'width',px(photo.rect.width));write(copy.image,'height',px(photo.rect.height))
   write(copy.image,'object-fit',photo.style.objectFit);write(copy.image,'object-position',photo.style.objectPosition)
   write(copy.image,'border-radius',photo.style.borderRadius)
   for(let index=0;index<photo.overlays.length;index++){
    const overlay=photo.overlays[index]!
    let element=copy.overlays[index]
    if(!element){element=document.createElement('span');element.setAttribute('aria-hidden','true');element.style.cssText='display:block;position:absolute;pointer-events:none';copy.overlays.push(element);copy.wrapper.append(element)}
    write(element,'left',px(overlay.rect.left-lens.left));write(element,'top',px(overlay.rect.top-lens.top))
    write(element,'width',px(overlay.rect.width));write(element,'height',px(overlay.rect.height))
    write(element,'background',overlay.background);write(element,'opacity',String(overlay.opacity));write(element,'border-radius',overlay.borderRadius)
   }
   while(copy.overlays.length>photo.overlays.length)copy.overlays.pop()!.remove()
   if(loaded){ready++;coverage.push(intersect(photo.visible,lens))}
  }
  // Filter first, then clip to the real image intersection. SVG samples may extend
  // past the lens for the bevel, but their output must not cover adjacent prose.
  write(surface.holder,'filter',surface.options.filter)
  write(surface.holder,'clip-path',path(coverage,lens))
  write(surface.holder,'opacity',ready?'1':'0')
  surface.report(ready?'active':chosen.length?'pending':'idle')
 }
 // The reveal header moves by transform after the scroll handler's own RAF.
 // Follow that short transition; there is no ticker once the movement ends.
 if(performance.now()<followUntil)schedule()
}

/**
 * Mirror only existing public photographs behind a visible phone header/nav.
 * Put holder above the native backdrop but below the tint/highlight/controls;
 * keep native backdrop blur enabled, since text/non-image areas are not mirrored.
 */
export function createMobileMediaGlassRenderer(target:HTMLElement,holder:HTMLElement,options:MobileMediaGlassOptions,onStatus:(status:MobileMediaGlassStatus)=>void):MobileMediaGlassRenderer|null{
 const source=document.querySelector<HTMLElement>('main')
 if(!source){onStatus('blocked');return null}
 let status:MobileMediaGlassStatus|undefined
 const surface:Surface={target,holder,source,options,copies:new Map(),dead:false,report(next){if(next!==status){status=next;onStatus(next)}},observer:new MutationObserver(records=>{
  if(records.some(record=>record.target===target&&record.attributeName==='data-scroll-hidden'&&record.oldValue!==target.getAttribute('data-scroll-hidden'))
   &&!matchMedia('(prefers-reduced-motion:reduce)').matches)followUntil=performance.now()+300
  if(!permitted(surface)){clear(surface);surface.report('blocked')}
  schedule()
 })}
 if(!permitted(surface)){onStatus('blocked');return null}
 const previousStyle=holder.getAttribute('style')
 holder.setAttribute('aria-hidden','true')
 holder.style.cssText='display:block;position:absolute;inset:0;pointer-events:none;overflow:visible;opacity:0'
 surface.observer.observe(source,{subtree:true,childList:true,attributes:true,attributeFilter:['src','srcset','sizes','class','style','hidden','type','autocomplete','contenteditable','data-private','data-glass-capture']})
 surface.observer.observe(target,{attributes:true,attributeOldValue:true,attributeFilter:['data-scroll-hidden','open']})
 source.addEventListener('load',schedule,true)
 surfaces.add(surface);listen();schedule()
 return {
  update(next){if(surface.dead)return;surface.options=next;if(!permitted(surface)){clear(surface);surface.report('blocked')}schedule()},
  refresh(){if(!surface.dead)schedule()},
  destroy(){
   if(surface.dead)return
   surface.dead=true;surface.observer.disconnect();source.removeEventListener('load',schedule,true)
   clear(surface);surfaces.delete(surface);unlisten()
   if(previousStyle===null)holder.removeAttribute('style');else holder.setAttribute('style',previousStyle)
  },
 }
}
