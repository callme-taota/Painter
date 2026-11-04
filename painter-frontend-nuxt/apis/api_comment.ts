import type { AxiosResponse } from "axios";
import { AxiosGet, AxiosPost } from "./axios";

export const CreateComment = async (data: any): Promise<AxiosResponse> => AxiosPost({ url: "/comment/create", data });

export const DeleteComment = async (data: any): Promise<AxiosResponse> => AxiosPost({ url: "/comment/delete", data });

export const LikeComment = async (data: any): Promise<AxiosResponse> => AxiosPost({ url: "/comment/like", data });

export const DisLikeComment = async (data: any): Promise<AxiosResponse> => AxiosPost({ url: "/comment/dislike", data });

export const GetComments = async (data: any): Promise<AxiosResponse> => AxiosGet({ url: "/comment/list", data });

export const GetCommentsL = async (data: any): Promise<AxiosResponse> => AxiosGet({ url: "/comment/list/l", data });