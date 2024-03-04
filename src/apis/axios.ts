import axios, { type AxiosResponse, type AxiosRequestConfig } from 'axios';

interface RequestParams {
    url: string;
    data?: any;
    config?: AxiosRequestConfig;
}

interface ResponseData {
    code: number;
    message: string;
    data: any;
}

const instance = axios.create({
    baseURL: 'http://localhost:3003',
    timeout: 5000,
    withCredentials: true,
});

export const AxiosPost = async ({ url, data, config }: RequestParams): Promise<AxiosResponse<ResponseData>> => {
    try {
        const response = await instance.post(url, data, config);
        return response.data;
    } catch (error) {
        if (error instanceof Error) throw new Error(error.message);
        throw new Error("Invalid request");
    }
};

export const AxiosGet = async ({ url, data, config }: RequestParams): Promise<AxiosResponse<ResponseData>> => {
    try {
        const response = await instance.get(url, { params: data, ...config });
        return response.data;
    } catch (error) {
        if (error instanceof Error) throw new Error(error.message);
        throw new Error("Invalid request");
    }
};

