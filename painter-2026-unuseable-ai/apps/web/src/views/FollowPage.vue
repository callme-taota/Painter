<template>
  <div class="apple-root">
    <div class="apple-mesh-bg">
      <div class="apple-blob apple-blob--cyan apple-parallax-layer" :style="blobA" />
      <div class="apple-blob apple-blob--violet apple-parallax-layer" :style="blobB" />
    </div>
    <section class="apple-section" style="padding-top: 36px">
      <div class="page-header" style="padding: 0 0 16px">
        <h1 style="margin: 0; font-size: 28px; font-weight: 700">关注列表</h1>
      </div>
      <div class="apple-glass apple-parallax-layer" :style="cardLift" style="padding: 24px">
        <div class="apple-row-actions" style="justify-content: flex-start; margin-bottom: 16px">
          <button type="button" class="apple-btn-secondary" @click="loadFollowings">我关注的人</button>
          <button type="button" class="apple-btn-secondary" @click="loadFollowers">关注我的人</button>
        </div>
        <p v-if="hint" class="apple-hint">{{ hint }}</p>
        <p v-if="error" class="apple-error">{{ error }}</p>
        <ul class="follow-ul">
          <li v-for="item in items" :key="item.userId" class="follow-li">{{ item.nickname || item.userId }}</li>
        </ul>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { useRoute } from "vue-router";
import { createClient } from "@painter/sdk-ts";
import { useWindowScroll } from "../composables/useWindowScroll";

const route = useRoute();
const { scrollY } = useWindowScroll();

const userId = String(route.params.id || localStorage.getItem("painter_user_id") || "u-admin");
const token = localStorage.getItem("painter_token") ?? "";
const items = ref<Array<{ userId: string; nickname: string }>>([]);
const hint = ref("");
const error = ref("");

const client = createClient({
  baseUrl: "http://localhost:18080/api/v1",
  headers: token ? { Authorization: `Bearer ${token}`, "X-User-Id": userId } : undefined,
});

const scaleBoost = computed(() => 1 + scrollY.value * 0.00006);
const blobA = computed(() => ({
  transform: `translate3d(0, ${scrollY.value * 0.08}px, 0) scale(${scaleBoost.value})`,
}));
const blobB = computed(() => ({
  transform: `translate3d(0, ${scrollY.value * -0.05}px, 0) scale(${scaleBoost.value})`,
}));
const cardLift = computed(() => ({
  transform: `translate3d(0, ${scrollY.value * -0.02}px, 0)`,
}));

const loadFollowings = async () => {
  try {
    error.value = "";
    const res = await client.listFollowings(userId);
    items.value = res.data.items;
    hint.value = "已加载关注列表";
  } catch (e) {
    error.value = (e as Error).message;
  }
};

const loadFollowers = async () => {
  try {
    error.value = "";
    const res = await client.listFollowers(userId);
    items.value = res.data.items;
    hint.value = "已加载粉丝列表";
  } catch (e) {
    error.value = (e as Error).message;
  }
};

onMounted(loadFollowings);
</script>

<style scoped>
.follow-ul {
  list-style: none;
  padding: 0;
  margin: 12px 0 0;
  display: grid;
  gap: 10px;
}
.follow-li {
  padding: 14px 16px;
  border-radius: 14px;
  background: rgba(255, 255, 255, 0.55);
  border: 1px solid rgba(0, 0, 0, 0.06);
  font-weight: 500;
}
</style>
