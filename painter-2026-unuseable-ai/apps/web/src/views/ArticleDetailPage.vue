<template>
  <div class="apple-root">
    <div class="apple-mesh-bg">
      <div class="apple-blob apple-blob--cyan apple-parallax-layer" :style="blobA" />
      <div class="apple-blob apple-blob--violet apple-parallax-layer" :style="blobB" />
    </div>
    <article class="apple-prose-wrap apple-parallax-layer" :style="articleLift">
      <p class="apple-prose-meta">ID · {{ articleId }}</p>
      <h1 class="apple-prose-title">{{ article?.title || (loading ? "加载中…" : "文章") }}</h1>
      <p v-if="article" class="apple-prose-meta">
        作者 {{ article.authorId }} · 更新 {{ formatDate(article.updatedAt) }} · {{ article.likeCount }} 赞
      </p>
      <div v-if="article" class="apple-glass" style="padding: 28px min(24px, 4vw); margin-bottom: 28px">
        <p v-if="article.summary" style="margin: 0 0 16px; font-size: 18px; color: rgba(29, 29, 31, 0.72); line-height: 1.5">
          {{ article.summary }}
        </p>
        <div class="apple-prose-body">{{ article.content }}</div>
      </div>
      <div class="apple-row-actions" style="justify-content: flex-start; margin-bottom: 24px">
        <button type="button" class="apple-btn-primary" @click="like">点赞</button>
        <button type="button" class="apple-btn-secondary" @click="collect">收藏</button>
      </div>
      <p v-if="hint" class="apple-hint">{{ hint }}</p>
      <p v-if="error" class="apple-error">{{ error }}</p>

      <h2 class="apple-section-title" style="margin-top: 48px; text-align: left; font-size: 22px">评论</h2>
      <div class="apple-glass" style="padding: 20px; margin-bottom: 16px">
        <div style="display: flex; gap: 10px; flex-wrap: wrap">
          <input v-model="commentContent" placeholder="写下想法…" style="flex: 1; min-width: 200px; border-radius: 12px; border: 1px solid rgba(0, 0, 0, 0.08); padding: 10px 12px" />
          <button type="button" class="apple-btn-primary" @click="submitComment">发送</button>
        </div>
      </div>
      <ul style="list-style: none; padding: 0; margin: 0; display: grid; gap: 10px">
        <li v-for="c in comments" :key="c.commentId" class="apple-glass" style="padding: 14px 16px">
          <span style="font-size: 13px; color: rgba(29, 29, 31, 0.5)">{{ c.userId }}</span>
          <p style="margin: 6px 0 0; line-height: 1.5">{{ c.content }}</p>
        </li>
      </ul>
    </article>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { useRoute } from "vue-router";
import { createClient } from "@painter/sdk-ts";
import { useWindowScroll } from "../composables/useWindowScroll";

const route = useRoute();
const { scrollY } = useWindowScroll();

const articleId = computed(() => String(route.params.id || route.query.ArticleID || "a-1"));
const article = ref<{
  articleId: string;
  title: string;
  summary: string;
  content: string;
  authorId: string;
  categoryId: string;
  likeCount: number;
  updatedAt: string;
} | null>(null);
const loading = ref(true);
const commentContent = ref("");
const comments = ref<Array<{ commentId: string; userId: string; content: string }>>([]);
const hint = ref("");
const error = ref("");
const token = localStorage.getItem("painter_token") ?? "";
const currentUserId = localStorage.getItem("painter_user_id") ?? "";

const client = createClient({
  baseUrl: "http://localhost:18080/api/v1",
  headers: {
    ...(token ? { Authorization: `Bearer ${token}` } : {}),
    ...(currentUserId ? { "X-User-Id": currentUserId } : {}),
  },
});

const scaleBoost = computed(() => 1 + scrollY.value * 0.00008);
const blobA = computed(() => ({
  transform: `translate3d(0, ${scrollY.value * 0.06}px, 0) scale(${scaleBoost.value})`,
}));
const blobB = computed(() => ({
  transform: `translate3d(0, ${scrollY.value * -0.04}px, 0) scale(${scaleBoost.value})`,
}));
const articleLift = computed(() => ({
  transform: `translate3d(0, ${scrollY.value * -0.02}px, 0)`,
}));

const formatDate = (raw: string) => {
  const d = new Date(raw);
  return Number.isNaN(d.getTime()) ? raw.slice(0, 10) : d.toLocaleString();
};

const loadArticle = async () => {
  loading.value = true;
  error.value = "";
  try {
    const res = await client.getArticle(articleId.value);
    article.value = res.data;
  } catch (e) {
    error.value = (e as Error).message;
    article.value = null;
  } finally {
    loading.value = false;
  }
};

const loadComments = async () => {
  try {
    error.value = "";
    const res = await client.listComments(articleId.value);
    comments.value = res.data.items.map((item) => ({
      commentId: item.commentId,
      userId: item.userId,
      content: item.content,
    }));
  } catch (e) {
    error.value = (e as Error).message;
  }
};

const submitComment = async () => {
  if (!commentContent.value.trim()) return;
  try {
    error.value = "";
    await client.createComment({ articleId: articleId.value, content: commentContent.value });
    commentContent.value = "";
    hint.value = "评论已发布";
    await loadComments();
  } catch (e) {
    error.value = (e as Error).message;
  }
};

const like = async () => {
  try {
    error.value = "";
    await client.likeArticle({ articleId: articleId.value });
    hint.value = "感谢点赞";
    await loadArticle();
  } catch (e) {
    error.value = (e as Error).message;
  }
};

const collect = async () => {
  try {
    error.value = "";
    await client.collectArticle({ articleId: articleId.value });
    hint.value = "已加入收藏";
  } catch (e) {
    error.value = (e as Error).message;
  }
};

watch(
  articleId,
  async () => {
    await Promise.all([loadArticle(), loadComments()]);
  },
  { immediate: true }
);
</script>

<style scoped>
.apple-prose-body {
  white-space: pre-wrap;
  word-break: break-word;
  font-size: 16px;
  line-height: 1.65;
  color: #1d1d1f;
}
</style>
