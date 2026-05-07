<template>
  <main class="app-shell">
    <header class="header">
      <nav class="nav">
        <RouterLink v-for="item in navItems" :key="item.to" :to="item.to" class="nav-link" active-class="active-link">
          {{ item.label }}
        </RouterLink>
      </nav>
      <div class="right">
        <RouterLink to="/settings/site" class="quick">快速设置</RouterLink>
        <RouterLink to="/" class="quick">返回总览</RouterLink>
      </div>
    </header>
    <section class="content">
      <RouterView />
    </section>
  </main>
</template>

<script setup lang="ts">
const navItems = [
  { to: "/", label: "总览" },
  { to: "/settings/site", label: "站点配置" },
  { to: "/content/tags", label: "标签管理" },
  { to: "/content/categories", label: "分类管理" },
  { to: "/users/permissions", label: "用户权限" },
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
.right { display: flex; gap: 8px; }
.content { margin-top: 60px; min-height: calc(100vh - 60px); padding: 0; }
@media (max-width: 1000px) {
  .header { width: calc(100% - 40px); padding: 0 20px; }
}
</style>
