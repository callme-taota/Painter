<script setup lang="ts">
import { GetArticle, ArticleLike, ArticleLikeCheck } from '@/apis/api_article';
import { CollectionCheck, Collection } from '@/apis/api_collection';
import { GetComments, CreateComment, DeleteComment, LikeComment, DisLikeComment, GetCommentsL } from '@/apis/api_comment';
import { ref, onMounted, computed } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { ArrowForward, ThumbsUpSharp, ChatboxEllipsesOutline, Star } from "@vicons/ionicons5"
import { NH1, NH5, NH6, NIcon, NTag, NDivider, NModal, NCard, NInput, NButton, NAvatar, NText } from 'naive-ui';
import { numTimeToString } from "@/utils/timeToStr"
import { useUserStore } from '@/stores/user';

const Route = useRoute()
const Router = useRouter()
const UserStore = useUserStore()

const articleID = ref()
const fullArticle = ref({})
const commentBox = ref(false)
const commentInput = ref("")
const comments = ref([])


</script>
<template>
    <div>
    </div>
</template>
<style>
</style>