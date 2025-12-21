package scheduler

import (
	"time"

	"github.com/BrunoTulio/pgopher/internal/config"
)

type Options struct {
	timezone      *time.Location
	Providers     []config.RemoteProvider
	Local         config.LocalBackupConfig
	Database      config.DatabaseConfig
	EncryptionKey string
}

func WithConfig(cfg *config.Config) func(*Options) {
	return func(o *Options) {
		o.timezone = cfg.MustLocation()
		o.Providers = cfg.RemoteProviders
		o.Local = cfg.LocalBackup
		o.Database = cfg.Database
		o.EncryptionKey = cfg.EncryptionKey
	}
}
