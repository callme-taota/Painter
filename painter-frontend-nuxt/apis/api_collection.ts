import type { AxiosResponse } from "axios";
import { AxiosPost } from "./axios";

export const CreateCollection = async (data: any): Promise<AxiosResponse> => AxiosPost({ url: "/collection/create", data });

export const DeleteCollection = async (data: any): Promise<AxiosResponse> => AxiosPost({ url: "/collection/delete", data });

export const CollectionList = async (data: any): Promise<AxiosResponse> => AxiosPost({ url: "/collection/list", data });

export const Collection = async (data: any): Promise<AxiosResponse> => AxiosPost({ url: "/article/collection", data });

export const CollectionCheck = async (data: any): Promise<AxiosResponse> => AxiosPost({ url: "/collection/check", data });