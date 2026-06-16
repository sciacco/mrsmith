package binocolo

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

func (h *Handler) handleListProvinces(w http.ResponseWriter, r *http.Request) {
	if h.provinceCache != nil {
		entry, err := h.provinceCache.GetValidProvinceCache(r.Context(), time.Now().UTC())
		if err != nil {
			h.binocoloCacheFailure(w, r, err)
			return
		}
		if entry != nil {
			httputil.JSON(w, http.StatusOK, json.RawMessage(entry.Response))
			return
		}
	}

	if !h.requireOpenAPIIT(w) {
		return
	}

	if h.provinceCache == nil {
		h.fetchListProvinces(w, r)
		return
	}

	var response json.RawMessage
	var upstreamErr error
	err := h.provinceCache.WithProvinceCacheLock(r.Context(), func(ctx context.Context) error {
		entry, err := h.provinceCache.GetValidProvinceCache(ctx, time.Now().UTC())
		if err != nil {
			return err
		}
		if entry != nil {
			response = entry.Response
			return nil
		}

		result, err := h.openapiit.CAP().ListProvinces(ctx)
		if err != nil {
			upstreamErr = err
			return nil
		}
		raw, err := json.Marshal(result)
		if err != nil {
			return err
		}
		fetchedAt := time.Now().UTC()
		if err := h.provinceCache.UpsertProvinceCache(ctx, provinceCacheWrite{
			Response:  raw,
			FetchedAt: fetchedAt,
			ExpiresAt: fetchedAt.Add(provinceCacheTTL),
		}); err != nil {
			return err
		}
		response = raw
		return nil
	})
	if err != nil {
		h.binocoloCacheFailure(w, r, err)
		return
	}
	if upstreamErr != nil {
		h.openAPIITFailure(w, r, "list_provinces", upstreamErr)
		return
	}

	httputil.JSON(w, http.StatusOK, json.RawMessage(response))
}

func (h *Handler) fetchListProvinces(w http.ResponseWriter, r *http.Request) {
	result, err := h.openapiit.CAP().ListProvinces(r.Context())
	if err != nil {
		h.openAPIITFailure(w, r, "list_provinces", err)
		return
	}

	httputil.JSON(w, http.StatusOK, result)
}
