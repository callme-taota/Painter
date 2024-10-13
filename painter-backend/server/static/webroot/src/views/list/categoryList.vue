<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { GetCategoriesList } from "@/apis/api_category"
import { useRouter } from 'vue-router'
import type { CategoryListItem } from '@/utils/interface';

const Router = useRouter()

onMounted(async () => {
    getCategoryList()
})

const categroyList = ref<CategoryListItem[]>([])

async function getCategoryList() {
    let res = await GetCategoriesList({})
    let list = res.data.categories
    categroyList.value = list
}

function toPage(id: number) {
    Router.push({ path: '/articleList', query: { type: 1, id: id } })
}

</script>

<template>
    <div class="page-header">
        <h1>分类</h1>
    </div>
    <div class="categories-cont">
        <div class="category-item" v-for="c in categroyList" @click="toPage(c.CategoryID)">
            <div class="category-item-name">
                {{ c.CategoryName }}
            </div>
            <div class="category-item-count">
                {{ c.ArticleCount }}
            </div>
        </div>
    </div>
</template>

<style>
.categories-cont{
    padding: 20px 160px;
    display: flex;
    justify-content: center;
    flex-wrap: wrap;
}

.category-item{
    padding: 6px 10px 6px 30px;
    background: var(--card--background);
    height: 32px;
    line-height: 32px;
    margin: 10px;
    border-radius: 22px;
    display: flex;
    align-items: center;
    justify-content: space-between;
    cursor: pointer;
    transition: 0.3s;
    box-shadow: 0 2px 16px -3px rgba(0, 0, 0, .2);
}

.category-item:hover{
    transform: scale(1.1);
    color: var(--color-rev);
    font-weight: bold;
    background: var(--base-hover-background);
}

.category-item-name{
    padding-right: 10px;
}

.category-item-count{
    background: var(--btn-hover-grey);
    width: 32px;
    height: 32px;
    text-align: center;
    border-radius: 50%;
}
</style>