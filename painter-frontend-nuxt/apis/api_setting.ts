import { AxiosPost, AxiosGet, type MyResponse } from './axios';

export const GetSettings = async (): Promise<MyResponse> => {
    const res = await AxiosGet({
        url: "/setting/",
    });
    return res
}

export const SetSettings = async (data: any): Promise<MyResponse> => {
    const res = await AxiosPost({
        url: "/setting/",
        data: data
    });
    return res
}

export const GetUserList = async (data: any): Promise<MyResponse> => {
    const res = await AxiosGet({
        url: "/setting/userlist",
        data: data
    });
    return res
}

export const SetUserPermission = async (data: any): Promise<MyResponse> => {
    const res = await AxiosPost({
        url: "/setting/user/permission",
        data: data
    });
    return res
}