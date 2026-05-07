<template>
  <section class="page-cont">
    <div class="page-header" style="padding-left: 0; padding-right: 0">
      <h1>分类管理</h1>
    </div>
    <div class="panel">
      <div class="admin-actions">
        <input v-model="name" placeholder="新分类名" style="flex: 1; min-width: 160px; max-width: 320px" />
        <button class="btn" type="button" :disabled="loading" @click="create">{{ loading ? "处理中..." : "创建" }}</button>
      </div>
      <p v-if="hint" class="hint">{{ hint }}</p>
      <p v-if="error" class="error">{{ error }}</p>
      <ul class="admin-list">
        <li v-for="item in items" :key="item.categoryId">
          <span>{{ item.name }}</span>
        </li>
      </ul>
    </div>
  </section>
</template>

<script setup lang="ts">
import { onMounted, ref } from "vue";
import { createClient } from "@painter/sdk-ts";

type CategoryItem = { categoryId: number; name: string };
const name = ref("");
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
    items.value = (res.data.items as CategoryItem[]) ?? [];
    hint.value = `已加载 ${items.value.length} 个分类`;
  } catch (e) {
    error.value = (e as Error).message;
  } finally {
    loading.value = false;
  }
};

const create = async () => {
  if (!name.value.trim()) return;
  try {
    loading.value = true;
    error.value = "";
    await client.createCategory({ name: name.value });
    name.value = "";
    hint.value = "分类创建成功";
    await load();
  } catch (e) {
    error.value = (e as Error).message;
  } finally {
    loading.value = false;
  }
};

onMounted(load);
</script>
