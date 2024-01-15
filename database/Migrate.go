package database

import (
	"painter-server-new/models"
	"painter-server-new/tolog"
)

func Migrate() error {
	err := Dbengine.AutoMigrate(&models.UserTable{})
	if err != nil {
		tolog.Log().Errorf("Migrate table user %e:", err).PrintAndWriteSafe()
		return err
	}
	err = Dbengine.AutoMigrate(&models.UserPassTable{})
	if err != nil {
		tolog.Log().Errorf("Migrate table userpass %e:", err).PrintAndWriteSafe()
		return err
	}
	err = Dbengine.AutoMigrate(&models.TagTable{})
	if err != nil {
		tolog.Log().Errorf("Migrate table tag %e:", err).PrintAndWriteSafe()
		return err
	}
	err = Dbengine.AutoMigrate(&models.HistoryTable{})
	if err != nil {
		tolog.Log().Errorf("Migrate table history %e:", err).PrintAndWriteSafe()
		return err
	}
	err = Dbengine.AutoMigrate(&models.FollowTable{})
	if err != nil {
		tolog.Log().Errorf("Migrate table follow %e:", err).PrintAndWriteSafe()
		return err
	}
	err = Dbengine.AutoMigrate(&models.CommentTable{})
	if err != nil {
		tolog.Log().Errorf("Migrate table comment %e:", err).PrintAndWriteSafe()
		return err
	}
	err = Dbengine.AutoMigrate(&models.CommentLikeTable{})
	if err != nil {
		tolog.Log().Errorf("Migrate table comment_like %e:", err).PrintAndWriteSafe()
		return err
	}
	err = Dbengine.AutoMigrate(&models.CollectionTable{})
	if err != nil {
		tolog.Log().Errorf("Migrate table collection %e:", err).PrintAndWriteSafe()
		return err
	}
	err = Dbengine.AutoMigrate(&models.CategoryTable{})
	if err != nil {
		tolog.Log().Errorf("Migrate table category %e:", err).PrintAndWriteSafe()
		return err
	}
	err = Dbengine.AutoMigrate(&models.ArticleTable{})
	if err != nil {
		tolog.Log().Errorf("Migrate table article %e:", err).PrintAndWriteSafe()
		return err
	}
	err = Dbengine.AutoMigrate(&models.ArticleCategoryTable{})
	if err != nil {
		tolog.Log().Errorf("Migrate table article_category %e:", err).PrintAndWriteSafe()
		return err
	}
	err = Dbengine.AutoMigrate(&models.ArticleContentTable{})
	if err != nil {
		tolog.Log().Errorf("Migrate table article_content %e:", err).PrintAndWriteSafe()
		return err
	}
	err = Dbengine.AutoMigrate(&models.ArticleLikeTable{})
	if err != nil {
		tolog.Log().Errorf("Migrate table article_like %e:", err).PrintAndWriteSafe()
		return err
	}
	err = Dbengine.AutoMigrate(&models.ArticleTable{})
	if err != nil {
		tolog.Log().Errorf("Migrate table article_tag %e:", err).PrintAndWriteSafe()
		return err
	}
	return nil
}
