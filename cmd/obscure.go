package cmd

import (
	"fmt"

	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/spf13/cobra"
)

// obscureCmd represents the obscure command
var obscureCmd = &cobra.Command{
	Use:   "obscure [plaintext]",
	Short: "Encrypt a password for safe storage in config files",
	Long: `Encrypt passwords and secrets using AES-256 encryption.

This command encrypts plaintext passwords (like Mega password, SMTP credentials)
into an encrypted string that can be safely stored in config.yaml or committed
to version control.

The encrypted value uses the obscureKey set at build time, providing basic
protection against accidental exposure. This is suitable for configuration files
but should not be considered fully secure encryption.

Examples:
  # Encrypt Mega password
  pgopher obscure "my-mega-password"
  
  # Encrypt SMTP password
  pgopher obscure "smtp-secret-123"
  
  # Use the output in config.yaml
  providers:
    - name: "mega"
      config:
        user: "me@mega.nz"
        pass: "XXX:4Yp8m2qK8nJ5vL9wX..."`,
	Args: cobra.ExactArgs(1),
	Run:  runObscure,
}

func init() {
	rootCmd.AddCommand(obscureCmd)
}

func runObscure(cmd *cobra.Command, args []string) {
	password := args[0]

	obscured := obscure.MustObscure(password)
	fmt.Printf("🔒 Obscured:  %s\n\n", obscured)
	fmt.Println("📋 Add to config.yaml:")
	fmt.Printf("  pass: \"%s\"\n", obscured)
}
