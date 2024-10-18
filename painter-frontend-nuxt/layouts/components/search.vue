<script setup lang="ts">
//base
import { ref, onMounted } from 'vue'
import { NIcon, NInput, NAvatar, NIconWrapper, useMessage } from 'naive-ui';
import { storeToRefs } from 'pinia';
import { useRouter } from 'vue-router';
//icons
import { Search28Regular } from '@vicons/fluent'
import { UserOutlined } from '@vicons/antd';
//store
import { useSearchStore } from '@/stores/search'
import { useUserStore } from '@/stores/user';
//utils
import { copy } from '@/utils/copy';

//ref
const SearchStore = useSearchStore()
const UserStore = useUserStore()
const Router = useRouter()
const Message = useMessage()
const { shows } = storeToRefs(SearchStore)
const { loginStatus, userNickName, userName, userHeaderField } = storeToRefs(UserStore)

//fn
onMounted(() => {
    UserStore.getLocalLoginStatus()
})

const off = () => {
    SearchStore.offShow()
}

const copyUserName = () => {
    copy(userName.value)
    Message.info(`复制用户名: ${userName.value} 成功`)
}

const goLogin = () => {
    Router.push({ path: "/login" })
    off()
}

const goRegister = () => {
    Router.push({ path: "/register" })
    off()
}

const goDashboard = () => {
    Router.push({ path: "/dashboard" })
    off()
}

</script>
<template>
    <Transition>
        <div class="search-base" v-if="shows">
            <div class="search-cover-layer" @click="off">
            </div>
            <div class="search-input-cont">
                <n-input autosize class="search-input" placeholder="搜点啥" maxlength="30" clearable />
                <div style="width: 10px;"></div>
                <div class="search-icon-cont">
                    <n-icon size="22">
                        <Search28Regular />
                    </n-icon>
                </div>
            </div>
            <div class="search-user-card" v-if="loginStatus">
                <n-avatar :size="46" :src="userHeaderField" round class="search-user-head"></n-avatar>
                <div class="search-user-card-info">
                    <div class="search-user-card-nickname" @click="goDashboard">
                        {{ userNickName }}
                    </div>
                    <div class="search-user-card-name" @click="copyUserName">
                        {{ userName }}
                    </div>
                </div>
            </div>
            <div class="search-user-card search-user-card-nologin" v-else>
                <n-icon-wrapper :size="46" :border-radius="23" color="#67a7ec">
                    <n-icon size="24">
                        <UserOutlined />
                    </n-icon>
                </n-icon-wrapper>
                <div class="search-user-card-info-nologin">
                    <div class="search-user-card-nickname" @click="goLogin">
                        登录
                    </div>
                    <div class="search-user-card-name search-user-card-reg" @click="goRegister">
                        注册
                    </div>
                </div>
            </div>
        </div>
    </Transition>
</template>
<style>
.search-base {
    position: fixed;
    width: 100%;
    height: 100%;
    background-color: rgba(0, 0, 0, 0.1);
    backdrop-filter: blur(12px);
    z-index: 5;
    display: flex;
    justify-content: center;
    align-items: center;
    transition: 0.3s;
}

.search-hide {
    display: none;
}

.search-cover-layer {
    width: 100%;
    height: 100%;
    z-index: 4;
    position: absolute;
}

.search-input-cont {
    min-width: 300px;
    padding: 5px 10px 5px 15px;
    height: 40px;
    z-index: 5;
    background-color: var(--article--background);
    backdrop-filter: blur(10px);
    border-radius: 12px;
    display: flex;
    align-items: center;
}

.search-icon-cont {
    width: 30px;
    height: 30px;
    display: flex;
    justify-content: center;
    align-items: center;
    border-radius: 7px;
    transition: 0.3s;
    cursor: pointer;
}

.search-icon-cont:hover {
    background: var(--layout--bottom-background);
}

.search-input {
    min-width: 270px;
    background-color: rgba(0, 0, 0, 0.05);
    backdrop-filter: blur(12px);
    border-radius: 8px;
}

.search-user-card {
    background-color: var(--article--background);
    backdrop-filter: blur(10px);
    z-index: 5;
    position: absolute;
    bottom: 60px;
    height: 60px;
    border-radius: 30px;
    display: flex;
    align-items: center;
    padding: 0 16px 0 9px;
    transition: 0.3s;
}

.search-user-card:hover {
    box-shadow: 0 0 4px var(--border-color);
}

.search-user-card-nologin {
    padding: 0 22px 0 9px;
}

.search-user-head {
    cursor: pointer;
}

.search-user-card-info {
    margin-left: 6px;
}

.search-user-card-info-nologin {
    margin-left: 12px;
}

.search-user-card-nickname {
    font-size: large;
    font-weight: bold;
    transition: 0.3s;
    height: 29px;
    line-height: 29px;
}

.search-user-card-nickname:hover {
    color: var(--second-highlight-color);
    cursor: pointer;
}

.search-user-card-reg:hover {
    color: var(--highlight-color);
    cursor: pointer;
}

.search-user-card-name {
    font-size: small;
    font-weight: 300;
    transition: 0.3s;
    height: 21px;
    line-height: 21px;
}
</style>