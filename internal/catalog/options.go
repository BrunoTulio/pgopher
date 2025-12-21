package catalog

import "github.com/BrunoTulio/pgopher/internal/config"

type Options struct {
	database   config.DatabaseConfig
	providers  []config.RemoteProvider
	backupDir  string
	encryptKey string
}

func WithConfig(cfg *config.Config) func(opt *Options) {
	return func(opt *Options) {
		opt.database = cfg.Database
		opt.providers = cfg.RemoteProviders
		opt.backupDir = cfg.LocalBackup.Dir
		opt.encryptKey = cfg.EncryptionKey
	}
}
