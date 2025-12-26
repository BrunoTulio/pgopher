package restore

import (
	"bufio"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/BrunoTulio/logr"
	"github.com/BrunoTulio/pgopher/internal/catalog"
	"github.com/BrunoTulio/pgopher/internal/config"
	"github.com/BrunoTulio/pgopher/internal/encoder"
	"github.com/BrunoTulio/pgopher/internal/notify"
	"github.com/BrunoTulio/pgopher/internal/remote"
)

type Restore struct {
	log      logr.Logger
	opt      *Options
	catSvr   *catalog.Catalog
	notifier notify.Notifier
}

func New(catSvr *catalog.Catalog, log logr.Logger) *Restore {
	return NewWithOpts(catSvr, log)
}

func NewWithOpts(catSvr *catalog.Catalog, log logr.Logger, opts ...FnOptions) *Restore {
	opt := &Options{}

	for _, o := range opts {
		o(opt)
	}
	return &Restore{
		opt:    opt,
		log:    log,
		catSvr: catSvr,
	}
}

func (r *Restore) Run(ctx context.Context, providerName, shortID string) error {

	files, err := r.catSvr.List(ctx, providerName)
	if err != nil {
		return fmt.Errorf("list catalog: %w", err)
	}

	var ff catalog.BackupFile
	for _, file := range files {
		if file.ShortID == shortID {
			ff = file
			break
		}
	}

	if ff.ShortID == "" {
		return fmt.Errorf("backup %s not found in %s", shortID, providerName)
	}

	backupPath := ff.Path
	var cleanup = func() {}

	if providerName != "local" {
		backupPath, cleanup, err = r.remotePath(ctx, providerName, ff)
		if err != nil {
			return err
		}
	}
	defer cleanup()
	backupFile, err := os.Open(backupPath)

	if err != nil {
		return fmt.Errorf("failed to open backup: %w", err)
	}
	defer func() {
		_ = backupFile.Close()
	}()

	gzReader, err := r.toReader(backupFile, backupPath)
	if err != nil {
		return err
	}
	defer func() {
		_ = gzReader.Close()
	}()

	err = r.executePgRestore(ctx, gzReader)

	if err != nil {
		return fmt.Errorf("failed to restore backup: %w", err)
	}

	return nil
}

func (r *Restore) toReader(backupFile *os.File, backupPath string) (io.ReadCloser, error) {
	var reader io.Reader = backupFile

	if strings.HasSuffix(backupPath, ".age") {
		if !r.opt.IsEncryptEnabled() {
			return nil, fmt.Errorf("backup is encrypted but no encryption key configured")
		}

		r.log.Info("🔐 Decrypting backup (streaming)...")

		enc, err := encoder.NewEncryptor(r.opt.EncryptionKey)
		if err != nil {
			return nil, fmt.Errorf("failed to create encryptor: %w", err)
		}

		decryptReader, err := enc.DecryptReader(backupFile) // ← Decripta o arquivo
		if err != nil {
			return nil, fmt.Errorf("decryption failed: %w", err)
		}

		reader = decryptReader
		r.log.Info("✅ Decryption completed")
	}

	r.log.Info("📦 Decompressing (streaming)...")
	gzReader, err := gzip.NewReader(reader) // ← Descomprime o resultado
	if err != nil {
		return nil, fmt.Errorf("failed to create gzip reader: %w", err)
	}

	if strings.HasSuffix(backupPath, ".age") {
		return struct {
			io.Reader
			io.Closer
		}{gzReader, gzReader}, nil
	}

	r.log.Info("✅ Decompression completed")
	return gzReader, nil
}

func (r *Restore) remotePath(ctx context.Context, providerName string, ff catalog.BackupFile) (string, func(), error) {
	var remoteProvider config.RemoteProvider

	for _, provider := range r.opt.Providers {
		if provider.Name == providerName {
			remoteProvider = provider
			break
		}
	}

	if remoteProvider.Name == "" {
		return "", nil, fmt.Errorf("provider %s not found in %s", providerName, providerName)
	}

	provider, err := remote.NewProviderWithOptions(r.log, remote.WithOptions(remoteProvider, r.opt.Database, r.opt.EncryptionKey))
	if err != nil {
		return "", nil, fmt.Errorf("new remote provider: %w", err)
	}

	tmpPath := filepath.Join(os.TempDir(), ff.Name)
	r.log.Infof("📥 Downloading to %s...", tmpPath)

	err = provider.Download(ctx, ff.Path, tmpPath)

	if err != nil {
		return "", nil, fmt.Errorf("download backup: %w", err)
	}

	clean := func() {
		if err := os.Remove(tmpPath); err != nil {
			r.log.Warnf("⚠️  Failed to remove temp file %s: %v", tmpPath, err)
		} else {
			r.log.Debugf("🧹 Cleaned up temp file: %s", tmpPath)
		}
	}
	return tmpPath, clean, nil
}

func (r *Restore) executePgRestore(ctx context.Context, input io.Reader) error {
	r.log.Info("🔄 Restoring database...")

	args := []string{
		"-h", r.opt.Database.Host,
		"-p", fmt.Sprintf("%d", r.opt.Database.Port),
		"-U", r.opt.Database.Username,
		"-d", r.opt.Database.Name,
		"--clean",     // DROP objects before creating
		"--if-exists", // Does not fail if object does not exist
		"--no-owner",  // Do not restore ownership
		"--no-acl",    // Do not restore ACLs
		"--verbose",
		"--single-transaction", // All in one transaction (rollback if failed)
	}

	cmd := exec.CommandContext(ctx, "pg_restore", args...)
	cmd.Env = append(os.Environ(), fmt.Sprintf("PGPASSWORD=%s", r.opt.Database.Password))
	cmd.Stdin = input

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start pg_restore: %w", err)
	}

	go func() {
		scanner := bufio.NewScanner(stderrPipe)
		scanner.Buffer(make([]byte, 64*1024), 2*1024*1024) // 2MB max

		for scanner.Scan() {
			r.log.Infof("pg_dump: %s", scanner.Text())
		}

		if err := scanner.Err(); err != nil {
			r.log.Errorf("Erro no scanner: %v", err)
		}
	}()

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("pg_restore failed: %w", err)
	}

	r.log.Info("✅ Restore completed successfully")
	return nil
}
