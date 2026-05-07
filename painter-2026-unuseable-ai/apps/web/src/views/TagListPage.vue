<template>
  <div class="apple-root">
    <div class="apple-mesh-bg list-mesh-bg" aria-hidden="true">
      <div class="apple-blob apple-blob--cyan" />
      <div class="apple-blob apple-blob--rose" />
    </div>
    <section style="position: relative; z-index: 1">
    <div class="page-header">
      <h1>标签</h1>
    </div>
    <div class="tags-toolbar">
      <button class="btn btn-ghost" type="button" :disabled="loading" @click="load">
        {{ loading ? "加载中..." : "刷新" }}
      </button>
      <span v-if="hint" class="hint">{{ hint }}</span>
      <span v-if="error" class="error">{{ error }}</span>
    </div>
    <p v-if="!loading && !items.length" class="empty tags-toolbar">暂无标签</p>
    <div v-else class="tags-cont">
      <RouterLink
        v-for="item in items"
        :key="item.tagId"
        class="tag-item"
        :to="{ path: '/articlelist', query: { type: '2', id: String(item.tagId) } }"
      >
        <span class="tag-item-name">{{ item.name }}</span>
        <span class="tag-item-count">{{ item.count }}</span>
      </RouterLink>
    </div>
  </section>
  </div>
</template>

<style scoped>
.list-mesh-bg .apple-blob {
  opacity: 0.35;
}
</style>

<script setup lang="ts">
import { onMounted, ref } from "vue";
import { createClient } from "@painter/sdk-ts";

type TagItem = { tagId: number; name: string; count: number };
const items = ref<TagItem[]>([]);
const loading = ref(false);
const hint = ref("");
const error = ref("");
const client = createClient({ baseUrl: "http://localhost:18080/api/v1" });

const load = async () => {
  try {
    loading.value = true;
    error.value = "";
    const res = await client.listTags();
    items.value = res.data.items as TagItem[];
    hint.value = `共 ${items.value.length} 个标签`;
  } catch (e) {
    error.value = (e as Error).message;
  } finally {
    loading.value = false;
  }
};

onMounted(load);
</script>
