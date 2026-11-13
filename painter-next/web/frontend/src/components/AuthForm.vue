<script setup lang="ts">
import {nextTick,onMounted,ref} from 'vue'

type Challenge={challenge_id:string;expires_at:string;csrf_token:string}
const props=defineProps<{mode:string;returnTo?:string}>()
const options=ref<any>(null),challenge=ref<Challenge|null>(null)
const username=ref(''),displayName=ref(''),password=ref(''),code=ref('')
const error=ref(''),busy=ref(false),loading=ref(true),recovery=ref(false),codeField=ref<HTMLInputElement>()
const passwordChanged=new URLSearchParams(location.search).has('password_changed')
function target(){
 try{const url=new URL(props.returnTo||'/me',location.origin);if(url.origin===location.origin)return url.pathname+url.search+url.hash}catch{}
 return '/me'
}
async function getOptions(){
 const r=await fetch('/api/v1/auth/options',{credentials:'same-origin',cache:'no-store'})
 if(!r.ok)throw new Error('登录服务暂不可用，请稍后重试。')
 return r.json()
}
async function showChallenge(value:Challenge){challenge.value=value;password.value='';code.value='';recovery.value=false;await nextTick();codeField.value?.focus()}
async function readError(r:Response){
 const data=await r.json().catch(()=>({}))
 return data.error?.message||(r.status===429?'尝试过于频繁，请稍后再试。':'未能完成，请重试。')
}
async function initialize(){
 loading.value=true;error.value=''
 try{
  options.value=await getOptions()
  if(props.mode==='login'&&new URLSearchParams(location.search).has('mfa')){
   const r=await fetch('/api/v1/auth/mfa/challenge',{credentials:'same-origin',cache:'no-store'})
   if(r.ok)await showChallenge(await r.json());else error.value='验证已过期，请重新输入账号和密码。'
  }
 }catch(e){error.value=(e as Error).message}finally{loading.value=false}
}
async function submit(){
 if(busy.value)return
 error.value=''
 const length=Array.from(password.value).length
 if(length<15||length>1024||new TextEncoder().encode(password.value).length>4096){error.value='密码需为 15–1024 个字符，且不超过 4096 字节。';return}
 busy.value=true
 try{
  options.value=await getOptions()
  const registering=props.mode==='register'
  const r=await fetch('/api/v1/auth/'+(registering?'register':'login'),{method:'POST',credentials:'same-origin',headers:{'Content-Type':'application/json','X-CSRF-Token':options.value.csrf_token},body:JSON.stringify({username:username.value.trim().toLowerCase(),password:password.value,...(registering?{display_name:displayName.value.trim()}:{})})})
  if(!r.ok)throw new Error(await readError(r))
  const data=await r.json()
  if(r.status===202&&data.mfa_required){await showChallenge(data);return}
  if(!data.authenticated)throw new Error('登录尚未完成，请重新尝试。')
  password.value='';location.assign(target())
 }catch(e){error.value=(e as Error).message}finally{busy.value=false}
}
async function verify(){
 if(busy.value||!challenge.value)return
 error.value='';busy.value=true
 try{
  const r=await fetch('/api/v1/auth/mfa/verify',{method:'POST',credentials:'same-origin',headers:{'Content-Type':'application/json','X-CSRF-Token':challenge.value.csrf_token},body:JSON.stringify({challenge_id:challenge.value.challenge_id,code:code.value.trim()})})
  if(!r.ok)throw new Error(await readError(r))
  const data=await r.json()
  if(!data.authenticated)throw new Error('验证尚未完成，请重新尝试。')
  code.value='';location.assign(target())
 }catch(e){error.value=(e as Error).message;code.value='';codeField.value?.focus()}finally{busy.value=false}
}
function restart(){location.assign('/login?return_to='+encodeURIComponent(target()))}
onMounted(initialize)
</script>

<template>
 <div>
  <p v-if="mode==='login'&&passwordChanged" class="notice" role="status">密码已更新，请重新登录。</p>
  <div v-if="loading" class="loading-line" aria-label="正在读取登录选项"></div>
  <p v-if="error" class="notice error" role="alert">{{error}}</p>
  <button v-if="!loading&&!options" type="button" @click="initialize">重新加载登录表单</button>
  <template v-if="options&&!loading">
   <form v-if="challenge" class="auth-form" @submit.prevent="verify">
    <h2>两步验证</h2>
    <p class="muted">{{recovery?'输入一组未使用的恢复码。每组只能使用一次。':'打开验证器，输入当前的 6 位动态码。'}}</p>
    <label>{{recovery?'恢复码':'验证器动态码'}}<input ref="codeField" v-model="code" required :inputmode="recovery?'text':'numeric'" :autocomplete="recovery?'off':'one-time-code'" autocapitalize="none" spellcheck="false" :pattern="recovery?undefined:'[0-9]{6}'" :maxlength="recovery?64:6" aria-describedby="mfa-expiry"></label>
    <small id="mfa-expiry" class="muted">验证请求有效期为 5 分钟。动态码不能重复使用。</small>
    <button class="primary" :disabled="busy">{{busy?'正在验证…':'验证并登录'}}</button>
    <div class="auth-mfa-actions"><button type="button" class="quiet" :disabled="busy" @click="recovery=!recovery;code='';error='';codeField?.focus()">{{recovery?'使用验证器动态码':'使用恢复码'}}</button><button type="button" class="quiet" :disabled="busy" @click="restart">重新登录</button></div>
   </form>
   <template v-else>
    <p v-if="mode==='register'&&!options.registration_open" class="notice">当前暂停注册。已有账号可前往登录。</p>
    <form v-else class="auth-form" @submit.prevent="submit">
     <p v-if="mode==='login'&&options.external_login_enabled===false" class="muted">当前仅允许管理员登录。</p>
     <label>用户名<input v-model="username" required minlength="2" maxlength="64" pattern="[A-Za-z0-9][A-Za-z0-9_-]{1,63}" autocomplete="username" autocapitalize="none" spellcheck="false" placeholder="字母、数字、下划线或连字符"></label>
     <label v-if="mode==='register'">显示名称<input v-model="displayName" required maxlength="100" autocomplete="nickname" placeholder="读者会看到的名字"></label>
     <label>密码<input v-model="password" required type="password" :autocomplete="mode==='register'?'new-password':'current-password'" :aria-describedby="mode==='register'?'password-hint':undefined"></label>
     <small v-if="mode==='register'" id="password-hint">使用 15–1024 个字符。建议使用容易记住的长密码，首尾空格也会保留。</small>
     <button class="primary" :disabled="busy">{{busy?'请稍候…':mode==='register'?'注册账号':'登录'}}</button>
    </form>
    <div class="auth-alternatives">
     <a v-if="mode==='login'&&options.registration_open" :href="'/register?return_to='+encodeURIComponent(target())">创建账号</a>
     <a v-if="mode==='register'" :href="'/login?return_to='+encodeURIComponent(target())">已有账号？登录</a>
     <a v-if="options.github_login_enabled" class="button" :href="'/auth/github/start?return_to='+encodeURIComponent(target())">使用 GitHub 继续</a>
    </div>
    <p v-if="mode==='login'" class="auth-help">忘记密码或遗失验证器？请联系站点管理员恢复。</p>
   </template>
  </template>
 </div>
</template>

<style scoped>
.auth-form h2{font-size:1.125rem;margin:0}.auth-mfa-actions{display:flex;justify-content:space-between;gap:8px;flex-wrap:wrap}.auth-form small{line-height:1.6}
</style>
