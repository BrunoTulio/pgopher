package backup

import (
	"bufio"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/BrunoTulio/logr"
	"github.com/BrunoTulio/pgopher/internal/encoder"
	"github.com/BrunoTulio/pgopher/internal/retention"
	"github.com/BrunoTulio/pgopher/internal/utils"
)

type (
	Local struct {
		log logr.Logger
		ret *retention.Local
		opt *Options
	}
)

func New(log logr.Logger) *Local {
	return NewWithFnOptions(log)
}

func NewWithFnOptions(log logr.Logger, opts ...func(*Options)) *Local {
	opt := &Options{}
	for _, o := range opts {
		o(opt)
	}

	return &Local{
		log: log,
		opt: opt,
		ret: createRetention(log, opt),
	}
}

func (b *Local) Run(ctx context.Context) (string, error) {
	b.log.Info("starting backup local")

	if err := os.MkdirAll(b.opt.OutputDir, os.ModePerm); err != nil {
		return "", fmt.Errorf("failed to create backup directory: %w", err)
	}

	filename := b.opt.GenerateFileName()

	if b.opt.IsEncryptEnabled() {
		filename += ".age"
	}
	f := filepath.Join(b.opt.OutputDir, filename)

	b.log.Infof("Backup file: %s", filename)
	startTime := time.Now()
	if err := b.executePgDump(ctx, f); err != nil {
		return "", fmt.Errorf("pg_dump failed: %w", err)
	}
	duration := time.Since(startTime)

	fileInfo, err := os.Stat(f)
	if err != nil {
		return "", fmt.Errorf("failed to stat file %s: %w", f, err)
	}

	if fileInfo.Size() == 0 {
		_ = os.Remove(f)
		return "", fmt.Errorf("backup file is empty")
	}
	b.log.Infof("✅ Backup completed successfully")
	b.log.Infof("   File: %s", filename)
	b.log.Infof("   Size: %s", utils.FormatBytes(fileInfo.Size()))
	b.log.Infof("   Duration: %s", duration.Round(time.Second))

	if b.opt.HasRetention() {
		b.log.Info("🧹 Running retention cleanup after backup...")

		if err := b.ret.Run(ctx); err != nil {
			b.log.Errorf("⚠️  Retention cleanup failed: %v", err)
		}
	}

	return f, nil
}

func createRetention(log logr.Logger, opt *Options) *retention.Local {
	return retention.NewLocalWithOptions(log,
		retention.WithRetention(opt.Retention.MaxBackups, opt.Retention.RetentionDays),
		retention.WithOutputDir(opt.OutputDir),
		retention.WithDatabaseName(opt.Database.Name),
	)
}

func (b *Local) executePgDump(ctx context.Context, outputPath string) error {
	outFile, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer func() {
		_ = outFile.Close()
	}()

	var finalWriter io.WriteCloser = outFile

	if b.opt.IsEncryptEnabled() {
		enc, err := encoder.NewEncryptor(b.opt.EncryptionKey)

		if err != nil {
			return fmt.Errorf("failed to create encryptor: %w", err)
		}
		ageWriter, err := enc.NewWriter(outFile)
		if err != nil {
			return fmt.Errorf("failed to create age writer: %w", err)
		}

		defer func() {
			_ = ageWriter.Close()
		}()

		finalWriter = ageWriter
	}

	gz := gzip.NewWriter(finalWriter)
	gz.Name = filepath.Base(outputPath)
	gz.ModTime = time.Now()
	defer func() {
		_ = gz.Close()
	}()

	args := []string{
		"-h", b.opt.Database.Host,
		"-p", fmt.Sprintf("%d", b.opt.Database.Port),
		"-U", b.opt.Database.Username,
		"-d", b.opt.Database.Name,
		"-F", "c", // Custom format
		"--no-privileges",          // Does not include GRANT/REVOKE (security/portability)
		"--no-owner",               // Without ownership
		"--no-acl",                 // Without ACLs
		"--verbose",                // Verbose outputPath
		"--compress=6",             // Compression level (0-9, default is 1)
		"--no-unlogged-table-data", // Do not backup unb.logged tables (they are volatile anyway)
		"--lock-wait-timeout=300",  // 5 minute timeout for locks
	}

	cmd := exec.CommandContext(ctx, "pg_dump", args...)
	cmd.Env = append(os.Environ(), fmt.Sprintf("PGPASSWORD=%s", b.opt.Database.Password))
	cmd.Stdout = gz

	stderrPipe, err := cmd.StderrPipe()

	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start pg_dump: %w", err)
	}

	go func() {
		scanner := bufio.NewScanner(stderrPipe)
		scanner.Buffer(make([]byte, 64*1024), 2*1024*1024) // 2MB max

		for scanner.Scan() {
			b.log.Infof("pg_dump: %s", scanner.Text())
		}

		if err := scanner.Err(); err != nil {
			b.log.Errorf("Erro no scanner: %v", err)
		}
	}()

	if err := cmd.Wait(); err != nil {
		_ = os.Remove(outputPath)
		return fmt.Errorf("pg_dump failed: %w", err)
	}

	return nil
}
