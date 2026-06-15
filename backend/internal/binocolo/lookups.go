package binocolo

import (
	"net/http"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

func (h *Handler) handleListProvinces(w http.ResponseWriter, r *http.Request) {
	if !h.requireOpenAPIIT(w) {
		return
	}

	result, err := h.openapiit.CAP().ListProvinces(r.Context())
	if err != nil {
		h.openAPIITFailure(w, r, "list_provinces", err)
		return
	}

	httputil.JSON(w, http.StatusOK, result)
}
