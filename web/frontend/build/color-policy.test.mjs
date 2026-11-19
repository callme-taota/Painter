import test from 'node:test'
import assert from 'node:assert/strict'
import {auditFile,auditHTML,auditScript,auditStylesheet,createContract} from './color-policy.mjs'

const contract=createContract({version:1,colors:{ink:{light:'#18181b',dark:'#fafafa',description:'Text',category:'interface'},paper:{light:'#fff',dark:'#18181b',description:'Surface',category:'interface'},'shadow-color':{light:'#00000020',dark:'#00000040',description:'Shadow',category:'interface'}},aliases:{text:'ink'},modes:{print:{ink:'#000'}}})
const css=(source,file='web/frontend/src/example.css')=>auditStylesheet(source,file,contract)
const script=source=>auditScript(source,'web/frontend/src/example.ts',contract)
const messages=issues=>issues.map(issue=>issue.message).join('\n')

test('approved shared keys, aliases, mix and system colors remain legal',()=>{
  assert.deepEqual(css('.card{color:var(--ink);background:color-mix(in srgb,var(--paper) 75%,transparent);border:1px solid var(--text);outline:2px solid CanvasText;box-shadow:0 2px 10px var(--shadow-color);fill:currentColor}'),[])
})

test('literal color syntaxes and CSS escapes cannot bypass the contract',()=>{
  for(const color of ['#fff','#12345678','rgb(0 0 0 / .5)','rgba(0,0,0,.5)','hsl(0 50% 50%)','oklch(.7 .2 40)','white','BLACK','red','r\\65 d','color(srgb 1 0 0)'])assert.match(messages(css(`.x{color:${color}}`)),/Raw color/,color)
})

test('unknown color keys and unregistered derived keys fail',()=>{
  assert.match(messages(css('.x{color:var(--invented)}')),/Undefined color key --invented/)
  assert.match(messages(css('.x{--private:var(--ink);color:var(--private)}')),/Unregistered component color key --private/)
  assert.match(messages(css('.x{--a:var(--b);--b:var(--ink);color:var(--a)}')),/Unregistered component color key --a/)
  assert.match(messages(css('.x{color:var(--ink,#fff)}')),/Raw color #fff/)
  assert.match(messages(css('.x{--new:var(--unknown);color:var(--new)}')),/Undefined color key --new/)
  assert.deepEqual(css('.x{--hover-turn:var(--runtime-turn);transform:rotate(var(--hover-turn))}'),[])
})

test('canonical definitions are forbidden even when their value is a valid key',()=>{
  assert.match(messages(css(':root{--ink:var(--paper)}')),/Canonical color key --ink cannot be redefined/)
  assert.match(messages(css('.x{--text:inherit}')),/Canonical color key --text cannot be redefined/)
  assert.deepEqual(css(':root{--ink:#18181b}','web/frontend/src/color-tokens.css'),[])
  assert.match(messages(css('@property --ink{syntax:"<color>";initial-value:#fff;inherits:true}')),/Canonical color key/)
  assert.match(messages(css('@property --local{syntax:"<color>";initial-value:#fff;inherits:true}')),/shared registry/)
})

test('derived shadow keys are constrained by both name and source',()=>{
  assert.deepEqual(css(':root{--shadow:0 2px 4px var(--shadow-color)}','web/frontend/src/shadcn-theme.css'),[])
  assert.match(messages(css(':root{--shadow:0 2px 4px var(--shadow-color)}')),/Unregistered component color key --shadow/)
  assert.match(messages(css(':root{--shadow:0 2px 4px #000}','web/frontend/src/shadcn-theme.css')),/Raw color/)
})

test('noncolor selectors, text, URLs, anchors and structural dimensions are not colors',()=>{
  assert.deepEqual(css('#fff{white-space:normal;content:"white #abc";background-image:url("/cover.svg#fff");--size:2px;border:var(--size) solid var(--ink)}'),[])
  assert.deepEqual(script('document.querySelector("#fff");const href="/#fff";const sample="Example uses #abc in source";const shape={whiteSpace:"normal"};'),[])
  assert.deepEqual(auditHTML('<a href="#fff" style="white-space:normal">white</a>','web/templates/example.html',contract),[])
  assert.deepEqual(auditHTML('<div data-color="ready" data-fill="white"></div>','web/templates/example.html',contract),[])
  assert.deepEqual(css('.x{background:color-mix(in srgb,var(--ink) calc(var(--progress,0)*100%),transparent)}'),[])
})

test('inline script styles cover assignments, kebab properties, editor themes and setProperty',()=>{
  for(const source of ['element.style.color="#fff"','element.style["background-color"]="white"','const editorTheme={"border-color":"#fff"}','element.style.setProperty("color","black")','CSS.supports("background","rgb(0 0 0)")','element.setAttribute("style","color:#fff")','element.style.cssText="color:#fff"','const color="#fff";element.style.color=color'])assert.match(messages(script(source)),/Raw color/,source)
  assert.match(messages(script('element.style.setProperty("--ink","var(--paper)")')),/Canonical color key/)
  assert.match(messages(script('const editorTheme={color:"var(--undefined-key)"}')),/Undefined color key/)
  assert.match(messages(script('const color="var(--undefined-key)";element.style.color=color')),/Undefined color key/)
  assert.match(messages(script('const editorTheme={"--private":"var(--ink)"}')),/Unregistered component color key/)
  assert.match(messages(script('const editorTheme={["background-color"]:"var(--unknown-key)"}')),/Undefined color key/)
  assert.match(messages(script('element.style.color="#"+"fff"')),/Raw color/)
  assert.deepEqual(script('const editorTheme={backgroundColor:"var(--paper)","border-color":"var(--ink)"};element.style.color="var(--ink)"'),[])
})

test('dynamic template literal colors cannot hide raw values or construct keys',()=>{
  for(const source of ['element.style.color=`rgb(${red} 0 0)`','element.style.background=`var(--${key})`','element.style.background=`${color}`','const theme={color:`color-mix(in srgb,${color},var(--ink))`}'])assert.notEqual(script(source).length,0,source)
  assert.deepEqual(script('element.style.background=`color-mix(in srgb,var(--paper) ${percent}%,var(--ink))`'),[])
  assert.deepEqual(script('element.style.boxShadow=`0 ${size}px 5px var(--shadow-color)`'),[])
  assert.deepEqual(script('element.style.setProperty("--viewport-top",(viewport?.offsetTop||0)+"px")'),[])
  assert.deepEqual(script('element.style.setProperty("--offset",active<0?"0px":index<active?"-18px":"26px")'),[])
  assert.deepEqual(script('element.style.setProperty("--filter",`url("#${id}")`)'),[])
})

test('runtime style setters, aliases and bulk assignments reject unproven colors',()=>{
  for(const source of [
    'element.style.color=runtimeColor',
    'element.style["background-color"]=runtimeColor',
    'element.style.setProperty("color",runtimeColor)',
    'element.style["setProperty"]("color",runtimeColor)',
    'element.style.setProperty(property,runtimeColor)',
    'element.style[method]("color",runtimeColor)',
    'const style=element.style;style.color=runtimeColor',
    'const style=element["style"];style.color=runtimeColor',
    'const {style:paint}=element;paint.color=runtimeColor',
    'Object.assign(element.style,{color:runtimeColor})',
    'Object["assign"](element.style,{color:runtimeColor})',
    'Object.assign(element.style,styleSource)',
    'element.style.cssText=css',
    'element.style["cssText"]=css',
    'element.setAttribute("style",css)',
    'element["setAttribute"]("style",css)',
    'element.style.color=colorVar("unknown")',
    'element.style.color=colorVar()',
    'const colors=["var(--ink)"];element.style.color=colors[index]',
  ])assert.notEqual(script(source).length,0,source)
  assert.deepEqual(script('element.style.color=colorVar("ink");element.style.backgroundColor=colorValue("paper","dark")'),[])
  assert.deepEqual(script('const colors=["var(--paper)","var(--ink)"] as const;element.style.backgroundColor=colors[index]!'),[])
  assert.deepEqual(script('const color="var(--ink)";element.style.color=color'),[])
  assert.deepEqual(script('Object.assign(element.style,{color:"var(--ink)"})'),[])
  assert.deepEqual(auditScript('aperture.style.setProperty("--game-mask-"+side,amount.toFixed(3)+"%")','web/frontend/src/homeShowcase.ts',contract),[])
  assert.match(messages(auditScript('const colors=["var(--ink)"] as const;card.style.setProperty("--work-day-color",String(runtimeColor))','web/frontend/src/homeWorkJourney.ts',contract)),/Dynamic custom property/)
})

test('Vue style blocks and template inline styles share the contract',()=>{
  const file='web/frontend/src/components/Example.vue'
  assert.match(messages(auditFile('<template><div :style="{\'background-color\':\'#fff\'}"/></template>',file,contract)),/Raw color/)
  assert.match(messages(auditFile('<template><svg fill="white"/></template><style>.x{color:#fff}</style>',file,contract)),/Raw color/)
  assert.deepEqual(auditFile('<template><div :style="{color:\'var(--ink)\'}"/></template><style>.x{color:var(--ink);white-space:normal}</style>',file,contract),[])
  assert.equal(auditFile('<template>\n<div/>\n</template>\n<style>\n.x{color:#fff}\n</style>',file,contract)[0].line,5)
})

test('inline SVG and JavaScript HTML sinks cannot conceal raw interface colors',()=>{
  assert.match(messages(css('.x{background-image:url("data:image/svg+xml,%3Csvg%3E%3Crect%20fill=%27%23fff%27/%3E%3C/svg%3E")}')),/Inline SVG: Raw color/)
  assert.match(messages(script('element.innerHTML=\'<svg><stop stop-color="white"/></svg>\'')),/Raw color/)
  assert.match(messages(script('element.insertAdjacentHTML("beforeend",\'<span style="color:#fff">X</span>\')')),/Raw color/)
})

test('Go HTML styles and raw UI color strings are checked without scanning prose',()=>{
  const file='internal/app/oidc.go'
  assert.match(messages(auditFile('var html=`<style>.x{color:#fff}</style>`',file,contract)),/Raw color/)
  assert.match(messages(auditFile('var manifest=Manifest{ThemeColor:"#ffffff"}',file,contract)),/Raw Go UI color/)
  assert.match(messages(auditHTML('<div style="color:&#35;fff"></div>','web/templates/example.html',contract)),/Raw color/)
  assert.deepEqual(auditFile('var html=`<a href="#fff" style="color:var(--ink)">white</a>`',file,contract),[])
})

test('only named image slots allow sampled RGB and numeric alpha',()=>{
  for(const expression of ['rgb(var(--moment-color-left) / .28)','rgb(var(--moment-color-right,0 0 0) / 22%)'])assert.deepEqual(css(`.x{box-shadow:0 1px 5px ${expression}}`,'web/frontend/src/momentsPublic.css'),[],expression)
  assert.notEqual(css('.x{color:rgb(var(--moment-color-left) / .28)}').length,0)
  for(const expression of ['rgb(var(--other) / .28)','rgb(var(--moment-color-left) / var(--ink))','rgb(var(--moment-color-left,white) / .28)','rgb(var(--moment-color-left) / 2)','rgb(var(--moment-color-left) / 101%)','rgb(var(--moment-color-left,999 0 0) / .2)'])assert.notEqual(css(`.x{color:${expression}}`,'web/frontend/src/momentsPublic.css').length,0,expression)
  assert.match(messages(css('.x{color:var(--moment-color-left)}','web/frontend/src/momentsPublic.css')),/Undefined color key/)
  assert.deepEqual(auditScript('preview.style.setProperty("--moment-color-left",left)','web/frontend/src/momentCover.ts',contract),[])
  assert.match(messages(script('preview.style.setProperty("--moment-color-left",left)')),/may only be written/)
  assert.match(messages(script('preview.style.setProperty("--unregistered",color)')),/Dynamic custom property/)
  assert.match(messages(script('preview.style.setProperty("--moment-color-"+side,color)')),/Dynamic color property names/)
  assert.match(messages(css('.x{--moment-color-left:0 0 0}','web/frontend/src/momentsPublic.css')),/may only be written/)
  assert.match(messages(auditScript('preview.style.setProperty("--moment-color-left","0 0 0")','web/frontend/src/momentCover.ts',contract)),/cannot contain literal colors/)
})

test('runtime scene color slots accept only a static array of registered roles',()=>{
  const file='web/frontend/src/homeWorkJourney.ts'
  assert.deepEqual(auditScript('const colors=["var(--paper)","var(--ink)"] as const;card.style.setProperty("--work-day-color",colors[index])',file,contract),[])
  assert.deepEqual(auditScript('const colors=["paper","ink"] as const;card.style.setProperty("--work-day-color",colorVar(colors[index]))',file,contract),[])
  assert.deepEqual(auditScript('const colors=["var(--paper)","var(--ink)"] as const;card.style.setProperty("--work-day-color",colors[Math.max(0,Math.min(1,Number(element.dataset.level)||0))]!)',file,contract),[])
  assert.match(messages(auditScript('const colors=["var(--paper)","var(--unknown)"];card.style.setProperty("--work-day-color",colors[index])',file,contract)),/Undefined color key|Dynamic custom property/)
  assert.match(messages(auditScript('card.style.setProperty("--work-day-color",runtimeColor)',file,contract)),/Dynamic custom property/)
  assert.match(messages(script('const colors=["var(--paper)"];card.style.setProperty("--work-day-color",colors[index])')),/Dynamic custom property/)
})

test('the shared registry accepts supported colors and rejects malformed values, injections and cycles',()=>{
  const registry=value=>({version:1,colors:{ink:{light:value,dark:value,description:'Text',category:'interface'}}})
  for(const value of ['#fff','#1234','#112233','#11223344','red','CanvasText','currentColor','rgb(0 10 20 / .4)','rgba(0,10,20,.4)','hsl(240deg 20% 50% / 50%)','oklch(.7 .2 40)','color-mix(in srgb,white 20%,black)'])assert.doesNotThrow(()=>createContract(registry(value)),value)
  for(const value of ['#12345','not-a-color','rgb(0 0)','rgb(garbage)','hsl(30deg 20deg 50deg)','var(--missing)','var(--ink())','color-mix(in srgb(),white,black)','color-mix(in srgb,white,black','white;--ink:black','url(data:text/css,red)','expression(alert(1))','"white"'])assert.throws(()=>createContract(registry(value)),/Invalid registry color/,value)
  assert.throws(()=>createContract(registry('var(--ink)')),/Cyclic color reference/)
  assert.throws(()=>createContract({...registry('#fff'),modes:{print:{ink:'var(--ink)'}}}),/Cyclic color reference/)
  assert.throws(()=>createContract({...registry('#fff'),aliases:{text:'constructor'}}),/Invalid color alias/)
})

test('malformed styles, script, Vue and registry fail closed',()=>{
  for(const source of ['.x{color:var(--ink}', '.x{color:"white}', '.x{color:var(--ink))}', '.x{color:rgb(0 0 0}'])assert.notEqual(css(source).length,0,source)
  assert.match(messages(script('const theme = {color:')),/Malformed script/)
  assert.match(messages(auditFile('<template><div></template>','web/frontend/src/broken.vue',contract)),/Malformed Vue/)
  for(const registry of [{}, {version:1,colors:{}}, {version:1,colors:{ink:{light:'#fff'}}}, {version:1,colors:{},aliases:{one:'two',two:'one'}}, {version:1,colors:{ink:{light:'#fff',dark:'#000',description:'Text',category:'interface'}},aliases:{bad:'missing'}}, {version:1,colors:{ink:{light:'#fff',dark:'#000',description:'Text',category:'interface'}},modes:{print:{unknown:'#fff'}}}])assert.throws(()=>createContract(registry))
})
