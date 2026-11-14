<script setup lang="ts">
withDefaults(defineProps<{name:string;size?:number}>(),{size:20})
const paths:Record<string,string>={
 menu:'M4 6h16M4 12h16M4 18h16',
 close:'m6 6 12 12M6 18 18 6',
 quote:'M4 5h6v7H6v6H3v-7l1-6Zm11 0h6v7h-4v6h-3v-7l1-6Z',
 posts:'M6 3h12a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2ZM8 7h8M8 11h8M8 15h5',
 pen:'m16 3 5 5M4 20l4-1L21 6a2.1 2.1 0 0 0-3-3L5 16l-1 4Z',
 user:'M16 7a4 4 0 1 1-8 0 4 4 0 0 1 8 0ZM4 21v-2a8 8 0 0 1 16 0v2',
 bookmark:'M6 3h12v18l-6-4-6 4V3Z',
 users:'M16 21v-2a6 6 0 0 0-12 0v2M14 4a4 4 0 0 1 0 8M20 21v-2a6 6 0 0 0-3-5M14 8a4 4 0 1 1-8 0 4 4 0 0 1 8 0Z',
 history:'M3 11a9 9 0 1 1 2 7M3 4v7h7M12 7v5l3 2',
 sessions:'M3 4h18v13H3V4ZM8 21h8M12 17v4',
 image:'M3 3h18v18H3V3Zm0 13 6-6 5 5 3-3 4 4M16 7h.01',
 settings:'m12 3 2 3 4-.2.5 4L21 12l-2.5 2.2-.5 4-4-.2-2 3-2-3-4 .2-.5-4L3 12l2.5-2.2.5-4 4 .2 2-3ZM15 12a3 3 0 1 1-6 0 3 3 0 0 1 6 0Z',
 tag:'M3 3h7l11 11-7 7L3 10V3Zm5 5h.01',
 shield:'m12 3 8 3v6c0 5-8 9-8 9S4 17 4 12V6l8-3Zm-4 9 3 3 5-6',
 comment:'M21 11a8 8 0 0 1-8 8H7l-5 3 2-6a8 8 0 1 1 17-5Z',
 chart:'M3 3v18h18M7 16v-4M12 16V7M17 16v-7',
 server:'M3 3h18v7H3V3Zm0 11h18v7H3v-7ZM7 6.5h.01M7 17.5h.01M11 6.5h6M11 17.5h6',
 logout:'M9 3H4v18h5M14 8l5 4-5 4M8 12h11',
 chevron:'m7 10 5 5 5-5',
 undo:'M4 10h9a6 6 0 0 1 0 12M4 10l5-5M4 10l5 5',
 redo:'M20 10h-9a6 6 0 0 0 0 12M20 10l-5-5M20 10l-5 5',
 search:'M17 10a7 7 0 1 1-14 0 7 7 0 0 1 14 0Zm-2 5 6 6',
 export:'M12 3v12m-5-5 5 5 5-5M4 16v5h16v-5',
 arrow:'M4 12h16m-6-6 6 6-6 6'
}
</script>
<template><svg class="ui-icon" :width="size" :height="size" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.65" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path :d="paths[name]||paths.posts"/></svg></template>
