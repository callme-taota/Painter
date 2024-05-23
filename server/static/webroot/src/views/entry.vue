<script setup lang="ts">
//base
import { onMounted, ref } from 'vue';
import { NIcon } from 'naive-ui'
//api
import { GetStartTime, GetDayPV, GetMonthPV } from '@/apis/api_common';
import { GetArticleCount } from '@/apis/api_article';
//icon
import { LogoGithub, LogoTwitter, LogoWechat, Mail } from '@vicons/ionicons5'
import { MouseOutlined as mouseIcon } from '@vicons/material';
//ref
const dateDiff = ref<string>("")
const dayPV = ref<number>(0)
const monthPV = ref<number>(0)
const articleCount = ref<number>(0)
//hook
onMounted(async () => {
    let res = await GetStartTime();
    if (res.ok) {
        let day = new Date(res.data.JSTimeStamp)
        let now = new Date()
        let diff = Math.abs(now.getTime() - day.getTime())
        const oneDay = 86400000; // 24 * 60 * 60 * 1000
        dateDiff.value = `${Math.floor(diff / oneDay)}天`;
    }
    res = await GetDayPV()
    if (res.ok) {
        dayPV.value = res.data.Count
    }
    res = await GetMonthPV()
    if (res.ok) {
        monthPV.value = res.data.Count
    }
    res = await GetArticleCount()
    if (res.ok) {
        articleCount.value = res.data.ArticleCount
    }
})

const goLink = (link: string) => {
    window.open(link, "_blank")
}
</script>
<template>
    <div class="entry-main-cont">
        <div class="entry-painter">
            <div class="entry-img-cont">
                <img class="entry-img" src="../assets/me.png" alt="Me">
            </div>
            <div class="entry-info">
                <span class="entry-info-highlight">
                    Hi,
                </span>
                <span>
                    &nbsp;
                </span>
                <span>
                    it's
                </span>
                <span class="entry-info-second-highlight entry-info-large-text">
                    Taota
                </span>
                <span>
                    here.
                </span>
            </div>
            <div class="entry-info">
                <span>
                    Welcome to
                </span>
                <span class="entry-info-box">
                    Painter
                </span>
                <span>
                    !
                </span>
            </div>
            <div class="entry-mouse">
                <n-icon>
                    <mouseIcon />
                </n-icon>
            </div>
        </div>

        <div class="entry-main-flex">
            <div class="entry-card6 entry-card entry-card-border">
                <div class="entry-card-cont">
                    <div class="entry-card-line1">
                        <span class="entry-info-second-highlight">
                            你好,
                        </span>
                        &nbsp;欢迎来到Painter 👋
                    </div>
                    <div class="entry-card-line2">
                        我叫&nbsp;
                        <span class="entry-info-second-highlight">
                            逃塔
                        </span>
                    </div>
                    <div class="entry-card-line4">
                        我是
                        <span class="entry-tag">
                            Golang工程师
                        </span>
                        <span class="entry-tag">
                            云原生
                        </span>
                        <span class="entry-tag">
                            学生
                        </span>
                        <span class="entry-tag">
                            摄影师
                        </span>
                        <span class="entry-tag">
                            Crypto玩家
                        </span>
                        <span class="entry-tag">
                            EDM Producer
                        </span>
                        <span class="entry-tag">
                            前端爱好者
                        </span>
                    </div>
                </div>
            </div>
            <div class="entry-card4 entry-card entry-card-linear-1">
                <div class="entry-card-cont">
                    <div class="entry-card-line1">
                        Painter为何物?
                    </div>
                    <div class="entry-card-line3">
                        Painter是一个通过Golang与Vue进行搭建的 社区博客 框架，目的是为了建立一个分享技术笔记以及日常的地方。
                    </div>
                </div>
            </div>

        </div>

        <div class="entry-main-flex">
            <div class="entry-card7 entry-card entry-card-border">
                <div class="entry-card-cont">
                    <div class="entry-card-line1">
                        探索
                    </div>
                </div>
            </div>
            <div class="entry-card3 entry-card entry-card-border">
                <div class="entry-card-cont">
                    <div class="entry-card-line1">
                        友链
                    </div>
                </div>
            </div>
        </div>

        <div class="entry-main-flex">
            <div class="entry-card4 entry-card entry-card-border">
                <div class="entry-card-cont">
                    <div class="entry-card-line1">
                        我的技能
                    </div>
                    <div class="entry-skill-roll">
                        <div class="entry-skill-single-c">
                            <div class="entry-skill-icon-cont">
                                <img class="entry-skill-icon" src="../assets/logos/Go.png" alt="">
                            </div>
                            <div class="entry-skill-icon-cont entry-skill-line2">
                                <img class="entry-skill-icon" src="../assets/logos/Ts.png" alt="">
                            </div>
                        </div>
                        <div class="entry-skill-single-c">
                            <div class="entry-skill-icon-cont">
                                <img class="entry-skill-icon" src="../assets/logos/Docker.png" alt="">
                            </div>
                            <div class="entry-skill-icon-cont entry-skill-line2">
                                <img class="entry-skill-icon" src="../assets/logos/K8s.png" alt="">
                            </div>
                        </div>
                        <div class="entry-skill-single-c">
                            <div class="entry-skill-icon-cont">
                                <img class="entry-skill-icon" src="../assets/logos/Mysql.png" alt="">
                            </div>
                            <div class="entry-skill-icon-cont entry-skill-line2">
                                <img class="entry-skill-icon" src="../assets/logos/Redis.png" alt="">
                            </div>
                        </div>
                        <div class="entry-skill-single-c">
                            <div class="entry-skill-icon-cont">
                                <img class="entry-skill-icon" src="../assets/logos/NodeJS.png" alt="">
                            </div>
                            <div class="entry-skill-icon-cont entry-skill-line2">
                                <img class="entry-skill-icon" src="../assets/logos/Vue.png" alt="">
                            </div>
                        </div>
                        <div class="entry-skill-single-c">
                            <div class="entry-skill-icon-cont">
                                <img class="entry-skill-icon" src="../assets/logos/React.png" alt="">
                            </div>
                            <div class="entry-skill-icon-cont entry-skill-line2">
                                <img class="entry-skill-icon" src="../assets/logos/Cpp.png" alt="">
                            </div>
                        </div>
                        <div class="entry-skill-single-c">
                            <div class="entry-skill-icon-cont">
                                <img class="entry-skill-icon" src="../assets/logos/Flutter.png" alt="">
                            </div>
                            <div class="entry-skill-icon-cont entry-skill-line2">
                                <img class="entry-skill-icon" src="../assets/logos/FL.png" alt="">
                            </div>
                        </div>
                        <div class="entry-skill-single-c">
                            <div class="entry-skill-icon-cont">
                                <img class="entry-skill-icon" src="../assets/logos/Go.png" alt="">
                            </div>
                            <div class="entry-skill-icon-cont entry-skill-line2">
                                <img class="entry-skill-icon" src="../assets/logos/Ts.png" alt="">
                            </div>
                        </div>
                        <div class="entry-skill-single-c">
                            <div class="entry-skill-icon-cont">
                                <img class="entry-skill-icon" src="../assets/logos/Docker.png" alt="">
                            </div>
                            <div class="entry-skill-icon-cont entry-skill-line2">
                                <img class="entry-skill-icon" src="../assets/logos/K8s.png" alt="">
                            </div>
                        </div>
                        <div class="entry-skill-single-c">
                            <div class="entry-skill-icon-cont">
                                <img class="entry-skill-icon" src="../assets/logos/Mysql.png" alt="">
                            </div>
                            <div class="entry-skill-icon-cont entry-skill-line2">
                                <img class="entry-skill-icon" src="../assets/logos/Redis.png" alt="">
                            </div>
                        </div>
                        <div class="entry-skill-single-c">
                            <div class="entry-skill-icon-cont">
                                <img class="entry-skill-icon" src="../assets/logos/NodeJS.png" alt="">
                            </div>
                            <div class="entry-skill-icon-cont entry-skill-line2">
                                <img class="entry-skill-icon" src="../assets/logos/Vue.png" alt="">
                            </div>
                        </div>
                        <div class="entry-skill-single-c">
                            <div class="entry-skill-icon-cont">
                                <img class="entry-skill-icon" src="../assets/logos/React.png" alt="">
                            </div>
                            <div class="entry-skill-icon-cont entry-skill-line2">
                                <img class="entry-skill-icon" src="../assets/logos/Cpp.png" alt="">
                            </div>
                        </div>
                        <div class="entry-skill-single-c">
                            <div class="entry-skill-icon-cont">
                                <img class="entry-skill-icon" src="../assets/logos/Flutter.png" alt="">
                            </div>
                            <div class="entry-skill-icon-cont entry-skill-line2">
                                <img class="entry-skill-icon" src="../assets/logos/FL.png" alt="">
                            </div>
                        </div>
                    </div>
                </div>
            </div>

            <div class="entry-card6 entry-card entry-card-with-background-1">
                <div class="entry-card-cont">
                    <div class="entry-card-line1">
                        关于本站
                    </div>
                    <br>
                    <div class="entry-about-cont">
                        <div>
                            昨日访问
                            <div class="entry-card-line2">
                                {{ dayPV }}
                            </div>
                        </div>
                        <div>
                            本月访问
                            <div class="entry-card-line2">
                                {{ monthPV }}
                            </div>
                        </div>
                        <div>
                        </div>
                    </div>
                    <div class="entry-about-cont">
                        <div>
                            文章总数
                            <div class="entry-card-line2">
                                {{ articleCount }}
                            </div>
                        </div>
                        <div>
                            已经运行
                            <div class="entry-card-line2">
                                {{ dateDiff }}
                            </div>
                        </div>
                        <div>
                        </div>
                    </div>
                </div>
            </div>
        </div>

        <div class="entry-main-flex">
            <div class="entry-card7 entry-card-linear-2">
                <div class="entry-card-cont">
                    <div class="entry-card-line2">
                        我的项目
                    </div>
                    <div class="entry-contact-me">
                        <div class="entry-text-cont" @click="goLink('https://github.com/callme-taota/Painter-Blog')">
                            Painter
                        </div>
                        <div class="entry-text-cont" @click="goLink('https://github.com/callme-taota/SendUCode')">
                            SendUCode
                        </div>
                        <div class="entry-text-cont" @click="goLink('https://github.com/callme-taota/CloudBox')">
                            CloudBox
                        </div>
                        <div class="entry-text-cont" @click="goLink('https://github.com/OpenAtomFoundation/pikiwidb')">
                            PikiwiDB(参与)
                        </div>
                    </div>
                </div>
            </div>
            <div class="entry-card3 entry-card entry-card-border">
                <div class="entry-card-cont">
                    <div class="entry-card-line1">
                        联系我
                    </div>
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
            </div>
        </div>
    </div>
</template>
<style>
.entry-main-cont {
    min-height: 640px;
    padding-bottom: 20px;
}

.entry-main-flex {
    padding: 10px 140px;
    display: flex;
    flex-direction: row;
    justify-content: space-between;
    transition: 0.5s;
}

.entry-painter {
    height: 86vh;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    position: relative;
}

.entry-mouse {
    position: absolute;
    bottom: 60px;
    font-size: x-large;
    border-radius: 18px;
    height: 50px;
    line-height: 56px;
    width: 38px;
    text-align: center;
    background-color: var(--btn-hover-grey);
    transition: 0.3s;
}

.entry-mouse:hover {
    background-color: var(--btn-hover-deep-grey);
}

.entry-info {
    padding: 0px 140px;
    text-align: center;
    font-size: xx-large;
    font-weight: bolder;
}

.entry-img-cont {
    display: flex;
    flex-direction: row;
    height: 180px;
    justify-content: center;
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

.entry-info-second-highlight {
    color: var(--second-highlight-color);
}

.entry-info-large-text {
    font-size: 46px;
}

.entry-info-box {
    font-weight: normal;
    color: var(--color-rev);
    background-color: var(--base-hover-background);
    padding: 0 0 0 8px;
    font-style: italic;
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

.entry-tag {
    margin: 4px 4px;
    padding: 2px 8px 2px 8px;
    color: var(--color-rev);
    background-color: #4facfe;
    border-radius: 10px;
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