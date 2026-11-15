import {t} from './i18n'
import './homeWorkJourney.css'
import {createPointMorph, type PointMorphRect} from './homePointMorph'
import {registerHomeSnapSource} from './homeSnap'

const clamp=(value:number)=>Math.max(0,Math.min(1,value))
const smooth=(a:number,b:number,v:number)=>{const t=clamp((v-a)/(b-a));return t*t*(3-2*t)}
const mix=(a:number,b:number,t:number)=>a+(b-a)*t
const rect=(element:HTMLElement,stage:DOMRect):PointMorphRect=>{const r=element.getBoundingClientRect();return{left:r.left-stage.left,top:r.top-stage.top,width:r.width,height:r.height}}
type CardFrame={element:HTMLElement;link:HTMLAnchorElement;box:PointMorphRect;day?:HTMLElement;target?:PointMorphRect;color:string}

// One real card and one real day field, held in one scroll-controlled stage.
export function initHomeWorkJourney(){
 for(const journey of document.querySelectorAll<HTMLElement>('[data-home-work-journey]')){
  if(journey.dataset.workJourneyInitialized)continue
  const story=journey.querySelector<HTMLElement>('[data-home-story]'),projects=journey.querySelector<HTMLElement>('.home-projects'),activity=journey.querySelector<HTMLElement>('.home-activity')
  const cards=Array.from(projects?.querySelectorAll<HTMLElement>('[data-project-card]')||[])
  const first=cards[0]?.querySelector<HTMLAnchorElement>('.project-card-link'),face=story?.querySelector<HTMLElement>('.story-leaf-front')
  if(!story||!projects||!activity||!first||!face||!cards.length)continue
  const intro=story,project=projects,calendar=activity,carrier=first
  const next=journey.nextElementSibling instanceof HTMLElement?journey.nextElementSibling:undefined
  const record=next?.matches('[data-collection-direction="music-games"]')?next.querySelector<HTMLElement>('[data-record-wall]')||undefined:undefined
  const reduced=matchMedia('(prefers-reduced-motion:reduce)')
  const phone=matchMedia('(max-width:720px)')
  const stage=document.createElement('div');stage.className='home-work-stage';stage.dataset.homeWorkStage='true'
  const nativeCalendarScroll=calendar.querySelector<HTMLElement>('.activity-scroll'),nativeScrollLeft=nativeCalendarScroll?.scrollLeft||0
  journey.insertBefore(stage,intro);stage.append(intro,project,calendar)
  // Moving the real section resets nested native scroll positions in browsers.
  if(nativeCalendarScroll)nativeCalendarScroll.scrollLeft=nativeScrollLeft
  journey.dataset.workManaged='true';journey.dataset.workJourneyInitialized='true'
  carrier.dataset.workCarrier='true';cards[0]!.classList.add('work-carrier');project.style.setProperty('--work-card-count',String(cards.length))
  const introFace=document.createElement('div');introFace.className='work-card-intro-face';introFace.setAttribute('aria-hidden','true')
  for(const child of Array.from(face.children))introFace.append(child.cloneNode(true))
  carrier.append(introFace)
  if(!phone.matches)project.querySelectorAll<HTMLImageElement>('.project-owner-avatar').forEach(img=>{img.loading='eager'})
  const caption=record?.querySelector<HTMLElement>('.home-record-caption')?.cloneNode(true) as HTMLElement|undefined
  if(caption){caption.classList.add('work-record-caption');caption.setAttribute('aria-hidden','true');caption.inert=true;stage.append(caption)}
  const jumps=document.createElement('nav');jumps.className='work-content-jumps sr-only';jumps.setAttribute('aria-label',t('直接查看首页内容'))
  const destinations:[string,string,number][]=[['home-projects',t('查看开源项目'),2.25],['home-activity',t('查看发布日历'),4.05],...(record?[['home-music',t('查看唱片墙'),6.02] as [string,string,number]]:[])]
  for(const[id,label]of destinations){const a=document.createElement('a');a.href='#'+id;a.textContent=label;jumps.append(a)}
  journey.insertBefore(jumps,stage)
  let projectChooser:HTMLElement|undefined
  if(cards.length>1){
   const chooser=document.createElement('div');projectChooser=chooser;chooser.inert=true;chooser.className='work-project-chooser';chooser.setAttribute('role','group');chooser.setAttribute('aria-label',t('选择开源项目'))
   const buttons=cards.map((card,index)=>{const button=document.createElement('button');button.type='button';button.dataset.workProjectPick=String(index);button.textContent=card.querySelector('h3')?.textContent||t('项目 {count}',{count:index+1});button.addEventListener('click',()=>card.querySelector<HTMLAnchorElement>('.project-card-link')?.focus({preventScroll:true}));chooser.append(button);return button})
   const selection=()=>{const selected=cards.findIndex(card=>card.classList.contains('is-active'));buttons.forEach((button,index)=>button.setAttribute('aria-pressed',String(index===(selected<0?cards.length-1:selected))))}
   const deck=project.querySelector<HTMLElement>('[data-project-deck]');deck?.after(chooser)
   if(deck)new MutationObserver(selection).observe(deck,{attributes:true,subtree:true,attributeFilter:['class']})
   selection()
  }
  // Do not create eager projection-image copies for the phone's ordinary
  // document. Desktop can initialise its morph when this boundary is crossed.
  let points:ReturnType<typeof createPointMorph>|undefined
  let frames:CardFrame[]=[],source:PointMorphRect={left:0,top:0,width:1,height:1},queued=0,near=true,needsMeasure=true,enhanced=false,measuring=false,focusPin:number|undefined
  const end=record?6:4.45
  const enabled=()=>document.body.dataset.homeMotion!=='false'&&!reduced.matches&&!phone.matches&&CSS.supports('position','sticky')&&CSS.supports('background','color-mix(in srgb,white,black)')
  registerHomeSnapSource(journey,()=>{
   if(!enhanced||!enabled())return[]
   const h=stage.clientHeight,start=scrollY+journey.getBoundingClientRect().top
   return [['intro',.2,.1,.3],['projects',2.25,2.13,2.37],['calendar',4.05,3.93,4.17]].map(([key,at,from,to])=>({key:String(key),top:start+Number(at)*h,from:start+Number(from)*h,to:start+Number(to)*h}))
  })

  function clearCard(card:HTMLElement){card.style.removeProperty('transform');card.style.removeProperty('opacity');card.style.removeProperty('--work-repo-alpha');card.style.removeProperty('--work-native-strength');card.style.removeProperty('--work-card-z');const link=card.querySelector<HTMLElement>('.project-card-link');if(link)for(const p of['background','border-radius','box-shadow'])link.style.removeProperty(p)}
  function reset(){
   enhanced=false;journey.classList.remove('work-journey-ready','work-measuring');next?.classList.remove('has-work-record-bridge');journey.style.removeProperty('--work-height');stage.removeAttribute('style');if(phone.matches){points?.destroy();points=undefined}else points?.reset();cards.forEach(clearCard)
   for(const element of[intro,project,calendar]){element.inert=false;element.removeAttribute('aria-hidden');element.style.removeProperty('opacity');element.style.removeProperty('pointer-events')}
   for(const scene of intro.querySelectorAll<HTMLElement>('[data-story-scene]')){scene.inert=false;scene.removeAttribute('aria-hidden');scene.style.removeProperty('--scene-opacity')}
   if(caption)caption.style.opacity='0';journey.dataset.workPhase='static';delete journey.dataset.workMeasured;focusPin=undefined
  }
  function measure(){
   if(!enabled()){reset();needsMeasure=false;return}
   measuring=true;enhanced=true;journey.classList.add('work-journey-ready','work-measuring');journey.style.setProperty('--work-height',`${(end+1)*100}svh`);next?.classList.toggle('has-work-record-bridge',!!record)
   cards.forEach(clearCard)
   const box=stage.getBoundingClientRect(),w=box.width,h=box.height
   const pw=w<=720?Math.min(w*(h<650?.53:.62),250):Math.min(w*.32,400),ph=pw*1.25
   source={left:w<=720?(w-pw)/2-20:w*.75-pw/2,top:w<=720?h-ph-(h<650?34:74):(h-ph)/2,width:pw,height:ph}
   frames=cards.map(element=>({element,link:element.querySelector<HTMLAnchorElement>('.project-card-link')!,box:rect(element,box),color:''}))
   const morph=points||(points=createPointMorph(stage,calendar,record));morph.measure();const targets=morph.cardTargets(cards.length)
   frames.forEach((frame,index)=>{frame.day=targets[index%targets.length];frame.target=frame.day?morph.dayRect(frame.day):undefined;frame.color=frame.day?getComputedStyle(frame.day).backgroundColor:'var(--subtle)';if(frame.day)frame.link.dataset.workTargetDate=frame.day.dataset.date||''})
   journey.classList.remove('work-measuring');journey.dataset.workMeasured='true';needsMeasure=false;measuring=false
  }
  function phase(u:number){return u<1?'intro':u<2?'paper-project':u<2.65?'projects':u<3.75?'project-calendar':u<4.4?'calendar':u<5.6&&record?'calendar-record':u<end?'records':'handoff'}
  function setScene(element:HTMLElement,alpha:number,interactive:boolean){element.style.opacity=alpha.toFixed(5);const focus=element.contains(document.activeElement);element.inert=!interactive&&!focus;element.setAttribute('aria-hidden',String(alpha<.01&&!focus));element.style.pointerEvents=interactive?'auto':'none'}
  function render(){
   queued=0;if(document.hidden)return
   if(!enabled()){if(enhanced)reset();const warm=document.body.dataset.homeWarmth!=='false'&&!reduced.matches?clamp(scrollY/Math.max(1,document.documentElement.scrollHeight-innerHeight)):0;document.body.style.setProperty('--home-warmth',String(warm));return}
   if(needsMeasure||!enhanced)measure();if(!enhanced||!frames.length)return
   const height=stage.clientHeight,raw=clamp(-journey.getBoundingClientRect().top/Math.max(1,height)/end)*end
   const u=focusPin===undefined?raw:focusPin,paper=smooth(1,2,u),toCalendar=smooth(2.65,3.75,u),toRecords=record?smooth(4.4,5.6,u):0
   journey.dataset.workRawUnits=raw.toFixed(5);journey.dataset.workUnits=u.toFixed(5);journey.dataset.workPhase=phase(u)
   stage.style.opacity=raw>=end&&focusPin===undefined?'0':'1';stage.style.pointerEvents=raw>=end&&focusPin===undefined?'none':'auto';stage.style.setProperty('--work-record-progress',String(toRecords))
   document.body.style.setProperty('--home-warmth',document.body.dataset.homeWarmth!=='false'?smooth(0,2,raw).toFixed(5):'0')
   const introAlpha=1-smooth(.9,1.32,u)
   setScene(intro,introAlpha,u<1.15)
   const scenes=Array.from(intro.querySelectorAll<HTMLElement>('[data-story-scene]'))
   scenes.forEach((scene,index)=>{const alpha=index===0?1-smooth(.35,.72,u):smooth(.35,.72,u);scene.style.setProperty('--scene-opacity',String(alpha));scene.inert=alpha<.1&&!scene.contains(document.activeElement);scene.setAttribute('aria-hidden',String(alpha<.01&&!scene.contains(document.activeElement)))})
   // The project section remains painted so its real first card is the paper.
   project.style.opacity='1';project.inert=!(u>=1.75&&u<2.72)&&!project.contains(document.activeElement);project.setAttribute('aria-hidden',String(u<1.75||u>3.74));project.style.pointerEvents=u>=1.75&&u<2.72?'auto':'none'
   if(projectChooser){const picking=u>=2&&u<2.65;projectChooser.inert=!picking;projectChooser.setAttribute('aria-hidden',String(!picking));projectChooser.style.visibility=picking?'visible':'hidden'}
   project.style.setProperty('--work-project-chrome',String(smooth(1.45,1.95,u)*(1-smooth(2.65,2.95,u))))
   carrier.style.setProperty('--work-intro-alpha',(1-smooth(.12,.76,paper)).toFixed(5))
   points?.render(toCalendar,toRecords)
   setScene(calendar,smooth(0,.15,toCalendar),u>=3.5&&u<4.55)
   calendar.style.setProperty('--work-calendar-chrome',String(smooth(.35,.95,toCalendar)*(1-smooth(0,.4,toRecords))))
   for(const [index,frame]of frames.entries()){
    const box=frame.box,landing=frame.target||box
    const start=index===0?source:{left:source.left+index*7,top:source.top+index*6,width:source.width,height:source.height}
    const appear=index===0?paper:smooth(.35,1,paper)
    const deckBox={left:mix(start.left,box.left,appear),top:mix(start.top,box.top,appear),width:mix(start.width,box.width,appear),height:mix(start.height,box.height,appear)}
    const target={left:mix(deckBox.left,landing.left,toCalendar),top:mix(deckBox.top,landing.top,toCalendar),width:mix(deckBox.width,landing.width,toCalendar),height:mix(deckBox.height,landing.height,toCalendar)}
    const angle=mix(index%2?3:-3,index===0?6:index%2?8:-8,1-paper)*(1-toCalendar)
    const nativeStrength=smooth(.8,1,paper)*(1-smooth(0,.2,toCalendar))
    frame.element.style.setProperty('--work-native-strength',String(nativeStrength))
    frame.element.style.setProperty('--work-hover-rotation',`${-angle}deg`)
    frame.element.style.setProperty('--work-card-z',String(index===0&&paper<.94?10:index+1))
    frame.element.style.transform=`translate3d(calc(${target.left-box.left}px + var(--dodge-x,0px) * var(--work-native-strength,0)),calc(${target.top-box.top}px + var(--card-lift,0px) * var(--work-native-strength,0)),0) rotate(calc(${angle}deg + var(--work-hover-turn,0deg) * var(--work-native-strength,0))) scale(${target.width/box.width},${target.height/box.height})`
    frame.element.style.opacity=String((index===0?1:smooth(.34,.8,paper))*(toCalendar>=.999?0:1))
    frame.element.style.setProperty('--work-repo-alpha',String((index===0?smooth(.3,.9,paper):smooth(.35,.9,paper))*(1-smooth(0,.22,toCalendar))))
    frame.link.style.background=index===0?`color-mix(in srgb,color-mix(in srgb,var(--hero-sheet),var(--paper) ${paper*100}%),${frame.color} ${toCalendar*100}%)`:`color-mix(in srgb,var(--paper),${frame.color} ${toCalendar*100}%)`
    frame.link.style.borderRadius=`${mix(12,3,toCalendar)/(target.width/box.width)}px / ${mix(12,3,toCalendar)/(target.height/box.height)}px`
    frame.link.style.boxShadow=`0 ${mix(24,7,paper)}px ${mix(60,24,paper)}px rgb(0 0 0 / ${.12*(1-toCalendar)})`
   }
   if(caption)caption.style.opacity=smooth(.78,1,toRecords).toFixed(5)
   dispatchEvent(new Event('painter:home-media-layout'))
  }
  function schedule(force=false){if(phone.matches&&!force)return;if(!queued&&!document.hidden&&(near||force))queued=requestAnimationFrame(render)}
  function invalidate(){if(measuring)return;needsMeasure=true;schedule(true)}
  function jump(id:string){const match=destinations.find(x=>x[0]===id);if(!match||!enabled())return false;focusPin=undefined;scrollTo({top:scrollY+journey.getBoundingClientRect().top+match[2]*innerHeight,behavior:'instant'});schedule(true);return true}
  jumps.addEventListener('click',event=>{const target=(event.target as HTMLElement).closest<HTMLAnchorElement>('a');if(target&&event instanceof MouseEvent&&!event.ctrlKey&&!event.metaKey&&!event.altKey&&!event.shiftKey&&jump(target.hash.slice(1)))event.preventDefault()})
  addEventListener('hashchange',()=>jump(location.hash.slice(1)))
  stage.addEventListener('focusin',event=>{if(!enabled())return;const target=event.target as Node;focusPin=project.contains(target)?2.25:calendar.contains(target)?4.05:intro.contains(target)?0.2:undefined;if(focusPin!==undefined){scrollTo({top:scrollY+journey.getBoundingClientRect().top+focusPin*innerHeight,behavior:'instant'});schedule(true)}})
  stage.addEventListener('focusout',()=>{focusPin=undefined;schedule(true)})
  function releaseScrollFocus(){if(focusPin===undefined)return;const active=document.activeElement;if(active instanceof HTMLElement&&stage.contains(active))active.blur();focusPin=undefined;schedule(true)}
  addEventListener('wheel',releaseScrollFocus,{passive:true});addEventListener('touchmove',releaseScrollFocus,{passive:true})
  addEventListener('scroll',()=>schedule(),{passive:true});addEventListener('resize',invalidate,{passive:true});addEventListener('painter:home-work-measure',invalidate)
  calendar.querySelector('.activity-scroll')?.addEventListener('scroll',()=>{if(!measuring&&journey.dataset.workPhase==='calendar')invalidate()},{passive:true})
  reduced.addEventListener('change',()=>{needsMeasure=true;schedule(true)})
  phone.addEventListener('change',()=>{needsMeasure=true;schedule(true)})
  document.addEventListener('visibilitychange',()=>{if(document.hidden){cancelAnimationFrame(queued);queued=0}else invalidate()})
  new MutationObserver(invalidate).observe(document.documentElement,{attributes:true,attributeFilter:['data-theme']})
  if('IntersectionObserver'in window)new IntersectionObserver(entries=>{near=entries[0]?.isIntersecting??true;if(near)schedule(true)},{rootMargin:'100px 0px'}).observe(journey)
  document.fonts.ready.then(invalidate);render();requestAnimationFrame(()=>{invalidate();if(location.hash)jump(location.hash.slice(1))})
 }
}
