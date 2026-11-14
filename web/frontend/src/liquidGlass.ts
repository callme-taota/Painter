import './liquidGlass.css'

const SVG_NS='http://www.w3.org/2000/svg'
const XLINK_NS='http://www.w3.org/1999/xlink'
const targets='.site-header,.dock-glass,[data-search-glass]'
const active=new WeakMap<HTMLElement,GlassSurface>()
const textures=new Map<string,string>()
let serial=0
let definitions:SVGSVGElement|null=null

type GlassSurface={
 target:HTMLElement
 optics:HTMLSpanElement
 filter:SVGFilterElement|null
 image:SVGFEImageElement|null
 channels:SVGFEDisplacementMapElement[]
 key:string
 id:string
 originalPosition:string
 positioned:boolean
}

const clamp=(value:number,min:number,max:number)=>Math.min(max,Math.max(min,value))
function svg<K extends keyof SVGElementTagNameMap>(name:K,attributes:Record<string,string>={}):SVGElementTagNameMap[K]{
 const element=document.createElementNS(SVG_NS,name)
 for(const [key,value]of Object.entries(attributes))element.setAttribute(key,value)
 return element
}

function supportedRefraction():boolean{
 // Parsing url() is not a rendering-support test: WebKit/Gecko may accept it
 // without filtering their live backdrop. Keep their reliable CSS blur path.
 return /(?:Chrome|Chromium|Edg)\//.test(navigator.userAgent)
  && !/(?:OPR|CriOS|FxiOS|EdgiOS)\//.test(navigator.userAgent)
  && CSS.supports('backdrop-filter','url("#painter-glass-support")')
  && 'SVGFEImageElement'in window
}

function filterFor(surface:GlassSurface):void{
 if(surface.filter)return
 if(!definitions){
  definitions=svg('svg',{'aria-hidden':'true',focusable:'false',width:'0',height:'0'})
  definitions.classList.add('liquid-glass-definitions')
  definitions.append(svg('defs'))
  document.body.append(definitions)
 }
 const filter=svg('filter',{id:surface.id,filterUnits:'userSpaceOnUse',primitiveUnits:'userSpaceOnUse','color-interpolation-filters':'sRGB'})
 const image=svg('feImage',{result:'lens-map',preserveAspectRatio:'none'})
 filter.append(image)
 for(let channel=0;channel<3;channel++){
  const name=['red','green','blue'][channel]!
  const displacement=svg('feDisplacementMap',{in:'SourceGraphic',in2:'lens-map',xChannelSelector:'R',yChannelSelector:'G',result:`refracted-${name}`})
  // Isolate one optical channel, then recombine. Their tiny scale differences
  // produce a subpixel fringe only where the map bends at the rounded rim.
  const matrix=new Array<number>(20).fill(0)
  matrix[channel*6]=1
  matrix[18]=1
  filter.append(displacement,svg('feColorMatrix',{in:`refracted-${name}`,type:'matrix',values:matrix.join(' '),result:name}))
  surface.channels.push(displacement)
 }
 filter.append(
  svg('feBlend',{in:'red',in2:'green',mode:'screen',result:'red-green'}),
  svg('feBlend',{in:'red-green',in2:'blue',mode:'screen',result:'refracted-color'}),
  svg('feGaussianBlur',{in:'refracted-color',stdDeviation:'.18'}),
 )
 definitions.querySelector('defs')!.append(filter)
 surface.filter=filter
 surface.image=image
 surface.optics.style.setProperty('--liquid-glass-filter',`url("#${surface.id}")`)
}

/** A bounded, static normal map; this canvas never contains page pixels. */
function edgeTexture(width:number,height:number,radius:number,strength:number,padding:number):string{
 const key=[width,height,radius,strength,padding].join(':')
 const cached=textures.get(key)
 if(cached)return cached
 const canvas=document.createElement('canvas')
 const fullWidth=width+padding*2,fullHeight=height+padding*2
 const sampling=Math.min(1,1280/fullWidth,256/fullHeight,Math.sqrt(180000/(fullWidth*fullHeight)))
 canvas.width=Math.max(1,Math.ceil(fullWidth*sampling))
 canvas.height=Math.max(1,Math.ceil(fullHeight*sampling))
 const context=canvas.getContext('2d')
 if(!context)throw new Error('The glass normal map is unavailable')
 const bitmap=context.createImageData(canvas.width,canvas.height)
 const halfWidth=width/2,halfHeight=height/2
 const bevel=clamp(Math.min(width,height)*.23,6,18)
 for(let row=0;row<canvas.height;row++)for(let column=0;column<canvas.width;column++){
  const x=(column+.5)*fullWidth/canvas.width-padding-halfWidth
  const y=(row+.5)*fullHeight/canvas.height-padding-halfHeight
  const qx=Math.abs(x)-halfWidth+radius,qy=Math.abs(y)-halfHeight+radius
  const cornerX=Math.max(qx,0),cornerY=Math.max(qy,0)
  const cornerLength=Math.hypot(cornerX,cornerY)
  const distance=Math.min(Math.max(qx,qy),0)+cornerLength-radius
  let normalX=0,normalY=0,bend=0
  if(distance<=0&&distance>-bevel){
   if(cornerLength>.0001){normalX=cornerX/cornerLength*Math.sign(x);normalY=cornerY/cornerLength*Math.sign(y)}
   else if(qx>qy)normalX=Math.sign(x)
   else normalY=Math.sign(y)
   const depth=clamp(-distance/bevel,0,1)
   // A convex rim approaches a flat optical center smoothly, without noise,
   // warping the foreground, or displacing content across the entire lens.
   bend=(1-depth)**2*(1+depth*.7)
  }
  const offset=(row*canvas.width+column)*4
  bitmap.data[offset]=Math.round(127.5+normalX*bend*127.5)
  bitmap.data[offset+1]=Math.round(127.5+normalY*bend*127.5)
  bitmap.data[offset+2]=128
  bitmap.data[offset+3]=255
 }
 context.putImageData(bitmap,0,0)
 const uri=canvas.toDataURL('image/png')
 // Viewport resizing must not create an unbounded collection of old textures.
 if(textures.size>=16)textures.delete(textures.keys().next().value!)
 textures.set(key,uri)
 return uri
}

function measure(surface:GlassSurface):void{
 const {target}=surface
 const width=Math.round(target.clientWidth),height=Math.round(target.clientHeight)
 if(!width||!height)return
 const computed=getComputedStyle(target)
 const radius=clamp(parseFloat(computed.borderTopLeftRadius)||0,0,Math.min(width,height)/2)
 const isDock=target.classList.contains('dock-glass')&&!target.classList.contains('site-header')
 const strength=clamp(Math.min(width,height)*(isDock?.38:.29),9,isDock?22:19)
 const padding=Math.ceil(strength+3)
 const key=[width,height,radius].join(':')
 if(key===surface.key&&surface.filter)return
 filterFor(surface)
 const filter=surface.filter!,image=surface.image!
 for(const [name,value]of Object.entries({x:-padding,y:-padding,width:width+padding*2,height:height+padding*2})){
  filter.setAttribute(name,String(value));image.setAttribute(name,String(value))
 }
 const uri=edgeTexture(width,height,radius,strength,padding)
 image.setAttribute('href',uri)
 image.setAttributeNS(XLINK_NS,'href',uri)
 surface.channels.forEach((channel,index)=>channel.setAttribute('scale',String(strength*2+(index-1)*1.35)))
 surface.key=key
 target.dataset.glassTextureSize=`${width}x${height}`
}

/**
 * Enhances only the public header and explicitly marked command surfaces.
 * Existing controls, focus, positions and V28 scroll ownership are untouched.
 * Returns a cleanup function; repeated calls never duplicate a live surface.
 */
export function initLiquidGlass(root:ParentNode=document):()=>void{
 const candidates=Array.from(root.querySelectorAll<HTMLElement>(targets))
 if(root instanceof HTMLElement&&root.matches(targets))candidates.unshift(root)
 const surfaces:GlassSurface[]=[]
 const supportsBackdrop=CSS.supports('backdrop-filter','blur(1px)')||CSS.supports('-webkit-backdrop-filter','blur(1px)')
 const refraction=supportedRefraction()
 // Native compositing owns phone scrolling. Do not rasterise/mirror content or
 // introduce a main-thread displacement pass to chase the composited viewport.
 const phone=matchMedia('(max-width:720px)')
 const reducedTransparency=matchMedia('(prefers-reduced-transparency:reduce)')
 const increasedContrast=matchMedia('(prefers-contrast:more)')
 const forcedColors=matchMedia('(forced-colors:active)')
 let queued=0,destroyed=false
 for(const target of candidates){
  if(active.has(target)||target.dataset.liquidGlass==='off')continue
  const optics=document.createElement('span')
  optics.className='liquid-glass-optics'
  optics.setAttribute('aria-hidden','true')
  const backdrop=document.createElement('span'),shine=document.createElement('span')
  backdrop.className='liquid-glass-backdrop';shine.className='liquid-glass-shine'
  optics.append(backdrop,shine)
  const positioned=getComputedStyle(target).position==='static'
  const surface:GlassSurface={target,optics,filter:null,image:null,channels:[],key:'',id:`painter-liquid-glass-${++serial}`,positioned,originalPosition:target.style.position}
  if(positioned)target.style.position='relative'
  target.prepend(optics)
  target.classList.add('liquid-glass-surface')
  target.dataset.liquidGlassReady='true'
  active.set(target,surface);surfaces.push(surface)
 }
 if(!surfaces.length)return()=>{}
 function solid():boolean{
  return !supportsBackdrop||reducedTransparency.matches||increasedContrast.matches||forcedColors.matches
   ||document.documentElement.classList.contains('reduce-transparency')||document.body.classList.contains('reduce-transparency')
 }
 function update(){
  queued=0
  if(destroyed)return
  const opaque=solid()
  for(const surface of surfaces){
   const mode=opaque?'solid':!phone.matches&&refraction&&surface.target.dataset.glassRefraction!=='off'?'refract':'blur'
   if(mode==='refract'){
    try{measure(surface)}catch{surface.target.dataset.liquidGlassMode='blur';continue}
   }
   surface.target.dataset.liquidGlassMode=mode
  }
 }
 function schedule(){if(!queued&&!destroyed)queued=requestAnimationFrame(update)}
 const observer='ResizeObserver'in window?new ResizeObserver(schedule):null
 surfaces.forEach(surface=>observer?.observe(surface.target))
 const mutations=new MutationObserver(schedule)
 mutations.observe(document.documentElement,{attributes:true,attributeFilter:['class','data-theme']})
 mutations.observe(document.body,{attributes:true,attributeFilter:['class']})
 surfaces.forEach(surface=>mutations.observe(surface.target,{attributes:true,attributeFilter:['data-glass-refraction']}))
 const queries=[reducedTransparency,increasedContrast,forcedColors,phone]
 queries.forEach(query=>query.addEventListener('change',schedule))
 addEventListener('resize',schedule,{passive:true})
 addEventListener('pageshow',schedule)
 update()
 return()=>{
  if(destroyed)return
  destroyed=true;cancelAnimationFrame(queued);observer?.disconnect();mutations.disconnect()
  queries.forEach(query=>query.removeEventListener('change',schedule))
  removeEventListener('resize',schedule);removeEventListener('pageshow',schedule)
  for(const surface of surfaces){
   surface.optics.remove();surface.filter?.remove();surface.target.classList.remove('liquid-glass-surface')
   if(surface.positioned)surface.target.style.position=surface.originalPosition
   delete surface.target.dataset.liquidGlassReady;delete surface.target.dataset.liquidGlassMode;delete surface.target.dataset.glassTextureSize
   active.delete(surface.target)
  }
  if(definitions&&!definitions.querySelector('filter')){definitions.remove();definitions=null;textures.clear()}
 }
}
