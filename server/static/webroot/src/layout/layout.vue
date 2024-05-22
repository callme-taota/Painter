<script setup lang="ts">
import { onMounted } from 'vue'
import { RouterView, useRouter, useRoute } from 'vue-router'
import { useMessage } from "naive-ui"
import LayoutHeader from "./components/layout-header.vue"
import LayoutFooter from "./components/layout-footer.vue"
import Search from './components/search.vue'

const Message = useMessage()

onMounted(() => {
    document.addEventListener('keydown', function (event) {
        if (event.key === 'F12') {
            Message.error('调皮！ 请遵守Apache-2.0 协议')
        }
    });
})
</script>
<template>
    <div id="base">
        <Search />
        <LayoutHeader />
        <div class="layout-router-cont">
            <RouterView />
        </div>
        <LayoutFooter />
    </div>
</template>
<style>
#base {
    width: 100%;
    min-height: 100%;
    position: absolute;
    background-color: var(--home-background);
}

.layout-router-cont {
    margin-top: 60px;
    min-height: calc(100vh - 60px);
}
</style>