import { AxiosPost, AxiosGet, type MyResponse } from './axios';

export const GetArticleByAuthor = async (data: any): Promise<MyResponse> => {
    const res = await AxiosGet({
        url: "/article/get/author",
        data: data
    });
    return res
}

export const GetArticleByTitle = async (data: any): Promise<MyResponse> => {
    const res = await AxiosPost({
        url: "/article/get/title",
        data: data
    });
    return res
}

export const GetArticleByCollection = async (data: any): Promise<MyResponse> => {
    const res = await AxiosGet({
        url: "/article/get/collection",
        data: data
    });
    return res
}

export const GetArticleByContent = async (data: any): Promise<MyResponse> => {
    const res = await AxiosPost({
        url: "/article/get/content",
        data: data
    });
    return res
}

export const GetArticleByCategory = async (data: any): Promise<MyResponse> => {
    const res = await AxiosGet({
        url: "/article/get/category",
        data: data
    });
    return res
}

export const GetArticleByTag = async (data: any): Promise<MyResponse> => {
    const res = await AxiosGet({
        url: "/article/get/tag",
        data: data
    });
    return res
}

export const GetArticle = async (data: any): Promise<MyResponse> => {
    const res = await AxiosGet({
        url: "/article/get",
        data: data
    });
    return res
}

export const CreateArticle = async (data: any): Promise<MyResponse> => {
    const res = await AxiosPost({
        url: "/article/create",
        data: data
    });
    return res
}

export const ArticleUpdateContent = async (data: any): Promise<MyResponse> => {
    const res = await AxiosPost({
        url: "/article/update/content",
        data: data
    });
    return res
}

export const ArticleUpdateSummary = async (data: any): Promise<MyResponse> => {
    const res = await AxiosPost({
        url: "/article/update/summary",
        data: data
    });
    return res
}

export const ArticleUpdateTitle = async (data: any): Promise<MyResponse> => {
    const res = await AxiosPost({
        url: "/article/update/title",
        data: data
    });
    return res
}

export const ArticleUpdateStatus = async (data: any): Promise<MyResponse> => {
    const res = await AxiosPost({
        url: "/article/update/status",
        data: data
    });
    return res
}

export const ArticleDelete = async (data: any): Promise<MyResponse> => {
    const res = await AxiosPost({
        url: "/article/delete",
        data: data
    });
    return res
}

export const ArticleLikeCreate = async (data: any): Promise<MyResponse> => {
    const res = await AxiosPost({
        url: "/article/like/create",
        data: data
    });
    return res
}

export const ArticleLikeDelete = async (data: any): Promise<MyResponse> => {
    const res = await AxiosPost({
        url: "/article/like/delete",
        data: data
    });
    return res
}

export const ArticleLike = async (data: any): Promise<MyResponse> => {
    const res = await AxiosPost({
        url: "/article/like",
        data: data
    });
    return res
}

export const ArticleLikeCheck = async (data: any): Promise<MyResponse> => {
    const res = await AxiosPost({
        url: "/article/like/check",
        data: data
    });
    return res
}

export const ArticleTagCreate = async (data: any): Promise<MyResponse> => {
    const res = await AxiosPost({
        url: "/article/tag/create",
        data: data
    });
    return res
}

export const ArticleTagUpdate = async (data: any): Promise<MyResponse> => {
    const res = await AxiosPost({
        url: "/article/tag/update",
        data: data
    });
    return res
}

export const ArticleTagDelete = async (data: any): Promise<MyResponse> => {
    const res = await AxiosPost({
        url: "/article/tag/delete",
        data: data
    });
    return res
}

export const GetSelfArticle = async (data: any): Promise<MyResponse> => {
    const res = await AxiosGet({
        url: "/article/get/self",
        data: data
    });
    return res
}

export const GetArticleList = async (data: any): Promise<MyResponse> => {
    const res = await AxiosGet({
        url: "/article/get/time",
        data: data
    })
    return res
}

export const UpdateArticleCategory = async (data: any): Promise<MyResponse> => {
    const res = await AxiosPost({
        url: "/article/update/category",
        data: data
    });
    return res
}