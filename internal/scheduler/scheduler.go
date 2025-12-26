package scheduler

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/BrunoTulio/logr"
	"github.com/BrunoTulio/pgopher/internal/backup"
	"github.com/BrunoTulio/pgopher/internal/config"
	"github.com/BrunoTulio/pgopher/internal/lock"
	"github.com/BrunoTulio/pgopher/internal/notify"
	"github.com/BrunoTulio/pgopher/internal/remote"

	"github.com/robfig/cron/v3"
)

type wrapLogger struct {
	logr.Logger
}

func (l *wrapLogger) Printf(format string, args ...interface{}) {
	l.Infof(format, args...)
}

type Scheduler struct {
	cron        *cron.Cron
	opt         *Options
	backupSvc   *backup.Local
	mu          sync.Mutex
	runningJobs int
	log         logr.Logger
	notifier    notify.Notifier
	locker      lock.Locker
	jobs        []JobInfo
}

func New(backupSvc *backup.Local,
	locker lock.Locker,
	notifier notify.Notifier,
	log logr.Logger,
) *Scheduler {
	return NewWithOptions(backupSvc, notifier, locker, log)
}

func NewWithOptions(
	backupSvc *backup.Local,
	notifier notify.Notifier,
	locker lock.Locker,
	log logr.Logger,
	opts ...func(options *Options),
) *Scheduler {
	opt := &Options{}

	for _, fn := range opts {
		fn(opt)
	}

	c := cron.New(
		cron.WithLocation(opt.timezone),
		cron.WithLogger(cron.VerbosePrintfLogger(&wrapLogger{log})),
	)

	return &Scheduler{
		cron:      c,
		opt:       opt,
		backupSvc: backupSvc,
		log:       log,
		notifier:  notifier,
		locker:    locker,
	}
}

func (s *Scheduler) Start() error {

	s.log.Info("🕐 Starting scheduler...")

	if err := s.scheduleLocalBackups(); err != nil {
		return fmt.Errorf("failed to schedule local backups: %w", err)
	}

	if err := s.scheduleRemoteBackups(); err != nil {
		return fmt.Errorf("failed to schedule remote backups: %w", err)
	}

	s.cron.Start()
	s.log.Info("✅ Scheduler started successfully")

	return nil
}

func (s *Scheduler) Stop() {
	s.log.Info("Stopping scheduler...")

	ctx := s.cron.Stop()
	<-ctx.Done()

	s.log.Info("✅ Scheduler stopped")
}

func (s *Scheduler) GetNextRuns() []time.Time {
	entries := s.cron.Entries()
	nextRuns := make([]time.Time, len(entries))

	for i, entry := range entries {
		nextRuns[i] = entry.Next
	}

	return nextRuns
}

func (s *Scheduler) GetJobs() []JobInfo {
	return s.jobs
}

func (s *Scheduler) GetJobsStatus() []JobStatus {
	entries := s.cron.Entries()
	idToEntry := make(map[cron.EntryID]cron.Entry, len(entries))
	for _, e := range entries {
		idToEntry[e.ID] = e
	}

	res := make([]JobStatus, 0, len(s.jobs))
	for _, j := range s.jobs {
		e, ok := idToEntry[j.ID]
		if !ok {
			continue
		}
		res = append(res, JobStatus{
			Name:     j.Name,
			Type:     j.Type,
			Schedule: j.Schedule,
			Next:     e.Next,
			Prev:     e.Prev,
		})
	}
	return res
}

func (s *Scheduler) GetRunningJobs() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.runningJobs
}

func (s *Scheduler) scheduleRemoteBackups() error {
	for _, provider := range s.opt.Providers {
		if !provider.Enabled {
			continue
		}

		schedules := provider.Schedule
		if len(schedules) == 0 {
			continue
		}

		for _, schedule := range schedules {
			cronExpr, err := s.convertCronExp(schedule)
			if err != nil {
				return fmt.Errorf("provider %s: failed to convert cron %s: %w", provider.Name, schedule, err)
			}

			id, err := s.cron.AddFunc(cronExpr, func() {
				s.runRemoteBackup(provider)
			})

			if err != nil {
				return fmt.Errorf("provider %s: failed to schedule %s: %w", provider.Name, schedule, err)
			}

			s.jobs = append(s.jobs, JobInfo{
				ID:       id,
				Name:     provider.Name,
				Type:     "remote",
				Schedule: schedule,
			})

			s.log.Infof("☁️  Scheduled provider backup %s at: %s (cron: %s)", provider.Name, schedule, cronExpr)
		}
	}
	return nil
}

func (s *Scheduler) scheduleLocalBackups() error {
	schedules := s.opt.Local.Schedule

	if len(schedules) == 0 {
		s.log.Info("No local backup schedules configured")
		return nil
	}

	for _, schedule := range schedules {

		cronExpr, err := s.convertCronExp(schedule)

		if err != nil {
			return fmt.Errorf("failed to convert cron expression: %w", err)
		}

		id, err := s.cron.AddFunc(cronExpr, func() {
			s.runLocalBackup()
		})

		if err != nil {
			return fmt.Errorf("failed to schedule backup at %s: %w", schedule, err)
		}

		s.jobs = append(s.jobs, JobInfo{
			ID:       id,
			Name:     "local",
			Type:     "local",
			Schedule: schedule,
		})

		s.log.Infof("📅 Scheduled local backup at: %s (cron: %s)", schedule, cronExpr)
	}
	return nil
}

func (s *Scheduler) runLocalBackup() {

	if s.locker.IsRestoreRunning() {
		s.log.Warn("⚠️  Restore in progress, skipping scheduled backup")
		return
	}

	s.mu.Lock()
	s.runningJobs++
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.runningJobs--
		s.mu.Unlock()
	}()

	s.log.Info("⏰ Scheduled backup local started")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	backupFile, err := s.backupSvc.Run(ctx)
	if err != nil {
		s.log.Errorf("❌ Backup local failed: %v", err)
		go func() {
			_ = s.notifier.Error(ctx, fmt.Sprintf("❌ Backup local failed: %v", err))
		}()
		return
	}

	s.log.Infof("✅ Backup local completed: %s", backupFile)
	go func() {
		_ = s.notifier.Success(ctx, fmt.Sprintf("✅ Backup local completed: %s", backupFile))
	}()
}

func (s *Scheduler) runRemoteBackup(remoteProvider config.RemoteProvider) {

	if s.locker.IsRestoreRunning() {
		s.log.Warn("⚠️  Restore in progress, skipping scheduled backup")
		return
	}

	s.log.Infof("☁️  Scheduled cfg backup started: %s", remoteProvider.Name)

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(remoteProvider.Timeout)*time.Second)
	defer cancel()

	provider, err := remote.NewProviderWithOptions( /*s.locker,*/ s.log, remote.WithOptions(remoteProvider, s.opt.Database, s.opt.EncryptionKey))
	if err != nil {
		s.log.Errorf("❌ Remote %s provider creation failed: %v", remoteProvider.Name, err)
		go func() {
			_ = s.notifier.Error(ctx, fmt.Sprintf("provider %s creation: %v", remoteProvider.Name, err))
		}()
		return
	}

	if err := provider.Backup(ctx); err != nil {
		s.log.Errorf("❌ Remote %s backup failed: %v", remoteProvider.Name, err)
		go func() {
			_ = s.notifier.Error(ctx, fmt.Sprintf("❌ Remote %s backup failed: %v", remoteProvider.Name, err))
		}()
		return
	}

	s.log.Infof("✅ Remote %s backup completed", remoteProvider.Name)

	go func() {
		_ = s.notifier.Success(ctx, fmt.Sprintf("✅ Remote %s backup completed", remoteProvider.Name))
	}()

}

func (s *Scheduler) convertCronExp(schedule string) (string, error) {
	var hour, minute int
	if _, err := fmt.Sscanf(schedule, "%d:%d", &hour, &minute); err != nil {
		s.log.Warnf("failed to parse hour %d:%d: %v", hour, minute, err)
		return "", fmt.Errorf("failed to parse hour %d:%d: %w", hour, minute, err)
	}

	cronExpr := fmt.Sprintf("%d %d * * *", minute, hour)
	return cronExpr, nil
}
