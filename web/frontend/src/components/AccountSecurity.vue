<script setup lang="ts">
import {colorValue} from "../color-tokens"
import {t} from "../i18n"

import {nextTick,onBeforeUnmount,onMounted,ref} from 'vue'
import {APIError,api,body,loadSession,session} from '../api'
import {toast} from '../toast'
import {confirmAction} from '../confirm'

type Security={mfa_enabled:boolean;mfa_enabled_at_ms:number|null;recovery_codes_remaining:number;has_local_password:boolean}
type Setup={secret:string;otpauth_uri:string;expires_at:string}
const emit=defineEmits<{updated:[number]}>()
const status=ref<Security|null>(null),setup=ref<Setup|null>(null),codes=ref<string[]>([])
const current=ref(''),code=ref(''),error=ref(''),recoveryNotice=ref(''),qrError=ref('')
const loading=ref(true),busy=ref(false),action=ref<'disable'|'regenerate'|''>(''),canvas=ref<HTMLCanvasElement>()
async function load(){loading.value=true;error.value='';try{status.value=await api<Security>('/me/security')}catch(e){error.value=(e as Error).message}finally{loading.value=false}}
function mutationError(e:unknown){if(e instanceof APIError&&[400,401,403,409,422].includes(e.status))error.value=e.message;else toast.error(e instanceof Error?e.message:t("操作失败，请稍后重试。"))}
async function refreshSession(){try{await loadSession();emit('updated',session.user.version)}catch{toast.warning(t("安全设置已更新，但登录状态未能刷新。请重新加载页面。"))}}
async function begin(){
 if(busy.value)return
 busy.value=true;error.value='';qrError.value=''
 try{
  setup.value=await api<Setup>('/me/mfa/setup',{method:'POST',body:body({current_password:current.value})})
  current.value='';code.value='';await nextTick()
  try{const QRCode=(await import('qrcode')).default;if(canvas.value)await QRCode.toCanvas(canvas.value,setup.value.otpauth_uri,{width:200,margin:2,errorCorrectionLevel:'M',color:{dark:colorValue('qr-ink'),light:colorValue('qr-paper')}})}catch{qrError.value=t("二维码未能生成，可手动输入下方密钥。")}
 }catch(e){mutationError(e)}finally{busy.value=false}
}
async function enable(){
 if(busy.value)return
 busy.value=true;error.value=''
 try{
  const data=await api<{recovery_codes:string[];csrf_token:string}>('/me/mfa/enable',{method:'POST',body:body({code:code.value})})
  session.csrf_token=data.csrf_token;setup.value=null;code.value='';codes.value=data.recovery_codes;recoveryNotice.value=t("两步验证已启用，其他设备已退出登录。请保存恢复码。")
  await refreshSession();await load();toast.success(t("两步验证已启用。"))
 }catch(e){mutationError(e)}finally{busy.value=false}
}
async function manage(){
 if(busy.value||!action.value)return
 if(action.value==='disable'&&!await confirmAction(t("关闭两步验证？"),t("关闭后所有设备及关联应用将退出登录，需要重新使用密码登录。"),t("关闭两步验证")))return
 busy.value=true;error.value=''
 try{
  const data=await api<{recovery_codes?:string[]}>('/me/mfa/'+(action.value==='disable'?'disable':'recovery-codes'),{method:'POST',body:body({current_password:current.value,code:code.value})})
  current.value='';code.value=''
  if(action.value==='disable'){location.assign('/login?mfa_disabled=1');return}
  codes.value=data.recovery_codes||[];recoveryNotice.value=t("旧恢复码已失效，其他设备已退出登录。请保存新的恢复码。");action.value='';await refreshSession();await load();toast.success(t("恢复码已更新。"))
 }catch(e){mutationError(e)}finally{busy.value=false}
}
async function copySecret(){try{await navigator.clipboard.writeText(setup.value?.secret||'');toast.success(t("密钥已复制，请粘贴到验证器。"))}catch{toast.error(t("无法自动复制，请选中密钥后手动复制。"))}}
function downloadCodes(){
 const text=[t("Painter 两步验证恢复码"),t("账号：{value1}", {value1:session.user?.username||''}),t("网站：{value1}", {value1:location.origin}),t("每组仅可使用一次，请保存在私人密码管理器或离线位置。"),'',...codes.value].join('\n')
 const url=URL.createObjectURL(new Blob([text],{type:'text/plain;charset=utf-8'}));const a=document.createElement('a');a.href=url;a.download='painter-recovery-codes.txt';a.click();setTimeout(()=>URL.revokeObjectURL(url),1000)
}
function beforeLeave(e:BeforeUnloadEvent){if(codes.value.length){e.preventDefault();e.returnValue=''}}
function cancel(){setup.value=null;action.value='';current.value='';code.value='';error.value='';qrError.value=''}
onMounted(()=>{window.addEventListener('beforeunload',beforeLeave);void load()})
onBeforeUnmount(()=>{window.removeEventListener('beforeunload',beforeLeave);cancel();codes.value=[]})
</script>

<template>
 <section class="panel account-security" aria-labelledby="account-security-title" :aria-busy="loading||busy">
  <div class="security-heading"><h2 id="account-security-title">{{t("两步验证")}}</h2><span v-if="status" class="security-state">{{status.mfa_enabled?t("已启用"):t("未启用")}}</span></div>
  <p class="muted">{{t("登录时，在密码或 GitHub 验证后再输入动态码。可使用 Apple 密码、1Password、Microsoft Authenticator 等验证器。")}}</p>
  <p v-if="loading" role="status">{{t("正在读取安全设置…")}}</p>
  <p v-if="error" class="notice error" role="alert">{{error}} <button v-if="!status" type="button" class="quiet" :disabled="busy||loading" @click="load">{{t("重新加载")}}</button></p>

  <template v-if="status&&!loading">
   <section v-if="codes.length" class="recovery-display" aria-labelledby="recovery-title">
    <h3 id="recovery-title">{{t("保存恢复码")}}</h3><p v-if="recoveryNotice" class="muted">{{recoveryNotice}}</p><p class="muted">{{t("仅在此显示一次。遗失验证器时，每组可恢复登录一次。请勿公开分享。")}}</p>
    <ul class="recovery-codes"><li v-for="item in codes" :key="item"><code>{{item}}</code></li></ul>
    <div class="form-actions"><button type="button" @click="downloadCodes">{{t("下载恢复码")}}</button><button type="button" class="primary" @click="codes=[];recoveryNotice='';toast.info(t('恢复码已收起。'))">{{t("我已妥善保存")}}</button></div>
   </section>
   <template v-else-if="!status.mfa_enabled">
    <form v-if="!setup" class="security-form" @submit.prevent="begin">
     <label v-if="status.has_local_password">{{t("当前登录密码")}}<input v-model="current" type="password" required autocomplete="current-password"></label>
     <p v-else class="muted">{{t("请先重新通过 GitHub 登录，5 分钟内可绑定验证器。")}}</p>
     <div class="form-actions"><button class="primary" :disabled="busy">{{busy?t("正在准备…"):t("绑定验证器")}}</button></div>
    </form>
    <div v-else class="mfa-setup">
     <div class="mfa-qr"><canvas ref="canvas" width="200" height="200" role="img" :aria-label="t('使用验证器扫描的两步验证二维码')"></canvas></div>
     <div class="mfa-setup-fields"><p>{{t("使用验证器扫描二维码，或手动添加下方密钥。")}}</p><p v-if="qrError" class="muted" role="status">{{qrError}}</p>
      <label>{{t("验证器密钥")}}<input :value="setup.secret" readonly spellcheck="false" autocomplete="off" class="security-secret"></label><button type="button" class="quiet" @click="copySecret">{{t("复制密钥")}}</button>
      <form class="security-form" @submit.prevent="enable"><label>{{t("6 位动态码")}}<input v-model="code" required inputmode="numeric" pattern="[0-9]{6}" maxlength="6" autocomplete="one-time-code"></label><small class="muted">{{t("绑定请求 10 分钟内有效。确认动态码后才会启用。")}}</small><div class="form-actions"><button class="primary" :disabled="busy">{{busy?t("正在验证…"):t("确认并启用")}}</button><button type="button" :disabled="busy" @click="cancel">{{t("取消")}}</button></div></form>
     </div>
    </div>
   </template>
   <template v-else>
    <p class="muted">{{t("剩余 {value1} 组恢复码。更换验证器前，请先确认仍可使用当前验证方式。", {value1:(status.recovery_codes_remaining)})}}</p>
    <div v-if="!action" class="form-actions"><button type="button" @click="action='regenerate'">{{t("重新生成恢复码")}}</button><button type="button" class="quiet danger" @click="action='disable'">{{t("关闭两步验证")}}</button></div>
    <form v-else class="security-form" @submit.prevent="manage"><h3>{{action==='disable'?t("关闭两步验证"):t("重新生成恢复码")}}</h3><label v-if="status.has_local_password">{{t("当前登录密码")}}<input v-model="current" type="password" required autocomplete="current-password"></label><p v-else class="muted">{{t("需要最近 5 分钟内完成的 GitHub 登录。")}}</p><label>{{t("动态码或未使用的恢复码")}}<input v-model="code" required maxlength="64" autocomplete="one-time-code" autocapitalize="none" spellcheck="false"></label><p class="muted">{{action==='disable'?t("所有设备和关联应用将退出登录。"):t("旧恢复码会全部失效，其他设备将退出登录。")}}</p><div class="form-actions"><button :class="action==='disable'?'danger':'primary'" :disabled="busy">{{busy?t("正在处理…"):action==='disable'?t("确认关闭并退出登录"):t("生成新的恢复码")}}</button><button type="button" :disabled="busy" @click="cancel">{{t("取消")}}</button></div></form>
   </template>
  </template>
 </section>
</template>

<style scoped>
.security-heading{display:flex;align-items:center;justify-content:space-between;gap:16px}.security-heading h2{margin:0}.security-state{color:var(--muted);font-size:var(--control-font-size,13px)}.account-security>p{max-width:70ch}.security-form{display:grid;gap:12px;max-width:480px;margin-top:20px}.security-form h3{margin:0}.security-form label,.mfa-setup-fields>label{display:grid;gap:8px}.security-form .form-actions{margin-top:4px}.mfa-setup{display:grid;grid-template-columns:200px minmax(0,1fr);gap:24px;margin-top:24px}.mfa-qr{align-self:start;background:var(--qr-paper);border-radius:var(--radius-surface,12px);overflow:hidden;width:200px;height:200px}.mfa-setup-fields{min-width:0}.mfa-setup-fields>p:first-child{margin-top:0}.security-secret{font-family:var(--mono,monospace);font-size:13px!important;letter-spacing:.02em;min-width:0;max-width:100%}.recovery-display{margin-top:24px}.recovery-codes{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:12px 24px;list-style:none;padding:16px 0;margin:0;max-width:560px}.recovery-codes code{font-size:13px;overflow-wrap:anywhere;user-select:all}.account-security .form-actions{flex-wrap:wrap;gap:8px}.account-security small{line-height:1.6}@media(max-width:600px){.mfa-setup{grid-template-columns:minmax(0,1fr)}.recovery-codes{grid-template-columns:minmax(0,1fr);gap:8px}.security-form{max-width:none}}
</style>
