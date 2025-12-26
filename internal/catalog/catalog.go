package catalog

import (
	"context"

	"fmt"
	"os"
	"strings"

	"github.com/BrunoTulio/logr"
	"github.com/BrunoTulio/pgopher/internal/config"
	"github.com/BrunoTulio/pgopher/internal/remote"
	"github.com/BrunoTulio/pgopher/internal/utils"
)

type (
	Catalog struct {
		opt *Options
		log logr.Logger
	}
	BackupFile struct {
		ShortID   string
		Name      string
		Size      int64
		ModTime   string
		Encrypted bool
	}
)

func New(log logr.Logger) *Catalog {
	return &Catalog{}
}

func NewWithOptions(log logr.Logger, opts ...func(*Options)) *Catalog {
	opt := &Options{}

	for _, o := range opts {
		o(opt)
	}

	return &Catalog{
		opt: opt,
		log: log,
	}
}

func (c *Catalog) List(ctx context.Context, providerName string) ([]BackupFile, error) {
	c.log.Infof("📂 Listing: %s", providerName)

	switch providerName {
	case "local":
		return c.listLocal()
	default:
		providerCfg, err := c.findProvider(providerName)
		if err != nil {
			return nil, fmt.Errorf("provider not found: %w", err)
		}
		return c.listRemote(ctx, providerCfg)
	}
}

func (c *Catalog) listLocal() ([]BackupFile, error) {
	entries, err := os.ReadDir(c.opt.backupDir)
	if err != nil {
		return nil, fmt.Errorf("read local: %w", err)
	}

	var files []BackupFile
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()

		if !utils.IsFileBackup(name) {
			continue
		}

		info, _ := entry.Info()
		modTime := info.ModTime()

		files = append(files, BackupFile{
			ShortID:   utils.GenerateShortID(entry.Name(), modTime),
			Name:      entry.Name(),
			Size:      info.Size(),
			ModTime:   utils.FormatTime(modTime),
			Encrypted: strings.HasSuffix(entry.Name(), ".age"),
		})
	}
	return files, nil
}

func (c *Catalog) listRemote(ctx context.Context, provider config.RemoteProvider) ([]BackupFile, error) {

	fsys, err := remote.NewProviderWithOptions(c.log, remote.WithOptions(provider, c.opt.database,
		c.opt.encryptKey))
	if err != nil {
		return nil, fmt.Errorf("remote fs: %w", err)
	}

	entries, err := fsys.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list remote: %w", err)
	}

	files := make([]BackupFile, 0, len(entries))
	for _, entry := range entries {

		files = append(files, BackupFile{
			ShortID:   utils.GenerateShortID(entry.Name, entry.ModTime),
			Name:      entry.Name,
			Size:      entry.Size,
			ModTime:   utils.FormatTime(entry.ModTime),
			Encrypted: strings.HasSuffix(entry.Name, ".age"),
		})
	}
	return files, nil
}

func (c *Catalog) findProvider(name string) (config.RemoteProvider, error) {
	for _, p := range c.opt.providers {
		if p.Name == name && p.Enabled {
			return p, nil
		}
	}
	return config.RemoteProvider{}, fmt.Errorf("provider %s not found", name)
}
