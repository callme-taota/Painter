<script setup lang="ts">
import {computed,onMounted,reactive,ref} from 'vue'
import {api,body} from '../api'

type Settings={version:number;enabled:boolean;client_id:string;client_secret_configured:boolean;source:'environment'|'database';callback_url:string}
const settings=reactive<Settings>({version:0,enabled:false,client_id:'',client_secret_configured:false,source:'environment',callback_url:''})
const secret=ref(''),loading=ref(true),busy=ref(false),loaded=ref(false),activeEnabled=ref(false),error=ref(''),message=ref(''),conflict=ref(false)
const status=computed(()=>activeEnabled.value?'已启用 GitHub 登录':settings.client_secret_configured?'GitHub 登录已停用':'尚未配置 GitHub 登录')
async function load(){loading.value=true;error.value='';message.value='';try{Object.assign(settings,await api<Settings>('/admin/auth/github'));activeEnabled.value=settings.enabled;secret.value='';loaded.value=true;conflict.value=false}catch(e){error.value=(e as Error).message||'GitHub 登录配置未能加载，请重试。'}finally{loading.value=false}}
async function save(){if(busy.value||conflict.value)return;error.value='';message.value='';if(settings.enabled&&(!settings.client_id.trim()||(!secret.value&&!settings.client_secret_configured))){error.value='启用前，请填写 Client ID 和 Client Secret。';return}busy.value=true;try{const data:Record<string,unknown>={version:settings.version,enabled:settings.enabled,client_id:settings.client_id};if(secret.value)data.client_secret=secret.value;Object.assign(settings,await api<Settings>('/admin/auth/github',{method:'PUT',body:body(data)}));activeEnabled.value=settings.enabled;secret.value='';message.value=settings.enabled?'GitHub 登录已启用。可在登录页使用 GitHub 入口。':'GitHub 登录已停用，应用凭据已保留。'}catch(e){const failure=e as Error&{status?:number};conflict.value=failure.status===409;error.value=failure.message||'配置未能保存，请稍后重试。'}finally{busy.value=false}}
async function copyCallback(){error.value='';message.value='';try{await navigator.clipboard.writeText(settings.callback_url);message.value='回调地址已复制。'}catch{error.value='暂时无法自动复制，请选中回调地址后手动复制。'}}
onMounted(load)
</script>

<template>
 <section class="panel github-settings" aria-labelledby="github-settings-title" :aria-busy="loading||busy">
  <h2 id="github-settings-title">GitHub 登录</h2>
  <p class="muted">登记站点的 OAuth App 后，成员可使用 GitHub 身份登录。GitHub 身份与本地账号分别管理，不会按用户名或邮箱合并。</p>
  <p v-if="loading" role="status">正在加载登录配置…</p>
  <div v-if="error" class="notice error" role="alert">{{error}} <button v-if="!loaded||conflict" type="button" class="quiet" :disabled="loading||busy" @click="load">重新加载配置</button></div>
  <p v-if="message" class="notice success" role="status">{{message}}</p>
  <form v-if="loaded&&!loading" @submit.prevent="save">
   <p class="github-status"><strong>{{status}}</strong><span class="muted">{{settings.source==='environment'?'当前使用服务器环境配置。首次保存后使用此处配置。':'当前使用站点保存的配置。'}}</span></p>
   <fieldset :disabled="busy" class="github-fields">
    <legend class="sr-only">GitHub OAuth 应用配置</legend>
    <label class="check"><input v-model="settings.enabled" type="checkbox">启用 GitHub 登录</label>
    <div class="form-grid">
     <div class="full"><label for="github-callback-url">Authorization callback URL</label><div class="github-callback"><input id="github-callback-url" :value="settings.callback_url" readonly aria-describedby="github-callback-help" autocomplete="off"><button type="button" @click="copyCallback">复制地址</button></div><small id="github-callback-help" class="muted">在 GitHub 应用中填写这个完整地址。</small></div>
     <label>Client ID<input v-model="settings.client_id" maxlength="256" autocomplete="off" spellcheck="false" :required="settings.enabled"></label>
     <div class="github-secret-field"><label for="github-client-secret">Client Secret<input id="github-client-secret" v-model="secret" type="password" maxlength="4096" autocomplete="new-password" :placeholder="settings.client_secret_configured?'已配置，留空保持':'粘贴 GitHub 生成的密钥'" aria-describedby="github-secret-help"></label><small id="github-secret-help" class="muted">{{settings.client_secret_configured?'已保存的密钥不会显示。留空保留，填写新密钥则替换。':'密钥仅在保存时提交给本站服务器。'}}</small></div>
    </div>
    <p class="muted github-help">新 GitHub 身份仅在开放注册时创建读者账号；已有 GitHub 账号仍可登录。</p>
    <div class="form-actions"><button class="primary" :disabled="busy||conflict">{{busy?'正在保存…':'保存 GitHub 登录'}}</button><a href="https://github.com/settings/developers" target="_blank" rel="noopener noreferrer">登记 GitHub OAuth App</a></div>
   </fieldset>
  </form>
 </section>
</template>

<style scoped>
.github-secret-field{display:grid;align-content:start;gap:7px}.github-secret-field>label{display:grid;gap:7px}.github-secret-field small{font-size:.78rem;line-height:1.8}.github-settings>p{max-width:70ch}.github-fields{border:0;margin:0;padding:0;min-width:0}.github-fields>.check{margin-bottom:24px;line-height:var(--control-leading)}.github-status{display:flex;flex-direction:column;gap:4px;margin:24px 0}.github-callback{display:flex;align-items:stretch;gap:8px;min-width:0}.github-callback input{min-width:0;flex:1}.github-callback button{flex-shrink:0}.github-help{margin-top:16px}.github-settings .form-actions{flex-wrap:wrap;align-items:center;gap:16px}.github-settings .notice{overflow-wrap:anywhere}@media(max-width:540px){.github-callback{flex-direction:column}.github-callback button{align-self:flex-start}}
</style>
