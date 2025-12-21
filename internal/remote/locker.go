package remote

type Locker interface {
	LockBackup() bool
	UnlockBackup()
}
