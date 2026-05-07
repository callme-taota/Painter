<template>
  <section class="page-cont">
    <div class="page-header" style="padding-left: 0; padding-right: 0">
      <h1>管理后台总览</h1>
    </div>
    <div class="panel">
      <div class="admin-actions">
        <button class="btn" type="button" :disabled="loading" @click="load">
          {{ loading ? "加载中..." : "加载统计" }}
        </button>
        <span v-if="hint" class="hint">{{ hint }}</span>
      </div>
      <p v-if="error" class="error">{{ error }}</p>
      <pre v-if="overview" class="raw-block">{{ overview }}</pre>
      <p v-else-if="!loading" class="empty">暂无总览数据</p>
      <ul v-if="history.length" class="admin-list">
        <li v-for="item in history" :key="item.date">
          <span>{{ item.date }}</span>
          <span style="font-size: 13px; color: #64748b">
            qps={{ item.qps }} · p95={{ item.p95ms }}ms · visits={{ item.visits }}
          </span>
        </li>
      </ul>
    </div>
  </section>
</template>

<script setup lang="ts">
import { onMounted, ref } from "vue";
import { createClient } from "@painter/sdk-ts";

const overview = ref("");
const history = ref<Array<{ date: string; qps: number; p95ms: number; visits: number }>>([]);
const loading = ref(false);
const error = ref("");
const hint = ref("");
const client = createClient({ baseUrl: "http://localhost:18080/api/v1" });

const load = async () => {
  try {
    loading.value = true;
    error.value = "";
    hint.value = "";
    const [ov, hs] = await Promise.all([client.getAnalyticsOverview(), client.getAnalyticsHistory(7)]);
    overview.value = JSON.stringify(ov.data, null, 2);
    history.value = hs.data.items;
    hint.value = "统计已更新";
  } catch (e) {
    error.value = (e as Error).message;
  } finally {
    loading.value = false;
  }
};

onMounted(load);
</script>
