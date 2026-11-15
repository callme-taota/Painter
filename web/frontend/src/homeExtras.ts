import {t,formatDate,formatNumber} from './i18n'
export function initHomeExtras(){
  for(const section of document.querySelectorAll<HTMLElement>('.home-activity')){
    const detail=section.querySelector<HTMLElement>('[data-activity-detail]'),calendar=section.querySelector<HTMLElement>('[data-activity-calendar]')
    if(!detail||!calendar)continue
    const days=Array.from(calendar.querySelectorAll<HTMLElement>('[data-date]'))
    const months=section.querySelector<HTMLElement>('[data-activity-months]')
    if(months&&!months.children.length)Array.from(calendar.children).forEach((week,index)=>{const dates=Array.from(week.querySelectorAll<HTMLElement>('[data-date]')).map(day=>day.dataset.date!);const date=dates.find(value=>value.endsWith('-01'))||(index===0?dates[0]:undefined);const label=document.createElement('span');label.textContent=date?formatDate(date+'T12:00:00',{month:'short'}):'';months.append(label)})
    let selected=days.length-1;while(selected>0&&days[selected].classList.contains('is-future'))selected--
    const scroll=section.querySelector<HTMLElement>('.activity-scroll');if(scroll&&days[selected])requestAnimationFrame(()=>{const edge=days[selected].getBoundingClientRect().right-scroll.getBoundingClientRect().right;scroll.scrollLeft+=edge+6})
    const describe=(day:HTMLElement)=>{detail.textContent=t('{date} · {count} 次发布 · 左右滑动查看更多日期',{date:formatDate(day.dataset.date+'T12:00:00'),count:formatNumber(Number(day.dataset.count||0))});days.forEach(d=>d.tabIndex=d===day?0:-1);selected=days.indexOf(day)}
    // Dense heatmap cells remain useful visual data, while phones get separate
    // full-size previous/next controls rather than tiny required tap targets.
    if(days.length){
      const navigation=document.createElement('div');navigation.className='activity-mobile-navigation';navigation.setAttribute('role','group');navigation.setAttribute('aria-label',t('选择发布日历日期'))
      const label=document.createElement('span')
      const previous=document.createElement('button'),next=document.createElement('button')
      for(const [button,direction,title] of [[previous,-1,t('上一天')],[next,1,t('下一天')]] as const){
        button.type='button';button.className='quiet';button.setAttribute('aria-label',title);button.innerHTML=`<svg class="icon" viewBox="0 0 24 24" aria-hidden="true"><path d="${direction<0?'m14 6-6 6 6 6':'m10 6 6 6-6 6'}"/></svg>`
        button.addEventListener('click',()=>{const day=days[Math.max(0,Math.min(days.length-1,selected+direction))];if(!day)return;describe(day);updateSelection();if(scroll){const item=day.getBoundingClientRect(),viewport=scroll.getBoundingClientRect();if(item.left<viewport.left)scroll.scrollLeft-=viewport.left-item.left+8;else if(item.right>viewport.right)scroll.scrollLeft+=item.right-viewport.right+8}})
      }
      function updateSelection(){const day=days[selected];if(!day)return;label.textContent=t('{date} · {count} 次发布',{date:formatDate(day.dataset.date+'T12:00:00'),count:formatNumber(Number(day.dataset.count||0))});previous.disabled=selected===0;next.disabled=selected===days.length-1;days.forEach(item=>item.classList.toggle('is-selected-day',item===day))}
      navigation.append(previous,label,next);calendar.closest('.activity-scroll')?.after(navigation)
      calendar.addEventListener('click',updateSelection);calendar.addEventListener('focusin',updateSelection);updateSelection()
    }
    days.forEach((day,i)=>{day.tabIndex=i===selected?0:-1;day.addEventListener('pointerenter',()=>describe(day));day.addEventListener('focus',()=>describe(day));day.addEventListener('click',()=>{describe(day);day.focus()});day.addEventListener('keydown',event=>{const delta=event.key==='ArrowRight'?7:event.key==='ArrowLeft'?-7:event.key==='ArrowDown'?1:event.key==='ArrowUp'?-1:0;if(delta){event.preventDefault();days[Math.max(0,Math.min(days.length-1,selected+delta))]?.focus()}else if(event.key==='Home'){event.preventDefault();days[0]?.focus()}else if(event.key==='End'){event.preventDefault();days.at(-1)?.focus()}})})
  }
  for(const deck of document.querySelectorAll<HTMLElement>('[data-project-deck]')){
    const cards=Array.from(deck.querySelectorAll<HTMLElement>('[data-project-card]'))
    const set=(active:number)=>cards.forEach((card,i)=>{card.style.setProperty('--card-order',String(i+1));card.classList.toggle('is-active',i===active);card.style.setProperty('--dodge-x',active<0?'0px':i<active?'-18px':i>active?'26px':'0px')})
    cards.forEach((card,index)=>{card.addEventListener('pointerenter',event=>{if(event.pointerType!=='touch')set(index)});card.addEventListener('focusin',()=>set(index));card.addEventListener('pointerdown',()=>set(index))})
    deck.addEventListener('pointerleave',()=>{if(!deck.contains(document.activeElement))set(-1)});deck.addEventListener('focusout',event=>{if(!deck.contains(event.relatedTarget as Node))set(-1)});set(-1)
  }
}
