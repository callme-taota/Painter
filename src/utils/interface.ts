
export interface ArticleInfoItem {
    ArticleTable: ArticleItem,
    ArticleTagTable: ArticleTagItem[],
    LikesNumber: number,
    CollectionNumber: number,
    CommentNumber: number,
}

export interface ArticleItem {
    ArticleID: number,
    Title: string,
    Author: number,
    Summary: string,
    ReadCount: number,
    IsTop: boolean,
    Status: number,//gorm:"comment:0 草稿，1 发布，2 隐藏，3 限制，4 封禁'"
    CategoryID: number,
    CreatedAt: string,
    UpdatedAt: string,
}

export interface ArticleContentItem {
    ArticleID: number,
    Content: string,
}

export interface ArticleTagItem {
    TagID: number,
    TagName: string,
    Description: string,
}

export interface CategoryListItem {
    ArticleCount: number,
    CategoryName: string,
    Description: string,
    CategoryID: number,
}

export interface MiniUser {
    ID: number,
    Email: string,
    NickName: string,
    HeaderField: string,
    CreatedAt: string,
}

export interface FullArticleItem {
    ArticleTable: ArticleItem,
    ArticleTagTable: ArticleTagItem[],
    ArticleContentTable: ArticleContentItem,
    User: MiniUser,
    LikesNumber: number,
    CollectionNumber: number,
    CommentNumber: number,
    CategoryName: string,
    Liked: boolean,
    Collected: boolean,
}

export interface CommentItem {
    CommentID: number,
    Content: number,
    UserID: number,
    ArticleID: number,
    CreateTime: string,
    NickName: string
    HeaderField: string
    LikeCount: number
    Liked: boolean
}