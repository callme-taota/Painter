<script setup lang="ts">
import { NCard, NInput, NForm, NFormItem, NIcon, NButton, useMessage } from 'naive-ui';
import { ref, onMounted } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { Mail, PhonePortraitOutline, PersonOutline, CreateOutline } from '@vicons/ionicons5';
import { useUserStore } from '@/stores/counter';
import { CreateUser } from '@/apis/api_user';

const Route = useRoute()
const Router = useRouter()
const Message = useMessage()
const UserStore = useUserStore()

const email = ref("")
const phone = ref("")
const username = ref("")
const password = ref("")
const userNickName = ref("")
function onlyAllowNumber(value: string) {
    return !value || /^\d+$/.test(value)
}

function setLoginType(value: number) {
    Router.push({ path: "/login", query: { type: value } })
}

async function signup() {
    let kemail = email.value;
    let kphone = phone.value;
    let kusername = username.value;
    let kpassword = password.value;
    let knickname = userNickName.value;
    let headerfield = "";
    let res = await CreateUser({"UserName": kusername, "Password": kpassword, "NickName": knickname, "Email": kemail, "Phone": kphone, "HeaderField": headerfield})
    if (res.ok) {
        Router.push("/login")
        Message.success("Signup successful")
    } else {
        Router.push("home")
        Message.success("Oops!")
    }
}

</script>

<template>
    <n-card title="注册">
        <n-form-item label="用户名">
            <n-input v-model:value="username" type="text" placeholder="请输入用户名"></n-input>
        </n-form-item>
        <n-form-item label="电话号码">
            <n-input v-model:value="phone" type="text" :allow-input="onlyAllowNumber" placeholder="请输入电话号码"></n-input>
        </n-form-item>
        <n-form-item label="邮箱地址">
            <n-input v-model:value="email" type="text" placeholder="请输入邮箱地址"></n-input>
        </n-form-item>
        <n-form-item label="用户昵称">
            <n-input v-model:value="userNickName" type="text" placeholder="请输入用户昵称"></n-input>
        </n-form-item>
        <n-form-item label="密码">
            <n-input type="password" v-model:value="password" show-password-on="mousedown" placeholder="请输入密码"
                :maxlength="8" />
        </n-form-item>
        <n-button @click="signup">
            注册
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
    </div>
</template>

<style>
.login-under-btns-cont {
    padding: 0px 16px;
    display: flex;
    justify-content: space-around;
}
</style>