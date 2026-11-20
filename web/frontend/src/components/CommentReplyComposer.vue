<script setup lang="ts">
import {computed,onMounted,ref} from 'vue'
import {t} from '../i18n'
const props=defineProps<{modelValue:string;targetId:string;name:string;busy:boolean;disabled:boolean}>()
const emit=defineEmits<{'update:modelValue':[value:string];submit:[];cancel:[]}>()
const input=ref<HTMLTextAreaElement>()
const count=computed(()=>Array.from(props.modelValue).length)
onMounted(()=>input.value?.focus({preventScroll:true}))
</script>

<template>
 <form class="discussion-reply-composer" :data-reply-to="targetId" @submit.prevent="emit('submit')" @keydown.esc.stop.prevent="!busy&&emit('cancel')">
  <label :for="'reply-body-'+targetId">{{t('回复 {name}',{name})}}</label>
  <textarea :id="'reply-body-'+targetId" ref="input" :value="modelValue" :disabled="busy" rows="3" required :placeholder="t('写下你的回复')" :aria-invalid="count>5000" :aria-describedby="'reply-count-'+targetId" @input="emit('update:modelValue',($event.target as HTMLTextAreaElement).value)"></textarea>
  <div class="discussion-reply-toolbar"><small :id="'reply-count-'+targetId" :class="{'discussion-limit':count>5000}">{{count}} / 5000</small><div><button type="button" class="quiet" :disabled="busy" @click="emit('cancel')">{{t('取消')}}</button><button class="primary" :disabled="disabled||busy||!modelValue.trim()||count>5000">{{busy?t('提交中…'):t('发表回复')}}</button></div></div>
 </form>
</template>
