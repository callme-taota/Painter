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