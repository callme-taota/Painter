<script setup lang="ts">
//base
import { ref, onMounted, computed } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { NIcon, NAvatar, NPagination, NInput, NButton, NText } from 'naive-ui';
import { marked } from 'marked';
import { useTitleStore } from '@/stores/title';
//icons
import { AccessTimeOutlined, LabelOutlined } from '@vicons/material';
import { FireOutlined, DeleteOutlined } from '@vicons/antd'
import { Comment48Regular, Star20Filled, Star20Regular } from '@vicons/fluent'
import { ThumbsUp, ThumbsUpFilled, Hashtag } from '@vicons/carbon';
//api
import { GetArticle, ArticleLike } from '@/apis/api_article';
import { Collection } from '@/apis/api_collection';
import { GetComments, CreateComment, DeleteComment, LikeComment, DisLikeComment } from '@/apis/api_comment';
//fn
import { dateToDescription } from "@/utils/timeToStr"
import type { FullArticleItem, CommentItem } from '@/utils/interface'

//store & route
const Route = useRoute()
const Router = useRouter()
const TitleStore = useTitleStore()

//refs
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
const renderedMarkdown = ref("")

const pageLimit = ref(10)
const pageNum = ref(1)
const listTotal = ref(0)
const pageTotal = computed<number>(() => {
    return Math.floor(listTotal.value / pageLimit.value) + 1
})

const commentInput = ref("")
const comments = ref<CommentItem[]>([])

const articleComment = ref()

//hook
onMounted(async () => {
    articleID.value = parseInt(Route.query.id as string)
    await getArticle()
    await getComment()
})

//function
const getArticle = async () => {
    let res = await GetArticle({ "ArticleID": articleID.value })
    fullArticle.value = res.data
    await renderMarkdown()
}

const getComment = async () => {
    let limit = pageLimit.value
    let offset = (pageNum.value - 1) * limit
    let res = await GetComments({ "ArticleID": articleID.value, "Limit": limit, "Offset": offset })
    comments.value = res.data.Comments
    listTotal.value = res.data.CommentCount
}

const getWithSizeChange = async (size: number) => {
    pageLimit.value = size
    await getComment()
}

const getWithNumChange = async (num: number) => {
    pageNum.value = num
    await getComment()
}

const likeArticle = async () => {
    let f = fullArticle.value
    if (f.Liked) {
        f.LikesNumber -= 1
    } else {
        f.LikesNumber += 1
    }
    f.Liked = !f.Liked
    await ArticleLike({ ArticleID: articleID.value })
}

const collectArticle = async () => {
    let f = fullArticle.value
    if (f.Collected) {
        f.CollectionNumber -= 1
    } else {
        f.CollectionNumber += 1
    }
    f.Collected = !f.Collected
    await Collection({ ArticleID: articleID.value })
}

const scrollToComment = () => {
    window.scrollTo(0, articleComment.value.offsetTop - 10)
}

const doLikeComment = async (c: CommentItem) => {
    if (c.Liked == true) {
        await DisLikeComment({ "CommentID": c.CommentID })
    } else if (c.Liked == false) {
        await LikeComment({ "CommentID": c.CommentID })
    }
    getComment()
}

const doComment = async () => {
    if (commentInput.value == "") return
    await CreateComment({ "ArticleID": articleID.value, "Content": commentInput.value })
    commentInput.value = ""
    getComment()
}

const delComment = async (c: CommentItem) => {
    if (!c.IsSelf) return
    await DeleteComment({ "CommentID": c.CommentID })
    getComment()
}

const renderMarkdown = async () => {
    const content = fullArticle.value.ArticleContentTable.Content;
    const markedContent = await marked(content);
    renderedMarkdown.value = markedContent;
}

const toCategoryPage = () => {
    let id = fullArticle.value.ArticleTable.CategoryID
    Router.push({ path: '/articleList', query: { type: 1, id: id } })
}

const toTagPage = (id: number) => {
    Router.push({ path: '/articleList', query: { type: 2, id: id } })
}

const toUserPage = (id: number) => {
    Router.push({ path: '/user', query: { id: id } })
}

</script>
<template>
    <div class="page-cont">
        <div class="article-title">{{ fullArticle.ArticleTable.Title }}</div>
        <div class="article-info-row">
            <div class="article-info-item article-info-user" @click="toUserPage(fullArticle.ArticleTable.Author)">
                <n-avatar round :size="20" :src="fullArticle.User.HeaderField"></n-avatar>
                <span>&nbsp;</span>
                {{ fullArticle.User.NickName }}
            </div>
            <div class="article-info-item">
                <n-icon size="16">
                    <AccessTimeOutlined></AccessTimeOutlined>
                </n-icon>
                <span>&nbsp;</span>
                {{ dateToDescription(fullArticle.ArticleTable.CreatedAt) }}
            </div>
            <div @click="toCategoryPage" class="article-info-item article-info-cursor">
                <n-icon size="16">
                    <LabelOutlined></LabelOutlined>
                </n-icon>
                <span>&nbsp;</span>
                {{ fullArticle.CategoryName }}
            </div>
            <div class="article-info-item">
                <n-icon size="16">
                    <FireOutlined></FireOutlined>
                </n-icon>
                <span>&nbsp;</span>
                {{ fullArticle.ArticleTable.ReadCount }}
            </div>
            <div class="article-info-item article-info-cursor" @click="scrollToComment">
                <n-icon size="16">
                    <Comment48Regular></Comment48Regular>
                </n-icon>
                <span>&nbsp;</span>
                {{ fullArticle.CommentNumber }}
            </div>
            <div class="article-info-item article-info-cursor" @click="likeArticle">
                <n-icon size="16">
                    <ThumbsUpFilled v-if="fullArticle.Liked"></ThumbsUpFilled>
                    <ThumbsUp v-else></ThumbsUp>
                </n-icon>
                <span>&nbsp;</span>
                {{ fullArticle.LikesNumber }}
            </div>
            <div class="article-info-item article-info-cursor" @click="collectArticle">
                <n-icon size="16">
                    <Star20Filled v-if="fullArticle.Collected"></Star20Filled>
                    <Star20Regular v-else></Star20Regular>
                </n-icon>
                <span>&nbsp;</span>
                {{ fullArticle.CollectionNumber }}
            </div>
        </div>
        <div class="article-info-row">
            <div class="article-tag-cont" v-for="t in fullArticle.ArticleTagTable" @click="toTagPage(t.TagID)">
                <n-icon>
                    <Hashtag />
                </n-icon>
                <span>&nbsp;</span>
                {{ t.TagName }}
            </div>
        </div>

        <div class="article-content" v-html="renderedMarkdown"></div>
        <div class="article-comment" ref="articleComment" v-if="comments.length != 0">
            <div class="article-comment-title">
                评论（{{ listTotal }}）
            </div>
            <div v-for="c in comments" class="comment-cont">
                <n-avatar style="margin-top:4px;" class="article-info-cursor" round size="medium" :src="c.HeaderField"
                    @click="toUserPage(c.UserID)"></n-avatar>
                <div style="width: 12px;"></div>
                <div>
                    <div>
                        <span class="article-comment-name article-info-cursor" @click="toUserPage(c.UserID)">
                            {{ c.NickName }}
                        </span>
                    </div>
                    <div class="article-comment-content">
                        {{ c.Content }}
                    </div>
                    <div style="display: flex;">
                        <n-text code>{{ dateToDescription(c.CreateTime) }}</n-text> &nbsp;
                        <div @click="doLikeComment(c)" class="article-info-cursor article-comment-like">
                            <n-icon size="14">
                                <ThumbsUpFilled v-if="c.Liked"></ThumbsUpFilled>
                                <ThumbsUp v-else></ThumbsUp>
                            </n-icon>
                            <div>
                                {{ c.LikeCount }}
                            </div>
                        </div>
                        <div style="width: 6px;"></div>
                        <div v-if="c.IsSelf" @click="delComment(c)" class="article-info-cursor article-comment-like">
                            <n-icon size="14">
                                <DeleteOutlined />
                            </n-icon>
                        </div>
                    </div>
                </div>
            </div>
            <div>
                <n-pagination v-model:page="pageNum" :page-count="pageTotal" v-model:page-size="pageLimit" show-size-picker
                    :page-sizes="[10, 20, 30, 40]" :on-update:page="getWithNumChange"
                    :on-update:page-size="getWithSizeChange" />
            </div>
            
        </div>

        <div class="article-comment-input">
            <div class="comment-input-area">
                <n-input type="textarea" placeholder="请输入评论" size="small" :autosize="{ minRows: 2 }" maxlength="100"
                    show-count v-model:value="commentInput"></n-input>
                <div style="width: 20px;"></div>
                <n-button @click="doComment" size="medium">发布</n-button>
            </div>
        </div>
    </div>
</template>
<style>
.article-title {
    font-weight: bolder;
    font-size: 50px;
    text-wrap: nowrap;
    text-overflow: ellipsis;
    overflow: hidden;
}

.article-info-row {
    display: flex;
    flex-direction: row;
    align-items: center;
    line-height: 34px;
    height: 34px;
}

.article-info-cursor {
    cursor: pointer;
}

.article-info-cursor:hover {
    color: var(--highlight-color);
}

.article-info-user {
    cursor: pointer;
    font-size: large;
    font-weight: bold;
}

.article-tag-cont {
    margin: 0 8px;
    font-size: normal;
    font-weight: light;
    display: flex;
    align-items: center;
    justify-content: center;
    padding-left: 8px;
    transition: 0.3s;
}

.article-tag-cont:hover {
    cursor: pointer;
    color: var(--highlight-color);
}

.article-content {
    background: var(--article--background);
    padding: 2px 20px;
    border-radius: 20px;
}

.article-comment {
    margin-top: 20px;
    background: var(--article--background);
    padding: 10px 20px 20px 30px;
    border-radius: 20px;
}

.article-comment-content {
    margin: 6px 0 10px 0;
}

.article-comment-title {
    font-weight: bold;
    font-size: large;
    margin-top: 10px;
}

.article-comment-name {
    font-weight: bold;
}

.article-comment-input {
    margin-top: 20px;
    background: var(--article--background);
    padding: 20px;
    border-radius: 20px;
}

.comment-cont {
    display: flex;
    flex-direction: row;
    align-items: flex-start;
    margin: 10px 0;
}

.article-comment-line {
    display: flex;
    justify-content: center;
    align-items: center;
}

.article-comment-like {
    width: 30px;
    display: flex;
    align-items: center;
    justify-content: center;
}

.comment-input-area {
    display: flex;
    flex-direction: row;
    align-items: center;
}
</style>