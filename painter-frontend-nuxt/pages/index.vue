<script setup lang="ts">
//base
import { onMounted, onUnmounted, ref } from "vue";
import { NIcon, NPopover } from "naive-ui";
//api
import { GetEntryInfo } from "@/apis/api_common";
//icon
import { LogoGithub, LogoTwitter, LogoWechat, Mail } from "@vicons/ionicons5";
import { QuestionCircleOutlined } from "@vicons/antd";
//ref
const dateDiff = ref<string>("");
const yesterDayPV = ref<number>(0);
const monthPV = ref<number>(0);
const articleCount = ref<number>(0);
const pageHeight = ref<number>(0);
//hook
let timer: number | undefined;
onMounted(async () => {
  if (typeof window !== 'undefined') {
    pageHeight.value = window.innerHeight;
    document.addEventListener("scroll", atPage);
    timer = window.setInterval(autoChangeName, 300);
    await getInfo();
  }
});

const getInfo = async () => {
  let res = await GetEntryInfo();
  if (res.ok) {
    let day = new Date(res.data.JSTimeStamp);
    let now = new Date();
    let diff = Math.abs(now.getTime() - day.getTime());
    const oneDay = 86400000; // 24 * 60 * 60 * 1000
    dateDiff.value = `${Math.floor(diff / oneDay)}天`;
    yesterDayPV.value = res.data.YesterdayCount;
    monthPV.value = res.data.CurrentMonthCount;
    articleCount.value = res.data.ArticleCount;
  }
};

const imTag = [
  "Golang工程师",
  "云原生工程师",
  "前端爱好者",
  "币圈玩家",
  "摄影佬",
  "EDM制作人",
  "学生",
];

// 动态变化
const titleName = ref("");
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
];
let currentIndex = 0;
const autoChangeName = () => {
  if (currentIndex >= painterList.length - 1) {
    if (typeof window !== 'undefined') {
      clearInterval(timer);
    }
  }
  titleName.value = painterList[currentIndex];
  currentIndex++;
};

const pageTranstion = ref<{
  [key: number]: { opacity: number; offset: number };
}>({
  1: { opacity: 1, offset: 0 },
  2: { opacity: 1, offset: 0 },
  3: { opacity: 1, offset: 0 },
});
const pageAt = ref<number>(0);

const scrollTo = (pos: number) => {
  if (typeof window !== 'undefined') {
    window.scrollTo({ left: 0, top: pos, behavior: "smooth" });
  }
};

const scrollToPage = (page: number) => {
  let pos = (pageHeight.value - 60) * (page - 1);
  scrollTo(pos);
};

const atPage = (e: any) => {
  pageAt.value = e.target.scrollingElement.scrollTop / (pageHeight.value - 60);
  fixPage(1, e);
  fixPage(2, e);
};

const pagePosition = (page: number) => {
  return (pageHeight.value - 60) * page;
};

const fixPage = (page: number, e: any) => {
  const range = 0.5;
  let atPage: number = pageAt.value;
  let distantTo = distantToFix(atPage, page, range);
  let offset = e.target.scrollingElement.scrollTop - pagePosition(page);
  if (distantTo > 0.7) {
    distantTo = 1;
  } else if (distantTo <= 0) {
    offset = 0;
  }
  pageTranstion.value[page].opacity = distantTo;
  pageTranstion.value[page].offset = offset;
};

const rangeAt = (at: number, fix: number, range: number): boolean => {
  if (at < fix + range && at > fix - range) {
    return true;
  }
  return false;
};

const distantToFix = (at: number, fix: number, range: number): number => {
  if (!rangeAt(at, fix, range)) {
    return 0;
  }
  if (at > fix) {
    return 1 - (at - fix) / range;
  } else {
    return 1 - (fix - at) / range;
  }
};

const goLink = (link: string) => {
  if (typeof window !== 'undefined') {
    window.open(link, "_blank");
  }
};
</script>
<template>
  <div class="entry-cont">

    <div class="entry-page">
      <span class="entry-painter-text" @click="scrollToPage(2)">
        {{ titleName }}
      </span>
    </div>
    <div class="entry-page entry-page-bold" :style="{
      opacity: pageTranstion[1].opacity,
      transform: 'translateY(' + pageTranstion[1].offset + 'px)',
    }">
      <div>
        <span class="second-highlight"> 你好, </span>
        &nbsp;欢迎来到
        <span> Painter 👋 </span>
        <span style="vertical-align: super; margin: 0 5px">
          <client-only>
            <n-popover>
              <template #trigger>
                <n-icon size="18">
                  <QuestionCircleOutlined />
                </n-icon>
              </template>
              <div>
                <div>Painter是一个通过Golang与Vue进行搭建的 社区博客 框架</div>
                <div>目的是为了建立一个分享技术笔记以及日常的地方</div>
              </div>
            </n-popover>
          </client-only>
        </span>
      </div>
    </div>

    <div class="entry-page entry-page-bold" :style="{
      opacity: pageTranstion[2].opacity,
      transform: 'translateY(' + pageTranstion[2].offset + 'px)',
    }">
      <div>
        <div style="display: flex; align-items: center">
          我是
          <span class="second-highlight"> 逃塔 </span>
        </div>

        <span v-for="tag in imTag" class="entry-tag">
          {{ tag }}
        </span>
        <div style="height: 10px"></div>
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

    <!-- <div class="entry-page entry-page-bold" :style="{
      opacity: pageTranstion[3].opacity,
      transform: 'translateY(' + pageTranstion[3].offset + 'px)',
    }"></div> -->

    <div class="entry-side-page-number">
      <div class="entry-side-to-btn" :style="{ background: pageAt < 0.5 ? 'var(--base-dot-hover)' : '' }"
        @click="scrollToPage(1)"></div>
      <div class="entry-side-to-btn" :style="{
        background:
          pageAt < 1.5 && pageAt > 0.5 ? 'var(--base-dot-hover)' : '',
      }" @click="scrollToPage(2)"></div>
      <div class="entry-side-to-btn" :style="{
        background:
          pageAt < 2.5 && pageAt > 1.5 ? 'var(--base-dot-hover)' : '',
      }" @click="scrollToPage(3)"></div>
      <!-- <div class="entry-side-to-btn" :style="{
        background:
          pageAt < 3.5 && pageAt > 2.5 ? 'var(--base-dot-hover)' : '',
      }" @click="scrollToPage(4)"></div> -->
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

.entry-page {
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

.second-highlight {
  color: var(--second-highlight-color);
}

.entry-page-bold {
  font-weight: bold;
  font-size: xx-large;
  transition: opacity 0.05s ease-in-out;
}

.entry-tag {
  margin: 4px 8px 4px 0;
  padding: 2px 4px 2px 4px;
  font-size: large;
  font-weight: lighter;
  font-style: italic;
  border-bottom: 1px solid var(--highlight-color);
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

.entry-side-page-number {
  position: fixed;
  right: 20px;
  top: 50%;
  width: 12px;
  display: flex;
  flex-direction: column;
  transform: translateY(-50%);
}

.entry-side-to-btn {
  border-radius: 50%;
  background-color: var(--base-dot);
  width: 12px;
  height: 12px;
  margin: 4px 0;
  cursor: pointer;
  transition: 0.3s;
}

.entry-side-to-btn:hover {
  background-color: var(--base-dot-hover);
}
</style>
