type Theme='light'|'dark'|'system'
export function initTheme(){
 const root=document.documentElement,system=matchMedia('(prefers-color-scheme:dark)'),panel=document.querySelector<HTMLDetailsElement>('.floating-theme'),trigger=panel?.querySelector<HTMLElement>('summary')
 const read=():Theme=>{try{const v=localStorage.getItem('painter-theme');return v==='light'||v==='dark'?v:'system'}catch{return 'system'}}
 const apply=()=>{const chosen=read();root.dataset.theme=chosen==='system'?(system.matches?'dark':'light'):chosen;const chrome=document.querySelector<HTMLMetaElement>('meta[name=theme-color]');if(chrome)chrome.content=root.dataset.theme==='dark'?'#09090b':'#ffffff';document.querySelectorAll<HTMLButtonElement>('[data-theme-option]').forEach(el=>{const checked=el.dataset.themeOption===chosen;el.setAttribute('aria-checked',String(checked));el.tabIndex=checked?0:-1});document.querySelectorAll('[data-solid-toggle]').forEach(el=>el.setAttribute('aria-pressed',String(root.classList.contains('reduce-transparency'))))}
 try{root.classList.toggle('reduce-transparency',localStorage.getItem('painter-solid')==='1')}catch{}
 apply();system.addEventListener('change',apply)
 const choices=Array.from(document.querySelectorAll<HTMLButtonElement>('[data-theme-option]'))
 choices.forEach((el,index)=>{el.addEventListener('click',()=>{try{localStorage.setItem('painter-theme',el.dataset.themeOption!)}catch{}apply()});el.addEventListener('keydown',e=>{let next=index;if(e.key==='ArrowDown'||e.key==='ArrowRight')next=(index+1)%choices.length;else if(e.key==='ArrowUp'||e.key==='ArrowLeft')next=(index+choices.length-1)%choices.length;else if(e.key==='Home')next=0;else if(e.key==='End')next=choices.length-1;else return;e.preventDefault();choices[next].focus();choices[next].click()})})
 trigger?.addEventListener('keydown',e=>{if(e.key==='ArrowDown'){e.preventDefault();panel!.open=true;choices.find(el=>el.getAttribute('aria-checked')==='true')?.focus()}})
 document.addEventListener('pointerdown',e=>{if(panel?.open&&!panel.contains(e.target as Node))panel.open=false})
 document.addEventListener('keydown',e=>{if(e.key==='Escape'&&panel?.open){panel.open=false;trigger?.focus()}})
 document.querySelectorAll('[data-solid-toggle]').forEach(el=>el.addEventListener('click',()=>{root.classList.toggle('reduce-transparency');try{localStorage.setItem('painter-solid',root.classList.contains('reduce-transparency')?'1':'0')}catch{}apply()}))
 document.querySelectorAll<HTMLElement>('[data-search-shortcut]').forEach(el=>el.textContent=/Mac|iPhone|iPad/.test(navigator.platform)?'⌘ K':'Ctrl K')
}
