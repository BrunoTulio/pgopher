package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/BrunoTulio/pgopher/internal/utils"
	"gopkg.in/yaml.v3"
)

type ConfigField struct {
	Name   string
	Base64 bool
}

type ProviderEnvSchema struct {
	Prefix string
	Config map[string]ConfigField
}

var (
	providerSchemas = map[string]ProviderEnvSchema{
		"s3": {
			Prefix: "REMOTE_S3_",
			Config: map[string]ConfigField{
				"PROVIDER": {
					Name: "provider",
				},
				"ACCESS_KEY_ID": {
					Name: "access_key_id",
				},
				"SECRET_ACCESS_KEY": {
					Name: "secret_access_key",
				},
				"REGION": {
					Name: "region",
				},
				"ENDPOINT": {
					Name: "endpoint",
				},
				"ACL": {
					Name: "acl",
				},
				"FORCE_PATH_STYLE": {
					Name: "force_path_style",
				},
				"NO_CHECK_BUCKET": {
					Name: "no_check_bucket",
				},
			},
		},
		"drive": {
			Prefix: "REMOTE_GDRIVE_",
			Config: map[string]ConfigField{
				"TOKEN": {
					Name:   "token",
					Base64: true,
				},
				"SCOPE": {
					Name: "scope",
				},
			},
		},
		"dropbox": {
			Prefix: "REMOTE_DROPBOX_",
			Config: map[string]ConfigField{
				"TOKEN": {
					Name:   "token",
					Base64: true,
				},
			},
		},
		"mega": {
			Prefix: "REMOTE_MEGA_",
			Config: map[string]ConfigField{
				"USER": {
					Name: "user",
				},
				"PASS": {
					Name: "pass",
				},
			},
		},
		"gcs": {
			Prefix: "REMOTE_GCS_",
			Config: map[string]ConfigField{
				"SERVICE_ACCOUNT_CREDENTIALS": {
					Name:   "service_account",
					Base64: true,
				},
				"PROJECT_NUMBER": {
					Name: "project_number",
				},
				"BUCKET_POLICY_ONLY": {
					Name: "bucket_policy_only",
				},
				"LOCATION": {
					Name: "location",
				},
				"STORAGE_CLASS": {
					Name: "storage_class",
				},
			},
		},
	}
)

func LoadFromYAML(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read yaml: %w", err)
	}

	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("unmarshal yaml: %w", err)
	}

	loadEnvOverrides(cfg)

	return cfg, cfg.Validate()
}

func loadEnvOverrides(cfg *Config) {

	if timezone, ok := stringLookup("TZ"); ok {
		cfg.Timezone = timezone
	}
	if runOnStartup, ok := boolLookup("RUN_ON_STARTUP"); ok {
		cfg.RunOnStartup = runOnStartup
	}
	if runRemoteOnStartup, ok := boolLookup("RUN_REMOTE_ON_STARTUP"); ok {
		cfg.RunRemoteOnStartup = runRemoteOnStartup
	}
	if encryptionKey, ok := stringLookup("BACKUP_ENCRYPTION_KEY"); ok {
		cfg.EncryptionKey = encryptionKey
	}

	if serverAddr, ok := stringLookup("SERVER_ADDR"); ok {
		cfg.Server.Addr = serverAddr
	}

	if databaseHost, ok := stringLookup("DATABASE_HOST"); ok {
		cfg.Database.Host = databaseHost
	}
	if databasePort, ok := intLookup("DATABASE_PORT"); ok {
		cfg.Database.Port = databasePort
	}
	if databaseUsername, ok := stringLookup("DATABASE_USERNAME"); ok {
		cfg.Database.Username = databaseUsername
	}
	if databasePassword, ok := stringLookup("DATABASE_PASSWORD"); ok {
		cfg.Database.Password = databasePassword
	}
	if databaseName, ok := stringLookup("DATABASE_NAME"); ok {
		cfg.Database.Name = databaseName
	}

	if localBackupDir, ok := stringLookup("BACKUP_DIR"); ok {
		cfg.LocalBackup.Dir = localBackupDir
	}
	if localBackupSchedule, ok := stringsLookup("BACKUP_SCHEDULE"); ok {
		cfg.LocalBackup.Schedule = localBackupSchedule
	}
	if localBackupEnabled, ok := boolLookup("BACKUP_ENABLED"); ok {
		cfg.LocalBackup.Enabled = localBackupEnabled
	}
	if localBackupRetentionDays, ok := intLookup("RETENTION_DAYS"); ok {
		cfg.LocalBackup.Retention.RetentionDays = &localBackupRetentionDays
	}
	if localBackupLimit, ok := intLookup("BACKUP_LIMIT"); ok {
		cfg.LocalBackup.Retention.MaxBackups = &localBackupLimit
	}

	if notificationSuccessEnabled, ok := boolLookup("NOTIFICATION_SUCCESS_ENABLED"); ok {
		cfg.Notification.SuccessEnabled = notificationSuccessEnabled
	}
	if notificationErrorEnabled, ok := boolLookup("NOTIFICATION_ERROR_ENABLED"); ok {
		cfg.Notification.ErrorEnabled = notificationErrorEnabled
	}
	if notificationEmails, ok := stringsLookup("NOTIFICATION_EMAIL"); ok {
		cfg.Notification.Emails = notificationEmails
	}
	if notificationEmailFrom, ok := stringLookup("NOTIFICATION_EMAIL_FROM"); ok {
		cfg.Notification.EmailFrom = notificationEmailFrom
	}
	if smtpServer, ok := stringLookup("SMTP_SERVER"); ok {
		cfg.Notification.SMTPServer = smtpServer
	}
	if smtpPort, ok := intLookup("SMTP_PORT"); ok {
		cfg.Notification.SMTPPort = smtpPort
	}
	if smtpUser, ok := stringLookup("SMTP_USER"); ok {
		cfg.Notification.SMTPUser = smtpUser
	}
	if smtpPassword, ok := stringLookup("SMTP_PASSWORD"); ok {
		cfg.Notification.SMTPPassword = smtpPassword
	}
	if smtpAuthMethod, ok := stringLookup("SMTP_AUTH_METHOD"); ok {
		cfg.Notification.SMTPAuth = smtpAuthMethod
	}
	if smtpTls, ok := boolLookup("SMTP_TLS"); ok {
		cfg.Notification.SMTPTLS = smtpTls
	}
	if discordWebhookUrl, ok := stringLookup("DISCORD_WEBHOOK_URL"); ok {
		cfg.Notification.DiscordWebhookURL = discordWebhookUrl
	}
	if telegramBotToken, ok := stringLookup("TELEGRAM_BOT_TOKEN"); ok {
		cfg.Notification.TelegramBotToken = telegramBotToken
	}
	if telegramChatId, ok := stringLookup("TELEGRAM_CHAT_ID"); ok {
		cfg.Notification.TelegramChatID = telegramChatId
	}

	cfg.RemoteProviders = overrideProviders(cfg.RemoteProviders)

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

func overrideProviders(providers []RemoteProvider) []RemoteProvider {
	for name, schema := range providerSchemas {
		prov, ok := findProviderOrCreate(providers, name)
		loadEnvOverride(schema.Prefix, schema.Config, prov)

		if !ok {
			providers = append(providers, *prov)
		}

	}

	return providers
}

func findProviderOrCreate(providers []RemoteProvider, name string) (*RemoteProvider, bool) {
	for i := range providers {
		if providers[i].Name == name {
			return &providers[i], true
		}
	}
	return &RemoteProvider{
		Name:    name,
		Type:    name,
		Enabled: true,
		Config:  map[string]string{},
	}, false
}

func loadEnvOverride(prefix string, configMap map[string]ConfigField, remote *RemoteProvider) {

	if providerEnabled, ok := boolLookup(prefix + "ENABLED"); ok {
		remote.Enabled = providerEnabled
	}

	if providerPath, ok := stringLookup(prefix + "PATH"); ok {
		remote.Path = providerPath
	}
	if providerSchedules, ok := stringsLookup(prefix + "SCHEDULE"); ok {
		remote.Schedule = providerSchedules
	}
	if providerMaxVersions, ok := intLookup(prefix + "MAX_VERSIONS"); ok {
		remote.MaxVersions = providerMaxVersions
	}
	if providerTimeout, ok := intLookup(prefix + "TIMEOUT"); ok {
		remote.Timeout = providerTimeout
	}

	for envKey, configKey := range configMap {
		if value, ok := stringLookup(prefix + envKey); ok {
			if configKey.Base64 {
				remote.Config[configKey.Name] = utils.DecodeBase64(value)
				continue
			}

			remote.Config[configKey.Name] = value
		}
	}

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
			"token": utils.DecodeBase64(tokenBase64),
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
			"token": utils.DecodeBase64(tokenBase64),
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
			"service_account_credentials": utils.DecodeBase64(accountBase64),
			"project_number":              stringOrEmpty(prefix+"PROJECT_NUMBER", ""),
			"bucket_policy_only":          stringOrEmpty(prefix+"BUCKET_POLICY_ONLY", "false"),
			"location":                    stringOrEmpty(prefix+"LOCATION", ""),
			"storage_class":               stringOrEmpty(prefix+"STORAGE_CLASS", "STANDARD"),
		},
	}
}
