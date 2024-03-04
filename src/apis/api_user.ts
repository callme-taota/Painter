import { type AxiosResponse } from 'axios';
import { AxiosPost } from './axios';

interface MyResponse<T = any> extends AxiosResponse {
    ok: boolean;
    msg: string;
}

export const CreateUser = async (data: any): Promise<AxiosResponse> => {
    const res = await AxiosPost({
        url:"/user/create",
        data: data
    });
    return res
}

export const LoginUserWithEmail = async (data: any): Promise<AxiosResponse> => {
    const res = await AxiosPost({
        url:"/user/login/email",
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

export const LoginUserWithUserName = async (data: any): Promise<AxiosResponse> => {
    const res = await AxiosPost({
        url:"/user/login/uname",
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

export const UserSelf = async (): Promise<AxiosResponse> => {
    const res = await AxiosPost({
        url:"/user/self",
    });
    return res
}

export const UserFull = async (): Promise<AxiosResponse> => {
    const res = await AxiosPost({
        url:"/user/full",
    });
    return res
}
