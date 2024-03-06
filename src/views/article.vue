<script setup>
import { GetArticle, ArticleLike, ArticleLikeCheck } from '@/apis/api_article';
import { GetCategoryByID } from '@/apis/api_category';
import { CollectionCheck, Collection } from '@/apis/api_collection';
import { ref, onMounted, computed } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { ArrowForward, ThumbsUpSharp, ChatboxEllipsesOutline, Star } from "@vicons/ionicons5"
import { NH1, NH6, NIcon, NTag, NDivider } from 'naive-ui';

const Route = useRoute()
const Router = useRouter()

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
        <span class="article-card-numbers">
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
    <div>

    </div>
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
</style>