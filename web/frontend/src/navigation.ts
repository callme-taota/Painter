import {t} from './i18n'
export type NavigationItem={id:string;label:string;url:string;roles?:string[];aliases?:string[];article?:boolean;section?:string}
export type NavigationGroup={id:string;label:string;icon:string;children:NavigationItem[]}
export function settingsSection(hash:string){const section=hash.replace(/^#/,'');return section==='features'||section==='security'?section:'site'}
export function navigation(role:string):NavigationGroup[]{
 const writer=['owner','author'],owner=['owner']
 const groups:NavigationGroup[]=[
  {id:'content',label:t("内容管理"),icon:'posts',children:[
   {id:'posts',label:role==='owner'?t("文章管理"):t("我的文章"),url:'/studio/posts',roles:writer,article:true},
   {id:'series',label:t("专栏管理"),url:'/studio/series',roles:writer},
   {id:'moments',label:role==='owner'?t("动态管理"):t("我的动态"),url:'/studio/moments',roles:writer},
   {id:'phrases',label:t("片语"),url:'/studio/phrases',roles:owner},
   {id:'friends',label:t("友链"),url:'/studio/friends',roles:owner},
   {id:'tags',label:t("标签管理"),url:'/studio/taxonomy',roles:owner},
   {id:'comments',label:t("评论管理"),url:'/studio/comments',roles:owner}]},
  {id:'operations',label:t("数据与运维"),icon:'chart',children:[
   {id:'analytics',label:role==='owner'?t("站点访问分析"):t("我的文章分析"),url:'/studio/analytics',roles:writer},
   {id:'operations',label:t("存储与任务"),url:'/studio/operations',roles:owner}]},
  {id:'site',label:t("站点管理"),icon:'settings',children:[
   {id:'home',label:t("首页内容"),url:'/studio/home',roles:owner},
   {id:'products',label:t("产品"),url:'/studio/products',roles:owner},
   {id:'site-profile',label:t("网站信息"),url:'/studio/settings#site',section:'site',roles:owner},
   {id:'site-features',label:t("功能与通知"),url:'/studio/settings#features',section:'features',roles:owner},
   {id:'site-security',label:t("登录安全"),url:'/studio/settings#security',section:'security',roles:owner},
   {id:'members',label:t("成员权限"),url:'/studio/members',roles:owner}]},
  {id:'personal',label:t("个人空间"),icon:'user',children:[
   {id:'profile',label:t("个人中心"),url:'/me'},
   {id:'bookmarks',label:t("收藏文章"),url:'/me/bookmarks'},
   {id:'following-feed',label:t("关注更新"),url:'/following'},
   {id:'following',label:t("关注与粉丝"),url:'/me/following',aliases:['/me/followers']},
   {id:'history',label:t("阅读足迹"),url:'/me/history'},
   {id:'media',label:t("我的媒体"),url:'/me/media'},
   {id:'sessions',label:t("登录会话"),url:'/me/sessions'}]}
 ]
 return groups.map(group=>({...group,children:group.children.filter(item=>!item.roles||item.roles.includes(role))})).filter(group=>group.children.length)
}
export function isNavigationItem(item:NavigationItem,path:string,hash:string){
 path=path.replace(/\/$/,'')||'/me'
 if(item.article)return path==='/studio/posts'||/^\/studio\/posts\/(new|\+?0*[1-9][0-9]*)$/.test(path)
 if(item.section)return path==='/studio/settings'&&settingsSection(hash)===item.section
 return path===item.url||!!item.aliases?.includes(path)
}
export function currentNavigation(groups:NavigationGroup[],path:string,hash:string){
 for(const group of groups){const item=group.children.find(item=>isNavigationItem(item,path,hash));if(item)return{group,item}}
 return undefined
}
