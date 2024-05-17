package database

import (
	"painter-server-new/models"
	"painter-server-new/models/APIs/Request"
	"painter-server-new/tolog"
	"painter-server-new/utils"
	"strconv"
)

var settingKey = []string{"mail_from", "mail_password", "mail_smtphost", "mail_smtpport", "mail_active", "site_name", "github_href", "icp_code", "can_register"}

func InitSettings() {
	for _, setting := range settingKey {
		CheckKeyExistOrCreate(setting)
	}
}

func CheckKeyExistOrCreate(key string) (bool, error) {
	s := models.PainterSettingTable{}
	res := DbEngine.Where("name = ?", key).First(&s)
	if res.RowsAffected == 0 {
		s.Name = key
		s.Value = "0"
		res = DbEngine.Create(&s)
		if res.Error != nil {
			tolog.Log().Infof("Error while create setting %e", res.Error).PrintAndWriteSafe()
			return false, res.Error
		} else {
			return true, nil
		}
	}
	return true, nil
}

func GetMailSetting() (models.MailSetting, error) {
	mailSetting := models.MailSetting{}
	//from
	mailFrom := models.PainterSettingTable{}
	res := DbEngine.Where("name = 'mail_from'").First(&mailFrom)
	if res.Error != nil {
		tolog.Log().Infof("Error while GetMailSetting %e", res.Error).PrintAndWriteSafe()
		return mailSetting, res.Error
	}
	mailSetting.From = mailFrom.Value
	//password
	mailPassword := models.PainterSettingTable{}
	res = DbEngine.Where("name = 'mail_password'").First(&mailPassword)
	if res.Error != nil {
		tolog.Log().Infof("Error while GetMailSetting %e", res.Error).PrintAndWriteSafe()
		return mailSetting, res.Error
	}
	mailSetting.Password = mailPassword.Value
	//host
	mailHost := models.PainterSettingTable{}
	res = DbEngine.Where("name = 'mail_smtphost'").First(&mailHost)
	if res.Error != nil {
		tolog.Log().Infof("Error while GetMailSetting %e", res.Error).PrintAndWriteSafe()
		return mailSetting, res.Error
	}
	mailSetting.SmtpHost = mailHost.Value
	//port
	mailPort := models.PainterSettingTable{}
	res = DbEngine.Where("name = 'mail_smtpport'").First(&mailPort)
	if res.Error != nil {
		tolog.Log().Infof("Error while GetMailSetting %e", res.Error).PrintAndWriteSafe()
		return mailSetting, res.Error
	}
	mailSetting.SmtpPort, _ = strconv.Atoi(mailPort.Value)
	//active
	mailActive := models.PainterSettingTable{}
	res = DbEngine.Where("name = 'mail_active'").First(&mailActive)
	if res.Error != nil {
		tolog.Log().Infof("Error while GetMailSetting %e", res.Error).PrintAndWriteSafe()
		return mailSetting, res.Error
	}
	mailSetting.Active = mailActive.Value == "1"
	return mailSetting, nil
}

func GetGithubHref() (string, error) {
	setting := models.PainterSettingTable{}
	res := DbEngine.Where("name = 'github_href'").First(&setting)
	if res.Error != nil {
		tolog.Log().Infof("Error while GetGithubHref %e", res.Error).PrintAndWriteSafe()
		return "", res.Error
	}
	return setting.Value, nil
}

func GetICPCode() (string, error) {
	setting := models.PainterSettingTable{}
	res := DbEngine.Where("name = 'icp_code'").First(&setting)
	if res.Error != nil {
		tolog.Log().Infof("Error while GetICPCode %e", res.Error).PrintAndWriteSafe()
		return "", res.Error
	}
	return setting.Value, nil
}

func CanRegister() (bool, error) {
	setting := models.PainterSettingTable{}
	res := DbEngine.Where("name = 'can_register'").First(&setting)
	if res.Error != nil {
		tolog.Log().Infof("Error while GetICPCode %e", res.Error).PrintAndWriteSafe()
		return false, res.Error
	}
	return setting.Value == "1", nil
}

func GetSiteName() (string, error) {
	setting := models.PainterSettingTable{}
	res := DbEngine.Where("name = 'site_name'").First(&setting)
	if res.Error != nil {
		tolog.Log().Infof("Error while GetICPCode %e", res.Error).PrintAndWriteSafe()
		return "", res.Error
	}
	return setting.Value, nil
}

func SetSetting(json Request.SetSettingJSON) error {
	settingItems := []models.PainterSettingTable{
		{Name: "site_name", Value: json.SiteName},
		{Name: "icp_code", Value: json.ICPCode},
		{Name: "github_href", Value: json.Github},
		{Name: "can_register", Value: utils.Bool2String(json.CanRegister)},
		{Name: "mail_active", Value: utils.Bool2String(json.MailActive)},
		{Name: "mail_from", Value: json.MailFrom},
		{Name: "mail_password", Value: json.MailPassword},
		{Name: "mail_smtphost", Value: json.MailSMTPHost},
		{Name: "mail_smtpport", Value: utils.Int2String(json.MailSMTPPort)},
	}
	for _, item := range settingItems {
		var setting models.PainterSettingTable
		DbEngine.Where("name = ?", item.Name).First(&setting)
		setting.Value = item.Value
		if err := DbEngine.Save(&setting).Error; err != nil {
			return err
		}
	}
	return nil
}
