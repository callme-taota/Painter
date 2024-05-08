<script setup lang="ts">
//base
import { ref, onMounted, computed } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { NIcon, NAvatar, NPagination, NButton, NEmpty } from 'naive-ui';
//icons

//api
import { GetFollowers, GetFollowings } from '@/apis/api_follow'
import { CreateFollow, DeleteFollow } from '@/apis/api_follow';

//fn
import { dateDiff } from "@/utils/timeToStr"
import type { FollowUser } from '@/utils/interface';

//store & route
const Route = useRoute()
const Router = useRouter()

//refs
const userID = ref<number>(0)
const listType = ref<number>(1)

const userList = ref<FollowUser[]>([])
const listLength = ref<number>(0)
const pageNum = ref(1)
const pageLimit = ref(20)
const pageCount = computed<number>(() => {
    return Math.floor(listLength.value / pageLimit.value) + 1
})
//fn
onMounted(async () => {
    userID.value = parseInt(Route.query.id as string)
    listType.value = parseInt(Route.query.type as string)
    await getFollowList()
})

const getFollowList = async () => {
    let type = listType.value
    let limit = pageLimit.value
    let offset = (pageNum.value - 1) * limit
    let res
    if (type == 1) {
        res = await GetFollowings({ "Limit": limit, "Offset": offset })
    } else {
        res = await GetFollowers({ "Limit": limit, "Offset": offset })
    }
    let list = res.data.FollowList
    let len = res.data.Total
    userList.value = list
    listLength.value = len
}

async function getWithSizeChange(size: number) {
    pageLimit.value = size
    await getFollowList()
}

async function getWithNumChange(num: number) {
    pageNum.value = num
    await getFollowList()
}

const doFollow = async (flag: boolean) => {
    if (flag) {
        await DeleteFollow({ "FollowingID": userID.value })
    } else {
        await CreateFollow({ "FollowingID": userID.value })
    }
    await getFollowList()
}

const goUser = (id: number) => {
    Router.push({ path: "/user", query: { id: id } })
}

</script>
<template>
    <div class="page-cont">
        <div class="follow-title">
            <h1>
                {{ listType == 2 ? '粉丝' : '关注列表' }}
            </h1>
        </div>
        <div>
            <div class="follow-userlist-card" v-for="user in userList">
                <div class="follow-card-left">
                    <n-avatar style="cursor: pointer;" @click="goUser(user.ID)" round :size="36"
                        :src="user.HeaderField"></n-avatar>
                    <div style="width: 12px;"></div>
                    <div style="cursor: pointer;" @click="goUser(user.ID)">
                        <div>
                            {{ user.NickName }}
                        </div>
                        <div>
                            上次登录: {{ dateDiff(user.LastLogin) }}
                        </div>
                    </div>
                </div>
                <div class="follow-btn" :class="{ 'follow-btn-on': user.Following }" @click="doFollow(user.Following)">
                    {{ user.Following ? '已关注' : '关注' }}
                </div>
            </div>
        </div>
        <br />
        <n-pagination v-model:page="pageNum" :page-count="pageCount" v-model:page-size="pageLimit" show-size-picker
            :page-sizes="[10, 20]" :on-update:page="getWithNumChange" :on-update:page-size="getWithSizeChange" />

    </div>
</template>
<style>
.follow-userlist-card {
    margin: 6px 0;
    height: 36px;
    padding: 16px;
    display: flex;
    justify-content: space-between;
    align-items: center;
    background: var(--card--background);
    border-radius: 12px;
}

.follow-card-left {
    display: flex;
    align-items: center;
}

.follow-btn {
    width: 60px;
    height: 30px;
    border-radius: 8px;
    transition: 0.3s;
    text-align: center;
    font-weight: bold;
    line-height: 30px;
    user-select: none;
    color: var(--color-rev);
    background: var(--base-hover-background);
    border: 1px solid rgba(0, 0, 0, 0);
    cursor: pointer;
}

.follow-btn-on {
    border: 1px solid var(--border-color);
    color: var(--color);
    background: var(--layout--bottom-background);
}

.follow-btn:hover {
    background: var(--highlight-color);
}
</style>