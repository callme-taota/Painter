package mail

import (
	"math/rand"
	"painter-server-new/cache"
	"painter-server-new/database"
	"painter-server-new/models"

	"github.com/callme-taota/tolog"

	"strconv"

	"gopkg.in/gomail.v2"
)

const subject = "Painter 验证"

func Sender(mail string) {
	body, code := CreateContent()
	msg := models.PostMessage{
		To:      mail,
		Subject: subject,
		Body:    body,
	}
	poster, err := database.GetMailSetting()
	if err != nil {
		tolog.Infof("Can't post email: %e", err).PrintAndWriteSafe()
		return
	}
	if !poster.Active {
		tolog.Infof("Can't post email: poster inactive").PrintAndWriteSafe()
		return
	}
	err = cache.CreateEmailCheck(mail, code)
	if err != nil {
		tolog.Infof("Can't post email: can't create code").PrintAndWriteSafe()
		return
	}

	tolog.Infoln("Post mail:", poster.From, msg.To, msg.Subject, msg.Body).PrintAndWriteSafe()
	m := gomail.NewMessage()
	m.SetHeader("From", poster.From)
	m.SetHeader("To", msg.To)
	m.SetHeader("Subject", msg.Subject)
	m.SetBody("text/html", msg.Body)

	tolog.Infoln("Post mail:", poster.SmtpPort, poster.SmtpHost, poster.From, poster.Password).PrintAndWriteSafe()
	d := gomail.NewDialer(poster.SmtpHost, poster.SmtpPort, poster.From, poster.Password)
	if err := d.DialAndSend(m); err != nil {
		tolog.Infof("Can't post email: post error, %e", err).PrintAndWriteSafe()
		return
	}
	tolog.Infof("Post email success").PrintAndWriteSafe()
	return
}

func CreateContent() (string, int) {
	n := RendNumber()
	nStr := strconv.Itoa(n)
	c := "<p>Painter 验证码: <strong>" + nStr + "</strong>。</p><p>出于安全原因，该验证码将于 5 分钟后失效。请勿将验证码透漏给他人。</p><p>若非您本人操作，请立即修改密码。</p>"
	return c, n
}

func RendNumber() int {
	return rand.Intn(9000) + 1000
}
