package retention

import "github.com/BrunoTulio/pgopher/internal/config"

type (
	FnOptions func(*Options)

	Options struct {
		Retention    config.RetentionConfig
		OutputDir    string
		DatabaseName string
	}
)

func WithConfig(config *config.Config) *Options {
	return &Options{
		Retention:    config.LocalBackup.Retention,
		OutputDir:    config.LocalBackup.Dir,
		DatabaseName: config.Database.Name,
	}
}

func WithRetention(maxBackups *int, retentionDays *int) FnOptions {
	return func(backupOptions *Options) {
		backupOptions.Retention.MaxBackups = maxBackups
		backupOptions.Retention.RetentionDays = retentionDays
	}
}

func WithOutputDir(dir string) FnOptions {
	return func(opts *Options) {
		opts.OutputDir = dir
	}
}

func WithDatabaseName(name string) FnOptions {
	return func(opts *Options) {
		opts.DatabaseName = name
	}
}

func (o *Options) HasRetention() bool {
	return o.Retention.HasMaxBackups() || o.Retention.HasRetentionDays()
}

func (o *Options) HasMaxBackups() bool {
	return o.Retention.HasMaxBackups()
}

func (o *Options) HasRetentionDays() bool {
	return o.Retention.HasRetentionDays()
}
