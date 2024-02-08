package database

import (
	"painter-server-new/models"
	"painter-server-new/tolog"
)

func CreateComment(articleID, userID int, content string) (int, error) {
	comment := &models.CommentTable{
		Content:   content,
		UserID:    userID,
		ArticleID: articleID,
	}
	result := Dbengine.Create(&comment)
	if result.Error != nil {
		tolog.Log().Infof("Error while CreateComment %e", result.Error).PrintAndWriteSafe()
		return -1, result.Error
	}
	commentID := comment.CommentID
	return commentID, nil
}

func DeleteComment(commentID int) (bool, error) {
	var comment models.CommentTable
	result := Dbengine.Delete(&comment, commentID)
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
	result := Dbengine.Create(&commentLike)
	if result.Error != nil {
		tolog.Log().Infof("Error while CreateCommentLike %e", result.Error).PrintAndWriteSafe()
		return false, result.Error
	}
	return true, nil
}

func DeleteCommentLike(commentID, userID int) (bool, error) {
	err := Dbengine.Where("comment_id = ? and user_id = ?", commentID, userID).Delete(&models.CommentLikeTable{}).Error
	if err != nil {
		tolog.Log().Infof("Error while DeleteArticleLike %e", err).PrintAndWriteSafe()
		return false, err
	}
	return true, nil
}
