package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BrunoTulio/pgopher/internal/utils"
	"github.com/spf13/cobra"
)

var (
	initOutputPath string
)

// initCmd represents the init command
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Create default config.yaml",
	Long: `Creates a default configuration file with examples.

By default, creates config.yaml in the current directory.
Use -o to specify a custom output path.

Examples:
  # Create config.yaml in current directory
  pgopher init

  # Create in specific location
  pgopher init -o /etc/pgopher/config.yaml`,
	Run: runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)
	initCmd.Flags().StringVarP(&initOutputPath, "output", "o", "", "Output file path (default: ./config.yaml)")
}

func runInit(cmd *cobra.Command, args []string) {
	log.Info("Starting config initialization")

	outputPath := initOutputPath
	if outputPath == "" {
		outputPath = "./pgopher.yaml"
	}

	absPath, err := filepath.Abs(outputPath)
	if err != nil {
		log.Fatalf("Invalid output path %s, error %v", outputPath, err)
	}

	log.Debugf("Output path resolved %s", absPath)

	if utils.FileExists(absPath) {
		log.Warnf("Config file already exists %s", absPath)
		fmt.Printf("⚠️  Config file already exists: %s\n", absPath)

		if !utils.AskConfirmation("Overwrite? (y/N)") {
			log.Info("User cancelled")
			fmt.Println("❌ Cancelled")
			return
		}
	}
	dir := filepath.Dir(absPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Fatalf("Failed to create directory %s, error %v", dir, err)
	}
	log.Debugf("Directory created %s", dir)
	log.Debug("Using default config template")

	if err := os.WriteFile(absPath, []byte(configDefault), 0644); err != nil {
		log.Fatalf("Failed to write config file %s, error %v", absPath, err)
	}

	log.Infof("Config file created successfully %s", absPath)
	fmt.Printf("✅ Config file created: %s\n", absPath)

	printNextSteps(absPath)
}

func printNextSteps(configPath string) {
	fmt.Println("\n📝 Next steps:")
	fmt.Println("   1. Edit config if needed:")
	fmt.Printf("      nano %s\n", configPath)
	fmt.Println("\n   2. Generate OAuth tokens (if using remote providers):")
	fmt.Println("      pgopher auth dropbox")
	fmt.Println("      pgopher auth gdrive")
	fmt.Println("      pgopher auth mega -p yourpassword")
	fmt.Println("\n   3. Validate config:")
	fmt.Println("      pgopher config validate")
	fmt.Println("\n   4. Test backup:")
	fmt.Println("      pgopher backup run")
	fmt.Println("\n   5. Start daemon:")
	fmt.Println("      pgopher daemon")
}
