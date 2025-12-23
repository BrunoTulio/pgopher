package cmd

import (
	"fmt"

	"github.com/BrunoTulio/pgopher/internal/version"
	"github.com/spf13/cobra"
)

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show version information",
	Long:  `Display version, build date, git commit and Go version`,
	Run: func(cmd *cobra.Command, args []string) {
		info := version.Get()

		fmt.Printf("pgopher version %s\n", info.Version)
		fmt.Printf("  Git commit:  %s\n", info.GitCommit)
		fmt.Printf("  Built:       %s\n", info.BuildDate)
		fmt.Printf("  Go version:  %s\n", info.GoVersion)
		fmt.Printf("  OS/Arch:     %s/%s\n", info.OS, info.Arch)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
