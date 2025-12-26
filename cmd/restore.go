package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/BrunoTulio/pgopher/internal/catalog"
	"github.com/BrunoTulio/pgopher/internal/database"
	"github.com/BrunoTulio/pgopher/internal/lock"
	"github.com/BrunoTulio/pgopher/internal/restore"
	"github.com/BrunoTulio/pgopher/internal/utils"
	"github.com/spf13/cobra"
)

var (
	restoreID       string
	restoreProvider string
	restoreLatest   bool
	restoreList     bool
	restoreForce    bool
)

// restoreCmd represents the restore command
var restoreCmd = &cobra.Command{
	Use:   "restore",
	Short: "Restore database from backup",
	Long: `Restore PostgreSQL database from a backup file.

Can restore from local file or fetch from remote provider.

Examples:
  # List available local backups
  pgopher restore --list

  # Restore by shortID
  pgopher restore --id abc123

  # Restore from specific local file
  pgopher restore --file /backups/mydb_20251226_083000.sql.gz

  # Restore latest local backup
  pgopher restore --latest

  # Restore from remote provider (latest)
  pgopher restore --provider s3 --latest

  # Force restore (skip connection checks)
  pgopher restore --id abc123 --force`,
	Run: runRestore,
}

func init() {
	rootCmd.AddCommand(restoreCmd)

	restoreCmd.Flags().StringVar(&restoreID, "id", "",
		"backup shortID from catalog")
	restoreCmd.Flags().StringVarP(&restoreProvider, "provider", "p", "local",
		"provider to restore from (local, s3, gcs, azure)")
	restoreCmd.Flags().BoolVar(&restoreLatest, "latest", false,
		"restore the most recent backup")
	restoreCmd.Flags().BoolVar(&restoreList, "list", false,
		"list available backups")
	restoreCmd.Flags().BoolVar(&restoreForce, "force", false,
		"force restore without confirmation")

}

func runRestore(cmd *cobra.Command, args []string) {
	log.Info("🔄 Starting restore process...")

	loadEnvIfExists()

	cfg, err := loadConfigOrFail()
	if err != nil {
		log.Fatalf("Error loading config: %v", err)
	}

	catalogService := catalog.NewWithOptions(log, catalog.WithConfig(cfg))

	if restoreList {
		if err := listAvailableBackups(catalogService, restoreProvider); err != nil {
			log.Fatalf("Failed to list backups: %v", err)
		}
		return
	}

	if err := validateRestoreFlags(); err != nil {
		log.Fatalf("Invalid flags: %v", err)
	}

	lockMgr := lock.New()
	log.Info("🔒 Acquiring restore lock...")
	if err := lockMgr.LockForRestore(); err != nil {
		log.Fatalf("Failed to acquire lock: %v", err)
	}

	defer func() {
		if err := lockMgr.UnlockForRestore(); err != nil {
			log.Errorf("Failed to release lock: %v", err)
		} else {
			log.Info("🔓 Lock released")
		}
	}()

	log.Infof("💾 Database: %s@%s:%d/%s",
		cfg.Database.Username,
		cfg.Database.Host,
		cfg.Database.Port,
		cfg.Database.Name)

	shortID, err := determineShortID(catalogService, restoreProvider)
	if err != nil {
		log.Fatalf("Failed to determine backup: %v", err)
	}

	log.Infof("📦 Selected backup shortID: %s", shortID)

	pgClient := database.NewClient(&cfg.Database)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	log.Info("Testing database connection...")
	if err := pgClient.TestConnection(ctx); err != nil {
		log.Fatalf("Database connection failed: %v", err)
	}
	log.Info("✅ Database connection successful")

	if !restoreForce {
		if !checkAndConfirmRestore(ctx, pgClient) {
			log.Info("Restore cancelled by user")
			return
		}
	} else {
		log.Warn("⚠️  Force mode enabled, skipping safety checks")
	}

	restoreService := restore.NewWithOpts(catalogService, log, restore.WithConfig(cfg))

	if err := restoreService.Run(ctx, restoreProvider, shortID); err != nil {
		log.Fatalf("Restore failed: %v", err)
	}
	log.Info("✅ Restore completed successfully!")

}

func checkAndConfirmRestore(ctx context.Context, pgClient *database.Client) bool {
	countConnections, err := pgClient.CountConnections(ctx)

	if err != nil {
		log.Fatalf("Failed to count connections: %v", err)
	}

	if countConnections > 0 {
		log.Warnf("⚠️  WARNING: %d active connection(s) to database", countConnections)
		log.Warn("⚠️  These connections will be terminated during restore!")

		if err := showActiveConnections(pgClient, ctx); err != nil {
			log.Fatalf("Failed to show active connections: %v", err)
		}
	} else {
		log.Info("✅ No active connections found")
	}

	log.Warnf("⚠️  WARNING: This will REPLACE all data in database ")
	log.Warn("⚠️  All existing data will be LOST!")

	return utils.AskConfirmation("Type 'yes' to confirm restore")
}

func listAvailableBackups(catalog *catalog.Catalog, provider string) error {
	log.Info("📋 Available local backups:")

	backups, err := catalog.List(context.Background(), provider)
	if err != nil {
		return fmt.Errorf("failed to list backups: %w", err)
	}

	if len(backups) == 0 {
		log.Info("  (no backups found)")
		return nil
	}

	for i, b := range backups {
		log.Infof("  %d. %s", i+1, b.Name)
		log.Infof("     Size: %s", utils.FormatBytes(b.Size))
		log.Infof("     Created: %s", b.ModTime)
		log.Infof("     ShortID: %s", b.ShortID)
		if i < len(backups)-1 {
			fmt.Println()
		}
	}

	return nil
}

func validateRestoreFlags() error {
	if restoreList {
		return nil
	}

	count := 0

	if restoreID != "" {
		count++
	}
	if restoreLatest {
		count++
	}

	if count == 0 {
		return fmt.Errorf("specify one of: --file, --id, or --latest")
	}

	if count > 1 {
		return fmt.Errorf("cannot specify multiple restore options (--file, --id, --latest)")
	}

	return nil
}

func determineShortID(catalog *catalog.Catalog, provider string) (string, error) {
	if restoreID != "" {
		return restoreID, nil
	}

	ctx := context.Background()
	backups, err := catalog.List(ctx, provider)
	if err != nil {
		return "", fmt.Errorf("failed to list backups: %w", err)
	}

	if len(backups) == 0 {
		return "", fmt.Errorf("no backups found")
	}

	if restoreLatest {
		latest := backups[0]
		log.Infof("🕐 Selected latest backup: %s", latest.Name)
		return latest.ShortID, nil
	}

	return "", fmt.Errorf("no backup selection criteria specified")
}

func showActiveConnections(pgClient *database.Client, ctx context.Context) error {
	connections, err := pgClient.ListConnections(ctx)
	if err != nil {
		return err
	}

	if len(connections) == 0 {
		return nil
	}

	log.Info("\nActive connections:")
	for _, conn := range connections {
		log.Infof("  • PID %d: %s@%s (%s) - %s",
			conn.PID,
			conn.Username,
			conn.ClientAddr,
			conn.AppName,
			conn.State)
	}
	fmt.Println()

	return nil
}
