package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/VitorCdSouza/okdock/api/internal/dockerx"
	"github.com/VitorCdSouza/okdock/api/internal/duckdns"
	"github.com/VitorCdSouza/okdock/api/internal/instance"
	"github.com/VitorCdSouza/okdock/api/internal/manager"
	"github.com/VitorCdSouza/okdock/api/internal/store"
	"github.com/VitorCdSouza/okdock/api/internal/template"
)

// a resposta de erro leva codigo e dados, e a mensagem e ultimo recurso para o JSON cru
type apiError struct {
	Error    string             `json:"error"`
	Message  string             `json:"message"`
	Params   map[string]any     `json:"params,omitempty"`
	Problems []template.Problem `json:"problems,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("escrevendo resposta", "err", err)
	}
}

func writeError(w http.ResponseWriter, err error) {
	var (
		notFound    *store.NotFoundError
		exists      *store.ExistsError
		invalidRoot *store.InvalidRootError
		validation  *template.ValidationError
		tmplMissing *template.NotFoundError
		tmplBuiltin *template.BuiltinError
		budget      *manager.ErrBudget
		port        *manager.ErrPortTaken
		dnsTaken    *manager.DNSTakenError
		unreachable *duckdns.UnreachableError
		dockerErr   *dockerx.Error
	)

	switch {
	case errors.As(err, &notFound):
		writeJSON(w, http.StatusNotFound, apiError{
			Error:   "not_found",
			Message: err.Error(),
			Params:  map[string]any{"name": notFound.Name},
		})
	case errors.Is(err, store.ErrNotFound):
		writeJSON(w, http.StatusNotFound, apiError{Error: "not_found", Message: err.Error()})
	case errors.As(err, &exists):
		writeJSON(w, http.StatusConflict, apiError{
			Error:   "already_exists",
			Message: err.Error(),
			Params:  map[string]any{"name": exists.Name},
		})
	case errors.Is(err, store.ErrExists):
		writeJSON(w, http.StatusConflict, apiError{Error: "already_exists", Message: err.Error()})
	case errors.As(err, &invalidRoot):
		writeJSON(w, http.StatusUnprocessableEntity, apiError{
			Error:   "invalid_root",
			Message: err.Error(),
			Params: map[string]any{
				"reason": invalidRoot.Reason,
				"path":   invalidRoot.Path,
				"detail": invalidRoot.Detail,
			},
		})
	case errors.Is(err, store.ErrInvalidRoot):
		writeJSON(w, http.StatusUnprocessableEntity, apiError{Error: "invalid_root", Message: err.Error()})
	case errors.As(err, &tmplMissing):
		writeJSON(w, http.StatusNotFound, apiError{
			Error:   "not_found",
			Message: err.Error(),
			Params:  map[string]any{"template": tmplMissing.ID},
		})
	case errors.As(err, &tmplBuiltin):
		writeJSON(w, http.StatusConflict, apiError{
			Error:   "template_builtin",
			Message: err.Error(),
			Params:  map[string]any{"template": tmplBuiltin.ID},
		})
	case errors.As(err, &validation):
		writeJSON(w, http.StatusUnprocessableEntity, apiError{
			Error:    "invalid_fields",
			Message:  err.Error(),
			Problems: validation.Problems,
		})
	case errors.As(err, &budget):
		writeJSON(w, http.StatusConflict, apiError{
			Error:   "memory_budget",
			Message: err.Error(),
			Params: map[string]any{
				"instance":  budget.Instance,
				"requested": instance.FormatMemory(budget.Requested),
				"free":      instance.FormatMemory(max64(0, budget.Budget-budget.Committed)),
				"budget":    instance.FormatMemory(budget.Budget),
				"committed": instance.FormatMemory(budget.Committed),
			},
		})
	case errors.As(err, &port):
		writeJSON(w, http.StatusConflict, apiError{
			Error:   "port_taken",
			Message: err.Error(),
			Params: map[string]any{
				"port":  port.Port,
				"proto": port.Proto,
				"owner": port.Owner,
			},
		})
	case errors.Is(err, duckdns.ErrInvalidDomain):
		writeJSON(w, http.StatusUnprocessableEntity, apiError{
			Error:   "invalid_domain",
			Message: err.Error(),
			Params:  map[string]any{"suffix": duckdns.Suffix},
		})
	case errors.Is(err, duckdns.ErrRejected):
		writeJSON(w, http.StatusUnprocessableEntity, apiError{Error: "dns_rejected", Message: err.Error()})
	case errors.As(err, &unreachable):
		writeJSON(w, http.StatusConflict, apiError{
			Error:   "dns_unreachable",
			Message: err.Error(),
			Params:  map[string]any{"detail": unreachable.Detail},
		})
	case errors.Is(err, duckdns.ErrUnreachable):
		writeJSON(w, http.StatusConflict, apiError{Error: "dns_unreachable", Message: err.Error()})
	case errors.Is(err, manager.ErrNoToken):
		writeJSON(w, http.StatusConflict, apiError{Error: "dns_token_missing", Message: err.Error()})
	case errors.Is(err, manager.ErrDNSDisabled):
		writeJSON(w, http.StatusConflict, apiError{Error: "dns_disabled", Message: err.Error()})
	case errors.As(err, &dnsTaken):
		writeJSON(w, http.StatusConflict, apiError{
			Error:   "dns_taken",
			Message: err.Error(),
			Params: map[string]any{
				"domain":   dnsTaken.Hostname,
				"instance": dnsTaken.Instance,
			},
		})
	case errors.Is(err, manager.ErrDNSTaken):
		writeJSON(w, http.StatusConflict, apiError{Error: "dns_taken", Message: err.Error()})
	case errors.As(err, &dockerErr):
		writeJSON(w, http.StatusConflict, apiError{
			Error:   "docker_failed",
			Message: dockerErr.Error(),
			Params:  map[string]any{"detail": dockerDetail(dockerErr)},
		})
	default:
		slog.Error("erro não tratado", "err", err)
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "internal", Message: err.Error()})
	}
}

func dockerDetail(e *dockerx.Error) string {
	if e.Stderr != "" {
		return e.Stderr
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return ""
}

func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func badRequest(w http.ResponseWriter, msg string) {
	writeJSON(w, http.StatusBadRequest, apiError{Error: "bad_request", Message: msg})
}

func decodeJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		badRequest(w, "corpo inválido: "+err.Error())
		return false
	}
	return true
}
