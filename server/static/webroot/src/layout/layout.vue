<script setup lang="ts">
import LayoutHeader from "./components/layout-header.vue"
import LayoutFooter from "./components/layout-footer.vue"
import { RouterView, useRouter, useRoute } from 'vue-router'

</script>
<template>
    <div id="base">
        <LayoutHeader />
        <div class="layout-router-cont">
            <RouterView />
        </div>
        <LayoutFooter />
    </div>
</template>
<style>
#base{
    width: 100%;
    min-height: 100%;
    position: absolute;
    background-color: var(--home-background);
}
.layout-router-cont{
    margin-top: 60px;
    min-height: calc(100vh - 60px);
}
</style>