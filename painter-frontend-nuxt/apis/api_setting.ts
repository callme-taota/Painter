import { AxiosPost, AxiosGet, type MyResponse } from './axios';

export const GetSettings = async (): Promise<MyResponse> => AxiosGet({ url: "/setting/" });

export const SetSettings = async (data: any): Promise<MyResponse> => AxiosPost({ url: "/setting/", data });

export const GetUserList = async (data: any): Promise<MyResponse> => AxiosGet({ url: "/setting/userlist", data });

export const SetUserPermission = async (data: any): Promise<MyResponse> => AxiosPost({ url: "/setting/user/permission", data });