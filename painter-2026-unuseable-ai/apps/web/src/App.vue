<template>
  <main class="app-shell">
    <header class="header">
      <nav class="nav">
        <RouterLink v-for="item in navItems" :key="item.to" :to="item.to" class="nav-link" active-class="active-link">
          {{ item.label }}
        </RouterLink>
      </nav>
      <div class="right">
        <span v-if="isLoggedIn" class="user">用户：{{ userId }}</span>
        <RouterLink v-if="isLoggedIn" to="/me" class="quick">个人中心</RouterLink>
        <RouterLink v-else to="/login" class="quick">去登录</RouterLink>
      </div>
    </header>
    <section class="content">
      <RouterView />
    </section>
  </main>
</template>

<script setup lang="ts">
import { computed } from "vue";

const token = localStorage.getItem("painter_token") ?? "";
const userId = localStorage.getItem("painter_user_id") ?? "-";
const isLoggedIn = computed(() => !!token);
const navItems = [
  { to: "/", label: "首页" },
  { to: "/articles", label: "文章" },
  { to: "/tags", label: "标签" },
  { to: "/categories", label: "分类" },
  { to: "/me", label: "我的主页" },
];
</script>

<style scoped>
.app-shell { min-height: 100vh; background: var(--home-background); color: var(--color); }
.header {
  width: calc(100% - 100px);
  height: 60px;
  position: fixed;
  top: 0;
  left: 0;
  z-index: 2;
  padding: 0 50px;
  display: flex;
  justify-content: space-between;
  align-items: center;
  background: var(--header-background);
  backdrop-filter: blur(12px);
  border-bottom: 1px solid #00000008;
}
.nav { display: flex; gap: 8px; flex-wrap: wrap; }
.nav-link, .quick {
  text-decoration: none;
  border-radius: 14px;
  height: 38px;
  padding: 0 12px;
  display: flex;
  align-items: center;
  transition: 0.2s;
}
.nav-link:hover, .quick:hover { background: var(--base-hover-background); color: var(--btn-hover-color); }
.active-link { background: var(--base-hover-background); color: var(--btn-hover-color); }
.right { display: flex; gap: 8px; align-items: center; }
.user { font-size: 13px; }
.content { margin-top: 60px; min-height: calc(100vh - 60px); }
@media (max-width: 1000px) {
  .header { width: calc(100% - 40px); padding: 0 20px; }
}
</style>
