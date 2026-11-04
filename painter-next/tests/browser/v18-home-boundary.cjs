// Read-only small-phone proof: one home gutter, readable scenes and static fallbacks.
const fs=require('fs'),{chromium}=require(process.env.PLAYWRIGHT_MODULE||'/tmp/painter-browser/node_modules/playwright');
const base=process.env.BASE_URL||'http://192.168.2.209:18081',rows=[];
(async()=>{const browser=await chromium.launch({executablePath:'/usr/bin/google-chrome',headless:true,args:['--no-sandbox']});try{
 for(const fallback of['scroll','reduced','no-js']){
  const context=await browser.newContext({viewport:{width:320,height:667},reducedMotion:fallback==='reduced'?'reduce':'no-preference',javaScriptEnabled:fallback!=='no-js'});
  await context.route('**/api/v1/analytics/events',route=>route.fulfill({json:{accepted:0}}));
  const page=await context.newPage();await page.goto(base+'/');await page.evaluate(()=>document.fonts.ready);
  if(fallback!=='no-js')await page.waitForFunction(()=>document.querySelector('[data-home-story]')?.dataset.renderer==='dom');
  for(const progress of fallback==='scroll'?[0,.44,.86]:[0]){
   if(fallback==='scroll'){await page.locator('[data-home-story]').evaluate((element,p)=>scrollTo(0,element.offsetTop+(element.offsetHeight-element.querySelector('.story-stage').clientHeight)*p),progress);await page.evaluate(()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve))));}
   const geometry=await page.evaluate(()=>{const box=element=>{const r=element.getBoundingClientRect();return{x:r.x,y:r.y,w:r.width,h:r.height,right:r.right,bottom:r.bottom}};return{main:box(document.querySelector('main')),ready:document.documentElement.classList.contains('story-ready'),active:[...document.querySelectorAll('[data-story-scene]')].filter(e=>!e.inert).map(e=>({chapter:e.dataset.storyScene,copy:box(e.firstElementChild)})),art:box(document.querySelector('[data-story-pages]')),overflow:document.documentElement.scrollWidth>innerWidth+1,titleSize:getComputedStyle(document.querySelector('.story-title')).fontSize}});
   if(geometry.main.w!==320||geometry.main.x!==0||geometry.overflow||geometry.active.some(e=>e.copy.w<220))throw Error('Home applies more than one gutter: '+JSON.stringify(geometry));
   if(fallback==='scroll'&&(geometry.active.length!==1||geometry.active[0].copy.bottom>geometry.art.y))throw Error('Active home copy collides with paper planes: '+JSON.stringify(geometry));
   if(fallback!=='scroll'&&(geometry.ready||geometry.active.length!==3))throw Error('Static fallback hides a home scene');
   rows.push({fallback,progress,...geometry});
  }
  await context.close();
 }
 fs.writeFileSync('tests/artifacts/v18/home-boundary.json',JSON.stringify({passed:true,origin:base,viewport:'320x667',scenarios:rows,actual_business_writes:0},null,2));console.log(JSON.stringify({passed:true,scenarios:rows.length,actual_business_writes:0}));
}finally{await browser.close()}})().catch(error=>{console.error(error);process.exitCode=1});
