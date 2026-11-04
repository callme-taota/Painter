import axios, { type AxiosResponse, type AxiosResponseHeaders, type InternalAxiosRequestConfig } from 'axios';
import { AxiosGet, AxiosPost, type MyResponse } from './axios';

export const CreateUser = async (data: any): Promise<MyResponse> => AxiosPost({ url: "/user/create", data });

export const LoginUserWithEmail = async (data: any): Promise<MyResponse> => AxiosPost({ url: "/user/login/email", data });

export const LoginUserWithEmailCheck = async (data: any): Promise<MyResponse> => AxiosPost({ url: "/user/login/email/pass", data });

export const LoginUserWithPhone = async (data: any): Promise<AxiosResponse> => AxiosPost({ url: "/user/login/phone", data });

export const LoginUserWithUserName = async (data: any): Promise<MyResponse> => AxiosPost({ url: "/user/login/uname", data });

export const LoginUserWithUserNameCheck = async (data: any): Promise<MyResponse> => AxiosPost({ url: "/user/login/uname/pass", data });

export const LoginUserWithCheck = async (data: any): Promise<AxiosResponse> => AxiosPost({ url: "/user/login/check", data });

export const UserExist = async (data: any): Promise<MyResponse> => AxiosPost({ url: "/user/login/exist", data });

export const Logout = async (data: any): Promise<AxiosResponse> => AxiosPost({ url: "/user/logout", data });

export const UserNameUpdate = async (data: any): Promise<AxiosResponse> => AxiosPost({ url: "/user/update/name", data });

export const UserEmailUpdate = async (data: any): Promise<AxiosResponse> => AxiosPost({ url: "/user/update/email", data });

export const UserNickNameUpdate = async (data: any): Promise<AxiosResponse> => AxiosPost({ url: "/user/update/nickname", data });

export const UserPhoneUpdate = async (data: any): Promise<AxiosResponse> => AxiosPost({ url: "/user/update/phone", data });

export const UserHeaderFieldUpdate = async (data: any): Promise<AxiosResponse> => AxiosPost({ url: "/user/update/headerfield", data });

export const UserUpdate = async (data: any): Promise<AxiosResponse> => AxiosPost({ url: "/user/update", data });

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

export const UserSelfFull = async (): Promise<AxiosResponse> => AxiosGet({ url: "/user/self/full" });

export const UserInfo = async (data: any): Promise<AxiosResponse> => AxiosGet({ url: "/user/info", data });

export const LoginSendMail = async (data: any): Promise<MyResponse> => AxiosPost({ url: "/user/login/send", data });

export const LoginMailCodeCheck = async (data: any): Promise<MyResponse> => AxiosPost({ url: "/user/login/mailcheck", data });