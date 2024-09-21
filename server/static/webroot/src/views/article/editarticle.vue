<script setup lang="ts">
//base
import { ref, onMounted, computed, onUnmounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { marked, Marked } from 'marked';
import { markedHighlight } from "marked-highlight";
import hljs from "highlight.js";
import 'highlight.js/styles/github.css'
import { NIcon, NInput, NSelect, NButton, useMessage } from 'naive-ui';
import RequiredStar from '@/components/required_star.vue'
//apis
import { GetCategoriesList } from '@/apis/api_category';
import { TagListFull } from '@/apis/api_tag';
import { GetArticle, ArticleUpdateContent, ArticleUpdateSummary, ArticleUpdateTitle, UpdateArticleCategory, ArticleTagUpdate, CreateArticle, ArticleUpdateStatusPublic, ArticleUpdateStatusDart, ArticleUpdate } from '@/apis/api_article';
//icons
import { CircleEdit20Regular, ProtocolHandler16Regular, PreviewLink20Regular, TextBold16Regular, TextStrikethrough16Regular, AppsList20Regular, TextNumberListLtr20Regular, Comment48Regular, Star20Regular } from '@vicons/fluent';
import { FormatItalicSharp } from '@vicons/material';
import { H1 as h1icon, H2 as h2icon, H3 as h3icon, H4 as h4icon } from '@vicons/tabler'
import { FireOutlined } from '@vicons/antd';
import { ThumbsUp } from '@vicons/carbon';
//utils
import type { FullArticleItem, ArticleTagItem, CategoryListItem } from '@/utils/interface';
import { dateToString, dateNow } from '@/utils/timeToStr';
import axios from 'axios';
//store
const Route = useRoute()
const Router = useRouter()
const Message = useMessage()
//refs
const editType = ref<number>() // edit -> 1, add -> 2
const articleID = ref<number>()
const fullArticle = ref<FullArticleItem>({
    ArticleTable: {
        ArticleID: 0,
        Title: "",
        Author: 0,
        Summary: "",
        ReadCount: 0,
        IsTop: false,
        Status: 0, //gorm:"comment:0 草稿，1 发布，2 隐藏，3 限制，4 封禁'"
        CategoryID: 0,
        CreatedAt: "",
        UpdatedAt: "",
    },
    ArticleTagTable: [],
    ArticleContentTable: {
        ArticleID: 0,
        Content: ''
    },
    LikesNumber: 0,
    CollectionNumber: 0,
    CommentNumber: 0,
    CategoryName: '',
    Liked: false,
    Collected: false,
    User: {
        ID: 0,
        Email: "",
        NickName: "",
        HeaderField: "",
        CreatedAt: "",
        LastLogin: ''
    }
})
const tagList = ref<ArticleTagItem[]>([])
const tagSelect = computed<number[]>({
    get() {
        let tagIDList = fullArticle.value.ArticleTagTable.map(obj => obj.TagID)
        return tagIDList
    },
    set(newValue) {
        let list = tagList.value
        let objList = newValue.map(id => list.find(obj => obj.TagID == id)).filter(obj => obj !== undefined) as { TagID: number; TagName: string; Description: string; }[]
        fullArticle.value.ArticleTagTable = objList
    }
})
const categoryList = ref<CategoryListItem[]>([])
const markdownEditType = ref<number>(2) // 1-> only edit, 2-> both, 3-> only preview
const renderedMarkdown = ref("")
const statusOptions = ref([
    {
        label: "草稿",
        value: 0
    },
    {
        label: "发布",
        value: 1
    },
])
//hook
onMounted(async () => {
    articleID.value = parseInt(Route.query.id as string)
    editType.value = parseInt(Route.query.type as string)
    await getSelections()
    if (editType.value == 1) {
        await getArticle()
    }
    document.addEventListener('keydown', handleCtrlS);
    document.addEventListener('paste', handlePaste)
    document.getElementById('editor')?.addEventListener('keydown', handleTab);
})

onUnmounted(() => {
    document.removeEventListener('keydown', handleCtrlS);
    document.removeEventListener('paste', handlePaste)
    document.getElementById('editor')?.removeEventListener('keydown', handleTab);
    // editorRef.value?.removeEventListener('keydown', handleTab);
});
//fn
const handleCtrlS = (event: KeyboardEvent) => {
    const isMac = navigator.platform.toUpperCase().indexOf('MAC') >= 0;
    if (isMac) {
        if (event.metaKey && event.key === 's') {
            event.preventDefault();
            doPost(0);
        }
    } else {
        if (event.ctrlKey && event.key === 's') {
            event.preventDefault();
            doPost(0);
        }
    }
}

const handlePaste = async (event: ClipboardEvent) => {
    if (event.clipboardData !== null && event.clipboardData.items) {
        const items = event.clipboardData.items;
        for (let i = 0; i < items.length; i++) {
            if (items[i].type.indexOf('image') !== -1) {
                const file = items[i].getAsFile();
                if (file) {
                    Message.info("上传图片ing～")
                    uploadImage(file);
                }
                break;
            }
        }
    }
}

async function handleTab(event: KeyboardEvent) {
    if (event.key === 'Tab') {
        event.preventDefault();
        const editor = document.getElementById('editor')?.querySelector('textarea') as HTMLTextAreaElement;
        if (editor) {
            const start = editor.selectionStart;
            const end = editor.selectionEnd;

            let content = editor.value.substring(0, start) + '\t' + editor.value.substring(end);
            editor.value = content
            handleContentChange(content)
            editor.selectionStart = editor.selectionEnd = start + 1;

        }
    }
}

const uploadImage = async (file: File) => {
    const formData = new FormData();
    formData.append("file", file);
    axios.post(import.meta.env.VITE_BASE_API_URL + "/api/file/upload", formData, {
        headers: {
            'Content-Type': 'multipart/form-data'
        },
        withCredentials: true
    }).then(async (response) => {
        if (response.data.ok) {
            let img_url = import.meta.env.VITE_BASE_API_URL + response.data.data.filePath + response.data.data.fileName
            Message.success("上传成功! ")
            addSomeContent(`![image](${img_url})`)
        }
    }).catch((error) => {
        Message.error("上传失败了")
    });
}

const getSelections = async () => {
    let tag_res = await TagListFull({})
    let category_res = await GetCategoriesList({})
    tagList.value = tag_res.data.Tags
    categoryList.value = category_res.data.categories
}

const getArticle = async () => {
    let res = await GetArticle({ "ArticleID": articleID.value })
    fullArticle.value = res.data
    await renderMarkdown()
}

const renderMarkdown = async () => {
    const content = fullArticle.value.ArticleContentTable.Content;
    const renderer = new marked.Renderer();

    let linkRenderer = renderer.link;
    renderer.link = (href, title, text) => {
        const html = linkRenderer.call(renderer, href, title, text)
        return html.replace(/^<a /, '<a target="_blank" ')
    }

    renderer.image = (href, title, text) => {
        return `
            <div class="md-image-container">
                <img class="md-image" src="${href}" alt="${text}" ${title ? `title="${title}"` : ''} />
            </div>
        `;
    };

    const markedInstance = new Marked(
        markedHighlight({
            langPrefix: 'hljs language-',
            highlight(code: string, lang: string, info: any) {
                const language = hljs.getLanguage(lang) ? lang : 'plaintext';
                return hljs.highlight(code, { language }).value;
            }
        })
    )
    markedInstance.use({ renderer });
    renderedMarkdown.value = await markedInstance.parse(content);
}

const handleContentChange = async (content: string) => {
    fullArticle.value.ArticleContentTable.Content = content
    await renderMarkdown()
}

const setMarkdownEditType = (type: number) => {
    markdownEditType.value = type
}

const addSomeContent = (add: string) => {
    const editor = document.getElementById('editor')?.querySelector('textarea') as HTMLTextAreaElement;
    if (editor) {
        const start = editor.selectionStart;
        const end = editor.selectionEnd;

        // Insert the tab character at the current cursor position
        let content = editor.value.substring(0, start) + '\n' + add + editor.value.substring(end);
        editor.value = content
        handleContentChange(content)

        // Move the cursor to the right position after inserting the tab character
        editor.selectionStart = editor.selectionEnd = start + 1;
    }
}

const doUpdate = async (): Promise<boolean> => {
    let id = articleID.value
    await ArticleUpdate({
        "ArticleID": id,
        "Title": fullArticle.value.ArticleTable.Title,
        "Summary": fullArticle.value.ArticleTable.Summary,
        "CategoryID": fullArticle.value.ArticleTable.CategoryID,
        "TagList": tagSelect.value,
        "Content": fullArticle.value.ArticleContentTable.Content,
    })
    if (fullArticle.value.ArticleTable.Status == 0) {
        await ArticleUpdateStatusDart({ "ArticleID": id })
    } else {
        await ArticleUpdateStatusPublic({ "ArticleID": id })
    }
    Message.info("修改成功")
    return true
}

const doCreate = async (): Promise<boolean> => {
    if (fullArticle.value.ArticleTable.Title == "" || fullArticle.value.ArticleContentTable.Content == "" || fullArticle.value.ArticleTable.Summary == "" || fullArticle.value.ArticleTable.CategoryID <= 0) {
        Message.warning("检查一下少了啥")
        return false
    }
    let res = await CreateArticle({
        "Title": fullArticle.value.ArticleTable.Title,
        "Content": fullArticle.value.ArticleContentTable.Content,
        "Summary": fullArticle.value.ArticleTable.Summary,
        "CategoryID": fullArticle.value.ArticleTable.CategoryID,
        "Tags": tagSelect.value
    })
    if (res.ok) {
        Message.info("新增成功")
        return true
    }
    return false
}

const doPost = async (type: number) => {
    let flag = false
    if (editType.value == 1) {
        flag = await doUpdate()
    } else if (editType.value == 2) {
        flag = await doCreate()
    }
    if (type == 1 && flag) {
        Router.push({ path: "/dashboard" })
    }
}

</script>
<template>
    <div class="page-cont">
        <h1>
            {{ editType == 1 ? '编辑文章' : '新增文章' }}
        </h1>
        <div class="editpage-cont">
            <div class="editpage-left">
                <div class="edit-card editpage-title-edit">
                    <div style="width: 54px; line-height: 1.5; user-select: none;">
                        标题
                        <RequiredStar />
                    </div>
                    <n-input placeholder="请输入标题" maxlength="30"
                        v-model:value="fullArticle.ArticleTable.Title"></n-input>
                </div>
                <div class="edit-card">
                    <div class="edit-card-title">
                        简介
                        <RequiredStar />
                    </div>
                    <n-input type="textarea" :autosize="{ minRows: 2 }" placeholder="请输入简介"
                        v-model:value="fullArticle.ArticleTable.Summary"></n-input>
                </div>
                <div class="edit-card">
                    <div class="edit-content-bar">
                        <span style="display: flex; margin-right: 6px;">
                            <div class="edit-bar-item" @click="addSomeContent('****')">
                                <n-icon size="24">
                                    <TextBold16Regular />
                                </n-icon>
                            </div>
                            <div class="edit-bar-item" @click="addSomeContent('~~~~')">
                                <n-icon size="24">
                                    <TextStrikethrough16Regular />
                                </n-icon>
                            </div>
                            <div class="edit-bar-item" @click="addSomeContent('**')">
                                <n-icon size="24">
                                    <FormatItalicSharp />
                                </n-icon>
                            </div>
                        </span>
                        <span style="display: flex; margin-right: 6px;">
                            <div class="edit-bar-item" @click="addSomeContent('# ')">
                                <n-icon size="24">
                                    <h1icon />
                                </n-icon>
                            </div>
                            <div class="edit-bar-item" @click="addSomeContent('## ')">
                                <n-icon size="24">
                                    <h2icon />
                                </n-icon>
                            </div>
                            <div class="edit-bar-item" @click="addSomeContent('### ')">
                                <n-icon size="24">
                                    <h3icon />
                                </n-icon>
                            </div>
                            <div class="edit-bar-item" @click="addSomeContent('#### ')">
                                <n-icon size="24">
                                    <h4icon />
                                </n-icon>
                            </div>
                        </span>
                        <span style="display: flex; margin-right: 6px;">
                            <div class="edit-bar-item" @click="addSomeContent('- ')">
                                <n-icon size="24">
                                    <AppsList20Regular />
                                </n-icon>
                            </div>
                            <div class="edit-bar-item" @click="addSomeContent('1. ')">
                                <n-icon size="24">
                                    <TextNumberListLtr20Regular />
                                </n-icon>
                            </div>
                        </span>
                        <span style="display: flex; margin-right: 6px;">
                            <div class="edit-bar-item" @click="setMarkdownEditType(1)">
                                <n-icon size="24">
                                    <CircleEdit20Regular />
                                </n-icon>
                            </div>
                            <div class="edit-bar-item" @click="setMarkdownEditType(2)">
                                <n-icon size="24">
                                    <ProtocolHandler16Regular />
                                </n-icon>
                            </div>
                            <div class="edit-bar-item" @click="setMarkdownEditType(3)">
                                <n-icon size="24">
                                    <PreviewLink20Regular />
                                </n-icon>
                            </div>
                        </span>
                    </div>
                    <div class="edit-content-main">
                        <div class="edit-content-text" :class="{ 'edit-content-flex': markdownEditType == 1 }"
                            v-if="markdownEditType == 1 || markdownEditType == 2">
                            <n-input type="textarea" :autosize="{ minRows: 10 }" placeholder="请输入正文内容"
                                v-model:value="fullArticle.ArticleContentTable.Content"
                                :on-update:value="handleContentChange" id="editor"></n-input>
                        </div>
                        <div style="width: 10px;" v-if="markdownEditType == 2"></div>
                        <div class="edit-content-preview" :class="{ 'edit-content-flex': markdownEditType == 3 }"
                            v-if="markdownEditType == 3 || markdownEditType == 2">
                            <div v-html="renderedMarkdown"></div>
                        </div>
                    </div>
                </div>
            </div>
            <div class="editpage-right">
                <div class="edit-card">
                    <div class="edit-card-title">
                        分类
                        <RequiredStar />
                    </div>
                    <n-select label-field="CategoryName" value-field="CategoryID" :options="categoryList"
                        v-model:value="fullArticle.ArticleTable.CategoryID" />
                </div>
                <div class="edit-card">
                    <div class="edit-card-title">
                        标签
                    </div>
                    <n-select label-field="TagName" value-field="TagID" multiple :options="tagList"
                        v-model:value="tagSelect" />
                </div>
                <div class="edit-card" v-if="editType == 1">
                    <div class="edit-card-title">
                        信息
                    </div>
                    <div style="display: flex; line-height: 34px; flex-wrap: wrap; justify-content: space-between;">
                        <div class="article-info-item">
                            <n-icon size="16">
                                <FireOutlined></FireOutlined>
                            </n-icon>
                            <span>&nbsp;</span>
                            {{ fullArticle.ArticleTable.ReadCount }}
                        </div>
                        <div class="article-info-item">
                            <n-icon size="16">
                                <Comment48Regular></Comment48Regular>
                            </n-icon>
                            <span>&nbsp;</span>
                            {{ fullArticle.CommentNumber }}
                        </div>
                        <div class="article-info-item">
                            <n-icon size="16">
                                <ThumbsUp></ThumbsUp>
                            </n-icon>
                            <span>&nbsp;</span>
                            {{ fullArticle.LikesNumber }}
                        </div>
                        <div class="article-info-item">
                            <n-icon size="16">
                                <Star20Regular></Star20Regular>
                            </n-icon>
                            <span>&nbsp;</span>
                            {{ fullArticle.CollectionNumber }}
                        </div>
                    </div>
                </div>
                <div class="edit-card">
                    <div class="edit-card-title">
                        发布
                    </div>
                    <div>
                        · 创建日期： {{ editType == 1 ? dateToString(fullArticle.ArticleTable.CreatedAt) : dateNow() }}
                    </div>
                    <div v-if="editType == 1">
                        · 上次更新： {{ dateToString(fullArticle.ArticleTable.UpdatedAt) }}
                    </div>
                    <div>
                        <n-select :options="statusOptions" v-model:value="fullArticle.ArticleTable.Status"></n-select>
                    </div>
                    <div style="height: 10px;"></div>
                    <n-button @click="doPost(1)">
                        {{ editType == 1 ? '更新' : '新增' }}
                    </n-button>
                </div>
            </div>
        </div>
    </div>
</template>
<style>
.edit-card {
    border-radius: 14px;
    background: var(--card--background);
    padding: 20px;
    margin-bottom: 20px;
    transition: 0.3s;
}

.edit-card:hover {
    box-shadow: 0 0 8px var(--base-hover-background);
}

.edit-card-title {
    user-select: none;
    margin-bottom: 8px;
}

.editpage-title-edit {
    display: flex;
    align-items: center;
}

.editpage-cont {
    display: flex;
}

.editpage-left {
    flex: 1;
}

.edit-content-bar {
    display: flex;
    margin-bottom: 6px;
}

.edit-bar-item {
    border-radius: 6px;
    margin: 0 3px;
    border: var(--content-border);
    display: flex;
    justify-content: center;
    align-items: center;
    cursor: pointer;
    padding: 2px;
    transition: 0.3s;
}

.edit-bar-item:hover {
    color: var(--highlight-color);
    border: 1px solid var(--highlight-color);
}

.edit-content-main {
    display: flex;
}

.edit-content-text {
    width: 49%;
}

.edit-content-flex {
    flex: 1;
}

.edit-content-preview {
    width: 49%;
    padding: 10px;
    border: var(--content-border);
    border-radius: 12px;
    transition: 0.3s;
}

.edit-content-preview:hover {
    border: 1px solid var(--border-color);
}

.editpage-right {
    margin-left: 20px;
    width: 260px;
}

@media (max-width: 900px) {
    .editpage-cont {
        display: block;
    }

    .editpage-right {
        width: auto;
        flex: 1;
        margin-left: 0px;
    }
}
</style>