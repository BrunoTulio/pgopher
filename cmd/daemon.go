package cmd

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/BrunoTulio/pgopher/internal/backup"
	"github.com/BrunoTulio/pgopher/internal/catalog"
	"github.com/BrunoTulio/pgopher/internal/config"
	"github.com/BrunoTulio/pgopher/internal/database"
	apphttp "github.com/BrunoTulio/pgopher/internal/http"
	"github.com/BrunoTulio/pgopher/internal/notify"
	"github.com/BrunoTulio/pgopher/internal/remote"
	"github.com/BrunoTulio/pgopher/internal/restore"
	"github.com/BrunoTulio/pgopher/internal/scheduler"
	"github.com/BrunoTulio/pgopher/internal/utils"
	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
)

var (
	cfgFile string
)

const (
	readTimeout  = 10 * time.Second
	idleTimeout  = 1 * time.Second
	writeTimeout = 10 * time.Second
)

// daemonCmd represents the daemon command
var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Run pgopher as a background service",
	Long: `Start pgopher in daemon mode to continuously run scheduled backups.

This command starts a long-running process that:
  - Schedules and executes backups based on config.yaml
  - Runs HTTP server for health checks and metrics (optional)
  - Handles graceful shutdown on SIGTERM/SIGINT
  - Optionally runs initial backup on startup

The daemon will stay running until stopped with Ctrl+C or kill signal.

Examples:
  # Start daemon with default config
  pgopher daemon

  # Start with custom config
  pgopher daemon --config /path/to/config.yaml

  # Run as systemd service
  systemctl start pgopher

  # Run with Docker
  docker run -d pgopher daemon`,
	Run: runDaemon,
}

func init() {
	rootCmd.AddCommand(daemonCmd)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is ./config.yaml)")
}

func runDaemon(cmd *cobra.Command, args []string) {
	log.Info("🦫 Starting pgopher - PostgreSQL Backup System")

	loadEnvIfExists()

	cfg, err := loadConfigOrFail()
	if err != nil {
		log.Fatalf("Error loading config: %v", err)
	}

	pgClient := database.NewClient(&cfg.Database)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	log.Info("Testing database connection...")
	if err := pgClient.TestConnection(ctx); err != nil {
		log.Fatalf("Database connection failed: %v", err)
	}
	log.Info("✅ Database connection successful")
	backupService := backup.NewWithFnOptions(log, backup.WithConfig(cfg))
	catalogService := catalog.NewWithOptions(log, catalog.WithConfig(cfg))
	restoreService := restore.NewWithOpts(catalogService, log, restore.WithConfig(cfg))
	notifierService := createNotifierService(cfg)

	if cfg.RunOnStartup {
		log.Info("Running initial backup...")
		backupFile, err := runOnStartBackupLocal(backupService)
		if err != nil {
			log.Errorf("Initial backup failed: %v", err)
		} else {
			log.Infof("Initial backup saved: %s", backupFile)
		}
	}

	if cfg.RunRemoteOnStartup {
		log.Info("Running remote backup...")

		for _, providerCfg := range cfg.RemoteProviders {
			if !providerCfg.Enabled {
				continue
			}

			log.Infof("📦 Initializing provider: %s (%s)", providerCfg.Name, providerCfg.Type)

			provider, err := remote.NewProviderWithOptions(restoreService, log,
				remote.WithOptions(providerCfg, cfg.Database, cfg.EncryptionKey),
			)
			if err != nil {
				log.Errorf("Failed to create provider %s: %v", providerCfg.Name, err)

				go func() {
					_ = notifierService.Error(ctx, fmt.Sprintf("Failed to create provider %s: %v", providerCfg.Name, err))
				}()
				continue
			}

			if err := runOnStartBackupRemote(provider, providerCfg); err != nil {
				log.Errorf("Initializing provider failed: %v", err)

				go func() {
					_ = notifierService.Error(ctx, fmt.Sprintf("Initializing provider failed: %v", err))
				}()
				continue
			}

			log.Infof("✅ Backup to %s completed!", providerCfg.Name)
			go func() {
				_ = notifierService.Success(ctx, fmt.Sprintf("✅ Backup to %s completed!", providerCfg.Name))
			}()
		}
	}

	sched := scheduler.NewWithOptions(
		backupService,
		notifierService,
		restoreService,
		log,
		scheduler.WithConfig(cfg),
	)

	if err := sched.Start(); err != nil {
		log.Fatalf("Failed to start scheduler: %v", err)
	}

	s := http.Server{
		ReadTimeout:  readTimeout,
		IdleTimeout:  idleTimeout,
		WriteTimeout: writeTimeout,
		Addr:         cfg.Server.Addr,
		Handler:      apphttp.New(cfg, catalogService, sched, log),
	}

	go func() {
		log.Infof("🌐 HTTP server on %s", cfg.Server.Addr)
		if err := s.ListenAndServe(); err != nil { // 👈 DIRETO!
			log.Fatalf("HTTP failed: %v", err)
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	log.Info("pgopher is running. Press Ctrl+C to stop.")
	<-sigChan

	log.Info("Shutting down gracefully...")
	sched.Stop()
	log.Info("✅ Shutdown complete")

}

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

		return cfg, nil
	}
	log.Info("📄 Config file not found, using environment variables")
	cfg, err := config.LoadFromEnv()
	if err != nil {
		return nil, fmt.Errorf("failed to load config from ENV: %w", err)
	}

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

func runOnStartBackupLocal(backupService *backup.Local) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	return backupService.Run(ctx)
}

func runOnStartBackupRemote(provider *remote.Provider, providerCfg config.RemoteProvider) error {

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Hour)
	defer cancel()

	if err := provider.Run(ctx); err != nil {
		return fmt.Errorf("backup to %s failed: %v", providerCfg.Name, err)
	}

	return nil
}
