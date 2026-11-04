import type { AxiosResponse } from "axios";
import { AxiosGet, AxiosPost, type MyResponse } from "./axios";

export const TagSuggest = async (data: any): Promise<AxiosResponse> => AxiosGet({ url: "/tag/suggest", data });

export const TagList = async (data: any): Promise<AxiosResponse> => AxiosGet({ url: "/tag/list", data });

export const TagListFull = async (data: any): Promise<AxiosResponse> => AxiosGet({ url: "/tag/list/full", data });

export const CreateTag = async (data: any): Promise<MyResponse> => AxiosPost({ url: "/tag/create", data });

export const UpdateTagName = async (data: any): Promise<AxiosResponse> => AxiosPost({ url: "/tag/update/name", data });

export const UpdateTagDesc = async (data: any): Promise<AxiosResponse> => AxiosPost({ url: "/tag/update/desc", data });

export const UpdateTag = async (data: any): Promise<MyResponse> => AxiosPost({ url: "/tag/update/", data });