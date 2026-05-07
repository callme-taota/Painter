<template>
  <div class="apple-root">
    <div class="apple-mesh-bg auth-mesh-bg" aria-hidden="true">
      <div class="apple-blob apple-blob--cyan" />
      <div class="apple-blob apple-blob--violet" />
    </div>
    <section class="page-cont auth-page-cont" style="position: relative; z-index: 1">
    <div class="login-cont">
      <div class="login-left">
        <div class="login-left-line">
          <div class="login-left-logo" aria-hidden="true">P</div>
          <div>
            <div class="login-welcome">欢迎来到 Painter</div>
            <div class="login-left-line" style="margin-top: 8px; font-size: 15px; opacity: 0.9">
              博客，但不止是博客。
            </div>
          </div>
        </div>
      </div>
      <div class="login-right">
        <div class="painter-login">Painter 登录</div>
        <div class="login-account-text">用户名</div>
        <input v-model="username" placeholder="用户名" autocomplete="username" />
        <div class="login-account-text">密码</div>
        <input v-model="password" type="password" placeholder="密码" autocomplete="current-password" />
        <p v-if="hint" class="hint">{{ hint }}</p>
        <p v-if="error" class="error">{{ error }}</p>
        <pre v-if="result" class="raw-block">{{ result }}</pre>
        <div class="login-btn-cont">
          <RouterLink class="login-link" to="/register">去注册</RouterLink>
          <button class="btn" type="button" @click="submit">登录</button>
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

const username = ref("admin");
const password = ref("admin");
const result = ref("");
const hint = ref("");
const error = ref("");
const client = createClient({ baseUrl: "http://localhost:18080/api/v1" });

const submit = async () => {
  try {
    error.value = "";
    const res = await client.identityLogin({ username: username.value, password: password.value });
    localStorage.setItem("painter_token", res.data.accessToken);
    localStorage.setItem("painter_user_id", res.data.userId);
    hint.value = "登录成功，后续页面会携带会话信息。";
    result.value = JSON.stringify(res.data, null, 2);
  } catch (e) {
    error.value = (e as Error).message;
    hint.value = "";
  }
};
</script>
