package restore

import (
	"github.com/BrunoTulio/pgopher/internal/config"
)

type (
	FnOptions func(*Options)
	Options   struct {
		Database      config.DatabaseConfig
		Providers     []config.RemoteProvider
		EncryptionKey string
		Dir           string
	}
)

func WithConfig(
	cfg *config.Config,
) FnOptions {
	return func(options *Options) {
		options.Database = cfg.Database
		options.EncryptionKey = cfg.EncryptionKey
		options.Providers = cfg.RemoteProviders
		options.Dir = cfg.LocalBackup.Dir
	}
}

func WithEncryptionKey(key string) FnOptions {
	return func(opts *Options) {
		opts.EncryptionKey = key
	}
}

func (o *Options) IsEncryptEnabled() bool {
	return o.EncryptionKey != ""
}
