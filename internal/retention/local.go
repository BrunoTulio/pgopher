package retention

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/BrunoTulio/logr"
	"github.com/BrunoTulio/pgopher/internal/utils"
)

type (
	Local struct {
		opt *Options
		log logr.Logger
	}

	BackupFiles []BackupFile

	BackupFile struct {
		Path    string
		ModTime time.Time
		Size    int64
	}
)

func NewLocal(log logr.Logger) *Local {
	return NewLocalWithOptions(log)
}
func NewLocalWithOptions(log logr.Logger, opts ...FnOptions) *Local {
	opt := &Options{}

	for _, o := range opts {
		o(opt)
	}
	return &Local{
		log: log,
		opt: opt,
	}
}

func (l *Local) Run(ctx context.Context) error {
	l.log.Info("🧹 starting local retention")

	if !l.opt.HasRetention() {
		l.log.Info("No retention policy configured, skipping cleanup")
		return nil
	}

	backups, err := l.findBackups()

	if err != nil {
		return fmt.Errorf("find backups: %w", err)
	}

	if len(backups) == 0 {
		l.log.Info("No backups found, nothing to clean")
		return nil
	}

	l.log.Infof("Found %d backup(s)", len(backups))

	var backupRemoved BackupFiles

	if l.opt.HasMaxBackups() {
		if backupRemoved, err = l.cleanByCount(backups, *l.opt.Retention.MaxBackups); err != nil {
			return fmt.Errorf("clean count: %w", err)
		}
	} else if l.opt.HasRetentionDays() {
		if backupRemoved, err = l.cleanByDays(backups, *l.opt.Retention.RetentionDays); err != nil {
			return fmt.Errorf("clean days: %w", err)
		}
	}

	l.log.Infof("✅ Cleanup completed:")
	l.log.Infof("   Removed: %d backup(s)", backupRemoved.Len())
	l.log.Infof("   Kept: %d backup(s)", len(backups)-backupRemoved.Len())
	l.log.Infof("   Space freed: %s", utils.FormatBytes(backupRemoved.Size()))

	return nil
}

func (l *Local) findBackups() (BackupFiles, error) {
	pattern := filepath.Join(l.opt.OutputDir, fmt.Sprintf("%s-*.sql.gz*", l.opt.DatabaseName))

	matches, err := filepath.Glob(pattern)

	if err != nil {
		return nil, fmt.Errorf("failed to find backups: %w", err)
	}

	backups := make(BackupFiles, 0, len(matches))

	for _, path := range matches {
		info, err := os.Stat(path)
		if err != nil {
			l.log.Warnf("Failed to stat %s: %v", path, err)
			continue
		}

		backups = append(backups, BackupFile{
			Path:    path,
			ModTime: info.ModTime(),
			Size:    info.Size(),
		})
	}

	sort.Sort(backups)

	return backups, nil

}

func (l *Local) cleanByCount(backups BackupFiles, maxBackups int) (BackupFiles, error) {

	if len(backups) < maxBackups {
		l.log.Warnf("%d backups, ignoring cleaning, as it did not reach the maximum value allowed %d", len(backups), maxBackups)
		return nil, nil
	}

	toRemove := backups[:len(backups)-maxBackups]
	removed := make(BackupFiles, 0, len(backups))

	for _, backup := range toRemove {
		l.log.Infof("Removing old backup: %s (age: %s, size: %s)",
			filepath.Base(backup.Path),
			utils.FormatDuration(time.Since(backup.ModTime)),
			utils.FormatBytes(backup.Size))

		err := os.Remove(backup.Path)

		if err != nil {
			l.log.Warnf("Failed to remove backup %s: %v", backup.Path, err)
			continue
		}
		removed = append(removed, backup)
	}

	return removed, nil

}

func (l *Local) cleanByDays(backups BackupFiles, retentionDays int) (BackupFiles, error) {
	var retentionsDays BackupFiles
	cutoff := time.Now().AddDate(0, 0, -retentionDays)

	for _, backup := range backups {
		if backup.ModTime.Before(cutoff) {
			retentionsDays = append(retentionsDays, backup)
		}
	}

	if len(retentionsDays) <= 0 {
		l.log.Warnf("No backups found in %d days", retentionDays)
		return nil, nil
	}

	removed := make(BackupFiles, 0, len(retentionsDays))

	for _, backup := range retentionsDays {
		l.log.Infof("Removing old backup: %s (age: %s, size: %s)",
			filepath.Base(backup.Path),
			utils.FormatDuration(time.Since(backup.ModTime)),
			utils.FormatBytes(backup.Size))

		if err := os.Remove(backup.Path); err != nil {
			l.log.Warnf("Failed to remove backup %s: %v", backup.Path, err)
			continue
		}
		removed = append(removed, backup)
	}

	return removed, nil
}

func (b BackupFiles) Paths() []string {
	paths := make([]string, len(b), len(b))
	for i, backup := range b {
		paths[i] = backup.Path
	}
	return paths
}

func (b BackupFiles) Size() int64 {
	var total int64 = 0
	for _, backup := range b {
		total += backup.Size
	}
	return total
}

func (b BackupFiles) Len() int           { return len(b) }
func (b BackupFiles) Less(i, j int) bool { return b[i].ModTime.Before(b[j].ModTime) }
func (b BackupFiles) Swap(i, j int)      { b[i], b[j] = b[j], b[i] }
