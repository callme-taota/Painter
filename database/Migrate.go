package database

import (
	"painter-server-new/models"

	"github.com/callme-taota/tolog"
)

func Migrate() error {
	err := DbEngine.AutoMigrate(&models.UserTable{})
	if err != nil {
		tolog.Errorf("Migrate table user %e:", err).PrintAndWriteSafe()
		return err
	}
	err = DbEngine.AutoMigrate(&models.UserPassTable{})
	if err != nil {
		tolog.Errorf("Migrate table userpass %e:", err).PrintAndWriteSafe()
		return err
	}
	err = DbEngine.AutoMigrate(&models.TagTable{})
	if err != nil {
		tolog.Errorf("Migrate table tag %e:", err).PrintAndWriteSafe()
		return err
	}
	err = DbEngine.AutoMigrate(&models.HistoryTable{})
	if err != nil {
		tolog.Errorf("Migrate table history %e:", err).PrintAndWriteSafe()
		return err
	}
	err = DbEngine.AutoMigrate(&models.FollowTable{})
	if err != nil {
		tolog.Errorf("Migrate table follow %e:", err).PrintAndWriteSafe()
		return err
	}
	err = DbEngine.AutoMigrate(&models.CommentTable{})
	if err != nil {
		tolog.Errorf("Migrate table comment %e:", err).PrintAndWriteSafe()
		return err
	}
	err = DbEngine.AutoMigrate(&models.CommentLikeTable{})
	if err != nil {
		tolog.Errorf("Migrate table comment_like %e:", err).PrintAndWriteSafe()
		return err
	}
	err = DbEngine.AutoMigrate(&models.CollectionTable{})
	if err != nil {
		tolog.Errorf("Migrate table collection %e:", err).PrintAndWriteSafe()
		return err
	}
	err = DbEngine.AutoMigrate(&models.CategoryTable{})
	if err != nil {
		tolog.Errorf("Migrate table category %e:", err).PrintAndWriteSafe()
		return err
	}
	err = DbEngine.AutoMigrate(&models.ArticleTable{})
	if err != nil {
		tolog.Errorf("Migrate table article %e:", err).PrintAndWriteSafe()
		return err
	}
	err = DbEngine.AutoMigrate(&models.ArticleContentTable{})
	if err != nil {
		tolog.Errorf("Migrate table article_content %e:", err).PrintAndWriteSafe()
		return err
	}
	err = DbEngine.AutoMigrate(&models.ArticleLikeTable{})
	if err != nil {
		tolog.Errorf("Migrate table article_like %e:", err).PrintAndWriteSafe()
		return err
	}
	err = DbEngine.AutoMigrate(&models.ArticleTagTable{})
	if err != nil {
		tolog.Errorf("Migrate table article_tag %e:", err).PrintAndWriteSafe()
		return err
	}
	err = DbEngine.AutoMigrate(&models.FileStorageTable{})
	if err != nil {
		tolog.Errorf("Migrate table file_storage %e:", err).PrintAndWriteSafe()
		return err
	}
	err = DbEngine.AutoMigrate(&models.VisitorRecordTable{})
	if err != nil {
		tolog.Errorf("Migrate table vis_record %e:", err).PrintAndWriteSafe()
		return err
	}
	err = DbEngine.AutoMigrate(&models.PainterSettingTable{})
	if err != nil {
		tolog.Errorf("Migrate table setting %e:", err).PrintAndWriteSafe()
		return err
	}
	err = DbEngine.AutoMigrate(&models.RuleTable{})
	if err != nil {
		tolog.Errorf("Migrate table rule %e:", err).PrintAndWriteSafe()
		return err
	}
	err = DbEngine.AutoMigrate(&models.GroupRuleTable{})
	if err != nil {
		tolog.Errorf("Migrate table group rule %e:", err).PrintAndWriteSafe()
		return err
	}
	err = DbEngine.AutoMigrate(&models.UserGroupTable{})
	if err != nil {
		tolog.Errorf("Migrate table user group %e:", err).PrintAndWriteSafe()
		return err
	}
	return nil
}
