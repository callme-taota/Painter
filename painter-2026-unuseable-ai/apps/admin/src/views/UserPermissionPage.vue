<template>
  <section class="page-cont">
    <div class="page-header" style="padding-left: 0; padding-right: 0">
      <h1>用户权限</h1>
    </div>
    <div class="panel">
      <div class="admin-actions">
        <button class="btn" type="button" :disabled="loading" @click="load">{{ loading ? "加载中..." : "加载用户" }}</button>
        <span v-if="hint" class="hint">{{ hint }}</span>
      </div>
      <p v-if="error" class="error">{{ error }}</p>
      <ul class="admin-list">
        <li v-for="item in users" :key="item.userId">
          <span>{{ item.userName }}（组 {{ item.group }}）</span>
          <div class="admin-actions">
            <button class="btn small" type="button" :disabled="loading" @click="setGroup(item.userId, 1)">管理员</button>
            <button class="btn small" type="button" :disabled="loading" @click="setGroup(item.userId, 3)">普通用户</button>
          </div>
        </li>
      </ul>
      <pre v-if="result" class="raw-block">{{ result }}</pre>
    </div>
  </section>
</template>

<script setup lang="ts">
import { ref } from "vue";
import { createClient } from "@painter/sdk-ts";

type UserItem = { userId: string; userName: string; group: number };
const users = ref<UserItem[]>([]);
const result = ref("");
const hint = ref("");
const error = ref("");
const loading = ref(false);
const client = createClient({ baseUrl: "http://localhost:18080/api/v1" });

const load = async () => {
  try {
    loading.value = true;
    error.value = "";
    const res = await client.getUserList();
    users.value = (res.data.items as UserItem[]) ?? [];
    hint.value = `已加载 ${users.value.length} 个用户`;
  } catch (e) {
    error.value = (e as Error).message;
  } finally {
    loading.value = false;
  }
};

const setGroup = async (userId: string, group: number) => {
  try {
    error.value = "";
    const res = await client.setUserPermission({ userId, group });
    result.value = JSON.stringify(res.data, null, 2);
    hint.value = "权限已更新";
    await load();
  } catch (e) {
    error.value = (e as Error).message;
  }
};
</script>
