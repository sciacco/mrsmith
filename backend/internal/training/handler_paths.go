package training

import (
	"net/http"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

// registerPathRoutes registra le route dei percorsi formativi, dei loro
// passi e delle assegnazioni persona-percorso (#161, slice 2 del task 7).
func (h *handler) registerPathRoutes(mux *http.ServeMux, protect func(http.Handler) http.Handler) {
	mux.Handle("GET /training/v1/paths", protect(h.requireStore(http.HandlerFunc(h.handleListPaths))))
	mux.Handle("POST /training/v1/paths", protect(h.requireStore(http.HandlerFunc(h.handleUpsertPath))))
	mux.Handle("GET /training/v1/paths/{id}", protect(h.requireStore(http.HandlerFunc(h.handleGetPath))))
	mux.Handle("PUT /training/v1/paths/{id}", protect(h.requireStore(http.HandlerFunc(h.handleUpsertPath))))
	mux.Handle("PUT /training/v1/paths/{id}/steps", protect(h.requireStore(http.HandlerFunc(h.handleReplacePathSteps))))
	mux.Handle("POST /training/v1/people/{id}/paths", protect(h.requireStore(http.HandlerFunc(h.handleAssignPersonPath))))
	mux.Handle("PUT /training/v1/people/{id}/paths/{pathId}", protect(h.requireStore(http.HandlerFunc(h.handleUpdatePersonPath))))
	mux.Handle("DELETE /training/v1/people/{id}/paths/{pathId}", protect(h.requireStore(http.HandlerFunc(h.handleRemovePersonPath))))
}

func (h *handler) handleListPaths(w http.ResponseWriter, r *http.Request) {
	h.handleRead(w, r, "training.list_paths", func() (any, error) {
		paths, err := h.store.ListPaths(r.Context())
		return PathListResponse{Paths: paths}, err
	})
}

func (h *handler) handleGetPath(w http.ResponseWriter, r *http.Request) {
	h.handleRead(w, r, "training.get_path", func() (any, error) {
		return h.store.GetPathDetail(r.Context(), r.PathValue("id"))
	})
}

func (h *handler) handleUpsertPath(w http.ResponseWriter, r *http.Request) {
	h.handleUpsert(w, r, func(principal Principal, id string) (ActionResponse, error) {
		input, ok := decodeJSONBody[PathInput](w, r)
		if !ok {
			return ActionResponse{}, nil
		}
		return h.store.UpsertPath(r.Context(), principal, id, input)
	})
}

func (h *handler) handleReplacePathSteps(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principalOrUnauthorized(w, r)
	if !ok {
		return
	}
	input, ok := decodeJSONBody[PathStepsInput](w, r)
	if !ok {
		return
	}
	response, err := h.store.ReplacePathSteps(r.Context(), principal, r.PathValue("id"), input)
	if err != nil {
		h.writeActionError(w, r, err, "training.replace_path_steps")
		return
	}
	httputil.JSON(w, http.StatusOK, response)
}

func (h *handler) handleAssignPersonPath(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principalOrUnauthorized(w, r)
	if !ok {
		return
	}
	input, ok := decodeJSONBody[PathAssignmentInput](w, r)
	if !ok {
		return
	}
	response, err := h.store.AssignPersonPath(r.Context(), principal, r.PathValue("id"), input)
	if err != nil {
		h.writeActionError(w, r, err, "training.assign_person_path")
		return
	}
	httputil.JSON(w, http.StatusCreated, response)
}

func (h *handler) handleUpdatePersonPath(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principalOrUnauthorized(w, r)
	if !ok {
		return
	}
	input, ok := decodeJSONBody[PathAssignmentUpdateInput](w, r)
	if !ok {
		return
	}
	response, err := h.store.UpdatePersonPath(r.Context(), principal, r.PathValue("id"), r.PathValue("pathId"), input)
	if err != nil {
		h.writeActionError(w, r, err, "training.update_person_path")
		return
	}
	httputil.JSON(w, http.StatusOK, response)
}

func (h *handler) handleRemovePersonPath(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principalOrUnauthorized(w, r)
	if !ok {
		return
	}
	response, err := h.store.RemovePersonPath(r.Context(), principal, r.PathValue("id"), r.PathValue("pathId"))
	if err != nil {
		h.writeActionError(w, r, err, "training.remove_person_path")
		return
	}
	httputil.JSON(w, http.StatusOK, response)
}
