import {Node as TiptapNode} from '@tiptap/core'
import {DOMSerializer,type DOMOutputSpec} from '@tiptap/pm/model'
import Link,{isAllowedUri} from '@tiptap/extension-link'
import {NodeSelection} from '@tiptap/pm/state'
import './articleCards.css'

export type ArticleReferenceType='github'|'steam'|'apple_music'|'link'|'moment'|'phrase'
export type ArticlePanelType='callout'|'metrics'|'summary'
export type ArticleCardType=ArticleReferenceType|ArticlePanelType
export type ArticleCardV1={v:1;type:ArticleReferenceType;url:string;title?:string;ref_id?:string;description?:string;subtitle?:string;image_url?:string;stats?:{stars?:number;forks?:number};metadata_source?:'provider'|'author';resolved_at?:string}
export type ArticleCardSource={label:string;url:string}
export type ArticleCardMetric={label:string;value:string;detail?:string;source?:ArticleCardSource}
export type ArticleCardEntry={title:string;body:string}
export type ArticlePanelCard={v:2;title?:string}&({type:'callout';tone:'note'|'insight'|'warning';body:string}|{type:'metrics';metrics:ArticleCardMetric[]}|{type:'summary';entries:ArticleCardEntry[]})
export type ArticleCard=ArticleCardV1|ArticlePanelCard
export const articleCardKinds=[{value:'callout',label:'提示'},{value:'metrics',label:'指标'},{value:'summary',label:'摘要组'},{value:'github',label:'GitHub 仓库'},{value:'steam',label:'Steam 游戏'},{value:'apple_music',label:'Apple Music'},{value:'link',label:'网页参考'},{value:'moment',label:'站内动态'},{value:'phrase',label:'站内片语'}] as const
const labels:Record<ArticleCardType,string>={callout:'提示',metrics:'指标',summary:'摘要组',github:'GitHub',steam:'Steam',apple_music:'Apple Music',link:'网页参考',moment:'动态',phrase:'片语'}
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
function publicURL(value:unknown):URL|null {
  if(typeof value!=='string'||!value.startsWith('https://')||new TextEncoder().encode(value).length>2048||/[\s\\\u0000-\u001f\u007f]/.test(value)||/%(?:0[0-9a-f]|1[0-9a-f]|7f)/i.test(value)||/[:%]/.test(value.split('/')[2]||''))return null
  try{const u=new URL(value);if(u.protocol!=='https:'||u.username||u.password||u.port||!/\./.test(u.hostname)||/^(?:localhost|127\.|0\.|10\.|169\.254\.|192\.168\.|172\.(?:1[6-9]|2\d|3[01])\.)/.test(u.hostname)||/\.(?:localhost|local|internal)$/.test(u.hostname)||u.hostname.endsWith('.'))return null;return u}catch{return null}
}

// This only controls the document representation. The server validates every
// preview/save and remains authoritative; unknown snapshots stay editable code.
export function parseArticleCard(raw:string):ArticleCard|null {
  if(new TextEncoder().encode(raw).length>8192)return null
  try{
    const c=JSON.parse(raw)
    if(!object(c)||!articleCardKinds.some(k=>k.value===c.type))return null
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
    if(c.v!==1||isArticlePanel(String(c.type)))return null
    if(isInternalArticleCard(c.type)){
      if(!internalID(c.ref_id)||c.url!==internalArticleCardURL(c.type,c.ref_id)||Object.keys(c).some(k=>!['v','type','url','ref_id'].includes(k)))return null
      return c as ArticleCardV1
    }
    const u=publicURL(c.url)
    if(!u||!text(c.title,200,true)||!text(c.description??'',1000)||!text(c.subtitle??'',200))return null
    if(Object.keys(c).some(k=>!['v','type','url','title','description','subtitle','image_url','stats','metadata_source','resolved_at'].includes(k)))return null
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
  return card?.v===1&&card.type==='link'&&card.url===href?card:null
}
export function articleCardMarkdown(card:ArticleCard){return '```painter-card\n'+JSON.stringify(card)+'\n```'}
export function articleReferenceTitle(card:ArticleCard){return articleReferencePrefix+encodeArticleCard(JSON.stringify(card))}
export function articleReferenceMarkdown(card:ArticleCard,label=card.title||labels[card.type]){if(card.v!==1||card.type!=='link')return articleCardMarkdown(card);return '['+label.replace(/[\\`*_{}\[\]<>()#+.!|&~-]/g,'\\$&')+'](<'+card.url.replace(/</g,'%3C').replace(/>/g,'%3E')+'> "'+articleReferenceTitle(card)+'")'}

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
  if(card.v===2)return panelSpec(card)
  if(isInternalArticleCard(card.type))return ['a',{class:'content-card content-card--'+card.type,href:card.url,'data-internal-card-state':'pending'},['span',{class:'content-card__body'},span('content-card__title',labels[card.type]+'引用'),span('content-card__description','正在读取公开内容…')]]
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
function panelSpec(card:ArticlePanelCard):DOMOutputSpec {
  const children:DOMOutputSpec[]=[]
  if(card.title)children.push(panelParagraph('title',card.title))
  if(card.type==='callout')children.push(panelParagraph('body',card.body))
  else{
    const items:DOMOutputSpec[]=(card.type==='metrics'?card.metrics:card.entries).map(item=>{
      const fields:DOMOutputSpec[]=[]
      if('value' in item){fields.push(panelParagraph('label',item.label),panelParagraph('value',item.value));if(item.detail)fields.push(panelParagraph('detail',item.detail));if(item.source)fields.push(['a',{class:'content-panel__source',href:item.source.url,target:'_blank',rel:'noopener noreferrer'},item.source.label])}
      else fields.push(panelParagraph('label',item.title),panelParagraph('body',item.body))
      return ['div',{class:'content-panel__item'},...fields]
    })
    children.push(['div',{class:'content-panel__grid'},...items])
  }
  return ['div',{class:'content-panel content-panel--'+card.type+(card.type==='callout'?' content-panel--'+card.tone:'')},...children]
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
    const card=parseArticleCard(node.attrs.snapshot),wrapper=document.createElement('div');wrapper.className='article-card-node'+(card&&isInternalArticleCard(card.type)?' article-card-node--internal':'')+(card?.v===2?' article-card-node--panel':'');wrapper.dataset.painterCard=encodeArticleCard(node.attrs.snapshot);wrapper.contentEditable='false'
    if(card)wrapper.append(DOMSerializer.renderSpec(document,articleCardSpec(card)).dom)
    const edit=document.createElement('button');edit.type='button';edit.className='article-card-node__edit';edit.textContent='编辑卡片';edit.setAttribute('aria-label','编辑'+(card?.title||'内容')+'卡片');edit.addEventListener('click',event=>{event.preventDefault();const position=getPos();if(typeof position==='number')this.options.onEdit?.(position)});wrapper.append(edit)
    wrapper.addEventListener('click',event=>{if(!(event.target instanceof Element)||!event.target.closest('a.content-card,.content-panel'))return;if(event.ctrlKey||event.metaKey||event.shiftKey||event.altKey)return;event.preventDefault();const position=getPos();if(typeof position==='number')editor.view.dispatch(editor.state.tr.setSelection(NodeSelection.create(editor.state.doc,position)))})
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
