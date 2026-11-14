// Node 24: pure document conversion, without a server, browser, database or fetch.
import assert from 'node:assert/strict'
import {registerHooks,createRequire} from 'node:module'
import {pathToFileURL} from 'node:url'
registerHooks({resolve(specifier,context,next){if(specifier==='./articleCards'&&context.parentURL?.endsWith('/editorMarkdown.ts'))return {url:new URL('./articleCards.ts',context.parentURL).href,shortCircuit:true};return next(specifier,context)},load(url,context,next){if(url.endsWith('.css'))return {format:'module',source:'',shortCircuit:true};return next(url,context)}})
const require=createRequire(new URL('../../web/frontend/package.json',import.meta.url))
const {MarkdownManager}=await import(pathToFileURL(require.resolve('@tiptap/markdown')))
const {StarterKit}=await import(pathToFileURL(require.resolve('@tiptap/starter-kit')))
const {ArticleCardNode,ArticleReferenceLink,articleCardMarkdown,articleReferenceTitle,parseArticleCard,parseArticleReference,articleCardSpec}=await import('../../web/frontend/src/articleCards.ts')
const {markdownCardAt}=await import('../../web/frontend/src/editorMarkdown.ts')
const manager=new MarkdownManager({extensions:[StarterKit.configure({link:false}),ArticleReferenceLink,ArticleCardNode],markedOptions:{gfm:true,breaks:false}})
const fixtures=[
 {v:2,type:'callout',title:'存档说明',tone:'warning',body:'当时的数据快照。\r\n未重新核验。'},
 {v:2,type:'metrics',title:'当期指标',metrics:[{label:'示例指标',value:'-1.055B',detail:'示例窗口\n原始口径保留',source:{label:'[01] 来源',url:'https://example.org/report?issue=1'}}]},
 {v:2,type:'summary',entries:[{title:'先防守',body:'保留作者判断。'},{title:'产品落地',body:'第二项说明。'}]},
]
let passed=0
function check(name,run){run();passed++;console.log('PASS '+name)}
function nodes(document,type){const result=[];function walk(node){if(node.type===type)result.push(node);for(const child of node.content||[])walk(child)}walk(document);return result}
const parse=value=>parseArticleCard(JSON.stringify(value))
for(const card of fixtures){
 check(card.type+' keeps exact snapshots through rich-document editing and source re-edit',()=>{
  const raw=JSON.stringify(card,null,2),source='## 原文\n\n```painter-card\n'+raw+'\n```\n\n末尾文字'
  assert.deepEqual(parseArticleCard(raw),card)
  const document=manager.parse(source);assert.equal(nodes(document,'articleCard').length,1)
  document.content.push({type:'paragraph',content:[{type:'text',text:'追加内容'}]})
  const serialized=manager.serialize(document),again=manager.parse(serialized)
  assert.equal(nodes(again,'articleCard')[0].attrs.snapshot,raw)
  assert.ok(serialized.includes('追加内容'));assert.deepEqual(markdownCardAt(source,source.indexOf('painter-card')).card,card)
  assert.equal(parseArticleReference(articleReferenceTitle(card),'https://example.org'),null)
 })
}
check('panels never accept v1 reference fields, unknown root fields or mismatched versions',()=>{
 for(const card of fixtures)for(const extra of [{url:'https://example.org'},{ref_id:'1'},{html:'<script>x()</script>'},{metrics:null},{v:1},{v:3},{title:null},{title:'x'.repeat(201)}])assert.equal(parse({...card,...extra}),null)
 for(const card of [{...fixtures[0],type:'future-panel'},{v:2,type:'github',url:'https://github.com/openai/codex',title:'x'}]){const doc=manager.parse(articleCardMarkdown(card));assert.equal(nodes(doc,'articleCard').length,0);assert.equal(nodes(doc,'codeBlock').length,1)}
})
check('nested metrics and sources reject missing fields, arrays, null and unknown keys',()=>{
 for(const item of [null,[],{}, {label:'x'}, {label:'x',value:'1',html:'x'}, {label:'x',value:'1',detail:null}, {label:'x',value:'1',source:null}, {label:'x',value:'1',source:[]}, {label:'x',value:'1',source:{label:'source'}}, {label:'x',value:'1',source:{label:'source',url:'https://example.org',target:'_blank'}}, {label:'x',value:'1',source:{label:'',url:'https://example.org'}}])assert.equal(parse({v:2,type:'metrics',metrics:[item]}),null)
 assert.ok(parse({v:2,type:'metrics',metrics:[{label:'x',value:'0',detail:''}]}))
})
check('summary entries and callout tones are strictly typed and bounded',()=>{
 for(const entry of [null,[],{}, {title:'x'}, {title:'',body:'x'}, {title:'x',body:null}, {title:'x',body:'x',url:'https://example.org'}, {title:'x'.repeat(201),body:'x'}, {title:'x',body:'x'.repeat(2001)}])assert.equal(parse({v:2,type:'summary',entries:[entry]}),null)
 for(const tone of ['other',['note'],null,{},1])assert.equal(parse({...fixtures[0],tone}),null)
 for(const body of ['', ' ', null, [], 'x'.repeat(2001)])assert.equal(parse({...fixtures[0],body}),null)
 for(const type of ['metrics','summary']){const key=type==='metrics'?'metrics':'entries',item=type==='metrics'?{label:'x',value:'1'}:{title:'x',body:'x'};assert.equal(parse({v:2,type,[key]:[]}),null);assert.equal(parse({v:2,type,[key]:Array(7).fill(item)}),null);assert.ok(parse({v:2,type,[key]:Array(6).fill(item)}))}
})
check('field limits count Unicode, snapshots stay within 8 KiB and multiline controls stay inert',()=>{
 assert.ok(parse({v:2,type:'callout',tone:'insight',body:'🌿'.repeat(2000)}))
 assert.equal(parse({v:2,type:'callout',tone:'insight',body:'🌿'.repeat(2001)}),null)
 assert.equal(parse({v:2,type:'summary',entries:Array(6).fill({title:'x',body:'x'.repeat(2000)})}),null)
 for(const body of ['x\ty','x\u0000y','x\u0085y'])assert.equal(parse({...fixtures[0],body}),null)
 for(const [key,value] of [['label','x'.repeat(121)],['value','x'.repeat(101)],['detail','x'.repeat(501)]])assert.equal(parse({v:2,type:'metrics',metrics:[{label:'x',value:'1',[key]:value}]}),null)
})
check('source URLs use the public HTTPS allowlist and enforce source label limits',()=>{
 for(const url of ['javascript:alert(1)','data:text/html,<script>x</script>','http://example.org','//example.org','https://localhost','https://127.0.0.1','https://example.org/%0a','https://user:pass@example.org','https://example.org:443'])assert.equal(parse({v:2,type:'metrics',metrics:[{label:'x',value:'1',source:{label:'source',url}}]}),null)
 assert.equal(parse({v:2,type:'metrics',metrics:[{label:'x',value:'1',source:{label:'x'.repeat(121),url:'https://example.org'}}]}),null)
})
check('DOM specs keep text literal, normalize line breaks and provide safe native source links',()=>{
 const literal='</p><img src=x onerror=alert(1)><script>never()</script>',card={v:2,type:'callout',tone:'note',body:literal+'\r\n第二行'}
 const spec=articleCardSpec(card);assert.equal(spec[0],'div');assert.equal(spec[1].class,'content-panel content-panel--callout content-panel--note')
 assert.deepEqual(spec[2],['p',{class:'content-panel__body'},literal,['br'],'第二行'])
 const metricSpec=articleCardSpec(fixtures[1]),source=metricSpec[3][2].at(-1)
 assert.equal(source[0],'a');assert.equal(source[1].href,fixtures[1].metrics[0].source.url);assert.equal(source[1].target,'_blank');assert.equal(source[1].rel,'noopener noreferrer')
})
console.log(JSON.stringify({passed}))
