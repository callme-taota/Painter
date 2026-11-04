import { AxiosGet, type MyResponse } from "./axios";

export const GetStartTime = async (): Promise<MyResponse> => AxiosGet({ url: "/common/starttime" });

export const GetDayPV = async (): Promise<MyResponse> => AxiosGet({ url: "/common/vis/preday" });

export const GetMonthPV = async (): Promise<MyResponse> => AxiosGet({ url: "/common/vis/currmonth" });

export const GetInfo = async (): Promise<MyResponse> => AxiosGet({ url: "/common/info" });

export const IsAdmin = async (): Promise<MyResponse> => AxiosGet({ url: "/common/isadmin" });

export const GetEntryInfo = async (): Promise<MyResponse> => AxiosGet({ url: "/common/entry" });