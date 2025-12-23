package config

import (
	"fmt"
	"time"
)

type Config struct {
	Server             Server             `yaml:"server"`
	Timezone           string             `yaml:"timezone"`
	Database           DatabaseConfig     `yaml:"database"`
	LocalBackup        LocalBackupConfig  `yaml:"local"`
	RemoteProviders    []RemoteProvider   `yaml:"providers"`
	Notification       NotificationConfig `yaml:"notification"`
	EncryptionKey      string             `yaml:"encryption_key"`
	RunOnStartup       bool               `yaml:"run_on_startup"`
	RunRemoteOnStartup bool               `yaml:"run_remote_on_startup"`
}

type Server struct {
	Addr string `yaml:"addr"`
}

type DatabaseConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	Name     string `yaml:"name"`
}

type RetentionConfig struct {
	RetentionDays *int `yaml:"retention_days"`
	MaxBackups    *int `yaml:"max_backups"`
}
type LocalBackupConfig struct {
	Dir       string          `yaml:"dir"`
	Schedule  []string        `yaml:"schedule"`
	Retention RetentionConfig `yaml:"retention"`
	Enabled   bool            `yaml:"enabled"`
}

type RemoteProvider struct {
	Name        string            `yaml:"name"`
	Type        string            `yaml:"type"` // "s3", "gdrive", "dropbox"
	Enabled     bool              `yaml:"enabled"`
	Schedule    []string          `yaml:"schedule"`
	Path        string            `yaml:"path"`
	MaxVersions int               `yaml:"maxVersions"` // 0 = sem versionamento
	Timeout     int               `yaml:"timeout"`     // segundos
	Config      map[string]string `yaml:"config"`
}

type NotificationConfig struct {
	SuccessEnabled bool `yaml:"success_enabled"`
	ErrorEnabled   bool `yaml:"error_enabled"`

	Emails       []string `yaml:"emails"`
	EmailFrom    string   `yaml:"email_from"`
	SMTPServer   string   `yaml:"smtp_server"`
	SMTPPort     int      `yaml:"smtp_port"`
	SMTPUser     string   `yaml:"smtp_user"`
	SMTPPassword string   `yaml:"smtp_password"`
	SMTPAuth     string   `yaml:"smtp_auth"`
	SMTPTLS      bool     `yaml:"smtp_tls"`

	DiscordWebhookURL string `yaml:"discord_webhook_url"`

	TelegramBotToken string `yaml:"telegram_bot_token"`
	TelegramChatID   string `yaml:"telegram_chat_id"`
}

func (c *Config) GetLocation() (*time.Location, error) {
	return time.LoadLocation(c.Timezone)
}

func (c *Config) MustLocation() *time.Location {
	loc, err := c.GetLocation()

	if err != nil {
		panic(err)
	}
	return loc
}

func (c *DatabaseConfig) ConnectionString() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s",
		c.Username,
		c.Password,
		c.Host,
		c.Port,
		c.Name,
	)
}

func (c *Config) IsEncryptEnabled() bool {
	return c.EncryptionKey != ""
}

func (c *Config) IsNotifyMail() bool {
	return c.Notification.IsMails()
}

func (c *Config) IsNotifyDiscord() bool {
	return c.Notification.DiscordWebhookURL != ""
}

func (c *Config) IsNotifyTelegram() bool {
	return c.Notification.TelegramBotToken != ""
}

func (c *NotificationConfig) IsMails() bool {
	return len(c.Emails) > 0
}

func (r *RetentionConfig) HasRetentionDays() bool {
	return r.RetentionDays != nil
}

func (r *RetentionConfig) HasMaxBackups() bool {
	return r.MaxBackups != nil
}
