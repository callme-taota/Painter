import {t} from './i18n'
import {toast} from './toast'
import './pwa.css'

type InstallPromptEvent=Event&{
 prompt:()=>Promise<unknown>
 userChoice:Promise<{outcome:'accepted'|'dismissed'}>
}
type InstallNavigator=Navigator&{install?:()=>Promise<unknown>;standalone?:boolean}
type AppHistoryEntry={index:number;url?:string}
type AppNavigation=EventTarget&{currentEntry?:AppHistoryEntry;entries:()=>AppHistoryEntry[]}
const installedKey='painter.pwa-installed'

/** Chrome exposes same-origin history entries; a referrer alone cannot prove
 * that Back is safe after redirects or location.replace(). */
function hasSafePreviousEntry():boolean{
 const navigation=(window as Window&{navigation?:AppNavigation}).navigation
 if(!navigation?.currentEntry||typeof navigation.entries!=='function')return false
 try{
  const current=navigation.currentEntry,previous=navigation.entries().find(entry=>entry.index===current.index-1)
  return !!previous?.url&&new URL(previous.url,location.href).origin===location.origin
 }catch{return false}
}

/** Install is always initiated by an explicit visitor click. No page reload,
 * permissions request, service worker, or content storage is needed. */
export function initPWA(){
 if(document.documentElement.hasAttribute('data-pwa-ready'))return
 document.documentElement.setAttribute('data-pwa-ready','')
 const browser=navigator as InstallNavigator
 const mode=matchMedia('(display-mode: standalone), (display-mode: minimal-ui), (display-mode: fullscreen), (display-mode: window-controls-overlay)')
 const installButtons=Array.from(document.querySelectorAll<HTMLButtonElement>('[data-pwa-install]'))
 const backButtons=Array.from(document.querySelectorAll<HTMLButtonElement>('[data-app-back]'))
 let deferred:InstallPromptEvent|null=null,busy=false,installed=false,nativeCompleted=false
 try{installed=sessionStorage.getItem(installedKey)==='1'}catch{}
 const inApp=()=>mode.matches||browser.standalone===true
 const canInstall=()=>isSecureContext&&!installed&&!inApp()&&(!!deferred||typeof browser.install==='function'&&!nativeCompleted)
 function sync(){
  const available=canInstall(),canBack=inApp()&&hasSafePreviousEntry()
  installButtons.forEach(button=>{button.hidden=!available;button.disabled=busy;button.setAttribute('aria-busy',String(busy))})
  backButtons.forEach(button=>{button.hidden=!canBack})
 }
 function help(){
  const security=isSecureContext?'':t('请先使用本站的 HTTPS 地址。')
  toast.info(security+t('使用 Chrome 时，可在地址栏或浏览器菜单中查找“安装”。是否提供安装由浏览器决定；已经安装的应用可从系统应用列表打开。当前版本需要联网使用。'),{id:'pwa-install-help',duration:14000})
 }
 async function install(){
  if(busy)return
  if(installed||inApp()){sync();toast.info(t('可从系统应用列表打开已安装的应用。'),{id:'pwa-install'});return}
  const prompt=deferred,nativeInstall=browser.install
  if(!isSecureContext||!prompt&&typeof nativeInstall!=='function'){sync();help();return}
  busy=true;sync()
  try{
   if(typeof nativeInstall==='function'){
    // The current document's linked manifest determines app identity. Call
    // synchronously from this click, before any await loses user activation.
    await nativeInstall.call(browser)
    nativeCompleted=true;deferred=null
    // A resolved Web Install API promise may also mean opening an existing app.
    if(!installed)toast.info(t('浏览器已完成安装或打开操作。可在系统应用列表中查看。'),{id:'pwa-install'})
   }else if(prompt){
    deferred=null
    await prompt.prompt()
    const choice=await prompt.userChoice
    if(!installed){
     if(choice.outcome==='accepted')toast.info(t('已确认安装，请在浏览器中完成后续操作。'),{id:'pwa-install'})
     else toast.info(t('已取消安装。你可以稍后从浏览器菜单重试。'),{id:'pwa-install'})
    }
   }
  }catch(error){
   if(!installed){
    if(error instanceof DOMException&&error.name==='AbortError')toast.info(t('已取消或暂时无法安装。你可以稍后从浏览器菜单重试。'),{id:'pwa-install'})
    else toast.warning(t('浏览器暂时无法安装。请检查网络，或从浏览器菜单重试。'),{id:'pwa-install'})
   }
  }finally{busy=false;sync()}
 }
 addEventListener('beforeinstallprompt',event=>{
  const prompt=event as InstallPromptEvent
  if(typeof prompt.prompt!=='function'||!prompt.userChoice)return
  event.preventDefault();deferred=prompt;sync()
 })
 addEventListener('appinstalled',()=>{
  installed=true;deferred=null
  try{sessionStorage.setItem(installedKey,'1')}catch{}
  sync();toast.success(t('应用已安装，可以从系统应用列表打开。'),{id:'pwa-install'})
 })
 installButtons.forEach(button=>button.addEventListener('click',()=>{
  button.closest('details')?.removeAttribute('open')
  void install()
 }))
 document.querySelectorAll<HTMLElement>('[data-pwa-help]').forEach(button=>button.addEventListener('click',()=>{
  button.closest('details')?.removeAttribute('open');help()
 }))
 backButtons.forEach(button=>button.addEventListener('click',()=>{
  if(!inApp())return
  if(hasSafePreviousEntry())history.back()
  else{sync();if(location.pathname!=='/'||location.search||location.hash)location.assign('/')}
 }))
 mode.addEventListener('change',sync)
 addEventListener('pageshow',sync);addEventListener('popstate',sync)
 ;(window as Window&{navigation?:AppNavigation}).navigation?.addEventListener('currententrychange',sync)
 sync()
}
