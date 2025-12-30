package remote

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/BrunoTulio/logr"
	"github.com/BrunoTulio/pgopher/internal/backup"
	"github.com/BrunoTulio/pgopher/internal/metadata"
	"github.com/BrunoTulio/pgopher/internal/utils"
	// Backends
	_ "github.com/rclone/rclone/backend/drive"
	_ "github.com/rclone/rclone/backend/dropbox"
	_ "github.com/rclone/rclone/backend/mega"
	_ "github.com/rclone/rclone/backend/s3"
)

var (
	rcloneInitOnce sync.Once
)

type (
	Provider struct {
		client *Client
		store  metadata.Store
		log    logr.Logger
	}

	BackupFile struct {
		Name    string
		Path    string
		ModTime time.Time
		Size    int64
	}
)

func NewProvider(store metadata.Store, log logr.Logger) (*Provider, error) {
	return NewProviderWithOptions(store, log)
}

func NewProviderWithOptions(
	store metadata.Store,
	log logr.Logger,
	opts ...FnOptions,
) (*Provider, error) {

	client, err := NewClientWithOptions(log, opts...)
	if err != nil {
		return nil, err
	}

	return &Provider{
		client: client,
		store:  store,
		log:    log,
	}, nil
}

func (p *Provider) Backup(ctx context.Context) error {

	log := p.log.WithMap(map[string]any{
		"operation": "remote_backup",
		"provider":  p.client.opt.Name,
		"type":      p.client.opt.Type,
	})

	log.Infof("☁️  Starting remote backup to %s...", p.client.opt.Name)
	startTime := time.Now()

	currentVersion, err := p.getNextVersion()
	if err != nil {
		return fmt.Errorf("failed to get current version: %w", err)
	}

	fileName := p.client.opt.GetRemoteFileName(currentVersion)
	tmpDir := os.TempDir()

	log.Infof("   Generating backup v%d: %s", currentVersion, fileName)

	localBackup := backup.NewWithFnOptions(p.log,
		backup.WithGenerateFileName(func() string {
			return fileName
		}),
		backup.WithOutputDir(tmpDir),
		backup.WithoutRetention(),
		backup.WithDatabase(p.client.opt.Database),
		backup.WithEncryptionKey(p.client.opt.EncryptionKey),
	)

	backupFile, err := localBackup.Run(ctx)
	if err != nil {
		return fmt.Errorf("backup generation failed: %w", err)
	}
	defer func() {
		_ = os.Remove(backupFile)
	}()

	log.Infof("   Uploading to %s...", p.client.opt.Name)

	obj, err := p.client.UploadFile(ctx, backupFile, fileName)
	if err != nil {
		return fmt.Errorf("upload failed: %w", err)
	}
	modTime := time.Now()
	duration := time.Since(startTime)
	remotePath := p.client.opt.RemotePathFor(fileName)

	lastBackup := metadata.LastBackup{
		ShortId:    utils.GenerateShortID(fileName, modTime),
		RemotePath: remotePath,
		UploadedAt: modTime,
		Size:       obj.Size(),
		Version:    currentVersion,
	}

	if err := p.store.SaveLastBackup(p.client.opt.Name, lastBackup); err != nil {
		log.Warnf("Failed to save backup metadata: %v", err)
	}

	log.Infof("✅ Remote backup v%d to %s completed in %s",
		currentVersion, p.client.opt.Name, duration.Round(time.Second))

	return nil
}

func (p *Provider) saveLastBackup(lastBackup metadata.LastBackup) {
	err := p.store.SaveLastBackup(p.client.opt.Name, lastBackup)
	if err != nil {
		p.log.Warnf("Failed to save backup metadata: %v", err)
	}
}

func (p *Provider) getNextVersion() (int, error) {
	if !p.client.opt.HasVersioning() {
		return 1, nil
	}

	lastVersion, err := p.store.GetLastBackup(p.client.opt.Name)
	if errors.Is(err, metadata.ErrNotFound) {
		return 1, nil
	}
	if err != nil {
		return 1, err
	}

	if lastVersion.Version >= p.client.opt.MaxVersions {
		return 1, nil
	}

	return lastVersion.Version + 1, nil

}
