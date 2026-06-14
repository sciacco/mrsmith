package raenad

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
	"github.com/sciacco/mrsmith/internal/platform/hubspot"
)

const quoteEventHubSpotStageTransition = "hubspot_stage_transition"

// HubSpotStageClient is the HubSpot subset needed by the explicit dealstage
// transition path.
type HubSpotStageClient interface {
	GetDealStage(ctx context.Context, dealID string) (*hubspot.DealStage, error)
	UpdateDeal(ctx context.Context, dealID string, properties map[string]any) (*hubspot.CRMObject, error)
}

type hubSpotStageConflictResponse struct {
	Error  string                    `json:"error"`
	Remote hubSpotStageRemoteDetails `json:"remote"`
}

type hubSpotStageRemoteDetails struct {
	PipelineID     string  `json:"pipeline_id"`
	PipelineLabel  *string `json:"pipeline_label,omitempty"`
	DealstageID    string  `json:"dealstage_id"`
	DealstageLabel *string `json:"dealstage_label,omitempty"`
}

func (h *Handler) handleStageTransition(w http.ResponseWriter, r *http.Request) {
	if !h.requireMistra(w) || !h.requireConfigDB(w) || !h.requireHubSpotStage(w) {
		return
	}
	quoteID, ok := quoteIDFromRequest(w, r)
	if !ok {
		return
	}
	req, ok := decodeStageTransitionRequest(w, r)
	if !ok {
		return
	}

	quote, err := h.loadQuote(r.Context(), h.deps.Mistra, quoteID)
	if errors.Is(err, sql.ErrNoRows) {
		httputil.Error(w, http.StatusNotFound, "quote_not_found")
		return
	}
	if err != nil {
		h.dbFailure(w, r, "quote_stage_load", err, "quote_id", quoteID)
		return
	}
	if quote.AuthoringStatus != authoringStatusReady {
		httputil.Error(w, http.StatusBadRequest, "quote_not_ready")
		return
	}
	dealID := strings.TrimSpace(deref(quote.HubSpotDealID))
	if dealID == "" {
		httputil.Error(w, http.StatusBadRequest, "hubspot_deal_missing")
		return
	}

	cfg, ok := h.readHubSpotDealPipelineConfig(r.Context())
	if !ok {
		httputil.Error(w, http.StatusServiceUnavailable, "raenad_config_not_configured")
		return
	}
	target, ok, err := h.stageSnapshot(r.Context(), cfg.PipelineID, req.TargetDealstageID)
	if err != nil {
		h.dbFailure(w, r, "quote_stage_target_validate", err, "quote_id", quoteID)
		return
	}
	if !ok {
		httputil.Error(w, http.StatusBadRequest, "validation_failed")
		return
	}

	remote, err := h.deps.HubSpotStage.GetDealStage(r.Context(), dealID)
	if err != nil {
		h.hubSpotStageFailure(w, "get_deal_stage", err, "quote_id", quoteID, "hubspot_deal_id", dealID)
		return
	}
	remotePipelineID := strings.TrimSpace(remote.Pipeline)
	remoteDealstageID := strings.TrimSpace(remote.Dealstage)
	if remoteDealstageID != req.ExpectedDealstageID {
		h.writeStageConflict(w, r.Context(), remotePipelineID, remoteDealstageID)
		return
	}

	if _, err := h.deps.HubSpotStage.UpdateDeal(r.Context(), dealID, map[string]any{"dealstage": target.DealstageID}); err != nil {
		h.hubSpotStageFailure(w, "update_deal_stage", err, "quote_id", quoteID, "hubspot_deal_id", dealID)
		return
	}

	actor := requestActor(r)
	tx, err := h.deps.Mistra.BeginTx(r.Context(), nil)
	if err != nil {
		h.dbFailure(w, r, "quote_stage_begin", err, "quote_id", quoteID)
		return
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(r.Context(), `
		UPDATE raenad.quote
		SET updated_by = $1,
			hubspot_pipeline_id = $2,
			hubspot_pipeline_label = $3,
			hubspot_dealstage_id = $4,
			hubspot_dealstage_label = $5
		WHERE id = $6`,
		actor.Subject, target.PipelineID, target.PipelineLabel, target.DealstageID, target.DealstageLabel, quoteID)
	if err != nil {
		h.dbFailure(w, r, "quote_stage_snapshot_update", err, "quote_id", quoteID)
		return
	}
	if err := h.appendQuoteEvent(r.Context(), tx, quoteID, quoteEventHubSpotStageTransition, actor.Subject, map[string]any{
		"hubspot_deal_id":         dealID,
		"expected_dealstage_id":   req.ExpectedDealstageID,
		"previous_pipeline_id":    remotePipelineID,
		"previous_dealstage_id":   remoteDealstageID,
		"target_pipeline_id":      target.PipelineID,
		"target_pipeline_label":   target.PipelineLabel,
		"target_dealstage_id":     target.DealstageID,
		"target_dealstage_label":  target.DealstageLabel,
		"authoring_status":        quote.AuthoringStatus,
		"hubspot_sync_status":     quote.HubSpotSyncStatus,
		"quote_owned_fields_sent": false,
		"quote_update_enqueued":   false,
		"ready_to_draft_demotion": false,
	}); err != nil {
		h.dbFailure(w, r, "quote_stage_event", err, "quote_id", quoteID)
		return
	}
	if err := tx.Commit(); err != nil {
		h.dbFailure(w, r, "quote_stage_commit", err, "quote_id", quoteID)
		return
	}

	h.respondQuoteByID(w, r, quoteID, http.StatusOK)
}

func decodeStageTransitionRequest(w http.ResponseWriter, r *http.Request) (stageTransitionRequest, bool) {
	defer r.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_payload")
		return stageTransitionRequest{}, false
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var req stageTransitionRequest
	if err := decoder.Decode(&req); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_payload")
		return stageTransitionRequest{}, false
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		httputil.Error(w, http.StatusBadRequest, "invalid_payload")
		return stageTransitionRequest{}, false
	}
	req.ExpectedDealstageID = strings.TrimSpace(req.ExpectedDealstageID)
	req.TargetDealstageID = strings.TrimSpace(req.TargetDealstageID)
	if req.ExpectedDealstageID == "" || req.TargetDealstageID == "" {
		httputil.Error(w, http.StatusBadRequest, "validation_failed")
		return stageTransitionRequest{}, false
	}
	return req, true
}

func (h *Handler) stageSnapshot(ctx context.Context, pipelineID, stageID string) (stageSnapshotResolved, bool, error) {
	var stageLabel, pipelineLabel sql.NullString
	err := h.deps.Mistra.QueryRowContext(ctx, `
		SELECT s.label, p.label
		FROM loader.hubs_stages s
		LEFT JOIN loader.hubs_pipeline p ON p.id = s.pipeline
		WHERE s.pipeline = $1 AND s.id = $2`, pipelineID, stageID).Scan(&stageLabel, &pipelineLabel)
	if errors.Is(err, sql.ErrNoRows) {
		return stageSnapshotResolved{}, false, nil
	}
	if err != nil {
		return stageSnapshotResolved{}, false, err
	}
	return stageSnapshotResolved{
		PipelineID:     pipelineID,
		PipelineLabel:  nullTrimmedStringPtr(pipelineLabel),
		DealstageID:    stageID,
		DealstageLabel: nullTrimmedStringPtr(stageLabel),
	}, true, nil
}

func (h *Handler) writeStageConflict(w http.ResponseWriter, ctx context.Context, pipelineID, dealstageID string) {
	remote := hubSpotStageRemoteDetails{
		PipelineID:  pipelineID,
		DealstageID: dealstageID,
	}
	if pipelineID != "" && dealstageID != "" {
		if snapshot, ok, err := h.stageSnapshot(ctx, pipelineID, dealstageID); err == nil && ok {
			remote.PipelineLabel = snapshot.PipelineLabel
			remote.DealstageLabel = snapshot.DealstageLabel
		}
	}
	httputil.JSON(w, http.StatusConflict, hubSpotStageConflictResponse{
		Error:  "hubspot_stage_conflict",
		Remote: remote,
	})
}

func (h *Handler) hubSpotStageFailure(w http.ResponseWriter, operation string, err error, attrs ...any) {
	args := []any{
		"operation", "raenad_hubspot_stage_" + operation,
	}
	args = append(args, safeHubSpotStageErrorAttrs(err)...)
	args = append(args, attrs...)
	h.logger.Warn("hubspot stage request failed", args...)
	httputil.Error(w, http.StatusBadGateway, "hubspot_request_failed")
}

func safeHubSpotStageErrorAttrs(err error) []any {
	attrs := []any{"error_kind", "hubspot_request_failed"}
	var apiErr *hubspot.APIError
	if errors.As(err, &apiErr) {
		attrs = append(attrs, "hubspot_status_code", apiErr.StatusCode)
	}
	return attrs
}
