
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

export interface UserItem {
    ID: number,
    Email: string,
    NickName: string,
    HeaderField: string,
    CreatedAt: string,
    LastLogin: string,
    UserName: string,
    PhoneNum: number
}

export interface MiniUser {
    ID: number,
    Email: string,
    NickName: string,
    HeaderField: string,
    CreatedAt: string,
    LastLogin: string,
}

export interface FollowUser {
    ID: number,
    Email: string,
    NickName: string,
    HeaderField: string,
    CreatedAt: string,
    LastLogin: string,
    Following: boolean,
}

export interface FullUserItem {
    ArticleNumber: number,
    ArticleList: ArticleInfoItem[],
    CollectionNumber: number,
    FollowingNumber: number,
    FollowerNumber: number,
    UserInfo: MiniUser
    Following: boolean
    TotalCount: number
    Self : boolean
}

export interface SelfItem {
    ArticleNumber: number,
    ArticleList: ArticleItem[],
    CollectionNumber: number,
    FollowingNumber: number,
    FollowerNumber: number,
    UserInfo: UserItem
    Following: boolean
    TotalCount: number
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
    IsSelf: boolean
}

export interface CategoryItem {
    CategoryID: number,
    CategoryName: string,
    Description: string
}