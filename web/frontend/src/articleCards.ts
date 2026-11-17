import {canonicalBilibiliURL,socialPlatform} from './articleEmbeds'
import {t} from './i18n'
import {Node as TiptapNode} from '@tiptap/core'
import {DOMSerializer,type DOMOutputSpec} from '@tiptap/pm/model'
import Link,{isAllowedUri} from '@tiptap/extension-link'
import {NodeSelection} from '@tiptap/pm/state'
import './articleCards.css'

export type ArticleReferenceType='github'|'steam'|'apple_music'|'link'|'moment'|'phrase'|'bilibili'
export type ArticlePanelType='callout'|'metrics'|'summary'
export type ArticleCardType=ArticleReferenceType|ArticlePanelType|'layout'
export type ArticleCardV1={v:1;type:ArticleReferenceType;url:string;title?:string;ref_id?:string;description?:string;subtitle?:string;image_url?:string;stats?:{stars?:number;forks?:number};metadata_source?:'provider'|'author';resolved_at?:string;display?:'card'|'title'|'social'}
export type ArticleCardSource={label:string;url:string}
export type ArticleCardMetric={label:string;value:string;detail?:string;source?:ArticleCardSource}
export type ArticleCardEntry={title:string;body:string}
export type ArticlePanelCard={v:2;title?:string}&({type:'callout';tone:'note'|'insight'|'warning';body:string}|{type:'metrics';metrics:ArticleCardMetric[]}|{type:'summary';entries:ArticleCardEntry[]})
export type ArticleTextRole='title'|'label'|'value'|'body'|'detail'
export type ArticleLayoutText={kind:'text';role:ArticleTextRole;text:string}
export type ArticleLayoutLink={kind:'link';text:string;url:string}
export type ArticleLayoutImage={kind:'image';src:string;alt:string}
export type ArticleLayoutStack={kind:'stack';children:ArticleLayoutNode[]}
export type ArticleLayoutNode=ArticleLayoutText|ArticleLayoutLink|ArticleLayoutImage|ArticleLayoutStack|{kind:'grid';children:ArticleLayoutStack[]}
export type ArticleLayoutPanel={kind:'panel';tone?:'note'|'insight'|'warning';children:ArticleLayoutNode[]}
export type ArticleLayoutCard={v:3;type:'layout';root:ArticleLayoutPanel}
export type ArticleCard=ArticleCardV1|ArticlePanelCard|ArticleLayoutCard
export const articleCardKinds=[{value:'callout',label:t('提示')},{value:'metrics',label:t('指标')},{value:'summary',label:t('摘要组')},{value:'layout',label:t('组合卡片')},{value:'github',label:t('GitHub 仓库')},{value:'steam',label:t('Steam 游戏')},{value:'apple_music',label:'Apple Music'},{value:'bilibili',label:t('B站视频')},{value:'link',label:t('网页参考')},{value:'moment',label:t('站内动态')},{value:'phrase',label:t('站内片语')}] as const
const labels:Record<ArticleCardType,string>={callout:t('提示'),metrics:t('指标'),summary:t('摘要组'),layout:t('组合卡片'),github:'GitHub',steam:'Steam',apple_music:'Apple Music',link:t('网页参考'),bilibili:t('B站视频'),moment:t('动态'),phrase:t('片语')}
export const articleReferencePrefix='painter:v1:'
export const isInternalArticleCard=(type:string)=>type==='moment'||type==='phrase'
export const isArticlePanel=(type:string):type is ArticlePanelType=>type==='callout'||type==='metrics'||type==='summary'
export const internalArticleCardURL=(type:string,id:string)=>(type==='moment'?'/moments/':'/phrases/')+id
const internalID=(value:unknown):value is string=>typeof value==='string'&&/^[1-9][0-9]{0,18}$/.test(value)&&BigInt(value)<=9223372036854775807n
const mediaURL=/^\/media\/[a-fA-F0-9-]{16,128}$/

function text(value:unknown,max:number,required=false):value is string{return typeof value==='string'&&[...value].length<=max&&(!required||!!value.trim())&&!/[\u0000-\u001f\u007f-\u009f]/.test(value)}
function bodyText(value:unknown,max:number,required=true):value is string{return typeof value==='string'&&[...value].length<=max&&(!required||!!value.trim())&&!/[\u0000-\u0009\u000b\u000c\u000e-\u001f\u007f-\u009f]/.test(value)}
function object(value:unknown):value is Record<string,any>{return !!value&&typeof value==='object'&&!Array.isArray(value)}
function fields(value:Record<string,unknown>,keys:string[]){return Object.keys(value).every(key=>keys.includes(key))}
function nonNullJSON(value:unknown):boolean {if(value===null)return false;if(Array.isArray(value))return value.every(nonNullJSON);return !object(value)||Object.values(value).every(nonNullJSON)}
function publicURL(value:unknown):URL|null {
  if(typeof value!=='string'||!value.startsWith('https://')||new TextEncoder().encode(value).length>2048||/[\s\\\u0000-\u001f\u007f]/.test(value)||/%(?:0[0-9a-f]|1[0-9a-f]|7f)/i.test(value)||/[:%]/.test(value.split('/')[2]||''))return null
  try{const u=new URL(value);if(u.protocol!=='https:'||u.username||u.password||u.port||!/\./.test(u.hostname)||/^(?:localhost|127\.|0\.|10\.|169\.254\.|192\.168\.|172\.(?:1[6-9]|2\d|3[01])\.)/.test(u.hostname)||/\.(?:localhost|local|internal)$/.test(u.hostname)||u.hostname.endsWith('.'))return null;return u}catch{return null}
}

// JSON.parse alone would silently discard duplicate keys, including escaped names.
function uniqueJSONKeys(raw:string){
  const containers:(Set<string>|null)[]=[]
  for(let i=0;i<raw.length;i++){
    const char=raw[i]
    if(char==='{')containers.push(new Set())
    else if(char==='[')containers.push(null)
    else if(char==='}'||char===']')containers.pop()
    else if(char==='"'){
      const start=i++
      for(;i<raw.length;i++){if(raw[i]==='\\')i++;else if(raw[i]==='"')break}
      let next=i+1;while(/\s/.test(raw[next]||'')&&next<raw.length)next++
      if(raw[next]===':'){
        const keys=containers.at(-1),key=JSON.parse(raw.slice(start,i+1)) as string
        if(!keys||keys.has(key))return false
        keys.add(key)
      }
    }
  }
  return true
}

function validLayout(root:unknown){
  let count=0,images=0
  const limits:Record<ArticleTextRole,number>={title:200,label:120,value:100,body:2000,detail:500}
  const visit=(node:unknown,depth:number,isRoot=false):boolean=>{
    if(depth>6||++count>64||!object(node))return false
    if(node.kind==='panel'||node.kind==='stack'||node.kind==='grid'){
      if(!fields(node,node.kind==='panel'?['kind','tone','children']:['kind','children'])||!Array.isArray(node.children)||node.children.length<1||node.children.length>(node.kind==='grid'?6:12))return false
      if(node.kind==='panel'&&(!isRoot||node.tone!==undefined&&!['note','insight','warning'].includes(node.tone)))return false
      if(isRoot&&node.kind!=='panel')return false
      return node.children.every(child=>(node.kind!=='grid'||object(child)&&child.kind==='stack')&&visit(child,depth+1))
    }
    if(isRoot)return false
    if(node.kind==='text')return fields(node,['kind','role','text'])&&typeof node.role==='string'&&Object.hasOwn(limits,node.role)&&(node.role==='body'||node.role==='detail'?bodyText(node.text,limits[node.role],true):text(node.text,limits[node.role as ArticleTextRole],true))
    if(node.kind==='link')return fields(node,['kind','text','url'])&&text(node.text,120,true)&&!!publicURL(node.url)
    if(node.kind==='image')return fields(node,['kind','src','alt'])&&++images<=6&&typeof node.src==='string'&&mediaURL.test(node.src)&&text(node.alt,200)
    return false
  }
  return visit(root,1,true)
}

// This only controls the document representation. The server validates every
// preview/save and remains authoritative; unknown snapshots stay editable code.
export function parseArticleCard(raw:string):ArticleCard|null {
  if(new TextEncoder().encode(raw).length>8192)return null
  try{
    const c=JSON.parse(raw)
    if(!object(c)||!articleCardKinds.some(k=>k.value===c.type))return null
    if(c.v===3)return c.type==='layout'&&fields(c,['v','type','root'])&&uniqueJSONKeys(raw)&&validLayout(c.root)?c as ArticleLayoutCard:null
    if(c.v===2){
      if(c.title!==undefined&&!text(c.title,200))return null
      if(c.type==='callout'){
        if(!fields(c,['v','type','title','tone','body'])||typeof c.tone!=='string'||!['note','insight','warning'].includes(c.tone)||!bodyText(c.body,2000))return null
      }else if(c.type==='metrics'){
        if(!fields(c,['v','type','title','metrics'])||!Array.isArray(c.metrics)||c.metrics.length<1||c.metrics.length>6)return null
        for(const item of c.metrics){
          if(!object(item)||!fields(item,['label','value','detail','source'])||!text(item.label,120,true)||!text(item.value,100,true)||item.detail!==undefined&&!bodyText(item.detail,500,false))return null
          if(item.source!==undefined&&(!object(item.source)||!fields(item.source,['label','url'])||!text(item.source.label,120,true)||!publicURL(item.source.url)))return null
        }
      }else if(c.type==='summary'){
        if(!fields(c,['v','type','title','entries'])||!Array.isArray(c.entries)||c.entries.length<1||c.entries.length>6)return null
        for(const item of c.entries)if(!object(item)||!fields(item,['title','body'])||!text(item.title,200,true)||!bodyText(item.body,2000))return null
      }else return null
      return c as ArticlePanelCard
    }
    if(c.v!==1||isArticlePanel(String(c.type))||c.type==='layout')return null
    if((c.type==='bilibili'||c.type==='link'&&Object.hasOwn(c,'display'))&&(!uniqueJSONKeys(raw)||!nonNullJSON(c)))return null
    if(c.type==='bilibili'||c.type==='link'&&c.display==='social'){
      if(!fields(c,['v','type','url','title','description','subtitle','image_url','metadata_source','resolved_at',...(c.type==='link'?['display']:[])]))return null
      for(const key of ['image_url','metadata_source','resolved_at'])if(Object.hasOwn(c,key)&&typeof c[key]!=='string')return null
    }
    if(isInternalArticleCard(c.type)){
      if(!internalID(c.ref_id)||c.url!==internalArticleCardURL(c.type,c.ref_id)||Object.keys(c).some(k=>!['v','type','url','ref_id'].includes(k)))return null
      return c as ArticleCardV1
    }
    const u=publicURL(c.url)
    if(!u||!text(c.title,200,true)||!(c.type==='link'&&c.display==='social'?bodyText(c.description??'',1000,false):text(c.description??'',1000))||!text(c.subtitle??'',200))return null
    if(Object.keys(c).some(k=>!['v','type','url','title','description','subtitle','image_url','stats','metadata_source','resolved_at','display'].includes(k))||c.display!==undefined&&(c.type!=='link'||!['card','title','social'].includes(c.display)))return null
    if(c.type==='bilibili'&&canonicalBilibiliURL(c.url)!==c.url)return null
    if(c.type==='github'&&(u.host!=='github.com'||u.search||u.hash||!/^\/[A-Za-z0-9][A-Za-z0-9_.-]{0,99}\/[A-Za-z0-9][A-Za-z0-9_.-]{0,99}\/?$/.test(u.pathname)))return null
    if(c.type==='steam'&&(u.host!=='store.steampowered.com'||!/^\/app\/[1-9][0-9]{0,9}(?:\/[^/]*)?\/?$/.test(u.pathname)))return null
    if(c.type==='apple_music'&&(u.host!=='music.apple.com'||!/^\/[a-zA-Z]{2}\/(?:album\/(?:[^/]+\/)?[1-9][0-9]{0,19}|playlist\/(?:[^/]+\/)?pl\.[A-Za-z0-9_.-]+)\/?$/.test(u.pathname)))return null
    if(c.image_url&&!mediaURL.test(c.image_url)){
      const image=publicURL(c.image_url);if(!image||image.hash)return null
      const h=image.hostname
      if(!(c.type==='github'&&h==='avatars.githubusercontent.com'||c.type==='steam'&&/(^|\.)steamstatic\.com$/.test(h)||c.type==='apple_music'&&/(^|\.)mzstatic\.com$/.test(h)))return null
    }
    if(c.stats){if(c.type!=='github'||typeof c.stats!=='object'||Object.keys(c.stats).some(k=>!['stars','forks'].includes(k)))return null;for(const value of Object.values(c.stats))if(!Number.isSafeInteger(value)||Number(value)<0)return null}
    if(c.metadata_source&&!['author','provider'].includes(c.metadata_source)||c.resolved_at&&(typeof c.resolved_at!=='string'||Number.isNaN(Date.parse(c.resolved_at))))return null
    return c as ArticleCardV1
  }catch{return null}
}

export function encodeArticleCard(value:string){const bytes=new TextEncoder().encode(value);let binary='';for(const b of bytes)binary+=String.fromCharCode(b);return btoa(binary).replace(/\+/g,'-').replace(/\//g,'_').replace(/=+$/,'')}
function decodeArticleCard(value:string){if(!/^[A-Za-z0-9_-]+$/.test(value)||value.length>16384)return '';try{const bytes=Uint8Array.from(atob(value.replace(/-/g,'+').replace(/_/g,'/')),c=>c.charCodeAt(0));return new TextDecoder('utf-8',{fatal:true}).decode(bytes)}catch{return ''}}
export function parseArticleReference(title:unknown,href:unknown):ArticleCardV1|null {
  if(typeof title!=='string'||!title.startsWith(articleReferencePrefix))return null
  const card=parseArticleCard(decodeArticleCard(title.slice(articleReferencePrefix.length)))
  return card?.v===1&&card.type==='link'&&(!card.display||card.display==='card')&&card.url===href?card:null
}
export function articleCardMarkdown(card:ArticleCard){return '```painter-card\n'+JSON.stringify(card)+'\n```'}
export function articleReferenceTitle(card:ArticleCard){if(card.v===1&&card.type==='link'&&card.display!=='social'){const {display,...inline}=card;return articleReferencePrefix+encodeArticleCard(JSON.stringify(inline))}return articleReferencePrefix+encodeArticleCard(JSON.stringify(card))}
export function articleLinkSnapshot(href:unknown,label:string,title?:unknown):ArticleCardV1|null {if(typeof title==='string'&&title.startsWith('painter:'))return null;const card=parseArticleCard(JSON.stringify({v:1,type:'link',url:href,title:label.trim()||(typeof href==='string'?publicURL(href)?.hostname:'')||'',metadata_source:'author'}));return card?.v===1&&card.type==='link'?card:null}
export function articleCardTitle(card:ArticleCard):string {
  if(card.v!==3)return card.title||labels[card.type]
  const find=(nodes:ArticleLayoutNode[]):string=>{for(const node of nodes){if(node.kind==='text'&&node.role==='title')return node.text;if('children' in node){const title=find(node.children);if(title)return title}}return ''}
  return find(card.root.children)||labels.layout
}
export function articleReferenceMarkdown(card:ArticleCard,label=articleCardTitle(card)){if(card.v!==1||card.type!=='link'||card.display==='social'||card.display==='title')return articleCardMarkdown(card);return '['+label.replace(/[\\`*_{}\[\]<>()#+.!|&~-]/g,'\\$&')+'](<'+card.url.replace(/</g,'%3C').replace(/>/g,'%3E')+'> "'+articleReferenceTitle(card)+'")'}

const span=(name:string,value:string):DOMOutputSpec=>['span',{class:name},value]
function detailSpec(card:ArticleCardV1):DOMOutputSpec[]{
  const children:DOMOutputSpec[]=[span('content-card__title',card.title||labels[card.type])]
  if(card.subtitle)children.push(span('content-card__subtitle',card.subtitle))
  if(card.description)children.push(span('content-card__description',card.description))
  const stats=[]
  if(card.stats?.stars!==undefined)stats.push('★ '+card.stats.stars)
  if(card.stats?.forks!==undefined)stats.push('Forks '+card.stats.forks)
  if(stats.length)children.push(span('content-card__stats',stats.join(' · ')))
  children.push(span('content-card__url',new URL(card.url).hostname))
  return children
}
export function articleCardSpec(card:ArticleCard):DOMOutputSpec {
  if(card.v!==1)return layoutSpec(card.v===2?compileArticlePanel(card):card.root,card.type)
  if(isInternalArticleCard(card.type))return ['a',{class:'content-card content-card--'+card.type,href:card.url,'data-internal-card-state':'pending'},['span',{class:'content-card__body'},span('content-card__title',t('{kind}引用',{kind:labels[card.type]})),span('content-card__description',t('正在读取公开内容…'))]]
  if(card.type==='bilibili'){
    const stage:DOMOutputSpec[]=[]
    if(card.image_url)stage.push(['img',{class:'content-video__poster',src:card.image_url,alt:'',loading:'lazy'}])
    stage.push(['button',{class:'content-video__load',type:'button','data-video-load':'',hidden:''},t('加载B站播放器')])
    const details:DOMOutputSpec[]=[span('content-video__title',card.title||labels.bilibili)]
    if(card.subtitle)details.push(span('content-video__subtitle',card.subtitle))
    if(card.description)details.push(span('content-video__description',card.description))
    details.push(span('content-video__destination',t('在B站打开')))
    return ['div',{class:'content-video','data-content-video':'bilibili'},['div',{class:'content-video__stage'},...stage],['div',{class:'content-video__details'},['a',{class:'content-video__source',href:card.url,target:'_blank',rel:'noopener noreferrer'},...details]]]
  }
  if(card.type==='link'&&card.display==='social'){
    const meta:DOMOutputSpec[]=[span('content-social__platform',socialPlatform(card.url))]
    if(card.subtitle)meta.push(span('content-social__author',card.subtitle))
    const content:DOMOutputSpec[]=[['span',{class:'content-social__meta'},...meta],span('content-social__title',card.title||labels.link)]
    if(card.description)content.push(span('content-social__text',card.description))
    if(card.image_url)content.push(['img',{class:'content-social__image',src:card.image_url,alt:'',loading:'lazy'}])
    content.push(span('content-social__url',new URL(card.url).hostname))
    return ['a',{class:'content-social',href:card.url,target:'_blank',rel:'noopener noreferrer'},...content]
  }
  if(card.type==='link'&&card.display==='title'){
    const svg=(name:string,path:string):DOMOutputSpec=>['http://www.w3.org/2000/svg svg',{class:name,width:'18',height:'18',viewBox:'0 0 24 24',focusable:'false',fill:'none',stroke:'currentColor','stroke-width':'1.65','stroke-linecap':'round','stroke-linejoin':'round','aria-hidden':'true'},['http://www.w3.org/2000/svg path',{d:path}]]
    return ['a',{class:'content-link-row',href:card.url,target:'_blank',rel:'noopener noreferrer'},svg('content-link-row__icon','M10 13a5 5 0 0 0 7.54.54l3-3a5 5 0 0 0-7.07-7.07l-1.72 1.71M14 11a5 5 0 0 0-7.54-.54l-3 3a5 5 0 0 0 7.07 7.07l1.71-1.71'),span('content-link-row__title',card.title||labels.link),span('content-link-row__host',new URL(card.url).hostname),svg('content-link-row__arrow','M7 17 17 7M7 7h10v10')]
  }
  const children:DOMOutputSpec[]=[]
  if(card.image_url)children.push(['img',{class:'content-card__image',src:card.image_url,alt:'',loading:'lazy',referrerpolicy:'no-referrer'}])
  children.push(['span',{class:'content-card__body'},span('content-card__eyebrow',labels[card.type]),...detailSpec(card)])
  return ['a',{class:'content-card content-card--'+card.type,href:card.url,target:'_blank',rel:'noopener noreferrer'},...children]
}

function panelParagraph(name:string,value:string):DOMOutputSpec {
  const lines=value.replace(/\r\n?/g,'\n').split('\n'),children:(string|DOMOutputSpec)[]=[]
  lines.forEach((line,index)=>{if(index)children.push(['br']);if(line)children.push(line)})
  return ['p',{class:'content-panel__'+name},...children]
}
export function compileArticlePanel(card:ArticlePanelCard):ArticleLayoutPanel {
  const children:ArticleLayoutNode[]=[]
  const paragraph=(role:ArticleTextRole,text:string):ArticleLayoutText=>({kind:'text',role,text})
  if(card.title)children.push(paragraph('title',card.title))
  if(card.type==='callout')children.push(paragraph('body',card.body))
  else{
    const items:ArticleLayoutStack[]=(card.type==='metrics'?card.metrics:card.entries).map(item=>{
      const fields:ArticleLayoutNode[]=[]
      if('value' in item){fields.push(paragraph('label',item.label),paragraph('value',item.value));if(item.detail)fields.push(paragraph('detail',item.detail));if(item.source)fields.push({kind:'link',text:item.source.label,url:item.source.url})}
      else fields.push(paragraph('label',item.title),paragraph('body',item.body))
      return {kind:'stack',children:fields}
    })
    children.push({kind:'grid',children:items})
  }
  return {kind:'panel',...(card.type==='callout'?{tone:card.tone}:{}),children}
}
function layoutSpec(root:ArticleLayoutPanel,preset:ArticlePanelType|'layout'):DOMOutputSpec {
  const nodeSpec=(node:ArticleLayoutNode,gridItem=false):DOMOutputSpec=>{
    if(node.kind==='text')return panelParagraph(node.role,node.text)
    if(node.kind==='link')return ['a',{class:'content-panel__source',href:node.url,target:'_blank',rel:'noopener noreferrer'},node.text]
    if(node.kind==='image')return ['img',{class:'content-panel__image',src:node.src,alt:node.alt,loading:'lazy'}]
    return ['div',{class:node.kind==='grid'?'content-panel__grid':gridItem?'content-panel__item':'content-panel__stack'},...node.children.map(child=>nodeSpec(child,node.kind==='grid'))]
  }
  return ['div',{class:'content-panel content-panel--'+preset+(root.tone?' content-panel--'+root.tone:'')},...root.children.map(node=>nodeSpec(node))]
}

export const ArticleCardNode=TiptapNode.create<{onEdit?:(position:number)=>void}>({
  // Keep paragraph (1000) as ProseMirror's default empty block, while this
  // explicit Markdown handler still runs before the normal code block (100).
  name:'articleCard',group:'block',atom:true,draggable:true,selectable:true,priority:900,
  addOptions(){return {onEdit:undefined}},
  addAttributes(){return {snapshot:{default:'',rendered:false,parseHTML:element=>decodeArticleCard(element.getAttribute('data-painter-card')||'')}}},
  parseHTML(){return [{tag:'div[data-painter-card]',getAttrs:element=>parseArticleCard(decodeArticleCard((element as HTMLElement).getAttribute('data-painter-card')||''))?null:false}]},
  renderHTML({node}){const card=parseArticleCard(node.attrs.snapshot);return card?['div',{'data-painter-card':encodeArticleCard(node.attrs.snapshot),class:'article-card-node'},articleCardSpec(card)]:['pre',{},['code',{},node.attrs.snapshot]]},
  markdownTokenName:'code',
  parseMarkdown(token,helpers){if(token.lang!=='painter-card'||typeof token.text!=='string'||!parseArticleCard(token.text))return [];return helpers.createNode('articleCard',{snapshot:token.text})},
  renderMarkdown(node){return '```painter-card\n'+String(node.attrs?.snapshot||'')+'\n```'},
  addNodeView(){return ({node,editor,getPos})=>{
    const card=parseArticleCard(node.attrs.snapshot),wrapper=document.createElement('div');wrapper.className='article-card-node'+(card&&isInternalArticleCard(card.type)?' article-card-node--internal':'')+(card&&card.v!==1?' article-card-node--panel':'')+(card?.v===1&&card.type==='link'&&card.display==='title'?' article-card-node--link-title':'');wrapper.dataset.painterCard=encodeArticleCard(node.attrs.snapshot);wrapper.contentEditable='false'
    if(card)wrapper.append(DOMSerializer.renderSpec(document,articleCardSpec(card)).dom)
    const edit=document.createElement('button');edit.type='button';edit.className='article-card-node__edit';edit.textContent=t('编辑卡片');edit.setAttribute('aria-label',t('编辑{name}卡片',{name:card?articleCardTitle(card):t('内容')}));edit.addEventListener('click',event=>{event.preventDefault();const position=getPos();if(typeof position==='number')this.options.onEdit?.(position)});wrapper.append(edit)
    wrapper.addEventListener('click',event=>{if(!(event.target instanceof Element)||!event.target.closest('a.content-card,a.content-link-row,a.content-social,.content-video,.content-panel'))return;if(event.ctrlKey||event.metaKey||event.shiftKey||event.altKey)return;event.preventDefault();const position=getPos();if(typeof position==='number')editor.view.dispatch(editor.state.tr.setSelection(NodeSelection.create(editor.state.doc,position)))})
    // DocumentEditor rescans through initArticleCards after initial mount and every document update.
    return {dom:wrapper,stopEvent:event=>event.target===edit,ignoreMutation:()=>true}
  }},
})

export const ArticleReferenceLink=Link.extend({
  addAttributes(){return {...this.parent?.(),title:{default:null,parseHTML:element=>element.getAttribute('data-painter-reference')||element.getAttribute('title')}}},
  renderHTML(props){
    const {HTMLAttributes}=props
    const card=parseArticleReference(HTMLAttributes.title,HTMLAttributes.href)
    if(!card)return this.parent?.(props)||['a',HTMLAttributes,0]
    // Clipboard/export HTML contains only the link. Preview text lives in its
    // mark view so copying a sentence cannot duplicate its reference metadata.
    return ['a',{...HTMLAttributes,title:null,'data-painter-reference':HTMLAttributes.title,class:'content-link__anchor',target:'_blank',rel:'noopener noreferrer'},0]
  },
  addMarkView(){return ({mark,HTMLAttributes})=>{
    const card=parseArticleReference(mark.attrs.title,mark.attrs.href)
    const attributes=card?{...HTMLAttributes,title:null,'data-painter-reference':mark.attrs.title,class:'content-link__anchor',target:'_blank',rel:'noopener noreferrer'}:{...HTMLAttributes,href:isAllowedUri(mark.attrs.href,this.options.protocols)?mark.attrs.href:''}
    const spec:DOMOutputSpec=card?['span',{class:'content-link'},['a',attributes,0],['span',{class:'content-link__preview',contenteditable:'false'},...detailSpec(card)]]:['a',attributes,0]
    const {dom,contentDOM}=DOMSerializer.renderSpec(document,spec)
    return {dom:dom as HTMLElement,contentDOM:contentDOM as HTMLElement,ignoreMutation:mutation=>mutation.type!=='selection'&&(mutation.type==='attributes'||!contentDOM?.contains(mutation.target))}
  }},
  renderMarkdown(node,helpers,context){const href=node.attrs?.href||'',title=node.attrs?.title||'';if(parseArticleReference(title,href))return `[${helpers.renderChildren(node)}](<${href}> "${title}")`;return this.parent?.(node,helpers,context)||`[${helpers.renderChildren(node)}](${href})`},
})
