<template>
  <div class="apple-root entry-marketing">
    <div class="apple-mesh-bg">
      <div class="apple-blob apple-blob--cyan apple-parallax-layer" :style="blobA" />
      <div class="apple-blob apple-blob--violet apple-parallax-layer" :style="blobB" />
      <div class="apple-blob apple-blob--rose apple-parallax-layer" :style="blobC" />
    </div>

    <section class="apple-hero-min-h apple-section" :style="heroLift">
      <h1 class="apple-hero-title apple-parallax-layer" :style="titleParallax">Painter 2026</h1>
      <p class="apple-hero-sub apple-parallax-layer" :style="subParallax">
        社区博客框架，极简界面与流畅视差。探索文章、标签与分类，或前往你的 Apple 风格个人主页。
      </p>
      <div class="apple-pill-row apple-parallax-layer" :style="pillParallax">
        <RouterLink class="apple-pill" to="/articles">浏览文章</RouterLink>
        <RouterLink class="apple-pill" to="/tag">标签</RouterLink>
        <RouterLink class="apple-pill" to="/category">分类</RouterLink>
        <RouterLink class="apple-pill" to="/login">登录</RouterLink>
        <RouterLink class="apple-pill" to="/register">注册</RouterLink>
        <RouterLink class="apple-pill apple-pill--emphasis" to="/me">我的主页</RouterLink>
      </div>
    </section>

    <section class="apple-section apple-secondary-block" :style="sectionParallax">
      <div class="apple-glass" style="padding: 40px min(32px, 5vw); text-align: center; max-width: 720px; margin: 0 auto">
        <h2 class="apple-section-title" style="margin-top: 0">为读写而设计</h2>
        <p class="apple-hero-sub" style="max-width: none">
          大留白、柔和渐变与毛玻璃层次，滚动时背景与前景以不同速率移动，形成轻微
          <strong>视差景深</strong>
          ，让首页与「我的主页」都有一致的空间感。
        </p>
        <div class="entry-chip-row">
          <span v-for="tag in imTags" :key="tag" class="entry-chip">{{ tag }}</span>
        </div>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed } from "vue";
import { useWindowScroll } from "../composables/useWindowScroll";

const imTags = ["Go 微服务", "Vue 3", "MySQL · Redis", "工程化", "开放接口"];

const { scrollY } = useWindowScroll();

const scaleBoost = computed(() => 1 + scrollY.value * 0.0001);

const blobA = computed(() => ({
  transform: `translate3d(0, ${scrollY.value * 0.11}px, 0) scale(${scaleBoost.value})`,
}));
const blobB = computed(() => ({
  transform: `translate3d(0, ${scrollY.value * -0.07}px, 0) scale(${scaleBoost.value})`,
}));
const blobC = computed(() => ({
  transform: `translate3d(0, ${scrollY.value * 0.05}px, 0) scale(${scaleBoost.value})`,
}));

const heroLift = computed(() => ({
  transform: `translate3d(0, ${scrollY.value * -0.02}px, 0)`,
}));

const titleParallax = computed(() => ({
  transform: `translate3d(0, ${scrollY.value * -0.015}px, 0)`,
}));

const subParallax = computed(() => ({
  transform: `translate3d(0, ${scrollY.value * -0.008}px, 0)`,
  opacity: String(Math.max(0.55, 1 - scrollY.value / 900)),
}));

const pillParallax = computed(() => ({
  transform: `translate3d(0, ${scrollY.value * -0.004}px, 0)`,
}));

const sectionParallax = computed(() => ({
  transform: `translate3d(0, ${scrollY.value * 0.03}px, 0)`,
}));
</script>

<style scoped>
.entry-marketing {
  padding-bottom: 80px;
}

.apple-pill--emphasis {
  background: rgba(0, 113, 227, 0.12);
  border-color: rgba(0, 113, 227, 0.35);
  color: #0071e3;
}

.entry-chip-row {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  justify-content: center;
  margin-top: 28px;
}

.entry-chip {
  font-size: 14px;
  padding: 8px 14px;
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.45);
  border: 1px solid rgba(0, 0, 0, 0.06);
}

.apple-secondary-block {
  padding-top: 20px;
}
</style>
