<script setup lang="ts">
//base
import { ref, onMounted } from 'vue';
import { NTabs, NTabPane, useMessage } from 'naive-ui';
import { useRouter } from 'vue-router';
import AdminBase from './admin_base.vue';
import AdminTag from './admin_tag.vue';
import AdminCategory from './admin_category.vue';
import AdminUser from './admin_user.vue';
//api
import { IsAdmin } from '@/apis/api_common';
//store
const Router = useRouter()
const Message = useMessage()
//hook
onMounted(async () => {
    let res = await IsAdmin()
    if (res.data.isAdmin) {
    } else {
        Message.error("无管理权限！")
        Router.push({ path: '/' })
    }
})
</script>
<template>
    <div class="page-header">
        <h1>
            站点管理
        </h1>
    </div>
    <div class="page-cont">
        <div>
            <n-tabs type="line" animated>
                <n-tab-pane name="base" tab="基础信息">
                    <AdminBase />
                </n-tab-pane>
                <n-tab-pane name="tag" tab="标签设置">
                    <AdminTag />
                </n-tab-pane>
                <n-tab-pane name="category" tab="类别设置">
                    <AdminCategory />
                </n-tab-pane>
                <n-tab-pane name="user" tab="用户设置">
                    <AdminUser />
                </n-tab-pane>
            </n-tabs>
        </div>
    </div>
</template>
<style></style>