package models

type PostMessage struct {
	To      string
	Subject string
	Body    string
}

type MailSetting struct {
	From     string
	Password string
	SmtpHost string
	SmtpPort int
	Active   bool
}
