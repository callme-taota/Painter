<script setup lang="ts">
//base
import { ref, onMounted, computed } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { NIcon, NAvatar, NPagination, NButton } from 'naive-ui';
//icons

//api
import { UserInfo } from "@/apis/api_user"
import { CreateFollow, DeleteFollow } from '@/apis/api_follow';
//fn
import { dateDiff } from "@/utils/timeToStr"
import type { FullUserItem } from '@/utils/interface'
//store & route
const Route = useRoute()
const Router = useRouter()

//refs
const userID = ref(0)
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
    Following: false
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
        await DeleteFollow({"FollowingID":userID.value})
    } else {
        await CreateFollow({"FollowingID":userID.value})
    }
    userInfo.value.Following = !userInfo.value.Following
}

</script>
<template>
    <div class="user-cont">
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
                <div class="user-info-item user-cursor">
                    <div class="user-info-item-number">
                        {{ userInfo.FollowingNumber }}
                    </div>
                    <div class="user-info-item-name">
                        关注
                    </div>
                </div>
                <div class="user-info-item user-cursor">
                    <div class="user-info-item-number">
                        {{ userInfo.FollowerNumber }}
                    </div>
                    <div class="user-info-item-name">
                        粉丝
                    </div>
                </div>
                <div class="user-info-item user-cursor">
                    <div class="user-info-item-number">
                        {{ userInfo.CollectionNumber }}
                    </div>
                    <div class="user-info-item-name">
                        收藏
                    </div>
                </div>
                <div class="user-info-item user-cursor">
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
    </div>
</template>
<style>
.user-cont {
    padding: 10px 120px;
}

.user-info-cont {
    padding: 30px;
    border-radius: 14px;
    background: var(--card--background);
    display: flex;
    flex-direction: column;
    justify-content: center;
    align-items: center;
    position: relative;
}

.user-info-head-cont {
    display: flex;
    flex-direction: column;
    justify-content: center;
    align-items: center;
}

.user-nick {
    font-size: x-large;
    font-weight: bolder;
    margin-top: -20px;
    z-index: 1;
    padding: 2px 50px;
    user-select: none;
    backdrop-filter: blur(2px);
}

.user-info-item {
    display: flex;
    flex-direction: column;
    justify-content: center;
    align-items: center;
    margin-top: 6px;
    user-select: none;
}

.user-info-item-number {
    font-size: large;
    font-weight: bolder;
}

.user-info-item-name {
    font-size: small;
    font-weight: 300;
}

.user-cursor {
    cursor: pointer;
    transition: 0.3s;
}

.user-cursor:hover {
    color: var(--highlight-color);
}

.user-follow-btn {
    position: absolute;
    right: 16px;
    top: 16px;
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

.user-follow-btn-on {
    border: 1px solid var(--border-color);
    color: var(--color);
    background: var(--layout--bottom-background);
}

.user-follow-btn:hover {
    background: var(--highlight-color);
}
</style>