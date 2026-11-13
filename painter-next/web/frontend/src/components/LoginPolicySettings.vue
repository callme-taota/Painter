<script setup lang="ts">
import {computed,ref,watch} from 'vue'
import {api,body,APIError,type Row} from '../api'
const props=defineProps<{settings:Row}>(),emit=defineEmits<{saved:[Row]}>()
const registration=ref(false),external=ref(true),busy=ref(false),error=ref(''),message=ref(''),conflict=ref(false)
watch(()=>props.settings.version,()=>{registration.value=!!props.settings.registration_enabled;external.value=props.settings.external_login_enabled!==false;conflict.value=false},{immediate:true})
const changed=computed(()=>registration.value!==!!props.settings.registration_enabled||external.value!==(props.settings.external_login_enabled!==false))
async function save(){
 if(busy.value||conflict.value)return
 busy.value=true;error.value='';message.value=''
 try{const latest=await api('/admin/settings');if(latest.version!==props.settings.version)throw new APIError(409,'CONFLICT','设置已在其他页面更新，请重新加载后再保存。');const data:Row={...latest,registration_enabled:registration.value,external_login_enabled:external.value};delete data.smtp_password_configured;delete data.smtp_password;const result=await api('/admin/settings',{method:'PUT',version:props.settings.version,body:body(data)});emit('saved',result);message.value='账号访问策略已保存。'}catch(e){const failure=e as Error&{status?:number};error.value=failure.message;conflict.value=failure.status===409}finally{busy.value=false}
}
</script>
<template>
 <form class="panel login-policy" aria-labelledby="login-policy-title" @submit.prevent="save">
  <h2 id="login-policy-title">账号访问</h2><p class="muted">分别控制新账号注册与已有外部账号登录。管理员始终可以登录维护，公开文章不受影响。</p>
  <p v-if="error" class="notice error" role="alert">{{error}} <a v-if="conflict" href="/studio/settings#security">重新加载最新设置</a></p><p v-if="message" class="notice success" role="status">{{message}}</p>
  <fieldset :disabled="busy" class="policy-options"><legend class="sr-only">注册与登录开关</legend>
   <label class="policy-row"><input v-model="external" type="checkbox"><span><strong>允许外部账号登录</strong><small class="muted">包括已有读者、创作者及接入应用。关闭后，这些账号的现有会话和应用授权也会失效。</small></span></label>
   <label class="policy-row"><input v-model="registration" type="checkbox"><span><strong>允许新账号注册</strong><small class="muted">包括本地注册与首次使用 GitHub 创建账号。新账号为读者，管理员权限单独分配。</small></span></label>
  </fieldset>
  <p v-if="!external" class="muted">当前将仅允许管理员登录{{registration?'，新账号注册也会暂停':''}}。</p>
  <div class="form-actions"><button class="primary" :disabled="busy||!changed||conflict">{{busy?'正在保存…':'保存账号访问策略'}}</button><a href="/me#account-security-title">管理我的两步验证</a></div>
 </form>
</template>
<style scoped>
.login-policy>p{max-width:70ch}.policy-options{border:0;margin:24px 0;padding:0;display:grid;gap:24px;min-width:0}.policy-row{display:flex;align-items:flex-start;gap:12px;cursor:pointer}.policy-row input{flex-shrink:0;margin-top:2px}.policy-row span{display:grid;gap:5px}.policy-row strong{font-size:14px;font-weight:500}.policy-row small{font-size:13px;line-height:1.6;max-width:68ch}
</style>
