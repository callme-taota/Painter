import {t} from './i18n'
import {marked,type Token,type Tokens} from 'marked'
import {parseArticleCard,parseArticleReference,articleLinkSnapshot,type ArticleCard} from './articleCards'

// Advance through every sibling's raw text, including code samples. Only edit
// a child stream that maps to one exact contiguous span of its parent's source.
type InlineLabelToken={type:string;text?:string;tokens?:InlineLabelToken[]}
type SourceToken=Token|Tokens.ListItem
type MarkdownCardSelection={from:number;to:number;card:ArticleCard;inline:boolean;label:string}
function linkLabel(tokens:InlineLabelToken[]):string{return tokens.map(token=>token.tokens?linkLabel(token.tokens):token.type==='br'?' ':token.text||'').join('')}
function inlineCardAt(token:SourceToken,offset:number,position:number):MarkdownCardSelection|null {
 if(['code','codespan','html','image'].includes(token.type))return null
 if(token.type==='link'&&'href'in token&&typeof token.href==='string'&&'text'in token&&typeof token.text==='string'){
  const title='title'in token&&typeof token.title==='string'?token.title:undefined,label='tokens'in token&&Array.isArray(token.tokens)?linkLabel(token.tokens):token.text
  const card=parseArticleReference(title,token.href)||articleLinkSnapshot(token.href,label,title)
  return card?{from:offset,to:offset+token.raw.length,card,inline:true,label}:null
 }
 const children='tokens'in token&&Array.isArray(token.tokens)?token.tokens:'items'in token&&Array.isArray(token.items)?token.items:undefined
 if(!children?.length||children.some(child=>typeof child.raw!=='string'))return null
 const raw=children.map(child=>child.raw).join(''),start=token.raw.indexOf(raw)
 if(!raw||start<0||token.raw.indexOf(raw,start+1)>=0)return null
 let cursor=offset+start
 for(const child of children){
  const end=cursor+child.raw.length
  if(position>=cursor&&position<=end){const found=inlineCardAt(child,cursor,position);if(found)return found}
  cursor=end
 }
 return null
}
export function markdownCardAt(source:string,position:number):MarkdownCardSelection|null {
 let offset=0
 for(const token of marked.lexer(source,{gfm:true,breaks:false})){
  const end=offset+token.raw.length
  if(position>=offset&&position<=end){
   if(token.type==='code'&&token.lang==='painter-card'){const card=parseArticleCard(token.text);if(card)return {from:offset,to:end,card,inline:false,label:''}}
   const found=inlineCardAt(token,offset,position);if(found)return found
  }
  offset=end
 }
 return null
}

// This check never replaces the saved string. It only decides whether the rich
// editor can safely edit a document. Unsupported documents remain editable in CM.
export function markdownCompatibility(source:string, serialized:string):string {
  const tokens=marked.lexer(source,{gfm:true,breaks:false})
  let issue=''
  if (/^\s*---\s*\r?\n[\s\S]*?\r?\n---(?:\s|$)/.test(source)) issue='Front matter'
  if (Object.keys(tokens.links).length) issue=t('引用式链接定义')
  marked.walkTokens(tokens,token=>{
    if(token.type==='html')issue=t('HTML 或注释')
    if(token.type==='code'&&'lang' in token&&typeof token.lang==='string'&&/\s/.test(token.lang.trim()))issue=t('带附加属性的代码围栏')
    if(!['code','codespan'].includes(token.type)&&'text' in token&&typeof token.text==='string'&&/(?:\[\^[^\]]+\]|^\s*:::[\w-]|\$\$)/m.test(token.text))issue=t('脚注、指令或数学扩展')
    if((token.type==='image'||token.type==='link')&&'href' in token&&typeof token.href==='string'&&!safeMediaURL(token.href))issue=t('不支持的链接地址')
  })
  if(issue)return issue
  try {
    if(canonicalHTML(marked.parse(source,{gfm:true,breaks:false,async:false}))!==canonicalHTML(marked.parse(serialized,{gfm:true,breaks:false,async:false})))return t('暂不支持完整还原的 Markdown 结构')
  }catch{return t('暂不支持完整还原的 Markdown 结构')}
  return ''
}

export function safeMediaURL(value:string){return /^(?:https?:|mailto:|\/[^/]|\/\/?$|#|\.{1,2}\/)/i.test(value)||!/^\s*(?:[\w+.-]+:|\/\/)/.test(value)}

function canonicalHTML(html:string){
  const root=new DOMParser().parseFromString(html,'text/html').body
  const walk=(node:Node,pre=false):unknown=>{
    if(node.nodeType===Node.TEXT_NODE){const text=node.textContent||'';if(pre)return text;if(!text.trim()&&/^(?:BODY|UL|OL|LI|BLOCKQUOTE|TABLE|THEAD|TBODY|TR)$/.test(node.parentElement?.tagName||''))return '';return text.replace(/\s+/g,' ')}
    if(!(node instanceof Element))return ''
    const name=node.tagName.toLowerCase(),literal=pre||name==='pre'||name==='code'
    const attrs=Array.from(node.attributes).map(a=>[a.name,a.value]).sort(([a],[b])=>a.localeCompare(b))
    const children=Array.from(node.childNodes).map(n=>walk(n,literal)).filter(x=>x!=='')
    return [name,attrs,children]
  }
  return JSON.stringify(Array.from(root.childNodes).map(n=>walk(n)).filter(x=>x!==''))
}
