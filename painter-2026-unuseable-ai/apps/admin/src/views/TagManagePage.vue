<template>
  <section class="page-cont">
    <div class="page-header" style="padding-left: 0; padding-right: 0">
      <h1>标签管理</h1>
    </div>
    <div class="panel">
      <div class="admin-actions">
        <input v-model="name" placeholder="新标签名" style="flex: 1; min-width: 160px; max-width: 320px" />
        <button class="btn" type="button" :disabled="loading" @click="create">{{ loading ? "处理中..." : "创建" }}</button>
      </div>
      <p v-if="hint" class="hint">{{ hint }}</p>
      <p v-if="error" class="error">{{ error }}</p>
      <ul class="admin-list">
        <li v-for="item in items" :key="item.tagId">
          <span>{{ item.name }}（{{ item.count }}）</span>
          <button class="btn small danger" type="button" :disabled="loading" @click="remove(item.tagId)">删除</button>
        </li>
      </ul>
    </div>
  </section>
</template>

<script setup lang="ts">
import { onMounted, ref } from "vue";
import { createClient } from "@painter/sdk-ts";

type TagItem = { tagId: number; name: string; count: number };
const name = ref("");
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
    items.value = (res.data.items as TagItem[]) ?? [];
    hint.value = `已加载 ${items.value.length} 个标签`;
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
    await client.createTag({ name: name.value });
    name.value = "";
    hint.value = "标签创建成功";
    await load();
  } catch (e) {
    error.value = (e as Error).message;
  } finally {
    loading.value = false;
  }
};

const remove = async (tagId: number) => {
  if (!window.confirm("确认删除该标签？")) return;
  try {
    loading.value = true;
    error.value = "";
    await client.deleteTag({ tagId });
    hint.value = "标签已删除";
    await load();
  } catch (e) {
    error.value = (e as Error).message;
  } finally {
    loading.value = false;
  }
};

onMounted(load);
</script>
