import type { AxiosResponse } from "axios";
import { AxiosGet, AxiosPost } from "./axios";

export const CategoryList = async (data: any): Promise<AxiosResponse> => {
    const res = await AxiosGet({
        url:"/category/list",
        data: data
    });
    return res
}

export const CategoryCreate = async (data: any): Promise<AxiosResponse> => {
    const res = await AxiosPost({
        url:"/category/create",
        data: data
    });
    return res
}

export const CategoryUpdateName = async (data: any): Promise<AxiosResponse> => {
    const res = await AxiosPost({
        url:"/category/update/name",
        data: data
    });
    return res
}

export const CategoryUpdateDesc = async (data: any): Promise<AxiosResponse> => {
    const res = await AxiosPost({
        url:"/category/update/desc",
        data: data
    });
    return res
}