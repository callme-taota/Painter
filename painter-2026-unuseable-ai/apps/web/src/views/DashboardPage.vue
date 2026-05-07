<template>
  <div class="apple-root">
    <div class="apple-mesh-bg">
      <div class="apple-blob apple-blob--cyan apple-parallax-layer" :style="blobA" />
      <div class="apple-blob apple-blob--violet apple-parallax-layer" :style="blobB" />
      <div class="apple-blob apple-blob--rose apple-parallax-layer" :style="blobC" />
    </div>

    <section class="apple-hero-min-h apple-section">
      <div class="apple-glass apple-profile-card apple-parallax-layer" :style="cardParallax">
        <template v-if="!token">
          <h1 class="apple-hero-title" style="font-size: clamp(32px, 6vw, 48px)">我的主页</h1>
          <p class="apple-hero-sub">登录后即可查看资料、文章与社交数据。极简排版、玻璃材质与轻视差景深。</p>
          <div class="apple-row-actions">
            <RouterLink class="apple-btn-primary" to="/login" style="text-align: center">登录</RouterLink>
            <RouterLink class="apple-btn-secondary" to="/register">注册账号</RouterLink>
          </div>
        </template>
        <template v-else-if="loading">
          <p class="apple-hint">加载中…</p>
        </template>
        <template v-else-if="profile">
          <div class="apple-avatar">{{ avatarLetter }}</div>
          <div class="apple-display-name">{{ displayName }}</div>
          <div class="apple-handle">@{{ profile.userId }}</div>
          <div class="apple-stats">
            <div class="apple-stat is-clickable" role="button" tabindex="0" @click="goFollow(1)">
              <div class="apple-stat-value">{{ followingCount }}</div>
              <div class="apple-stat-label">关注</div>
            </div>
            <div class="apple-stat is-clickable" role="button" tabindex="0" @click="goFollow(2)">
              <div class="apple-stat-value">{{ followerCount }}</div>
              <div class="apple-stat-label">粉丝</div>
            </div>
            <RouterLink :to="{ path: '/articlelist', query: { type: '4', id: userIdStr } }" class="apple-stat is-clickable">
              <div class="apple-stat-value">···</div>
              <div class="apple-stat-label">收藏</div>
            </RouterLink>
            <RouterLink :to="{ path: '/articlelist', query: { type: '3', id: userIdStr } }" class="apple-stat is-clickable">
              <div class="apple-stat-value">{{ articleCount }}</div>
              <div class="apple-stat-label">文章</div>
            </RouterLink>
          </div>
          <div class="apple-row-actions">
            <RouterLink class="apple-btn-primary" to="/editor/new">新建文章</RouterLink>
            <RouterLink class="apple-btn-secondary" to="/articles">浏览社区</RouterLink>
          </div>
        </template>
        <template v-else>
          <h2 class="apple-display-name" style="margin-top: 0">暂时无法加载主页</h2>
          <p class="apple-hero-sub">{{ error || "请稍后重试或重新登录。" }}</p>
          <div class="apple-row-actions">
            <RouterLink class="apple-btn-primary" to="/login">重新登录</RouterLink>
            <button type="button" class="apple-btn-secondary" @click="bootstrap">重试</button>
          </div>
        </template>
        <p v-if="error && profile" class="apple-error" style="margin-top: 16px">{{ error }}</p>
      </div>
      <p class="apple-scroll-hint">向下滚动 · 视差景深</p>
    </section>

    <section v-if="token && profile" class="apple-section">
      <h2 class="apple-section-title">近期写作</h2>
      <p v-if="articlesLoading" class="apple-hint" style="text-align: center">加载文章列表…</p>
      <p v-else-if="!myArticles.length" class="apple-hint" style="text-align: center">暂无公开文章，试试写一篇。</p>
      <div v-else class="apple-article-grid">
        <RouterLink v-for="a in myArticles" :key="a.articleId" class="apple-article-tile" :to="`/articles/${a.articleId}`">
          <span class="apple-article-tile-title">{{ a.title }}</span>
          <span class="apple-article-tile-meta">{{ formatDate(a.updatedAt) }} · {{ (a.summary ?? "").slice(0, 80) || "阅读全文" }}</span>
        </RouterLink>
      </div>
    </section>

    <section v-if="token && profile" class="apple-section">
      <details class="apple-tools apple-glass">
        <summary>开发者与调试</summary>
        <div class="apple-tools-body">
          <div class="apple-row-actions">
            <button type="button" class="apple-btn-secondary" :disabled="busy" @click="loadProfileDump">拉取自画像 JSON</button>
            <button type="button" class="apple-btn-secondary" :disabled="busy" @click="loadSettingsDump">公共配置</button>
            <button type="button" class="apple-btn-secondary" :disabled="busy" @click="checkSession">检查会话</button>
          </div>
          <p v-if="session" class="apple-kv">{{ session }}</p>
          <pre v-if="profileRaw" class="apple-kv" style="white-space: pre-wrap; margin: 0">{{ profileRaw }}</pre>
          <pre v-if="infoRaw" class="apple-kv" style="white-space: pre-wrap; margin: 0">{{ infoRaw }}</pre>
        </div>
      </details>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { useRouter } from "vue-router";
import { createClient } from "@painter/sdk-ts";
import { useWindowScroll } from "../composables/useWindowScroll";

type ArticleRow = {
  articleId: string;
  title: string;
  summary?: string;
  authorId?: string;
  updatedAt?: string;
};

type SelfProfile = {
  userId: string;
  userName: string;
  email: string;
  nickName: string;
  phone: string;
  group: number;
};

const router = useRouter();
const { scrollY } = useWindowScroll();

const token = ref(localStorage.getItem("painter_token") ?? "");
const userIdStr = ref(localStorage.getItem("painter_user_id") ?? "");
const loading = ref(false);
const articlesLoading = ref(false);
const busy = ref(false);
const error = ref("");
const profile = ref<SelfProfile | null>(null);
const followingCount = ref(0);
const followerCount = ref(0);
const articleCount = ref(0);
const myArticles = ref<ArticleRow[]>([]);
const session = ref("");
const profileRaw = ref("");
const infoRaw = ref("");

const api = () =>
  createClient({
    baseUrl: "http://localhost:18080/api/v1",
    headers:
      token.value && userIdStr.value
        ? { Authorization: `Bearer ${token.value}`, "X-User-Id": userIdStr.value }
        : token.value
          ? { Authorization: `Bearer ${token.value}` }
          : undefined,
  });

const displayName = computed(() => {
  const n = profile.value?.nickName?.trim();
  if (n) return n;
  return profile.value?.userName ?? userIdStr.value ?? "用户";
});

const avatarLetter = computed(() => {
  const base = displayName.value.trim();
  return base ? base.slice(0, 1).toUpperCase() : "?";
});

const scaleBoost = computed(() => 1 + scrollY.value * 0.00012);

const blobA = computed(() => ({
  transform: `translate3d(0, ${scrollY.value * 0.14}px, 0) scale(${scaleBoost.value})`,
}));

const blobB = computed(() => ({
  transform: `translate3d(0, ${scrollY.value * -0.09}px, 0) scale(${scaleBoost.value})`,
}));

const blobC = computed(() => ({
  transform: `translate3d(0, ${scrollY.value * 0.06}px, 0) scale(${scaleBoost.value})`,
}));

const cardParallax = computed(() => ({
  transform: `translate3d(0, ${scrollY.value * -0.04}px, 0)`,
}));

const formatDate = (raw?: string) => {
  if (!raw) return "";
  const d = new Date(raw);
  return Number.isNaN(d.getTime()) ? raw.slice(0, 10) : d.toLocaleDateString();
};

const loadMyArticles = async () => {
  if (!token.value || !userIdStr.value) return;
  articlesLoading.value = true;
  try {
    const arts = await api().listArticles(undefined, 120);
    const rows = (arts.data.items as ArticleRow[]) ?? [];
    const mine = rows.filter((r) => r.authorId === userIdStr.value);
    articleCount.value = mine.length;
    myArticles.value = mine.slice(0, 12);
  } finally {
    articlesLoading.value = false;
  }
};

const bootstrap = async () => {
  token.value = localStorage.getItem("painter_token") ?? "";
  userIdStr.value = localStorage.getItem("painter_user_id") ?? "";
  if (!token.value || !userIdStr.value) {
    profile.value = null;
    return;
  }
  loading.value = true;
  error.value = "";
  try {
    const res = await api().getSelf();
    profile.value = res.data as SelfProfile;
    const [fol, fer] = await Promise.all([api().listFollowings(userIdStr.value), api().listFollowers(userIdStr.value)]);
    followingCount.value = fol.data.items.length;
    followerCount.value = fer.data.items.length;
    await loadMyArticles();
  } catch (e) {
    error.value = (e as Error).message;
    profile.value = null;
  } finally {
    loading.value = false;
  }
};

const goFollow = (type: number) => {
  router.push({ path: "/follow", query: { type: String(type), id: userIdStr.value } });
};

const loadProfileDump = async () => {
  busy.value = true;
  profileRaw.value = "";
  try {
    const res = await api().getSelf();
    profileRaw.value = JSON.stringify(res.data, null, 2);
  } catch (e) {
    profileRaw.value = (e as Error).message;
  } finally {
    busy.value = false;
  }
};

const loadSettingsDump = async () => {
  busy.value = true;
  infoRaw.value = "";
  try {
    const res = await api().listConfigs("public");
    infoRaw.value = JSON.stringify(res.data, null, 2);
  } catch (e) {
    infoRaw.value = (e as Error).message;
  } finally {
    busy.value = false;
  }
};

const checkSession = async () => {
  busy.value = true;
  session.value = "";
  try {
    if (!token.value) {
      session.value = "未登录";
      return;
    }
    const res = await api().checkLogin({ accessToken: token.value });
    session.value = res.data.loggedIn ? `已登录：${res.data.userId ?? ""}` : "登录态失效";
  } catch (e) {
    session.value = (e as Error).message;
  } finally {
    busy.value = false;
  }
};

onMounted(bootstrap);
</script>
