<script setup lang="ts">
//base
import { ref, onMounted, computed } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { NIcon, NAvatar, NPagination, NButton, NEmpty } from 'naive-ui';
import ArticleCard from '@/components/article_card.vue'
//icons

//api
import { UserInfo } from "@/apis/api_user"
import { CreateFollow, DeleteFollow } from '@/apis/api_follow';
import { GetArticleByAuthor } from '@/apis/api_article';
//fn
import { dateDiff } from "@/utils/timeToStr"
import type { FullUserItem, ArticleInfoItem } from '@/utils/interface'
//store & route
const Route = useRoute()
const Router = useRouter()

//refs
const userID = ref<number>(0)
const userInfo = ref<FullUserItem>({
    ArticleNumber: 0,
    ArticleList: [],
    CollectionNumber: 0,
    FollowingNumber: 0,
    FollowerNumber: 0,
    UserInfo: {
        ID: 0,
        Email: '',
        NickName: '',
        HeaderField: '',
        CreatedAt: '',
        LastLogin: ''
    },
    Following: false,
    TotalCount: 0,
    Self: false
})

//fn
onMounted(async () => {
    userID.value = parseInt(Route.query.id as string)
    let res = await UserInfo({ ID: userID.value })
    userInfo.value = res.data
})

const doFollow = async () => {
    let flag = userInfo.value.Following
    if (flag) {
        await DeleteFollow({ "FollowingID": userID.value })
    } else {
        await CreateFollow({ "FollowingID": userID.value })
    }
    userInfo.value.Following = !userInfo.value.Following
}

const goArticleList = (id: number) => {
    Router.push({ path: "/articlelist", query: { type: 3, id: id } })
}

const goFollowing = (id: number) => {
    if (userInfo.value.Self) {
        Router.push({ path: "/follow", query: { type: 1, id: id } })
    }
}

const goFollower = (id: number) => {
    if (userInfo.value.Self) {
        Router.push({ path: "/follow", query: { type: 2, id: id } })
    }
}

const goCollections = (id: number) => {
    Router.push({ path: "/articlelist", query: { type: 4, id: id } })
}

</script>
<template>
    <div class="page-cont">
        <div class="user-info-cont">
            <div class="user-info-head-cont">
                <n-avatar round :size="80" :src="userInfo.UserInfo.HeaderField"></n-avatar>
                <div class="user-nick">
                    {{ userInfo.UserInfo.NickName }}
                </div>
            </div>
            <div>
                上次登录 {{ dateDiff(userInfo.UserInfo.LastLogin) }}
            </div>
            <div style="display: flex; justify-content: space-between; width: 80%;">
                <div class="user-info-item" :class="{ 'user-cursor': userInfo.Self }" @click="goFollowing(userID)">
                    <div class="user-info-item-number">
                        {{ userInfo.FollowingNumber }}
                    </div>
                    <div class="user-info-item-name">
                        关注
                    </div>
                </div>
                <div class="user-info-item" :class="{ 'user-cursor': userInfo.Self }" @click="goFollower(userID)">
                    <div class="user-info-item-number">
                        {{ userInfo.FollowerNumber }}
                    </div>
                    <div class="user-info-item-name">
                        粉丝
                    </div>
                </div>
                <div class="user-info-item user-cursor" @click="goCollections(userID)">
                    <div class="user-info-item-number">
                        {{ userInfo.CollectionNumber }}
                    </div>
                    <div class="user-info-item-name">
                        收藏
                    </div>
                </div>
                <div class="user-info-item user-cursor" @click="goArticleList(userID)">
                    <div class="user-info-item-number">
                        {{ userInfo.ArticleNumber }}
                    </div>
                    <div class="user-info-item-name">
                        文章
                    </div>
                </div>
            </div>
            <div class="user-follow-btn" :class="{ 'user-follow-btn-on': userInfo.Following }" @click="doFollow">
                {{ userInfo.Following ? '已关注' : '关注' }}
            </div>
        </div>
        <div class="user-article-list" :class="{ 'user-article-list-none': userInfo.ArticleList.length == 0 }">
            <div class="article-list-flex">
                <ArticleCard v-for="item in userInfo.ArticleList" :article="item"></ArticleCard>
            </div>
            <n-empty description="啥文章也没有诶" v-if="userInfo.ArticleList.length == 0">
                <template #extra>
                    <n-button>
                        看看别的
                    </n-button>
                </template>
            </n-empty>
        </div>
    </div>
</template>
<style></style>