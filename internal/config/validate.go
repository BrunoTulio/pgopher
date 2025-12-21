package config

import (
	"fmt"
	"net"
	"regexp"
	"strings"

	"github.com/BrunoTulio/logr"
)

// Validate validates the entire configuration
func (c *Config) Validate() error {
	if err := c.validateDatabase(); err != nil {
		return fmt.Errorf("database config: %w", err)
	}

	if err := c.validateTimezone(); err != nil {
		return fmt.Errorf("timezone config: %w", err)
	}

	if err := c.validateLocalBackup(); err != nil {
		return fmt.Errorf("local backup config: %w", err)
	}

	if err := c.validateRemoteProviders(); err != nil {
		return fmt.Errorf("remote providers config: %w", err)
	}

	if err := c.validateNotification(); err != nil {
		return fmt.Errorf("notify config: %w", err)
	}

	return nil
}

// validateDatabase validate database settings
func (c *Config) validateDatabase() error {
	db := c.Database

	if strings.TrimSpace(db.Host) == "" {
		return fmt.Errorf("DATABASE_HOST is required")
	}

	if !isValidHost(db.Host) {
		return fmt.Errorf("DATABASE_HOST has invalid format: %s", db.Host)
	}

	if db.Port < 1 || db.Port > 65535 {
		return fmt.Errorf("DATABASE_PORT must be between 1 and 65535, got %d", db.Port)
	}

	if strings.TrimSpace(db.Username) == "" {
		return fmt.Errorf("DATABASE_USERNAME is required")
	}

	if db.Password == "" {
		logr.Warn("DATABASE_PASSWORD is empty - this is insecure!")
	}

	if strings.TrimSpace(db.Name) == "" {
		return fmt.Errorf("DATABASE_NAME is required")
	}

	if !isValidPostgresName(db.Name) {
		return fmt.Errorf("DATABASE_NAME contains invalid characters: %s", db.Name)
	}

	return nil
}

// validateTimezone checks if the timezone is valid
func (c *Config) validateTimezone() error {
	if c.Timezone == "" {
		return fmt.Errorf("timezone cannot be empty")
	}

	_, err := c.GetLocation()
	if err != nil {
		return fmt.Errorf("invalid timezone '%s': %w", c.Timezone, err)
	}
	return nil
}

// validateLocalBackup validate local backup settings
func (c *Config) validateLocalBackup() error {
	lb := c.LocalBackup

	if strings.TrimSpace(lb.Dir) == "" {
		return fmt.Errorf("BACKUP_DIR is required")
	}

	// Validate path (cannot have dangerous characters)
	if strings.Contains(lb.Dir, "..") {
		return fmt.Errorf("BACKUP_DIR cannot contain '..'")
	}

	if len(lb.Schedule) > 0 {
		for _, schedule := range lb.Schedule {
			if !isValidTimeFormat(schedule) {
				return fmt.Errorf("invalid schedule format '%s', expected HH:MM", schedule)
			}
		}
	}

	// Validate retention - must have at least one strategy OR none
	hasRetentionDays := lb.Retention.HasRetentionDays()
	hasMaxBackups := lb.Retention.HasMaxBackups()

	if hasRetentionDays && hasMaxBackups {
		return fmt.Errorf("cannot use both RETENTION_DAYS and BACKUP_LIMIT simultaneously, choose one")
	}

	if hasRetentionDays {
		if *lb.Retention.RetentionDays < 1 {
			return fmt.Errorf("RETENTION_DAYS must be >= 1, got %d", *lb.Retention.RetentionDays)
		}
		if *lb.Retention.RetentionDays > 3650 { // ~10 anos
			logr.Warnf("RETENTION_DAYS is very high (%d days). Are you sure?", *lb.Retention.RetentionDays)
		}
	}

	if hasMaxBackups {
		if *lb.Retention.MaxBackups < 1 {
			return fmt.Errorf("BACKUP_LIMIT must be >= 1, got %d", *lb.Retention.MaxBackups)
		}
		if *lb.Retention.MaxBackups > 1000 {
			logr.Warnf("BACKUP_LIMIT is very high (%d backups). This may consume significant disk space.", *lb.Retention.MaxBackups)
		}
	}

	return nil
}

func (c *Config) validateRemoteProviders() error {
	if len(c.RemoteProviders) == 0 {
		logr.Info("No remote providers configured")
		return nil
	}

	providerNames := make(map[string]bool)

	for i, provider := range c.RemoteProviders {
		if providerNames[provider.Name] {
			return fmt.Errorf("provider[%d]: duplicate provider name '%s'", i, provider.Name)
		}
		providerNames[provider.Name] = true

		if strings.TrimSpace(provider.Name) == "" {
			return fmt.Errorf("provider[%d]: name is required", i)
		}

		if strings.TrimSpace(provider.Type) == "" {
			return fmt.Errorf("provider[%d] (%s): type is required", i, provider.Name)
		}

		if provider.Enabled && strings.TrimSpace(provider.Path) == "" {
			return fmt.Errorf("provider[%d] (%s): path is required when enabled", i, provider.Name)
		}

		// ✅ Validar configurações específicas de cada tipo
		if provider.Enabled {
			if err := validateProviderConfig(i, &provider); err != nil {
				return err
			}
		}

		if len(provider.Schedule) > 0 {
			for _, schedule := range provider.Schedule {
				if !isValidTimeFormat(schedule) {
					return fmt.Errorf("provider[%d] (%s): invalid schedule format '%s', expected HH:MM",
						i, provider.Name, schedule)
				}
			}
		} else if provider.Enabled {
			logr.Warnf("Provider '%s' is enabled but has no schedule configured", provider.Name)
		}

		if provider.ScheduleDay != nil {
			day := *provider.ScheduleDay
			if day < 0 || day > 6 {
				return fmt.Errorf("provider[%d] (%s): schedule_day must be between 0 (Sunday) and 6 (Saturday), got %d",
					i, provider.Name, day)
			}
		}

		if provider.MaxVersions < 0 {
			return fmt.Errorf("provider[%d] (%s): max_versions cannot be negative, got %d",
				i, provider.Name, provider.MaxVersions)
		}
		if provider.MaxVersions > 100 {
			logr.Warnf("Provider '%s' has max_versions=%d which is very high",
				provider.Name, provider.MaxVersions)
		}

		if provider.Timeout < 60 {
			return fmt.Errorf("provider[%d] (%s): timeout must be at least 60 seconds, got %d",
				i, provider.Name, provider.Timeout)
		}
		if provider.Timeout > 86400 { // 24 horas
			logr.Warnf("Provider '%s' has timeout=%ds (>24h). This seems excessive.",
				provider.Name, provider.Timeout)
		}

	}

	return nil
}

// ✅ validateProviderConfig valida configurações específicas de cada tipo de provider
func validateProviderConfig(index int, provider *RemoteProvider) error {
	switch strings.ToLower(provider.Type) {
	case "s3":
		return validateS3Config(index, provider)
	case "drive":
		return validateGDriveConfig(index, provider)
	case "dropbox":
		return validateDropboxConfig(index, provider)
	case "mega":
		return validateMegaConfig(index, provider)
	case "google cloud storage":
		return validateGCSConfig(index, provider)
	default:
		logr.Warnf("Provider[%d] (%s): unknown type '%s', skipping specific validation",
			index, provider.Name, provider.Type)
		return nil
	}
}

// ✅ validateS3Config valida configurações do S3
func validateS3Config(index int, provider *RemoteProvider) error {
	required := []string{"access_key_id", "secret_access_key", "region"}

	for _, field := range required {
		if val, ok := provider.Config[field]; !ok || strings.TrimSpace(val) == "" {
			return fmt.Errorf("provider[%d] (%s): S3 requires '%s' in config",
				index, provider.Name, field)
		}
	}

	// Validar provider
	validProviders := map[string]bool{
		"AWS": true, "Minio": true, "Wasabi": true, "DigitalOcean": true,
		"Ceph": true, "Cloudflare": true, "Alibaba": true, "Other": true,
	}
	if provider, ok := provider.Config["provider"]; ok {
		if !validProviders[provider] {
			logr.Warnf("Provider[%d] (%s): S3 provider '%s' may not be supported",
				index, provider, provider)
		}
	}

	// Validar endpoint se for Minio ou outro
	if prov := provider.Config["provider"]; prov == "Minio" || prov == "Other" {
		if endpoint, ok := provider.Config["endpoint"]; !ok || strings.TrimSpace(endpoint) == "" {
			return fmt.Errorf("provider[%d] (%s): S3 provider '%s' requires 'endpoint'",
				index, provider.Name, prov)
		}
	}

	// Validar region format
	if region := provider.Config["region"]; region != "" {
		regionRegex := regexp.MustCompile(`^[a-z]{2}-[a-z]+-\d+$`)
		if !regionRegex.MatchString(region) && region != "us-east-1" {
			logr.Warnf("Provider[%d] (%s): region '%s' has unusual format",
				index, provider.Name, region)
		}
	}

	return nil
}

// ✅ validateGDriveConfig valida configurações do Google Drive
func validateGDriveConfig(index int, provider *RemoteProvider) error {
	required := []string{"token"}

	for _, field := range required {
		if val, ok := provider.Config[field]; !ok || strings.TrimSpace(val) == "" {
			return fmt.Errorf("provider[%d] (%s): Google Drive requires '%s' in config",
				index, provider.Name, field)
		}
	}

	//Validar formato do client_id
	clientID := provider.Config["client_id"]
	if !strings.HasSuffix(clientID, ".apps.googleusercontent.com") {
		logr.Warnf("Provider[%d] (%s): client_id doesn't look like a Google OAuth client ID",
			index, provider.Name)
	}

	// Validar se token é JSON válido
	token := provider.Config["token"]
	if !strings.HasPrefix(token, "{") || !strings.HasSuffix(token, "}") {
		return fmt.Errorf("provider[%d] (%s): token must be a valid JSON object",
			index, provider.Name)
	}

	// Validar scope
	if scope, ok := provider.Config["scope"]; ok && scope != "" {
		validScopes := map[string]bool{
			"drive": true, "drive.file": true, "drive.readonly": true,
			"drive.metadata.readonly": true, "drive.appdata": true,
		}
		if !validScopes[scope] {
			logr.Warnf("Provider[%d] (%s): unusual scope '%s'",
				index, provider.Name, scope)
		}
	}

	return nil
}

// ✅ validateDropboxConfig valida configurações do Dropbox
func validateDropboxConfig(index int, provider *RemoteProvider) error {
	token, ok := provider.Config["token"]
	if !ok || strings.TrimSpace(token) == "" {
		return fmt.Errorf("provider[%d] (%s): Dropbox requires 'token' in config",
			index, provider.Name)
	}

	// Validar formato do token (Dropbox tokens geralmente começam com "sl.")
	if !strings.HasPrefix(token, "sl.") {
		logr.Warnf("Provider[%d] (%s): Dropbox token doesn't start with 'sl.' - may be invalid",
			index, provider.Name)
	}

	if len(token) < 50 {
		logr.Warnf("Provider[%d] (%s): Dropbox token seems too short",
			index, provider.Name)
	}

	return nil
}

// ✅ validateMegaConfig valida configurações do Mega
func validateMegaConfig(index int, provider *RemoteProvider) error {
	user, hasUser := provider.Config["user"]
	pass, hasPass := provider.Config["pass"]

	if !hasUser || strings.TrimSpace(user) == "" {
		return fmt.Errorf("provider[%d] (%s): Mega requires 'user' (email) in config",
			index, provider.Name)
	}

	if !hasPass || strings.TrimSpace(pass) == "" {
		return fmt.Errorf("provider[%d] (%s): Mega requires 'pass' (password) in config",
			index, provider.Name)
	}

	// Validar formato do email
	if !isValidEmail(user) {
		return fmt.Errorf("provider[%d] (%s): Mega 'user' must be a valid email address, got '%s'",
			index, provider.Name, user)
	}

	// Avisar sobre senha fraca
	if len(pass) < 8 {
		logr.Warnf("Provider[%d] (%s): Mega password is very short (< 8 chars)",
			index, provider.Name)
	}

	return nil
}

// ✅ validateGCSConfig valida configurações do Google Cloud Storage
func validateGCSConfig(index int, provider *RemoteProvider) error {
	credentials, ok := provider.Config["service_account_credentials"]
	if !ok || strings.TrimSpace(credentials) == "" {
		return fmt.Errorf("provider[%d] (%s): Google Cloud Storage requires 'service_account_credentials' (JSON) in config",
			index, provider.Name)
	}

	// Validar se é JSON válido
	if !strings.HasPrefix(credentials, "{") || !strings.HasSuffix(credentials, "}") {
		return fmt.Errorf("provider[%d] (%s): service_account_credentials must be a valid JSON object",
			index, provider.Name)
	}

	// Verificar campos essenciais no JSON
	requiredFields := []string{"type", "project_id", "private_key", "client_email"}
	for _, field := range requiredFields {
		if !strings.Contains(credentials, fmt.Sprintf(`"%s"`, field)) {
			logr.Warnf("Provider[%d] (%s): service_account_credentials may be missing '%s' field",
				index, provider.Name, field)
		}
	}

	// Validar type = service_account
	if !strings.Contains(credentials, `"type":"service_account"`) {
		return fmt.Errorf("provider[%d] (%s): service_account_credentials must have type=service_account",
			index, provider.Name)
	}

	// Validar storage class se fornecido
	if class, ok := provider.Config["storage_class"]; ok && class != "" {
		validClasses := map[string]bool{
			"STANDARD": true, "NEARLINE": true, "COLDLINE": true,
			"ARCHIVE": true, "MULTI_REGIONAL": true, "REGIONAL": true,
		}
		if !validClasses[strings.ToUpper(class)] {
			logr.Warnf("Provider[%d] (%s): unknown storage_class '%s'",
				index, provider.Name, class)
		}
	}

	return nil
}

// validateNotification validate notify settings
func (c *Config) validateNotification() error {
	notif := c.Notification

	if !notif.IsMails() && notif.DiscordWebhookURL == "" && notif.TelegramBotToken == "" {
		return nil
	}

	if notif.IsMails() {
		for _, email := range notif.Emails {
			if !isValidEmail(email) {
				return fmt.Errorf("NOTIFICATION_EMAIL has invalid format: %s", email)
			}

		}

		if notif.SMTPServer == "" {
			return fmt.Errorf("SMTP_SERVER is required when NOTIFICATION_EMAIL is set")
		}

		if notif.SMTPPort < 1 || notif.SMTPPort > 65535 {
			return fmt.Errorf("SMTP_PORT must be between 1 and 65535, got %d", notif.SMTPPort)
		}

		validSMTPPorts := map[int]bool{25: true, 465: true, 587: true, 2525: true}
		if !validSMTPPorts[notif.SMTPPort] {
			logr.Warnf("SMTP_PORT=%d is unusual. Common ports are 25, 465, 587, 2525", notif.SMTPPort)
		}

		if notif.EmailFrom != "" && !isValidEmail(notif.EmailFrom) {
			return fmt.Errorf("NOTIFICATION_EMAIL_FROM has invalid format: %s", notif.EmailFrom)
		}

		validAuthMethods := map[string]bool{"login": true, "plain": true, "cram-md5": true, "none": true}
		if !validAuthMethods[strings.ToLower(notif.SMTPAuth)] {
			return fmt.Errorf("SMTP_AUTH_METHOD must be one of: login, plain, cram-md5, got '%s'", notif.SMTPAuth)
		}
	}

	if notif.DiscordWebhookURL != "" {
		if !strings.HasPrefix(notif.DiscordWebhookURL, "http://") && !strings.HasPrefix(notif.DiscordWebhookURL, "https://") {
			return fmt.Errorf("WEBHOOK_URL must start with http:// or https://")
		}

	}

	return nil
}

// Validation helper functions

// isValidHost validates whether the host is valid (hostname or IP)
func isValidHost(host string) bool {
	if net.ParseIP(host) != nil {
		return true
	}
	hostnameRegex := regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?)*$`)
	return hostnameRegex.MatchString(host)
}

// isValidPostgresName validate PostgreSQL database name
func isValidPostgresName(name string) bool {
	if len(name) > 63 {
		return false
	}
	nameRegex := regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_\-]*$`)
	return nameRegex.MatchString(name)
}

// isValidTimeFormat validates HH:MM format
func isValidTimeFormat(timeStr string) bool {
	timeRegex := regexp.MustCompile(`^([0-1][0-9]|2[0-3]):([0-5][0-9])$`)
	return timeRegex.MatchString(timeStr)
}

// isValidEmail validate email format
func isValidEmail(email string) bool {
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
	return emailRegex.MatchString(email)
}
