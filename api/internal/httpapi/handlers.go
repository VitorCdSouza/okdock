package httpapi

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/VitorCdSouza/okdock/api/internal/dockerx"
	"github.com/VitorCdSouza/okdock/api/internal/instance"
	"github.com/VitorCdSouza/okdock/api/internal/manager"
	"github.com/VitorCdSouza/okdock/api/internal/template"
)

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) system(w http.ResponseWriter, r *http.Request) {
	info, err := s.mgr.System(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, info)
}

type templatesResponse struct {
	Templates  []template.Template `json:"templates"`
	Categories []template.Category `json:"categories"`
}

func (s *Server) listTemplates(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, templatesResponse{
		Templates:  s.templates.All(),
		Categories: s.templates.Categories(),
	})
}

func (s *Server) getTemplate(w http.ResponseWriter, r *http.Request) {
	t, ok := s.templates.Get(r.PathValue("id"))
	if !ok {
		writeJSON(w, http.StatusNotFound, apiError{
			Error:   "not_found",
			Message: fmt.Sprintf("template %q is not in the catalog", r.PathValue("id")),
		})
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func (s *Server) createTemplate(w http.ResponseWriter, r *http.Request) {
	var t template.Template
	if !decodeJSON(w, r, &t) {
		return
	}
	if _, exists := s.templates.Get(t.ID); exists {
		writeJSON(w, http.StatusConflict, apiError{
			Error:   "template_exists",
			Message: fmt.Sprintf("a template with id %q already exists", t.ID),
		})
		return
	}
	if err := s.templates.Save(t); err != nil {
		writeError(w, err)
		return
	}
	saved, _ := s.templates.Get(t.ID)
	writeJSON(w, http.StatusCreated, saved)
}

func (s *Server) saveTemplate(w http.ResponseWriter, r *http.Request) {
	var t template.Template
	if !decodeJSON(w, r, &t) {
		return
	}
	t.ID = r.PathValue("id")
	if err := s.templates.Save(t); err != nil {
		writeError(w, err)
		return
	}
	saved, _ := s.templates.Get(t.ID)
	writeJSON(w, http.StatusOK, saved)
}

func (s *Server) deleteTemplate(w http.ResponseWriter, r *http.Request) {
	if err := s.templates.Delete(r.PathValue("id")); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type instancesResponse struct {
	Instances []instance.Instance `json:"instances"`
	States    []instance.State    `json:"states"`
}

func (s *Server) listInstances(w http.ResponseWriter, r *http.Request) {
	list, err := s.mgr.List(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	if list == nil {
		list = []instance.Instance{}
	}
	writeJSON(w, http.StatusOK, instancesResponse{Instances: list, States: instance.AllStates})
}

func (s *Server) getInstance(w http.ResponseWriter, r *http.Request) {
	inst, err := s.mgr.Get(r.Context(), r.PathValue("name"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, inst)
}

func (s *Server) createInstance(w http.ResponseWriter, r *http.Request) {
	var req manager.SpecRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	spec, err := s.mgr.Create(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, spec)
}

func (s *Server) updateInstance(w http.ResponseWriter, r *http.Request) {
	var req manager.SpecRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	spec, err := s.mgr.Update(r.Context(), r.PathValue("name"), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, spec)
}

func (s *Server) deleteInstance(w http.ResponseWriter, r *http.Request) {
	keepData := r.URL.Query().Get("keepData") != "false"
	if err := s.mgr.Delete(r.Context(), r.PathValue("name"), keepData); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type previewComposeResponse struct {
	Compose  string   `json:"compose"`
	Recreate []string `json:"recreate,omitempty"`
}

func (s *Server) previewCompose(w http.ResponseWriter, r *http.Request) {
	var req manager.SpecRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	yml, err := s.mgr.PreviewCompose(req)
	if err != nil {
		writeError(w, err)
		return
	}
	resp := previewComposeResponse{Compose: string(yml)}
	if old, err := s.mgr.Store().Get(req.Name); err == nil {
		if next, err := s.mgr.BuildSpec(req); err == nil {
			resp.Recreate = manager.RecreateFields(old, next)
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

type imageSearchResponse struct {
	Images []dockerx.ImageHit `json:"images"`
}

// the search is by repository name, the registry knows nothing about the tag
func (s *Server) searchImages(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	hits, err := s.mgr.SearchImages(r.Context(), r.URL.Query().Get("q"), limit)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, imageSearchResponse{Images: hits})
}

type imageTagsResponse struct {
	Tags []string `json:"tags"`
}

func (s *Server) imageTags(w http.ResponseWriter, r *http.Request) {
	tags, err := s.mgr.ImageTags(r.Context(), r.URL.Query().Get("image"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, imageTagsResponse{Tags: tags})
}

// what the panel fills in on its own, and a missing image is an empty answer, not an error
func (s *Server) suggestFromImage(w http.ResponseWriter, r *http.Request) {
	out, err := s.mgr.SuggestFromImage(r.Context(), r.URL.Query().Get("image"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) getCompose(w http.ResponseWriter, r *http.Request) {
	raw, err := s.mgr.Compose(r.Context(), r.PathValue("name"))
	if err != nil {
		writeError(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/yaml; charset=utf-8")
	_, _ = w.Write(raw)
}

func (s *Server) action(fn func(context.Context, string) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := fn(r.Context(), r.PathValue("name")); err != nil {
			writeError(w, err)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	}
}

func (s *Server) setArchived(archived bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := s.mgr.SetArchived(r.Context(), r.PathValue("name"), archived); err != nil {
			writeError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func (s *Server) clearError(w http.ResponseWriter, r *http.Request) {
	s.mgr.ClearError(r.PathValue("name"))
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) setRoot(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Root string `json:"root"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := s.mgr.SetRoot(strings.TrimSpace(req.Root)); err != nil {
		writeError(w, err)
		return
	}
	s.system(w, r)
}

func (s *Server) setTemplatesDir(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Templates string `json:"templates"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := s.mgr.SetTemplatesDir(strings.TrimSpace(req.Templates)); err != nil {
		writeError(w, err)
		return
	}
	s.system(w, r)
}

func (s *Server) getDNS(w http.ResponseWriter, _ *http.Request) {
	status := s.mgr.DNS()
	if status.Links == nil {
		status.Links = []manager.DNSLink{}
	}
	if status.Domains == nil {
		status.Domains = []instance.DNS{}
	}
	writeJSON(w, http.StatusOK, status)
}

func (s *Server) setDNSToken(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token string `json:"token"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := s.mgr.SetDNSToken(r.Context(), strings.TrimSpace(req.Token)); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, s.mgr.DNS())
}

func (s *Server) addDNSDomain(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Domain string `json:"domain"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	entry, err := s.mgr.AddDNSDomain(r.Context(), req.Domain)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, entry)
}

func (s *Server) removeDNSDomain(w http.ResponseWriter, r *http.Request) {
	if err := s.mgr.RemoveDNSDomain(r.PathValue("domain")); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) syncDNS(w http.ResponseWriter, _ *http.Request) {
	go s.mgr.SyncDNS(context.Background())
	w.WriteHeader(http.StatusAccepted)
}

func (s *Server) linkDNS(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Domain string `json:"domain"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	link, err := s.mgr.LinkDNS(r.Context(), r.PathValue("name"), req.Domain)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, link)
}

func (s *Server) unlinkDNS(w http.ResponseWriter, r *http.Request) {
	if err := s.mgr.UnlinkDNS(r.PathValue("name")); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) getLogs(w http.ResponseWriter, r *http.Request) {
	tail := 200
	if v := r.URL.Query().Get("tail"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 5000 {
			tail = n
		}
	}
	follow := r.URL.Query().Get("follow") != "false"

	flusher, ok := w.(http.Flusher)
	if !ok {
		badRequest(w, "this connection does not support streaming")
		return
	}

	rc, err := s.mgr.Logs(r.Context(), r.PathValue("name"), tail, follow)
	if err != nil {
		writeError(w, err)
		return
	}
	defer rc.Close()

	sseHeaders(w)
	flusher.Flush()

	sc := bufio.NewScanner(rc)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		if r.Context().Err() != nil {
			return
		}
		payload, err := json.Marshal(sc.Text())
		if err != nil {
			continue
		}
		fmt.Fprintf(w, "event: log\ndata: %s\n\n", payload)
		flusher.Flush()
	}
	fmt.Fprint(w, "event: end\ndata: {}\n\n")
	flusher.Flush()
}

func (s *Server) events(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		badRequest(w, "this connection does not support streaming")
		return
	}
	ch, cancel := s.mgr.Events().Subscribe()
	defer cancel()

	sseHeaders(w)
	fmt.Fprint(w, "event: ready\ndata: {}\n\n")
	flusher.Flush()

	ping := timeTicker(25)
	defer ping.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ping.C:
			fmt.Fprint(w, ": ping\n\n")
			flusher.Flush()
		case ev, ok := <-ch:
			if !ok {
				return
			}
			payload, err := json.Marshal(ev)
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", ev.Type, payload)
			flusher.Flush()
		}
	}
}

func sseHeaders(w http.ResponseWriter) {
	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	h.Set("Connection", "keep-alive")
	h.Set("X-Accel-Buffering", "no")
}
