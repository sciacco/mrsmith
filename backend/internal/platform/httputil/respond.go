package httputil

import (
	"encoding/json"
	"net/http"

	"github.com/sciacco/mrsmith/internal/platform/logging"
)

func JSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func Error(w http.ResponseWriter, status int, message string) {
	JSON(w, status, map[string]string{"error": message})
}

func InternalError(w http.ResponseWriter, r *http.Request, err error, message string, attrs ...any) {
	args := []any{"error", err}
	args = append(args, attrs...)
	logging.FromContext(r.Context()).Error(message, args...)
	Error(w, http.StatusInternalServerError, "internal_server_error")
}
