package database

import (
	"painter-server-new/models"
	"painter-server-new/tolog"
	"strconv"
)

var settingKey = []string{"mail_from", "mail_password", "mail_smtphost", "mail_smtpport", "mail_active"}

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
		s.Description = "0"
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
	active, _ := strconv.Atoi(mailActive.Value)
	if active == 1 {
		mailSetting.Active = true
	} else {
		mailSetting.Active = false
	}
	return mailSetting, nil
}
