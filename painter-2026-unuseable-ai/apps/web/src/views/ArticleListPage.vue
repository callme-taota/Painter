<template>
  <div class="apple-root">
    <div class="apple-mesh-bg article-list-mesh" aria-hidden="true">
      <div class="apple-blob apple-blob--cyan" />
      <div class="apple-blob apple-blob--violet" />
    </div>
    <section style="position: relative; z-index: 1">
    <div class="page-header">
      <h1>文章列表</h1>
    </div>
    <div class="page-cont">
      <div class="article-list-toolbar">
        <button class="btn" type="button" :disabled="loading" @click="load">
          {{ loading ? "加载中..." : "刷新列表" }}
        </button>
        <span v-if="hint" class="hint">{{ hint }}</span>
      </div>
      <p v-if="error" class="error">{{ error }}</p>
      <p v-if="!loading && !items.length" class="empty">暂无文章</p>
      <div v-else class="article-list-flex">
        <RouterLink
          v-for="item in items"
          :key="item.articleId"
          :to="`/articles/${item.articleId}`"
          class="article-card"
        >
          <div class="article-card-cont">
            <div class="article-card-title">{{ item.title }}</div>
            <div class="article-card-summary">{{ summaryText(item) }}</div>
            <div class="article-more-cont">
              <div class="article-more-item">更新 {{ formatDate(item.updatedAt || item.createdAt) }}</div>
              <div class="article-more-item">赞 {{ item.likeCount ?? "—" }}</div>
              <div class="article-more-item">评 {{ item.commentCount ?? "—" }}</div>
            </div>
            <div v-if="item.tags?.length" class="article-card-tag-row">
              <span v-for="t in item.tags.slice(0, 4)" :key="t" class="article-card-tag-chip">{{ t }}</span>
            </div>
          </div>
        </RouterLink>
      </div>
    </div>
  </section>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from "vue";
import { createClient } from "@painter/sdk-ts";

type ArticleItem = {
  articleId: string;
  title: string;
  summary?: string;
  createdAt?: string;
  updatedAt?: string;
  likeCount?: number;
  commentCount?: number;
  tags?: string[];
};

const items = ref<ArticleItem[]>([]);
const loading = ref(false);
const error = ref("");
const hint = ref("");
const client = createClient({ baseUrl: "http://localhost:18080/api/v1" });

const summaryText = (item: ArticleItem) => {
  const s = item.summary?.trim();
  if (s) return s;
  return "点击查看全文";
};

const formatDate = (raw?: string) => {
  if (!raw) return "—";
  const d = new Date(raw);
  if (Number.isNaN(d.getTime())) return raw.slice(0, 10);
  return d.toLocaleDateString();
};

const load = async () => {
  try {
    loading.value = true;
    error.value = "";
    const res = await client.listArticles(undefined, 20);
    items.value = (res.data.items as ArticleItem[]) ?? [];
    hint.value = `共加载 ${items.value.length} 篇`;
  } catch (e) {
    error.value = (e as Error).message;
  } finally {
    loading.value = false;
  }
};

onMounted(load);
</script>

<style scoped>
.article-list-mesh .apple-blob {
  opacity: 0.38;
  transform: scale(1.1);
}
</style>
