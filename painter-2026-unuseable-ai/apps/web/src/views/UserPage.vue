<template>
  <div class="apple-root">
    <div class="apple-mesh-bg">
      <div class="apple-blob apple-blob--cyan apple-parallax-layer" :style="blobA" />
      <div class="apple-blob apple-blob--violet apple-parallax-layer" :style="blobB" />
      <div class="apple-blob apple-blob--rose apple-parallax-layer" :style="blobC" />
    </div>

    <section class="apple-hero-min-h apple-section">
      <div class="apple-glass apple-profile-card apple-parallax-layer" :style="cardParallax">
        <template v-if="loading">
          <p class="apple-hint">加载用户…</p>
        </template>
        <template v-else-if="info">
          <div class="apple-avatar">{{ avatarLetter }}</div>
          <div class="apple-display-name">{{ displayName }}</div>
          <div class="apple-handle">@{{ info.userId }}</div>
          <div class="apple-stats">
            <div class="apple-stat">
              <div class="apple-stat-value">{{ followingCount }}</div>
              <div class="apple-stat-label">关注</div>
            </div>
            <div class="apple-stat">
              <div class="apple-stat-value">{{ followerCount }}</div>
              <div class="apple-stat-label">粉丝</div>
            </div>
            <div class="apple-stat">
              <div class="apple-stat-value">{{ articleCount }}</div>
              <div class="apple-stat-label">文章</div>
            </div>
            <div class="apple-stat">
              <div class="apple-stat-value">{{ info.group }}</div>
              <div class="apple-stat-label">组别</div>
            </div>
          </div>
          <div v-if="!isSelf" class="apple-row-actions">
            <button type="button" class="apple-btn-primary" :disabled="!canFollow || busy" @click="follow">
              {{ busy ? "…" : "关注" }}
            </button>
            <RouterLink class="apple-btn-secondary" :to="{ path: '/articlelist', query: { type: '3', id: targetId } }">
              Ta 的文章
            </RouterLink>
          </div>
          <p v-if="hint" class="apple-hint" style="margin-top: 16px">{{ hint }}</p>
        </template>
        <template v-else>
          <p class="apple-error">{{ error || "用户不存在或无法加载" }}</p>
        </template>
      </div>
      <p class="apple-scroll-hint">视差背景 · 个人展示</p>
    </section>

    <section v-if="info" class="apple-section">
      <h2 class="apple-section-title">公开作品</h2>
      <p v-if="articlesLoading" class="apple-hint" style="text-align: center">加载中…</p>
      <p v-else-if="!theirArticles.length" class="apple-hint" style="text-align: center">暂无文章</p>
      <div v-else class="apple-article-grid">
        <RouterLink v-for="a in theirArticles" :key="a.articleId" class="apple-article-tile" :to="`/articles/${a.articleId}`">
          <span class="apple-article-tile-title">{{ a.title }}</span>
          <span class="apple-article-tile-meta">{{ formatDate(a.updatedAt) }}</span>
        </RouterLink>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { useRoute } from "vue-router";
import { createClient } from "@painter/sdk-ts";
import { useWindowScroll } from "../composables/useWindowScroll";

type ArticleRow = {
  articleId: string;
  title: string;
  summary?: string;
  authorId?: string;
  updatedAt?: string;
};

type UserInfo = {
  userId: string;
  userName: string;
  email: string;
  nickName: string;
  phone: string;
  group: number;
};

const route = useRoute();
const { scrollY } = useWindowScroll();

const targetId = ref("");
const loading = ref(true);
const articlesLoading = ref(false);
const busy = ref(false);
const error = ref("");
const hint = ref("");
const info = ref<UserInfo | null>(null);
const followingCount = ref(0);
const followerCount = ref(0);
const articleCount = ref(0);
const theirArticles = ref<ArticleRow[]>([]);

const token = ref(localStorage.getItem("painter_token") ?? "");
const selfId = ref(localStorage.getItem("painter_user_id") ?? "");

const isSelf = computed(() => !!selfId.value && selfId.value === targetId.value);
const canFollow = computed(() => !!token.value && !!selfId.value && !isSelf.value);

const api = () =>
  createClient({
    baseUrl: "http://localhost:18080/api/v1",
    headers:
      token.value && selfId.value
        ? { Authorization: `Bearer ${token.value}`, "X-User-Id": selfId.value }
        : undefined,
  });

const displayName = computed(() => {
  const n = info.value?.nickName?.trim();
  if (n) return n;
  return info.value?.userName ?? targetId.value;
});

const avatarLetter = computed(() => {
  const base = displayName.value.trim();
  return base ? base.slice(0, 1).toUpperCase() : "?";
});

const scaleBoost = computed(() => 1 + scrollY.value * 0.00012);
const blobA = computed(() => ({
  transform: `translate3d(0, ${scrollY.value * 0.12}px, 0) scale(${scaleBoost.value})`,
}));
const blobB = computed(() => ({
  transform: `translate3d(0, ${scrollY.value * -0.08}px, 0) scale(${scaleBoost.value})`,
}));
const blobC = computed(() => ({
  transform: `translate3d(0, ${scrollY.value * 0.05}px, 0) scale(${scaleBoost.value})`,
}));
const cardParallax = computed(() => ({
  transform: `translate3d(0, ${scrollY.value * -0.035}px, 0)`,
}));

const formatDate = (raw?: string) => {
  if (!raw) return "";
  const d = new Date(raw);
  return Number.isNaN(d.getTime()) ? raw.slice(0, 10) : d.toLocaleDateString();
};

const resolveTarget = () => {
  targetId.value = String(route.params.id || route.query.UserID || route.query.id || "u-admin");
};

const load = async () => {
  resolveTarget();
  token.value = localStorage.getItem("painter_token") ?? "";
  selfId.value = localStorage.getItem("painter_user_id") ?? "";
  loading.value = true;
  error.value = "";
  info.value = null;
  try {
    const res = await api().getUserInfo(targetId.value);
    info.value = res.data as UserInfo;
    const [fol, fer] = await Promise.all([api().listFollowings(targetId.value), api().listFollowers(targetId.value)]);
    followingCount.value = fol.data.items.length;
    followerCount.value = fer.data.items.length;
    articlesLoading.value = true;
    const arts = await api().listArticles(undefined, 120);
    const rows = (arts.data.items as ArticleRow[]) ?? [];
    const mine = rows.filter((r) => r.authorId === targetId.value);
    articleCount.value = mine.length;
    theirArticles.value = mine.slice(0, 12);
  } catch (e) {
    error.value = (e as Error).message;
  } finally {
    loading.value = false;
    articlesLoading.value = false;
  }
};

const follow = async () => {
  if (!canFollow.value) return;
  busy.value = true;
  hint.value = "";
  try {
    await api().createFollow({ targetUserId: targetId.value });
    hint.value = "已发送关注请求";
  } catch (e) {
    hint.value = (e as Error).message;
  } finally {
    busy.value = false;
  }
};

watch(
  () => [route.params.id, route.query.UserID, route.query.id],
  () => {
    load();
  },
  { immediate: true }
);
</script>
