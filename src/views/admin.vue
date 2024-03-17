<script setup lang="js">
import { NH1, NTabs, NTabPane, NTable, NButton, NModal, NCard, NInput, NPagination } from "naive-ui";
import { ref, onMounted, computed } from 'vue';
import { TagList, CreateTag, UpdateTagName, UpdateTagDesc } from "@/apis/api_tag";
import { CategoryList, CategoryCreate, CategoryUpdateName, CategoryUpdateDesc } from "@/apis/api_category";

onMounted(() => {
    getTags()
    getCates()
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
    console.log(res)
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

</script>
<template>
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
            <n-pagination v-model:page="catePanePageNum" :page-count="catePaneTotal" v-model:page-size="catePaneLimit"
                show-size-picker :page-sizes="[10, 20, 30, 40]" :on-update:page="getCateWithNumChange"
                :on-update:page-size="getCateWithSizeChange" />
        </n-tab-pane>
    </n-tabs>

    <n-modal v-model:show="modalVis">
        <n-card style="width: 500px; position: fixed; left: 50%;top: 50%;transform: translate(-50%,-50%);" title="更改"
            :bordered="false" size="huge" role="dialog" aria-modal="true">
            <div class="input-area">
                <n-input placeholder="请输入" maxlength="100" show-count v-model:value="inputValue"></n-input>
                <div style="width: 20px;"></div>
                <n-button @click="submitModal">提交</n-button>
            </div>
        </n-card>
    </n-modal>

</template>
<style>
.input-area {
    display: flex;
    flex-direction: row;
    align-items: center;
}
</style>