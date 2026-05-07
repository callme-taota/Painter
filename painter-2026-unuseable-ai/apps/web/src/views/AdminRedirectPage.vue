<template>
  <div class="apple-root">
    <section class="apple-section apple-hero-min-h" style="min-height: calc(100vh - 120px)">
      <div class="apple-glass apple-profile-card" style="max-width: 520px">
        <h1 class="apple-display-name" style="margin-top: 0">后台入口</h1>
        <p class="apple-hero-sub">独立 Admin 应用已就绪，点击下方按钮在新标签打开。</p>
        <div class="apple-row-actions">
          <a class="apple-btn-primary" :href="adminUrl" target="_blank" rel="noreferrer" style="text-align: center">打开 Admin</a>
        </div>
        <p class="apple-kv" style="margin-top: 16px">{{ adminUrl }}</p>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
const adminUrl = "http://localhost:5174/";
</script>
