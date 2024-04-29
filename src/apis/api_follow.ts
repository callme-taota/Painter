import type { AxiosResponse } from "axios";
import { AxiosGet, AxiosPost } from "./axios";

export const CreateFollow = async (data: any): Promise<AxiosResponse> => {
    const res = await AxiosPost({
        url:"/follow/follow",
        data: data
    });
    return res
}

export const DeleteFollow = async (data: any): Promise<AxiosResponse> => {
    const res = await AxiosPost({
        url:"/follow/unfollow",
        data: data
    });
    return res
}

export const GetFollowers = async (data: any): Promise<AxiosResponse> => {
    const res = await AxiosGet({
        url:"/follow/followers",
        data: data
    });
    return res
}

export const GetFollowings = async (data: any): Promise<AxiosResponse> => {
    const res = await AxiosGet({
        url:"/follow/followings",
        data: data
    });
    return res
}