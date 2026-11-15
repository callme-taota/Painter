import {initSearch} from './search'
import {dismissMobileOverlay} from './mobileOverlays'
import {loadLike,toggleLike,type LikeState} from './postLike'
export async function initExperience(){initSearch();const postId=document.body.dataset.postId;const like=document.querySelector<HTMLButtonElement>('[data-floating-action=like]');if(like&&postId){const update=(state:LikeState)=>{if(state.postId!==postId)return;like.hidden=!state.session?.authenticated;like.disabled=state.busy||!state.loaded||!state.session?.authenticated;like.setAttribute('aria-pressed',String(state.liked));like.setAttribute('aria-label',`${state.liked?'取消此文的赞':'赞这篇文章'}，${state.count} 赞`);like.querySelector('[data-like-count]')!.textContent=String(state.count);const message=document.querySelector<HTMLElement>('[data-floating-status]');if(message)message.textContent=state.session?.authenticated?state.error:'';if(!state.session?.authenticated)like.remove()};window.addEventListener('painter:like',e=>update((e as CustomEvent).detail));like.addEventListener('click',()=>void toggleLike(postId));update(await loadLike(postId))}document.querySelectorAll<HTMLButtonElement>('[data-floating-action=top],[data-floating-action=bottom]').forEach(button=>button.addEventListener('click',async()=>{
 const overlay=button.closest<HTMLDialogElement>('dialog')
 // Popstate can restore the position that preceded the mobile menu. Complete
 // that dismissal before starting the requested page movement.
 if(overlay?.open)await dismissMobileOverlay(overlay)
 const menu=button.closest<HTMLDetailsElement>('details.dock-more')
 if(menu)menu.open=false
 scrollTo({top:button.dataset.floatingAction==='bottom'?document.documentElement.scrollHeight:0,behavior:matchMedia('(prefers-reduced-motion:reduce)').matches?'instant':'smooth'})
}));const auth=document.querySelector<HTMLElement>('[data-auth-form]');if(auth){const [{createApp},{default:AuthForm}]=await Promise.all([import('vue'),import('./components/AuthForm.vue')]);createApp(AuthForm,{mode:auth.dataset.authForm,returnTo:auth.dataset.returnTo}).mount(auth)}
}
