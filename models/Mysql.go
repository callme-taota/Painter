package models

import (
	"gorm.io/gorm"
	"time"
)

type UserTable struct {
	ID          int    `gorm:"primaryKey;autoIncrement"`
	UserName    string `gorm:"type:varchar(255);unique"`
	Email       string `gorm:"type:varchar(255);unique"`
	AdminFlag   int    `gorm:"type:tinyint"`
	LastLogin   time.Time
	NickName    string `gorm:"type:varchar(255)"`
	PhoneNum    int    `gorm:"type:int;unique"`
	HeaderField string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   gorm.DeletedAt
}

func (UserTable) TableName() string {
	return "user"
}

type UserPassTable struct {
	ID       int
	Password string `gorm:"type:varchar(255)"`
}

func (UserPassTable) TableName() string {
	return "userpass"
}

type TagTable struct {
	TagID       int    `gorm:"primaryKey;not null,"`
	TagName     string `gorm:"type:varchar(255);unique"`
	Description string
}

func (TagTable) TableName() string {
	return "tag"
}

type HistoryTable struct {
	HistoryID   int `gorm:"primaryKey;autoIncrement"`
	UserID      int `gorm:"index"`
	ArticleID   int
	HistoryTime time.Time
}

func (HistoryTable) TableName() string {
	return "history"
}

type FollowTable struct {
	FollowID    int       `gorm:"primaryKey;autoIncrement"`
	FollowerID  int       `gorm:"uniqueIndex:fl;not null"`
	FollowingID int       `gorm:"uniqueIndex:fl;not null"`
	FollowTime  time.Time `gorm:"autoCreateTime"`
}

func (FollowTable) TableName() string {
	return "follow"
}

type CommentTable struct {
	CommentID  int `gorm:"primaryKey;autoIncrement"`
	Content    string
	UserID     int
	ArticleID  int
	CreateTime int `gorm:"autoUpdateTime"`
}

func (CommentTable) TableName() string {
	return "comment"
}

type CommentLikeTable struct {
	CommentID int `gorm:"uniqueIndex:com_like"`
	UserID    int `gorm:"uniqueIndex:com_like"`
}

func (CommentLikeTable) TableName() string {
	return "comment_like"
}

type CollectionTable struct {
	CollectionID   int `gorm:"primaryKey;autoIncrement"`
	UserID         int `gorm:"uniqueIndex:collection_ua"`
	ArticleID      int `gorm:"uniqueIndex:collection_ua"`
	CollectionTime int `gorm:"autoUpdateTime"`
}

func (CollectionTable) TableName() string {
	return "collection"
}

type CategoryTable struct {
	CategoryID   int    `gorm:"primaryKey;autoIncrement"`
	CategoryName string `gorm:"type:varchar(255)"`
	Description  string
}

func (CategoryTable) TableName() string {
	return "category"
}

type ArticleTable struct {
	ArticleID  int `gorm:"primaryKey;autoIncrement"`
	Title      string
	Author     int
	Summary    string
	ReadCount  int
	IsTop      bool
	Status     int `gorm:"comment:0 草稿，1 发布，2 隐藏，3 限制，4 封禁'"`
	CategoryID int
	CreatedAt  time.Time
	UpdatedAt  time.Time
	DeletedAt  gorm.DeletedAt `gorm:"index"`
}

func (ArticleTable) TableName() string {
	return "article"
}

type ArticleContentTable struct {
	ArticleID int    `gorm:"uniqueIndex:art_cont"`
	Content   string `gorm:"type:text;uniqueIndex:art_cont,length:12"`
}

func (ArticleContentTable) TableName() string {
	return "article_content"
}

type ArticleLikeTable struct {
	ArticleID int `gorm:"uniqueIndex:art_like"`
	UserID    int `gorm:"uniqueIndex:art_like"`
	CreatedAt time.Time
}

func (ArticleLikeTable) TableName() string {
	return "article_like"
}

type ArticleTagTable struct {
	ArticleID int `gorm:"uniqueIndex:art_tag"`
	TagID     int `gorm:"uniqueIndex:art_tag"`
}

func (ArticleTagTable) TableName() string {
	return "article_tag"
}
