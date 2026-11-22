<script setup lang="ts">
import {t} from '../i18n'
defineProps<{busy:boolean}>()
defineEmits<{retry:[]}>()
</script>
<template>
 <p class="notice" role="status">{{t("暂未确认保存结果。当前输入已保留，请重试确认保存。")}} <button type="button" class="quiet" :disabled="busy" @click="$emit('retry')">{{t("重试确认保存")}}</button></p>
</template>
