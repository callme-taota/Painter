<script setup >
import { useUserStore } from "@/stores/counter"
import { ref, onMounted } from 'vue'
import { UserFull } from "@/apis/api_user"
import { useRouter, useRoute } from "vue-router"
import { NCard, NIcon, NPageHeader, NGrid, NGi, NStatistic, NAvatar, NButton } from "naive-ui"
import { ArrowForward } from "@vicons/ionicons5"

const UserStore = useUserStore()
const Route = useRoute()
const Router = useRouter()

const articleList = ref([])
const articleNumber = ref(0)
const collectionNumber = ref(0)
const followerNumber = ref(0)
const followingNumber = ref(0)
const userInfo = ref({
    "UserName": "",
    "NickName": "",
    "HeaderField": "",
})

onMounted(() => {
    getFullInfo()
})

async function getFullInfo() {
    let res = await UserFull()
    console.log(res)
    if (res.ok) {
        articleList.value = res.data.ArticleList
        articleNumber.value = res.data.ArticleNumber
        followerNumber.value = res.data.FollowerNumber
        followingNumber.value = res.data.FollowingNumber
        collectionNumber.value = res.data.CollectionNumber
        userInfo.value = res.data.UserInfo
    } else {
        UserStore.clear()
    }
}

function goPage(id) {
    Router.push({ path: "/article", query: { id: id } })
}

function toArticleList(id) {
    Router.push({ path: "/articlelist", query: { type: 3, id: id } })
}

function toFollowing(id) {
    Router.push({ path: "/follow", query: { type: 1, id: id } })
}

function toFollower(id) {
    Router.push({ path: "/follow", query: { type: 2, id: id } })
}

function toCollection(id) {
    Router.push({ path: "/articlelist", query: { type: 4,  id: id } })
}

function logout() {
    UserStore.logout()
    Router.push("/home")
}

function goSet() {
    Router.push("/admin")
}

</script>

<template>
    <n-page-header :subtitle="userInfo.UserName">
        <n-grid :cols="4">
            <n-gi>
                <n-statistic class="number-hover-point" @click="toArticleList(userInfo.ID)" label="文章数"
                    :value="articleNumber" />
            </n-gi>
            <n-gi>
                <n-statistic class="number-hover-point" @click="toFollowing(userInfo.ID)" label="关注者"
                    :value="followingNumber" />
            </n-gi>
            <n-gi>
                <n-statistic class="number-hover-point" @click="toFollower(userInfo.ID)" label="被关注者"
                    :value="followerNumber" />
            </n-gi>
            <n-gi>
                <n-statistic class="number-hover-point" @click="toCollection(userInfo.ID)" label="收藏数"
                    :value="collectionNumber" />
            </n-gi>
        </n-grid>

        <template #title>{{ userInfo.NickName }}
        </template>

        <template #avatar>
            <n-avatar :src="userInfo.HeaderField" />
        </template>
    </n-page-header>
    <br>
    <n-card v-for="item in articleList" :title="item.Title" size="medium">

        <template #header-extra>
            <n-button text style="font-size: 24px" @click="goPage(item.ArticleID)">
                <n-icon>
                    <arrow-forward />
                </n-icon>
            </n-button>
        </template>
        {{ item.Summary }}

    </n-card>
    <n-button @click="logout">
        登出
    </n-button>
    &nbsp;
    <n-button @click="goSet">
        管理
    </n-button>
</template>

<style>
.number-hover-point {
    cursor: pointer;
}
</style>