<script setup lang="ts">
import {computed,nextTick,ref,watch} from 'vue'
import Icon from './Icon.vue'
import SelectField from './SelectField.vue'
import {currentNavigation,isNavigationItem,type NavigationGroup} from '../navigation'
const props=defineProps<{groups:NavigationGroup[];path:string;hash:string;user:Record<string,any>|null;busy:boolean}>()
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
function changeTheme(value:string|number){theme.value=String(value);try{localStorage.setItem('painter-theme',theme.value)}catch{}document.documentElement.dataset.theme=theme.value==='system'?(matchMedia('(prefers-color-scheme:dark)').matches?'dark':'light'):theme.value}
</script>
<template>
 <a class="studio-brand" href="/" aria-label="Painter 首页"><img :src="'/assets/images/painter-logo.png'" width="38" height="38" alt=""><span>Painter<small>创作与管理</small></span></a>
 <nav ref="nav" class="studio-links" aria-label="工作台导航"><section v-for="group in groups" :key="group.id" class="studio-nav-section" :class="{'is-current':active?.group.id===group.id}"><button type="button" class="studio-section-trigger" :aria-expanded="!!expanded[group.id]" :aria-controls="'studio-section-'+group.id" @click="toggle(group.id)"><Icon :name="group.icon" :size="18"/><span>{{group.label}}</span><Icon name="chevron" :size="15" class="studio-section-chevron"/></button><div v-show="expanded[group.id]" :id="'studio-section-'+group.id" class="studio-section-links"><a v-for="item in group.children" :key="item.id" :href="item.url" :class="{active:isNavigationItem(item,path,hash)}" :aria-current="isNavigationItem(item,path,hash)?'page':undefined" @click="$emit('navigate')">{{item.label}}</a></div></section></nav>
 <div class="studio-sidebar-bottom"><a class="studio-visit" href="/" target="_blank" rel="noopener"><Icon name="arrow" :size="17"/><span>查看网站</span></a><SelectField :model-value="theme" @update:model-value="changeTheme" label="外观主题" :options="[{value:'system',label:'跟随系统'},{value:'light',label:'浅色'},{value:'dark',label:'深色'}]"/><div class="studio-account"><span class="avatar">{{(user?.display_name||'?').slice(0,1)}}</span><div><strong>{{user?.display_name}}</strong><small>{{({owner:'站长',author:'作者',reader:'读者'} as Record<string,string>)[user?.role]}}</small></div><button type="button" class="quiet" aria-label="退出登录" :disabled="busy" @click="$emit('logout')"><Icon name="logout" :size="18"/></button></div></div>
</template>
