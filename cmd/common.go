package cmd

import (
	"fmt"
	"os"

	"github.com/BrunoTulio/pgopher/internal/config"
	"github.com/BrunoTulio/pgopher/internal/notify"
	"github.com/BrunoTulio/pgopher/internal/utils"
	"github.com/joho/godotenv"
)

func loadEnvIfExists() {
	envFile := ".env"

	if _, err := os.Stat(envFile); err != nil {
		return
	}

	if err := godotenv.Load(envFile); err != nil {
		log.Warnf("⚠️  Failed to load .env: %v", err)
		return
	}

	log.Info("🔧 Loaded .env file (development mode)")
}

func loadConfigOrFail() (*config.Config, error) {
	if cfgFile == "" {
		cfgFile = "./pgopher.yaml"
	}

	if utils.FileExists(cfgFile) {
		var err error
		cfg, err := config.LoadFromYAML(cfgFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load YAML config: %w", err)
		}
		utils.InitTimezone(cfg.MustLocation(), "2006-01-02 15:04:05")

		return cfg, nil
	}
	log.Info("📄 Config file not found, using environment variables")
	cfg, err := config.LoadFromEnv()
	if err != nil {
		return nil, fmt.Errorf("failed to load config from ENV: %w", err)
	}

	utils.InitTimezone(cfg.MustLocation(), "2006-01-02 15:04:05")

	return cfg, nil

}

func createNotifierService(cfg *config.Config) notify.Notifier {
	notifierService := notify.NewMultiNotifier(cfg.Notification.SuccessEnabled, cfg.Notification.ErrorEnabled, log)
	if cfg.IsNotifyMail() {
		notifierService.AddNotifier(notify.NewMail(
			cfg.Notification.SMTPServer,
			cfg.Notification.SMTPPort,
			cfg.Notification.SMTPUser,
			cfg.Notification.SMTPPassword,
			cfg.Notification.Emails,
			cfg.Notification.EmailFrom,
			cfg.Notification.SMTPAuth,
			cfg.Notification.SMTPTLS,
			log,
		))
	}

	if cfg.IsNotifyDiscord() {
		notifierService.AddNotifier(notify.NewDiscord(
			cfg.Notification.DiscordWebhookURL,
			log,
		))
	}

	if cfg.IsNotifyTelegram() {
		notifierService.AddNotifier(notify.NewTelegramNotifier(
			cfg.Notification.TelegramBotToken,
			cfg.Notification.TelegramChatID,
			log,
		))
	}

	return notifierService
}

func findProvider(cfg *config.Config, provider string) (*config.RemoteProvider, error) {

	for _, remoteProvider := range cfg.RemoteProviders {

		if remoteProvider.Name == provider {
			return &remoteProvider, nil
		}
	}

	return nil, fmt.Errorf("failed to find remote provider: %s", provider)

}
