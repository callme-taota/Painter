// Run with Node 24. No browser, application server, provider request or write.
import assert from 'node:assert/strict'
import {registerHooks,createRequire} from 'node:module'
import {pathToFileURL} from 'node:url'
registerHooks({resolve(specifier,context,next){if(['./articleEmbeds','./i18n'].includes(specifier)&&context.parentURL?.includes('/web/frontend/src/'))return {url:new URL(specifier+'.ts',context.parentURL).href,shortCircuit:true};if(specifier==='./articleCards'&&context.parentURL?.endsWith('/editorMarkdown.ts'))return {url:new URL('./articleCards.ts',context.parentURL).href,shortCircuit:true};return next(specifier,context)},load(url,context,next){if(url.endsWith('.css'))return {format:'module',source:'',shortCircuit:true};return next(url,context)}})
const require=createRequire(new URL('../../web/frontend/package.json',import.meta.url))
const {MarkdownManager}=await import(pathToFileURL(require.resolve('@tiptap/markdown')))
const {StarterKit}=await import(pathToFileURL(require.resolve('@tiptap/starter-kit')))
const {getSchema}=await import(pathToFileURL(require.resolve('@tiptap/core')))
const {ArticleCardNode,ArticleReferenceLink,articleCardMarkdown,articleReferenceMarkdown,parseArticleCard,parseArticleReference,articleCardSpec}=await import('../../web/frontend/src/articleCards.ts')
const {markdownCardAt}=await import('../../web/frontend/src/editorMarkdown.ts')
const manager=new MarkdownManager({extensions:[StarterKit.configure({link:false}),ArticleReferenceLink,ArticleCardNode],markedOptions:{gfm:true,breaks:false}})
const fixture={v:1,type:'github',url:'https://github.com/openai/codex',title:'openai/codex',description:'可搜索的说明 <script>never executable</script>',image_url:'https://avatars.githubusercontent.com/u/14957082?v=4',stats:{stars:0},metadata_source:'provider'}
let checks=0
function check(name,fn){fn();checks++;console.log('PASS '+name)}
function nodes(document,type){const result=[];function walk(node){if(node.type===type)result.push(node);for(const child of node.content||[])walk(child)}walk(document);return result}
check('empty document fills with a paragraph rather than an implicit invalid card',()=>{
 const schema=getSchema([StarterKit.configure({link:false}),ArticleReferenceLink,ArticleCardNode]),empty=schema.topNodeType.createAndFill().toJSON()
 assert.equal(empty.content[0].type,'paragraph');assert.equal(nodes(empty,'articleCard').length,0);assert.equal(manager.serialize(empty),'')
 for(const source of ['', ' ', '\n', '\n\n', '  \n  '])assert.equal(manager.serialize(manager.parse(source)),'')
})
check('known snapshot becomes an atom and preserves complete JSON through two edits',()=>{
 const raw=JSON.stringify(fixture,null,2),source='# 标题\n\n```painter-card\n'+raw+'\n```\n\n正文 *继续*。'
 const first=manager.parse(source),cards=nodes(first,'articleCard');assert.equal(cards.length,1);assert.equal(cards[0].attrs.snapshot,raw)
 first.content.push({type:'paragraph',content:[{type:'text',text:'追加内容'}]})
 const output=manager.serialize(first),second=manager.parse(output);assert.equal(nodes(second,'articleCard')[0].attrs.snapshot,raw);assert.deepEqual(JSON.parse(nodes(second,'articleCard')[0].attrs.snapshot),fixture)
})
check('inline Unicode Emoji text and snapshot survive normal link Markdown roundtrips',()=>{
 const card={v:1,type:'link',url:'https://example.org/reference?q=1&lang=zh',title:'网页标题',description:'正文参考'},label='🔎 中文 [参考] *原文* ~~字面~~'
 const first=manager.parse('## '+articleReferenceMarkdown(card,label)),serialized=manager.serialize(first),second=manager.parse(serialized)
 const content=nodes(second,'text').filter(n=>n.marks?.some(m=>m.type==='link'));assert.equal(content.map(n=>n.text).join(''),label)
 for(const text of content){const mark=text.marks.find(m=>m.type==='link');assert.deepEqual(parseArticleReference(mark.attrs.title,mark.attrs.href),card)}
 assert.equal(nodes(second,'articleCard').length,0)
})
check('literal code, unknown types and future versions never become card atoms',()=>{
 for(const source of ['```json\n'+JSON.stringify(fixture)+'\n```','````markdown\n'+articleCardMarkdown(fixture)+'\n````','```painter-card\n{"v":2,"type":"github","url":"javascript:alert(1)"}\n```','```painter-card\n{"v":1,"type":"future-widget","title":"keep raw"}\n```']){
  const json=manager.parse(source);assert.equal(nodes(json,'articleCard').length,0);assert.equal(nodes(json,'codeBlock').length,1);assert.ok(manager.serialize(json).includes(nodes(json,'codeBlock')[0].content?.[0]?.text||''))
 }
})
check('bad URLs, external artwork and forged reference destinations fail closed',()=>{
 for(const url of ['javascript:alert(1)','https://127.0.0.1/a','https://localhost/a','https://evil.test/repo'])assert.equal(parseArticleCard(JSON.stringify({...fixture,url})),null)
 assert.equal(parseArticleCard(JSON.stringify({...fixture,image_url:'https://evil.test/image.jpg'})),null)
 const card={v:1,type:'link',url:'https://example.org',title:'网页'};const parsed=manager.parse(articleReferenceMarkdown(card));const mark=nodes(parsed,'text')[0].marks.find(m=>m.type==='link');assert.equal(parseArticleReference(mark.attrs.title,'https://other.example'),null)
})
check('DOM specs retain metadata as text and preserve known zero without fake unknown counts',()=>{
 const spec=articleCardSpec(fixture),serialized=JSON.stringify(spec);assert.ok(serialized.includes('content-card--github'));assert.ok(serialized.includes('★ 0'));assert.ok(!serialized.includes('Forks 0'))
 const fields=[];function walk(spec){if(typeof spec==='string')fields.push(spec);else if(Array.isArray(spec))for(const child of spec.slice(2))walk(child)}walk(spec);assert.ok(fields.includes(fixture.description))
})
check('source card editing uses token ranges and never sees literal nested sample cards',()=>{const source='# title\n\n'+articleCardMarkdown(fixture)+'\n\nend';const at=source.indexOf('painter-card');const match=markdownCardAt(source,at);assert.deepEqual(match.card,fixture);assert.ok(source.slice(match.from,match.to).startsWith('```painter-card'));assert.equal(markdownCardAt('````markdown\n'+articleCardMarkdown(fixture)+'\n````',25),null)})
console.log(JSON.stringify({passed:checks}))
