package binocolo

import (
	"net/http"
	"strings"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

// Segnalazioni: five handlers over the shared Binocolo access role. Contents
// (PUT) and state (POST …/state) are separate operations so the state change
// stays available while the contents are locked in `chiusa`.

func (h *Handler) handleListSegnalazioni(w http.ResponseWriter, r *http.Request) {
	items, err := h.segnalazioni.list(r.Context())
	if err != nil {
		h.maFailure(w, r, "segnalazione_list", err)
		return
	}
	httputil.JSON(w, http.StatusOK, items)
}

func (h *Handler) handleCreateSegnalazione(w http.ResponseWriter, r *http.Request) {
	var body SegnalazioneWrite
	if err := decodeMABody(r, &body); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_json")
		return
	}
	subject, email := companySearchRefreshActor(r.Context())
	item, err := h.segnalazioni.create(r.Context(), body, subject, email)
	if err != nil {
		h.maFailure(w, r, "segnalazione_create", err)
		return
	}
	httputil.JSON(w, http.StatusCreated, item)
}

func (h *Handler) handleGetSegnalazione(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	item, err := h.segnalazioni.get(r.Context(), id)
	if err != nil {
		h.maFailure(w, r, "segnalazione_get", err, "segnalazione_id", id)
		return
	}
	httputil.JSON(w, http.StatusOK, item)
}

func (h *Handler) handleUpdateSegnalazioneContent(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	var body SegnalazioneWrite
	if err := decodeMABody(r, &body); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_json")
		return
	}
	subject, email := companySearchRefreshActor(r.Context())
	item, err := h.segnalazioni.updateContent(r.Context(), id, body, subject, email)
	if err != nil {
		h.maFailure(w, r, "segnalazione_update", err, "segnalazione_id", id)
		return
	}
	httputil.JSON(w, http.StatusOK, item)
}

func (h *Handler) handleSetSegnalazioneState(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	var body SegnalazioneStateRequest
	if err := decodeMABody(r, &body); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_json")
		return
	}
	subject, email := companySearchRefreshActor(r.Context())
	item, err := h.segnalazioni.setState(r.Context(), id, body.State, subject, email)
	if err != nil {
		h.maFailure(w, r, "segnalazione_set_state", err, "segnalazione_id", id, "state", body.State)
		return
	}
	httputil.JSON(w, http.StatusOK, item)
}
