import { AxiosGet, AxiosPost, type MyResponse } from "./axios";

export const GetStartTime = async (): Promise<MyResponse> => {
    const res = await AxiosGet({
        url: "/common/starttime",
    });
    return res
}

export const GetDayPV = async (): Promise<MyResponse> => {
    const res = await AxiosGet({
        url: "/common/vis/preday",
    });
    return res
}

export const GetMonthPV = async (): Promise<MyResponse> => {
    const res = await AxiosGet({
        url: "/common/vis/currmonth",
    });
    return res
}