import type { AxiosResponse } from "axios";
import { AxiosPost } from "./axios";

export const CreateCollection = async (data: any): Promise<AxiosResponse> => {
    const res = await AxiosPost({
        url:"/collection/create",
        data: data
    });
    return res
}

export const DeleteCollection = async (data: any): Promise<AxiosResponse> => {
    const res = await AxiosPost({
        url:"/collection/delete",
        data: data
    });
    return res
}

export const CollectionList = async (data: any): Promise<AxiosResponse> => {
    const res = await AxiosPost({
        url:"/collection/list",
        data: data
    });
    return res
}

export const Collection = async (data: any): Promise<AxiosResponse> => {
    const res = await AxiosPost({
        url:"/article/collection",
        data: data
    });
    return res
}

export const CollectionCheck = async (data: any): Promise<AxiosResponse> => {
    const res = await AxiosPost({
        url:"/collection/check",
        data: data
    });
    return res
}