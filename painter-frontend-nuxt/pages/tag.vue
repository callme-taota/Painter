<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { TagListFull } from "@/apis/api_tag"
import { useRouter } from 'vue-router'

const Router = useRouter()

onMounted(async () => {
    getTagList()
})

const tagList = ref<TagListItem[]>([])

interface TagListItem {
    ArticleCount: number,
    TagName: string,
    Description: string,
    TagID: number,
}

async function getTagList() {
    let res = await TagListFull({})
    let list = res.data.Tags
    tagList.value = list
}

function toPage(id: number) {
    Router.push({ path: '/articleList', query: { type: 2, id: id } })
}

</script>


<template>
    <div class="catrgories-header">
        <h1>标签</h1>
    </div>
    <div class="tags-cont">
        <div class="tag-item" v-for="c in tagList" @click="toPage(c.TagID)">
            <div class="tag-item-name">
                {{ c.TagName }}
            </div>
            <div class="tag-item-count">
                {{ c.ArticleCount }}
            </div>
        </div>
    </div>
</template>

<style>
.catrgories-header {
    padding: 0 120px;
}

.tags-cont {
    padding: 20px 160px;
    display: flex;
    justify-content: center;
    flex-wrap: wrap;
}

.tag-item {
    padding: 6px 10px 6px 30px;
    background: var(--card--background);
    height: 32px;
    line-height: 32px;
    margin: 10px;
    border-radius: 22px;
    display: flex;
    align-items: center;
    justify-content: space-between;
    cursor: pointer;
    transition: 0.3s;
    box-shadow: 0 2px 16px -3px rgba(0, 0, 0, .2);
}

.tag-item:hover {
    transform: scale(1.1);
    color: var(--color-rev);
    font-weight: bold;
    background: var(--base-hover-background);
}

.tag-item-name {
    padding-right: 10px;
}

.tag-item-count {
    background: var(--btn-hover-grey);
    width: 32px;
    height: 32px;
    text-align: center;
    border-radius: 50%;
}
</style>