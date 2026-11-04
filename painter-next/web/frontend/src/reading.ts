// Native disclosure works without JS; Escape and outside click are enhancements.
export function initArticleInfo(){
 const details=document.querySelector<HTMLDetailsElement>('.article-info')
 if(!details)return
 const trigger=details.querySelector<HTMLElement>('summary')
 document.addEventListener('pointerdown',event=>{if(details.open&&event.target instanceof Node&&!details.contains(event.target))details.open=false})
 document.addEventListener('keydown',event=>{if(event.key==='Escape'&&details.open&&!document.querySelector('dialog[open],[role="dialog"][aria-modal="true"]')){details.open=false;if(details.contains(document.activeElement))trigger?.focus();event.preventDefault()}})
}
