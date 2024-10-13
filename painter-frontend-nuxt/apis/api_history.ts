import type { AxiosResponse } from "axios";
import { AxiosGet, AxiosPost } from "./axios";

export const CreateHistory = async (data: any): Promise<AxiosResponse> => {
    const res = await AxiosPost({
        url:"/history/create",
        data: data
    });
    return res
}

export const GetHistory = async (data: any): Promise<AxiosResponse> => {
const res = await AxiosGet({
        url:"/history/list",
        data: data
    });
    return res
}