import { marked } from 'marked'
import {parseArticleCard} from './articleCards'

// Use the Markdown token stream, so a sample fence inside a larger code block
// can never be mistaken for the card the author intends to edit.
export function markdownCardAt(source:string,position:number){
 let offset=0
 for(const token of marked.lexer(source,{gfm:true,breaks:false})){
  const end=offset+token.raw.length
  if(position>=offset&&position<=end&&token.type==='code'&&token.lang==='painter-card'){
   const card=parseArticleCard(token.text)
   if(card)return {from:offset,to:end,card}
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
  if (Object.keys(tokens.links).length) issue='引用式链接定义'
  marked.walkTokens(tokens,token=>{
    if(token.type==='html')issue='HTML 或注释'
    if(token.type==='code'&&'lang' in token&&typeof token.lang==='string'&&/\s/.test(token.lang.trim()))issue='带附加属性的代码围栏'
    if(!['code','codespan'].includes(token.type)&&'text' in token&&typeof token.text==='string'&&/(?:\[\^[^\]]+\]|^\s*:::[\w-]|\$\$)/m.test(token.text))issue='脚注、指令或数学扩展'
    if((token.type==='image'||token.type==='link')&&'href' in token&&typeof token.href==='string'&&!safeMediaURL(token.href))issue='不支持的链接地址'
  })
  if(issue)return issue
  try {
    if(canonicalHTML(marked.parse(source,{gfm:true,breaks:false,async:false}))!==canonicalHTML(marked.parse(serialized,{gfm:true,breaks:false,async:false})))return '暂不支持完整还原的 Markdown 结构'
  }catch{return '暂不支持完整还原的 Markdown 结构'}
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
