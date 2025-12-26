package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/BrunoTulio/pgopher/internal/backup"
	"github.com/BrunoTulio/pgopher/internal/config"
	"github.com/BrunoTulio/pgopher/internal/database"
	"github.com/BrunoTulio/pgopher/internal/lock"
	"github.com/BrunoTulio/pgopher/internal/remote"
	"github.com/spf13/cobra"
)

var (
	backupProvider string
	backupLocal    bool
	backupTimeout  int
)

// backupCmd represents the backup command
var backupCmd = &cobra.Command{
	Use:   "backup",
	Short: "Backup a manual PostgreSQL backup",
	Long: `Execute a one-time backup of the PostgreSQL database.

By default, creates a local backup in the configured backup directory.
Use --provider to upload the backup to cloud storage (Dropbox, Google Drive, S3, etc.).

The backup process:
  1. Connects to PostgreSQL database
  2. Creates a compressed SQL dump (pg_dump)
  3. Optionally encrypts the backup file
  4. Saves locally and/or uploads to remote provider
  5. Applies retention policies (removes old backups)
  6. Sends notification on success or failure

Examples:
  # Local backup only
  pgopher backup

  # Backup to Dropbox
  pgopher backup --provider dropbox

  # Backup to Google Drive
  pgopher backup --provider gdrive

  # Backup to S3
  pgopher backup --provider s3

  # Local + remote backup
  pgopher backup --local --provider dropbox

  # Custom timeout (default: 30 minutes)
  pgopher backup --timeout 60

  # With custom config file
  pgopher backup --config /etc/pgopher/config.yaml --provider gdrive`,
	Run: runBackup,
}

func init() {
	rootCmd.AddCommand(backupCmd)

	backupCmd.Flags().StringVarP(&backupProvider, "provider", "p", "",
		"remote provider (dropbox, gdrive, s3, mega, gcs)")
	backupCmd.Flags().BoolVarP(&backupLocal, "local", "l", false,
		"keep local backup (default: false when using --provider)")
	backupCmd.Flags().IntVarP(&backupTimeout, "timeout", "t", 30,
		"timeout in minutes")
}

func runBackup(cmd *cobra.Command, args []string) {
	log.Info("🚀 Starting manual backup...")
	loadEnvIfExists()
	cfg, err := loadConfigOrFail()

	if err != nil {
		log.Fatalf("Error loading config: %v", err)
	}

	log.Infof("💾 Database: %s@%s:%d/%s",
		cfg.Database.Username,
		cfg.Database.Host,
		cfg.Database.Port,
		cfg.Database.Name)
	log.Infof("📁 Backup directory: %s", cfg.LocalBackup.Dir)

	pgClient := database.NewClient(&cfg.Database)
	testCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	log.Info("Testing database connection...")
	if err := pgClient.TestConnection(testCtx); err != nil {
		log.Fatalf("❌ Database connection failed: %v", err)
	}
	log.Info("✅ Database connection successful")

	remoteCfg := checkProvider(cfg)
	lockMgr := lock.New()
	backupService := backup.NewWithFnOptions(log, backup.WithConfig(cfg))
	notifierService := createNotifierService(cfg)

	timeoutDuration := time.Duration(backupTimeout) * time.Minute
	ctx, cancel := context.WithTimeout(context.Background(), timeoutDuration)
	defer cancel()

	if backupLocal {
		if lockMgr.IsRestoreRunning() {
			log.Warn("⚠️  Restore in progress, skipping local backup")
			return
		}

		backupFile, err := backupService.Run(ctx)
		if err != nil {
			go func() {
				_ = notifierService.Error(context.Background(), fmt.Sprintf("Backup failed: %v", err))
			}()
			log.Fatalf("backup failed: %v", err)
		}
		log.Infof("✅ Local backup saved: %s", backupFile)

		go func() {
			_ = notifierService.Error(context.Background(), fmt.Sprintf(" Local backup saved: %s", backupFile))
		}()
	}

	if remoteCfg != nil {
		if lockMgr.IsRestoreRunning() {
			log.Warn("⚠️  Restore in progress, skipping local backup")
			return
		}

		log.Infof("☁️  Uploading to: %s (%s)", remoteCfg.Name, remoteCfg.Type)
		log.Infof("📍 Remote path: %s", remoteCfg.Path)

		provider, err := remote.NewProviderWithOptions( /*restoreService,*/ log,
			remote.WithOptions(*remoteCfg, cfg.Database, cfg.EncryptionKey),
		)
		if err != nil {
			log.Fatalf("❌ Failed to initialize provider: %v", err)
		}

		if err := provider.Backup(ctx); err != nil {
			log.Errorf("❌ Upload to %s failed: %v", remoteCfg.Name, err)
			go func() {
				_ = notifierService.Error(context.Background(), fmt.Sprintf("Upload to %s failed: %v", remoteCfg.Name, err))
			}()
			log.Fatalf("remote upload failed: %v", err)
		}

		log.Infof("✅ Uploaded to %s successfully!", remoteCfg.Name)
		go func() {
			_ = notifierService.Success(context.Background(), fmt.Sprintf("Backup uploaded to %s", remoteCfg.Name))
		}()
	}

}

func checkProvider(cfg *config.Config) *config.RemoteProvider {
	if backupProvider != "" {
		log.Infof("✅ Remote backup  initialize")
		providerCfg, err := findProvider(cfg, backupProvider)

		if err != nil {
			log.Fatalf("❌ Provider '%s' not found or not enabled", backupProvider)
		}

		return providerCfg
	}

	return nil
}
