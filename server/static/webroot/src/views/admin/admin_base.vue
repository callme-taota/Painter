<script setup lang="ts">
//base
import { ref, onMounted } from 'vue'
import { NInput, NButton, NSwitch, NInputNumber, useMessage } from 'naive-ui';
//icon

//api
import { GetSettings, SetSettings } from '@/apis/api_setting';
//store
const Message = useMessage()
//ref
interface baseSetting {
    CanRegister: boolean
    Github: string
    ICPCode: string
    SiteName: string
    Mail: mail
}
interface mail {
    Active: boolean
    From: string
    Password: string
    SmtpHost: string
    SmtpPort: number
}
const config = ref<baseSetting>({
    CanRegister: false,
    Github: '',
    ICPCode: '',
    SiteName: '',
    Mail: {
        Active: false,
        From: '',
        Password: '',
        SmtpHost: '',
        SmtpPort: 0
    }
})
//hook
onMounted(async () => {
    let res = await GetSettings()
    if (res.ok) {
        config.value = res.data
    }
})
//fn
const submit = async () => {
    let res = await SetSettings({
        "SiteName": config.value.SiteName,
        "ICPCode": config.value.ICPCode,
        "Github": config.value.Github,
        "CanRegister": config.value.CanRegister,
        "MailActive": config.value.Mail.Active,
        "MailFrom": config.value.Mail.From,
        "MailPassword": config.value.Mail.Password,
        "MailSMTPHost": config.value.Mail.SmtpHost,
        "MailSMTPPort": config.value.Mail.SmtpPort
    })
    if( res.ok ) {
        Message.info("更新成功")
    } else {
        Message.error("提交时发生错误")
    }
} 
</script>
<template>
    <div>
        <h3>
            常规选项
        </h3>
        <div class="input-item">
            <div class="input-left">
                站点名称
            </div>
            <div class="input-right">
                <n-input v-model:value="config.SiteName" placeholder="站点名称"></n-input>
            </div>
        </div>
        <div class="input-item">
            <div class="input-left">
                ICP码
            </div>
            <div class="input-right">
                <n-input v-model:value="config.ICPCode" placeholder="ICP码"></n-input>
            </div>
        </div>
        <div class="input-item">
            <div class="input-left">
                Github链接
            </div>
            <div class="input-right">
                <n-input v-model:value="config.Github" placeholder="Github链接"></n-input>
            </div>
        </div>
        <div class="input-item">
            <div class="input-left">
                是否开启注册
            </div>
            <div class="input-right">
                <n-switch v-model:value="config.CanRegister" />
            </div>
        </div>
        <h3>
            邮件设置
        </h3>
        <div class="input-item">
            <div class="input-left">
                是否开启邮件服务
            </div>
            <div class="input-right">
                <n-switch v-model:value="config.Mail.Active" />
            </div>
        </div>
        <div class="input-item">
            <div class="input-left">
                发件人
            </div>
            <div class="input-right">
                <n-input v-model:value="config.Mail.From" :disabled="!config.Mail.Active" placeholder="发件人"></n-input>
            </div>
        </div>
        <div class="input-item">
            <div class="input-left">
                密码
            </div>
            <div class="input-right">
                <n-input v-model:value="config.Mail.Password" :disabled="!config.Mail.Active" placeholder="密码"></n-input>
            </div>
        </div>
        <div class="input-item">
            <div class="input-left">
                SMTP服务地址
            </div>
            <div class="input-right">
                <n-input v-model:value="config.Mail.SmtpHost" :disabled="!config.Mail.Active" placeholder="SMTP服务地址"></n-input>
            </div>
        </div>
        <div class="input-item">
            <div class="input-left">
                SMTP服务端口
            </div>
            <div class="input-right">
                <n-input-number v-model:value="config.Mail.SmtpPort" :disabled="!config.Mail.Active" :show-button="false"></n-input-number>
            </div>
        </div>
        <br>
        <n-button @click="submit">
            保存修改
        </n-button>
    </div>
</template>
<style>
.input-item {
    display: flex;
    margin: 10px 0;
    align-items: center;
}

.input-left {
    width: 140px;
}

.input-right {
    width: 300px;
}
</style>