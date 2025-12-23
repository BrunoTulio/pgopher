package cmd

import (
	"fmt"
	"strings"

	"github.com/BrunoTulio/pgopher/internal/auth"
	"github.com/spf13/cobra"
)

// authCmd represents the auth command
var authCmd = &cobra.Command{
	Use:   "auth [provider]",
	Short: "Authenticate pgopher with Dropbox or Google Drive for remote backups",
	Long: `Start the OAuth2 flow to authenticate pgopher with a cloud provider.

This command opens an OAuth2 authorization flow (in your browser) and waits for
the callback on a local HTTP server. Once the flow is completed successfully,
an access/refresh token pair is obtained, encoded (base64) and stored so that
pgopher can upload backups to the selected provider without asking again.

Supported providers:
  - dropbox   (OAuth2 app created in Dropbox Developers)
  - gdrive    (Google Drive OAuth2 client from Google Cloud Console)

Examples:
  pgopher auth dropbox
  pgopher auth gdrive`,
	Args: cobra.ExactArgs(1),

	Run: runAuth,
}

func init() {
	rootCmd.AddCommand(authCmd)
}

func runAuth(cmd *cobra.Command, args []string) {
	provider := args[0]
	a := auth.New(log)

	token, err := a.Run(provider)

	if err != nil {
		log.Fatalf("authentication failed: %v", err)
	}

	fmt.Println("\n✅ Authentication successful!")
	fmt.Printf("📋 Provider: %s\n", provider)
	fmt.Println("\n🔐 Token (base64):")
	fmt.Println(token)

	fmt.Println("\n💡 Usage:")
	fmt.Println("1. Copy the token above")
	fmt.Println("2. Add to config.yaml:")
	fmt.Printf("   remote_providers:\n")
	fmt.Printf("     - name: \"%s\"\n", provider)
	fmt.Printf("       config:\n")
	fmt.Printf("         token: \"%s\"\n", token)
	fmt.Println("\nOR set environment variable:")
	fmt.Printf("   export REMOTE_%s_TOKEN=\"%s\"\n",
		strings.ToUpper(provider), token)

}
