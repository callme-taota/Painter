// Semantic form inputs remain usable without JavaScript; enhancement owns only
// disclosure dismissal, selection summaries and filtering the available tags.
export function initSearchFilters() {
 const form=document.querySelector<HTMLFormElement>('[data-search-filters]')
 if(!form)return
 form.querySelectorAll<HTMLElement>('[data-enhanced]').forEach(e=>e.hidden=false)
 const details=[...form.querySelectorAll<HTMLDetailsElement>('details')]
 const tags=[...form.querySelectorAll<HTMLInputElement>('input[name=tag_id]')]
 const selected=form.querySelector<HTMLElement>('.search-selected-tags')!
 const status=form.querySelector<HTMLElement>('[data-filter-status]')!
 const count=form.querySelector<HTMLElement>('[data-tag-count]')!
 const query=form.querySelector<HTMLInputElement>('[data-tag-query]')!
 let dirty=false
 function place(d:HTMLDetailsElement) {
  const popup=d.querySelector<HTMLElement>('.search-filter-popover')!
  popup.style.top=''
  if(matchMedia('(max-width:720px)').matches&&d.open){const r=d.querySelector('summary')!.getBoundingClientRect();popup.style.top=Math.max(8,Math.min(r.bottom+8,innerHeight-popup.offsetHeight-12))+'px'}
 }
 function update() {
  const checked=tags.filter(t=>t.checked)
  count.textContent=checked.length?`${checked.length} 个`:'全部'
  selected.replaceChildren()
  for(const tag of checked){const button=document.createElement('button');button.type='button';button.className='search-selected-tag';const name=tag.closest('label')!.querySelector('span')!.textContent!;button.textContent=name;button.setAttribute('aria-label',`移除标签：${name}`);const svg=document.createElementNS('http://www.w3.org/2000/svg','svg');svg.setAttribute('viewBox','0 0 24 24');svg.classList.add('icon');svg.setAttribute('aria-hidden','true');const path=document.createElementNS(svg.namespaceURI,'path');path.setAttribute('d','m7 7 10 10M17 7 7 17');svg.append(path);button.append(svg);button.addEventListener('click',()=>{tag.checked=false;dirty=true;update();form!.querySelector<HTMLElement>('[data-tag-filter] summary')?.focus()});selected.append(button)}
  for(const tag of tags)tag.disabled=checked.length>=20&&!tag.checked
  status.textContent=dirty?'筛选已更改，点击“搜索”查看结果。':''
 }
 update()
 form.addEventListener('change',e=>{dirty=true;if((e.target as HTMLInputElement).name==='sort'){const label=(e.target as HTMLElement).closest('label')!.querySelector('span')!.firstChild!.textContent!;form.querySelector('[data-sort-label]')!.textContent=label;const d=(e.target as HTMLElement).closest('details')!;d.open=false;d.querySelector('summary')!.focus()}update()})
 form.querySelector('[data-tags-clear]')!.addEventListener('click',()=>{tags.forEach(t=>t.checked=false);dirty=true;update()})
 form.querySelector('[data-filter-done]')!.addEventListener('click',()=>{const d=form.querySelector<HTMLDetailsElement>('[data-tag-filter]')!;d.open=false;d.querySelector('summary')!.focus()})
 query.addEventListener('input',()=>{const value=query.value.normalize('NFKC').toLocaleLowerCase().trim();let visible=0;for(const tag of tags){const row=tag.closest<HTMLLabelElement>('label')!;row.hidden=!row.querySelector('span')!.textContent!.normalize('NFKC').toLocaleLowerCase().includes(value);if(!row.hidden)visible++}form.querySelector<HTMLElement>('[data-tags-no-match]')!.hidden=visible>0})
 for(const d of details){d.addEventListener('toggle',()=>{if(d.open){for(const other of details)if(other!==d)other.open=false;place(d)}});d.addEventListener('keydown',e=>{if(e.key==='Escape'&&d.open){e.preventDefault();e.stopPropagation();d.open=false;d.querySelector('summary')!.focus()}});d.addEventListener('focusout',e=>{if(e.relatedTarget instanceof Node&&!d.contains(e.relatedTarget))d.open=false})}
 window.addEventListener('resize',()=>details.forEach(place))
 window.addEventListener('scroll',()=>details.forEach(d=>{if(d.open)place(d)}),{passive:true})
 document.addEventListener('pointerdown',e=>{for(const d of details)if(!d.contains(e.target as Node))d.open=false})
}
