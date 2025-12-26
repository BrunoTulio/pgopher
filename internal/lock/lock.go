package lock

type Locker interface {
	IsRestoreRunning() bool
	LockForRestore() error
	UnlockForRestore() error
}
