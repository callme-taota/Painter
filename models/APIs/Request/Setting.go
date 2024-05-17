package Request

type SetSettingJSON struct {
	SiteName     string `json:"SiteName"`
	ICPCode      string `json:"ICPCode"`
	Github       string `json:"Github"`
	CanRegister  bool   `json:"CanRegister"`
	MailActive   bool   `json:"MailActive"`
	MailFrom     string `json:"MailFrom"`
	MailPassword string `json:"MailPassword"`
	MailSMTPHost string `json:"MailSMTPHost"`
	MailSMTPPort int    `json:"MailSMTPPort"`
}
