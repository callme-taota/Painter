<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { NButton, NPagination, NResult } from 'naive-ui'
import { useRouter, useRoute } from 'vue-router'
import { GetArticleByCategory, GetArticleByTag, GetArticleByAuthor, GetArticleByCollection, GetArticleList } from '@/apis/api_article'
import { type ArticleInfoItem } from '@/utils/interface'
import ArticleCard from '@/components/article_card.vue'

const Route = useRoute()
const Router = useRouter()

onMounted(async () => {
    searchType.value = parseInt(Route.query.type as string)
    searchTarget.value = parseInt(Route.query.id as string)
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
    } else if (type == 4) {
        res = await GetArticleByCollection({ "Limit": limit, "Offset": offset, "UserID": searchTarget.value })
    } else {
        res = await GetArticleList({ "Limit": limit, "Offset": offset })
    }
    let list = res.data.ArticleList
    let len = res.data.ArticleCount
    if(len == 0) {
        list = []
    }
    isFirst.value = false
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

const searchType = ref(1) // 1 -> category, 2 -> tag, 3 -> author, 4-> user's collection, 5-> time(default)
const searchTarget = ref(0)

const articleList = ref<ArticleInfoItem[]>([])
const pageLimit = ref(10)
const pageNum = ref(1)
const listTotal = ref(0)
const pageTotal = computed<number>(() => {
    return Math.floor(listTotal.value / pageLimit.value) + 1
})
const isFirst = ref(true)

const goBack = () => {
    Router.go(-1)
}
</script>

<template>
    <div class="page-header">
        <h1>文章列表</h1>
    </div>
    <div class="article-list-cont" v-if="articleList.length > 0">
        <div class="article-list-flex">
            <ArticleCard v-for="item in articleList" :article="item"></ArticleCard>
        </div>
        <n-pagination style="float:right;" v-model:page="pageNum" :page-count="pageTotal" v-model:page-size="pageLimit"
            show-size-picker :page-sizes="[10, 20, 30, 40]" :on-update:page="getWithNumChange"
            :on-update:page-size="getWithSizeChange" />
    </div>
    <div class="article-list-cont" v-if="articleList.length == 0 && isFirst == false">
        <n-result status="404" title="好像是没东西的样子" description="换个地方吧～">
            <template #footer>
                <n-button @click="goBack">返回</n-button>
            </template>
        </n-result>
    </div>
</template>

<style>
.article-list-cont {
    padding: 10px 120px;
    transition: 0.3s;
}

@media (max-width: 1000px) {
    .article-list-cont {
        padding: 10px 80px;
    }
}


.article-list-flex {
    display: flex;
    flex-direction: row;
    justify-content: space-between;
    transition: 0.5s;
    flex-wrap: wrap;
}
</style>