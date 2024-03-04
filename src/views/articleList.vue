<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { NCard, NButton, NIcon, NPagination, NTag } from 'naive-ui'
import { ArrowForward, ThumbsUpSharp, ChatboxEllipsesOutline, Star } from "@vicons/ionicons5"
import { useRouter, useRoute } from 'vue-router'
import { GetArticleByCategory, GetArticleByTag, GetArticleByAuthor, GetArticleByCollection } from '@/apis/api_article'

const Router = useRouter()
const Route = useRoute()

onMounted(async () => {
    searchType.value = Route.query.type
    searchTarget.value = Route.query.id
    getAritcleList()
})

async function getAritcleList() {
    let res;
    let type = searchType.value
    let limit = pageLimit.value
    let offset = (pageNum.value - 1) * limit
    if (type == 1) {
        res = await GetArticleByCategory({ "Limit": limit, "Offset": offset, "CategoryID": searchTarget.value })
    } else if (type == 2) {
        res = await GetArticleByTag({ "Limit": limit, "Offset": offset, "TagID": searchTarget.value })
    } else if (type == 3) {
        res = await GetArticleByAuthor({ "Limit": limit, "Offset": offset, "Author": searchTarget.value })
    } else {
        res = await GetArticleByCollection({ "Limit": limit, "Offset": offset, "UserID": searchTarget.value })
    }
    let list = res.data.ArticleList
    let len = res.data.ArticleCount

    articleList.value = list
    listTotal.value = len
}

async function getWithSizeChange(size: number) {
    pageLimit.value = size
    await getAritcleList()
}

async function getWithNumChange(num: number) {
    pageNum.value = num
    await getAritcleList()
}

const searchType = ref(1) // 1 -> category, 2 -> tag, 3 -> author
const searchTarget = ref(0)

const articleList = ref<ArticleInfoItem[]>([])
const pageLimit = ref(10)
const pageNum = ref(1)
const listTotal = ref(0)
const pageTotal = computed<number>(() => {
    return Math.floor(listTotal.value / pageLimit.value) + 1
})

function goPage(id: number) {
    Router.push({ path: "/article", query: { id: id } })
}

interface ArticleInfoItem {
    ArticleTable: ArticleItem,
    ArticleTagTable: ArticleTagItem[],
    LikesNumber: number,
    CollectionNumber: number,
    CommentNumber: number,
}

interface ArticleItem {
    ArticleID: number,
    Title: string,
    Author: number,
    Summary: string,
    ReadCount: number,
    IsTop: boolean,
    Status: number,//gorm:"comment:0 草稿，1 发布，2 隐藏，3 限制，4 封禁'"
    CategoryID: number,
    CreatedAt: string,
    UpdatedAt: string,
}

interface ArticleTagItem {
    TagID: number,
    TagName: string,
    Description: string,
}

</script>

<template>
    <n-card v-for="item in articleList" :title="item.ArticleTable.Title" size="medium">

        <template #header-extra>
            <n-button text style="font-size: 24px" @click="goPage(item.ArticleTable.ArticleID)">
                <n-icon>
                    <arrow-forward />
                </n-icon>
            </n-button>
        </template>
        {{ item.ArticleTable.Summary }}

        <template #footer>
            <span v-for="tag in item.ArticleTagTable">
                <n-tag type="info">
                    {{ tag.TagName }}
                </n-tag>
                <span>&nbsp;</span>
                <span>&nbsp;</span>
            </span>
        </template>

        <template #action>
            <div class="article-card-info">
                <span class="article-card-numbers">
                    <n-icon size="16">
                        <thumbs-up-sharp />
                    </n-icon>
                    {{ item.LikesNumber }}
                </span>
                <span class="article-card-numbers">
                    <n-icon size="16">
                        <chatbox-ellipses-outline />
                    </n-icon>
                    {{ item.CommentNumber }}
                </span>
                <span class="article-card-numbers">
                    <n-icon size="16">
                        <star />
                    </n-icon>
                    {{ item.CollectionNumber }}
                </span>
            </div>
        </template>

    </n-card>
    <n-pagination v-model:page="pageNum" :page-count="pageTotal" v-model:page-size="pageLimit" show-size-picker
        :page-sizes="[10, 20, 30, 40]" :on-update:page="getWithNumChange" :on-update:page-size="getWithSizeChange" />
</template>

<style>
.article-card-info {
    display: flex;
    justify-content: space-around;
    line-height: 16px;
}

.article-card-numbers {
    width: 40px;
    line-height: 16px;
    font-size: 16px;
    display: flex;
    justify-content: space-around;
}
</style>