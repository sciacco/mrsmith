package raenad

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"github.com/sciacco/mrsmith/internal/platform/hubspot"
)

func TestStageTransitionRouteDependenciesAndValidation(t *testing.T) {
	t.Run("requires config database", func(t *testing.T) {
		state := lifecycleMistraState()
		mux := http.NewServeMux()
		RegisterRoutes(mux, Deps{
			Mistra:       openRaenadTestDBWithState(t, state),
			HubSpotStage: &fakeStageHubSpot{},
		})

		rec := serveRaenadJSON(mux, http.MethodPost, "/aenad/v1/quotes/101/hubspot/stage", stagePayload("stage-1", "stage-2"))
		if rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), "raenad_config_not_configured") {
			t.Fatalf("expected missing config 503, got %d body=%q", rec.Code, rec.Body.String())
		}
	})

	t.Run("rejects invalid quote id", func(t *testing.T) {
		mux := http.NewServeMux()
		RegisterRoutes(mux, Deps{
			Mistra:       openRaenadTestDBWithState(t, lifecycleMistraState()),
			ConfigDB:     openRaenadTestDBWithState(t, lifecycleConfigState()),
			HubSpotStage: &fakeStageHubSpot{},
		})

		rec := serveRaenadJSON(mux, http.MethodPost, "/aenad/v1/quotes/not-a-number/hubspot/stage", stagePayload("stage-1", "stage-2"))
		if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "invalid_quote_id") {
			t.Fatalf("expected invalid_quote_id, got %d body=%q", rec.Code, rec.Body.String())
		}
	})

	t.Run("rejects unknown body fields", func(t *testing.T) {
		mux := http.NewServeMux()
		RegisterRoutes(mux, Deps{
			Mistra:       openRaenadTestDBWithState(t, lifecycleMistraState()),
			ConfigDB:     openRaenadTestDBWithState(t, lifecycleConfigState()),
			HubSpotStage: &fakeStageHubSpot{},
		})

		payload := stagePayload("stage-1", "stage-2")
		payload["dealname"] = "client-owned"
		rec := serveRaenadJSON(mux, http.MethodPost, "/aenad/v1/quotes/101/hubspot/stage", payload)
		if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "invalid_payload") {
			t.Fatalf("expected invalid_payload, got %d body=%q", rec.Code, rec.Body.String())
		}
	})

	t.Run("requires both stage ids", func(t *testing.T) {
		mux := http.NewServeMux()
		RegisterRoutes(mux, Deps{
			Mistra:       openRaenadTestDBWithState(t, lifecycleMistraState()),
			ConfigDB:     openRaenadTestDBWithState(t, lifecycleConfigState()),
			HubSpotStage: &fakeStageHubSpot{},
		})

		rec := serveRaenadJSON(mux, http.MethodPost, "/aenad/v1/quotes/101/hubspot/stage", stagePayload(" ", "stage-2"))
		if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "validation_failed") {
			t.Fatalf("expected validation_failed, got %d body=%q", rec.Code, rec.Body.String())
		}
	})
}

func TestStageTransitionQuotePrerequisites(t *testing.T) {
	t.Run("quote must be ready", func(t *testing.T) {
		state := lifecycleMistraState()
		seedQuote(state, authoringStatusDraft, ptr("030"), []raenadTestQuoteLine{validStoredLine(101, 1)})
		state.quotes[0].HubSpotDealID = ptr("deal-1")
		hs := &fakeStageHubSpot{}
		mux := stageTransitionMux(t, state, lifecycleConfigState(), hs)

		rec := serveRaenadJSON(mux, http.MethodPost, "/aenad/v1/quotes/101/hubspot/stage", stagePayload("stage-1", "stage-2"))
		if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "quote_not_ready") {
			t.Fatalf("expected quote_not_ready, got %d body=%q", rec.Code, rec.Body.String())
		}
		if hs.getCalls != 0 || len(hs.updateProps) != 0 {
			t.Fatalf("hubspot should not be called for not-ready quote, get=%d updates=%#v", hs.getCalls, hs.updateProps)
		}
	})

	t.Run("quote must have hubspot deal", func(t *testing.T) {
		state := lifecycleMistraState()
		seedQuote(state, authoringStatusReady, ptr("030"), []raenadTestQuoteLine{validStoredLine(101, 1)})
		hs := &fakeStageHubSpot{}
		mux := stageTransitionMux(t, state, lifecycleConfigState(), hs)

		rec := serveRaenadJSON(mux, http.MethodPost, "/aenad/v1/quotes/101/hubspot/stage", stagePayload("stage-1", "stage-2"))
		if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "hubspot_deal_missing") {
			t.Fatalf("expected hubspot_deal_missing, got %d body=%q", rec.Code, rec.Body.String())
		}
		if hs.getCalls != 0 || len(hs.updateProps) != 0 {
			t.Fatalf("hubspot should not be called without deal id, get=%d updates=%#v", hs.getCalls, hs.updateProps)
		}
	})
}

func TestStageTransitionRejectsTargetOutsideConfiguredPipeline(t *testing.T) {
	state := lifecycleMistraState()
	state.stages = append(state.stages, raenadTestStage{
		ID:            "stage-other",
		Label:         ptr("Other"),
		Pipeline:      "pipeline-other",
		DisplayOrder:  intPtr(2),
		PipelineLabel: ptr("Other Pipeline"),
	})
	seedReadyQuoteWithDeal(state)
	hs := &fakeStageHubSpot{}
	mux := stageTransitionMux(t, state, lifecycleConfigState(), hs)

	rec := serveRaenadJSON(mux, http.MethodPost, "/aenad/v1/quotes/101/hubspot/stage", stagePayload("stage-1", "stage-other"))
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "validation_failed") {
		t.Fatalf("expected validation_failed for outside pipeline target, got %d body=%q", rec.Code, rec.Body.String())
	}
	if hs.getCalls != 0 || len(hs.updateProps) != 0 {
		t.Fatalf("hubspot should not be called for invalid target, get=%d updates=%#v", hs.getCalls, hs.updateProps)
	}
}

func TestStageTransitionLiveConflictIncludesRemoteStage(t *testing.T) {
	state := lifecycleMistraState()
	state.stages = append(state.stages,
		raenadTestStage{ID: "stage-2", Label: ptr("Target"), Pipeline: "pipeline-1", DisplayOrder: intPtr(2), PipelineLabel: ptr("Pipeline")},
		raenadTestStage{ID: "stage-remote", Label: ptr("Remote"), Pipeline: "pipeline-1", DisplayOrder: intPtr(3), PipelineLabel: ptr("Pipeline")},
	)
	seedReadyQuoteWithDeal(state)
	hs := &fakeStageHubSpot{
		liveStage: &hubspot.DealStage{ID: "deal-1", Pipeline: "pipeline-1", Dealstage: "stage-remote"},
	}
	mux := stageTransitionMux(t, state, lifecycleConfigState(), hs)

	rec := serveRaenadJSON(mux, http.MethodPost, "/aenad/v1/quotes/101/hubspot/stage", stagePayload("stage-1", "stage-2"))
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "hubspot_stage_conflict") {
		t.Fatalf("expected conflict, got %d body=%q", rec.Code, rec.Body.String())
	}
	var got hubSpotStageConflictResponse
	decodeRaenadResponse(t, rec, &got)
	if got.Remote.PipelineID != "pipeline-1" || got.Remote.DealstageID != "stage-remote" {
		t.Fatalf("remote stage not returned: %#v", got.Remote)
	}
	if got.Remote.PipelineLabel == nil || *got.Remote.PipelineLabel != "Pipeline" || got.Remote.DealstageLabel == nil || *got.Remote.DealstageLabel != "Remote" {
		t.Fatalf("remote labels not returned: %#v", got.Remote)
	}
	if len(hs.updateProps) != 0 {
		t.Fatalf("conflict should not update HubSpot, got %#v", hs.updateProps)
	}
}

func TestStageTransitionSuccessUpdatesOnlyStageSnapshot(t *testing.T) {
	state := lifecycleMistraState()
	state.stages = append(state.stages, raenadTestStage{
		ID:            "stage-2",
		Label:         ptr("Target"),
		Pipeline:      "pipeline-1",
		DisplayOrder:  intPtr(2),
		PipelineLabel: ptr("Pipeline"),
	})
	seedReadyQuoteWithDeal(state)
	state.quotes[0].HubSpotSyncStatus = hubSpotSyncStatusSucceeded
	configState := lifecycleConfigState()
	hs := &fakeStageHubSpot{
		liveStage: &hubspot.DealStage{ID: "deal-1", Pipeline: "pipeline-1", Dealstage: "stage-1"},
	}
	mux := stageTransitionMux(t, state, configState, hs)

	rec := serveRaenadJSON(mux, http.MethodPost, "/aenad/v1/quotes/101/hubspot/stage", stagePayload(" stage-1 ", " stage-2 "))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%q", rec.Code, rec.Body.String())
	}
	var got quoteResponse
	decodeRaenadResponse(t, rec, &got)
	if got.AuthoringStatus != authoringStatusReady {
		t.Fatalf("stage transition must not demote ready quote, got %q", got.AuthoringStatus)
	}
	if got.HubSpotSyncStatus != hubSpotSyncStatusSucceeded {
		t.Fatalf("stage transition must not reset sync status, got %q", got.HubSpotSyncStatus)
	}
	if got.HubSpotPipelineID == nil || *got.HubSpotPipelineID != "pipeline-1" || got.HubSpotDealstageID == nil || *got.HubSpotDealstageID != "stage-2" {
		t.Fatalf("snapshot not updated: pipeline=%v stage=%v", got.HubSpotPipelineID, got.HubSpotDealstageID)
	}
	if got.HubSpotPipelineLabel == nil || *got.HubSpotPipelineLabel != "Pipeline" || got.HubSpotDealstageLabel == nil || *got.HubSpotDealstageLabel != "Target" {
		t.Fatalf("snapshot labels not updated: pipeline=%v stage=%v", got.HubSpotPipelineLabel, got.HubSpotDealstageLabel)
	}
	if len(hs.updateProps) != 1 {
		t.Fatalf("expected one HubSpot update, got %#v", hs.updateProps)
	}
	props := hs.updateProps[0]
	if len(props) != 1 || props["dealstage"] != "stage-2" {
		t.Fatalf("stage endpoint must send only dealstage, got %#v", props)
	}
	if len(configState.hubspotRequests) != 0 {
		t.Fatalf("stage transition must not enqueue quote-owned update, got %#v", configState.hubspotRequests)
	}
	if !hasRaenadEvent(state, 101, quoteEventHubSpotStageTransition) {
		t.Fatalf("expected stage transition event, got %#v", state.events[101])
	}
	lastEvent := state.events[101][len(state.events[101])-1]
	if !strings.Contains(lastEvent.Payload, `"quote_update_enqueued":false`) || !strings.Contains(lastEvent.Payload, `"ready_to_draft_demotion":false`) {
		t.Fatalf("event should record no queue and no demotion: %s", lastEvent.Payload)
	}
	if state.quotes[0].AuthoringStatus != authoringStatusReady || state.quotes[0].HubSpotSyncStatus != hubSpotSyncStatusSucceeded {
		t.Fatalf("stored quote status mutated unexpectedly: %#v", state.quotes[0])
	}
}

func TestStageTransitionHubSpotFailureIsSanitized(t *testing.T) {
	state := lifecycleMistraState()
	state.stages = append(state.stages, raenadTestStage{ID: "stage-2", Label: ptr("Target"), Pipeline: "pipeline-1", DisplayOrder: intPtr(2), PipelineLabel: ptr("Pipeline")})
	seedReadyQuoteWithDeal(state)
	hs := &fakeStageHubSpot{getErr: errors.New("upstream leaked detail\nsecret-token")}
	mux := stageTransitionMux(t, state, lifecycleConfigState(), hs)

	rec := serveRaenadJSON(mux, http.MethodPost, "/aenad/v1/quotes/101/hubspot/stage", stagePayload("stage-1", "stage-2"))
	if rec.Code != http.StatusBadGateway || !strings.Contains(rec.Body.String(), "hubspot_request_failed") {
		t.Fatalf("expected stable hubspot failure, got %d body=%q", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "secret-token") {
		t.Fatalf("response leaked hubspot detail: %q", rec.Body.String())
	}
}

func TestStageTransitionHubSpotFailureLogOmitsResponseBody(t *testing.T) {
	state := lifecycleMistraState()
	state.stages = append(state.stages, raenadTestStage{ID: "stage-2", Label: ptr("Target"), Pipeline: "pipeline-1", DisplayOrder: intPtr(2), PipelineLabel: ptr("Pipeline")})
	seedReadyQuoteWithDeal(state)
	hs := &fakeStageHubSpot{getErr: &hubspot.APIError{StatusCode: http.StatusInternalServerError, Body: `{"token":"secret-token","email":"jane@example.com"}`}}

	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	mux := http.NewServeMux()
	RegisterRoutes(mux, Deps{
		Mistra:       openRaenadTestDBWithState(t, state),
		ConfigDB:     openRaenadTestDBWithState(t, lifecycleConfigState()),
		HubSpotStage: hs,
		Logger:       logger,
	})

	rec := serveRaenadJSON(mux, http.MethodPost, "/aenad/v1/quotes/101/hubspot/stage", stagePayload("stage-1", "stage-2"))
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d body=%q", rec.Code, rec.Body.String())
	}
	gotLogs := logs.String()
	if strings.Contains(gotLogs, "secret-token") || strings.Contains(gotLogs, "jane@example.com") {
		t.Fatalf("stage failure log leaked HubSpot body: %q", gotLogs)
	}
	if !strings.Contains(gotLogs, "hubspot_status_code=500") {
		t.Fatalf("stage failure log should retain stable status metadata: %q", gotLogs)
	}
}

func stageTransitionMux(t *testing.T, mistraState, configState *raenadTestState, hs HubSpotStageClient) *http.ServeMux {
	t.Helper()
	mux := http.NewServeMux()
	RegisterRoutes(mux, Deps{
		Mistra:       openRaenadTestDBWithState(t, mistraState),
		ConfigDB:     openRaenadTestDBWithState(t, configState),
		HubSpotStage: hs,
	})
	return mux
}

func seedReadyQuoteWithDeal(state *raenadTestState) {
	seedQuote(state, authoringStatusReady, ptr("030"), []raenadTestQuoteLine{validStoredLine(101, 1)})
	state.quotes[0].HubSpotDealID = ptr("deal-1")
	state.quotes[0].HubSpotPipelineID = ptr("pipeline-1")
	state.quotes[0].HubSpotPipelineLabel = ptr("Pipeline")
	state.quotes[0].HubSpotDealstageID = ptr("stage-1")
	state.quotes[0].HubSpotDealstageLabel = ptr("Initial")
}

func stagePayload(expected, target string) map[string]any {
	return map[string]any{
		"expected_dealstage_id": expected,
		"target_dealstage_id":   target,
	}
}

type fakeStageHubSpot struct {
	liveStage   *hubspot.DealStage
	getErr      error
	updateErr   error
	getCalls    int
	updateIDs   []string
	updateProps []map[string]any
}

func (f *fakeStageHubSpot) GetDealStage(_ context.Context, dealID string) (*hubspot.DealStage, error) {
	f.getCalls++
	if f.getErr != nil {
		return nil, f.getErr
	}
	if f.liveStage != nil {
		cp := *f.liveStage
		if cp.ID == "" {
			cp.ID = dealID
		}
		return &cp, nil
	}
	return &hubspot.DealStage{ID: dealID, Pipeline: "pipeline-1", Dealstage: "stage-1"}, nil
}

func (f *fakeStageHubSpot) UpdateDeal(_ context.Context, dealID string, properties map[string]any) (*hubspot.CRMObject, error) {
	if f.updateErr != nil {
		return nil, f.updateErr
	}
	f.updateIDs = append(f.updateIDs, dealID)
	copied := make(map[string]any, len(properties))
	for k, v := range properties {
		copied[k] = v
	}
	f.updateProps = append(f.updateProps, copied)
	return &hubspot.CRMObject{ID: dealID}, nil
}
