<script setup lang="ts">
import {onMounted,ref} from 'vue'
import {api,loadSession,session,list} from '../api'
const props=defineProps<{userId:string}>(),enabled=ref(true),followed=ref(false),busy=ref(true),error=ref('')
onMounted(async()=>{try{await loadSession();enabled.value=(await api('/site')).follows_enabled;if(session.authenticated){let cursor='';do{const data=await api('/me/following?limit=50'+(cursor?'&cursor='+encodeURIComponent(cursor):''));followed.value=list(data).some(x=>String(x.id)===props.userId);if(followed.value||!data.has_more)break;cursor=data.next_cursor}while(cursor)}}catch{}finally{busy.value=false}})
async function toggle(){if(!session.authenticated){location.href='/login?return_to='+encodeURIComponent(location.pathname+location.search);return}busy.value=true;try{await api(`/users/${props.userId}/follow`,{method:followed.value?'DELETE':'PUT'});followed.value=!followed.value}catch(e){error.value=(e as Error).message}finally{busy.value=false}}
</script>
<template><div v-if="session.user?.id!==userId"><p v-if="!enabled&&!followed" class="muted">站点目前关闭了新关注。</p><button v-else :disabled="busy" :aria-pressed="followed" :aria-label="followed?'取消关注作者':'关注作者'" :title="followed?'取消关注作者':'关注作者'" @click="toggle">{{followed?'已关注':'关注作者'}}</button><p v-if="error" role="alert" class="notice error">{{error}}</p></div></template>
