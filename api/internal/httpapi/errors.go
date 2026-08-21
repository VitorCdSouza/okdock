package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/VitorCdSouza/gamedock/api/internal/catalog"
	"github.com/VitorCdSouza/gamedock/api/internal/dockerx"
	"github.com/VitorCdSouza/gamedock/api/internal/manager"
	"github.com/VitorCdSouza/gamedock/api/internal/store"
)

type apiError struct {
	Error    string   `json:"error"`
	Message  string   `json:"message"`
	Problems []string `json:"problems,omitempty"`
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
		validation *catalog.ValidationError
		budget     *manager.ErrBudget
		port       *manager.ErrPortTaken
		dockerErr  *dockerx.Error
	)

	switch {
	case errors.Is(err, store.ErrNotFound):
		writeJSON(w, http.StatusNotFound, apiError{Error: "not_found", Message: err.Error()})
	case errors.Is(err, store.ErrExists):
		writeJSON(w, http.StatusConflict, apiError{Error: "already_exists", Message: err.Error()})
	case errors.As(err, &validation):
		writeJSON(w, http.StatusUnprocessableEntity, apiError{
			Error:    "invalid_fields",
			Message:  "alguns campos não passaram na validação do provedor",
			Problems: validation.Problems,
		})
	case errors.As(err, &budget):
		writeJSON(w, http.StatusConflict, apiError{Error: "memory_budget", Message: err.Error()})
	case errors.As(err, &port):
		writeJSON(w, http.StatusConflict, apiError{Error: "port_taken", Message: err.Error()})
	case errors.As(err, &dockerErr):
		writeJSON(w, http.StatusConflict, apiError{Error: "docker_failed", Message: dockerErr.Error()})
	default:
		slog.Error("erro não tratado", "err", err)
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "internal", Message: err.Error()})
	}
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
