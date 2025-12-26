package lock

import (
	"os"
	"path/filepath"
	"time"

	"github.com/gofrs/flock"
)

type FileLock struct {
	flock *flock.Flock
}

func (f *FileLock) IsRestoreRunning() bool {
	locked, err := f.flock.TryLock()
	if err != nil || !locked {
		return true
	}
	_ = f.flock.Unlock()
	return false
}

func (f *FileLock) LockForRestore() error {
	for {
		locked, err := f.flock.TryLock()
		if err != nil {
			return err
		}
		if locked {
			return nil
		}
		time.Sleep(1 * time.Second)
	}
}

func (f *FileLock) UnlockForRestore() error {
	return f.flock.Unlock()
}

func New() Locker {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		cacheDir = os.TempDir()
	}

	lockDir := filepath.Join(cacheDir, "pgopher")
	if err := os.MkdirAll(lockDir, os.ModePerm); err != nil {
		lockDir = cacheDir
	}

	lockFile := filepath.Join(lockDir, "restore.lock")

	return &FileLock{
		flock: flock.New(lockFile),
	}
}
