// Versioned Markdown and fixed-player boundary. No app or provider requests.
import assert from 'node:assert/strict'
import {registerHooks,createRequire} from 'node:module'
import {pathToFileURL} from 'node:url'
registerHooks({resolve(specifier,context,next){if(['./articleEmbeds','./i18n'].includes(specifier)&&context.parentURL?.includes('/web/frontend/src/'))return {url:new URL(specifier+'.ts',context.parentURL).href,shortCircuit:true};return next(specifier,context)},load(url,context,next){if(url.endsWith('.css'))return {format:'module',source:'',shortCircuit:true};return next(url,context)}})
const require=createRequire(new URL('../../web/frontend/package.json',import.meta.url))
const {MarkdownManager}=await import(pathToFileURL(require.resolve('@tiptap/markdown')))
const {StarterKit}=await import(pathToFileURL(require.resolve('@tiptap/starter-kit')))
const {ArticleCardNode,ArticleReferenceLink,articleCardMarkdown,articleReferenceMarkdown,parseArticleCard,parseArticleReference,articleCardSpec,articleReferenceTitle}=await import('../../web/frontend/src/articleCards.ts')
const {canonicalBilibiliURL,bilibiliPlayerURL,socialPlatform}=await import('../../web/frontend/src/articleEmbeds.ts')
const manager=new MarkdownManager({extensions:[StarterKit.configure({link:false}),ArticleReferenceLink,ArticleCardNode]})
const social={v:1,type:'link',display:'social',url:'https://www.xiaohongshu.com/explore/example',title:'周末的照片',subtitle:'@一位作者',description:'第一行分享文字\n第二行 <script>仅作文本</script>',image_url:'/media/'+'a'.repeat(48),metadata_source:'author'}
const video={v:1,type:'bilibili',url:'https://www.bilibili.com/video/BV1B7411m7LV/?p=2',title:'视频参考',description:'文章中的视频说明',image_url:social.image_url,metadata_source:'author'}
let checks=0
function check(name,fn){fn();checks++;console.log('PASS '+name)}
function nodes(document,type){const found=[];function walk(n){if(n.type===type)found.push(n);for(const c of n.content||[])walk(c)}walk(document);return found}
for(const card of [social,video])check(card.type+' keeps snapshot and neighboring prose through Markdown edits',()=>{
 const source='前文\n\n'+articleCardMarkdown(card)+'\n\n后文',first=manager.parse(source),again=manager.parse(manager.serialize(first));assert.equal(nodes(again,'articleCard').length,1);assert.deepEqual(parseArticleCard(nodes(again,'articleCard')[0].attrs.snapshot),card);assert(manager.serialize(again).includes('后文'))
})
check('social remains block-only and multi-line text does not weaken ordinary links',()=>{
 assert(articleReferenceMarkdown(social).startsWith('```painter-card'));assert.equal(parseArticleReference(articleReferenceTitle(social),social.url),null)
 assert.equal(parseArticleCard(JSON.stringify({...social,display:'card'})),null)
 const spec=articleCardSpec(social);assert.equal(spec[1].class,'content-social');assert(JSON.stringify(spec).includes(JSON.stringify(social.description)));assert(!JSON.stringify(spec).includes('iframe'))
})
check('new snapshots reject wrong optional types, unknown keys, remote posters, duplicates and null',()=>{
 for(const card of [social,video]){
  for(const field of ['image_url','metadata_source','resolved_at'])for(const value of [0,false,[],{},null])assert.equal(parseArticleCard(JSON.stringify({...card,[field]:value})),null)
  for(const extra of [{stats:false},{stats:{}},{ref_id:''},{iframe:'https://evil.test/'}])assert.equal(parseArticleCard(JSON.stringify({...card,...extra})),null)
  assert.equal(parseArticleCard(JSON.stringify({...card,image_url:'https://i0.hdslb.com/poster.jpg'})),null)
  assert.equal(parseArticleCard(JSON.stringify(card).slice(0,-1)+',"title":"overridden"}'),null)
 }
})
check('normalization discards only two bounded tracking keys and keeps strict page identity',()=>{
 const base='https://www.bilibili.com/video/BV1B7411m7LV/'
 assert.equal(canonicalBilibiliURL(base.slice(0,-1)+'?p=1'),base)
 assert.equal(canonicalBilibiliURL(base+'?spm_id_from=333.337&vd_source=a%2Bb&p=2'),base+'?p=2')
 for(const tail of ['?p=0','?p=02','?p=1001','?p=2&p=3','?p=%32','?next=https://evil.test/','?spm_id_from=a&spm_id_from=b','?vd_source=%ZZ','?vd_source='+('x'.repeat(513)),'?','#part'])assert.equal(canonicalBilibiliURL(base+tail),null)
 for(const url of [base.replace('www.bilibili.com','www.bilibili.com.evil.test'),base.replace('https://','http://'),base.replace('www.','evil@www.'),'https://player.bilibili.com/player.html?bvid=BV1B7411m7LV','https://b23.tv/short'])assert.equal(canonicalBilibiliURL(url),null)
 assert.equal(parseArticleCard(JSON.stringify({...video,url:base+'?vd_source=abc'})),null)
})
check('player URLs are reconstructed with no autoplay and legacy IDs never lose precision',()=>{
 const url=new URL(bilibiliPlayerURL(video.url));assert.equal(url.origin,'https://player.bilibili.com');assert.equal(url.pathname,'/player.html');assert.equal(url.searchParams.get('p'),'2');assert.equal(url.searchParams.get('autoplay'),'0')
 const av='999999999999999999',legacy='https://www.bilibili.com/video/av'+av+'/';assert.equal(new URL(bilibiliPlayerURL(legacy)).searchParams.get('aid'),av)
 assert.equal(bilibiliPlayerURL(video.url+'&vd_source=x'),null)
 const spec=JSON.stringify(articleCardSpec(video));assert(spec.includes('data-content-video')&&spec.includes('hidden'));assert(!spec.includes('iframe')&&!spec.includes('player.bilibili.com'))
})
check('platform identity uses hostname boundaries, not lookalike substrings',()=>{
 assert.equal(socialPlatform('https://m.weibo.cn/1'),'微博');assert.equal(socialPlatform('https://mobile.x.com/a'),'X');assert.equal(socialPlatform('https://www.xhslink.com/a'),'小红书');assert.equal(socialPlatform('https://bilibili.com.evil.test/'),'分享')
})
console.log(JSON.stringify({passed:checks}))
