<script setup>
import { GetArticle, ArticleLike, ArticleLikeCheck } from '@/apis/api_article';
import { GetCategoryByID } from '@/apis/api_category';
import { CollectionCheck, Collection } from '@/apis/api_collection';
import { GetComments, CreateComment, DeleteComment, LikeComment, DisLikeComment, GetCommentsL } from '@/apis/api_comment';
import { ref, onMounted, computed } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { ArrowForward, ThumbsUpSharp, ChatboxEllipsesOutline, Star } from "@vicons/ionicons5"
import { NH1, NH5, NH6, NIcon, NTag, NDivider, NModal, NCard, NInput, NButton, NAvatar, NText } from 'naive-ui';
import { numTimeToString } from "@/utils/timeToStr"
import { useUserStore } from '@/stores/counter';

const Route = useRoute()
const Router = useRouter()
const UserStore = useUserStore()

const articleID = ref()
const fullArticle = ref({
    ArticleTable: {
        "Title": "",
    },
    ArticleContentTable: {
        "Content": "",
    },
    LikesNumber: 0
})
const categoryName = ref("")
const updateTime = ref("")
const commentBox = ref(false)
const commentInput = ref("")
const comments = ref([])

onMounted(async () => {
    articleID.value = Route.query.id
    getArticle()
})

async function likeArticle() {
    let f = fullArticle.value
    let res = await ArticleLikeCheck({ ArticleID: parseInt(articleID.value) })
    if (res.data.IsLiked) {
        f.LikesNumber -= 1
    } else {
        f.LikesNumber += 1
    }
    await ArticleLike({ ArticleID: parseInt(articleID.value) })
}

async function collectArticle() {
    let f = fullArticle.value
    let res = await CollectionCheck({ ArticleID: parseInt(articleID.value) })
    if (res.data.IsCollected) {
        f.CollectionNumber -= 1
    } else {
        f.CollectionNumber += 1
    }
    await Collection({ ArticleID: parseInt(articleID.value) })
}

async function getArticle() {
    let res = await GetArticle({ "ArticleID": articleID.value })
    fullArticle.value = res.data
    let categoryID = res.data.ArticleTable.CategoryID
    let cate_res = await GetCategoryByID({ "CategoryID": categoryID })
    categoryName.value = cate_res.data.CategoryName
    let time = new Date(res.data.ArticleTable.CreatedAt)
    updateTime.value = time.getFullYear() + "-" + time.getMonth() + "-" + time.getDate()
    console.log(res, cate_res, time)
}

async function openComment() {
    if (UserStore.loginStatus) {
        let res = await GetCommentsL({ "ArticleID": articleID.value, "Limit": 5 })
        commentBox.value = true
        comments.value = res.data.Comments
    } else {
        let res = await GetComments({ "ArticleID": articleID.value, "Limit": 5 })
        commentBox.value = true
        comments.value = res.data.Comments
    }
}

async function doComment() {
    let res = await CreateComment({ "ArticleID": parseInt(articleID.value), "Content": commentInput.value })
    commentInput.value = ""
    openComment()
}

async function doLikeComment(c) {
    if (c.Liked == true) {
        let res = await DisLikeComment({ "CommentID": parseInt(c.CommentID) })
    } else if (c.Liked == false) {
        let res = await LikeComment({ "CommentID": parseInt(c.CommentID) })
    } else {

    }
    openComment()
}

</script>

<template>
    <n-h1>{{ fullArticle.ArticleTable.Title }}</n-h1>
    <div>
        <span v-for="tag in fullArticle.ArticleTagTable">
            <n-tag type="info">
                {{ tag.TagName }}
            </n-tag>
            <span>&nbsp;</span>
            <span>&nbsp;</span>
        </span>
    </div>
    <br>
    <div class="article-flex">
        <n-h6 prefix="bar" align-text>
            类别：{{ categoryName }}
        </n-h6>
        <span class="article-card-numbers" @click="likeArticle">
            <n-icon size="16">
                <thumbs-up-sharp />
            </n-icon>
            {{ fullArticle.LikesNumber }}
        </span>
        <span class="article-card-numbers" @click="openComment">
            <n-icon size="16">
                <chatbox-ellipses-outline />
            </n-icon>
            {{ fullArticle.CommentNumber }}
        </span>
        <span class="article-card-numbers" @click="collectArticle">
            <n-icon size="16">
                <star />
            </n-icon>
            {{ fullArticle.CollectionNumber }}
        </span>
    </div>
    <div>更新于 {{ updateTime }}</div>
    <n-divider />
    <div>
        {{ fullArticle.ArticleContentTable.Content }}
    </div>
    <n-modal v-model:show="commentBox">
        <n-card style="width: 1000px; position: fixed; left: 50%;top: 50%;transform: translate(-50%,-50%);" title="评论"
            :bordered="false" size="huge" role="dialog" aria-modal="true">
            <div class="comment-input-area">
                <n-input type="textarea" placeholder="请输入评论" size="small" :autosize="{ minRows: 2 }" maxlength="100"
                    show-count v-model:value="commentInput"></n-input>
                <div style="width: 20px;"></div>
                <n-button @click="doComment">发布</n-button>
            </div>
            <n-divider />
            <div>
                <div v-for="c in comments" class="comment-cont">
                    <n-avatar round size="medium" :src="c.HeaderField"></n-avatar>
                    <div style="width: 10px;"></div>
                    <div>
                        <n-h5 style="margin-bottom: 2px;">{{ c.NickName }}</n-h5>
                        <n-text>
                            {{ c.Content }}
                        </n-text>
                        <br>
                        <n-text code>{{ numTimeToString(c.CreateTime) }}</n-text> &nbsp;
                        <span @click="doLikeComment(c)">
                            <n-icon size="16">
                                <thumbs-up-sharp />
                            </n-icon>
                            {{ c.LikeCount }}
                        </span>
                    </div>
                </div>
            </div>
        </n-card>
    </n-modal>
</template>

<style scoped>
.article-flex {
    display: flex;
    flex-direction: row;
    justify-content: space-between;
}

.article-card-numbers {
    width: 40px;
    line-height: 16px;
    font-size: 16px;
    display: flex;
    justify-content: space-around;
}

.comment-input-area {
    display: flex;
    flex-direction: row;
    align-items: center;
}

.comment-cont {
    display: flex;
    flex-direction: row;
    align-items: flex-start;
    margin-top: 0px;
    margin-bottom: 10px;
}
</style>