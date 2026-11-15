<script setup lang="ts">
import {computed,onBeforeUnmount,onMounted,ref,watch} from 'vue'
import {api,body,date} from '../api'
import {toast} from '../toast'
import {confirmAction} from '../confirm'
import SelectField from './SelectField.vue'

type Client={id:string;name:string;kind:'web'|'spa'|'native';redirect_uris:string[];disabled_at_ms:number|null;created_at_ms:number}
const clients=ref<Client[]>([]),issuer=ref(''),loading=ref(true),loaded=ref(false),busy=ref(false),creating=ref(false)
const name=ref(''),kind=ref('web'),redirects=ref(''),error=ref('')
const secret=ref(''),secretClient=ref(''),savedSecret=ref(false)
const tableWrap=ref<HTMLElement>(),tableOverflow=ref(false)
let tableObserver:ResizeObserver|undefined
watch(tableWrap,element=>{tableObserver?.disconnect();if(!element){tableOverflow.value=false;return}const measure=()=>{tableOverflow.value=element.scrollWidth>element.clientWidth+1};tableObserver=new ResizeObserver(measure);tableObserver.observe(element);const table=element.querySelector('table');if(table)tableObserver.observe(table);measure()})
const kinds=[{value:'web',label:'Web 服务端应用'},{value:'spa',label:'浏览器单页应用'},{value:'native',label:'桌面或移动客户端'}]
const activeClients=computed(()=>clients.value.filter(c=>!c.disabled_at_ms))
const help=computed(()=>kind.value==='web'?'由服务端保管 Client Secret，使用 HTTPS 回调地址。':kind.value==='spa'?'不使用客户端密钥。浏览器通过 PKCE 完成授权；回调地址必须为 HTTPS。':'通过系统浏览器登录，支持已登记的应用 URL Scheme、HTTPS 或 IP 回环地址。')
async function load(){loading.value=true;error.value='';try{const data=await api<{issuer:string;items:Client[]}>('/admin/identity/clients');issuer.value=data.issuer;clients.value=data.items;loaded.value=true}catch(e){error.value=(e as Error).message}finally{loading.value=false}}
async function create(){
 if(busy.value)return
 const uris=redirects.value.split(/\r?\n/).map(s=>s.trim()).filter(Boolean)
 if(!uris.length){error.value='请填写至少一个完整回调地址。';return}
 busy.value=true;error.value=''
 try{
  const result=await api<{client:Client;client_secret?:string}>('/admin/identity/clients',{method:'POST',body:body({name:name.value.trim(),kind:kind.value,redirect_uris:uris})})
  if(result.client_secret){secret.value=result.client_secret;secretClient.value=result.client.name;savedSecret.value=false}
  creating.value=false;name.value='';redirects.value='';await load();toast.success('应用已创建，可以使用下方 Client ID 接入。')
 }catch(e){toast.error((e as Error).message)}finally{busy.value=false}
}
async function disable(client:Client){
 if(busy.value||!await confirmAction('停用这个接入应用？',`“${client.name}”将不能继续登录，已有授权和令牌同时失效。`,'停用应用'))return
 busy.value=true;error.value=''
 try{await api('/admin/identity/clients/'+encodeURIComponent(client.id),{method:'DELETE'});await load();toast.success('应用已停用，相关授权已撤销。')}catch(e){toast.error((e as Error).message)}finally{busy.value=false}
}
async function rotate(client:Client){
 if(busy.value||!await confirmAction('轮换客户端密钥？','现有密钥和该应用的授权将失效。轮换后需要更新接入应用服务器上的配置。','生成新密钥'))return
 busy.value=true;error.value=''
 try{const result=await api<{client_secret:string}>('/admin/identity/clients/'+encodeURIComponent(client.id)+'/rotate-secret',{method:'POST',body:'{}'});secret.value=result.client_secret;secretClient.value=client.name;savedSecret.value=false;toast.success('客户端密钥已轮换，请保存并更新接入应用。')}catch(e){toast.error((e as Error).message)}finally{busy.value=false}
}
async function copy(value:string,label:string){try{await navigator.clipboard.writeText(value);toast.success(label+'已复制。')}catch{toast.error('无法自动复制，请选中内容后手动复制。')}}
function clearSecret(){secret.value='';secretClient.value='';savedSecret.value=false}
function beforeLeave(e:BeforeUnloadEvent){if(secret.value&&!savedSecret.value){e.preventDefault();e.returnValue=''}}
onMounted(()=>{window.addEventListener('beforeunload',beforeLeave);void load()})
onBeforeUnmount(()=>{window.removeEventListener('beforeunload',beforeLeave);tableObserver?.disconnect();clearSecret()})
</script>

<template>
 <section class="panel identity-clients" aria-labelledby="identity-title" :aria-busy="loading||busy">
  <div class="identity-heading"><h2 id="identity-title">接入其他应用</h2><button v-if="loaded&&!creating&&!secret" type="button" :disabled="busy" @click="creating=true;error=''">添加应用</button></div>
  <p class="muted">让自己的 Web、桌面和移动应用复用本站账号与两步验证。通过 OIDC 授权码和 PKCE 登录，每个应用单独登记、撤销。</p>
  <p v-if="loading" role="status">正在读取接入应用…</p>
  <p v-if="error" class="notice error" role="alert">{{error}} <button v-if="!loaded" type="button" class="quiet" :disabled="loading||busy" @click="load">重新加载</button></p>
  <template v-if="loaded">
   <div class="identity-issuer"><label for="identity-issuer">Issuer<input id="identity-issuer" :value="issuer" readonly spellcheck="false"></label><button type="button" @click="copy(issuer,'Issuer')">复制地址</button></div>
   <details class="identity-instructions"><summary>接入说明</summary><p class="muted">客户端从 <code>{{issuer}}/.well-known/openid-configuration</code> 发现端点；请求 <code>openid profile</code>。需要持续访问时再请求 <code>offline_access</code>。所有客户端使用 PKCE S256，并在本地验证 state、nonce、签名、Issuer 与 Audience。</p><p class="muted">回调地址逐条精确匹配，不使用通配符。原生应用使用系统浏览器；IP 回环回调可动态选择端口。令牌仅代表登录身份，不授予 Painter 后台管理权限。</p></details>
   <section v-if="secret" class="identity-secret" aria-labelledby="identity-secret-title"><h3 id="identity-secret-title">保存 {{secretClient}} 的 Client Secret</h3><p class="muted">仅在此显示一次。轮换后请更新接入应用的配置，旧密钥已失效。只放在接入应用的服务端，不要放进前端、客户端安装包或公开仓库。</p><label for="identity-client-secret">Client Secret<textarea id="identity-client-secret" :value="secret" readonly rows="3" spellcheck="false" autocomplete="off"></textarea></label><div class="form-actions"><button type="button" @click="copy(secret,'Client Secret')">复制密钥</button><button type="button" class="primary" @click="clearSecret">已保存，收起密钥</button></div></section>
   <form v-if="creating" class="identity-create" @submit.prevent="create"><h3>添加接入应用</h3><fieldset :disabled="busy"><legend class="sr-only">接入应用信息</legend><div class="form-grid"><label>应用名称<input v-model="name" required maxlength="100" placeholder="例如我的笔记应用" autocomplete="off"></label><SelectField v-model="kind" :options="kinds" label="应用类型" :disabled="busy"/><p class="muted full identity-kind-help">{{help}}</p><label class="full">回调地址<textarea v-model="redirects" required rows="3" maxlength="8192" spellcheck="false" autocomplete="off" placeholder="每行填写一个完整回调地址"></textarea><small class="muted">只填写由自己控制的地址，最多 10 个。</small></label></div><div class="form-actions"><button class="primary" :disabled="busy">{{busy?'正在创建…':'创建应用'}}</button><button type="button" :disabled="busy" @click="creating=false">取消</button></div></fieldset></form>
   <p v-if="tableOverflow" id="identity-scroll-hint" class="muted identity-scroll-hint">左右滑动查看回调地址和操作</p><div v-if="activeClients.length" ref="tableWrap" class="identity-table-wrap" role="region" aria-label="已启用的接入应用" :aria-describedby="tableOverflow?'identity-scroll-hint':undefined" tabindex="0"><table class="identity-table"><thead><tr><th scope="col">应用</th><th scope="col">Client ID 与回调</th><th scope="col">操作</th></tr></thead><tbody><tr v-for="client in activeClients" :key="client.id"><td><strong>{{client.name}}</strong><small class="muted">{{kinds.find(k=>k.value===client.kind)?.label}}</small><small class="muted">{{date(client.created_at_ms)}} 创建</small></td><td><code>{{client.id}}</code><ul><li v-for="uri in client.redirect_uris" :key="uri">{{uri}}</li></ul></td><td><div class="identity-row-actions"><button type="button" class="quiet" :disabled="busy||!!secret" @click="copy(client.id,'Client ID')">复制 ID</button><button v-if="client.kind==='web'" type="button" class="quiet" :disabled="busy||!!secret" @click="rotate(client)">轮换密钥</button><button type="button" class="quiet danger" :disabled="busy" @click="disable(client)">停用</button></div></td></tr></tbody></table></div>
   <p v-else-if="!loading&&!creating" class="muted identity-empty">尚未接入其他应用。添加应用后，使用生成的 Client ID 配置登录。</p>
   <p v-if="clients.length>activeClients.length" class="muted identity-disabled">另有 {{clients.length-activeClients.length}} 个已停用应用，其授权均已撤销。</p>
  </template>
 </section>
</template>

<style scoped>
.identity-heading{display:flex;align-items:center;justify-content:space-between;gap:16px}.identity-heading h2{margin:0}.identity-clients>p{max-width:70ch}.identity-issuer{display:flex;gap:8px;align-items:flex-end;margin-top:24px}.identity-issuer label{display:grid;gap:8px;flex:1;min-width:0}.identity-issuer button{flex-shrink:0}.identity-instructions{margin:20px 0}.identity-instructions summary{cursor:pointer;font-size:14px}.identity-instructions p{font-size:13px;line-height:1.8;max-width:75ch;overflow-wrap:anywhere}.identity-clients code{font-size:12px;overflow-wrap:anywhere}.identity-create,.identity-secret{padding:24px 0;margin:24px 0;border-top:1px solid var(--line)}.identity-create h3,.identity-secret h3{font-size:15px;margin:0 0 16px}.identity-create fieldset{border:0;padding:0;margin:0;min-width:0}.identity-kind-help{margin:0;font-size:13px;line-height:1.6}.identity-secret label{display:grid;gap:8px}.identity-secret textarea{font-family:var(--mono,monospace);font-size:13px;overflow-wrap:anywhere}.identity-clients .form-actions{gap:8px;flex-wrap:wrap}.identity-table-wrap{max-width:100%;overflow:auto;margin-top:24px}.identity-table{width:100%;border-collapse:collapse;text-align:left;font-size:13px;min-width:580px}.identity-table th{font-size:12px;font-weight:500;color:var(--muted)}.identity-table th,.identity-table td{padding:12px 12px 12px 0;vertical-align:top;border-bottom:1px solid var(--line)}.identity-table td:first-child{width:24%}.identity-table td:nth-child(2){width:50%;overflow-wrap:anywhere}.identity-table td:first-child>small{display:block;margin-top:4px}.identity-table ul{list-style:none;padding:0;margin:8px 0 0;display:grid;gap:4px;color:var(--muted);font-size:12px}.identity-row-actions{display:flex;align-items:flex-start;flex-direction:column;gap:2px}.identity-row-actions button{white-space:nowrap}.identity-empty{margin:24px 0 0}.identity-disabled{font-size:12px;margin:16px 0 0}.identity-scroll-hint{font-size:12px;line-height:1.6;margin:20px 0 -12px}@media(max-width:540px){.identity-issuer{align-items:stretch;flex-direction:column}.identity-issuer button{align-self:flex-start}.identity-heading{align-items:flex-start;flex-wrap:wrap}}
</style>
