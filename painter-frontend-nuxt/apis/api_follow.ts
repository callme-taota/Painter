import type { AxiosResponse } from "axios";
import { AxiosGet, AxiosPost } from "./axios";

export const CreateFollow = async (data: any): Promise<AxiosResponse> => AxiosPost({ url: "/follow/follow", data });

export const DeleteFollow = async (data: any): Promise<AxiosResponse> => AxiosPost({ url: "/follow/unfollow", data });

export const GetFollowers = async (data: any): Promise<AxiosResponse> => AxiosGet({ url: "/follow/followers", data });

export const GetFollowings = async (data: any): Promise<AxiosResponse> => AxiosGet({ url: "/follow/followings", data });