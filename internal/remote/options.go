package remote

import (
	"fmt"
	"os"
	"strings"

	"github.com/BrunoTulio/pgopher/internal/config"
)

type (
	FnOptions func(*Options)

	Options struct {
		Name          string
		Type          string // s3, drive, dropbox, mega
		Path          string // prefixo remoto: bucket/pasta/base
		MaxVersions   int    // 0 = sobrescreve, >0 = rotaciona versões
		Config        map[string]string
		Database      config.DatabaseConfig
		EncryptionKey string
	}
)

func WithOptions(cfg config.RemoteProvider, database config.DatabaseConfig, encryptionKey string) FnOptions {
	return func(opt *Options) {
		opt.Name = cfg.Name
		opt.Type = cfg.Type
		opt.Path = cfg.Path
		opt.MaxVersions = cfg.MaxVersions
		opt.Config = cfg.Config
		opt.Database = database
		opt.EncryptionKey = encryptionKey

	}
}

func WithMaxVersions(maxVersions int) FnOptions {
	return func(opts *Options) {
		opts.MaxVersions = maxVersions
	}
}

func WithPath(path string) FnOptions {
	return func(opts *Options) {
		opts.Path = path
	}
}

func (o *Options) HasVersioning() bool {
	return o.MaxVersions > 0
}

// GetRemoteFileName gera nome do arquivo baseado na estratégia
func (o *Options) GetRemoteFileName(currentVersion int) string {
	ext := ".sql.gz"
	if o.EncryptionKey != "" {
		ext += ".age"
	}

	if o.HasVersioning() {
		return fmt.Sprintf("%s-v%d%s", o.Database.Name, currentVersion, ext)
	}

	return fmt.Sprintf("%s%s", o.Database.Name, ext)
}

func (o *Options) GetRcloneRemotePath() string {
	return fmt.Sprintf("%s:%s", o.Name, o.Path)
}

func (o *Options) RemotePathFor(fileName string) string {
	if o.Path == "" {
		return fileName
	}
	return fmt.Sprintf("%s/%s", o.Path, fileName)
}

func (o *Options) CleanupEnv() {
	envPrefix := o.EnvPrefix()

	_ = os.Unsetenv(envPrefix + "TYPE")
	for k := range o.Config {
		_ = os.Unsetenv(envPrefix + strings.ToUpper(k))
	}
}

func (o *Options) EnvPrefix() string {
	return fmt.Sprintf("RCLONE_CONFIG_%s_", strings.ToUpper(o.Name))

}

func (o *Options) SetupEnv() error {
	envPrefix := o.EnvPrefix()

	if err := os.Setenv(envPrefix+"TYPE", o.Type); err != nil {
		o.CleanupEnv() // 👈 LIMPA se erro

		return fmt.Errorf("set TYPE env: %w", err)
	}

	for k, v := range o.Config {
		envKey := envPrefix + strings.ToUpper(k)
		if err := os.Setenv(envKey, v); err != nil {
			o.CleanupEnv() // 👈 LIMPA se erro

			return fmt.Errorf("set %s env: %w", envKey, err)
		}
	}

	return nil

}
