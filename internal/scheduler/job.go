package scheduler

import (
	"time"

	"github.com/robfig/cron/v3"
)

type JobInfo struct {
	ID       cron.EntryID
	Name     string // ex: "local", "dropbox", "gdrive"
	Type     string // ex: "local", "remote"
	Schedule string // "03:00"
}

type JobStatus struct {
	Name     string
	Type     string
	Schedule string
	Next     time.Time
	Prev     time.Time
}
