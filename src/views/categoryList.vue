<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { CategoryList } from "@/apis/api_category"
import { NCard, NButton, NIcon, NPagination } from 'naive-ui'
import { ArrowForward } from "@vicons/ionicons5"
import { useRouter } from 'vue-router'

const Router = useRouter()

onMounted(async () => {
    getCategoryList()
})

const categroyList = ref<CategoryListItem[]>([])
const listLength = ref(0)
const pageNum = ref(1)
const pageLimit = ref(20)
const pageCount = computed<number>(() => {
    return Math.floor(listLength.value / pageLimit.value) + 1
})

interface CategoryListItem {
    CategoryName: string,
    Description: string,
    CategoryID: number,
}

async function getCategoryList() {
    let limit = pageLimit.value
    let offset = (pageNum.value - 1) * limit
    let res = await CategoryList({ "Limit": limit, "Offset": offset })
    let list = res.data.categories
    let len = res.data.categoriesNumber
    categroyList.value = list
    listLength.value = len
}

async function getCategoryWithSizeChange(size: number) {
    pageLimit.value = size
    await getCategoryList()
}

async function getCategoryWithNumChange(num: number) {
    pageNum.value = num
    await getCategoryList()
}

function toPage(id: number) {
    Router.push({ path: '/articleList', query: { type: 1, id: id } })
}

</script>

<template>
    <n-card v-for="item in categroyList" :title="item.CategoryName" size="medium">
        <template #header-extra>
            <n-button text style="font-size: 24px" @click="toPage(item.CategoryID)">
                <n-icon>
                    <arrow-forward />
                </n-icon>
            </n-button>
        </template>
        {{ item.Description }}
    </n-card>
    <n-pagination v-model:page="pageNum" :page-count="pageCount" v-model:page-size="pageLimit" show-size-picker
        :page-sizes="[10, 20, 30, 40]" :on-update:page="getCategoryWithNumChange"
        :on-update:page-size="getCategoryWithSizeChange" />
</template>

<style>
</style>