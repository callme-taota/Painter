<template>
  <section class="page-cont">
    <div class="page-header" style="padding-left: 0; padding-right: 0">
      <h1>站点配置</h1>
    </div>
    <div class="panel">
      <h3 style="margin: 0; font-size: 16px">常规选项</h3>
      <div class="input-item">
        <div class="input-left">站点名称</div>
        <div class="input-right">
          <input v-model="form.siteName" placeholder="站点名称" />
        </div>
      </div>
      <div class="input-item">
        <div class="input-left">Github</div>
        <div class="input-right">
          <input v-model="form.github" placeholder="Github 仓库或主页链接" />
        </div>
      </div>
      <div class="input-item">
        <div class="input-left">允许注册</div>
        <div class="input-right">
          <label style="display: flex; align-items: center; gap: 8px">
            <input v-model="form.canRegister" type="checkbox" />
            <span style="font-size: 14px">开启用户自助注册</span>
          </label>
        </div>
      </div>
      <div class="admin-actions" style="margin-top: 8px">
        <button class="btn" type="button" :disabled="loading" @click="load">{{ loading ? "读取中..." : "读取" }}</button>
        <button class="btn" type="button" :disabled="loading" @click="save">{{ loading ? "保存中..." : "保存修改" }}</button>
      </div>
      <p v-if="hint" class="hint">{{ hint }}</p>
      <p v-if="error" class="error">{{ error }}</p>
      <pre v-if="data" class="raw-block">{{ data }}</pre>
    </div>
  </section>
</template>

<script setup lang="ts">
import { onMounted, ref } from "vue";
import { createClient } from "@painter/sdk-ts";

const client = createClient({ baseUrl: "http://localhost:18080/api/v1" });
const data = ref("");
const loading = ref(false);
const error = ref("");
const hint = ref("");
const form = ref({
  siteName: "Painter 2026",
  github: "",
  canRegister: true,
});

const load = async () => {
  try {
    loading.value = true;
    error.value = "";
    const res = await client.getAdminSettings();
    form.value.siteName = res.data.siteName ?? "Painter 2026";
    form.value.github = res.data.github ?? "";
    form.value.canRegister = !!res.data.canRegister;
    data.value = JSON.stringify(res.data, null, 2);
    hint.value = "读取成功";
  } catch (e) {
    error.value = (e as Error).message;
  } finally {
    loading.value = false;
  }
};

const save = async () => {
  try {
    loading.value = true;
    error.value = "";
    const res = await client.updateAdminSettings({
      siteName: form.value.siteName,
      github: form.value.github,
      canRegister: form.value.canRegister,
    });
    data.value = JSON.stringify(res.data, null, 2);
    hint.value = "保存成功";
  } catch (e) {
    error.value = (e as Error).message;
  } finally {
    loading.value = false;
  }
};

onMounted(load);
</script>
