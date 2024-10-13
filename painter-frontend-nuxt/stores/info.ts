import { defineStore } from "pinia";
import { ref } from "vue";
import { GetInfo } from "@/apis/api_common";

export const useInfoStore = defineStore("infoStore", () => {
    const github_href = ref<string>("")
    const can_register = ref<boolean>(false)
    const icp_code = ref<string>("")
    const site_name = ref<string>("")
    const can_mail = ref<boolean>(false)

    const getInfo = async () => {
        let res = await GetInfo()
        if (res.ok) {
            github_href.value = res.data.Github
            can_register.value = res.data.CanRegister
            icp_code.value = res.data.ICPCode
            site_name.value = res.data.SiteName
            can_mail.value = res.data.CanSendMail
        }
    }

    return {
        github_href,
        can_register,
        icp_code,
        site_name,
        can_mail,
        getInfo
    }
});