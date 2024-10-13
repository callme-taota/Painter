<script setup lang="ts">
// base
import { ref, onMounted, onUnmounted, Transition } from 'vue'
import { NSteps, NStep, NInput, NDropdown, NButton, useMessage } from 'naive-ui';
import { useRouter } from 'vue-router';
import './auth.css'
// icon

// api

// utils
import { validateEmail, validatePassword, validateUserID } from '@/utils/check';
// store
import { useUserStore } from '@/stores/user';
import { useThemeStore } from '@/stores/theme';
import { CreateUser } from '@/apis/api_user';
import { useInfoStore } from '@/stores/info';
const UserStore = useUserStore()
const ThemeStore = useThemeStore()
const InfoStore = useInfoStore()
const Message = useMessage()
const Router = useRouter()
// ref
const current = ref<number>(1)

const userMail = ref<string>("")
const isMailExist = ref<boolean>(false)

const userID = ref<string>("")
const userIDExist = ref<boolean>(false)

const userPass = ref<string>("")
const userPass2nd = ref<string>("")
const userPassEqual = ref<boolean>(true)
const userPassOK = ref<boolean>(true)

const dropDownOptions1st = ref([
    {
        label: '已有账号',
        key: 'login',
    },
])
const dropDownOptions2nd = ref([
    {
        label: '上一步',
        key: 'back',
    },
])
// hook
onMounted(() => {
    if (InfoStore.can_register != true) {
        Message.info("当前站点未开启注册功能")
        Router.push({ path: "/" })
    }
    ThemeStore.hideHeader();
})
onUnmounted(() => {
    ThemeStore.showHeader();
})
// fn
const handleDropDownSelect = async (key: string) => {
    switch (key) {
        case 'login':
            goLogin()
            return
        case 'back':
            current.value = current.value - 1
            return
        default:
            return
    }
}

const goLogin = () => {
    Router.push({ path: "/login" })
}

const firstStep = async () => {
    let has = await UserStore.checkExist(userMail.value)
    if (has) {
        isMailExist.value = true
    } else {
        let isMail = validateEmail(userMail.value)
        if (isMail) {
            isMailExist.value = false
            current.value = current.value + 1
        } else {
            isMailExist.value = true
        }
    }
}

const secondStep = async () => {
    let has = await UserStore.checkExist(userID.value)
    if (has) {
        userIDExist.value = true
    } else {
        isMailExist.value = false
        current.value = current.value + 1
    }
}

const allowedID = (value: string) => !value || validateUserID(value)

const thirdStep = async () => {
    let pass = userPass.value
    let pass2nd = userPass2nd.value
    if (pass != pass2nd) {
        userPassEqual.value = false
    } else {
        let isPasswordOK = validatePassword(pass)
        if (isPasswordOK) {
            let res = await CreateUser({ "UserName": userID.value, "Password": userPass.value, "NickName": userID.value, "Email": userMail.value, "Phone": "", "HeaderField": "" })
            if (res.ok) {
                current.value = current.value + 1
                setTimeout(() => {
                    goLogin()
                }, 1000);
            } else {
                Message.error("不对劲，十分有十二分不对劲")
            }
        } else {
            userPassOK.value = false
        }
    }
}

</script>
<template>
    <div class="page-cont">
        <div class="login-cont">
            <div class="login-left">
                <n-steps vertical :current="current">
                    <n-step title="账号邮箱" description="请输入您的邮箱地址" />
                    <n-step title="设置账号" description="请输入您的个人信息" />
                    <n-step title="输入密码" description="请输入您的登录密码" />
                    <n-step title="注册成功" description="恭喜您，注册成功！" />
                </n-steps>
            </div>
            <div class="login-right">
                <Transition>
                    <div>
                        <div v-if="current == 1">
                            <div class="painter-login">
                                Painter 注册
                            </div>
                            <div class="login-account-text">
                                请输入您的邮箱地址
                            </div>
                            <n-input v-model:value="userMail" placeholder="电子邮件" authentication="mail"></n-input>
                            <div v-if="isMailExist" style="color: var(--highlight-color);">
                                该邮箱已被绑定或格式错误
                            </div>
                            <div class="login-btn-cont">
                                <n-dropdown trigger="click" :options="dropDownOptions1st"
                                    @select="handleDropDownSelect">
                                    <n-button text>
                                        其他
                                    </n-button>
                                </n-dropdown>
                                <n-button color="#2080f0" style="color: #fff;" type="info" @click="firstStep">
                                    下一步
                                </n-button>
                            </div>
                        </div>
                        <div v-if="current == 2">
                            <div class="painter-login">
                                欢迎来到 Painter
                            </div>
                            <div class="login-account-text">
                                我们还有些设置
                            </div>
                            <div class="login-account-text">
                                用户ID
                            </div>
                            <n-input v-model:value="userID" placeholder="用户ID" :allow-input="allowedID"></n-input>
                            <div v-if="userIDExist" style="color: var(--highlight-color);">
                                请换一个试试
                            </div>
                            <div class="login-btn-cont">
                                <n-dropdown trigger="click" :options="dropDownOptions2nd"
                                    @select="handleDropDownSelect">
                                    <n-button text>
                                        其他
                                    </n-button>
                                </n-dropdown>
                                <n-button color="#2080f0" style="color: #fff;" type="info" @click="secondStep">
                                    下一步
                                </n-button>
                            </div>
                        </div>
                        <div v-if="current == 3">
                            <div class="painter-login">
                                欢迎你 {{ userID }}
                            </div>
                            <div class="login-account-text">
                                密码
                            </div>
                            <n-input v-model:value="userPass" placeholder="密码" type="password" authentication="password"
                                show-password-on="click"></n-input>
                            <div class="login-account-text">
                                再次输入
                            </div>
                            <n-input v-model:value="userPass2nd" placeholder="验证密码" type="password"
                                authentication="password" show-password-on="click"></n-input>
                            <div class="login-account-text">
                                · 至少8位
                                <br>
                                · 同时有数字和字母
                            </div>
                            <div v-if="!userPassEqual" style="color: var(--highlight-color);">
                                请检查密码
                            </div>
                            <div v-if="!userPassOK" style="color: var(--highlight-color);">
                                密码安全性不足
                            </div>
                            <div class="login-btn-cont">
                                <n-dropdown trigger="click" :options="dropDownOptions2nd"
                                    @select="handleDropDownSelect">
                                    <n-button text>
                                        其他
                                    </n-button>
                                </n-dropdown>
                                <n-button color="#2080f0" style="color: #fff;" type="info" @click="thirdStep">
                                    下一步
                                </n-button>
                            </div>
                        </div>
                        <div v-if="current == 4">
                            <div class="painter-login">
                                注册成功
                            </div>
                            <div class="login-account-text">
                                即将跳转到登录页
                            </div>
                        </div>
                    </div>
                </Transition>
            </div>
        </div>
    </div>
</template>
<style></style>