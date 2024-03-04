<script setup lang="ts">
import { NSpace, NLayout, NLayoutHeader, NLayoutSider, NLayoutContent, NLayoutFooter, NConfigProvider, NBackTop, NIcon, NMenu, NDivider, NGradientText, NButton, NAvatar, NInput, type MenuOption, NMessageProvider } from 'naive-ui'
import { darkTheme } from 'naive-ui'
import { ref, type Component, h, onMounted } from 'vue';
import { RouterView, useRouter, useRoute } from 'vue-router'
import {
    HomeOutline as HomeIcon,
    ListCircleSharp as CategoryIcon,
    PricetagsOutline as TagIcon,
    Apps as AppsIcon,
} from '@vicons/ionicons5'
import { useUserStore } from '@/stores/counter';

const router = useRouter()
const route = useRoute()
const userStore = useUserStore()

onMounted(() => {
    setTimeout(() => {
        let path = route.path.substring(1, route.path.length)
        menuValue.value = path
    }, 150);
})

function renderIcon(icon: Component) {
    return () => h(NIcon, null, { default: () => h(icon) })
}

function goHome() {
    router.push("home")
}

function menuChange(key: string) {
    menuValue.value = key
    router.push("/" + key)
}

function userGo() {
    let flag = userStore.loginStatus
    if (flag) {
        router.push("/user")
    } else {
        router.push("/login")
    }
}

const menuValue = ref("")
const inverted = ref(false)
const menuOptions = ref([
    {
        label: '主页',
        key: 'home',
        icon: renderIcon(HomeIcon)
    },
    {
        label: '类别',
        key: 'category',
        icon: renderIcon(CategoryIcon),
    },
    {
        label: '标签',
        key: 'tag',
        icon: renderIcon(TagIcon),
    },
])
</script>

<template>
    <n-message-provider>
        <n-config-provider :theme="darkTheme">
            <n-space vertical size="large">
                <n-layout style="height: 100hv;">
                    <n-layout-header style="height: 86px;">
                        <div style="display: flex;">
                            <n-icon size="22" style="padding: 6px;">
                                <apps-icon />
                            </n-icon>
                            <n-divider vertical />
                            <n-gradient-text @click="goHome" :gradient="{
            from: 'rgb(185, 185, 185)',
            to: 'rgb(200, 200, 200)'
        }">
                                Painter-Blog
                            </n-gradient-text>
                        </div>
                        <div style="display: flex;">
                            <n-input placeholder="搜索" autosize clearable style="min-width: 100px;" />
                            <n-divider vertical />
                            <n-button quaternary>
                                友链
                            </n-button>
                            <n-button quaternary>
                                GitHub
                            </n-button>
                            <n-divider vertical />
                            <n-avatar @click="userGo" round size="small"
                                src="https://07akioni.oss-cn-beijing.aliyuncs.com/07akioni.jpeg"
                                fallback-src="https://07akioni.oss-cn-beijing.aliyuncs.com/07akioni.jpeg" />
                        </div>

                    </n-layout-header>
                    <n-layout has-sider>
                        <n-layout-sider bordered show-trigger collapse-mode="width" :collapsed-width="64" :width="240"
                            :native-scrollbar="false" :inverted="inverted">
                            <n-menu :inverted="inverted" :collapsed-width="64" :collapsed-icon-size="22"
                                :value="menuValue" :options="menuOptions" :on-update:value="menuChange" />
                        </n-layout-sider>
                        <n-layout-content :native-scrollbar="false">
                            <div class="view-container">
                                <RouterView />
                            </div>
                            <n-layout-footer position="absolute">
                                <div class="footer-info">@ 2019 - 2024 <a href="http://blog.callmetaota.fun/">逃塔小站</a>
                                </div>
                                <div class="footer-info">Power by Taota</div>
                            </n-layout-footer>
                        </n-layout-content>
                    </n-layout>
                </n-layout>
            </n-space>
            <n-back-top :right="100" />
        </n-config-provider>
    </n-message-provider>
</template>

<style scoped>
.n-layout-header {
    background: rgba(128, 128, 128, 0.2);
    padding: 24px;
}

.n-layout-footer {
    padding: 14px;
    display: flex;
    justify-content: space-between;
}

.n-layout-header {
    display: flex;
    justify-content: space-between;
}

.n-divider {
    height: auto;
}

.n-layout-sider {
    background: rgba(128, 128, 128, 0.3);
}

.n-layout-content {
    background: rgba(128, 128, 128, 0.4);
}

.n-gradient-text {
    font-size: 22px;
    line-height: 34px;
    user-select: none;
}

.view-container {
    height: calc(100vh - 146px);
    padding: 20px 20px 40px 20px;
    overflow: scroll;
}

.footer-info {
    user-select: none;
    line-height: 20px;
}
</style>