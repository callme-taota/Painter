<script setup lang="ts">
//base
import { ref, onMounted, onUnmounted, Transition } from 'vue';
import { storeToRefs } from 'pinia'
import { NIcon, NAvatar } from 'naive-ui'
import { useRouter } from 'vue-router'
//icons
import { Search, LogoGithub, Sunny, Moon, SyncOutline } from '@vicons/ionicons5'
import { ArticleOutlined, AccountCircleOutlined } from '@vicons/material'
//store
import { useThemeStore } from '@/stores/theme'
import { useSearchStore } from '@/stores/search'
import { useInfoStore } from '@/stores/info';
import { useTitleStore } from '@/stores/title';
import { useUserStore } from '@/stores/user';
const SearchStore = useSearchStore()
const themeStore = useThemeStore()
const InfoStore = useInfoStore()
const TitleStore = useTitleStore()
const UserStore = useUserStore()
const Router = useRouter()
const { github_href, site_name } = storeToRefs(InfoStore)
const { secondaryTitle } = storeToRefs(TitleStore)
const { loginStatus, userHeaderField } = storeToRefs(UserStore)
//ref
const isScrolled = ref(false)
const themeChanger = ref(false)
//hook
onMounted(() => {
    themeStore.SetThemeAuto()
    window.addEventListener('scroll', handleScroll);
});

onUnmounted(() => {
    window.removeEventListener('scroll', handleScroll);
});

//fn
const handleScroll = () => {
    isScrolled.value = window.scrollY > 0;
};

const setTheme = (themeID: number) => {
    if (themeID == 1) {
        themeStore.SetThemeAuto()
    } else if (themeID == 2) {
        themeStore.SetThemeLight()
    } else {
        themeStore.SetThemeDark()
    }
    hideThemeChanger()
}

const showThemeChanger = () => {
    themeChanger.value = !themeChanger.value
}

const hideThemeChanger = () => {
    themeChanger.value = false
}

const goGithub = () => {
    window.open(github_href.value, "_blank");
}

const goHome = () => {
    Router.push("/")
}

const goArticleList = () => {
    Router.push("/articlelist")
}

const goCategories = () => {
    Router.push("/category")
}

const goTags = () => {
    Router.push("/tag")
}

const showSearch = () => {
    SearchStore.onShow()
}

</script>
<template>
    <Transition>
        <div v-if="themeStore.headerDisplay" class="layout-header-cont"
            :class="{ 'layout-header-cont-after-scroll': isScrolled }">
            <div class="layout-header-left">
                <img src="../../public/logo.png" @click="goHome" alt="Taota-Logo" class="layout-logo" >
                <Transition>
                    <div class="layout-header-painter">{{ site_name }}</div>
                </Transition>
            </div>
            <Transition>
                <div v-if="isScrolled" class="layout-header-center">
                    <!-- {{ secondaryTitle }} -->
                </div>
            </Transition>
            <div class="layout-header-right">
                <div class="layout-header-right-icon-cont" :class="{'layout-header-right-icon-hover':!loginStatus,'layout-header-right-icon-avatar':loginStatus}" @click="showSearch">
                    <n-icon size="20" v-if="!loginStatus">
                        <AccountCircleOutlined />
                    </n-icon>
                    <n-avatar :src="userHeaderField" v-else :size="30"></n-avatar>
                </div>
                <div class="layout-header-right-icon-cont layout-header-right-icon-hover" @click="showThemeChanger">
                    <n-icon size="20">
                        <sunny />
                    </n-icon>
                </div>
                <div class="layout-header-right-icon-cont layout-header-right-icon-hover" @click="goGithub">
                    <n-icon size="20">
                        <logo-github />
                    </n-icon>
                </div>

                <div class="layout-header-right-text-cont" @click="goTags">
                    标签
                </div>
                <div class="layout-header-right-text-cont" @click="goCategories">
                    分类
                </div>
                <div class="layout-header-right-text-cont" @click="goArticleList">
                    文章
                </div>
                <!-- <div class="layout-header-right-text-cont" @click="goHome">
                    主页
                </div> -->
                <div class="layout-header-right-icon-cont  layout-header-right-icon-hover latout-header-right-icon-afterhide" @click="goArticleList">
                    <n-icon size="20">
                        <ArticleOutlined />
                    </n-icon>
                </div>
            </div>
            <Transition name="fade-slide">
                <div class="layout-header-theme-changer-cont" v-show="themeChanger" @mouseleave="hideThemeChanger">
                    <div class="layout-header-theme-changer-item" @click="setTheme(2)">
                        <n-icon size="20">
                            <sunny />
                        </n-icon>
                        &nbsp;&nbsp;
                        浅色
                    </div>
                    <div class="layout-header-theme-changer-item" @click="setTheme(3)">
                        <n-icon size="20">
                            <moon />
                        </n-icon>
                        &nbsp;&nbsp;
                        深色
                    </div>
                    <div class="layout-header-theme-changer-item" @click="setTheme(1)">
                        <n-icon size="20">
                            <sync-outline />
                        </n-icon>
                        &nbsp;&nbsp;
                        跟随系统
                    </div>
                </div>
            </Transition>
        </div>
    </Transition>
</template>
<style>
.layout-header-cont {
    width: calc(100% - 100px);
    height: 60px;
    display: flex;
    position: fixed;
    padding: 0 50px;
    justify-content: space-between;
    transition: 0.5s;
    z-index: 2;
}

@media (max-width: 1000px) {
    .layout-header-cont {
        width: calc(100% - 40px);
        padding: 0 20px;
    }
}

@media (max-width: 500px) {
    .layout-header-painter {
        display: none;
    }
}

.layout-header-cont-after-scroll {
    background-color: var(--header-background);
    backdrop-filter: blur(12px);
}

.layout-logo {
    height: 40px;
    border-radius: 14px;
    transition: 0.3s;
    cursor: pointer;
}

.layout-header-left {
    display: flex;
    padding: 0 10px;
    align-items: center;
}

.layout-header-painter {
    margin-left: 10px;
    line-height: 40px;
    height: 40px;
    border-radius: 14px;
    transition: 0.3s;
    padding: 0 10px;
    user-select: none;
    font-weight: bolder;
    font-size: large
}

.layout-header-painter:hover {
    background-color: var(--base-hover-background);
    color: var(--btn-hover-color);
}

.layout-header-center {
    display: flex;
    align-items: center;
    font-size: large;
    font-weight: bold;
}

.layout-header-right {
    display: flex;
    align-items: center;
    flex-direction: row-reverse;
}

.layout-header-right-icon-cont {
    transition: 0.3s;
    margin: 0 4px;
    border-radius: 14px;
    height: 40px;
    width: 40px;
    display: flex;
    align-items: center;
    cursor: pointer;
    justify-content: center;
}

.layout-header-right-icon-hover:hover {
    background-color: var(--base-hover-background);
    color: var(--btn-hover-color);
}

.layout-header-right-icon-avatar:hover{
    transform: scale(1.2);
}

.layout-header-right-text-cont {
    margin: 0 4px;
    transition: 0.3s;
    border-radius: 14px;
    height: 40px;
    padding: 0 12px;
    display: flex;
    align-items: center;
    justify-content: center;
    font-weight: bold;
    user-select: none;
    cursor: pointer;
}

.layout-header-right-text-cont:hover {
    background-color: var(--base-hover-background);
    color: var(--btn-hover-color);
}

.layout-header-theme-changer-cont {
    position: fixed;
    background-color: var(--header-background);
    backdrop-filter: blur(12px);
    border-radius: 16px;
    border: 1px solid var(--border-color);
    width: 100px;
    height: 110px;
    top: 60px;
    right: 62px;
    padding: 2px 6px;
    transition: 0.5s;
}

.layout-header-theme-changer-item {
    height: 30px;
    line-height: 30px;
    display: flex;
    margin: 5px 0;
    justify-content: start;
    align-items: center;
    user-select: none;
    padding-left: 6px;
    border-radius: 8px;
    cursor: pointer;
}

.layout-header-theme-changer-item:hover {
    background-color: var(--btn-hover-grey);
}

.latout-header-right-icon-afterhide {
    display: none;
}

@media (max-width: 1200px) {
    .layout-header-right-text-cont {
        display: none;
    }

    .latout-header-right-icon-afterhide {
        display: flex;
    }
}
</style>