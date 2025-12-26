package backup

import (
	"fmt"
	"time"

	"github.com/BrunoTulio/pgopher/internal/config"
)

type (
	FnOptions func(*Options)
	Options   struct {
		GenerateFileName func() string // File name (empty = generates with timestamp)
		OutputDir        string        // Output directory (empty = uses config)
		Retention        config.RetentionConfig
		Database         config.DatabaseConfig
		EncryptionKey    string
	}
)

func WithConfig(
	cfg *config.Config,
) FnOptions {
	return func(opt *Options) {
		opt.GenerateFileName = func() string {
			timestamp := time.Now().Format("20060102-150405")
			return fmt.Sprintf("%s-%s.sql.gz", cfg.Database.Name, timestamp)
		}
		opt.OutputDir = cfg.LocalBackup.Dir
		opt.Retention = cfg.LocalBackup.Retention
		opt.Database = cfg.Database
		opt.EncryptionKey = cfg.EncryptionKey
	}
}

func WithDatabase(
	database config.DatabaseConfig,
) FnOptions {
	return func(options *Options) {
		options.Database = database
	}
}

func WithOutputDir(dir string) FnOptions {
	return func(opts *Options) {
		opts.OutputDir = dir
	}
}

func WithGenerateFileName(fn func() string) FnOptions {
	return func(backupOptions *Options) {
		backupOptions.GenerateFileName = fn
	}
}

func WithEncryptionKey(encryptionKey string) FnOptions {
	return func(backupOptions *Options) {
		backupOptions.EncryptionKey = encryptionKey
	}
}

func WithoutRetention() FnOptions {
	return func(opts *Options) {
		opts.Retention = config.RetentionConfig{
			MaxBackups:    nil,
			RetentionDays: nil,
		}
	}
}

func (o *Options) HasRetention() bool {
	return o.Retention.HasMaxBackups() || o.Retention.HasRetentionDays()
}

func (o *Options) IsEncryptEnabled() bool {
	return o.EncryptionKey != ""
}
