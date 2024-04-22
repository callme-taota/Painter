<script setup>
import { NCard, NButton, NPagination } from 'naive-ui';
import { ref, onMounted, computed } from 'vue'
import { GetFollowers , GetFollowings } from '@/apis/api_follow'
import { useRoute, useRouter } from 'vue-router'

const Route = useRoute()
const Router = useRouter()

const followType = ref(1)

const followList = ref([])
const listLength = ref(0)
const pageNum = ref(1)
const pageLimit = ref(20)
const pageCount = computed<number>(() => {
    return Math.floor(listLength.value / pageLimit.value) + 1
})

onMounted(async () => {
    followType.value = Route.query.type
    getFollowList()
})

async function getFollowList() {
    let type = followType.value
    let limit = pageLimit.value
    let offset = (pageNum.value - 1) * limit
    let res
    if (type == 1) {
        res = await GetFollowers({ "Limit": limit, "Offset": offset})
    } else {
        res = await GetFollowings({ "Limit": limit, "Offset": offset})
    }
    let list = res.data.FollowList
    let len = res.data.Total
    followList.value = list
    listLength.value = len
}

</script>

<template>
    <n-card :title="f.NickName" size="small" v-for="f in followList">

        <template #header-extra>
            <n-button dashed>
                已关注
            </n-button>
        </template>
    </n-card>
    <n-pagination v-model:page="pageNum" :page-count="pageCount" v-model:page-size="pageLimit" show-size-picker
        :page-sizes="[10, 20, 30, 40]" :on-update:page="getCategoryWithNumChange" :on-update:page-size="getCategoryWithSizeChange" />
</template>

<style></style>