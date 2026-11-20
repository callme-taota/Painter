<script setup lang="ts">
import {t} from '../i18n'
import {date,type Row} from '../api'
defineProps<{comment:Row;authenticated:boolean;replyEnabled:boolean;disabled:boolean;owner:boolean;reply?:boolean}>()
defineEmits<{reply:[comment:Row];remove:[comment:Row];like:[comment:Row]}>()
</script>

<template>
 <article :id="'comment-'+comment.id" class="discussion-comment" :class="{'discussion-comment--reply':reply}">
  <a :href="'/users/'+comment.user?.id" class="discussion-avatar" :aria-label="comment.user?.display_name"><img v-if="comment.user?.avatar_url" class="avatar" :src="comment.user.avatar_url" width="36" height="36" alt=""><span v-else class="avatar">{{(comment.user?.display_name||'?').slice(0,1)}}</span></a>
  <div class="discussion-author"><div class="discussion-identity"><a :href="'/users/'+comment.user?.id"><strong>{{comment.user?.display_name}}</strong></a><span v-if="comment.is_author" class="discussion-author-badge">{{t('作者')}}</span></div><time :datetime="comment.created_at">{{date(comment.created_at)}}</time></div>
  <p v-if="reply&&comment.reply_to" class="discussion-reply-context">{{t('回复')}} <a :href="'#comment-'+comment.reply_to.id">{{comment.reply_to.user?.display_name}}</a></p>
  <p v-else-if="reply&&comment.reply_to_deleted" class="discussion-reply-context">{{t('原评论已不可用')}}</p>
  <p class="discussion-body">{{comment.body}}</p>
  <div v-if="authenticated" class="discussion-actions">
   <button class="quiet" :disabled="disabled" :aria-pressed="!!comment.liked" @click="$emit('like',comment)"><svg class="icon" viewBox="0 0 24 24" aria-hidden="true"><path d="M7 21H4V10h3m0 11h10a2 2 0 0 0 2-1.7l1.5-9A2 2 0 0 0 18.5 8H14l1-5-2-1-6 8v11Z"/></svg>{{comment.liked?t('已赞'):t('赞')}} {{comment.like_count||0}}</button>
   <button v-if="replyEnabled" data-comment-reply class="quiet" :disabled="disabled" :aria-label="t('回复 {name}',{name:comment.user?.display_name||''})" @click="$emit('reply',comment)">{{t('回复')}}</button>
   <button v-if="comment.is_self||owner" class="quiet danger" :disabled="disabled" @click="$emit('remove',comment)">{{t('删除')}}</button>
  </div>
 </article>
</template>
