import 'vite/modulepreload-polyfill'
import './style.css'
import './controls.css'
import './homeStory.css'
import './phrases.css'
import './reading.css'
import './friends.css'
import {initFriendAvatars} from './friends'
initFriendAvatars()
import './header-scroll.css'
import {initScrollHeader} from './scrollHeader'
import {initArticleInfo} from './reading'
initScrollHeader()
initArticleInfo()
import {initTheme} from './theme'
initTheme()
const target=document.querySelector<HTMLElement>('[data-post-island]')
if(target){const load=async()=>{observer?.disconnect();const [{createApp},{default:Island}]=await Promise.all([import('vue'),import('./components/PostIsland.vue')]);createApp(Island,{postId:target.dataset.postId,revisionId:target.dataset.revisionId,receipt:target.dataset.receipt}).mount(target)};const observer='IntersectionObserver'in window?new IntersectionObserver(es=>{if(es.some(e=>e.isIntersecting))void load()},{rootMargin:'400px'}):null;if(observer)observer.observe(target);else void load()}
for(const el of document.querySelectorAll<HTMLElement>('[data-follow-island]')){const [{createApp},{default:Follow}]=await Promise.all([import('vue'),import('./components/FollowIsland.vue')]);createApp(Follow,{userId:el.dataset.userId}).mount(el)}
import {initTelemetry} from './telemetry'
try{initTelemetry()}catch{}

import {initExperience} from './experience'
void initExperience().catch(()=>{})

const isHome=document.body.dataset.pageKind==='home'
if(isHome)void (async()=>{
 const extras=await import('./homeExtras');extras.initHomeExtras()
 // Let the calendar settle its native latest-date scroll before measuring the gathered field.
 await new Promise<void>(resolve=>requestAnimationFrame(()=>resolve()))
 if(document.querySelector('[data-home-work-journey]')){const work=await import('./homeWorkJourney');work.initHomeWorkJourney()}
 const story=await import('./homeStory');story.initHomeStory()
 const showcase=await import('./homeShowcase');showcase.initHomeShowcase()
 dispatchEvent(new Event('painter:home-work-measure'))
 const snap=await import('./homeSnap');snap.initHomeSnap()
 // V31 lenses are independent; the former media effect painted a single rail.
 if(!document.querySelector('[data-site-dock]')){const glass=await import('./homeControlGlass');glass.initHomeControlGlass()}
})().catch(()=>{})

if(document.querySelector('[data-phrase-gallery]'))void import('./phrases').then(m=>m.initPhrases()).catch(()=>{})

import './searchPage.css'
import {initSearchFilters} from './searchFilters'
initSearchFilters()

import './author.css'
import {initAuthorBackgrounds} from './author'
initAuthorBackgrounds()
import './homeWriting.css'
if(document.querySelector('[data-home-writing]'))void import('./homeWriting').then(m=>m.initHomeWriting()).catch(()=>{})

import './shadcn-theme.css'

import './moments.css'
import './momentImages.css'
import {initMoments} from './moments'
initMoments()
import './creation.css'
import './homepageManager.css'
import './homeWalls.css'
import './homeShowcase.css'
if(!isHome&&document.querySelector('[data-game-story],[data-record-wall],[data-library-cover]'))void import('./homeShowcase').then(m=>m.initHomeShowcase()).catch(()=>{})
import {initCreationControls} from './creation'
initCreationControls()

import './homeExtras.css'
if(!isHome&&document.querySelector('[data-activity-calendar],[data-project-deck]'))void import('./homeExtras').then(m=>m.initHomeExtras()).catch(()=>{})
import {initArticleCards} from './contentCards'
initArticleCards()

import {initLiquidGlass} from './liquidGlass'
import './public-chrome.css'
initLiquidGlass()
import {initDock} from './dock'
initDock()

import './gamePlatforms.css'
import './searchGlass.css'

import './mobileContent.css'
import './mobileChrome.css'
import {initMobileViewport,registerMobileOverlay} from './mobileOverlays'
import {initMobileNavigation} from './mobileNavigation'
initMobileViewport()
initMobileNavigation()
const searchDialog=document.querySelector<HTMLDialogElement>('#site-search')
if(searchDialog)registerMobileOverlay(searchDialog,()=>document.querySelector<HTMLElement>('[data-search-open]')?.click())

import './series.css'
import './sharing.css'
import {initSharing} from './sharing'
initSharing()
