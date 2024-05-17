<script setup lang="ts">
import { storeToRefs } from 'pinia';
import { useInfoStore } from '@/stores/info';
const InfoStore = useInfoStore()
const { icp_code } = storeToRefs(InfoStore);
</script>
<template>
    <div class="layout-footer-cont">
        <div class="footer-info">@ 2019 - 2024 · <a href="http://blog.callmetaota.fun/">逃塔小站</a>
        </div>
        <div>{{ icp_code }}</div>
        <div class="footer-info">Power by Taota</div>
    </div>
</template>
<style>
.layout-footer-cont {
    display: flex;
    justify-content: space-between;
    padding:20px 40px;
    height: 40px;
    line-height: 40px;
    background: var(--layout--bottom-background);
}
</style>