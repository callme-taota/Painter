<script setup lang="ts">
//base
import { onMounted, onUnmounted, ref } from 'vue';
import { NIcon, NPopover } from 'naive-ui'
//api
import { GetEntryInfo } from '@/apis/api_common';
//icon
import { LogoGithub, LogoTwitter, LogoWechat, Mail } from '@vicons/ionicons5'
import { QuestionCircleOutlined } from '@vicons/antd';
//ref
const dateDiff = ref<string>("")
const yesterDayPV = ref<number>(0)
const monthPV = ref<number>(0)
const articleCount = ref<number>(0)
const pageHeight = ref<number>(window.innerHeight)
//hook
let timer: number | undefined;
onMounted(async () => {
    let res = await GetEntryInfo();
    if (res.ok) {
        let day = new Date(res.data.JSTimeStamp)
        let now = new Date()
        let diff = Math.abs(now.getTime() - day.getTime())
        const oneDay = 86400000; // 24 * 60 * 60 * 1000
        dateDiff.value = `${Math.floor(diff / oneDay)}天`;
        yesterDayPV.value = res.data.YesterdayCount
        monthPV.value = res.data.CurrentMonthCount
        articleCount.value = res.data.ArticleCount
    }
    timer = setInterval(autoChangeName, 300)
    document.addEventListener('scroll', atPage)
})

const imTag = ["Golang工程师", "云原生小白", "前端爱好者", "学生", "币圈玩家", "摄影佬", "EDM制作人"]

// 动态变化
const titleName = ref("")
const painterList = [
    "Painter",
    "Pai|nter",
    "Pai nter",
    "Pai|nter",
    "Pa|nter",
    "P|nter",
    "|nter",
    " nter",
    "E|nter",
    "E|nter",
    "Enter",
]
let currentIndex = 0
const autoChangeName = () => {
    if (currentIndex >= painterList.length - 1) {
        clearInterval(timer)
    }
    titleName.value = painterList[currentIndex]
    currentIndex++
}

const scrollTo = (pos: number) => {
    window.scrollTo({ left: 0, top: pos, behavior: "smooth" })
}

const scrollToPage = (page: number) => {
    let pos = (pageHeight.value - 60) * (page - 1)
    scrollTo(pos)
}

const atPage = () => {

}

const goLink = (link: string) => {
    window.open(link, "_blank")
}
</script>
<template>
    <div class="entry-cont">
        <div class="entry-page">
            <span class="entry-painter-text"
                @click="scrollToPage(2)">
                {{ titleName }}
            </span>
        </div>
        <div class="entry-page">
            Page2
        </div>
        <div class="entry-page">
            Page3
        </div>
    </div>
</template>
<style>
.entry-cont {
    min-height: calc(100vh - 80px);
    display: flex;
    flex-direction: column;
    padding: 0 80px 20px 80px;
    position: relative;
}

.entry-page{
    position: relative;
    height: calc(100vh - 60px);
    display: flex;
    align-items: center;
    justify-content: center;
}

.entry-painter-text {
    font-weight: 500;
    font-size: xx-large;
    padding: 0 18px;
    cursor: pointer;
    transition: 0.08s;
    text-align: center;
    transition: 0.3s;
}

.entry-painter-text:hover {
    text-shadow: 0 0 12px #fff;
    color: var(--color-rev);
    background-color: var(--base-hover-background);
}

</style>