package httpapi

import (
	"crypto/tls"
	"fmt"
	"mime"
	"net"
	"net/mail"
	"net/smtp"
	"os"
	"strings"
	"time"
)

type codeMailer interface {
	Configured() bool
	SendCode(to, code, purpose string) error
}

type smtpMailer struct { host, port, username, password, from, site string }

func newSMTPMailer() *smtpMailer {
	host := strings.TrimSpace(os.Getenv("SMTP_HOST"))
	if host == "" { host = "smtp.gmail.com" }
	port := strings.TrimSpace(os.Getenv("SMTP_PORT"))
	if port == "" { port = "587" }
	return &smtpMailer{host:host, port:port, username:strings.TrimSpace(os.Getenv("SMTP_USERNAME")), password:strings.Join(strings.Fields(os.Getenv("SMTP_PASSWORD")), ""), from:strings.TrimSpace(os.Getenv("SMTP_FROM")), site:strings.TrimRight(strings.TrimSpace(os.Getenv("SITE_URL")), "/")}
}

func (m *smtpMailer) Configured() bool {
	if m.username == "" || m.password == "" || m.from == "" { return false }
	_, err := mail.ParseAddress(m.from)
	return err == nil && !strings.ContainsAny(m.host+m.port, "\r\n")
}

func (m *smtpMailer) SendCode(to, code, purpose string) error {
	if !m.Configured() { return fmt.Errorf("email delivery is not configured") }
	address, err := mail.ParseAddress(m.from)
	if err != nil { return err }
	subject, heading, explanation := "Код подтверждения Zently", "Подтвердите почту", "Введите этот код в Zently, чтобы завершить создание аккаунта."
	if purpose == "reset_password" { subject, heading, explanation = "Восстановление пароля Zently", "Сброс пароля", "Введите этот код в Zently, чтобы задать новый пароль." }
	body := fmt.Sprintf("%s\r\n\r\n%s\r\n\r\n%s\r\n\r\nКод действует 10 минут. Если вы не запрашивали это письмо, просто проигнорируйте его.\r\n", heading, explanation, code)
	if m.site != "" { body += "\r\nZently: " + m.site + "\r\n" }
	message := []byte("From: " + address.String() + "\r\nTo: " + to + "\r\nSubject: " + mime.QEncoding.Encode("UTF-8", subject) + "\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n" + body)
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(m.host, m.port), 10*time.Second)
	if err != nil { return err }
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(15*time.Second))
	client, err := smtp.NewClient(conn, m.host)
	if err != nil { return err }
	defer client.Close()
	if ok, _ := client.Extension("STARTTLS"); !ok { return fmt.Errorf("SMTP server does not offer STARTTLS") }
	if err := client.StartTLS(&tls.Config{ServerName:m.host, MinVersion:tls.VersionTLS12}); err != nil { return err }
	if err := client.Auth(smtp.PlainAuth("", m.username, m.password, m.host)); err != nil { return err }
	if err := client.Mail(address.Address); err != nil { return err }
	if err := client.Rcpt(to); err != nil { return err }
	writer, err := client.Data()
	if err != nil { return err }
	if _, err := writer.Write(message); err != nil { _ = writer.Close(); return err }
	if err := writer.Close(); err != nil { return err }
	return client.Quit()
}
