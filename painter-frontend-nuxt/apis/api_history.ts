import type { AxiosResponse } from "axios";
import { AxiosGet, AxiosPost } from "./axios";

export const CreateHistory = async (data: any): Promise<AxiosResponse> => AxiosPost({ url: "/history/create", data });

export const GetHistory = async (data: any): Promise<AxiosResponse> => AxiosGet({ url: "/history/list", data });