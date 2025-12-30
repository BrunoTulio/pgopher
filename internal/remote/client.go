package remote

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/BrunoTulio/logr"
	"github.com/BrunoTulio/pgopher/internal/utils"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/operations"
	"github.com/schollz/progressbar/v3"
)

type Client struct {
	log  logr.Logger
	opt  *Options
	fsys fs.Fs
}

func NewClient(log logr.Logger) (*Client, error) {
	return NewClientWithOptions(log)
}

func NewClientWithOptions(
	log logr.Logger,
	opts ...FnOptions,
) (*Client, error) {

	initRclone()

	opt := &Options{}

	for _, o := range opts {
		o(opt)
	}

	fsys, err := createRemoteFs(opt)
	if err != nil {
		return nil, fmt.Errorf("failed to create remote filesystem: %w", err)
	}

	p := &Client{
		log:  log,
		opt:  opt,
		fsys: fsys,
	}

	return p, nil
}

func (c *Client) List(ctx context.Context) ([]BackupFile, error) {
	defer c.opt.CleanupEnv()

	c.log.Infof("📂 Listing remote: %s", c.opt.Name)

	entries, err := c.fsys.List(ctx, c.opt.Path)
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

	c.log.Infof("📂 Found %d files", len(files))
	return files, nil
}

func (c *Client) Download(ctx context.Context, fileName, localPath string) error {
	defer c.opt.CleanupEnv()

	c.log.Infof("📂 Download remote: %s", c.opt.Name)

	obj, err := c.fsys.NewObject(ctx, fileName)
	if err != nil {
		return fmt.Errorf("download remote: %w", err)
	}

	c.log.Infof("   File size: %s", utils.FormatBytes(obj.Size()))

	reader, err := obj.Open(ctx)
	if err != nil {
		return fmt.Errorf("failed to open remote file: %w", err)
	}
	defer func() {
		_ = reader.Close()
	}()

	localFile, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("failed to create local file: %w", err)
	}
	defer func() {
		_ = localFile.Close()
	}()

	bar := progressbar.DefaultBytes(obj.Size(), fmt.Sprintf("Downloading %s", fileName))

	_, err = io.Copy(io.MultiWriter(localFile, bar), reader)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}

	c.log.Infof("✅ Downloaded %s", fileName)
	return nil
}

// UploadFile - upload raw
func (c *Client) UploadFile(ctx context.Context, localPath, remoteName string) (fs.Object, error) {
	file, err := os.Open(localPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open local file: %w", err)
	}
	defer func() {
		_ = file.Close()
	}()

	fileInfo, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}

	c.log.Infof("   File size: %s", utils.FormatBytes(fileInfo.Size()))

	fullPath := c.opt.RemotePathFor(remoteName)
	obj, err := operations.Rcat(ctx, c.fsys, fullPath, file, fileInfo.ModTime(), nil)
	if err != nil {
		return nil, fmt.Errorf("rclone upload failed: %w", err)
	}

	c.log.Infof("   ✅ Uploaded: %s", remoteName)
	return obj, nil
}

func (c *Client) CleanupEnvs() {
	c.opt.CleanupEnv()
}

func initRclone() {
	rcloneInitOnce.Do(func() {
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
	err := opt.SetupEnv()
	if err != nil {
		return nil, fmt.Errorf("setup environment: %w", err)
	}
	remotePath := opt.Name + ":"
	fsys, err := fs.NewFs(ctx, remotePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create fs: %w", err)
	}
	return fsys, nil
}
