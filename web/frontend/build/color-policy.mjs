import fs from 'node:fs'
import path from 'node:path'
import {fileURLToPath} from 'node:url'
import {execFileSync} from 'node:child_process'
import postcss from 'postcss'
import ts from 'typescript'
import {parse as parseVue} from '@vue/compiler-sfc'
import {parse as parseTemplate} from '@vue/compiler-dom'

const generatedCSS = 'web/frontend/src/color-tokens.css'
const generatedTS = 'web/frontend/src/color-tokens.ts'
const sampledFile = 'web/frontend/src/momentCover.ts'
const sampledCSS = 'web/frontend/src/momentsPublic.css'
const sampledKeys = new Set(['--moment-color-left', '--moment-color-right'])
// These values combine approved colors with geometry. They cannot contain new
// colors, and their names do not grant permission to redefine canonical keys.
const derivedKeys = new Map([
  ['--shadow-xs', ['web/frontend/src/shadcn-theme.css']],
  ['--shadow', ['web/frontend/src/shadcn-theme.css']],
  ['--surface-shadow', ['web/frontend/src/shadcn-theme.css']],
  ['--glass-shadow', ['web/frontend/src/shadcn-theme.css']],
  ['--raycast-shadow', ['web/frontend/src/raycast-search.css', 'web/frontend/src/searchGlass.css']],
  ['--liquid-glass-tint', ['web/frontend/src/liquidGlass.css', 'web/frontend/src/searchGlass.css', 'web/frontend/src/mobileWorkbench.css', 'web/frontend/src/mobileChrome.css']],
  ['--search-glass-fill', ['web/frontend/src/searchGlass.css']],
  ['--search-glass-tint', ['web/frontend/src/searchGlass.css']],
  ['--search-glass-line', ['web/frontend/src/searchGlass.css']],
  ['--raycast-line', ['web/frontend/src/raycast-search.css', 'web/frontend/src/searchGlass.css']],
  ['--raycast-backdrop', ['web/frontend/src/raycast-search.css', 'web/frontend/src/searchGlass.css']],
  ['--raycast-surface', ['web/frontend/src/raycast-search.css']],
  ['--home-surface', ['web/frontend/src/homeStory.css']],
  ['--work-day-color', ['web/frontend/src/homeWorkJourney.ts']],
])
const urlSlots = new Set(['--liquid-glass-filter'])
const namedColors = new Set(('aliceblue antiquewhite aqua aquamarine azure beige bisque black blanchedalmond blue blueviolet brown burlywood cadetblue chartreuse chocolate coral cornflowerblue cornsilk crimson cyan darkblue darkcyan darkgoldenrod darkgray darkgreen darkgrey darkkhaki darkmagenta darkolivegreen darkorange darkorchid darkred darksalmon darkseagreen darkslateblue darkslategray darkslategrey darkturquoise darkviolet deeppink deepskyblue dimgray dimgrey dodgerblue firebrick floralwhite forestgreen fuchsia gainsboro ghostwhite gold goldenrod gray green greenyellow grey honeydew hotpink indianred indigo ivory khaki lavender lavenderblush lawngreen lemonchiffon lightblue lightcoral lightcyan lightgoldenrodyellow lightgray lightgreen lightgrey lightpink lightsalmon lightseagreen lightskyblue lightslategray lightslategrey lightsteelblue lightyellow lime limegreen linen magenta maroon mediumaquamarine mediumblue mediumorchid mediumpurple mediumseagreen mediumslateblue mediumspringgreen mediumturquoise mediumvioletred midnightblue mintcream mistyrose moccasin navajowhite navy oldlace olive olivedrab orange orangered orchid palegoldenrod palegreen paleturquoise palevioletred papayawhip peachpuff peru pink plum powderblue purple rebeccapurple red rosybrown royalblue saddlebrown salmon sandybrown seagreen seashell sienna silver skyblue slateblue slategray slategrey snow springgreen steelblue tan teal thistle tomato turquoise violet wheat white whitesmoke yellow yellowgreen').split(' '))
const colorFunctions = new Set(['rgb', 'rgba', 'hsl', 'hsla', 'hwb', 'lab', 'lch', 'oklab', 'oklch', 'color', 'device-cmyk'])
const systemColors = new Set('canvas canvastext linktext visitedtext activetext buttonface buttontext buttonborder field fieldtext highlight highlighttext selecteditem selecteditemtext mark marktext graytext accentcolor accentcolortext'.split(' '))
const colorProperty = property => /^(?:color|background|background-image|border(?:-(?:top|right|bottom|left|block|inline)(?:-(?:start|end))?)?|outline|box-shadow|text-shadow|fill|stroke|filter|backdrop-filter|mask(?:-image)?|-webkit-mask(?:-image)?|text-decoration|text-emphasis|column-rule|scrollbar-color)$/.test(property) || property.endsWith('-color')
const kebab = property => property.startsWith('--') ? property : property.replace(/[A-Z]/g, letter => '-'+letter.toLowerCase())
const unescapeCSS = value => value.replace(/\\([\da-f]{1,6})\s?|\\([^\r\n])/gi, (_, hex, char) => hex ? String.fromCodePoint(parseInt(hex, 16) || 0xfffd) : char)

export function createContract(registry) {
  if (!registry || registry.version !== 1 || !registry.colors || typeof registry.colors !== 'object' || Array.isArray(registry.colors)) throw new Error('Invalid color registry: expected version 1 and colors object')
  const keys = new Set(), aliases = registry.aliases || {}
  for (const [key, entry] of Object.entries(registry.colors)) {
    if (!/^[a-z][a-z0-9-]*$/.test(key) || !entry || typeof entry !== 'object' || !['light', 'dark', 'description', 'category'].every(field => typeof entry[field] === 'string' && entry[field].trim())) throw new Error(`Invalid color registry entry: ${key}`)
    keys.add('--'+key)
  }
  if (!keys.size || typeof aliases !== 'object' || Array.isArray(aliases)) throw new Error('Invalid color registry aliases')
  const resolve = (key, trail = new Set()) => {
    if (Object.hasOwn(registry.colors,key)) return
    if (trail.has(key) || !/^[a-z][a-z0-9-]*$/.test(key) || typeof aliases[key] !== 'string') throw new Error(`Invalid color alias: ${key}`)
    trail.add(key); resolve(aliases[key], trail)
  }
  for (const key of Object.keys(aliases)) {
    if (Object.hasOwn(registry.colors,key)) throw new Error(`Duplicate color key: ${key}`)
    resolve(key); keys.add('--'+key)
  }
  for (const [mode, values] of Object.entries(registry.modes || {})) {
    if (!['print', 'contrast'].includes(mode) || !values || typeof values !== 'object' || Array.isArray(values)) throw new Error(`Invalid color mode: ${mode}`)
    for (const [key, value] of Object.entries(values)) if (!keys.has('--'+key) || typeof value !== 'string' || !value.trim()) throw new Error(`Invalid ${mode} color: ${key}`)
  }
  for(const [key,entry] of Object.entries(registry.colors))for(const theme of ['light','dark'])validateRegistryColor(entry[theme],keys,`${key}.${theme}`)
  for(const [mode,values] of Object.entries(registry.modes||{}))for(const [key,value] of Object.entries(values))validateRegistryColor(value,keys,`${mode}.${key}`)
  for(const theme of ['light','dark'])for(const overrides of [{},...Object.values(registry.modes||{})]){
    const done=new Set(),active=new Set()
    const visit=key=>{
      if(active.has(key))throw new Error(`Cyclic color reference: ${key}`)
      if(done.has(key))return
      active.add(key)
      const value=overrides[key]??(Object.hasOwn(aliases,key)?`var(--${aliases[key]})`:registry.colors[key][theme])
      const references=items=>items.flatMap(item=>item.type==='function'?(item.value.toLowerCase()==='var'?[item.children[0].value.slice(2),...references(item.children.slice(1))]:references(item.children)):[])
      for(const reference of references(tokens(value)))visit(reference)
      active.delete(key);done.add(key)
    }
    for(const key of keys)visit(key.slice(2))
  }
  return {keys}
}

function validateRegistryColor(value,keys,label) {
  const invalid=()=>{throw new Error(`Invalid registry color ${label}: ${value}`)}
  if(/[;{}<>\\]|\/\*|["']/.test(value))invalid()
  let parsed
  try{parsed=tokens(value)}catch{invalid()}
  const split=items=>{const parts=[[]];for(const item of items){if(item.type==='symbol'&&item.value===',')parts.push([]);else parts.at(-1).push(item)}return parts}
  const valid=items=>{
    if(items.length!==1)return false
    const item=items[0],name=item.value.toLowerCase()
    if(item.type==='word')return /^#(?:[\da-f]{3}|[\da-f]{4}|[\da-f]{6}|[\da-f]{8})$/i.test(name)||namedColors.has(name)||systemColors.has(name)||['transparent','currentcolor'].includes(name)
    if(item.type!=='function')return false
    if(name==='var'){
      const [key,...fallback]=split(item.children)
      return key.length===1&&key[0].type==='word'&&keys.has(key[0].value)&&fallback.length<=1&&(!fallback.length||valid(fallback[0]))
    }
    if(name==='color-mix'){
      const [space,first,second,...extra]=split(item.children)
      if(!space||!first||!second||extra.length)return false
      const header=space.map(part=>part.value).join(' ')
      if(space.some(part=>part.type!=='word')||!/^in (?:srgb|srgb-linear|display-p3|a98-rgb|prophoto-rgb|rec2020|lab|oklab|lch|oklch|hsl|hwb|xyz|xyz-d50|xyz-d65)(?: (?:shorter|longer|increasing|decreasing) hue)?$/.test(header))return false
      const stop=part=>{const percentage=part.at(-1);if(percentage?.type==='word'&&/^\d*\.?\d+%$/.test(percentage.value)){if(Number.parseFloat(percentage.value)>100)return false;part=part.slice(0,-1)}return valid(part)}
      return stop(first)&&stop(second)
    }
    if(!['rgb','rgba','hsl','hsla','hwb','lab','lch','oklab','oklch'].includes(name)||item.children.some(child=>child.type==='function'||child.type==='url'))return false
    const serialized=item.children.map(part=>part.value).join(' ')
    const number='[+-]?(?:\\d*\\.\\d+|\\d+\\.?\\d*)(?:e[+-]?\\d+)?'
    const channel=`${number}%?`,angle=`${number}(?:deg|grad|rad|turn)?`
    const channels=['hsl','hsla','hwb'].includes(name)?[angle,`${number}%`,`${number}%`]:['lch','oklch'].includes(name)?[channel,channel,angle]:[channel,channel,channel]
    const alpha=`${number}%?`
    const modern=new RegExp(`^${channels.join(' ')}(?: / ${alpha})?$`,'i')
    const legacy=['rgb','rgba','hsl','hsla'].includes(name)&&new RegExp(`^${channels.join(' , ')}(?: , ${alpha})?$`,'i')
    return modern.test(serialized)||Boolean(legacy&&legacy.test(serialized))
  }
  if(!valid(parsed))invalid()
}

function tokens(value) {
  const result = []
  let index = 0
  const read = end => {
    const items = []
    while (index < value.length) {
      const char = value[index]
      if (/\s/.test(char)) {index++; continue}
      if (value.startsWith('/*', index)) {
        const close = value.indexOf('*/', index+2)
        if (close < 0) throw new Error('Unclosed CSS comment')
        index = close+2; continue
      }
      if (char === ')' && end) {index++; return items}
      if (char === '(') {index++;items.push({type:'function',value:'',children:read(true)});continue}
      if (char === ')') throw new Error('Unexpected CSS parenthesis')
      if (char === '"' || char === "'") {
        const quote = char; index++
        let closed = false
        while (index < value.length) {if (value[index] === '\\') {index += 2; continue} if (value[index++] === quote) {closed=true; break}}
        if (!closed) throw new Error('Unclosed CSS string')
        continue
      }
      const match = value.slice(index).match(/^(?:\\[\da-f]{1,6}\s?|\\[^\r\n]|[\w#.%+-])+/i)
      if (!match) {items.push({type:'symbol', value:char}); index++; continue}
      index += match[0].length
      const word = unescapeCSS(match[0])
      if (value[index] === '(') {
        index++
        if (word.toLowerCase() === 'url') {
          const start=index
          let quoted = '', closed = false
          while (index < value.length) {const at=value[index++]; if(at==='\\'){index++;continue} if(quoted){if(at===quoted)quoted='';continue} if(at==='"'||at==="'"){quoted=at;continue} if(at===')'){closed=true;break}}
          if (!closed) throw new Error('Unclosed CSS URL')
          items.push({type:'url', value:value.slice(start,index-1).trim().replace(/^(["'])([\s\S]*)\1$/,'$2')})
        } else items.push({type:'function', value:word, children:read(true)})
      } else items.push({type:'word', value:word})
    }
    if (end) throw new Error('Unclosed CSS function')
    return items
  }
  result.push(...read(false))
  return result
}

function hasColor(items, keys) {
  return items.some(item => item.type === 'word' && (item.value.startsWith('#') || namedColors.has(item.value.toLowerCase()) || keys.has(item.value)) || item.type === 'function' && (colorFunctions.has(item.value.toLowerCase()) || hasColor(item.children, keys)))
}

function inspectValue(value, {file, contract, structural = new Set(), issue, dynamic = false, ignoreUnknown = false}) {
  let parsed
  try {parsed=tokens(value)} catch (error) {issue(`Malformed color value: ${error.message}`); return}
  function inspect(items, numeric = false) {
    for (const item of items) {
      if(item.type==='url'&&/^data:image\/svg\+xml[;,]/i.test(item.value)){
        try{
          const comma=item.value.indexOf(','),metadata=item.value.slice(0,comma),body=item.value.slice(comma+1)
          const svg=/;base64$/i.test(metadata)?Buffer.from(body,'base64').toString('utf8'):decodeURIComponent(body)
          for(const problem of auditHTML(svg,file,contract))issue(`Inline SVG: ${problem.message}`)
        }catch{issue('Malformed inline SVG color source')}
      }
      if (item.type === 'word' && (item.value.startsWith('#') || namedColors.has(item.value.toLowerCase()))) issue(`Raw color ${item.value}; use a registered color key`)
      if (item.type !== 'function') continue
      const name = item.value.toLowerCase()
      if (colorFunctions.has(name)) {
        const key = item.children[0]?.children?.[0]?.value
        const serialized = item.children.map(child => child.type === 'function' ? child.value+'('+child.children.map(part=>part.value).join(' ')+')' : child.value).join(' ')
        const match = serialized.match(/^var\(--moment-color-(?:left|right)(?: , (\d+) (\d+) (\d+))?\) \/ (\d*\.?\d+)(%)?$/)
        const sample = name === 'rgb' && file === sampledCSS && sampledKeys.has(key) && match && (!match[1] || match.slice(1,4).every(channel=>Number(channel)<=255)) && Number(match[4]) <= (match[5] ? 100 : 1)
        if (!sample) issue(`Raw color function ${name}(); use registered keys or color-mix()`)
        continue
      }
      if (name === 'var') {
        const key = item.children[0]?.value
        if (!key || !/^--[a-z][a-z0-9-]*$/.test(key)) {issue('Color var() must name one literal registered key'); continue}
        if (!contract.keys.has(key) && !derivedKeys.has(key) && !urlSlots.has(key) && !numeric && !structural.has(key) && !ignoreUnknown) issue(`Undefined color key ${key}`)
        if (item.children.length > 1) inspect(item.children.slice(1), numeric)
        continue
      }
      if(name==='attr')issue('Color values cannot be supplied through attr(); use a registered key')
      inspect(item.children, numeric || ['calc','min','max','clamp'].includes(name))
    }
  }
  inspect(parsed)
  if (dynamic && /__PAINTER_DYNAMIC__(?![\w-])/.test(value)) {
    // Interpolation may supply a numeric shadow length or mix percentage. It
    // cannot construct a property name, a var key, or an arbitrary color value.
    const remainder = value.replace(/__PAINTER_DYNAMIC__(?:%|px|rem|em|vh|vw|svh|dvh|deg)/g, '0').replace(/(?:calc|min|max|clamp)\([^)]*__PAINTER_DYNAMIC__[^)]*\)/g, '0')
    if (remainder.includes('__PAINTER_DYNAMIC__')) issue('Dynamic color interpolation must use a literal registered key; interpolate only numeric parameters')
  }
}

export function auditStylesheet(source, file, contract, options = {}) {
  const issues = []
  const issue = (message, line = 1) => issues.push({file, line, message})
  let sheet
  try {sheet=postcss.parse(options.inline ? `a{${source}}` : source, {from:file})} catch (error) {issue(`Malformed stylesheet: ${error.reason || error.message}`, error.line); return issues}
  if (file === generatedCSS) return issues
  const custom = []
  sheet.walkDecls(declaration => {if(declaration.prop.startsWith('--'))custom.push(declaration)})
  const colored = new Set(contract.keys)
  let changed = true
  while (changed) {
    changed=false
    for (const declaration of custom) {
      try {if(!colored.has(declaration.prop) && hasColor(tokens(declaration.value), colored)){colored.add(declaration.prop);changed=true}} catch {}
    }
  }
  const structural = new Set()
  const references=items=>items.flatMap(item=>item.type==='function'?(item.value.toLowerCase()==='var'?[item.children[0]?.value]:references(item.children)):[])
  changed=true
  while(changed){
    changed=false
    for(const declaration of custom)if(!colored.has(declaration.prop)&&!structural.has(declaration.prop)){
      try{if(references(tokens(declaration.value)).every(key=>structural.has(key))){structural.add(declaration.prop);changed=true}}catch{}
    }
  }
  for (const key of options.structural || []) structural.add(key)
  sheet.walkDecls(declaration => {
    const property = unescapeCSS(declaration.prop), line = declaration.source?.start?.line || 1
    const report = message => issue(message, line)
    if (contract.keys.has(property)) report(`Canonical color key ${property} cannot be redefined outside ${generatedCSS}`)
    if (sampledKeys.has(property)) report(`Sampled image color ${property} may only be written by ${sampledFile}`)
    if (property.startsWith('--') && colored.has(property) && !contract.keys.has(property) && !derivedKeys.get(property)?.includes(file)) report(`Unregistered component color key ${property}; add it to the shared registry`)
    if (colorProperty(property) || property.startsWith('--')) inspectValue(declaration.value, {file, contract, structural, issue:report,ignoreUnknown:property.startsWith('--')&&!colored.has(property)})
  })
  sheet.walkAtRules('property',rule=>{
    if(contract.keys.has(unescapeCSS(rule.params.trim())))issue(`Canonical color key ${rule.params} cannot be registered outside ${generatedCSS}`,rule.source?.start?.line)
    if(rule.nodes?.some(node=>node.prop==='syntax'&&node.value.includes('<color>'))){
      issue(`Component @property color ${rule.params} must use the shared registry`,rule.source?.start?.line)
      for(const declaration of rule.nodes)if(declaration.prop==='initial-value')inspectValue(declaration.value,{file,contract,structural,issue:message=>issue(message,declaration.source?.start?.line)})
    }
  })
  return issues
}

function literalValue(node) {
  if(!node)return null
  if(ts.isParenthesizedExpression(node))return literalValue(node.expression)
  if (ts.isStringLiteralLike(node)) return {value:node.text, dynamic:false}
  if (ts.isTemplateExpression(node)) return {value:node.head.text+node.templateSpans.map(span=>'__PAINTER_DYNAMIC__'+span.literal.text).join(''), dynamic:true}
  if(ts.isBinaryExpression(node)&&node.operatorToken.kind===ts.SyntaxKind.PlusToken){const left=literalValue(node.left),right=literalValue(node.right);if(left&&right)return {value:left.value+right.value,dynamic:left.dynamic||right.dynamic}}
  return null
}

function numericStyleExpression(node) {
  const value=literalValue(node)
  if(value)return /^[\d\s.+%*/()_-]*(?:(?:px|rem|em|vh|vw|svh|dvh|deg)[\d\s.+%*/()_-]*)?$/.test(value.value.replaceAll('__PAINTER_DYNAMIC__','0')) || ['auto','none',''].includes(value.value)
  if(ts.isNumericLiteral(node)||ts.isPrefixUnaryExpression(node))return true
  if(ts.isParenthesizedExpression(node))return numericStyleExpression(node.expression)
  if(ts.isConditionalExpression(node))return numericStyleExpression(node.whenTrue)&&numericStyleExpression(node.whenFalse)
  if(ts.isBinaryExpression(node)&&node.operatorToken.kind===ts.SyntaxKind.PlusToken)return /^(?:px|rem|em|vh|vw|svh|dvh|deg|%)$/.test(literalValue(node.right)?.value||'') || numericStyleExpression(node.left)&&numericStyleExpression(node.right)
  return ts.isCallExpression(node)&&(ts.isIdentifier(node.expression)&&node.expression.text==='String'||ts.isPropertyAccessExpression(node.expression)&&node.expression.name.text==='toFixed')
}

export function auditScript(source, file, contract) {
  const issues=[]
  if(file===generatedTS)return issues
  const program=ts.createSourceFile(file, source, ts.ScriptTarget.Latest, true, ts.ScriptKind.TSX)
  const report=(node,message)=>issues.push({file,line:program.getLineAndCharacterOfPosition(node.getStart()).line+1,message})
  for(const diagnostic of program.parseDiagnostics)report(program, `Malformed script: ${ts.flattenDiagnosticMessageText(diagnostic.messageText,' ')}`)
  const literalArrays=new Map()
  const literalConstants=new Map(),styleAliases=new Set()
  const unwrap=node=>node&&(ts.isAsExpression(node)||ts.isParenthesizedExpression(node)||ts.isSatisfiesExpression(node)||ts.isNonNullExpression(node))?unwrap(node.expression):node
  const gather=node=>{
    if(ts.isVariableDeclaration(node)&&ts.isIdentifier(node.name)){
      const value=unwrap(node.initializer)
      const readonly=Boolean(node.parent.flags&ts.NodeFlags.Const)&&node.initializer&&ts.isAsExpression(node.initializer)&&node.initializer.type.getText(program)==='const'
      if(readonly&&value&&ts.isArrayLiteralExpression(value)&&value.elements.every(element=>literalValue(element)&&!literalValue(element).dynamic))literalArrays.set(node.name.text,value.elements.map(element=>literalValue(element).value))
      if(value&&literalValue(value)&&(node.parent.flags&ts.NodeFlags.Const))literalConstants.set(node.name.text,value)
      if(value&&(ts.isPropertyAccessExpression(value)&&value.name.text==='style'||ts.isElementAccessExpression(value)&&literalValue(value.argumentExpression)?.value==='style'))styleAliases.add(node.name.text)
    }
    if(ts.isVariableDeclaration(node)&&ts.isObjectBindingPattern(node.name))for(const element of node.name.elements)if(ts.isIdentifier(element.name)&&(element.propertyName?.getText(program)||element.name.text)==='style')styleAliases.add(element.name.text)
    ts.forEachChild(node,gather)
  }
  gather(program)
  const staticColorArray=node=>{
    node=unwrap(node)
    const wrapped=ts.isCallExpression(node)&&ts.isIdentifier(node.expression)&&node.expression.text==='colorVar'&&node.arguments.length===1
    const lookup=unwrap(wrapped?node.arguments[0]:node)
    const values=ts.isElementAccessExpression(lookup)&&ts.isIdentifier(lookup.expression)?literalArrays.get(lookup.expression.text):null
    return values?.length&&values.every(value=>wrapped?contract.keys.has('--'+value):/^var\(--[a-z][a-z0-9-]*\)$/.test(value)&&contract.keys.has(value.slice(4,-1)))
  }
  const colorCall=node=>{
    node=unwrap(node)
    return ts.isCallExpression(node)&&node.arguments.length>0&&ts.isIdentifier(node.expression)&&['colorVar','colorValue'].includes(node.expression.text)&&contract.keys.has('--'+(literalValue(node.arguments[0])?.value||''))
  }
  const styleTarget=node=>{
    node=unwrap(node)
    return ts.isIdentifier(node)&&styleAliases.has(node.text)||ts.isPropertyAccessExpression(node)&&node.name.text==='style'||ts.isElementAccessExpression(node)&&literalValue(node.argumentExpression)?.value==='style'
  }
  const approvedStyleValue=(node,seen=new Set())=>{
    node=unwrap(node)
    if(literalValue(node))return true
    if(ts.isConditionalExpression(node))return approvedStyleValue(node.whenTrue,seen)&&approvedStyleValue(node.whenFalse,seen)
    if(ts.isIdentifier(node)&&literalConstants.has(node.text)&&!seen.has(node.text)){seen.add(node.text);return approvedStyleValue(literalConstants.get(node.text),seen)}
    return Boolean(colorCall(node)||staticColorArray(node))
  }
  const enforceStyleValue=(node,property)=>{
    if(property==='cssText'){
      const value=literalValue(node)
      if(!value||value.dynamic)report(node,'Dynamic cssText is forbidden; assign typed style properties using registered colors')
      else{checked.add(node);issues.push(...auditStylesheet(value.value,file,contract,{inline:true}))}
      return
    }
    if(colorProperty(kebab(property))&&!approvedStyleValue(node))report(node,`Dynamic ${property} must use literal registered colors or a static registered color array`)
    check(node,property)
  }
  const checked=new Set()
  const check=(node, property, force=false)=>{
    const value=literalValue(node)
    if(!value || checked.has(node))return
    const colored=colorProperty(kebab(property)) || property.startsWith('--') || force
    if(!colored)return
    checked.add(node)
    if(contract.keys.has(property))report(node,`Canonical color key ${property} cannot be redefined outside ${generatedCSS}`)
    if(property.startsWith('--')&&!contract.keys.has(property)){
      try{if(hasColor(tokens(value.value),contract.keys)&&!derivedKeys.get(property)?.includes(file))report(node,`Unregistered component color key ${property}; add it to the shared registry`)}catch{}
    }
    const dynamic=value.dynamic&&!(property.startsWith('--')&&/^\s*url\(/i.test(value.value))
    inspectValue(value.value,{file,contract,issue:message=>report(node,message),dynamic})
  }
  const propertyName=node=>node&&(ts.isComputedPropertyName(node)?literalValue(node.expression)?.value||'':ts.isIdentifier(node)||ts.isStringLiteralLike(node)?node.text:'')
  const visit=node=>{
    if(ts.isPropertyAssignment(node))check(node.initializer,propertyName(node.name))
    if(ts.isBinaryExpression(node)&&node.operatorToken.kind>=ts.SyntaxKind.FirstAssignment&&node.operatorToken.kind<=ts.SyntaxKind.LastAssignment){
      if(ts.isPropertyAccessExpression(node.left)){
        if(node.left.name.text==='cssText'){
          const value=literalValue(node.right)
          if(!value||value.dynamic)report(node,'Dynamic cssText is forbidden; assign typed style properties using registered colors')
          else{checked.add(node.right);issues.push(...auditStylesheet(value.value,file,contract,{inline:true}))}
        }else if(['innerHTML','outerHTML'].includes(node.left.name.text)){
          const value=literalValue(node.right)
          if(value){checked.add(node.right);issues.push(...auditHTML(value.value,file,contract))}
        }else if(styleTarget(node.left.expression))enforceStyleValue(node.right,node.left.name.text)
        else check(node.right,node.left.name.text)
      }
      if(ts.isElementAccessExpression(node.left)){
        const property=propertyName(node.left.argumentExpression)
        if(styleTarget(node.left.expression)){if(!property)report(node,'Dynamic style property names are forbidden');else enforceStyleValue(node.right,property)}
        else check(node.right,property)
      }
    }
    if(ts.isCallExpression(node)&&(ts.isPropertyAccessExpression(node.expression)||ts.isElementAccessExpression(node.expression))){
      const method=ts.isPropertyAccessExpression(node.expression)?node.expression.name.text:literalValue(node.expression.argumentExpression)?.value
      if(!method&&styleTarget(node.expression.expression))report(node,'Dynamic style method names are forbidden')
      if(method==='setProperty'&&node.arguments.length>=2){
        const property=literalValue(node.arguments[0])?.value
        if(property){
          if(contract.keys.has(property))report(node,`Canonical color key ${property} cannot be redefined outside ${generatedCSS}`)
          if(sampledKeys.has(property)){
            if(file!==sampledFile)report(node,`Sampled image color ${property} may only be written by ${sampledFile}`)
            else if(literalValue(node.arguments[1]))report(node,'Sampled image color slots cannot contain literal colors')
          }else if(urlSlots.has(property)){
            const value=literalValue(node.arguments[1])
            if(file!=='web/frontend/src/liquidGlass.ts'||!value||!/^url\(["']?#[\w-]+["']?\)$/.test(value.value))report(node,'The glass filter URL slot requires its approved local fragment writer')
          }else {
            const value=node.arguments[1]
            const registeredArray=derivedKeys.get(property)?.includes(file)&&staticColorArray(value)
            if(derivedKeys.has(property)&&!derivedKeys.get(property).includes(file))report(node,`Component color key ${property} cannot be assigned from ${file}`)
            if(property.startsWith('--')&&!literalValue(value)&&!registeredArray&&!(derivedKeys.has(property)?colorCall(value):numericStyleExpression(value)))report(node,`Dynamic custom property ${property} must supply a numeric parameter or a registered static color array`)
            if(colorProperty(kebab(property)))enforceStyleValue(value,property)
            else check(value,property)
          }
        }else{
          const expression=node.arguments[0],prefix=ts.isBinaryExpression(expression)&&expression.operatorToken.kind===ts.SyntaxKind.PlusToken?literalValue(expression.left)?.value:''
          const structuralName=file==='web/frontend/src/homeShowcase.ts'&&prefix==='--game-mask-'&&numericStyleExpression(node.arguments[1])
          if(!structuralName)report(node,'Dynamic color property names are forbidden; use one literal registered key')
        }
      }
      if(method==='supports'&&node.arguments.length>=2)check(node.arguments[1],literalValue(node.arguments[0])?.value||'')
      if(method==='setAttribute'&&node.arguments.length>=2){const property=literalValue(node.arguments[0])?.value||'';if(property==='style'){const value=literalValue(node.arguments[1]);if(!value||value.dynamic)report(node,'Dynamic inline style attributes are forbidden');else issues.push(...auditStylesheet(value.value,file,contract,{inline:true}))}else if(colorProperty(kebab(property)))enforceStyleValue(node.arguments[1],property);else check(node.arguments[1],property)}
      if(method==='insertAdjacentHTML'&&node.arguments.length>=2){const value=literalValue(node.arguments[1]);if(value)issues.push(...auditHTML(value.value,file,contract))}
    }
    if(ts.isCallExpression(node)&&(ts.isPropertyAccessExpression(node.expression)||ts.isElementAccessExpression(node.expression))&&ts.isIdentifier(node.expression.expression)&&node.expression.expression.text==='Object'&&(ts.isPropertyAccessExpression(node.expression)?node.expression.name.text:literalValue(node.expression.argumentExpression)?.value)==='assign'&&node.arguments.length>1&&styleTarget(node.arguments[0])){
      for(const argument of node.arguments.slice(1)){
        if(!ts.isObjectLiteralExpression(argument)){report(node,'Dynamic Object.assign style sources are forbidden');continue}
        for(const property of argument.properties)if(ts.isPropertyAssignment(property)){const name=propertyName(property.name);if(!name)report(property,'Dynamic style property names are forbidden');else enforceStyleValue(property.initializer,name)}else report(property,'Dynamic style property spreads are forbidden')
      }
    }
    const value=literalValue(node)
    if(value&&!checked.has(node)){
      const parent=node.parent, selector=parent&&ts.isCallExpression(parent)&&ts.isPropertyAccessExpression(parent.expression)&&['querySelector','querySelectorAll','closest','matches','getElementById'].includes(parent.expression.name.text)
      const nonColorContext=parent&&(ts.isPropertyAssignment(parent)&&!colorProperty(kebab(propertyName(parent.name)))||ts.isBinaryExpression(parent)&&ts.isPropertyAccessExpression(parent.left)&&!colorProperty(kebab(parent.left.name.text)))
      const raw=/^\s*#[\da-f]{3,8}\s*$/i.test(value.value)||/^\s*(?:black|white)\s*$/i.test(value.value)||!nonColorContext&&/^\s*(?:var|color-mix)\s*\(/i.test(value.value)||/\b(?:rgba?|hsla?|hwb|oklch|oklab|lab|lch|color)\s*\(/i.test(value.value)
      if(raw&&!selector)check(node,'color',true)
    }
    ts.forEachChild(node,visit)
  }
  visit(program)
  return issues
}

export function auditHTML(source,file,contract,{vue=false}={}) {
  const issues=[]
  const lineAt=index=>source.slice(0,index).split('\n').length
  for(const match of source.matchAll(/<style\b[^>]*>([\s\S]*?)<\/style\s*>/gi))issues.push(...auditStylesheet(match[1],file,contract).map(issue=>({...issue,line:issue.line+lineAt(match.index)-1})))
  if(vue){
    let template
    try{template=parseTemplate(source)}catch(error){return [...issues,{file,line:error.loc?.start?.line||1,message:`Malformed template: ${error.message}`}]}
    function visit(node){
      if(node.type===1)for(const attribute of node.props){
        const located=problems=>problems.map(problem=>({...problem,line:problem.line+attribute.loc.start.line-1}))
        if(attribute.type===6&&attribute.value){if(attribute.name==='style')issues.push(...located(auditStylesheet(attribute.value.content,file,contract,{inline:true})));else if(colorProperty(attribute.name))inspectValue(attribute.value.content,{file,contract,issue:message=>issues.push({file,line:attribute.loc.start.line,message})})}
        if(attribute.type===7&&attribute.name==='bind'&&attribute.exp){const name=attribute.arg?.content;if(name==='style')issues.push(...located(auditScript(`const __style=(${attribute.exp.content})`,file,contract)));else if(name&&colorProperty(name))issues.push(...located(auditScript(`const __style={${JSON.stringify(name)}:(${attribute.exp.content})}`,file,contract)))}
      }
      for(const child of node.children||[])visit(child)
    }
    visit(template)
  }else{
    // Go template actions are not HTML. Remove them before reading attributes;
    // values supplied by the renderer still receive Go-side contract checks.
    const html=source.replace(/{{[\s\S]*?}}/g,match=>' '.repeat(match.length))
    for(const tag of html.matchAll(/<[A-Za-z][^>]*>/g))for(const match of tag[0].matchAll(/\s(style|color|fill|stroke|bgcolor|stop-color|flood-color|lighting-color|solid-color)\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s>]+))/gi)){
      const property=match[1].toLowerCase(),value=(match[2]??match[3]??match[4]).replace(/&#(x[\da-f]+|\d+);?|&(amp|quot|apos|lt|gt|num|colon|lpar|rpar);/gi,(_,number,name)=>number?String.fromCodePoint(Number.parseInt(number[0].toLowerCase()==='x'?number.slice(1):number,number[0].toLowerCase()==='x'?16:10)):({amp:'&',quot:'"',apos:"'",lt:'<',gt:'>',num:'#',colon:':',lpar:'(',rpar:')'}[name.toLowerCase()]))
      if(property==='style')issues.push(...auditStylesheet(value,file,contract,{inline:true}).map(issue=>({...issue,line:lineAt(tag.index)})))
      else inspectValue(value,{file,contract,issue:message=>issues.push({file,line:lineAt(tag.index),message})})
    }
  }
  return issues
}

export function auditFile(source,file,contract) {
  if(file.endsWith('.css'))return auditStylesheet(source,file,contract)
  if(file.endsWith('.ts'))return auditScript(source,file,contract)
  if(file.endsWith('.vue')){
    const {descriptor,errors}=parseVue(source,{filename:file})
    if(errors.length)return errors.map(error=>({file,line:error.loc?.start?.line||1,message:`Malformed Vue file: ${error.message||error}`}))
    const located=(problems,block)=>problems.map(problem=>({...problem,line:problem.line+block.loc.start.line-1}))
    return [...descriptor.styles.flatMap(style=>located(auditStylesheet(style.content,file,contract),style)),...[descriptor.script,descriptor.scriptSetup].filter(Boolean).flatMap(script=>located(auditScript(script.content,file,contract),script)),...(descriptor.template?located(auditHTML(descriptor.template.content,file,contract,{vue:true}),descriptor.template):[])]
  }
  if(file.endsWith('.html'))return auditHTML(source,file,contract)
  if(file.endsWith('.go')){
    const issues=auditHTML(source,file,contract)
    for(const match of source.matchAll(/(?:"(?:[^"\\]|\\.)*"|`[^`]*`)/g)){
      let value=match[0].slice(1,-1)
      if(match[0][0]==='"'){try{value=JSON.parse(match[0])}catch{continue}}
      if(/^\s*#[\da-f]{3,8}\s*$/i.test(value))issues.push({file,line:source.slice(0,match.index).split('\n').length,message:`Raw Go UI color ${value}; consume the shared color registry`})
    }
    return issues
  }
  return []
}

export function checkRepository(root) {
  const contract=createContract(JSON.parse(fs.readFileSync(path.join(root,'web/colors.json'),'utf8')))
  const files=new Set(execFileSync('git',['ls-files','-z','--cached','--others','--exclude-standard','--','web/frontend/src','web/templates'],{cwd:root,encoding:'utf8'}).split('\0').filter(file=>/\.(?:css|ts|vue|html)$/.test(file)))
  for(const file of ['internal/app/oidc.go','internal/app/pwa.go','internal/app/covers.go'])files.add(file)
  const issues=[]
  for(const file of [...files].sort())if(fs.existsSync(path.join(root,file)))issues.push(...auditFile(fs.readFileSync(path.join(root,file),'utf8'),file,contract))
  return {issues,files:files.size}
}

if(process.argv[1]&&path.resolve(process.argv[1])===fileURLToPath(import.meta.url)){
  try{
    const root=path.resolve(path.dirname(fileURLToPath(import.meta.url)),'../../..')
    const {issues,files}=checkRepository(root)
    for(const issue of issues)console.error(`${issue.file}:${issue.line}: ${issue.message}`)
    if(issues.length)process.exitCode=1
    else console.log(`Color contract passed (${files} UI source files)`)
  }catch(error){console.error(`Color contract failed: ${error.message}`);process.exitCode=1}
}
