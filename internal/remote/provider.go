package remote

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/BrunoTulio/logr"
	"github.com/BrunoTulio/pgopher/internal/backup"
	"github.com/BrunoTulio/pgopher/internal/utils"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/operations"
	"github.com/schollz/progressbar/v3"

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
		log            logr.Logger
		opt            *Options
		fsys           fs.Fs
		currentVersion int
		locker         Locker
	}

	BackupFile struct {
		Name    string
		Path    string
		ModTime time.Time
		Size    int64
	}
)

func NewProvider(locker Locker, log logr.Logger) (*Provider, error) {
	return NewProviderWithOptions(locker, log)
}

func NewProviderWithOptions(
	locker Locker,
	log logr.Logger,
	opts ...FnOptions,
) (*Provider, error) {
	initRclone()

	opt := &Options{}

	for _, o := range opts {
		o(opt)
	}

	fsys, err := createRemoteFs(opt)
	if err != nil {
		return nil, fmt.Errorf("failed to create remote filesystem: %w", err)
	}

	p := &Provider{
		log:            log,
		opt:            opt,
		fsys:           fsys,
		currentVersion: 1,
		locker:         locker,
	}

	return p, nil
}

func (p *Provider) Run(ctx context.Context) error {
	if !p.locker.LockBackup() {
		p.log.Warn("🔒 Restore ativo, backup adiado")
		return nil
	}
	defer p.locker.UnlockBackup()
	defer p.opt.CleanupEnv()

	log := p.log.WithMap(map[string]any{
		"operation": "remote_backup",
		"provider":  p.opt.Name,
		"type":      p.opt.Type,
	})

	log.Infof("☁️  Starting remote backup to %s...", p.opt.Name)
	startTime := time.Now()

	fileName := p.opt.GetRemoteFileName(p.currentVersion)
	tmpDir := os.TempDir()

	log.Infof("   Generating backup: %s", fileName)

	localBackup := backup.NewWithFnOptions(p.log,
		backup.WithGenerateFileName(func() string {
			return fileName
		}),
		backup.WithOutputDir(tmpDir),
		backup.WithoutRetention(),
		backup.WithDatabase(p.opt.Database),
	)

	backupFile, err := localBackup.Run(ctx)
	if err != nil {
		return fmt.Errorf("backup generation failed: %w", err)
	}
	defer func() {
		_ = os.Remove(backupFile)
	}()
	log.Infof("   Uploading to %s...", p.opt.Name)
	if err := p.uploadFile(ctx, backupFile, fileName); err != nil {
		return fmt.Errorf("upload failed: %w", err)
	}

	duration := time.Since(startTime)
	log.Infof("✅ Remote backup to %s completed in %s", p.opt.Name, duration.Round(time.Second))

	return nil
}

func (p *Provider) List(ctx context.Context) ([]BackupFile, error) {
	p.log.Infof("📂 Listing remote: %s", p.opt.Name)

	entries, err := p.fsys.List(ctx, p.opt.Path)
	if err != nil {
		return nil, fmt.Errorf("list remote: %w", err)
	}
	fileMap := make(map[string]fs.DirEntry)

	var files []BackupFile
	for _, entry := range entries {
		remote := entry.Remote()

		if !utils.IsFileBackup(remote) {
			continue
		}

		if existing, found := fileMap[remote]; found {
			if entry.ModTime(ctx).After(existing.ModTime(ctx)) {
				fileMap[remote] = entry
			}
		} else {
			fileMap[remote] = entry
		}

	}
	for _, entry := range fileMap {
		files = append(files, BackupFile{
			Name:    entry.Remote(),
			Size:    entry.Size(),
			ModTime: entry.ModTime(ctx),
		})
	}

	p.log.Infof("📂 Found %d files", len(files))
	return files, nil
}
func (p *Provider) Download(ctx context.Context, fileName, localPath string) error {
	p.log.Infof("📂 Download remote: %s", p.opt.Name)

	obj, err := p.fsys.NewObject(ctx, fileName)

	if err != nil {
		return fmt.Errorf("download remote: %w", err)
	}
	p.log.Infof("   File size: %s", utils.FormatBytes(obj.Size()))

	reader, err := obj.Open(ctx)
	if err != nil {
		return fmt.Errorf("failed to open remote file: %w", err)
	}
	defer reader.Close()

	localFile, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("failed to create local file: %w", err)
	}
	defer func() {
		_ = localFile.Close()
	}()

	bar := progressbar.DefaultBytes(
		obj.Size(),
		fmt.Sprintf("Downloading %s", fileName),
	)

	_, err = io.Copy(io.MultiWriter(localFile, bar), reader)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}

	p.log.Infof("✅ Downloaded %s", fileName)
	return nil
}

func (p *Provider) uploadFile(ctx context.Context, localPath, remoteName string) error {
	file, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("failed to open local file: %w", err)
	}
	defer func() {
		_ = file.Close()
	}()

	fileInfo, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}

	p.log.Infof("   File size: %s", utils.FormatBytes(fileInfo.Size()))

	fullPath := p.opt.RemotePathFor(remoteName)

	_, err = operations.Rcat(ctx, p.fsys, fullPath, file, fileInfo.ModTime(), nil)
	if err != nil {
		return fmt.Errorf("rclone upload failed: %w", err)
	}

	p.log.Infof("   ✅ Uploaded: %s", remoteName)

	return nil
}

func initRclone() {
	rcloneInitOnce.Do(func() {
		//configfile.Install()
		configureRclone()
	})
}

func configureRclone() {
	ctx := context.Background()
	ci := fs.GetConfig(ctx)

	// Log Level
	// - LogLevelDebug: Modo desenvolvimento (muito verboso)
	// - LogLevelInfo: Modo produção (normal)
	// - LogLevelError: Apenas erros
	ci.LogLevel = fs.LogLevelDebug // Trocar para Debug se precisar

	// Performance
	ci.Transfers = 4                             // Conexões paralelas (bom para uploads grandes)
	ci.Checkers = 8                              // Checkers paralelos
	ci.BufferSize = 16 * 1024 * 1024             // 16 MB buffer (importante!)
	ci.StreamingUploadCutoff = 100 * 1024 * 1024 // 100 MB (streaming acima disso)

	// Comportamento
	ci.UseListR = false       // Não usar ListR (melhor para poucos arquivos)
	ci.NoGzip = false         // Usar compressão quando possível
	ci.NoCheckDest = false    // Sempre verificar destino
	ci.IgnoreChecksum = false // Validar checksums
	ci.DryRun = false         // Executar de verdade

	// Timeouts e Retries
	ci.ConnectTimeout = fs.Duration(60 * time.Second)
	ci.Timeout = fs.Duration(5 * time.Minute)
	ci.LowLevelRetries = 10 // Tentativas em erro
	ci.Retries = 3          // Retries de alto nível

	// Stats e Progress
	ci.StatsOneLine = false
	ci.Progress = false
	ci.StatsLogLevel = fs.LogLevelInfo

	// Outros
	ci.UserAgent = "pgopher-backup/1.0"
}

func createRemoteFs(opt *Options) (fs.Fs, error) {
	ctx := context.Background()
	//data := config.LoadedData()

	//data.SetValue(opt.Name, "type", opt.Type)
	//for k, v := range opt.Config {
	//	data.SetValue(opt.Name, k, v)
	//}
	if err := opt.SetupEnv(); err != nil {
		return nil, fmt.Errorf("setup environment: %w", err)
	}

	remotePath := opt.Name + ":"

	fsys, err := fs.NewFs(ctx, remotePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create fs: %w", err)
	}

	return fsys, nil
}
