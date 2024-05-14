<script setup>
import { NH1, NTabs, NTabPane, NTable, NButton, NModal, NCard, NInput, NPagination, NSelect, NPopconfirm } from "naive-ui";
import { ref, onMounted, computed } from 'vue';
import { TagList, CreateTag, UpdateTagName, UpdateTagDesc, TagListFull } from "@/apis/api_tag";
import { CategoryList, CategoryCreate, CategoryUpdateName, CategoryUpdateDesc, GetCategoriesList } from "@/apis/api_category";
import { GetSelfArticle, CreateArticle, GetArticle, ArticleDelete, ArticleUpdateTitle, ArticleUpdateContent, ArticleUpdateSummary } from "@/apis/api_article";

onMounted(() => {
    getTags()
    getCates()
    getArticle()
})

//modal 
const modalVis = ref(false)
const inputValue = ref("")
const inputType = ref(1)
const inputTarget = ref(null)

async function submitModal() {
    let type = inputType.value
    let t = inputTarget.value
    if (type == 1) {
        let res = await UpdateTagName({ "TagID": parseInt(t.TagID), "Name": inputValue.value })
        getTags()
    } else if (type == 2) {
        let res = await UpdateTagDesc({ "TagID": parseInt(t.TagID), "Description": inputValue.value })
        getTags()
    } else if (type == 3) {
        let res = await CreateTag({ "Name": inputValue.value, "Description": inputValue.value })
        getTags()
    } else if (type == 4) {
        let res = await CategoryUpdateName({ "OldName": t.CategoryName, "NewName": inputValue.value })
        getCates()
    } else if (type == 5) {
        let res = await CategoryUpdateDesc({ "Name": t.CategoryName, "Description": inputValue.value })
        getCates()
    } else if (type == 6) {
        let res = await CategoryCreate({ "Name": inputValue.value, "Description": inputValue.value })
        getCates()
    }
    modalVis.value = false
}

async function change(target, type) {
    if (type == 1) {
        inputValue.value = target.TagName
    } else if (type == 2) {
        inputValue.value = target.Description
    } else if (type == 3) {

    } else if (type == 4) {
        inputValue.value = target.CategoryName
    } else if (type == 5) {
        inputValue.value = target.Description
    }
    inputType.value = type
    inputTarget.value = target
    modalVis.value = true
}

// tag pane
const tagPaneLimit = ref(10);
const tagPanePageNum = ref(1);
const tagPanePageListTotal = ref(0);
const tagPaneTotal = computed(() => {
    return Math.floor(tagPanePageListTotal.value / tagPaneLimit.value) + 1
});
const tagList = ref([])

async function getTagWithSizeChange(size) {
    tagPaneLimit.value = size
    await getTags()
}

async function getTagWithNumChange(num) {
    tagPanePageNum.value = num
    await getTags()
}

async function getTags() {
    let limit = tagPaneLimit.value;
    let offset = (tagPanePageNum.value - 1) * limit
    let res = await TagList({ "Limit": limit, "Offset": offset });
    tagList.value = res.data.Tags
    tagPanePageListTotal.value = res.data.TagNumber
}

//category pane
const catePaneLimit = ref(10);
const catePanePageNum = ref(1);
const catePanePageListTotal = ref(0);
const catePaneTotal = computed(() => {
    return Math.floor(catePanePageListTotal.value / catePaneLimit.value) + 1
});
const cateList = ref([])

async function getCateWithSizeChange(size) {
    catePaneLimit.value = size
    await getCates()
}

async function getCateWithNumChange(num) {
    catePanePageNum.value = num
    await getCates()
}

async function getCates() {
    let limit = catePaneLimit.value;
    let offset = (catePanePageNum.value - 1) * limit
    let res = await CategoryList({ "Limit": limit, "Offset": offset });
    cateList.value = res.data.categories
    catePanePageListTotal.value = res.data.categoriesNumber
}

//article pane
const articlePaneLimit = ref(10);
const articlePanePageNum = ref(1);
const articlePanePageListTotal = ref(0);
const articlePaneTotal = computed(() => {
    return Math.floor(articlePanePageListTotal.value / articlePaneLimit.value) + 1
});
const articleList = ref([])

const articleModalVis = ref(false)
const articleUpdateTitleModalVis = ref(false)
const articleUpdateContentModalVis = ref(false)
const articleTitle = ref("")
const articleContent = ref("")
const articleCategory = ref(null)
const articleCategoryOptions = ref([])
const articleTags = ref([])
const articleTagOptions = ref([])
const articleSummary = ref("")
const articleChange = ref("")
const articleChangeType = ref(1)
const articleTarget = ref(null)

async function getarticleWithSizeChange(size) {
    articlePaneLimit.value = size
    await getArticle()
}

async function getarticleWithNumChange(num) {
    articlePanePageNum.value = num
    await getArticle()
}

async function getArticle() {
    let limit = articlePaneLimit.value;
    let offset = (articlePanePageNum.value - 1) * limit
    let res = await GetSelfArticle({ "Limit": limit, "Offset": offset });
    articleList.value = res.data.ArticleList
    articlePanePageListTotal.value = res.data.ArticleCount
}

const createArticle = () => {
    articleModalVis.value = true
}

const getCategoryList = async () => {
    let res = await GetCategoriesList()
    console.log(res)
    articleCategoryOptions.value = res.data.categories
}

const getTagList = async () => {
    let res = await TagListFull()
    console.log(res)
    articleTagOptions.value = res.data.Tags
}

const articleSubmit = async () => {
    let res = await CreateArticle({
        Title: articleTitle.value,
        Content: articleContent.value,
        Summary: articleSummary.value,
        CatrgoryID: articleCategory.value,
        Tags: articleTags.value
    })
    articleModalVis.value = false
}

const UpdateArticle = async (target, type) => {
    let ArticleID = target.ArticleTable.ArticleID
    articleTarget.value = target
    articleChangeType.value = type
    if (type == 1) {
        articleUpdateTitleModalVis.value = true
        articleTitle.value = target.ArticleTable.Title
    } else if (type == 2) {
        articleUpdateContentModalVis.value = true
        let res = await GetArticle({ ArticleID })
        articleChange.value = res.data.ArticleContentTable.Content
    } else if (type == 3) {
        articleUpdateContentModalVis.value = true
        articleChange.value = target.ArticleTable.Summary
    } else if (type == 4) {
        let res = await ArticleDelete({ ArticleID })
    }
}

const handleNegativeClick = () => { }

const submitArticleModal = async () => {
    let ArticleID = articleTarget.value.ArticleTable.ArticleID
    let type = articleChangeType.value
    if (type == 1) {
        let res = await ArticleUpdateTitle({ ArticleID, Title: articleTitle.value })
        articleUpdateTitleModalVis.value = false
    } else if (type == 2) {
        let res = await ArticleUpdateContent({ ArticleID, Content: articleChange.value })
        articleUpdateContentModalVis.value = false
    } else if (type == 3) {
        let res = await ArticleUpdateSummary({ ArticleID, Summary: articleChange.value })
        articleUpdateContentModalVis.value = false
    }
}

</script>
<template>
    <div class="page-cont">

        <n-h1>管理</n-h1>
        <n-tabs type="line" animated>
            <n-tab-pane name="tag" tab="标签">
                <n-button @click="change({}, 3)">新增</n-button>
                <br>
                <n-table>
                    <thead>
                        <tr>
                            <th>ID</th>
                            <th>标签名</th>
                            <th>标签说明</th>
                            <th>动作</th>
                        </tr>
                    </thead>
                    <tbody>
                        <tr v-for="t in tagList">
                            <td>{{ t.TagID }}</td>
                            <td>{{ t.TagName }}</td>
                            <td>{{ t.Description }}</td>
                            <td>
                                <n-button @click="change(t, 1)">改名</n-button> &nbsp;
                                <n-button @click="change(t, 2)">改介绍</n-button>
                            </td>
                        </tr>
                    </tbody>
                </n-table>
                <br>
                <n-pagination v-model:page="tagPanePageNum" :page-count="tagPaneTotal" v-model:page-size="tagPaneLimit"
                    show-size-picker :page-sizes="[5, 20, 30, 40]" :on-update:page="getTagWithNumChange"
                    :on-update:page-size="getTagWithSizeChange" />
            </n-tab-pane>
            <n-tab-pane name="cate" tab="类别">
                <n-button @click="change({}, 6)">新增</n-button>
                <br>
                <n-table>
                    <thead>
                        <tr>
                            <th>ID</th>
                            <th>标签名</th>
                            <th>标签说明</th>
                            <th>动作</th>
                        </tr>
                    </thead>
                    <tbody>
                        <tr v-for="c in cateList">
                            <td>{{ c.CategoryID }}</td>
                            <td>{{ c.CategoryName }}</td>
                            <td>{{ c.Description }}</td>
                            <td>
                                <n-button @click="change(c, 4)">改名</n-button> &nbsp;
                                <n-button @click="change(c, 5)">改介绍</n-button>
                            </td>
                        </tr>
                    </tbody>
                </n-table>
                <br>
                <n-pagination v-model:page="catePanePageNum" :page-count="catePaneTotal"
                    v-model:page-size="catePaneLimit" show-size-picker :page-sizes="[10, 20, 30, 40]"
                    :on-update:page="getCateWithNumChange" :on-update:page-size="getCateWithSizeChange" />
            </n-tab-pane>
        </n-tabs>

        <n-modal v-model:show="modalVis">
            <n-card style="width: 500px; position: fixed; left: 50%;top: 50%;transform: translate(-50%,-50%);"
                title="更改" :bordered="false" size="huge" role="dialog" aria-modal="true">
                <div class="input-area">
                    <n-input placeholder="请输入" maxlength="100" show-count v-model:value="inputValue"></n-input>
                    <div style="width: 20px;"></div>
                    <n-button @click="submitModal">提交</n-button>
                </div>
            </n-card>
        </n-modal>

        <n-modal v-model:show="articleUpdateTitleModalVis">
            <n-card style="width: 500px; position: fixed; left: 50%;top: 50%;transform: translate(-50%,-50%);"
                title="更改" :bordered="false" size="huge" role="dialog" aria-modal="true">
                <div class="input-area">
                    <n-input placeholder="请输入" v-model:value="articleTitle"></n-input>
                    <div style="width: 20px;"></div>
                    <n-button @click="submitArticleModal">提交</n-button>
                </div>
            </n-card>
        </n-modal>

        <n-modal v-model:show="articleUpdateContentModalVis">
            <n-card style="width: 500px; position: fixed; left: 50%;top: 50%;transform: translate(-50%,-50%);"
                title="更改" :bordered="false" size="huge" role="dialog" aria-modal="true">
                <div class="input-area">
                    <n-input placeholder="请输入" v-model:value="articleChange" :autosize="{
                        minRows: 3
                    }" type="textarea"></n-input>
                    <div style="width: 20px;"></div>
                    <n-button @click="submitArticleModal">提交</n-button>
                </div>
            </n-card>
        </n-modal>
    </div>

</template>
<style>
.input-area {
    display: flex;
    flex-direction: row;
    align-items: center;
}
</style>