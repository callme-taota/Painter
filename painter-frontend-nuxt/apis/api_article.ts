import { AxiosPost, AxiosGet, type MyResponse } from './axios';

export const GetArticleByAuthor = async (data: any): Promise<MyResponse> => AxiosGet({ url: "/article/get/author", data });

export const GetArticleByTitle = async (data: any): Promise<MyResponse> => AxiosPost({ url: "/article/get/title", data });

export const GetArticleByCollection = async (data: any): Promise<MyResponse> => AxiosGet({ url: "/article/get/collection", data });

export const GetArticleByContent = async (data: any): Promise<MyResponse> => AxiosPost({ url: "/article/get/content", data });

export const GetArticleByCategory = async (data: any): Promise<MyResponse> => AxiosGet({ url: "/article/get/category", data });

export const GetArticleByTag = async (data: any): Promise<MyResponse> => AxiosGet({ url: "/article/get/tag", data });

export const GetArticle = async (data: any): Promise<MyResponse> => AxiosGet({ url: "/article/get", data });

export const CreateArticle = async (data: any): Promise<MyResponse> => AxiosPost({ url: "/article/create", data });

export const ArticleUpdateContent = async (data: any): Promise<MyResponse> => AxiosPost({ url: "/article/update/content", data });

export const ArticleUpdateSummary = async (data: any): Promise<MyResponse> => AxiosPost({ url: "/article/update/summary", data });

export const ArticleUpdateTitle = async (data: any): Promise<MyResponse> => AxiosPost({ url: "/article/update/title", data });

export const ArticleUpdateStatusDart = async (data: any): Promise<MyResponse> => AxiosPost({ url: "/article/update/status/dart", data });

export const ArticleUpdateStatusPublic = async (data: any): Promise<MyResponse> => AxiosPost({ url: "/article/update/status/public", data });

export const ArticleDelete = async (data: any): Promise<MyResponse> => AxiosPost({ url: "/article/delete", data });

export const ArticleUpdate = async (data: any): Promise<MyResponse> => AxiosPost({ url: "/article/update", data });

export const ArticleLikeCreate = async (data: any): Promise<MyResponse> => AxiosPost({ url: "/article/like/create", data });

export const ArticleLikeDelete = async (data: any): Promise<MyResponse> => AxiosPost({ url: "/article/like/delete", data });

export const ArticleLike = async (data: any): Promise<MyResponse> => AxiosPost({ url: "/article/like", data });

export const ArticleLikeCheck = async (data: any): Promise<MyResponse> => AxiosPost({ url: "/article/like/check", data });

export const ArticleTagCreate = async (data: any): Promise<MyResponse> => AxiosPost({ url: "/article/tag/create", data });

export const ArticleTagUpdate = async (data: any): Promise<MyResponse> => AxiosPost({ url: "/article/tag/update", data });

export const ArticleTagDelete = async (data: any): Promise<MyResponse> => AxiosPost({ url: "/article/tag/delete", data });

export const GetSelfArticle = async (data: any): Promise<MyResponse> => AxiosGet({ url: "/article/get/self", data });

export const GetArticleList = async (data: any): Promise<MyResponse> => AxiosGet({ url: "/article/get/time", data });

export const UpdateArticleCategory = async (data: any): Promise<MyResponse> => AxiosPost({ url: "/article/update/category", data });

export const GetArticleCount = async (): Promise<MyResponse> => AxiosGet({ url: "/article/get/count" });