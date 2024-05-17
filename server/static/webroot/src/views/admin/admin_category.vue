<script setup lang="ts">
//base
import { ref, onMounted, computed, h } from 'vue'
import { NButton, NPagination, NDataTable, NIcon, NModal, NCard, NInput, useMessage, type DataTableColumns } from 'naive-ui';
//icon
import { CircleEdit20Regular as editIcon, Add24Filled as addIcon } from '@vicons/fluent';
//api
import { CategoryList, CategoryCreate, CategoryUpdate } from "@/apis/api_category";
//utils
import type { CategoryItem } from '@/utils/interface';
//store
const Message = useMessage()
//ref
const pageLimit = ref<number>(10)
const pageNum = ref<number>(1)
const pageTotal = computed<number>(() => Math.floor(listTotal.value / pageLimit.value) + 1)
const listTotal = ref<number>(0)
const categoryList = ref<CategoryItem[]>([])
const createColumns = ({
    edit,
}: {
    edit: (row: CategoryItem) => void,
}): DataTableColumns<CategoryItem> => {
    return [
        {
            title: '类别号',
            key: 'CategoryID',
            width: 80,
            fixed: 'left',
            align: 'center'
        },
        {
            title: '类别名',
            fixed: 'left',
            key: 'CategoryName',
            width: 120,
        },
        {
            fixed: 'left',
            title: '介绍',
            key: 'Description',
        },
        {
            title: '操作',
            key: 'actions',
            align: 'center',
            fixed: 'right',
            width: 80,
            render(row) {
                return [
                    h(
                        NButton,
                        {
                            strong: true,
                            tertiary: true,
                            size: 'small',
                            onClick: () => edit(row)
                        },
                        {
                            default: () => h(
                                NIcon,
                                {
                                    component: editIcon
                                }
                            )
                        }
                    ),
                ]
            }
        }
    ]
}
const columns = createColumns({
    edit(row: CategoryItem) {
        editModalItem.value = {
            "CategoryID": row.CategoryID,
            "Name": row.CategoryName,
            "Description": row.Description
        }
        showEditModal.value = true
    },
})
const showAddModal = ref<boolean>(false)
const addModalItem = ref({
    "Name": "",
    "Description": ""
})
const showEditModal = ref<boolean>(false)
const editModalItem = ref({
    "CategoryID": 0,
    "Name": "",
    "Description": ""
})
//hook
onMounted(async () => {
    await getCategories()
})
//fn
const getCategories = async () => {
    let limit = pageLimit.value;
    let offset = (pageNum.value - 1) * limit
    let res = await CategoryList({ "Limit": limit, "Offset": offset });
    categoryList.value = res.data.categories
    listTotal.value = res.data.categoriesNumber
}

const getWithSizeChange = async (size: number) => {
    pageLimit.value = size
    await getCategories()
}

const getWithNumChange = async (num: number) => {
    pageNum.value = num
    await getCategories()
}

const submitEdit = async () => {
    let obj = editModalItem.value
    let res = await CategoryUpdate(obj)
    if (res.ok) {
        Message.info("修改成功")
    } else {
        Message.info("修改失败")
    }
    await getCategories()
    showEditModal.value = false
}

const submitAdd = async () => {
    let obj = addModalItem.value
    let res = await CategoryCreate(obj)
    if (res.ok) {
        Message.info("新增成功")
    } else {
        Message.info("新增失败")
    }
    addModalItem.value = {
        "Name": "",
        "Description": ""

    }
    await getCategories()
    showAddModal.value = false
}
</script>
<template>
    <div>
        <n-data-table :columns="columns" :data="categoryList"></n-data-table>
        <div style="height: 10px;"></div>
        <div class="admin-pagination">
            <n-button @click="showAddModal = true">
                <n-icon>
                    <addIcon />
                </n-icon>
            </n-button>
            <n-pagination v-model:page="pageNum" :page-count="pageTotal" v-model:page-size="pageLimit" show-size-picker
                :page-sizes="[10, 20, 30]" :on-update:page="getWithNumChange"
                :on-update:page-size="getWithSizeChange" />
        </div>

    </div>
    <!-- add -->
    <n-modal v-model:show="showAddModal">
        <n-card title="新增分类"
            style="width: 500px;  position: fixed; left: 50%;top: 50%;transform: translate(-50%,-50%);">
            <div>分类名</div>
            <n-input v-model:value="addModalItem.Name" placeholder="分类名" />
            <div>分类介绍</div>
            <n-input v-model:value="addModalItem.Description" placeholder="分类介绍" />
            <div style="height: 10px;"></div>
            <n-button @click="submitAdd">新增</n-button>
        </n-card>
    </n-modal>

    <!-- edit -->
    <n-modal v-model:show="showEditModal">
        <n-card title="修改分类"
            style="width: 500px;  position: fixed; left: 50%;top: 50%;transform: translate(-50%,-50%);">
            <div>分类名</div>
            <n-input v-model:value="editModalItem.Name" placeholder="分类名" />
            <div>分类介绍</div>
            <n-input v-model:value="editModalItem.Description" placeholder="分类介绍" />
            <div style="height: 10px;"></div>
            <n-button @click="submitEdit">提交</n-button>
        </n-card>
    </n-modal>

</template>
<style>
.admin-pagination {
    display: flex;
    justify-content: space-between;
}
</style>