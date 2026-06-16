package binocolo

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

func (h *Handler) handleListProvinces(w http.ResponseWriter, r *http.Request) {
	if !h.requireOpenAPIIT(w) {
		return
	}

	_, raw, err := listProvincesWithCache(r.Context(), h.provinceCache, h.openapiit, timeNowUTC)
	if err != nil {
		if errors.Is(err, errProvinceCacheFailure) {
			h.binocoloCacheFailure(w, r, err)
			return
		}
		h.openAPIITFailure(w, r, "list_provinces", err)
		return
	}
	httputil.JSON(w, http.StatusOK, json.RawMessage(raw))
}
