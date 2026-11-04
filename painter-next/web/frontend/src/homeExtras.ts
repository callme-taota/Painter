export function initHomeExtras(){
  for(const section of document.querySelectorAll<HTMLElement>('.home-activity')){
    const detail=section.querySelector<HTMLElement>('[data-activity-detail]'),calendar=section.querySelector<HTMLElement>('[data-activity-calendar]')
    if(!detail||!calendar)continue
    const days=Array.from(calendar.querySelectorAll<HTMLElement>('[data-date]'))
    let selected=days.length-1;while(selected>0&&days[selected].classList.contains('is-future'))selected--
    const scroll=section.querySelector<HTMLElement>('.activity-scroll');if(scroll&&days[selected])requestAnimationFrame(()=>{const edge=days[selected].getBoundingClientRect().right-scroll.getBoundingClientRect().right;scroll.scrollLeft+=edge+6})
    const describe=(day:HTMLElement)=>{detail.textContent=`${day.dataset.date} · ${day.dataset.count} 次发布 · 左右滑动查看更多日期`;days.forEach(d=>d.tabIndex=d===day?0:-1);selected=days.indexOf(day)}
    days.forEach((day,i)=>{day.tabIndex=i===selected?0:-1;day.addEventListener('pointerenter',()=>describe(day));day.addEventListener('focus',()=>describe(day));day.addEventListener('click',()=>{describe(day);day.focus()});day.addEventListener('keydown',event=>{const delta=event.key==='ArrowRight'?7:event.key==='ArrowLeft'?-7:event.key==='ArrowDown'?1:event.key==='ArrowUp'?-1:0;if(delta){event.preventDefault();days[Math.max(0,Math.min(days.length-1,selected+delta))]?.focus()}else if(event.key==='Home'){event.preventDefault();days[0]?.focus()}else if(event.key==='End'){event.preventDefault();days.at(-1)?.focus()}})})
  }
  for(const deck of document.querySelectorAll<HTMLElement>('[data-project-deck]')){
    const cards=Array.from(deck.querySelectorAll<HTMLElement>('[data-project-card]'))
    const set=(active:number)=>cards.forEach((card,i)=>{card.style.setProperty('--card-order',String(i+1));card.classList.toggle('is-active',i===active);card.style.setProperty('--dodge-x',active<0?'0px':i<active?'-18px':i>active?'26px':'0px')})
    cards.forEach((card,index)=>{card.addEventListener('pointerenter',event=>{if(event.pointerType!=='touch')set(index)});card.addEventListener('focusin',()=>set(index));card.addEventListener('pointerdown',()=>set(index))})
    deck.addEventListener('pointerleave',()=>{if(!deck.contains(document.activeElement))set(-1)});deck.addEventListener('focusout',event=>{if(!deck.contains(event.relatedTarget as Node))set(-1)});set(-1)
  }
}
