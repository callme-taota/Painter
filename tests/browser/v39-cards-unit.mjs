// Node 24, no browser/service/database/provider. V24 compatibility remains separate.
import assert from 'node:assert/strict'
import {registerHooks,createRequire} from 'node:module'
import {pathToFileURL} from 'node:url'
registerHooks({resolve(specifier,context,next){if(['./articleEmbeds','./i18n'].includes(specifier)&&context.parentURL?.includes('/web/frontend/src/'))return {url:new URL(specifier+'.ts',context.parentURL).href,shortCircuit:true};if(specifier==='./articleCards'&&context.parentURL?.endsWith('/editorMarkdown.ts'))return {url:new URL('./articleCards.ts',context.parentURL).href,shortCircuit:true};return next(specifier,context)},load(url,context,next){if(url.endsWith('.css'))return {format:'module',source:'',shortCircuit:true};return next(url,context)}})
const require=createRequire(new URL('../../web/frontend/package.json',import.meta.url))
const {MarkdownManager}=await import(pathToFileURL(require.resolve('@tiptap/markdown')))
const {StarterKit}=await import(pathToFileURL(require.resolve('@tiptap/starter-kit')))
const {ArticleCardNode,ArticleReferenceLink,articleCardMarkdown,parseArticleCard,parseArticleReference,articleCardSpec,articleReferenceTitle}=await import('../../web/frontend/src/articleCards.ts')
const {markdownCardAt}=await import('../../web/frontend/src/editorMarkdown.ts')
const manager=new MarkdownManager({extensions:[StarterKit.configure({link:false}),ArticleReferenceLink,ArticleCardNode],markedOptions:{gfm:true,breaks:false}})
function nodes(document,type){const result=[];function walk(node){if(node.type===type)result.push(node);for(const child of node.content||[])walk(child)}walk(document);return result}
let checks=0
function check(name,fn){fn();checks++;console.log('PASS '+name)}
for(const type of ['moment','phrase']){
 const card={v:1,type,ref_id:'42',url:type==='moment'?'/moments/42':'/phrases/42'}
 check(type+' reference-only Markdown roundtrips preserve identity and surrounding text',()=>{const raw=JSON.stringify(card,null,2),source='正文之前\n\n```painter-card\n'+raw+'\n```\n\n正文之后';assert.deepEqual(parseArticleCard(raw),card);const first=manager.parse(source);assert.equal(nodes(first,'articleCard').length,1);assert.equal(nodes(first,'articleCard')[0].attrs.snapshot,raw);first.content.push({type:'paragraph',content:[{type:'text',text:'继续编辑'}]});const second=manager.parse(manager.serialize(first));assert.equal(nodes(second,'articleCard')[0].attrs.snapshot,raw);assert.ok(manager.serialize(second).includes('继续编辑'))})
 check(type+' source re-edit and safe DOM spec contain no cached content or image',()=>{const source=articleCardMarkdown(card),found=markdownCardAt(source,source.indexOf('painter-card'));assert.deepEqual(found.card,card);const spec=JSON.stringify(articleCardSpec(card));assert.ok(spec.includes(card.url));assert.ok(!spec.includes('target'));assert.ok(!spec.includes('img'));assert.equal(parseArticleReference(articleReferenceTitle(card),card.url),null)})
 check(type+' snapshots, destination changes and invalid identities fail closed',()=>{for(const fields of [{title:'private text'},{title:null},{description:'private text'},{image_url:'/media/0123456789abcdef'},{metadata_source:'author'},{url:'https://example.org'+card.url},{ref_id:'01'},{ref_id:'9223372036854775808'},{ref_id:42}])assert.equal(parseArticleCard(JSON.stringify({...card,...fields})),null)})
}
check('unknown versions and types keep ordinary code',()=>{for(const card of [{v:2,type:'moment',ref_id:'1',url:'/moments/1'},{v:1,type:'future-local',ref_id:'1',url:'/moments/1'}]){const parsed=manager.parse(articleCardMarkdown(card));assert.equal(nodes(parsed,'articleCard').length,0);assert.equal(nodes(parsed,'codeBlock').length,1)}})
console.log(JSON.stringify({passed:checks}))
