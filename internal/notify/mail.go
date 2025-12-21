package notify

import (
	"context"
	"fmt"
	"strings"

	"github.com/BrunoTulio/logr"
	"github.com/wneessen/go-mail"
)

type (
	MailNotifier struct {
		log        logr.Logger
		smtpHost   string
		smtpPort   int
		smtpAuth   string
		tlsPolicy  bool
		username   string
		password   string
		recipients []string
		from       string
	}
)

func (m *MailNotifier) Success(ctx context.Context, msg string) error {
	subject := fmt.Sprintf("✅ Backup Success")
	body := fmt.Sprintf("Backup completed successfully, %s!", msg)
	return m.sendEmail(ctx, subject, body)
}

func (m *MailNotifier) Error(ctx context.Context, errMsg string) error {
	subject := fmt.Sprintf("❌ **Backup Failed** ")
	body := fmt.Sprintf("O processo de backup falhou.\n\nDetalhes do erro:\n%s", errMsg)
	return m.sendEmail(ctx, subject, body)
}

func NewMail(
	smtpHost string,
	smtpPort int,
	username string,
	password string,
	recipients []string,
	from string,
	smtpAuth string,
	tlsPolicy bool,
	log logr.Logger) Notifier {
	return &MailNotifier{
		log:        log,
		smtpHost:   smtpHost,
		smtpPort:   smtpPort,
		smtpAuth:   smtpAuth,
		tlsPolicy:  tlsPolicy,
		username:   username,
		password:   password,
		recipients: recipients,
		from:       from,
	}
}

func (m *MailNotifier) sendEmail(ctx context.Context, subject, body string) error {
	ms, err := m.toMessage(subject, body)
	if err != nil {
		return fmt.Errorf("toMessage: %w", err)
	}

	client, err := mail.NewClient(
		m.smtpHost,
		mail.WithPort(m.smtpPort),
		mail.WithSMTPAuth(mail.SMTPAuthPlain),
		mail.WithUsername(m.username),
		mail.WithPassword(m.password),
		mail.WithTLSPolicy(tlsPolicyFromBool(m.tlsPolicy)),
		mail.WithSMTPAuth(smtpAuthTypeFromString(m.smtpAuth)),
	)

	if err != nil {
		return fmt.Errorf("NewClient: %w", err)
	}

	defer func() {
		_ = client.Close()
	}()

	if err := client.DialAndSendWithContext(ctx, ms); err != nil {
		return fmt.Errorf("DialAndSend: %w", err)
	}

	return nil

}

func (m *MailNotifier) toMessage(subject, body string) (*mail.Msg, error) {
	msg := mail.NewMsg()
	if err := msg.From(m.from); err != nil {
		return nil, fmt.Errorf("from: %v", err)
	}

	for _, rec := range m.recipients {
		if err := msg.AddTo(rec); err != nil {
			return nil, fmt.Errorf("to: %v", err)
		}
	}

	msg.Subject(subject)
	msg.SetBodyString(mail.TypeTextPlain, body)

	return msg, nil
}
func smtpAuthTypeFromString(value string) mail.SMTPAuthType {
	switch strings.ToLower(strings.TrimSpace(value)) {

	case "auto", "autodiscover", "autodiscovery":
		return mail.SMTPAuthAutoDiscover

	case "plain":
		return mail.SMTPAuthPlain

	case "plain-noenc":
		return mail.SMTPAuthPlainNoEnc

	case "login":
		return mail.SMTPAuthLogin

	case "login-noenc":
		return mail.SMTPAuthLoginNoEnc

	case "cram-md5", "crammd5":
		return mail.SMTPAuthCramMD5

	case "scram-sha-1", "scram-sha1", "scramsha1":
		return mail.SMTPAuthSCRAMSHA1

	case "scram-sha-1-plus", "scram-sha1-plus", "scramsha1plus":
		return mail.SMTPAuthSCRAMSHA1PLUS

	case "scram-sha-256", "scram-sha256", "scramsha256":
		return mail.SMTPAuthSCRAMSHA256

	case "scram-sha-256-plus", "scram-sha256-plus", "scramsha256plus":
		return mail.SMTPAuthSCRAMSHA256PLUS

	case "xoauth2", "oauth2":
		return mail.SMTPAuthXOAUTH2

	case "none", "noauth", "":
		return mail.SMTPAuthNoAuth

	default:
		return mail.SMTPAuthNoAuth

	}
}

func tlsPolicyFromBool(enabled bool) mail.TLSPolicy {
	switch enabled {
	case true:
		return mail.TLSMandatory
	default:
		return mail.NoTLS
	}
}
