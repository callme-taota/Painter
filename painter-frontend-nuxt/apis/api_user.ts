import axios, { type AxiosResponse, type AxiosResponseHeaders, type InternalAxiosRequestConfig } from 'axios';
import { AxiosGet, AxiosPost, type MyResponse } from './axios';

export const CreateUser = async (data: any): Promise<MyResponse> => {
    const res = await AxiosPost({
        url:"/user/create",
        data: data
    });
    return res
}

export const LoginUserWithEmail = async (data: any): Promise<MyResponse> => {
    const res = await AxiosPost({
        url:"/user/login/email",
        data: data
    });
    return res
}

export const LoginUserWithEmailCheck = async (data: any): Promise<MyResponse> => {
    const res = await AxiosPost({
        url:"/user/login/email/pass",
        data: data
    });
    return res
}

export const LoginUserWithPhone = async (data: any): Promise<AxiosResponse> => {
    const res = await AxiosPost({
        url:"/user/login/phone",
        data: data
    });
    return res
}

export const LoginUserWithUserName = async (data: any): Promise<MyResponse> => {
    const res = await AxiosPost({
        url:"/user/login/uname",
        data: data
    });
    return res
}

export const LoginUserWithUserNameCheck = async (data: any): Promise<MyResponse> => {
    const res = await AxiosPost({
        url:"/user/login/uname/pass",
        data: data
    });
    return res
}

export const LoginUserWithCheck = async (data: any): Promise<AxiosResponse> => {
    const res = await AxiosPost({
        url:"/user/login/check",
        data: data
    });
    return res
}

export const UserExist = async (data: any): Promise<MyResponse> => {
    const res = await AxiosPost({
        url:"/user/login/exist",
        data: data
    });
    return res
}

export const Logout = async (data: any): Promise<AxiosResponse> => {
    const res = await AxiosPost({
        url:"/user/logout",
        data: data
    });
    return res
}

export const UserNameUpdate = async (data: any): Promise<AxiosResponse> => {
    const res = await AxiosPost({
        url:"/user/update/name",
        data: data
    });
    return res
}

export const UserEmailUpdate = async (data: any): Promise<AxiosResponse> => {
    const res = await AxiosPost({
        url:"/user/update/email",
        data: data
    });
    return res
}

export const UserNickNameUpdate = async (data: any): Promise<AxiosResponse> => {
    const res = await AxiosPost({
        url:"/user/update/nickname",
        data: data
    });
    return res
}

export const UserPhoneUpdate = async (data: any): Promise<AxiosResponse> => {
    const res = await AxiosPost({
        url:"/user/update/phone",
        data: data
    });
    return res
}

export const UserHeaderFieldUpdate = async (data: any): Promise<AxiosResponse> => {
    const res = await AxiosPost({
        url:"/user/update/headerfield",
        data: data
    });
    return res
}

export const UserUpdate = async (data: any): Promise<AxiosResponse> => {
    const res = await AxiosPost({
        url:"/user/update",
        data: data
    });
    return res
}

export const UserSelf = async (): Promise<MyResponse> => {
    try {
        const res = await AxiosGet({
            url: "/user/self",
        });
        return res;
    } catch (error) {
        if (axios.isAxiosError(error) && error.response) {
            return {
                ok: false,
                status: error.response.status,
                statusText: error.response.statusText,
                headers: error.response.headers,
                config: error.response.config,
                msg: error.response.data?.message || '请求失败',
                data: error.response.data
            };
        }
        return {
            ok: false,
            status: 0,
            statusText: '',
            headers: {} as AxiosResponseHeaders,
            config: {} as InternalAxiosRequestConfig,
            msg: '未知错误',
            data: null
        };
    }
}

export const UserSelfFull = async (): Promise<AxiosResponse> => {
    const res = await AxiosGet({
        url:"/user/self/full",
    });
    return res
}

export const UserInfo = async (data: any): Promise<AxiosResponse> => {
    const res = await AxiosGet({
        url:"/user/info",
        data: data
    });
    return res
}

export const LoginSendMail = async (data: any): Promise<MyResponse> => {
    const res = await AxiosPost({
        url:"/user/login/send",
        data: data
    });
    return res
}

export const LoginMailCodeCheck = async (data: any): Promise<MyResponse> => {
    const res = await AxiosPost({
        url:"/user/login/mailcheck",
        data: data
    });
    return res
}