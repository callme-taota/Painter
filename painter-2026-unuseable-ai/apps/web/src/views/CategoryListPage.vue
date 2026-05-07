<template>
  <div class="apple-root">
    <div class="apple-mesh-bg list-mesh-bg" aria-hidden="true">
      <div class="apple-blob apple-blob--violet" />
      <div class="apple-blob apple-blob--cyan" />
    </div>
    <section style="position: relative; z-index: 1">
    <div class="page-header">
      <h1>分类</h1>
    </div>
    <div class="tags-toolbar">
      <button class="btn btn-ghost" type="button" :disabled="loading" @click="load">
        {{ loading ? "加载中..." : "刷新" }}
      </button>
      <span v-if="hint" class="hint">{{ hint }}</span>
      <span v-if="error" class="error">{{ error }}</span>
    </div>
    <p v-if="!loading && !items.length" class="empty tags-toolbar">暂无分类</p>
    <div v-else class="categories-cont">
      <RouterLink
        v-for="item in items"
        :key="item.categoryId"
        class="category-item"
        :to="{ path: '/articlelist', query: { type: '1', id: String(item.categoryId) } }"
      >
        <span class="category-item-name">{{ item.name }}</span>
        <span class="category-item-count">·</span>
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

type CategoryItem = { categoryId: number; name: string };
const items = ref<CategoryItem[]>([]);
const loading = ref(false);
const hint = ref("");
const error = ref("");
const client = createClient({ baseUrl: "http://localhost:18080/api/v1" });

const load = async () => {
  try {
    loading.value = true;
    error.value = "";
    const res = await client.listCategories();
    items.value = res.data.items as CategoryItem[];
    hint.value = `共 ${items.value.length} 个分类`;
  } catch (e) {
    error.value = (e as Error).message;
  } finally {
    loading.value = false;
  }
};

onMounted(load);
</script>
