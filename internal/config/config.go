package config

import (
	"fmt"
	"strconv"
	"time"
)

type Config struct {
	Server             Server
	Timezone           string
	Database           DatabaseConfig
	LocalBackup        LocalBackupConfig
	RemoteProviders    []RemoteProvider
	Notification       NotificationConfig
	EncryptionKey      string
	RunOnStartup       bool
	RunRemoteOnStartup bool
}

type Server struct {
	Addr string
}

type DatabaseConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	Name     string
}

type RetentionConfig struct {
	RetentionDays *int // Opcional
	MaxBackups    *int // Opcional
}
type LocalBackupConfig struct {
	Dir       string
	Schedule  []string // ["08:00", "20:00"]
	Retention RetentionConfig
	Enabled   bool
}

type RemoteProvider struct {
	Name        string
	Type        string // "s3", "gdrive", "dropbox"
	Enabled     bool
	Schedule    []string
	ScheduleDay *int // 0-6 para dia da semana
	Path        string
	MaxVersions int // 0 = sem versionamento
	Timeout     int // segundos
	Config      map[string]string
}

type NotificationConfig struct {
	SuccessEnabled bool
	ErrorEnabled   bool

	Emails       []string
	EmailFrom    string
	SMTPServer   string
	SMTPPort     int
	SMTPUser     string
	SMTPPassword string
	SMTPAuth     string
	SMTPTLS      bool

	DiscordWebhookURL string

	TelegramBotToken string
	TelegramChatID   string
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

func LoadFromEnv() (*Config, error) {
	cfg := &Config{
		Timezone:           stringOrEmpty("TZ", time.UTC.String()),
		RunOnStartup:       boolOrEmpty("RUN_ON_STARTUP", false),
		RunRemoteOnStartup: boolOrEmpty("RUN_REMOTE_ON_STARTUP", false),
		EncryptionKey:      stringOrEmpty("BACKUP_ENCRYPTION_KEY", ""),
	}

	cfg.Server = Server{
		Addr: stringOrEmpty("SERVER_ADDR", ":8080"),
	}

	cfg.Database = DatabaseConfig{
		Host:     mustString("DATABASE_HOST"),
		Port:     intOrEmpty("DATABASE_PORT", 5432),
		Username: mustString("DATABASE_USERNAME"),
		Password: mustString("DATABASE_PASSWORD"),
		Name:     mustString("DATABASE_NAME"),
	}

	cfg.LocalBackup = LocalBackupConfig{
		Dir:      stringOrEmpty("BACKUP_DIR", "/backups"),
		Schedule: stringsOrEmpty("BACKUP_SCHEDULE", []string{}),
		Enabled:  true,
	}
	if days := stringOrEmpty("RETENTION_DAYS", ""); days != "" {
		if d, err := strconv.Atoi(days); err == nil {
			cfg.LocalBackup.Retention.RetentionDays = &d
		}
	}
	if limit := stringOrEmpty("BACKUP_LIMIT", ""); limit != "" {
		if d, err := strconv.Atoi(limit); err == nil {
			cfg.LocalBackup.Retention.MaxBackups = &d
		}
	}

	cfg.RemoteProviders = loadProviders()

	cfg.Notification = NotificationConfig{
		SuccessEnabled:    boolOrEmpty("NOTIFICATION_SUCCESS_ENABLED", false),
		ErrorEnabled:      boolOrEmpty("NOTIFICATION_ERROR_ENABLED", false),
		Emails:            stringsOrEmpty("NOTIFICATION_EMAIL", []string{}),
		EmailFrom:         stringOrEmpty("NOTIFICATION_EMAIL_FROM", ""),
		SMTPServer:        stringOrEmpty("SMTP_SERVER", ""),
		SMTPPort:          intOrEmpty("SMTP_PORT", 587),
		SMTPUser:          stringOrEmpty("SMTP_USER", ""),
		SMTPPassword:      stringOrEmpty("SMTP_PASSWORD", ""),
		SMTPAuth:          stringOrEmpty("SMTP_AUTH_METHOD", "login"),
		SMTPTLS:           boolOrEmpty("SMTP_TLS", true),
		DiscordWebhookURL: stringOrEmpty("DISCORD_WEBHOOK_URL", ""),
		TelegramBotToken:  stringOrEmpty("TELEGRAM_BOT_TOKEN", ""),
		TelegramChatID:    stringOrEmpty("TELEGRAM_CHAT_ID", ""),
	}

	return cfg, cfg.Validate()
}

func loadProviders() []RemoteProvider {
	var providers []RemoteProvider

	// 1. S3 (AWS/MinIO/Wasabi/DigitalOcean Spaces)
	if s3 := loadS3Provider(); s3 != nil {
		providers = append(providers, *s3)
	}

	// 2. Google Drive
	if gdrive := loadGDriveProvider(); gdrive != nil {
		providers = append(providers, *gdrive)
	}

	// 3. Dropbox
	if dropbox := loadDropboxProvider(); dropbox != nil {
		providers = append(providers, *dropbox)
	}

	// 4. Mega
	if mega := loadMegaProvider(); mega != nil {
		providers = append(providers, *mega)
	}

	// 5. Google Cloud Storage (GCS)
	if gcs := loadGCSProvider(); gcs != nil {
		providers = append(providers, *gcs)
	}

	return providers
}

func loadS3Provider() *RemoteProvider {
	prefix := "REMOTE_S3_"

	if !boolOrEmpty(prefix+"ENABLED", false) {
		return nil
	}

	return &RemoteProvider{
		Name:        "s3",
		Type:        "s3",
		Enabled:     true,
		Path:        stringOrEmpty(prefix+"PATH", ""),
		Schedule:    stringsOrEmpty(prefix+"SCHEDULE", []string{}),
		MaxVersions: intOrEmpty(prefix+"MAX_VERSIONS", 0),
		Timeout:     intOrEmpty(prefix+"TIMEOUT", 7200),
		Config: map[string]string{
			"provider":          stringOrEmpty(prefix+"PROVIDER", "AWS"),
			"access_key_id":     stringOrEmpty(prefix+"ACCESS_KEY_ID", ""),
			"secret_access_key": stringOrEmpty(prefix+"SECRET_ACCESS_KEY", ""),
			"region":            stringOrEmpty(prefix+"REGION", "us-east-1"),
			"endpoint":          stringOrEmpty(prefix+"ENDPOINT", ""),
			"acl":               stringOrEmpty(prefix+"ACL", "private"),
			"force_path_style":  stringOrEmpty(prefix+"FORCE_PATH_STYLE", "false"),
			"no_check_bucket":   stringOrEmpty(prefix+"NO_CHECK_BUCKET", "true"),
		},
	}
}

func loadGDriveProvider() *RemoteProvider {
	prefix := "REMOTE_GDRIVE_"

	if !boolOrEmpty(prefix+"ENABLED", false) {
		return nil
	}
	tokenBase64 := stringOrEmpty(prefix+"TOKEN", "")
	return &RemoteProvider{
		Name:        "gdrive",
		Type:        "drive",
		Enabled:     true,
		Path:        stringOrEmpty(prefix+"PATH", ""),
		Schedule:    stringsOrEmpty(prefix+"SCHEDULE", []string{}),
		MaxVersions: intOrEmpty(prefix+"MAX_VERSIONS", 0),
		Timeout:     intOrEmpty(prefix+"TIMEOUT", 7200),
		Config: map[string]string{
			"token": decodeBase64(tokenBase64),
			"scope": stringOrEmpty(prefix+"SCOPE", "drive"),
		},
	}
}

func loadDropboxProvider() *RemoteProvider {
	prefix := "REMOTE_DROPBOX_"

	if !boolOrEmpty(prefix+"ENABLED", false) {
		return nil
	}

	tokenBase64 := stringOrEmpty(prefix+"TOKEN", "")

	return &RemoteProvider{
		Name:        "dropbox",
		Type:        "dropbox",
		Enabled:     true,
		Path:        stringOrEmpty(prefix+"PATH", ""),
		Schedule:    stringsOrEmpty(prefix+"SCHEDULE", []string{}),
		MaxVersions: intOrEmpty(prefix+"MAX_VERSIONS", 0),
		Timeout:     intOrEmpty(prefix+"TIMEOUT", 7200),
		Config: map[string]string{
			"token": decodeBase64(tokenBase64),
		},
	}
}

func loadMegaProvider() *RemoteProvider {
	prefix := "REMOTE_MEGA_"

	if !boolOrEmpty(prefix+"ENABLED", false) {
		return nil
	}

	return &RemoteProvider{
		Name:        "mega",
		Type:        "mega",
		Enabled:     true,
		Path:        stringOrEmpty(prefix+"PATH", ""),
		Schedule:    stringsOrEmpty(prefix+"SCHEDULE", []string{}),
		MaxVersions: intOrEmpty(prefix+"MAX_VERSIONS", 0),
		Timeout:     intOrEmpty(prefix+"TIMEOUT", 7200),
		Config: map[string]string{
			"user": stringOrEmpty(prefix+"USER", ""),
			"pass": stringOrEmpty(prefix+"PASS", ""),
		},
	}
}

func loadGCSProvider() *RemoteProvider {
	prefix := "REMOTE_GCS_"

	if !boolOrEmpty(prefix+"ENABLED", false) {
		return nil
	}

	accountBase64 := stringOrEmpty(prefix+"SERVICE_ACCOUNT_CREDENTIALS", "")

	return &RemoteProvider{
		Name:        "gcs",
		Type:        "google cloud storage",
		Enabled:     true,
		Path:        stringOrEmpty(prefix+"PATH", ""),
		Schedule:    stringsOrEmpty(prefix+"SCHEDULE", []string{}),
		MaxVersions: intOrEmpty(prefix+"MAX_VERSIONS", 0),
		Timeout:     intOrEmpty(prefix+"TIMEOUT", 7200),
		Config: map[string]string{
			"service_account_credentials": decodeBase64(accountBase64),
			"project_number":              stringOrEmpty(prefix+"PROJECT_NUMBER", ""),
			"bucket_policy_only":          stringOrEmpty(prefix+"BUCKET_POLICY_ONLY", "false"),
			"location":                    stringOrEmpty(prefix+"LOCATION", ""),
			"storage_class":               stringOrEmpty(prefix+"STORAGE_CLASS", "STANDARD"),
		},
	}
}
