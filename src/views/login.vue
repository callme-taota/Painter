<script setup lang="ts">
import { NCard, NInput, NForm, NFormItem, NIcon, NButton } from 'naive-ui';
import { ref, onMounted } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { Mail, PhonePortraitOutline, PersonOutline, CreateOutline } from '@vicons/ionicons5';
import { useUserStore } from '@/stores/counter';

const Route = useRoute()
const Router = useRouter()
const UserStore = useUserStore()

const loginType = ref(1) // 1 -> email, 2-> username, 3 -> phone
const email = ref("")
const phone = ref("")
const username = ref("")
const password = ref("")
function onlyAllowNumber(value: string) {
    return !value || /^\d+$/.test(value)
}

onMounted(() => {
    let type = Route.query.type
    if (type != null && typeof type == "string") {
        loginType.value = parseInt(type)
    }
})

function setLoginType(value: number) {
    loginType.value = value
}

function goSignUp(){
    Router.push("/signup")
}

function login(){
    let type = loginType.value
    if (type == 1) {
        UserStore.loginWithEmail(email.value, password.value)
    } else if (type == 2) {
        UserStore.loginWithUName(username.value, password.value)
    } else {
        UserStore.loginWithPhone(phone.value, password.value)
    }
    Router.push({ path: "/user" })
}

</script>

<template>
    <n-card title="登陆">
        <n-form-item label="邮箱" v-if="loginType == 1">
            <n-input v-model:value="email" type="text" placeholder="请输入邮箱地址"></n-input>
        </n-form-item>
        <n-form-item label="用户名" v-if="loginType == 2">
            <n-input v-model:value="username" type="text" placeholder="请输入用户名"></n-input>
        </n-form-item>
        <n-form-item label="电话号码" v-if="loginType == 3">
            <n-input v-model:value="phone" :allow-input="onlyAllowNumber" type="text" placeholder="请输入电话号码"></n-input>
        </n-form-item>
        <n-form-item label="密码">
            <n-input type="password" v-model:value="password" show-password-on="mousedown" placeholder="请输入密码"
                :maxlength="8" />
        </n-form-item>
        <n-button @click="login">
            登陆
        </n-button>
    </n-card>
    <div class="login-under-btns-cont">
        <n-button text @click="setLoginType(1)">
            <template #icon>
                <n-icon>
                    <mail />
                </n-icon>
            </template>
            邮箱登陆
        </n-button>
        <n-button text @click="setLoginType(2)">
            <template #icon>
                <n-icon>
                    <PersonOutline />
                </n-icon>
            </template>
            用户名登陆
        </n-button>
        <n-button text @click="setLoginType(3)">

            <template #icon>
                <n-icon>
                    <PhonePortraitOutline />
                </n-icon>
            </template>
            电话登陆
        </n-button>
        <n-button text @click="goSignUp">
            <template #icon>
                <n-icon>
                    <CreateOutline />
                </n-icon>
            </template>
            注册
        </n-button>
    </div>
</template>

<style>
.login-under-btns-cont {
    padding: 0px 16px;
    display: flex;
    justify-content: space-around;
}
</style>