
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

export interface ArticleTagItem {
    TagID: number,
    TagName: string,
    Description: string,
}