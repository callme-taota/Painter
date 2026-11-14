<script setup lang="ts">
import {nextTick,onMounted,reactive,ref} from 'vue'
type Status={available:boolean;installed:boolean;csrf_token:string;postgres_ready:boolean;schema_version:number;public_base_url:string;github_configured:boolean}
const status=ref<Status|null>(null),loading=ref(true),busy=ref(false),error=ref(''),step=ref(1),created=ref(false)
const form=reactive({site_name:'Painter',tagline:'',admin_username:'',admin_display_name:'',password:'',confirm:'',setup_token:''})
const steps=['环境检查','站点资料','管理员账号']
function clearSecrets(){form.password='';form.confirm='';form.setup_token=''}
async function load(){loading.value=true;error.value='';try{const r=await fetch('/api/v1/setup/status',{credentials:'same-origin',cache:'no-store'});if(!r.ok)throw Error('暂时无法读取安装状态，请稍后重试。');status.value=await r.json();if(status.value?.installed)clearSecrets()}catch(e){error.value=(e as Error).message}finally{loading.value=false}}
function changeStep(value:number){step.value=value;error.value='';void nextTick(()=>document.querySelector<HTMLElement>('.install-step h2')?.focus())}
function next(){error.value='';if(step.value===2&&!form.site_name.trim()){error.value='请填写站点名称。';return}changeStep(step.value+1)}
async function complete(){
 if(busy.value||!status.value?.available)return
 error.value=''
 if(form.password!==form.confirm){error.value='两次输入的密码不一致。';return}
 if(Array.from(form.password).length<15||Array.from(form.password).length>1024||new TextEncoder().encode(form.password).length>4096){error.value='密码需为15–1024个字符，且不超过4096字节。';return}
 busy.value=true
 try{
  const {confirm,...data}=form
  const r=await fetch('/api/v1/setup/complete',{method:'POST',credentials:'same-origin',headers:{'Content-Type':'application/json','X-CSRF-Token':status.value.csrf_token},body:JSON.stringify({...data,site_name:data.site_name.trim(),admin_username:data.admin_username.trim().toLowerCase(),admin_display_name:data.admin_display_name.trim()})})
  const result=await r.json()
  if(!r.ok){
   if(result.error?.code==='SETUP_COMPLETED'){await load();return}
   if(result.error?.code==='CSRF_FAILED'){const retry=await fetch('/api/v1/setup/status',{credentials:'same-origin',cache:'no-store'});if(retry.ok)status.value=await retry.json()}
   throw Error(result.error?.message||'安装未完成，请检查信息后重试。')
  }
  clearSecrets();created.value=true;status.value={...status.value,installed:true,available:false}
 }catch(e){error.value=(e as Error).message}finally{busy.value=false}
}
onMounted(load)
</script>

<template>
 <div class="install-wizard">
  <header class="install-heading"><h1>{{status?.installed?'网站已就绪':'安装 Painter'}}</h1><p>{{status?.installed?'安装入口已关闭，你可以使用已有账号登录。':'完成站点资料和管理员设置，开始写下第一篇文章。'}}</p></header>
  <div v-if="loading" class="install-loading" role="status">正在检查安装环境…</div>
  <template v-else-if="status?.installed">
   <div class="install-done"><h2>{{created?'管理员账号已创建':'安装已经完成'}}</h2><p>{{created?'站点资料与管理员账号已保存。请使用刚才设置的用户名和密码登录。':'已有文章、账号和设置会继续保留。这个页面不能重新创建管理员。'}}</p><a class="button primary" href="/login?installed=1">登录管理后台</a><a class="install-home" href="/">查看网站</a></div>
  </template>
  <template v-else-if="status?.available">
   <ol class="install-steps" aria-label="安装进度"><li v-for="(name,index) in steps" :key="name" :aria-current="step===index+1?'step':undefined" :class="{passed:step>index+1}">{{name}}</li></ol>
   <p v-if="error" class="notice error" role="alert">{{error}}</p>
   <section v-if="step===1" class="install-step" aria-labelledby="environment-title"><h2 id="environment-title" tabindex="-1">确认安装环境</h2><p>服务已连接到数据库，请确认这是你要安装的站点地址。</p><dl class="install-checks"><div><dt>数据库</dt><dd><span class="check-result">{{status.postgres_ready?'PostgreSQL 已连接':'等待连接'}}</span></dd></div><div><dt>数据结构</dt><dd>版本 {{status.schema_version}}，迁移已完成</dd></div><div><dt>站点地址</dt><dd>{{status.public_base_url}}</dd></div></dl><p class="install-hint">接下来需要服务器生成的安装码。安装脚本用户可在服务器运行 <code>./scripts/painter setup-token</code> 查看。</p><div class="install-actions"><button class="primary" :disabled="!status.postgres_ready" @click="next">开始设置</button></div></section>
   <form v-else-if="step===2" class="install-step" @submit.prevent="next"><h2 tabindex="-1">给这个博客一个名字</h2><div class="install-fields"><label>站点名称<input v-model="form.site_name" required maxlength="100" autocomplete="off" placeholder="读者会记住的名字"></label><div class="install-field"><label for="install-tagline">一句介绍<textarea id="install-tagline" v-model="form.tagline" maxlength="300" rows="3" placeholder="这里会记录些什么？" aria-describedby="install-tagline-hint"></textarea></label><small id="install-tagline-hint">以后可以在后台修改。</small></div></div><div class="install-actions"><button type="button" @click="changeStep(1)">上一步</button><button class="primary">继续</button></div></form>
   <form v-else class="install-step" @submit.prevent="complete"><h2 tabindex="-1">创建你的管理员账号</h2><p>这是站点的第一个管理员。完成安装后，公开注册保持关闭，可在后台开启。</p><div class="install-fields"><label>管理员用户名<input v-model="form.admin_username" required minlength="2" maxlength="64" pattern="[A-Za-z0-9][A-Za-z0-9_-]{1,63}" autocomplete="username" autocapitalize="none" spellcheck="false" placeholder="字母、数字、下划线或连字符"></label><label>管理员显示名称<input v-model="form.admin_display_name" required maxlength="100" autocomplete="nickname" placeholder="读者看到的名字"></label><div class="install-passwords"><label>管理员密码<input v-model="form.password" required type="password" autocomplete="new-password" aria-describedby="install-password-hint"></label><label>确认管理员密码<input v-model="form.confirm" required type="password" autocomplete="new-password"></label></div><small id="install-password-hint">至少15个字符，推荐使用容易记住的长口令。</small><div class="install-field"><label for="install-token">安装码<input id="install-token" v-model="form.setup_token" required minlength="32" maxlength="256" type="password" autocomplete="off" autocapitalize="none" spellcheck="false" aria-describedby="install-token-hint"></label><small id="install-token-hint">使用服务器生成的安装码，确认你有权初始化这个站点。</small></div></div><div class="install-actions"><button type="button" :disabled="busy" @click="changeStep(2)">上一步</button><button class="primary" :disabled="busy">{{busy?'正在完成安装…':'完成安装'}}</button></div></form>
  </template>
  <div v-else class="install-unavailable"><h2>等待服务器开启安装</h2><p>请在服务器配置随机的 <code>SETUP_TOKEN</code> 并重启服务，再回到这里继续。使用新版安装脚本时会自动生成。</p><p v-if="error" class="notice error" role="alert">{{error}}</p><button :disabled="loading" @click="load">重新检查</button></div>
 </div>
</template>

<style scoped>
.install-wizard{max-width:760px;margin:28px auto 56px}.install-heading{padding-bottom:28px}.install-heading h1{font-size:clamp(2.2rem,5vw,3.25rem);line-height:1.4;letter-spacing:-.03em;margin:0 0 14px}.install-heading p,.install-step>p,.install-done p,.install-unavailable p{color:var(--muted);font-size:.95rem;line-height:1.9;margin:0 0 24px}.install-steps{display:flex;list-style:none;gap:24px;margin:0 0 32px;padding:0;border-bottom:1px solid var(--line)}.install-steps li{flex:1;padding:14px 0;color:var(--muted);font-size:.88rem;line-height:1.7}.install-steps li[aria-current=step]{color:var(--accent);font-weight:600;border-bottom:2px solid var(--accent);margin-bottom:-1px}.install-steps li.passed{color:var(--ink)}.install-step h2,.install-done h2,.install-unavailable h2{font-size:1.6rem;line-height:1.6;margin:0 0 16px}.install-checks{margin:28px 0}.install-checks>div{display:grid;grid-template-columns:110px minmax(0,1fr);gap:20px;padding:18px 0;border-bottom:1px solid var(--line);font-size:.9rem;line-height:1.8}.install-checks dt{color:var(--muted)}.install-checks dd{margin:0;overflow-wrap:anywhere;font-variant-numeric:tabular-nums}.check-result{color:var(--success);line-height:var(--control-leading)}.install-hint{font-size:.82rem!important}.install-hint code,.install-unavailable code{font-size:.9em;overflow-wrap:anywhere}.install-fields{display:grid;gap:24px}.install-field{display:grid;gap:8px}.install-fields label{display:grid;gap:8px;font-size:.88rem;color:var(--ink);font-weight:550}.install-fields input,.install-fields textarea{font-weight:400;min-height:48px}.install-fields small{color:var(--muted);font-size:.8rem;font-weight:400;line-height:1.8}.install-passwords{display:grid;grid-template-columns:1fr 1fr;gap:24px}.install-passwords+small{margin-top:-16px}.install-actions{display:flex;justify-content:flex-end;gap:12px;margin-top:32px}.install-actions button{min-height:48px}.install-done,.install-unavailable{padding:28px 0;border-top:1px solid var(--line)}.install-home{display:inline-flex;align-items:center;min-height:44px;padding:10px;margin-left:12px;font-size:.85rem;color:var(--muted)}.install-loading{padding:32px 0;color:var(--muted)}@media(max-width:720px){.install-wizard{margin:8px auto 24px}.install-heading{padding-bottom:20px}.install-steps{gap:16px;margin-bottom:26px}.install-steps li{font-size:.8rem}.install-step h2,.install-done h2,.install-unavailable h2{font-size:1.35rem}.install-passwords{grid-template-columns:1fr;gap:24px}.install-checks>div{grid-template-columns:80px minmax(0,1fr);gap:12px;font-size:.85rem}.install-actions{justify-content:space-between}.install-heading p,.install-step>p,.install-done p,.install-unavailable p{font-size:.88rem}.install-fields input{font-size:1rem}}
</style>
