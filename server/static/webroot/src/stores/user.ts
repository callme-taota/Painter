import { ref } from 'vue'
import { defineStore } from 'pinia'
import { LoginUserWithEmail, LoginUserWithPhone, LoginUserWithUserName, Logout, UserSelf } from '@/apis/api_user'

export const useUserStore = defineStore('user', () => {
  const userName = ref("")
  const userNickName = ref("")
  const userHeaderField = ref("")
  const loginStatus = ref(getData("painter-blog-login-status"))


  function setLocalLoginStatus() {
    let status = loginStatus.value
    saveData("painter-blog-login-status", status)
  }
  function getLocalLoginStatus() {
    let status = getData("painter-blog-login-status")
    loginStatus.value = status
  }
  async function loadSelfData() {
    let res = await UserSelf()
    userName.value = res.data.UserName
    userNickName.value = res.data.NickName
    userHeaderField.value = res.data.Headerfield
  }
  async function loginWithEmail(email: string, password: string) {
    let res = await LoginUserWithEmail({ "Email": email, "Password": password })
    if (res.data.Session != "") {
      saveData("painter-blog-session", res.data.Session)
      loginStatus.value = true
      setLocalLoginStatus()
      await loadSelfData()
    }
  }
  async function loginWithUName(userName: string, password: string) {
    let res = await LoginUserWithUserName({ "UserName": userName, "Password": password })
    if (res.data.Session != "") {
      saveData("painter-blog-session", res.data.Session)
      loginStatus.value = true
      setLocalLoginStatus()
      await loadSelfData()
    }
  }
  async function loginWithPhone(phone: string, password: string) {
    let res = await LoginUserWithPhone({ "Phone": phone, "Password": password })
    if (res.data.Session != "") {
      saveData("painter-blog-session", res.data.Session)
      loginStatus.value = true
      setLocalLoginStatus()
      await loadSelfData()
    }
  }
  function clear() {
    userName.value = ""
    userNickName.value = ""
    userHeaderField.value = ""
    loginStatus.value = false
    setLocalLoginStatus()
  }

  async function logout() {
    userName.value = ""
    userNickName.value = ""
    userHeaderField.value = ""
    loginStatus.value = false
    let session = getData("painter-blog-session")
    let res = await Logout({ "Session": session })
    setLocalLoginStatus()
    saveData("painter-blog-session", "")
  }

  return { userName, userNickName, userHeaderField, loginStatus, loginWithEmail, loginWithPhone, loginWithUName, logout, clear }
})

function saveData(key: string, value: any) {
  localStorage.setItem(key, JSON.stringify(value))
}

function getData(key: string) {
  const data = localStorage.getItem(key)
  return data ? JSON.parse(data) : null
}