// Pure schema/render/Markdown checks. No server, database, browser or network.
import assert from 'node:assert/strict'
import {registerHooks,createRequire} from 'node:module'
import {pathToFileURL} from 'node:url'
registerHooks({load(url,context,next){if(url.endsWith('.css'))return {format:'module',source:'',shortCircuit:true};return next(url,context)}})
const require=createRequire(new URL('../../web/frontend/package.json',import.meta.url))
const {MarkdownManager}=await import(pathToFileURL(require.resolve('@tiptap/markdown')))
const {StarterKit}=await import(pathToFileURL(require.resolve('@tiptap/starter-kit')))
const {ArticleCardNode,ArticleReferenceLink,articleCardMarkdown,articleCardSpec,articleCardTitle,parseArticleCard,compileArticlePanel}=await import('../../web/frontend/src/articleCards.ts')
const manager=new MarkdownManager({extensions:[StarterKit.configure({link:false}),ArticleReferenceLink,ArticleCardNode],markedOptions:{gfm:true,breaks:false}})
const text=(role='body',value='正文')=>({kind:'text',role,text:value})
const stack=children=>({kind:'stack',children})
const card=children=>({v:3,type:'layout',root:{kind:'panel',children}})
const image={kind:'image',src:'/media/12345678-abcd-1234-abcd-123456789abc',alt:''}
const source={kind:'link',text:'来源',url:'https://example.org/report'}
const mixed=card([text('title','组合研究说明'),{kind:'grid',children:[stack([text('label','指标'),text('value','12%'),source]),stack([image,text('body','当时判断。\r\n保留口径。')])]},stack([text('detail','存档未复核'),source])])
const parse=value=>parseArticleCard(JSON.stringify(value))
let passed=0
function check(name,run){run();passed++;console.log('PASS '+name)}
function nodes(document,type){const result=[];function walk(node){if(node.type===type)result.push(node);for(const child of node.content||[])walk(child)}walk(document);return result}
function specs(node,tag){return Array.isArray(node)?[...(node[0]===tag?[node]:[]),...node.slice(2).flatMap(child=>specs(child,tag))]:[]}

check('mixed layout retains exact snapshot through document edits and Markdown roundtrip',()=>{
 const raw=JSON.stringify(mixed,null,2),document=manager.parse('正文\n\n```painter-card\n'+raw+'\n```\n\n结尾')
 assert.deepEqual(parseArticleCard(raw),mixed);assert.equal(nodes(document,'articleCard').length,1)
 document.content.push({type:'paragraph',content:[{type:'text',text:'追加文字'}]})
 const serialized=manager.serialize(document),again=manager.parse(serialized)
 assert.equal(nodes(again,'articleCard')[0].attrs.snapshot,raw);assert.ok(serialized.includes('追加文字'));assert.equal(articleCardTitle(mixed),'组合研究说明')
})
check('unknown versions remain editable fences and mismatched layout versions are rejected',()=>{
 for(const value of [{...mixed,v:4},{...mixed,type:'future-layout'}]){const document=manager.parse(articleCardMarkdown(value));assert.equal(nodes(document,'articleCard').length,0);assert.equal(nodes(document,'codeBlock').length,1)}
 for(const value of [{...mixed,v:1},{...mixed,v:2},{v:1,type:'layout',url:'https://example.org',title:'x'}])assert.equal(parse(value),null)
})
check('all node boundaries reject unknown keys, null, nested panels and invalid grid children',()=>{
 for(const root of [null,[],{kind:'stack',children:[text()]},{kind:'panel',children:[]},{kind:'panel',children:[text()],tone:null},{kind:'panel',children:[text()],style:'color:red'}])assert.equal(parse({...mixed,root}),null)
 for(const node of [null,{},[],{kind:'panel',children:[text()]},{kind:'grid',children:[text()]},{kind:'stack',children:[]},{...text(),html:'<b>x</b>'},{...source,target:'_self'},{...image,alt:null},{kind:'image',src:image.src},{kind:'text',role:'html',text:'x'}])assert.equal(parse(card([node])),null)
 assert.equal(parse({...mixed,title:'not a v3 field'}),null)
})
check('duplicate keys including escaped names are rejected before normalization',()=>{
 for(const raw of [JSON.stringify(mixed).replace('"v":3','"v":3,"v":3'),JSON.stringify(card([text()])).replace('"text":"正文"','"text":"正文","te\\u0078t":"重复"'),JSON.stringify(card([image])).replace('"alt":""','"alt":"","alt":"重复"')])assert.equal(parseArticleCard(raw),null)
 assert.ok(parse(card([text('body','保留字符串中的 { "text": "值" } 和 \\ 转义。')])))
})
check('depth, node count, children count and image count have exact limits',()=>{
 let nested=text();for(let i=0;i<4;i++)nested=stack([nested]);assert.ok(parse(card([nested])));assert.equal(parse(card([stack([nested])])),null)
 const grid={kind:'grid',children:Array.from({length:6},()=>stack(Array.from({length:9},()=>text())))}
 const at64=card([text(),text(),grid]);assert.ok(parse(at64));grid.children[0].children.push(text());assert.equal(parse(at64),null)
 assert.ok(parse(card(Array.from({length:6},()=>image))));assert.equal(parse(card(Array.from({length:7},()=>image))),null)
 assert.ok(parse(card(Array.from({length:12},()=>text()))));assert.equal(parse(card(Array.from({length:13},()=>text()))),null)
 assert.equal(parse(card([{kind:'grid',children:Array.from({length:7},()=>stack([text()]))}])),null)
})
check('text roles enforce lengths, required content and newline/control policy',()=>{
 for(const [role,limit] of Object.entries({title:200,label:120,value:100,body:2000,detail:500})){assert.ok(parse(card([text(role,'字'.repeat(limit))])));assert.equal(parse(card([text(role,'字'.repeat(limit+1))])),null);for(const value of ['', ' ', '\t', null])assert.equal(parse(card([text(role,value)])),null)}
 for(const role of ['body','detail'])assert.ok(parse(card([text(role,'第一行\r\n第二行\n第三行')])))
 for(const role of ['title','label','value'])assert.equal(parse(card([text(role,'第一行\n第二行')])),null)
 for(const value of ['字\u0000字','字\t字','字\u0085字'])assert.equal(parse(card([text('body',value)])),null)
 assert.equal(parse(card(Array.from({length:5},()=>text('body','x'.repeat(2000))))),null)
})
check('links and uploaded images reject executable, remote and malformed destinations',()=>{
 for(const url of ['javascript:alert(1)','http://example.org','https://127.0.0.1','https://user:pass@example.org','https://example.org:443','https://example.org/%0a'])assert.equal(parse(card([{...source,url}])),null)
 for(const src of ['https://example.org/image.png','data:image/png,x','/media/12345678-abcd-1234-abcd-123456789abc?x=1','/media/not-an-id'])assert.equal(parse(card([{...image,src}])),null)
 assert.ok(parse(card([image])));assert.equal(parse(card([{...image,alt:'x'.repeat(201)}])),null)
})
check('generic DOM specs keep literal text, source semantics and uploaded image attributes',()=>{
 const literal='</p><script>never()</script>',value=card([text('body',literal+'\r\n第二行'),source,image]),spec=articleCardSpec(value)
 assert.equal(spec[1].class,'content-panel content-panel--layout');assert.deepEqual(specs(spec,'p')[0],['p',{class:'content-panel__body'},literal,['br'],'第二行'])
 assert.deepEqual(specs(spec,'a')[0],['a',{class:'content-panel__source',href:source.url,target:'_blank',rel:'noopener noreferrer'},source.text])
 assert.deepEqual(specs(spec,'img')[0],['img',{class:'content-panel__image',src:image.src,alt:'',loading:'lazy'}])
})
check('v2 presets compile into the same nodes while preserving prior markup',()=>{
 const value={v:2,type:'metrics',title:'指标',metrics:[{label:'名称',value:'12',detail:'说明',source:{label:'来源',url:source.url}}]},tree=compileArticlePanel(value),spec=articleCardSpec(value)
 assert.equal(tree.children[1].kind,'grid');assert.equal(tree.children[1].children[0].kind,'stack');assert.equal(spec[1].class,'content-panel content-panel--metrics');assert.equal(spec[3][2][1].class,'content-panel__item')
})
console.log(JSON.stringify({passed}))
