package cmd

import (
	"os"

	"github.com/BrunoTulio/logr"
	"github.com/BrunoTulio/logr/adapters/zap.v1"
	"github.com/spf13/cobra"
)

var (
	log     logr.Logger
	cfgFile string

	configDefault = `# =============================================================================
# PGOPHER - PostgreSQL Backup Configuration
# =============================================================================

server:
  addr: ":8080"

timezone: "" #Ex: America/Sao_Paulo, UTC, by default UTC

database:
  host: "localhost"
  port: 5432
  username: ""
  password: ""
  name: ""

local:
  dir: "/data/backups"
  schedule: 
    - "02:00"
    - "14:00"
  retention:
    # retention_days: 30
    # max_backups: 10
  enabled: true

providers:
  - name: "s3"
    type: "s3"
    enabled: true
    schedule: 
      - "02:00"
      - "14:00"
    path: "backups/db" #bucket or bucket/folder
    maxVersions: 5
    timeout: 300 #seconds
    config:
      provider: "s3"
      access_key_id: ""
      secret_access_key: ""
      region: ""
      # endpoint: ""
      # acl: ""
      # force_path_style: ""
      # no_check_bucket: ""

  - name: "drive"
    type: "drive"
    enabled: false
    schedule: 
      - "02:00"
      - "14:00"
    path: "backups" #folder
    maxVersions: 0
    timeout: 600 #seconds
    config:
      token: "" #json format base64
      scope: "drive"

  - name: "dropbox"
    type: "dropbox"
    enabled: false
    schedule: 
      - "02:00"
      - "14:00"
    path: "backups" #folder
    maxVersions: 0
    timeout: 600 #seconds
    config:
      token: "" #json format base64

  - name: "mega"
    type: "mega"
    enabled: false
    schedule: 
      - "02:00"
      - "14:00"
    path: "backups" #folder
    maxVersions: 0
    timeout: 600 #seconds
    config:
      user: "" 
      pass: "" #obscure password

  - name: "gcs"
    type: "gcs"
    enabled: false
    schedule: 
      - "02:00"
      - "14:00"
    path: "backups" #folder
    maxVersions: 0
    timeout: 600 #seconds
    config:
      service_account_credentials: ""  #json format base64
      project_number: "" 
      # bucket_policy_only: ""
      # location: ""
      # storage_class: ""

notification:
  success_enabled: true
  error_enabled: true
  emails:
    - "admin@example.com"
    - "ops@example.com"
  email_from: "backup@example.com"
  smtp_server: ""
  smtp_port: 587
  smtp_user: ""
  smtp_password: ""
  smtp_auth: "plain"
  smtp_tls: false
  discord_webhook_url: "" #https://discord.com/api/webhooks/...
  telegram_bot_token: "" 
  telegram_chat_id: ""

encryption_key: ""  #my-super-secret-key

run_on_startup: false
run_remote_on_startup: false
`
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "pgopher",
	Short: "Automated PostgreSQL backup and restore service",
	Long: `pgopher is a small daemon focused on safe, automated backups of PostgreSQL databases.
Designed to be simple to operate in production, with focus on:

- Scheduled backups via cron (ex: "0 2 * * *")
- Encryption of dumps before storage (AGE)
- Local and remote provider storage (S3, Dropbox, Google Drive, MEGA)
- Success/error notifications (Discord, Telegram, Email)
- Guided CLI restoration with confirmation before overwriting database

Examples:

  # Create default config.yaml
  pgopher init

  # Validate configuration
  pgopher config validate -c config.yaml

  # Run manual backup
  pgopher backup run

  # Restore specific backup
  pgopher restore run dropbox <shortID> prod_db

  # Start daemon (scheduler + HTTP API)
  pgopher daemon -c /etc/pgopher/config.yaml`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		log = zap.New(
			zap.WithConsole(true),
			zap.WithConsoleLevel("INFO"),
			zap.WithConsoleFormatter("TEXT"),
			zap.WithEnableCaller(false),
		)
	},
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is ./config.yaml)")

}
