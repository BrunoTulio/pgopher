package http

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"path/filepath"
	"slices"
	"syscall"
	"time"

	"github.com/BrunoTulio/logr"
	"github.com/BrunoTulio/pgopher/internal/catalog"
	"github.com/BrunoTulio/pgopher/internal/config"
	"github.com/BrunoTulio/pgopher/internal/scheduler"
	"github.com/BrunoTulio/pgopher/internal/utils"
)

type (
	Server struct {
		scheduler  *scheduler.Scheduler
		catalogSrv *catalog.Catalog
		config     *config.Config
		log        logr.Logger
	}
	StatusResponse struct {
		RunningJobs int       `json:"running_jobs"`
		NextRuns    []string  `json:"next_runs"`
		Timestamp   time.Time `json:"timestamp"`
	}

	ProvidersResponse struct {
		Providers []string `json:"providers"`
	}

	JobStatusResponse struct {
		Name     string `json:"name"`
		Type     string `json:"type"`
		Schedule string `json:"schedule"`
		Next     string `json:"next"`
		Prev     string `json:"prev"`
	}

	DiskStatusResponse struct {
		Total       uint64  `json:"totalBytes"`
		TotalHuman  string  `json:"totalHuman"`
		Free        uint64  `json:"freeBytes"`
		FreeHuman   string  `json:"freeHuman"`
		Used        uint64  `json:"usedBytes"`
		UsedHuman   string  `json:"usedHuman"`
		UsedPercent float64 `json:"usedPercent"`
	}

	LocalStorageResponse struct {
		TotalSize      int64  `json:"totalSizeBytes"`
		TotalSizeHuman string `json:"totalSizeHuman"`
		BackupCount    int    `json:"backupCount"`
		OldestBackup   string `json:"oldestBackup,omitempty"`
		NewestBackup   string `json:"newestBackup,omitempty"`
		AvgSize        int64  `json:"avgSizeBytes"`
		AvgSizeHuman   string `json:"avgSizeHuman"`
		Directory      string `json:"directory"`
	}
)

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", s.handleHealth)
	mux.HandleFunc("GET /status", s.handleStatus)
	mux.HandleFunc("GET /providers", s.handleProviders)
	mux.HandleFunc("GET /catalog/{provider}", s.handleCatalogProvider)
	mux.HandleFunc("GET /storage/local", s.handlerStorageLocal)

	mux.ServeHTTP(w, r)
}

func New(
	cfg *config.Config,
	catalogSrv *catalog.Catalog,
	scheduler *scheduler.Scheduler,
	log logr.Logger,
) http.Handler {
	return &Server{
		scheduler:  scheduler,
		config:     cfg,
		log:        log,
		catalogSrv: catalogSrv,
	}
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "OK"})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	jobs := s.scheduler.GetJobsStatus()

	out := make([]JobStatusResponse, 0, len(jobs))
	for _, j := range jobs {
		out = append(out, JobStatusResponse{
			Name:     j.Name,
			Type:     j.Type,
			Schedule: j.Schedule,
			Next:     j.Next.Format(time.RFC3339),
			Prev:     j.Prev.Format(time.RFC3339),
		})
	}

	resp := map[string]any{
		"running_jobs": s.scheduler.GetRunningJobs(),
		"jobs":         out,
		"timestamp":    time.Now().UTC(),
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleProviders(w http.ResponseWriter, r *http.Request) {
	providers := make([]string, 0)
	for _, p := range s.config.RemoteProviders {
		if p.Enabled {
			providers = append(providers, p.Name)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"providers": append(providers, "local"),
	})
}

func (s *Server) handleCatalogProvider(w http.ResponseWriter, r *http.Request) {
	providers := []string{"local"}
	for _, p := range s.config.RemoteProviders {
		if p.Enabled {
			providers = append(providers, p.Name)
		}
	}

	providerName := r.PathValue("provider")
	providerExist := slices.Contains(providers, providerName)

	if !providerExist {
		http.Error(w, fmt.Sprintf("provider '%s' not found", providerName), http.StatusBadRequest)
		return
	}

	files, err := s.catalogSrv.List(r.Context(), providerName)

	if err != nil {
		s.log.Errorf("catalog list failed: %v", err)
		http.Error(w, "failed to list backups", http.StatusInternalServerError)
		return
	}

	filesResp := make([]map[string]any, len(files))
	for i, file := range files {
		filesResp[i] = map[string]any{
			"short_id":   file.ShortID,
			"name":       file.Name,
			"size_bytes": file.Size,
			"size_human": utils.FormatBytes(file.Size),
			"mod_time":   file.ModTime,
			"encrypted":  file.Encrypted,
		}
	}

	response := map[string]interface{}{
		"provider":  providerName,
		"count":     len(files),
		"files":     filesResp,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)

}

func (s *Server) handlerStorageLocal(w http.ResponseWriter, r *http.Request) {
	disk, err := s.getDiskStatus()

	if err != nil {
		s.log.Errorf("failed disk status: %v", err)
		http.Error(w, "failed disk status", http.StatusInternalServerError)
		return
	}

	backup, err := s.getLocalStorageStats()

	if err != nil {
		s.log.Errorf("failed local storage stats %v", err)
		http.Error(w, "failed local storage stats", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"backup":    backup,
		"disk":      disk,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)

}

func (s *Server) getLocalStorageStats() (LocalStorageResponse, error) {
	backupDir := s.config.LocalBackup.Dir

	var totalSize int64
	var count int
	var oldest, newest time.Time

	err := filepath.WalkDir(backupDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		if !utils.IsFileBackup(d.Name()) {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return err
		}

		totalSize += info.Size()
		count++

		modTime := info.ModTime()
		if oldest.IsZero() || modTime.Before(oldest) {
			oldest = modTime
		}
		if newest.IsZero() || modTime.After(newest) {
			newest = modTime
		}

		return nil
	})
	if err != nil {
		return LocalStorageResponse{}, fmt.Errorf("failed to scan backup directory: %w", err)
	}

	avgSize := int64(0)
	if count > 0 {
		avgSize = totalSize / int64(count)
	}

	stats := LocalStorageResponse{
		TotalSize:      totalSize,
		TotalSizeHuman: utils.FormatBytes(totalSize),
		BackupCount:    count,
		AvgSize:        avgSize,
		AvgSizeHuman:   utils.FormatBytes(avgSize),
		Directory:      backupDir,
	}

	if !oldest.IsZero() {
		stats.OldestBackup = oldest.Format(time.RFC3339)
	}
	if !newest.IsZero() {
		stats.NewestBackup = newest.Format(time.RFC3339)
	}

	return stats, nil
}

func (s *Server) getDiskStatus() (DiskStatusResponse, error) {
	var stat syscall.Statfs_t

	if err := syscall.Statfs(s.config.LocalBackup.Dir, &stat); err != nil {
		return DiskStatusResponse{}, fmt.Errorf("statfs: %w", err)
	}

	total := stat.Blocks * uint64(stat.Bsize)
	free := stat.Bavail * uint64(stat.Bsize)
	used := total - free

	usedPercent := 0.0
	if total > 0 {
		usedPercent = (float64(used) / float64(total)) * 100
	}

	return DiskStatusResponse{
		Total:       total,
		TotalHuman:  utils.FormatBytes(int64(total)),
		Free:        free,
		FreeHuman:   utils.FormatBytes(int64(free)),
		Used:        used,
		UsedHuman:   utils.FormatBytes(int64(used)),
		UsedPercent: usedPercent,
	}, nil
}
