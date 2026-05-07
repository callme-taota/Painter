package repository

import (
	"fmt"
	"strings"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"painter-2026/content-service/internal/domain"
)

type ArticleRecord struct {
	ArticleID    string `gorm:"primaryKey"`
	Title        string
	Summary      string
	Content      string
	AuthorID     string
	CategoryID   string
	Status       string
	LikeCount    int
	CollectCount int
	UpdatedAt    time.Time
	CreatedAt    time.Time
}

type CommentRecord struct {
	CommentID string `gorm:"primaryKey"`
	ArticleID string
	UserID    string
	Content   string
	LikeCount int
	CreatedAt time.Time
}

type TagRecord struct {
	TagID int `gorm:"primaryKey;autoIncrement"`
	Name  string
	Desc  string
	Count int
}

type CategoryRecord struct {
	CategoryID int `gorm:"primaryKey;autoIncrement"`
	Name       string
	Desc       string
}

type UserArticleRelation struct {
	ID        uint `gorm:"primaryKey"`
	UserID    string
	ArticleID string
	Type      string
}

type UserCommentLike struct {
	ID        uint `gorm:"primaryKey"`
	UserID    string
	CommentID string
}

type MySQLContentRepo struct {
	db *gorm.DB
}

func NewMySQLContentRepo(dsn string) (*MySQLContentRepo, error) {
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	if err := db.AutoMigrate(&ArticleRecord{}, &CommentRecord{}, &TagRecord{}, &CategoryRecord{}, &UserArticleRelation{}, &UserCommentLike{}, &ArticleSearchToken{}); err != nil {
		return nil, err
	}
	r := &MySQLContentRepo{db: db}
	if err := r.seed(); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *MySQLContentRepo) seed() error {
	var count int64
	if err := r.db.Model(&ArticleRecord{}).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		_ = r.db.Create(&ArticleRecord{
			ArticleID:  "a-1",
			Title:      "Painter-2026 Preview",
			Summary:    "first migrated article",
			Content:    "hello",
			AuthorID:   "u-admin",
			CategoryID: "1",
			Status:     "public",
		}).Error
	}
	var tagCount int64
	_ = r.db.Model(&TagRecord{}).Count(&tagCount).Error
	if tagCount == 0 {
		_ = r.db.Create([]TagRecord{{Name: "Go", Desc: "golang"}, {Name: "Vue", Desc: "frontend"}}).Error
	}
	var catCount int64
	_ = r.db.Model(&CategoryRecord{}).Count(&catCount).Error
	if catCount == 0 {
		_ = r.db.Create([]CategoryRecord{{Name: "默认分类", Desc: "default"}}).Error
	}
	return nil
}

func (r *MySQLContentRepo) List(start, limit int) ([]domain.Article, int) {
	var rows []ArticleRecord
	var total int64
	_ = r.db.Model(&ArticleRecord{}).Count(&total).Error
	_ = r.db.Order("updated_at desc").Offset(start).Limit(limit).Find(&rows).Error
	items := make([]domain.Article, 0, len(rows))
	for _, a := range rows {
		items = append(items, domain.Article{
			ArticleID:    a.ArticleID,
			Title:        a.Title,
			Summary:      a.Summary,
			Content:      a.Content,
			AuthorID:     a.AuthorID,
			CategoryID:   parseCategoryID(a.CategoryID),
			LikeCount:    a.LikeCount,
			CommentCount: 0,
			UpdatedAt:    a.UpdatedAt,
		})
	}
	return items, int(total)
}

func (r *MySQLContentRepo) Create(cmd domain.CreateArticleCommand) string {
	id := fmt.Sprintf("a-%d", time.Now().UnixNano())
	_ = r.db.Create(&ArticleRecord{
		ArticleID:  id,
		Title:      cmd.Title,
		Summary:    cmd.Summary,
		Content:    cmd.Content,
		AuthorID:   "u-admin",
		CategoryID: cmd.CategoryID,
		Status:     "public",
	}).Error
	return id
}

func (r *MySQLContentRepo) Count() int {
	var total int64
	_ = r.db.Model(&ArticleRecord{}).Count(&total).Error
	return int(total)
}

func (r *MySQLContentRepo) GetArticle(articleID string) (ArticleRecord, error) {
	var row ArticleRecord
	err := r.db.Where("article_id = ?", articleID).First(&row).Error
	return row, err
}

func (r *MySQLContentRepo) UpdateArticle(articleID string, updates map[string]interface{}) error {
	return r.db.Model(&ArticleRecord{}).Where("article_id = ?", articleID).Updates(updates).Error
}

func (r *MySQLContentRepo) DeleteArticle(articleID string) error {
	if err := r.DeleteArticleSearchIndex(articleID); err != nil {
		return err
	}
	return r.db.Where("article_id = ?", articleID).Delete(&ArticleRecord{}).Error
}

func (r *MySQLContentRepo) ListComments(articleID string) ([]CommentRecord, error) {
	var rows []CommentRecord
	err := r.db.Where("article_id = ?", articleID).Order("created_at asc").Find(&rows).Error
	return rows, err
}

func (r *MySQLContentRepo) CreateComment(articleID, userID, content string) (string, error) {
	id := fmt.Sprintf("cm-%d", time.Now().UnixNano())
	err := r.db.Create(&CommentRecord{CommentID: id, ArticleID: articleID, UserID: userID, Content: content}).Error
	return id, err
}

func (r *MySQLContentRepo) DeleteComment(commentID string) error {
	return r.db.Where("comment_id = ?", commentID).Delete(&CommentRecord{}).Error
}

func (r *MySQLContentRepo) LikeComment(userID, commentID string) error {
	return r.db.Create(&UserCommentLike{UserID: userID, CommentID: commentID}).Error
}

func (r *MySQLContentRepo) DislikeComment(userID, commentID string) error {
	return r.db.Where("user_id = ? AND comment_id = ?", userID, commentID).Delete(&UserCommentLike{}).Error
}

func (r *MySQLContentRepo) ListLikedComments(userID string) ([]UserCommentLike, error) {
	var rows []UserCommentLike
	err := r.db.Where("user_id = ?", userID).Find(&rows).Error
	return rows, err
}

func (r *MySQLContentRepo) UpsertUserArticleRelation(userID, articleID, t string) error {
	var rel UserArticleRelation
	err := r.db.Where("user_id = ? AND article_id = ? AND type = ?", userID, articleID, t).First(&rel).Error
	if err == nil {
		return nil
	}
	return r.db.Create(&UserArticleRelation{UserID: userID, ArticleID: articleID, Type: t}).Error
}

func (r *MySQLContentRepo) DeleteUserArticleRelation(userID, articleID, t string) error {
	return r.db.Where("user_id = ? AND article_id = ? AND type = ?", userID, articleID, t).Delete(&UserArticleRelation{}).Error
}

func (r *MySQLContentRepo) HasUserArticleRelation(userID, articleID, t string) (bool, error) {
	var c int64
	err := r.db.Model(&UserArticleRelation{}).Where("user_id = ? AND article_id = ? AND type = ?", userID, articleID, t).Count(&c).Error
	return c > 0, err
}

func (r *MySQLContentRepo) ListUserArticleRelations(userID, t string) ([]UserArticleRelation, error) {
	var rows []UserArticleRelation
	err := r.db.Where("user_id = ? AND type = ?", userID, t).Find(&rows).Error
	return rows, err
}

func (r *MySQLContentRepo) ListTags(keyword string) ([]TagRecord, error) {
	var rows []TagRecord
	q := r.db.Model(&TagRecord{})
	if keyword != "" {
		q = q.Where("lower(name) like ?", "%"+strings.ToLower(keyword)+"%")
	}
	err := q.Find(&rows).Error
	return rows, err
}

func (r *MySQLContentRepo) CreateTag(name, desc string) error {
	return r.db.Create(&TagRecord{Name: name, Desc: desc}).Error
}

func (r *MySQLContentRepo) UpdateTag(tagID int, name, desc string) error {
	updates := map[string]interface{}{}
	if name != "" {
		updates["name"] = name
	}
	if desc != "" {
		updates["desc"] = desc
	}
	if len(updates) == 0 {
		return nil
	}
	if tagID == 0 {
		return r.db.Model(&TagRecord{}).Where("id > 0").Limit(1).Updates(updates).Error
	}
	return r.db.Model(&TagRecord{}).Where("tag_id = ?", tagID).Updates(updates).Error
}

func (r *MySQLContentRepo) DeleteTag(tagID int) error {
	return r.db.Where("tag_id = ?", tagID).Delete(&TagRecord{}).Error
}

func (r *MySQLContentRepo) ListCategories() ([]CategoryRecord, error) {
	var rows []CategoryRecord
	err := r.db.Find(&rows).Error
	return rows, err
}

func (r *MySQLContentRepo) CreateCategory(name, desc string) error {
	return r.db.Create(&CategoryRecord{Name: name, Desc: desc}).Error
}

func (r *MySQLContentRepo) UpdateCategory(categoryID int, name, desc string) error {
	updates := map[string]interface{}{}
	if name != "" {
		updates["name"] = name
	}
	if desc != "" {
		updates["desc"] = desc
	}
	if len(updates) == 0 {
		return nil
	}
	if categoryID == 0 {
		return r.db.Model(&CategoryRecord{}).Where("id > 0").Limit(1).Updates(updates).Error
	}
	return r.db.Model(&CategoryRecord{}).Where("category_id = ?", categoryID).Updates(updates).Error
}

func parseCategoryID(raw string) string {
	if raw == "" {
		return "1"
	}
	return raw
}
