package training

import (
	"net/http"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

// registerGroupRoutes registra il CRUD dei gruppi locali (#152, slice 1 del
// task 6): platea delle regole a gruppo e collegamento alle aree di
// competenza.
func (h *handler) registerGroupRoutes(mux *http.ServeMux, protect func(http.Handler) http.Handler) {
	mux.Handle("GET /training/v1/groups", protect(h.requireStore(http.HandlerFunc(h.handleListGroups))))
	mux.Handle("POST /training/v1/groups", protect(h.requireStore(http.HandlerFunc(h.handleUpsertGroup))))
	mux.Handle("PUT /training/v1/groups/{id}", protect(h.requireStore(http.HandlerFunc(h.handleUpsertGroup))))
	mux.Handle("PUT /training/v1/groups/{id}/members", protect(h.requireStore(http.HandlerFunc(h.handleReplaceGroupMembers))))
	mux.Handle("DELETE /training/v1/groups/{id}", protect(h.requireStore(http.HandlerFunc(h.handleDeleteGroup))))
}

func (h *handler) handleListGroups(w http.ResponseWriter, r *http.Request) {
	h.handleRead(w, r, "training.list_groups", func() (any, error) {
		groups, err := h.store.ListGroups(r.Context())
		return GroupListResponse{Groups: groups}, err
	})
}

func (h *handler) handleUpsertGroup(w http.ResponseWriter, r *http.Request) {
	h.handleUpsert(w, r, func(principal Principal, id string) (ActionResponse, error) {
		input, ok := decodeJSONBody[GroupInput](w, r)
		if !ok {
			return ActionResponse{}, nil
		}
		return h.store.UpsertGroup(r.Context(), principal, id, input)
	})
}

func (h *handler) handleReplaceGroupMembers(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principalOrUnauthorized(w, r)
	if !ok {
		return
	}
	input, ok := decodeJSONBody[GroupMembersInput](w, r)
	if !ok {
		return
	}
	response, err := h.store.ReplaceGroupMembers(r.Context(), principal, r.PathValue("id"), input)
	if err != nil {
		h.writeActionError(w, r, err, "training.replace_group_members")
		return
	}
	httputil.JSON(w, http.StatusOK, response)
}

func (h *handler) handleDeleteGroup(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principalOrUnauthorized(w, r)
	if !ok {
		return
	}
	response, err := h.store.DeleteGroup(r.Context(), principal, r.PathValue("id"))
	if err != nil {
		h.writeActionError(w, r, err, "training.delete_group")
		return
	}
	httputil.JSON(w, http.StatusOK, response)
}
