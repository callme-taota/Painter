<template>
  <div class="apple-root">
    <div class="apple-mesh-bg">
      <div class="apple-blob apple-blob--cyan apple-parallax-layer" :style="blobA" />
      <div class="apple-blob apple-blob--rose apple-parallax-layer" :style="blobB" />
    </div>
    <section class="apple-section" style="padding-top: 40px">
      <h1 class="apple-hero-title" style="font-size: clamp(28px, 5vw, 40px); text-align: center; margin-bottom: 8px">新建文章</h1>
      <p class="apple-hero-sub" style="text-align: center; margin-bottom: 28px">简洁编辑器，发布后将在文章流中展示。</p>
      <div class="apple-glass apple-editor-card apple-parallax-layer" :style="cardParallax">
        <label class="apple-kv" style="display: block; margin-bottom: 6px">标题</label>
        <input v-model="title" placeholder="标题（必填）" />
        <label class="apple-kv" style="display: block; margin: 16px 0 6px">正文（支持 Markdown）</label>
        <textarea v-model="content" rows="14" placeholder="内容…"></textarea>
        <div class="apple-row-actions" style="justify-content: flex-start; margin-top: 20px">
          <button type="button" class="apple-btn-primary" @click="submit">发布</button>
        </div>
        <p v-if="hint" class="apple-hint" style="margin-top: 12px">{{ hint }}</p>
        <p v-if="error" class="apple-error" style="margin-top: 8px">{{ error }}</p>
        <pre v-if="result" class="apple-kv" style="white-space: pre-wrap; margin-top: 12px">{{ result }}</pre>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from "vue";
import { createClient } from "@painter/sdk-ts";
import { useWindowScroll } from "../composables/useWindowScroll";

const title = ref("");
const content = ref("");
const result = ref("");
const error = ref("");
const hint = ref("");
const token = localStorage.getItem("painter_token") ?? "";
const currentUserId = localStorage.getItem("painter_user_id") ?? "";

const client = createClient({
  baseUrl: "http://localhost:18080/api/v1",
  headers: {
    ...(token ? { Authorization: `Bearer ${token}` } : {}),
    ...(currentUserId ? { "X-User-Id": currentUserId } : {}),
  },
});

const { scrollY } = useWindowScroll();
const scaleBoost = computed(() => 1 + scrollY.value * 0.00006);
const blobA = computed(() => ({
  transform: `translate3d(0, ${scrollY.value * 0.07}px, 0) scale(${scaleBoost.value})`,
}));
const blobB = computed(() => ({
  transform: `translate3d(0, ${scrollY.value * -0.05}px, 0) scale(${scaleBoost.value})`,
}));
const cardParallax = computed(() => ({
  transform: `translate3d(0, ${scrollY.value * -0.025}px, 0)`,
}));

const submit = async () => {
  if (!title.value.trim() || !content.value.trim()) {
    error.value = "标题和内容不能为空";
    return;
  }
  try {
    error.value = "";
    const res = await client.createArticle({
      title: title.value,
      content: content.value,
      categoryId: "c-general",
      summary: content.value.slice(0, 120),
    });
    result.value = JSON.stringify(res.data, null, 2);
    hint.value = "发布成功";
  } catch (e) {
    error.value = (e as Error).message;
  }
};
</script>
