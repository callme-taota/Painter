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
    document.addEventListener('scroll', changeTitleStyle)
    document.addEventListener('scroll', changePage2Opacity)
    document.addEventListener('scroll', changePage3Opacity)
    timer = setInterval(autoChangeName, 300)
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

// 视差计算 Page1
const titleOpacity = ref(1)
const titleLetterSpacing = ref(2)
const changeTitleStyle = (e: any) => {
    let formTop = e.target.scrollingElement.scrollTop
    if (formTop <= 220) {
        titleOpacity.value = 1 - (formTop / 200)
        titleLetterSpacing.value = 4 + 0.2 * formTop
    }
}

// 视差计算 Page2
const page2Opacity = ref(0)
const page2Offset = ref(0)
const changePage2Opacity = (e: any) => {
    let scrollTop = e.target.scrollingElement.scrollTop
    let height = pageHeight.value
    let page2Start = height - 70
    let page2End = height + 80

    if (scrollTop >= page2Start && scrollTop < page2End) {
        page2Opacity.value = 1
        page2Offset.value = (scrollTop - page2Start)
    } else {
        page2Opacity.value = 0
    }
}

// 视差计算 Page3
const page3Opacity = ref(0)
const page3Offset = ref(0)
const changePage3Opacity = (e: any) => {
    let scrollTop = e.target.scrollingElement.scrollTop
    let height = pageHeight.value
    let page3Start = height + 120
    let page3End = height + 270

    console.log(scrollTop, page3Start, page3End)
    if (scrollTop >= page3Start && scrollTop < page3End) {
        page3Opacity.value = 1
        page3Offset.value = (scrollTop - page3Start)
    } else {
        page3Opacity.value = 0
    }
}


const scrollToPage = (page: number) => {
    let pos = (pageHeight.value - 60) * (page - 1)
    scrollTo(pos)
}

const goLink = (link: string) => {
    window.open(link, "_blank")
}
</script>
<template>
    <div class="entry-cont">

        <!-- page1 -->
        <div class="entry-painter">
            <span class="entry-painter-text"
                :style="{ 'letter-spacing': titleLetterSpacing + 'px', 'opacity': titleOpacity }"
                @click="scrollToPage(2)">
                {{ titleName }}
            </span>
        </div>

        <!-- page2 -->
        <div class="entry-page-2"
            :style="{ 'opacity': page2Opacity, 'transform': 'translateY(' + page2Offset + 'px)' }">
            <span class="second-highlight">
                你好,
            </span>
            &nbsp;欢迎来到
            <span>
                Painter
                👋
            </span>
            <span style="vertical-align: super; margin: 0 5px;">
                <n-popover>
                    <template #trigger>
                        <n-icon size="18">
                            <QuestionCircleOutlined />
                        </n-icon>
                    </template>
                    <div>
                        <div>
                            Painter是一个通过Golang与Vue进行搭建的 社区博客 框架
                        </div>
                        <div>
                            目的是为了建立一个分享技术笔记以及日常的地方
                        </div>
                    </div>
                </n-popover>
            </span>
        </div>

        <!-- page3 -->
        <div class="entry-page-3"
            :style="{ 'opacity': page3Opacity, 'transform': 'translateY(' + page3Offset + 'px)' }">
            <div style="display: flex; align-items: center;">
                我是
                <span class="second-highlight">
                    逃塔
                </span>
            </div>

            <span v-for="tag in imTag" class="entry-tag">
                {{ tag }}
            </span>
            <div style="height: 10px;"></div>
            <div class="entry-contact-me">
                <div class="entry-icon-cont" @click="goLink('https://github.com/callme-taota')">
                    <n-icon size="20">
                        <logo-github />
                    </n-icon>
                </div>
                <div class="entry-icon-cont" @click="goLink('https://twitter.com/Taota_chen')">
                    <n-icon size="20">
                        <logo-twitter />
                    </n-icon>
                </div>
                <div class="entry-icon-cont" @click="goLink('http://www.callmetaota.fun/wechat.jpeg')">
                    <n-icon size="20">
                        <logo-wechat />
                    </n-icon>
                </div>
                <div class="entry-icon-cont" @click="goLink('mailto:taota.chen@gmail.com')">
                    <n-icon size="20">
                        <mail />
                    </n-icon>
                </div>
            </div>
        </div>

        <div style="height: 540px;"></div>



    </div>
</template>
<style>
.entry-cont {
    min-height: calc(100vh - 80px);
    display: flex;
    flex-direction: column;
    padding: 0 80px 20px 80px;
}

.entry-painter {
    min-height: calc(100vh - 65px);
    display: flex;
    flex-direction: column;
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
}

.entry-painter-text:hover {
    text-shadow: 0 0 12px #fff;
    color: var(--color-rev);
    background-color: var(--base-hover-background);
}

.entry-main-flex {
    padding: 10px 140px;
    display: flex;
    flex-direction: row;
    justify-content: space-between;
    transition: 0.5s;
}

.entry-page-2 {
    height: calc(50vh - 30px);
    font-weight: bold;
    font-size: xx-large;
    display: flex;
    align-items: end;
    transition: opacity 0.3s ease-in-out;
    flex-wrap: wrap;
    align-content: flex-end;
}

.entry-page-3 {
    font-weight: bold;
    transition: opacity 0.3s ease-in-out;
    font-size: xx-large;
}

.entry-tag {
    margin: 4px 8px 4px 0;
    padding: 2px 4px 2px 4px;
    font-size: large;
    font-weight: lighter;
    font-style: italic;
    border-bottom: 1px solid var(--highlight-color);
}





.entry-info {
    padding: 0px 140px;
    text-align: center;
    font-size: xx-large;
    font-weight: bolder;
}


.entry-img {
    height: 100%;
    border-radius: 50%;
    transition: 0.3s;
}

.entry-img:hover {
    transform: scale(1.1);
}

.entry-info-highlight {
    color: var(--highlight-color);
}

.second-highlight {
    color: var(--second-highlight-color);
}

.entry-info-large-text {
    font-size: 46px;
}

.entry-card3 {
    width: 29.3%;
}

.entry-card4 {
    width: 39.3%;
}

.entry-card6 {
    width: 59.3%;
}

.entry-card7 {
    width: 69.3%;
}

.entry-card {
    background-color: var(--card--background);
    border-radius: 20px;
    display: flex;
    align-items: center;
    transition: 0.3s;
}

.entry-card-border:hover {
    box-shadow: 0 0 4px var(--border-color);
}

.entry-card-with-background-1 {
    background-image: url('../assets/net.png');
    background-size: cover;
    background-repeat: no-repeat;
    color: #fff;
}

.entry-card-linear-1 {
    background: linear-gradient(90deg, #479ff2 0%, #5d5af7 100%);
    border-radius: 20px;
    color: var(--color-rev);
}

.entry-card-linear-2 {
    background: linear-gradient(90deg, #ffb95e 0%, #ffa791 100%);
    border-radius: 20px;
    color: var(--color-rev);
}

.entry-card-cont {
    overflow: hidden;
    padding: 30px;
    /* width: 100%; */
}

.entry-card-line1 {
    font-size: large;
    font-weight: bold;
}

.entry-card-line2 {
    font-size: 30px;
    font-weight: bolder;
    display: flex;
}

.entry-card-line3 {
    min-height: 30px;
    line-height: 30px;
    font-size: larger;
    font-weight: bold;
}

.entry-card-line4 {
    display: flex;
    flex-wrap: wrap;
    font-size: larger;
    font-weight: bold;
    justify-content: space-between;
    align-items: center;
}


.entry-contact-me {
    flex-wrap: wrap;
    display: flex;
    margin-top: 8px;
}

.entry-icon-cont {
    transition: 0.3s;
    border-radius: 14px;
    height: 40px;
    width: 40px;
    display: flex;
    align-items: center;
    cursor: pointer;
    justify-content: center;
    background: var(--layout--bottom-background);
    margin-right: 8px;
}

.entry-icon-cont:hover {
    background-color: var(--base-hover-background);
    color: var(--btn-hover-color);
}

.entry-text-cont {
    transition: 0.3s;
    border-radius: 14px;
    height: 40px;
    padding: 2px 8px;
    display: flex;
    align-items: center;
    cursor: pointer;
    justify-content: center;
    background: var(--layout--bottom-background);
    margin-right: 8px;
    font-weight: bolder;
}

.entry-text-cont:hover {
    background-color: var(--btn-hover-grey);
    color: var(--btn-hover-color);
}

.entry-skill-roll {
    margin-top: 20px;
    display: flex;
    flex-wrap: nowrap;
    animation: rowleft 30s linear infinite;
}

.entry-skill-single-c {
    display: flex;
    flex-direction: column;
    margin-left: 1rem;
    user-select: none;
}

.entry-skill-icon-cont {
    height: 100px;
    width: 100px;
    display: flex;
    align-items: center;
    justify-content: center;
    border-radius: 26px;
    box-shadow: 0 2px 16px -3px rgba(0, 0, 0, .15);
}

.entry-skill-icon {
    width: 60%;
}

.entry-skill-line2 {
    margin-top: 1rem;
    transform: translate(-50px);
}

.entry-about-cont {
    grid-template-columns: repeat(3, 1fr);
    gap: 44px 24px;
    display: grid;
}

@media (max-width: 1200px) {

    .entry-main-flex {
        padding: 0px 80px;
        flex-direction: column;
    }

    .entry-info {
        padding: 0 10px;
    }

    .entry-card3,
    .entry-card4,
    .entry-card6,
    .entry-card7 {
        width: 100%;
        margin: 10px 0;
    }
}

@media (max-width: 500px) {
    .entry-main-flex {
        padding: 0px 20px;
        flex-direction: column;
    }
}

@keyframes rowleft {
    0% {
        transform: translateX(0);
    }

    50% {
        transform: translateX(-50%);
    }
}
</style>