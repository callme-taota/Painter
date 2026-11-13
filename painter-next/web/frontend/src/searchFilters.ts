import {registerMobileOverlay,dismissMobileOverlay} from './mobileOverlays'
// Keep one set of form inputs. Desktop disclosures become a phone task sheet.
export function initSearchFilters(){
 const form=document.querySelector<HTMLFormElement>('[data-search-filters]');if(!form)return
 form.querySelectorAll<HTMLElement>('[data-enhanced]').forEach(e=>e.hidden=false)
 const details=[...form.querySelectorAll<HTMLDetailsElement>('details')]
 const tags=[...form.querySelectorAll<HTMLInputElement>('input[name=tag_id]')]
 const sorts=[...form.querySelectorAll<HTMLInputElement>('input[name=sort]')]
 const selected=form.querySelector<HTMLElement>('.search-selected-tags')!,status=form.querySelector<HTMLElement>('[data-filter-status]')!,count=form.querySelector<HTMLElement>('[data-tag-count]')!,query=form.querySelector<HTMLInputElement>('[data-tag-query]')!
 const phone=matchMedia('(max-width:720px)');let dirty=false
 const sheet=document.createElement('dialog');sheet.className='mobile-sheet mobile-filter-sheet';sheet.setAttribute('aria-labelledby','mobile-filter-title')
 sheet.innerHTML='<header class="mobile-sheet-heading"><button type="button" class="quiet" data-filter-cancel>取消</button><h2 id="mobile-filter-title" tabindex="-1">筛选</h2><span aria-hidden="true"></span></header><div class="mobile-sheet-content"></div><div class="mobile-sheet-actions"><button type="button" class="quiet" data-filter-reset>重置</button><button type="button" class="primary" data-filter-apply>查看结果</button></div>'
 form.append(sheet);registerMobileOverlay(sheet,()=>{if(lastOwner)openFilter(lastOwner)})
 let lastOwner:HTMLDetailsElement|null=null
 let owner:HTMLDetailsElement|null=null,panel:HTMLElement|null=null,previousDirty=false,snapshot:boolean[]=[]
 function sortLabel(){const value=sorts.find(s=>s.checked)?.value;form!.querySelector('[data-sort-label]')!.textContent=value==='recent'?'最近发布':'相关性'}
 function update(){
  const checked=tags.filter(t=>t.checked);count.textContent=checked.length?`${checked.length} 个`:'全部';selected.replaceChildren()
  for(const tag of checked){const button=document.createElement('button');button.type='button';button.className='search-selected-tag';const name=tag.closest('label')!.querySelector('span')!.textContent!;button.textContent=name;button.setAttribute('aria-label',`移除标签：${name}`);const svg=document.createElementNS('http://www.w3.org/2000/svg','svg');svg.setAttribute('viewBox','0 0 24 24');svg.classList.add('icon');svg.setAttribute('aria-hidden','true');const path=document.createElementNS(svg.namespaceURI,'path');path.setAttribute('d','m7 7 10 10M17 7 7 17');svg.append(path);button.append(svg);button.addEventListener('click',()=>{tag.checked=false;dirty=true;update();form!.querySelector<HTMLElement>('[data-tag-filter] summary')?.focus()});selected.append(button)}
  for(const tag of tags)tag.disabled=checked.length>=20&&!tag.checked
  status.textContent=dirty?'筛选已更改，点击“搜索”查看结果。':'';sortLabel()
 }
 function openFilter(detail:HTMLDetailsElement){
  if(sheet.open)return
  details.forEach(d=>d.open=false);owner=detail;lastOwner=detail;panel=detail.querySelector<HTMLElement>('.search-filter-popover')!
  previousDirty=dirty;snapshot=[...tags,...sorts].map(x=>x.checked);sheet.returnValue='';sheet.querySelector('#mobile-filter-title')!.textContent=detail.hasAttribute('data-tag-filter')?'选择标签':'文章排序'
  sheet.querySelector('.mobile-sheet-content')!.append(panel);sheet.showModal()
  requestAnimationFrame(()=>{sheet.querySelector<HTMLElement>('#mobile-filter-title')?.focus({preventScroll:true})})
 }
 sheet.addEventListener('close',()=>{
  if(sheet.returnValue!=='apply'){[...tags,...sorts].forEach((x,i)=>x.checked=snapshot[i]);dirty=previousDirty}
  if(panel&&owner){owner.append(panel);owner.open=false;owner.querySelector<HTMLElement>('summary')?.focus({preventScroll:true})}
  panel=null;owner=null;update()
 })
 sheet.querySelector('[data-filter-cancel]')!.addEventListener('click',()=>sheet.close('cancel'))
 sheet.querySelector('[data-filter-reset]')!.addEventListener('click',()=>{if(owner?.hasAttribute('data-tag-filter'))tags.forEach(x=>x.checked=false);else sorts.forEach(x=>x.checked=x.value==='relevance');dirty=true;update()})
 sheet.querySelector('[data-filter-apply]')!.addEventListener('click',()=>{void dismissMobileOverlay(sheet,'apply').then(()=>form.requestSubmit())})
 sheet.addEventListener('click',event=>{if(event.target!==sheet)return;const b=sheet.getBoundingClientRect();if(event.clientX<b.left||event.clientX>b.right||event.clientY<b.top||event.clientY>b.bottom)sheet.close('cancel')})
 form.addEventListener('submit',event=>{if(sheet.open){event.preventDefault();void dismissMobileOverlay(sheet,'apply').then(()=>form.requestSubmit())}})
 form.addEventListener('change',event=>{
  dirty=true
  if((event.target as HTMLInputElement).name==='sort'&&!sheet.open){const d=(event.target as HTMLElement).closest('details')!;d.open=false;d.querySelector('summary')!.focus()}
  update()
 })
 form.querySelector('[data-tags-clear]')!.addEventListener('click',()=>{tags.forEach(t=>t.checked=false);dirty=true;update()})
 form.querySelector('[data-filter-done]')!.addEventListener('click',()=>{const d=form.querySelector<HTMLDetailsElement>('[data-tag-filter]')!;d.open=false;d.querySelector('summary')!.focus()})
 query.addEventListener('input',()=>{const value=query.value.normalize('NFKC').toLocaleLowerCase().trim();let visible=0;for(const tag of tags){const row=tag.closest<HTMLLabelElement>('label')!;row.hidden=!row.querySelector('span')!.textContent!.normalize('NFKC').toLocaleLowerCase().includes(value);if(!row.hidden)visible++}form.querySelector<HTMLElement>('[data-tags-no-match]')!.hidden=visible>0})
 for(const d of details){
  d.querySelector('summary')!.addEventListener('click',event=>{if(phone.matches){event.preventDefault();openFilter(d)}})
  d.addEventListener('toggle',()=>{if(d.open)for(const other of details)if(other!==d)other.open=false})
  d.addEventListener('keydown',event=>{if(event.key==='Escape'&&d.open){event.preventDefault();event.stopPropagation();d.open=false;d.querySelector('summary')!.focus()}})
  d.addEventListener('focusout',event=>{if(event.relatedTarget instanceof Node&&!d.contains(event.relatedTarget))d.open=false})
 }
 phone.addEventListener('change',()=>{if(sheet.open)sheet.close('cancel');details.forEach(d=>d.open=false)})
 document.addEventListener('pointerdown',event=>{if(sheet.open)return;for(const d of details)if(!d.contains(event.target as Node))d.open=false})
 update()
}
