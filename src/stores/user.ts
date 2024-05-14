import { ref } from 'vue'
import { defineStore } from 'pinia'
import { LoginUserWithEmail, LoginUserWithPhone, LoginUserWithUserName, Logout, UserSelf, UserExist, LoginMailCodeCheck } from '@/apis/api_user'
import type { MyResponse } from '@/apis/axios'

export const useUserStore = defineStore('user', () => {
  const userName = ref("")
  const userNickName = ref("")
  const userHeaderField = ref("")
  const loginStatus = ref(getData("painter-blog-login-status"))


  function setLocalLoginStatus() {
    let status = loginStatus.value
    saveData("painter-blog-login-status", status)
  }
  async function getLocalLoginStatus() {
    let status = getData("painter-blog-login-status")
    if (status == true) {
      await loadSelfData()
    }
    loginStatus.value = status
    return status
  }
  async function loadSelfData(): Promise<MyResponse> {
    let res = await UserSelf()
    if (res.ok) {
      userName.value = res.data.UserName
      userNickName.value = res.data.NickName
      userHeaderField.value = res.data.HeaderField
    } else {
      logout()
    }
    return res
  }
  async function login(key: string, password: string, notMail: boolean | undefined): Promise<MyResponse> {
    let res
    if (notMail) {
      res = await loginWithUName(key, password)
    } else {
      res = await loginWithEmail(key, password)
    }
    return res
  }
  async function loginWithEmail(email: string, password: string): Promise<MyResponse> {
    let res = await LoginUserWithEmail({ "Email": email, "Password": password })
    if (res.data.Session != "") {
      saveData("painter-blog-session", res.data.Session)
      loginStatus.value = true
      setLocalLoginStatus()
      await loadSelfData()
    }
    return res
  }
  async function loginWithEmailCode(email: string, code: number): Promise<MyResponse> {
    // let codeNum: number = parseInt(code)
    let res = await LoginMailCodeCheck({ "Email": email, "Code": code })
    if (res.data.Session != "") {
      saveData("painter-blog-session", res.data.Session)
      loginStatus.value = true
      setLocalLoginStatus()
      await loadSelfData()
    }
    return res
  }
  async function loginWithUName(userName: string, password: string): Promise<MyResponse> {
    let res = await LoginUserWithUserName({ "UserName": userName, "Password": password })
    if (res.data.Session != "") {
      saveData("painter-blog-session", res.data.Session)
      loginStatus.value = true
      setLocalLoginStatus()
      await loadSelfData()
    }
    return res
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
    await Logout({ "Session": session })
    setLocalLoginStatus()
    saveData("painter-blog-session", "")
  }
  async function checkExist(key: string): Promise<boolean> {
    let res = await UserExist({ "Key": key })
    return res.data.Has
  }

  return { userName, userNickName, userHeaderField, loginStatus, loginWithEmail, loginWithPhone, loginWithUName, logout, clear, getLocalLoginStatus, loadSelfData, login, checkExist, loginWithEmailCode }
})

function saveData(key: string, value: any) {
  localStorage.setItem(key, JSON.stringify(value))
}

function getData(key: string) {
  const data = localStorage.getItem(key)
  return data ? JSON.parse(data) : null
}