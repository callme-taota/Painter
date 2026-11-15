<script setup lang="ts">
import {t} from "../i18n"

import {onMounted,ref} from 'vue'
import {api,APIError,loadSession,session} from '../api'
import {clearLikeSessions} from '../postLike'
import {toast} from '../toast'
const props=defineProps<{userId:string}>(),enabled=ref(true),followed=ref(false),busy=ref(true),ready=ref(false),error=ref('')
function report(e:unknown,interactive=false){if(e instanceof APIError&&e.status===401){clearLikeSessions();if(interactive)toast.error(t("登录已失效，请重新登录。"),{id:'session-expired'});return}if(interactive)toast.error((e as Error).message,{id:'follow:'+props.userId});else error.value=(e as Error).message}
async function load(){busy.value=true;error.value='';try{await loadSession();if(session.authenticated&&String(session.user?.id)!==props.userId){const data=await api(`/users/${props.userId}/follow-state`);enabled.value=data.follows_enabled;followed.value=data.followed;ready.value=true}}catch(e){report(e)}finally{busy.value=false}}
onMounted(load)
async function toggle(){if(session.loading||!session.authenticated||busy.value||!ready.value)return;busy.value=true;error.value='';try{await api(`/users/${props.userId}/follow`,{method:followed.value?'DELETE':'PUT'});followed.value=!followed.value;toast.success(followed.value?t("已关注此人"):t("已取消关注"),{id:'follow:'+props.userId})}catch(e){report(e,true)}finally{busy.value=false}}
</script>
<template><div v-if="!session.loading&&session.authenticated&&String(session.user?.id)!==userId"><p v-if="!enabled&&!followed" class="muted">{{t("站点目前关闭了新关注。")}}</p><button v-else :disabled="busy||!ready" :aria-pressed="followed" :aria-label="followed?t('取消关注'):t('关注此人')" :title="followed?t('取消关注'):t('关注此人')" @click="toggle">{{busy&&!ready?t("载入中…"):followed?t("已关注"):t("关注")}}</button><p v-if="error" role="alert" class="notice error">{{error}} <button v-if="!ready" class="quiet" type="button" :disabled="busy" @click="load">{{t("重试")}}</button></p></div></template>
