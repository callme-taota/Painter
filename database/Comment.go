package database

import (
	"painter-server-new/models"
	"painter-server-new/models/APIs/Response"
	"painter-server-new/tolog"
)

func CreateComment(articleID, userID int, content string) (int, error) {
	comment := &models.CommentTable{
		Content:   content,
		UserID:    userID,
		ArticleID: articleID,
	}
	result := DbEngine.Create(&comment)
	if result.Error != nil {
		tolog.Log().Infof("Error while CreateComment %e", result.Error).PrintAndWriteSafe()
		return -1, result.Error
	}
	commentID := comment.CommentID
	return commentID, nil
}

func DeleteComment(userID, commentID int) (bool, error) {
	var comment models.CommentTable
	result := DbEngine.First(&comment, commentID)
	if result.Error != nil {
		tolog.Log().Infof("Error while DeleteComment %e", result.Error).PrintAndWriteSafe()
		return false, result.Error
	}
	if comment.UserID != userID {
		return false, nil
	}
	result = DbEngine.Delete(&comment, commentID)
	if result.Error != nil {
		tolog.Log().Infof("Error while DeleteComment %e", result.Error).PrintAndWriteSafe()
		return false, result.Error
	}
	return true, nil
}

func CreateCommentLike(commentID, userID int) (bool, error) {
	commentLike := &models.CommentLikeTable{
		CommentID: commentID,
		UserID:    userID,
	}
	result := DbEngine.Create(&commentLike)
	if result.Error != nil {
		tolog.Log().Infof("Error while CreateCommentLike %e", result.Error).PrintAndWriteSafe()
		return false, result.Error
	}
	return true, nil
}

func DeleteCommentLike(commentID, userID int) (bool, error) {
	err := DbEngine.Where("comment_id = ? and user_id = ?", commentID, userID).Delete(&models.CommentLikeTable{}).Error
	if err != nil {
		tolog.Log().Infof("Error while DeleteArticleLike %e", err).PrintAndWriteSafe()
		return false, err
	}
	return true, nil
}

func GetCommentByArticleID(articleID, limit, offset int) ([]Response.FullComment, error) {
	var comments []Response.FullComment
	res := DbEngine.Select("comment.*, user.nick_name, user.header_field, COUNT(comment_like.comment_id) AS like_count").
		Joins("INNER JOIN user ON comment.user_id = user.id").
		Joins("LEFT JOIN comment_like ON comment.comment_id = comment_like.comment_id").
		Where("article_id = ?", articleID).
		Group("comment.comment_id").
		Limit(limit).
		Offset(offset).
		Find(&comments)
	if res.Error != nil {
		return nil, res.Error
	}
	return comments, nil
}

func GetCommentLikeCount(commentID int) (int, error) {
	var count int64
	res := DbEngine.Model(&models.CommentLikeTable{}).Where("comment_id = ?", commentID).Count(&count)
	if res.Error != nil {
		return 0, res.Error
	}
	return int(count), nil
}

func GetCommentsWithLikeInfoByArticleID(articleID, limit, offset, userID int) ([]Response.FullCommentWithLike, error) {
	var comments []Response.FullCommentWithLike
	res := DbEngine.Select("comment.*, user.nick_name, user.header_field, COUNT(cl.comment_id) as like_count, MAX(cl.user_id) = ? as liked",
		userID).
		Joins("inner join user on comment.user_id = user.id").
		Joins("left join comment_like cl on comment.comment_id = cl.comment_id").
		Where("article_id = ?", articleID).
		Group("comment.comment_id").
		Limit(limit).
		Offset(offset).
		Find(&comments)
	if res.Error != nil {
		return nil, res.Error
	}
	return comments, nil
}
