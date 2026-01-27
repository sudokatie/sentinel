package alerter

import (
	"crypto/tls"
	"fmt"
	"net/smtp"
	"strings"
	"time"

	"github.com/katieblackabee/sentinel/internal/config"
)

type EmailSender struct {
	config *config.EmailConfig
}

func NewEmailSender(cfg *config.EmailConfig) *EmailSender {
	return &EmailSender{config: cfg}
}

func (e *EmailSender) Send(alert *Alert) error {
	subject, body := e.buildEmail(alert)

	msg := fmt.Sprintf("From: %s\r\n"+
		"To: %s\r\n"+
		"Subject: %s\r\n"+
		"MIME-Version: 1.0\r\n"+
		"Content-Type: text/plain; charset=\"utf-8\"\r\n"+
		"\r\n"+
		"%s",
		e.config.FromAddress,
		strings.Join(e.config.ToAddresses, ", "),
		subject,
		body,
	)

	addr := fmt.Sprintf("%s:%d", e.config.SMTPHost, e.config.SMTPPort)

	var auth smtp.Auth
	if e.config.SMTPUser != "" {
		auth = smtp.PlainAuth("", e.config.SMTPUser, e.config.SMTPPassword, e.config.SMTPHost)
	}

	if e.config.SMTPTLS {
		return e.sendWithTLS(addr, auth, msg)
	}

	return smtp.SendMail(addr, auth, e.config.FromAddress, e.config.ToAddresses, []byte(msg))
}

func (e *EmailSender) sendWithTLS(addr string, auth smtp.Auth, msg string) error {
	conn, err := tls.Dial("tcp", addr, &tls.Config{
		ServerName: e.config.SMTPHost,
	})
	if err != nil {
		// Try STARTTLS instead
		return e.sendWithSTARTTLS(addr, auth, msg)
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, e.config.SMTPHost)
	if err != nil {
		return fmt.Errorf("creating SMTP client: %w", err)
	}
	defer client.Close()

	if auth != nil {
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("SMTP auth: %w", err)
		}
	}

	if err := client.Mail(e.config.FromAddress); err != nil {
		return fmt.Errorf("SMTP mail: %w", err)
	}

	for _, to := range e.config.ToAddresses {
		if err := client.Rcpt(to); err != nil {
			return fmt.Errorf("SMTP rcpt: %w", err)
		}
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("SMTP data: %w", err)
	}

	_, err = w.Write([]byte(msg))
	if err != nil {
		return fmt.Errorf("writing message: %w", err)
	}

	err = w.Close()
	if err != nil {
		return fmt.Errorf("closing writer: %w", err)
	}

	return client.Quit()
}

func (e *EmailSender) sendWithSTARTTLS(addr string, auth smtp.Auth, msg string) error {
	client, err := smtp.Dial(addr)
	if err != nil {
		return fmt.Errorf("dialing SMTP: %w", err)
	}
	defer client.Close()

	// Try STARTTLS
	if ok, _ := client.Extension("STARTTLS"); ok {
		config := &tls.Config{ServerName: e.config.SMTPHost}
		if err := client.StartTLS(config); err != nil {
			return fmt.Errorf("STARTTLS: %w", err)
		}
	}

	if auth != nil {
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("SMTP auth: %w", err)
		}
	}

	if err := client.Mail(e.config.FromAddress); err != nil {
		return fmt.Errorf("SMTP mail: %w", err)
	}

	for _, to := range e.config.ToAddresses {
		if err := client.Rcpt(to); err != nil {
			return fmt.Errorf("SMTP rcpt: %w", err)
		}
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("SMTP data: %w", err)
	}

	_, err = w.Write([]byte(msg))
	if err != nil {
		return fmt.Errorf("writing message: %w", err)
	}

	err = w.Close()
	if err != nil {
		return fmt.Errorf("closing writer: %w", err)
	}

	return client.Quit()
}

func (e *EmailSender) buildEmail(alert *Alert) (subject, body string) {
	if alert.Type == "down" {
		return e.buildDownEmail(alert)
	}
	return e.buildRecoveryEmail(alert)
}

func (e *EmailSender) buildDownEmail(alert *Alert) (subject, body string) {
	subject = fmt.Sprintf("[SENTINEL] DOWN: %s", alert.Check.Name)

	body = fmt.Sprintf(`Service: %s
URL: %s
Status: DOWN
Time: %s
Error: %s

--
Sentinel Uptime Monitor`,
		alert.Check.Name,
		alert.Check.URL,
		alert.Timestamp.Format(time.RFC1123),
		alert.Error,
	)

	return subject, body
}

func (e *EmailSender) buildRecoveryEmail(alert *Alert) (subject, body string) {
	subject = fmt.Sprintf("[SENTINEL] RECOVERED: %s", alert.Check.Name)

	duration := "unknown"
	if alert.Incident != nil {
		duration = alert.Incident.DurationString()
	}

	body = fmt.Sprintf(`Service: %s
URL: %s
Status: UP
Time: %s
Downtime: %s

--
Sentinel Uptime Monitor`,
		alert.Check.Name,
		alert.Check.URL,
		alert.Timestamp.Format(time.RFC1123),
		duration,
	)

	return subject, body
}
