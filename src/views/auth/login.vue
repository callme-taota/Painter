<script setup lang="ts">
// base
import { ref, onMounted, onUnmounted, Transition } from 'vue'
import { NSteps, NStep, NInput, NDropdown, NButton, useMessage, NInputNumber } from 'naive-ui';
import { useRouter } from 'vue-router';
import { LoginUserWithEmailCheck, LoginUserWithUserNameCheck, LoginSendMail } from '@/apis/api_user'
import './auth.css'
// icon

// api

// utils
import { validateEmail } from '@/utils/check';
// store
import { useUserStore } from '@/stores/user';
import { useThemeStore } from '@/stores/theme';
import { useInfoStore } from '@/stores/info';
const UserStore = useUserStore()
const ThemeStore = useThemeStore()
const InfoStore = useInfoStore()
const Message = useMessage()
const Router = useRouter()
// ref
const current = ref<number>(1)

const isKeyMail = ref<boolean>(false)
const userKey = ref<string>("")
const userKeySuccess = ref<boolean>(true)

const userPass = ref<string>("")
const userPassSuccess = ref<boolean>(true)

const mailCode = ref<number>()
const mailCodeSuccess = ref<boolean>(true)

const dropDownOptions1st = ref([
    {
        label: '注册',
        key: 'register',
    },
    {
        label: '忘记密码',
        key: 'foget',
    },
])
const dropDownOptions2nd = ref([
    {
        label: '邮箱登录',
        key: 'mail',
    },
    {
        label: 'ID登录',
        key: 'id',
    },
    {
        label: '上一步',
        key: 'back',
    },
])
// hook
onMounted(() => {
    ThemeStore.hideHeader();
})
onUnmounted(() => {
    ThemeStore.showHeader();
})
// fn
const handleDropDownSelect = async (key: string) => {
    switch (key) {
        case 'register':
            goRegister()
            return
        case 'foget':
            return
        case 'mail':
            isKeyMail.value = true
            return
        case 'id':
            isKeyMail.value = false
            return
        case 'back':
            current.value = current.value - 1
            return
        default:
            return
    }
}

const goRegister = () => {
    Router.push({ path: "/register" })
}

const firstStep = async () => {
    let has = await UserStore.checkExist(userKey.value)
    if (!has) {
        userKeySuccess.value = false
    } else {
        let isMail = validateEmail(userKey.value)
        if (isMail) {
            isKeyMail.value = true
        } else {
            isKeyMail.value = false
        }
        userKeySuccess.value = true
        current.value = current.value + 1
    }
}

const secondStep = async () => {
    let can_mail = InfoStore.can_mail
    let isMail = isKeyMail.value
    let res
    if (can_mail) {
        if (isMail) {
            res = await LoginUserWithEmailCheck({ "Email": userKey.value, "Password": userPass.value })
        } else {
            res = await LoginUserWithUserNameCheck({ "UserName": userKey.value, "Password": userPass.value })
        }
        if (res.ok && res.data.Correct) {
            await LoginSendMail({ "Email": userKey.value })
            current.value = current.value + 1
        } else {
            userPassSuccess.value = false
        }
    } else {
        if (isMail) {
            res = await UserStore.loginWithEmail(userKey.value, userPass.value)
        } else {
            res = await UserStore.loginWithUName(userKey.value, userPass.value)
        }
        if (res.ok && res.data.Correct) {
            current.value = current.value + 2
            setTimeout(() => {
                Router.push({ path: "/" })
            }, 1000);
        } else {
            userPassSuccess.value = false
        }
    }
}

const sendAgain = async () => {
    await LoginSendMail({ "Email": userKey.value })
}

const thirdStep = async () => {
    if (mailCode.value as number < 1000 || mailCode.value as number > 9999) {
        Message.error("验证码错误")
    }
    let res = await UserStore.loginWithEmailCode(userKey.value, mailCode.value as number)
    console.log(res)
    if (res.ok == true) {
        current.value = current.value + 1
        setTimeout(() => {
            Router.push({ path: "/" })
        }, 1000);
    } else {
        mailCodeSuccess.value = false
    }
}

</script>
<template>
    <div class="page-cont">
        <div class="login-cont">
            <div class="login-left">
                <div class="login-left-line">
                    <img src="../../assets/logo.png" alt="painter" class="login-left-logo">
                    <div>
                        <div class="login-welcome">
                            欢迎来到Painter
                        </div>
                        <div class="login-left-line">
                            博客，但不止是博客。
                        </div>
                    </div>
                </div>
            </div>
            <div class="login-right">
                <Transition>
                    <div>
                        <div v-if="current == 1">
                            <div class="painter-login">
                                Painter 登录
                            </div>
                            <div class="login-account-text">
                                电子邮件或用户ID
                            </div>
                            <n-input v-model:value="userKey" placeholder="电子邮件或用户ID"
                                authentication="username"></n-input>
                            <div v-if="!userKeySuccess" style="color: var(--highlight-color);">
                                请检查账号信息
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
                                欢迎你 {{ userKey }}
                            </div>
                            <div class="login-account-text">
                                正在使用{{ isKeyMail ? '邮箱' : '用户ID' }}登录
                            </div>
                            <n-input v-model:value="userPass" placeholder="密码" type="password" authentication="password"
                                show-password-on="click"></n-input>
                            <div v-if="!userPassSuccess" style="color: var(--highlight-color);">
                                请检查密码
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
                                欢迎你 {{ userKey }}
                            </div>
                            <div class="login-account-text">
                                请输入邮箱中的验证码
                            </div>
                            <n-input-number v-model:value="mailCode" placeholder="验证码"
                                :show-button="false"></n-input-number>
                            <div v-if="!mailCodeSuccess" style="color: var(--highlight-color);">
                                验证码错误
                            </div>
                            <div class="login-btn-cont">
                                <n-button text @click="sendAgain">
                                    没有收到邮件
                                </n-button>
                                <n-button color="#2080f0" style="color: #fff;" type="info" @click="thirdStep">
                                    下一步
                                </n-button>
                            </div>
                        </div>
                        <div v-if="current == 4">
                            <div class="painter-login">
                                欢迎你 {{ userKey }}
                            </div>
                            <div class="login-account-text">
                                登录成功，即将跳转回首页
                            </div>
                        </div>
                    </div>
                </Transition>
            </div>
        </div>
    </div>
</template>
<style></style>