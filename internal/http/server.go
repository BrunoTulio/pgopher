package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"time"

	"github.com/BrunoTulio/logr"
	"github.com/BrunoTulio/pgopher/internal/catalog"
	"github.com/BrunoTulio/pgopher/internal/config"
	"github.com/BrunoTulio/pgopher/internal/scheduler"
	"github.com/BrunoTulio/pgopher/internal/utils"
)

type Server struct {
	scheduler  *scheduler.Scheduler
	catalogSrv *catalog.Catalog
	config     *config.Config
	log        logr.Logger
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", s.handleHealth)
	mux.HandleFunc("GET /status", s.handleStatus)
	mux.HandleFunc("GET /providers", s.handleProviders)
	mux.HandleFunc("GET /catalog/{provider}", s.handleCatalogProvider)

	mux.ServeHTTP(w, r)
}

type StatusResponse struct {
	RunningJobs int       `json:"running_jobs"`
	NextRuns    []string  `json:"next_runs"`
	Timestamp   time.Time `json:"timestamp"`
}

type ProvidersResponse struct {
	Providers []string `json:"providers"`
}

type JobStatus struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Schedule string `json:"schedule"`
	Next     string `json:"next"`
	Prev     string `json:"prev"`
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

	out := make([]JobStatus, 0, len(jobs))
	for _, j := range jobs {
		out = append(out, JobStatus{
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
			"mod_time":   file.ModTime.Format(time.RFC3339),
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
