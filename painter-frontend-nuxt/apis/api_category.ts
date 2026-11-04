import type { AxiosResponse } from "axios";
import { AxiosGet, AxiosPost, type MyResponse } from "./axios";

export const CategoryList = async (data: any): Promise<AxiosResponse> => AxiosGet({ url: "/category/list", data });

export const CategoryCreate = async (data: any): Promise<MyResponse> => AxiosPost({ url: "/category/create", data });

export const CategoryUpdate = async (data: any): Promise<MyResponse> => AxiosPost({ url: "/category/update/", data });

export const CategoryUpdateName = async (data: any): Promise<AxiosResponse> => AxiosPost({ url: "/category/update/name", data });

export const CategoryUpdateDesc = async (data: any): Promise<AxiosResponse> => AxiosPost({ url: "/category/update/desc", data });

export const GetCategoryByID = async (data: any): Promise<AxiosResponse> => AxiosGet({ url: "/category/get", data });

export const GetCategoriesList = async (data: any): Promise<AxiosResponse> => AxiosGet({ url: "/category/get/fulllist", data });