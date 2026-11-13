<script setup lang="ts">
import {onMounted,ref} from 'vue'
import {api,APIError,loadSession,session,list} from '../api'
import {clearLikeSessions} from '../postLike'
const props=defineProps<{userId:string}>(),enabled=ref(true),followed=ref(false),busy=ref(true),error=ref('')
function report(e:unknown){if(e instanceof APIError&&e.status===401){session.authenticated=false;session.user=null;session.csrf_token='';clearLikeSessions();return}error.value=(e as Error).message}
onMounted(async()=>{try{await loadSession();if(session.authenticated){enabled.value=(await api('/site')).follows_enabled;let cursor='';do{const data=await api('/me/following?limit=50'+(cursor?'&cursor='+encodeURIComponent(cursor):''));followed.value=list(data).some(x=>String(x.id)===props.userId);if(followed.value||!data.has_more)break;cursor=data.next_cursor}while(cursor)}}catch(e){if(e instanceof APIError&&e.status===401)report(e)}finally{busy.value=false}})
async function toggle(){if(session.loading||!session.authenticated||busy.value)return;busy.value=true;try{await api(`/users/${props.userId}/follow`,{method:followed.value?'DELETE':'PUT'});followed.value=!followed.value}catch(e){report(e)}finally{busy.value=false}}
</script>
<template><div v-if="!session.loading&&session.authenticated&&session.user?.id!==userId"><p v-if="!enabled&&!followed" class="muted">站点目前关闭了新关注。</p><button v-else :disabled="busy" :aria-pressed="followed" :aria-label="followed?'取消关注作者':'关注作者'" :title="followed?'取消关注作者':'关注作者'" @click="toggle">{{followed?'已关注':'关注作者'}}</button><p v-if="error" role="alert" class="notice error">{{error}}</p></div></template>
