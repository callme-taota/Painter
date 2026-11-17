<script setup lang="ts">
import {t} from "../i18n"

import {computed,nextTick,onBeforeUnmount,onMounted,ref,shallowRef,watch} from 'vue'
import {Editor,EditorContent} from '@tiptap/vue-3'
import {Extension,getMarkRange} from '@tiptap/core'
import StarterKit from '@tiptap/starter-kit'
import {Markdown} from '@tiptap/markdown'
import Image from '@tiptap/extension-image'
import {TableKit,TableCell,TableHeader} from '@tiptap/extension-table'
import {TaskItem,TaskList} from '@tiptap/extension-list'
import Placeholder from '@tiptap/extension-placeholder'
import Paragraph from '@tiptap/extension-paragraph'
import {markdownCompatibility,safeMediaURL} from '../editorMarkdown'
import {randomID} from '../random'
import Icon from './Icon.vue'
import SelectField from './SelectField.vue'
import ArticleCardComposer from './ArticleCardComposer.vue'
import {ArticleCardNode,ArticleReferenceLink,parseArticleCard,parseArticleReference,articleLinkSnapshot,articleReferenceTitle,type ArticleCard,type ArticleCardType} from '../articleCards'
import {initArticleCards} from '../contentCards'
import {toast} from '../toast'

const props=defineProps<{modelValue:string;disabled?:boolean;active?:boolean}>()
const emit=defineEmits<{'update:modelValue':[value:string];save:[];'pick-image':[];image:[file:File];compatibility:[issue:string];composition:[value:boolean];'source-paste':[value:string]}>()
const editor=shallowRef<Editor>(),issue=ref(''),revision=ref(0),composing=ref(false),linkOpen=ref(false),linkValue=ref(''),linkError=ref(''),linkInput=ref<HTMLInputElement>()
const cardOpen=ref(false),cardInitial=ref<ArticleCard|null>(null),cardKind=ref<ArticleCardType>('github'),cardLabel=ref(''),cardInline=ref(false),cardPresentation=ref<'social'>(),cardRange=ref<{from:number;to:number;block:boolean}|null>(null)
const slash=ref<{from:number;to:number;query:string;x:number;y:number}|null>(null),slashIndex=ref(0),bubble=ref<{x:number;y:number}|null>(null)
let applying=false,pending=false,lastEmitted=props.modelValue,compositionTimer:ReturnType<typeof setTimeout>|undefined,slashDismissedAt=-1
const editable=computed(()=>!props.disabled&&!issue.value)
// Tiptap's default Markdown paragraph parser hoists a standalone image into the
// document root, even when Image is inline. Keep both image forms valid Markdown.
const ImageParagraph=Paragraph.extend({parseMarkdown(token,helpers){if(token.tokens?.length===1&&token.tokens[0].type==='image')return helpers.createNode('paragraph',undefined,helpers.parseInline(token.tokens));return this.parent?.(token,helpers)||[]}})
const blockType=computed(()=>{revision.value;for(const level of [1,2,3])if(editor.value?.isActive('heading',{level}))return String(level);return 'paragraph'})
const commands=[
  {label:t("正文"),hint:t("普通段落"),search:'text paragraph zhengwen',run:(e:Editor)=>e.chain().focus().setParagraph().run()},
  {label:t("一级标题"),hint:t("大标题"),search:'h1 heading title biaoti',run:(e:Editor)=>e.chain().focus().setHeading({level:1}).run()},
  {label:t("二级标题"),hint:t("章节"),search:'h2 heading biaoti',run:(e:Editor)=>e.chain().focus().setHeading({level:2}).run()},
  {label:t("三级标题"),hint:t("小节"),search:'h3 heading biaoti',run:(e:Editor)=>e.chain().focus().setHeading({level:3}).run()},
  {label:t("无序列表"),hint:t("项目列表"),search:'list bullet liebiao',run:(e:Editor)=>e.chain().focus().toggleBulletList().run()},
  {label:t("有序列表"),hint:t("按序排列"),search:'list ordered number liebiao',run:(e:Editor)=>e.chain().focus().toggleOrderedList().run()},
  {label:t("待办列表"),hint:t("可勾选事项"),search:'task todo check daiban',run:(e:Editor)=>e.chain().focus().toggleTaskList().run()},
  {label:t("引用"),hint:t("引用文字"),search:'quote yinyong',run:(e:Editor)=>e.chain().focus().toggleBlockquote().run()},
  {label:t("代码块"),hint:t("保留代码格式"),search:'code daima',run:(e:Editor)=>e.chain().focus().toggleCodeBlock().run()},
  {label:t("表格"),hint:t("3 列 × 3 行"),search:'table biaoge',run:(e:Editor)=>e.chain().focus().insertTable({rows:3,cols:3,withHeaderRow:true}).run()},
  {label:t("图片"),hint:t("从设备上传"),search:'image photo tupian',run:()=>{emit('pick-image');return true}},
  {label:t("提示"),hint:t("说明、判断或注意事项"),search:'callout note insight warning tishi',run:()=>{openCards('callout');return true}},
  {label:t("指标"),hint:t("数值、口径与来源"),search:'metrics number data zhibiao',run:()=>{openCards('metrics');return true}},
  {label:t("摘要组"),hint:t("并列要点与结论"),search:'summary group zhaiyao',run:()=>{openCards('summary');return true}},
  {label:t("组合卡片"),hint:t("组合文字、指标、来源与图片"),search:'layout compose zuhe',run:()=>{openCards('layout');return true}},
  {label:t("GitHub 仓库"),hint:t("公开仓库卡片"),search:'github repository card kapian',run:()=>{openCards('github');return true}},
  {label:t("Steam 游戏"),hint:t("商店游戏卡片"),search:'steam game card youxi',run:()=>{openCards('steam');return true}},
  {label:t("B站视频"),hint:t("点击加载的站内视频播放器"),search:'bilibili video BV av shipin',run:()=>{openCards('bilibili');return true}},
  {label:'Apple Music',hint:t("专辑或歌单卡片"),search:'apple music album playlist card yinyue',run:()=>{openCards('apple_music');return true}},
  {label:t("社交分享"),hint:t("作者、文字与封面的链接卡片"),search:'social share weibo xhs x twitter douyin fenxiang',run:()=>{openCards('link',undefined,'social');return true}},
  {label:t("网页参考"),hint:t("文字、Emoji 与来源预览"),search:'link reference emoji card wangye',run:()=>{openCards('link');return true}},
  {label:t("站内动态"),hint:t("引用已公开的动态与照片"),search:'moment internal card dongtai',run:()=>{openCards('moment');return true}},
  {label:t("站内片语"),hint:t("引用已公开的句子或词语"),search:'phrase sentence word internal card pianyu',run:()=>{openCards('phrase');return true}},
  {label:t("分隔线"),hint:t("分开两段内容"),search:'divider horizontal rule fengexian',run:(e:Editor)=>e.chain().focus().setHorizontalRule().run()}
]
const filteredCommands=computed(()=>{const query=(slash.value?.query||'').toLowerCase();return commands.filter(c=>(c.label+' '+c.search).toLowerCase().includes(query))})
function isActive(name:string){revision.value;return editor.value?.isActive(name)||false}
function isComposing(){return composing.value||!!editor.value?.view.composing}
function commit(){const e=editor.value;if(!e||applying||!pending||isComposing()||issue.value)return;pending=false;const markdown=e.getMarkdown();if(markdown!==lastEmitted){lastEmitted=markdown;emit('update:modelValue',markdown)}}
function inspect(){const e=editor.value;if(!e)return;issue.value=markdownCompatibility(props.modelValue,e.getMarkdown());e.setEditable(editable.value,false);emit('compatibility',issue.value);void nextTick(()=>initArticleCards(e.view.dom))}
function insertMarkdown(value:string){if(!editor.value||!editable.value)return false;editor.value.chain().focus().insertContent(value,{contentType:'markdown'}).run();return true}
function replaceText(token:string,replacement:string){const e=editor.value;if(!e||!editable.value)return false;let range:{from:number;to:number}|null=null;e.state.doc.descendants((node,pos)=>{if(range||!node.isText)return;const at=node.text?.indexOf(token)??-1;if(at>=0)range={from:pos+at,to:pos+at+token.length}});if(!range)return false;e.commands.insertContentAt(range,replacement,{contentType:'markdown',updateSelection:false});return true}
function chooseBlock(value:string|number){if(!editable.value)return;if(value==='paragraph')editor.value?.chain().focus().setParagraph().run();else editor.value?.chain().focus().setHeading({level:Number(value) as 1|2|3}).run()}
function selectedLinkCard(e:Editor){const attrs=e.getAttributes('link'),range=getMarkRange(e.state.selection.$from,e.schema.marks.link,attrs);if(!range)return null;const label=e.state.doc.textBetween(range.from,range.to,' '),card=parseArticleReference(attrs.title,attrs.href)||articleLinkSnapshot(attrs.href,label,attrs.title);return card?{...range,label,card}:null}
function openLink(){const e=editor.value;if(!e)return;const node=e.state.doc.nodeAt(e.state.selection.from),block=node?.type.name==='articleCard'?parseArticleCard(node.attrs.snapshot):null;if(block?.type==='link'||selectedLinkCard(e)){openCards();return}const attrs=e.getAttributes('link');cardOpen.value=false;linkValue.value=attrs.href||'';linkError.value='';linkOpen.value=true;void nextTick(()=>linkInput.value?.focus())}
function applyLink(){if(!editable.value)return;const e=editor.value;if(!e)return;const value=linkValue.value.trim();if(value&&!safeMediaURL(value)){linkError.value=t("请输入网页、邮件或站内链接。");toast.error(linkError.value);return}const chain=e.chain().focus().extendMarkRange('link'),applied=value?chain.setLink({href:value}).run():chain.unsetLink().run();if(!applied){toast.error(t("链接尚未应用，请重新选择文字后重试。"));return}linkOpen.value=false;toast.success(value?t("链接已应用到当前草稿。"):t("链接已从当前文字移除。"))}
function openCards(kind?:ArticleCardType,position?:number,presentation?:'social'){
 const e=editor.value;if(!e||!editable.value)return
 linkOpen.value=false;slash.value=null;bubble.value=null
 let from=position??e.state.selection.from,to=e.state.selection.to,initial:ArticleCard|null=null,block=false,label=e.state.doc.textBetween(from,to,' ')
 if(!kind){const node=e.state.doc.nodeAt(from);if(node?.type.name==='articleCard'){initial=parseArticleCard(node.attrs.snapshot);to=from+node.nodeSize;block=true}else{const link=selectedLinkCard(e);if(link){initial=link.card;from=link.from;to=link.to;label=link.label}}}
 cardPresentation.value=presentation;cardInitial.value=initial;cardKind.value=initial?.type||kind||'github';cardLabel.value=label;cardInline.value=!presentation&&(initial?.type==='link'&&!block||!initial&&kind==='link');cardRange.value={from,to,block};cardOpen.value=true
}
function closeCards(){cardOpen.value=false;editor.value?.commands.focus()}
function applyCard(value:{card:ArticleCard;inline:boolean;label:string}){
 const e=editor.value,range=cardRange.value;if(!e||!range||!editable.value)return
 let applied=false
 if(value.inline&&value.card.v===1&&value.card.type==='link'){const attrs={href:value.card.url,title:articleReferenceTitle(value.card)};if(!range.block&&e.state.doc.textBetween(range.from,range.to,' ')===value.label&&range.from!==range.to)applied=e.chain().focus().setTextSelection(range).setMark('link',attrs).run();else{const text={type:'text',text:value.label,marks:[{type:'link',attrs}]};applied=e.chain().focus().insertContentAt(range,range.block?{type:'paragraph',content:[text]}:text,{contentType:'json'}).run()}}
 else{const node={type:'articleCard',attrs:{snapshot:JSON.stringify(value.card)}};applied=e.chain().focus().insertContentAt(range,range.block?node:[node,{type:'paragraph'}],{contentType:'json'}).run()}
 if(!applied){toast.error(t("卡片尚未应用，请重新选择插入位置后重试。"));return}
 cardOpen.value=false;toast.success(cardInitial.value?t("卡片已更新到当前草稿，尚未发布。"):t("卡片已插入当前草稿，尚未发布。"))
}
function updateMenus(){const e=editor.value;if(!e||!props.active||!editable.value||isComposing()){slash.value=null;bubble.value=null;return}const {from,to,empty,$from}=e.state.selection
  bubble.value=null
  if(!empty&&!e.isActive('codeBlock')){try{const start=e.view.coordsAtPos(from),end=e.view.coordsAtPos(to);bubble.value={x:Math.max(12,Math.min(innerWidth-180,(start.left+end.right)/2-75)),y:Math.max(8,start.top-48)}}catch{}}
  const text=$from.parent.textBetween(0,$from.parentOffset,'\n','\ufffc'),match=empty&&$from.parent.type.name==='paragraph'&&/^\/([^\s/]*)$/.exec(text)
  if(match&&from!==slashDismissedAt){try{const box=e.view.coordsAtPos(from);slash.value={from:from-text.length,to:from,query:match[1],x:Math.max(8,Math.min(innerWidth-288,box.left)),y:Math.max(8,Math.min(innerHeight-350,box.bottom+8))};slashIndex.value=Math.min(slashIndex.value,Math.max(0,filteredCommands.value.length-1));return}catch{}}
  slash.value=null;slashIndex.value=0
}
function runSlash(index:number){if(!editable.value)return;const e=editor.value,menu=slash.value,item=filteredCommands.value[index];if(!e||!menu||!item)return;e.chain().focus().deleteRange({from:menu.from,to:menu.to}).run();slash.value=null;item.run(e)}
function onKeyDown(event:KeyboardEvent){if(event.isComposing||event.keyCode===229||isComposing())return false;if((event.ctrlKey||event.metaKey)&&event.key.toLowerCase()==='s'){event.preventDefault();commit();emit('save');return true}if(!slash.value)return false;if(event.key==='Escape'){slashDismissedAt=editor.value?.state.selection.from||-1;slash.value=null;event.preventDefault();return true}if(['ArrowDown','ArrowUp'].includes(event.key)){const count=filteredCommands.value.length;if(count)slashIndex.value=(slashIndex.value+(event.key==='ArrowDown'?1:-1)+count)%count;event.preventDefault();return true}if(event.key==='Enter'&&filteredCommands.value.length){event.preventDefault();runSlash(slashIndex.value);return true}return false}
function compositionStart(){composing.value=true;slash.value=null;bubble.value=null;emit('composition',true)}
function compositionEnd(){clearTimeout(compositionTimer);compositionTimer=setTimeout(()=>{composing.value=false;emit('composition',false);commit();updateMenus()},30)}
function pastedMarkdown(text:string){const e=editor.value;if(!e||!text)return false;const json=e.markdown?.parse(text);if(!json)return false;const issue=markdownCompatibility(text,e.markdown!.serialize(json));if(issue){const marker='PAINTERSOURCEPASTE'+randomID().replace(/[^a-z0-9]/gi,'');let source='';applying=true;try{e.commands.insertContent({type:'text',text:marker},{contentType:'json'});source=e.getMarkdown().replace(marker,text)}finally{applying=false}emit('source-paste',source);return true}if(!/(?:^|\n)(?:#{1,6}\s|[-*+]\s|\d+[.)]\s|>\s|```|~~~|\|)|\*\*[^*]+\*\*|!?\[[^\]]+\]\(/m.test(text))return false;e.commands.insertContent(text,{contentType:'markdown'});return true}
onMounted(()=>{
  try{editor.value=new Editor({
    content:props.modelValue,contentType:'markdown',editable:!props.disabled,
    extensions:[StarterKit.configure({paragraph:false,underline:false,link:false,trailingNode:false}),ArticleReferenceLink.configure({openOnClick:false,autolink:true,defaultProtocol:'https'}),ArticleCardNode.configure({onEdit:position=>openCards(undefined,position)}),ImageParagraph,Image.configure({inline:true}),TableKit.configure({table:{resizable:false,allowTableNodeSelection:true},tableCell:false,tableHeader:false}),TableCell.extend({content:'paragraph'}),TableHeader.extend({content:'paragraph'}),TaskList,TaskItem.configure({nested:true}),Placeholder.configure({placeholder:t("开始写作，输入 / 插入标题、列表、卡片或图片…")}),Markdown.configure({markedOptions:{gfm:true,breaks:false}}),Extension.create({name:'painterMarkdownTab',addKeyboardShortcuts(){return {Tab:()=>{if(this.editor.isActive('codeBlock'))return this.editor.commands.insertContent('  ');return false}}}})],
    editorProps:{attributes:{class:'document-prose',role:'textbox','aria-label':t("文档正文"),'aria-multiline':'true',spellcheck:'false'},handleKeyDown:(_view,event)=>onKeyDown(event),handlePaste:(_view,event)=>{if(!editable.value)return false;const file=Array.from(event.clipboardData?.files||[]).find(f=>f.type.startsWith('image/'));if(file){emit('image',file);event.preventDefault();return true}const text=event.clipboardData?.getData('text/plain')||'';if(!event.clipboardData?.getData('text/html')&&pastedMarkdown(text)){event.preventDefault();return true}return false},handleDrop:(view,event)=>{if(!editable.value)return false;const file=Array.from(event.dataTransfer?.files||[]).find(f=>f.type.startsWith('image/'));if(!file)return false;event.preventDefault();const position=view.posAtCoords({left:event.clientX,top:event.clientY});if(position)editor.value?.commands.setTextSelection(position.pos);emit('image',file);return true},handleDOMEvents:{compositionstart:()=>{compositionStart();return false},compositionend:()=>{compositionEnd();return false}}},
    onUpdate:({transaction})=>{if(applying||!transaction.docChanged)return;if(cardOpen.value&&cardRange.value){const range=cardRange.value,from=transaction.mapping.map(range.from,1);cardRange.value={...range,from,to:range.from===range.to?from:Math.max(from,transaction.mapping.map(range.to,-1))}}pending=true;commit();revision.value++;updateMenus();void nextTick(()=>{if(editor.value)initArticleCards(editor.value.view.dom)})},
    onSelectionUpdate:()=>{revision.value++;updateMenus()},
    onFocus:()=>updateMenus(),onBlur:()=>{bubble.value=null}
  });inspect();window.addEventListener('scroll',updateMenus,true);window.addEventListener('resize',updateMenus)}catch{issue.value=t("无法安全解析的 Markdown 结构");editor.value?.setEditable(false,false);emit('compatibility',issue.value)}
})
watch(()=>props.modelValue,value=>{const e=editor.value;if(!e||value===lastEmitted)return;applying=true;pending=false;lastEmitted=value;try{e.commands.setContent(value,{contentType:'markdown',emitUpdate:false});inspect()}catch{issue.value=t("无法安全解析的 Markdown 结构");e.setEditable(false,false);emit('compatibility',issue.value)}finally{applying=false}revision.value++;updateMenus()})
watch(editable,value=>{editor.value?.setEditable(value,false);if(!value){linkOpen.value=false;cardOpen.value=false}updateMenus()},{flush:'sync'})
watch(()=>props.active,()=>{linkOpen.value=false;cardOpen.value=false;updateMenus()})
onBeforeUnmount(()=>{clearTimeout(compositionTimer);window.removeEventListener('scroll',updateMenus,true);window.removeEventListener('resize',updateMenus);editor.value?.destroy()})
defineExpose({insertMarkdown,replaceText,isComposing,flush:commit,openCards,focus:()=>editor.value?.commands.focus()})
</script>

<template>
  <div class="document-editor" :class="{'document-readonly':!editable}">
    <div class="document-toolbar" role="toolbar" :aria-label="t('文档格式工具栏')">
      <div class="document-block-type"><SelectField :model-value="blockType" :disabled="!editable" @update:model-value="chooseBlock" :label="t('段落样式')" :options="[{value:'paragraph',label:t('正文')},{value:'1',label:t('一级标题')},{value:'2',label:t('二级标题')},{value:'3',label:t('三级标题')}]"/></div>
      <div class="document-tool-group" role="group" :aria-label="t('文字格式')">
        <button :disabled="!editable" :aria-pressed="isActive('bold')" :aria-label="t('加粗')" :title="t('加粗 · Ctrl / ⌘ B')" @mousedown.prevent @click="editor?.chain().focus().toggleBold().run()"><b>B</b></button>
        <button :disabled="!editable" :aria-pressed="isActive('italic')" :aria-label="t('斜体')" :title="t('斜体 · Ctrl / ⌘ I')" @mousedown.prevent @click="editor?.chain().focus().toggleItalic().run()"><i>I</i></button>
        <button :disabled="!editable" :aria-pressed="isActive('strike')" :aria-label="t('删除线')" :title="t('删除线')" @mousedown.prevent @click="editor?.chain().focus().toggleStrike().run()"><s>S</s></button>
        <button :disabled="!editable" :aria-pressed="isActive('link')" :title="t('编辑链接')" @mousedown.prevent @click="openLink">{{t("链接")}}</button>
      </div>
      <div class="document-tool-group" role="group" :aria-label="t('插入内容')">
        <button :disabled="!editable" :aria-pressed="isActive('bulletList')" @mousedown.prevent @click="editor?.chain().focus().toggleBulletList().run()">{{t("列表")}}</button>
        <button :disabled="!editable" :aria-pressed="isActive('blockquote')" @mousedown.prevent @click="editor?.chain().focus().toggleBlockquote().run()">{{t("引用")}}</button>
        <button :disabled="!editable" :aria-pressed="isActive('codeBlock')" @mousedown.prevent @click="editor?.chain().focus().toggleCodeBlock().run()">{{t("代码")}}</button>
        <button :disabled="!editable" :title="t('插入表格')" @mousedown.prevent @click="editor?.chain().focus().insertTable({rows:3,cols:3,withHeaderRow:true}).run()">{{t("表格")}}</button>
        <button :disabled="!editable" :title="t('插入或编辑内容卡片')" data-open-cards @mousedown.prevent @click="openCards()">{{t("卡片")}}</button>
        <button :disabled="!editable" :title="t('上传图片')" :aria-label="t('插入图片')" @mousedown.prevent @click="emit('pick-image')"><Icon name="image" :size="17"/></button>
      </div>
      <div class="document-tool-group document-history" role="group" :aria-label="t('编辑历史')">
        <button :disabled="!editable||!editor?.can().undo()" :aria-label="t('撤销')" :title="t('撤销 · Ctrl / ⌘ Z')" @mousedown.prevent @click="editor?.chain().focus().undo().run()"><Icon name="undo" :size="17"/></button>
        <button :disabled="!editable||!editor?.can().redo()" :aria-label="t('重做')" :title="t('重做 · Ctrl / ⌘ Shift Z')" @mousedown.prevent @click="editor?.chain().focus().redo().run()"><Icon name="redo" :size="17"/></button>
      </div>
    </div>
    <form v-if="linkOpen" class="document-link-form" @submit.prevent="applyLink"><label>{{t("链接地址")}}<input ref="linkInput" v-model="linkValue" :placeholder="t('https:// 或站内路径')" @keydown.esc.prevent="linkOpen=false;editor?.commands.focus()"></label><button type="submit">{{t("应用链接")}}</button><button type="button" class="quiet" @click="linkValue='';applyLink()">{{t("移除链接")}}</button><button type="button" class="quiet" @click="linkOpen=false;editor?.commands.focus()">{{t("取消")}}</button><p v-if="linkError" role="alert">{{linkError}}</p></form>
    <ArticleCardComposer v-if="cardOpen" :initial="cardInitial" :kind="cardKind" :preferred-display="cardPresentation" :label="cardLabel" :inline="cardInline" :editing="!!cardInitial" :disabled="!editable" @close="closeCards" @apply="applyCard"/>
    <div v-if="editor?.isActive('table')&&editable" class="document-table-tools" role="group" :aria-label="t('表格操作')"><button @mousedown.prevent @click="editor?.chain().focus().addRowAfter().run()">{{t("添一行")}}</button><button @mousedown.prevent @click="editor?.chain().focus().addColumnAfter().run()">{{t("添一列")}}</button><button @mousedown.prevent @click="editor?.chain().focus().deleteRow().run()">{{t("删除本行")}}</button><button @mousedown.prevent @click="editor?.chain().focus().deleteColumn().run()">{{t("删除本列")}}</button><button @mousedown.prevent @click="editor?.chain().focus().deleteTable().run()">{{t("删除表格")}}</button></div>
    <EditorContent :editor="editor"/>
    <div class="document-footnote"><span>{{issue?t("请在 Markdown 源码中编辑这篇文章。"):t("输入 / 插入内容，选中文字设置格式。")}}</span><span>{{t("Markdown 保存")}}</span></div>
    <Teleport to="body">
      <div v-if="slash&&active" class="document-slash" role="listbox" :aria-label="t('插入内容')" :style="{left:slash.x+'px',top:slash.y+'px'}"><div class="document-menu-label">{{t("插入内容")}}</div><button v-for="(item,index) in filteredCommands" :key="item.label" role="option" :aria-selected="index===slashIndex" :class="{selected:index===slashIndex}" @mousedown.prevent @mousemove="slashIndex=index" @click="runSlash(index)"><span>{{item.label}}</span><small>{{item.hint}}</small></button><p v-if="!filteredCommands.length">{{t("没有匹配的内容。按 Esc 继续写作。")}}</p><div class="document-menu-help">{{t("上下键选择 · Enter 插入 · Esc 关闭")}}</div></div>
      <div v-if="bubble&&active&&!linkOpen&&!cardOpen" class="document-bubble" role="toolbar" :aria-label="t('选中文字格式')" :style="{left:bubble.x+'px',top:bubble.y+'px'}"><button :aria-pressed="isActive('bold')" :aria-label="t('加粗所选文字')" @mousedown.prevent @click="editor?.chain().focus().toggleBold().run()"><b>B</b></button><button :aria-pressed="isActive('italic')" :aria-label="t('斜体所选文字')" @mousedown.prevent @click="editor?.chain().focus().toggleItalic().run()"><i>I</i></button><button @mousedown.prevent @click="openLink">{{t("链接")}}</button></div>
    </Teleport>
  </div>
</template>

<style scoped>
.document-editor{background:var(--paper);border:1px solid var(--line);border-radius:var(--radius-surface);min-width:0;color:var(--ink)}
.document-toolbar{display:flex;align-items:center;flex-wrap:wrap;gap:4px 8px;padding:10px 12px;border-bottom:1px solid var(--line)}
.document-toolbar button,.document-table-tools button,.document-bubble button{min-width:36px;min-height:var(--control-height);padding:6px 9px;background:transparent;border:0;border-radius:var(--radius-control);font-size:var(--control-font);color:var(--ink);line-height:var(--control-leading)}
.document-toolbar button:hover,.document-table-tools button:hover,.document-bubble button:hover{background:var(--subtle);line-height:var(--control-leading)}
.document-toolbar button[aria-pressed=true],.document-bubble button[aria-pressed=true]{background:var(--accent-soft);color:var(--accent);line-height:var(--control-leading)}
.document-tool-group{display:flex;align-items:center;gap:2px}.document-tool-group+.document-tool-group{border-left:1px solid var(--line);padding-left:8px}.document-history{margin-left:auto}
.document-block-type{margin:0}.document-block-type select{font-size:.78rem;min-height:36px;padding:5px 24px 5px 8px;max-width:125px;border:0;background-color:transparent}
.document-link-form{display:flex;flex-wrap:wrap;align-items:end;gap:8px;padding:12px 16px;background:var(--subtle);border-bottom:1px solid var(--line)}.document-link-form label{flex:1;min-width:180px;font-size:.75rem}.document-link-form input{font-size:.85rem}.document-link-form button{font-size:.75rem}.document-link-form p{flex-basis:100%;margin:0;color:var(--danger);font-size:.8rem}
.document-table-tools{display:flex;flex-wrap:wrap;gap:4px;padding:6px 12px;background:var(--subtle);border-bottom:1px solid var(--line)}
.document-editor :deep(.document-prose){min-height:540px;max-width:850px;margin:0 auto;padding:36px 44px 80px;font-family:var(--font-ui);font-size:18px;line-height:1.85;outline:none;overflow-wrap:anywhere;caret-color:var(--accent);tab-size:2}
/* Keep focus at the writing entry instead of framing the entire document. */
.document-editor :deep(.document-prose:focus-visible){outline:none}
.document-editor:has(.document-prose:focus-visible) .document-toolbar{border-bottom-color:var(--accent)}
@media(forced-colors:active){.document-editor:has(.document-prose:focus-visible) .document-toolbar{border-bottom-color:Highlight}}
.document-editor :deep(.document-prose>p){margin:0 0 1.25em}.document-editor :deep(.document-prose>p:last-child){margin-bottom:0}
.document-editor :deep(.document-prose h1),.document-editor :deep(.document-prose h2),.document-editor :deep(.document-prose h3){font-family:var(--font-display);font-weight:600;line-height:1.6;margin:1.8em 0 .7em;letter-spacing:-.02em;text-wrap:balance}
.document-editor :deep(.document-prose h1){font-size:2rem}.document-editor :deep(.document-prose h2){font-size:1.6rem}.document-editor :deep(.document-prose h3){font-size:1.25rem}.document-editor :deep(.document-prose>:first-child){margin-top:0}
.document-editor :deep(.document-prose ul),.document-editor :deep(.document-prose ol){padding-left:1.6em;margin:1em 0}.document-editor :deep(.document-prose li p){margin:.25em 0}.document-editor :deep(.document-prose li>ul),.document-editor :deep(.document-prose li>ol){margin:.25em 0}
.document-editor :deep(.document-prose ul[data-type=taskList]){list-style:none;padding-left:0}.document-editor :deep(.document-prose ul[data-type=taskList] li){display:flex;gap:10px;align-items:flex-start}.document-editor :deep(.document-prose ul[data-type=taskList] li>label){margin:.25em 0 0;flex:none}.document-editor :deep(.document-prose ul[data-type=taskList] li>div){flex:1;min-width:0}.document-editor :deep(.document-prose input[type=checkbox]){width:17px;min-height:17px;padding:0;accent-color:var(--accent)}
.document-editor :deep(.document-prose blockquote){margin:1.6em 0;padding:0 0 0 20px;border-left:1px solid var(--muted);color:var(--muted)}.document-editor :deep(.document-prose blockquote p){margin:.7em 0}
.document-editor :deep(.document-prose pre){font-family:"SFMono-Regular",Consolas,"Noto Sans CJK SC",monospace;font-size:.85em;line-height:1.75;background:var(--subtle);padding:20px;border-radius:var(--radius-menu);overflow-x:auto;white-space:pre;margin:1.5em 0}.document-editor :deep(.document-prose code){font-family:"SFMono-Regular",Consolas,"Noto Sans CJK SC",monospace;font-size:.88em;background:var(--subtle);padding:2px 4px;border-radius:var(--radius-item)}.document-editor :deep(.document-prose pre code){padding:0;background:transparent;font-size:inherit}
.document-editor :deep(.document-prose a){color:var(--accent);text-decoration:underline;text-underline-offset:3px}.document-editor :deep(.document-prose img){max-width:100%;height:auto;border-radius:var(--radius-item);vertical-align:middle}.document-editor :deep(.document-prose .ProseMirror-selectednode){outline:2px solid var(--accent)}
.document-editor :deep(.document-prose .tableWrapper){overflow-x:auto;margin:1.5em 0}.document-editor :deep(.document-prose table){border-collapse:collapse;width:100%;table-layout:fixed;margin:1.5em 0}.document-editor :deep(.document-prose td),.document-editor :deep(.document-prose th){border:1px solid var(--line);padding:8px 12px;vertical-align:top;position:relative;min-width:64px}.document-editor :deep(.document-prose th){background:var(--subtle);font-weight:600}.document-editor :deep(.document-prose td p),.document-editor :deep(.document-prose th p){margin:0}.document-editor :deep(.document-prose .selectedCell){background:var(--accent-soft)}
.document-editor :deep(.document-prose hr){border:0;border-top:1px solid var(--line);margin:2em 0}.document-editor :deep(.document-prose p.is-editor-empty:first-child::before){content:attr(data-placeholder);float:left;color:var(--muted);pointer-events:none;height:0}.document-editor :deep(.document-prose ::selection){background:var(--accent-soft);color:var(--ink)}
.document-footnote{display:flex;justify-content:space-between;gap:12px;border-top:1px solid var(--line);padding:12px 20px;font-size:.72rem;color:var(--muted)}
.document-slash,.document-bubble{position:fixed;z-index:80;background:var(--paper);color:var(--ink);box-shadow:var(--surface-shadow,var(--shadow));border-radius:var(--radius-menu);padding:6px;max-width:calc(100vw - 16px)}.document-slash{width:280px;max-height:min(340px,70vh);overflow-y:auto}.document-slash button{display:flex;align-items:center;justify-content:space-between;gap:12px;width:100%;padding:var(--option-pad-y) 10px;min-height:var(--control-height);border:0;background:transparent;border-radius:var(--radius-item);font-size:var(--control-font);text-align:left;color:var(--ink);line-height:var(--control-leading)}.document-slash button.selected{background:var(--accent-soft);color:var(--accent);line-height:var(--control-leading)}.document-slash small{font-size:.72rem;color:var(--muted)}.document-slash p{font-size:.8rem;padding:8px;margin:0}.document-menu-label,.document-menu-help{font-size:.68rem;color:var(--muted);padding:6px 10px}.document-menu-help{border-top:1px solid var(--line);margin-top:4px}.document-bubble{display:flex;gap:2px;padding:4px}
@media(max-width:720px){.document-toolbar{padding:7px;gap:4px}.document-tool-group+.document-tool-group{padding-left:4px}.document-history{margin-left:0}.document-toolbar button{min-width:34px;min-height:var(--control-height);padding:6px 8px;line-height:var(--control-leading)}.document-block-type select{min-height:40px;max-width:104px}.document-editor :deep(.document-prose){padding:24px 20px 60px;min-height:480px;font-size:18px}.document-editor :deep(.document-prose h1){font-size:1.7rem}.document-editor :deep(.document-prose h2){font-size:1.4rem}.document-editor :deep(.document-prose h3){font-size:1.15rem}.document-footnote{padding:10px 14px;font-size:.68rem;flex-wrap:wrap}.document-slash button{min-height:var(--control-height);line-height:var(--control-leading)}.document-bubble button{min-height:var(--control-height);line-height:var(--control-leading)}}
</style>
