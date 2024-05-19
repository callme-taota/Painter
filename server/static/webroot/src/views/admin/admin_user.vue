<script setup lang="ts">
//base
import { ref, onMounted, computed, h } from 'vue'
import { NButton, NPagination, NDataTable, NPopselect, NAvatar, useMessage, NTag, type DataTableColumns } from 'naive-ui';
//icon
import { CircleEdit20Regular as editIcon } from '@vicons/fluent';
//api
import { GetUserList, SetUserPermission } from '@/apis/api_setting'
//utils
import type { UserItem } from '@/utils/interface';
//store
const Message = useMessage()
type TagType = "default" | "error" | "primary" | "info" | "success" | "warning";
//ref
const pageLimit = ref<number>(10)
const pageNum = ref<number>(1)
const pageTotal = computed<number>(() => Math.floor(listTotal.value / pageLimit.value) + 1)
const listTotal = ref<number>(0)
const userList = ref<UserItem[]>([])
const createColumns = (): DataTableColumns<UserItem> => {
    return [
        {
            title: '序号',
            key: 'ID',
            width: 60,
            align: 'center',
            fixed: 'left',
        },
        {
            title: '头像',
            key: 'HeaderField',
            width: 60,
            fixed: 'left',
            align: 'center',
            render(row) {
                return h(
                    NAvatar,
                    {
                        size: 'small',
                        src: row.HeaderField,
                    }
                )
            }
        },
        {
            title: '用户ID',
            fixed: 'left',
            key: 'UserName',
        },
        {
            title: '用户昵称',
            fixed: 'left',
            key: 'NickName',
        },
        {
            title: '用户权限',
            width: 120,
            align: 'center',
            fixed: 'right',
            key: 'UserGroup',
            render(row) {
                return h(
                    NPopselect,
                    {
                        options: groupOptions,
                        trigger: "click",
                        onUpdateValue: (newValue) => updateUserPermission(newValue, row)
                    },
                    () => [
                        h(
                            NButton,
                            {
                                type: (() => {
                                    const typeMap: { [key: number]: TagType } = {
                                        1: "error",
                                        2: "success",
                                        3: "info",
                                    };
                                    return typeMap[row.UserGroup] || "default";
                                })() as TagType
                            },
                            () => {
                                type UserGroupMap = {
                                    [key: number]: string;
                                };
                                const map: UserGroupMap = {
                                    1: "管理员",
                                    2: "创作者",
                                    3: "普通用户",
                                };
                                return map[row.UserGroup] || "未知用户组";
                            }
                        )
                    ]
                );
            }

        }
    ]
}
const columns = createColumns()
const groupOptions = [
    {
        label: "管理员",
        value: 1
    },
    {
        label: "创作者",
        value: 2
    },
    {
        label: "普通用户",
        value: 3
    }
]
//hook
onMounted(async () => {
    await getList()
})
//fn
const getList = async () => {
    let limit = pageLimit.value;
    let offset = (pageNum.value - 1) * limit
    let res = await GetUserList({ "Limit": limit, "Offset": offset });
    userList.value = res.data.Users
    listTotal.value = res.data.UserNumber
}

const getWithSizeChange = async (size: number) => {
    pageLimit.value = size
    await getList()
}

const getWithNumChange = async (num: number) => {
    pageNum.value = num
    await getList()
}

const updateUserPermission = async (newValue: number, row: UserItem) => {
    row.UserGroup = newValue
    console.log(row)
    await SetUserPermission({ "UserID": row.ID, "GroupID": newValue })
}
</script>
<template>
    <div>
        <n-data-table :columns="columns" :data="userList"></n-data-table>
        <div style="height: 10px;"></div>
        <div class="admin-pagination">
            <div>

            </div>
            <n-pagination v-model:page="pageNum" :page-count="pageTotal" v-model:page-size="pageLimit" show-size-picker
                :page-sizes="[10, 20, 30]" :on-update:page="getWithNumChange"
                :on-update:page-size="getWithSizeChange" />
        </div>

    </div>
</template>
<style>
.admin-pagination {
    display: flex;
    justify-content: space-between;
}
</style>