package cmd

import (
	"context"
	"fmt"

	"github.com/BrunoTulio/pgopher/internal/catalog"
	"github.com/BrunoTulio/pgopher/internal/lock"
	"github.com/BrunoTulio/pgopher/internal/utils"
	"github.com/spf13/cobra"
)

var (
	restoreFile     string
	restoreProvider string
	restoreLatest   bool
	restoreList     bool
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

  # Restore from specific local file
  pgopher restore --file /backups/mydb_20251226_083000.sql.gz

  # Restore latest local backup
  pgopher restore --latest

  # Restore from Dropbox (latest)
  pgopher restore --provider dropbox --latest

  # Restore specific file from Google Drive
  pgopher restore --provider gdrive --file backups/mydb_20251226.sql.gz`,
	Run: runRestore,
}

func init() {
	rootCmd.AddCommand(restoreCmd)

	restoreCmd.Flags().StringVarP(&restoreFile, "file", "f", "",
		"backup file path to restore")
	restoreCmd.Flags().StringVarP(&restoreProvider, "provider", "p", "local",
		"remote provider to fetch from (dropbox, gdrive, s3)")
	restoreCmd.Flags().BoolVar(&restoreLatest, "latest", false,
		"restore the most recent backup")
	restoreCmd.Flags().BoolVar(&restoreList, "list", false,
		"list available backups")

}

func runRestore(cmd *cobra.Command, args []string) {
	log.Info("🔄 Starting restore process...")

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

	catalogService := catalog.NewWithOptions(log, catalog.WithConfig(cfg))
	lockMgr := lock.New()

	fmt.Println(lockMgr)

	if restoreList {
		if err := listAvailableBackups(catalogService, restoreProvider); err != nil {
			log.Fatalf("Failed to list backups: %v", err)
		}
		return
	}
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
