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
	"github.com/BrunoTulio/pgopher/internal/lock"
	"github.com/BrunoTulio/pgopher/internal/remote"
	"github.com/BrunoTulio/pgopher/internal/scheduler"
	"github.com/spf13/cobra"
)

const (
	readTimeout  = 10 * time.Second
	idleTimeout  = 1 * time.Second
	writeTimeout = 10 * time.Second
)

// daemonCmd represents the daemon command
var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Backup pgopher as a background service",
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

  # Backup as systemd service
  systemctl start pgopher

  # Backup with Docker
  docker run -d pgopher daemon`,
	Run: runDaemon,
}

func init() {
	rootCmd.AddCommand(daemonCmd)
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
	lockMgr := lock.New()
	backupService := backup.NewWithFnOptions(log, backup.WithConfig(cfg))
	catalogService := catalog.NewWithOptions(log, catalog.WithConfig(cfg))
	notifierService := createNotifierService(cfg)

	if cfg.RunOnStartup {
		if lockMgr.IsRestoreRunning() {
			log.Warn("⚠️  Restore in progress, skipping scheduled local backup")
		} else {
			log.Info("Running initial backup...")
			backupFile, err := runOnStartBackupLocal(backupService)
			if err != nil {
				log.Errorf("Initial backup failed: %v", err)
			} else {
				log.Infof("Initial backup saved: %s", backupFile)
			}
		}
	}

	if cfg.RunRemoteOnStartup {

		log.Info("Running remote backup...")

		for _, providerCfg := range cfg.RemoteProviders {
			if !providerCfg.Enabled {
				continue
			}

			if lockMgr.IsRestoreRunning() {
				log.Warnf("⚠️  Restore in progress, skipping scheduled remote provider %s backup", providerCfg.Name)
				continue
			}

			log.Infof("📦 Initializing provider: %s (%s)", providerCfg.Name, providerCfg.Type)

			provider, err := remote.NewProviderWithOptions(log,
				remote.WithOptions(providerCfg, cfg.Database, cfg.EncryptionKey),
			)
			if err != nil {
				log.Errorf("Failed to create provider %s: %v", providerCfg.Name, err)

				go func(name string, err error) {
					_ = notifierService.Error(ctx, fmt.Sprintf("Failed to create provider %s: %v", name, err))
				}(providerCfg.Name, err)
				continue
			}

			if err := runOnStartBackupRemote(provider, providerCfg); err != nil {
				log.Errorf("Initializing provider failed: %v", err)
				go func(name string, err error) {
					_ = notifierService.Error(ctx, fmt.Sprintf("Initializing provider %s failed: %v", name, err))
				}(providerCfg.Name, err)
				continue
			}

			log.Infof("✅ Backup to %s completed!", providerCfg.Name)
			go func(name string) {
				_ = notifierService.Success(ctx, fmt.Sprintf("✅ Backup to %s completed!", name))
			}(providerCfg.Name)
		}
	}

	sched := scheduler.NewWithOptions(
		backupService,
		notifierService,
		lockMgr,
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
		if err := s.ListenAndServe(); err != nil {
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

func runOnStartBackupLocal(backupService *backup.Local) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	return backupService.Run(ctx)
}

func runOnStartBackupRemote(provider *remote.Provider, providerCfg config.RemoteProvider) error {

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Hour)
	defer cancel()

	if err := provider.Backup(ctx); err != nil {
		return fmt.Errorf("backup to %s failed: %v", providerCfg.Name, err)
	}

	return nil
}
