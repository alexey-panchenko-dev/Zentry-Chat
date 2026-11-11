package httpapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"net/mail"
	"os"
	"strings"
	"time"
)

type codeMailer interface {
	Configured() bool
	SendCode(to, code, purpose string) error
}

type transactionalMailer struct {
	apiKey string
	from   string
	name   string
	site   string
	client *http.Client
}

func newTransactionalMailer() *transactionalMailer {
	name := strings.TrimSpace(os.Getenv("BREVO_FROM_NAME"))
	if name == "" {
		name = "Zently"
	}
	return &transactionalMailer{
		apiKey: strings.TrimSpace(os.Getenv("BREVO_API_KEY")),
		from:   strings.TrimSpace(os.Getenv("BREVO_FROM_EMAIL")),
		name:   name,
		site:   strings.TrimRight(strings.TrimSpace(os.Getenv("SITE_URL")), "/"),
		client: &http.Client{Timeout: 15 * time.Second},
	}
}

func (m *transactionalMailer) Configured() bool {
	if m.apiKey == "" || m.from == "" || strings.ContainsAny(m.apiKey+m.from+m.name, "\r\n") {
		return false
	}
	address, err := mail.ParseAddress(m.from)
	return err == nil && address.Address == m.from
}

func (m *transactionalMailer) SendCode(to, code, purpose string) error {
	if !m.Configured() {
		return fmt.Errorf("transactional email provider is not configured")
	}
	subject := "Код подтверждения Zently"
	body := "Введите этот код в Zently, чтобы завершить создание аккаунта."
	if purpose == "reset_password" {
		subject = "Восстановление пароля Zently"
		body = "Введите этот код в Zently, чтобы задать новый пароль."
	}
	textBody := body + "\n\nКод: " + code + "\n\nКод действует 10 минут. Если вы не запрашивали это письмо, просто проигнорируйте его."
	if m.site != "" {
		textBody += "\n\nZently: " + m.site
	}
	markup, err := template.New("zently-email").Parse(`
<!doctype html><html lang="ru"><body style="margin:0;padding:24px 12px;background:#0b090e;color:#f5f3f7;font-family:Arial,Helvetica,sans-serif">
<table role="presentation" width="100%" cellspacing="0" cellpadding="0" border="0"><tr><td align="center">
<table role="presentation" width="100%" cellspacing="0" cellpadding="0" border="0" style="max-width:520px;background:#17141b;border:1px solid #342532;border-radius:22px;overflow:hidden">
<tr><td style="padding:5px;background:linear-gradient(100deg,#ff1493 0%,#17141b 54%,#b7ff39 100%)"><div style="height:3px"></div></td></tr>
<tr><td style="padding:28px 30px 10px">
<table role="presentation" cellspacing="0" cellpadding="0" border="0"><tr><td width="42" height="38" align="center" valign="middle" style="width:42px;height:38px;border-radius:11px;background:#ff1493"><table role="presentation" cellspacing="0" cellpadding="0" border="0"><tr><td width="20" height="4" style="width:20px;height:4px;border-radius:4px;background:#0b090e"></td><td width="5" style="width:5px"></td><td width="7" height="7" style="width:7px;height:7px;border-radius:50%;background:#b7ff39"></td></tr></table></td><td style="padding-left:11px;color:#f5f3f7;font-size:25px;font-weight:bold;letter-spacing:-1px">zently</td></tr></table>
<p style="margin:30px 0 8px;color:#b7ff39;font-size:11px;letter-spacing:3px;text-transform:uppercase">Ближе друг к другу</p>
<h1 style="margin:0;color:#f5f3f7;font-size:28px;line-height:1.2">{{.Heading}}</h1>
<p style="margin:14px 0 0;color:#b7aebc;font-size:15px;line-height:1.6">{{.Explanation}}</p>
</td></tr>
<tr><td style="padding:24px 30px 10px"><div style="padding:18px 16px;text-align:center;border:1px solid #53334c;border-radius:16px;background:#211a24"><div style="color:#b7aebc;font-size:12px;letter-spacing:1px">ТВОЙ КОД</div><div style="padding-top:8px;color:#b7ff39;font-size:34px;font-weight:bold;letter-spacing:9px">{{.Code}}</div></div></td></tr>
<tr><td style="padding:8px 30px 28px;color:#b7aebc;font-size:13px;line-height:1.6">Код действует <strong style="color:#f5f3f7">10 минут</strong>. Если вы не запрашивали это письмо, просто проигнорируйте его.{{if .Site}}<br><br><a href="{{.Site}}" style="color:#ff1493;text-decoration:none">Открыть Zently</a>{{end}}</td></tr>
<tr><td style="height:3px;background:linear-gradient(90deg,#ff1493,#17141b 54%,#b7ff39)"></td></tr>
</table></td></tr></table></body></html>`)
	if err != nil {
		return err
	}
	var htmlBody bytes.Buffer
	if err := markup.Execute(&htmlBody, struct{ Heading, Explanation, Code, Site string }{subject, body, code, m.site}); err != nil {
		return err
	}
	requestBody, err := json.Marshal(map[string]any{
		"sender": map[string]string{"name": m.name, "email": m.from},
		"to":     []map[string]string{{"email": to}},
		"subject": subject,
		"textContent": textBody,
		"htmlContent": htmlBody.String(),
	})
	if err != nil {
		return err
	}
	request, err := http.NewRequest(http.MethodPost, "https://api.brevo.com/v3/smtp/email", bytes.NewReader(requestBody))
	if err != nil {
		return err
	}
	request.Header.Set("accept", "application/json")
	request.Header.Set("content-type", "application/json")
	request.Header.Set("api-key", m.apiKey)
	response, err := m.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("transactional email provider returned HTTP %d", response.StatusCode)
	}
	return nil
}
