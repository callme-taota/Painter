<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { type ArticleInfoItem } from '@/utils/interface'
import { dateToDescription } from '@/utils/timeToStr'
import { NIcon } from 'naive-ui'
import { AccessTimeFilledSharp, ThumbUpAltOutlined as ThumbUp, CommentOutlined as Comment } from '@vicons/material'
import { useRouter } from 'vue-router'

const Router = useRouter()

defineProps<{
    article: ArticleInfoItem
}>()

function goPage(id : number) {
    Router.push({ path: "/article", query: { id: id } })
}

</script>
<template>
    <div class="article-card" @click="goPage(article.ArticleTable.ArticleID)">
        <div class="article-card-cont">
            <div class="article-card-title">
                {{ article.ArticleTable.Title }}
            </div>
            <div class="article-card-summary">
                {{ article.ArticleTable.Summary }}
            </div>
            <div class="article-more-cont">
                <div class="article-more-item">
                    <n-icon size="14">
                        <AccessTimeFilledSharp />
                    </n-icon>
                    {{ dateToDescription(article.ArticleTable.CreatedAt) }}
                </div>
                <div class="article-more-item">
                    <n-icon size="14">
                        <ThumbUp />
                    </n-icon>
                    {{ article.LikesNumber }}
                </div>
                <div class="article-more-item">
                    <n-icon size="14">
                        <Comment />
                    </n-icon>
                    {{ article.CommentNumber }}
                </div>
            </div>
            <div class="article-card-tag">
                <span class="article-card-tag-cont" v-for="tag in article.ArticleTagTable">
                    {{ tag.TagName }}
                </span>
            </div>
        </div>
    </div>
</template>
<style>
.article-card {
    transition: 0.3s;
    width: 49%;
    height: 155px;
    background: var(--card--background);
    border-radius: 20px;
    margin-bottom: 20px;
    cursor: pointer;
}

.article-card:hover {
    box-shadow: 0 0 6px var(--border-color);
}

.article-card-cont {
    padding: 26px;
}

.article-card-title {
    line-height: 36px;
    height: 36px;
    font-size: 22px;
    font-weight: bolder;
    text-overflow: ellipsis;
    overflow: hidden;
}

.article-card-summary {
    width: 100%;
    text-overflow: ellipsis;
    overflow: hidden;
}

.article-card-tag {
    width: 100%;
    height: 24px;
    text-overflow: ellipsis;
    overflow: hidden;
}

.article-more-cont {
    display: flex;
    justify-content: space-between;
}

.article-more-item {
    display: flex;
    align-items: center;
}

.article-card-tag-cont {
    line-height: 24px;
    padding: 6px 8px;
    margin: 2px 4px 2px 0;
    background: var(--btn-hover-grey);
    border-radius: 12px;
}

@media (max-width: 1000px) {
    .article-card {
        width: 100%;
    }
}
</style>