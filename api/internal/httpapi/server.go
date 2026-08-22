package httpapi

import (
	"io/fs"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/VitorCdSouza/gamedock/api/internal/manager"
)

type Options struct {
	Manager     *manager.Manager
	WebFS       fs.FS
	AllowOrigin string
}

type Server struct {
	mgr         *manager.Manager
	webFS       fs.FS
	allowOrigin string
	mux         *http.ServeMux
}

func New(o Options) *Server {
	s := &Server{
		mgr:         o.Manager,
		webFS:       o.WebFS,
		allowOrigin: o.AllowOrigin,
		mux:         http.NewServeMux(),
	}
	s.routes()
	return s
}

func (s *Server) routes() {
	m := s.mux

	m.HandleFunc("GET /api/v1/health", s.health)
	m.HandleFunc("GET /api/v1/system", s.system)
	m.HandleFunc("PUT /api/v1/system/root", s.setRoot)
	m.HandleFunc("GET /api/v1/events", s.events)

	m.HandleFunc("GET /api/v1/providers", s.listProviders)
	m.HandleFunc("GET /api/v1/providers/{id...}", s.getProvider)

	m.HandleFunc("GET /api/v1/dns", s.getDNS)
	m.HandleFunc("PUT /api/v1/dns", s.setDNSToken)
	m.HandleFunc("POST /api/v1/dns/sync", s.syncDNS)
	m.HandleFunc("POST /api/v1/dns/domains", s.addDNSDomain)
	m.HandleFunc("DELETE /api/v1/dns/domains/{domain}", s.removeDNSDomain)

	m.HandleFunc("GET /api/v1/instances", s.listInstances)
	m.HandleFunc("POST /api/v1/instances", s.createInstance)
	m.HandleFunc("POST /api/v1/instances/preview-compose", s.previewCompose)
	m.HandleFunc("GET /api/v1/instances/{name}", s.getInstance)
	m.HandleFunc("PUT /api/v1/instances/{name}", s.updateInstance)
	m.HandleFunc("DELETE /api/v1/instances/{name}", s.deleteInstance)
	m.HandleFunc("GET /api/v1/instances/{name}/compose", s.getCompose)
	m.HandleFunc("GET /api/v1/instances/{name}/logs", s.getLogs)
	m.HandleFunc("POST /api/v1/instances/{name}/start", s.action(s.mgr.Start))
	m.HandleFunc("POST /api/v1/instances/{name}/stop", s.action(s.mgr.Stop))
	m.HandleFunc("POST /api/v1/instances/{name}/restart", s.action(s.mgr.Restart))
	m.HandleFunc("POST /api/v1/instances/{name}/update-image", s.action(s.mgr.UpdateImage))
	m.HandleFunc("POST /api/v1/instances/{name}/archive", s.setArchived(true))
	m.HandleFunc("POST /api/v1/instances/{name}/unarchive", s.setArchived(false))
	m.HandleFunc("POST /api/v1/instances/{name}/clear-error", s.clearError)
	m.HandleFunc("PUT /api/v1/instances/{name}/dns", s.linkDNS)
	m.HandleFunc("DELETE /api/v1/instances/{name}/dns", s.unlinkDNS)

	if s.webFS != nil {
		m.Handle("/", s.spa())
	}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if s.allowOrigin != "" {
		h := w.Header()
		h.Set("Access-Control-Allow-Origin", s.allowOrigin)
		h.Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		h.Set("Access-Control-Allow-Headers", "Content-Type")
		h.Set("Vary", "Origin")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
	}
	start := time.Now()
	sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
	s.mux.ServeHTTP(sw, r)
	if strings.HasPrefix(r.URL.Path, "/api/") {
		slog.Info("http",
			"method", r.Method, "path", r.URL.Path,
			"status", sw.status, "dur", time.Since(start).Round(time.Millisecond))
	}
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }
