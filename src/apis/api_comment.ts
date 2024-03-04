import type { AxiosResponse } from "axios";
import { AxiosPost } from "./axios";

export const CreateComment = async (data: any): Promise<AxiosResponse> => {
    const res = await AxiosPost({
        url:"/comment/create",
        data: data
    });
    return res
}

export const DeleteComment = async (data: any): Promise<AxiosResponse> => {
    const res = await AxiosPost({
        url:"/comment/delete",
        data: data
    });
    return res
}

export const LikeComment = async (data: any): Promise<AxiosResponse> => {
    const res = await AxiosPost({
        url:"/comment/like",
        data: data
    });
    return res
}

export const DisLikeComment = async (data: any): Promise<AxiosResponse> => {
const res = await AxiosPost({
        url:"/comment/dislike",
        data: data
    });
    return res
}