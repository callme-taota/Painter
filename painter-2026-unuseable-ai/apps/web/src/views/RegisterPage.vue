<template>
  <div class="apple-root">
    <div class="apple-mesh-bg auth-mesh-bg" aria-hidden="true">
      <div class="apple-blob apple-blob--rose" />
      <div class="apple-blob apple-blob--cyan" />
    </div>
    <section class="page-cont auth-page-cont" style="position: relative; z-index: 1">
    <div class="login-cont">
      <div class="login-left">
        <div class="login-welcome" style="margin-bottom: 16px">Painter 注册</div>
        <ol class="register-steps">
          <li>填写用户 ID、用户名与密码</li>
          <li>邮箱可选，用于后续找回与通知</li>
          <li>提交后前往登录页完成会话建立</li>
        </ol>
      </div>
      <div class="login-right">
        <div class="painter-login">创建账号</div>
        <div class="login-account-text">用户 ID（必填）</div>
        <input v-model="userId" placeholder="用户ID" autocomplete="off" />
        <div class="login-account-text">用户名（必填）</div>
        <input v-model="userName" placeholder="用户名" autocomplete="username" />
        <div class="login-account-text">邮箱（可选）</div>
        <input v-model="email" placeholder="邮箱" type="email" autocomplete="email" />
        <div class="login-account-text">密码（必填）</div>
        <input v-model="passwd" type="password" placeholder="密码" autocomplete="new-password" />
        <p v-if="hint" class="hint">{{ hint }}</p>
        <p v-if="error" class="error">{{ error }}</p>
        <pre v-if="result" class="raw-block">{{ result }}</pre>
        <div class="login-btn-cont">
          <RouterLink class="login-link" to="/login">已有账号</RouterLink>
          <button class="btn" type="button" :disabled="loading" @click="submit">
            {{ loading ? "提交中..." : "注册" }}
          </button>
        </div>
      </div>
    </div>
  </section>
  </div>
</template>

<style scoped>
.auth-mesh-bg .apple-blob {
  opacity: 0.42;
}
</style>

<script setup lang="ts">
import { ref } from "vue";
import { createClient } from "@painter/sdk-ts";

const userId = ref("");
const userName = ref("");
const email = ref("");
const passwd = ref("");
const result = ref("");
const hint = ref("");
const error = ref("");
const loading = ref(false);
const client = createClient({ baseUrl: "http://localhost:18080/api/v1" });

const submit = async () => {
  if (!userId.value.trim() || !userName.value.trim() || !passwd.value.trim()) {
    error.value = "用户ID、用户名、密码为必填项";
    return;
  }
  try {
    loading.value = true;
    error.value = "";
    const res = await client.identityRegister({
      userId: userId.value,
      userName: userName.value,
      email: email.value,
      passwd: passwd.value,
    });
    hint.value = "注册成功，请前往登录";
    result.value = JSON.stringify(res.data, null, 2);
  } catch (e) {
    error.value = (e as Error).message;
  } finally {
    loading.value = false;
  }
};
</script>
