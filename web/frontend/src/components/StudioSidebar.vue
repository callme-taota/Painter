<script setup lang="ts">
import {locale,t} from "../i18n"

import {computed,nextTick,ref,watch} from 'vue'
import Icon from './Icon.vue'
import SelectField from './SelectField.vue'
import {currentNavigation,isNavigationItem,type NavigationGroup} from '../navigation'
const props=defineProps<{groups:NavigationGroup[];path:string;hash:string;user:Record<string,any>|null;busy:boolean;presentation?:'rail'|'sheet'}>()
defineEmits<{logout:[];navigate:[]}>()
const active=computed(()=>currentNavigation(props.groups,props.path,props.hash))
const expanded=ref<Record<string,boolean>>({})
const nav=ref<HTMLElement>()
function revealCurrent(){const container=nav.value,link=container?.querySelector<HTMLElement>('a[aria-current="page"]');if(!container||!link||!container.clientHeight)return;const viewport=container.getBoundingClientRect(),item=link.getBoundingClientRect();if(item.top<viewport.top)container.scrollTop+=item.top-viewport.top;else if(item.bottom>viewport.bottom)container.scrollTop+=item.bottom-viewport.bottom}
defineExpose({revealCurrent})
function readExpanded(){try{const saved=JSON.parse(sessionStorage.getItem('painter-navigation-v9:'+props.user?.id)||'{}');expanded.value=Object.fromEntries(props.groups.map(group=>[group.id,saved[group.id]===true]))}catch{expanded.value={}};if(active.value)expanded.value[active.value.group.id]=true;void nextTick(revealCurrent)}
function toggle(id:string){expanded.value[id]=!expanded.value[id];try{sessionStorage.setItem('painter-navigation-v9:'+props.user?.id,JSON.stringify(expanded.value))}catch{}}
watch(()=>props.user?.id,readExpanded,{immediate:true})
watch(()=>active.value?.item.id,()=>{if(active.value)expanded.value[active.value.group.id]=true;void nextTick(revealCurrent)})
function readTheme(){try{return localStorage.getItem('painter-theme')||'system'}catch{return 'system'}}
const theme=ref(readTheme())
const localeNext=location.pathname+location.search+location.hash
function localeURL(lang:'zh-CN'|'en'){return '/locale?lang='+encodeURIComponent(lang)+'&next='+encodeURIComponent(localeNext)}
function changeTheme(value:string|number){theme.value=String(value);try{localStorage.setItem('painter-theme',theme.value)}catch{}document.documentElement.dataset.theme=theme.value==='system'?(matchMedia('(prefers-color-scheme:dark)').matches?'dark':'light'):theme.value}
</script>
<template>
 <a v-if="presentation!=='sheet'" class="studio-brand" href="/" :aria-label="t('Painter 首页')"><img :src="'/assets/images/painter-logo.png'" width="38" height="38" alt=""><span>Painter<small>{{t("创作与管理")}}</small></span></a>
 <nav ref="nav" class="studio-links" :class="{'studio-sheet-links':presentation==='sheet'}" :aria-label="t('工作台导航')"><section v-for="group in groups" :key="group.id" class="studio-nav-section" :class="{'is-current':active?.group.id===group.id}"><h3 v-if="presentation==='sheet'" class="studio-sheet-group"><Icon :name="group.icon" :size="17"/>{{group.label}}</h3><button v-else type="button" class="studio-section-trigger" :aria-expanded="!!expanded[group.id]" :aria-controls="'studio-section-'+group.id" @click="toggle(group.id)"><Icon :name="group.icon" :size="18"/><span>{{group.label}}</span><Icon name="chevron" :size="15" class="studio-section-chevron"/></button><div v-show="presentation==='sheet'||expanded[group.id]" :id="'studio-section-'+group.id" class="studio-section-links"><a v-for="item in group.children" :key="item.id" :href="item.url" :class="{active:isNavigationItem(item,path,hash)}" :aria-current="isNavigationItem(item,path,hash)?'page':undefined" @click="$emit('navigate')">{{item.label}}</a></div></section></nav>
 <div class="studio-sidebar-bottom"><a class="studio-visit" href="/" target="_blank" rel="noopener"><Icon name="arrow" :size="17"/><span>{{t("查看网站")}}</span></a><SelectField :model-value="theme" @update:model-value="changeTheme" :label="t('外观主题')" :options="[{value:'system',label:t('跟随系统')},{value:'light',label:t('浅色')},{value:'dark',label:t('深色')}]"/><div class="studio-language" role="group" :aria-label="t('语言')"><a data-locale-switch :href="localeURL('zh-CN')" lang="zh-CN" :aria-current="locale==='zh-CN'?'true':undefined">简体中文</a><a data-locale-switch :href="localeURL('en')" lang="en" :aria-current="locale==='en'?'true':undefined">English</a></div><div class="studio-account"><span class="avatar">{{(user?.display_name||'?').slice(0,1)}}</span><div><strong>{{user?.display_name}}</strong><small>{{({owner:t("站长"),author:t("作者"),reader:t("读者")} as Record<string,string>)[user?.role]}}</small></div><button type="button" class="quiet" :aria-label="t('退出登录')" :disabled="busy" @click="$emit('logout')"><Icon name="logout" :size="18"/></button></div></div>
</template>

<style scoped>
.studio-language{display:flex;align-items:center;gap:4px;padding:3px;border:1px solid var(--line);border-radius:var(--radius-control);background:var(--subtle)}
.studio-language a{display:flex;align-items:center;justify-content:center;flex:1;min-width:0;min-height:30px;padding:4px 8px;border-radius:calc(var(--radius-control) - 3px);color:var(--muted);font:500 .75rem/1.4 var(--font-ui);text-decoration:none}
.studio-language a:hover{color:var(--ink);background:var(--paper)}
.studio-language a[aria-current]{color:var(--ink);background:var(--paper);box-shadow:0 1px 2px rgb(0 0 0 / .05)}
.studio-language a:focus-visible{outline:2px solid var(--accent);outline-offset:2px}
@media(max-width:960px){.studio-language a{min-height:44px}}
</style>
