package raenad

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/sciacco/mrsmith/internal/platform/hubspot"
)

func TestHubSpotQueueWorkerRuntimeDisabled(t *testing.T) {
	state := hubSpotWorkerState()
	delete(state.config, "raenad.hubspot_queue_worker")
	state.hubspotRequests = []raenadTestHubSpotRequest{workerRequest(1, hubSpotOperationCreateDeal, 101, `{"quote_id":101}`)}

	stats, err := newTestHubSpotQueueWorker(t, state, &fakeHubSpotDealClient{}).ProcessOnce(context.Background())
	if err != nil {
		t.Fatalf("process once: %v", err)
	}
	if !stats.ConfigDisabled || stats.Claimed != 0 {
		t.Fatalf("expected disabled worker with no claims, got %+v", stats)
	}
	if state.hubspotRequests[0].Status != hubSpotRequestStatusPending || state.hubspotRequests[0].AttemptCount != 0 {
		t.Fatalf("disabled worker touched request: %#v", state.hubspotRequests[0])
	}
}

func TestHubSpotQueueWorkerClaimScopeLeavesUnsupportedRowsUntouched(t *testing.T) {
	state := hubSpotWorkerState()
	seedQuote(state, authoringStatusReady, ptr("030"), []raenadTestQuoteLine{validStoredLine(101, 1)})
	state.hubspotRequests = []raenadTestHubSpotRequest{
		workerRequest(1, hubSpotOperationCreateDeal, 101, payloadJSON(t, hubSpotOperationCreateDeal, state.quotes[0], "actor@example.com")),
		{ID: 2, Status: hubSpotRequestStatusPending, Operation: "hubspot.attach_pdf", EntityType: hubSpotQueueEntityQuote, EntityID: "101", DedupeKey: "pdf", Payload: `{}`},
		{ID: 3, Status: hubSpotRequestStatusPending, Operation: hubSpotOperationCreateDeal, EntityType: "other.entity", EntityID: "101", DedupeKey: "other", Payload: `{}`},
	}
	hs := &fakeHubSpotDealClient{createdID: "deal-1"}

	stats, err := newTestHubSpotQueueWorker(t, state, hs).ProcessOnce(context.Background())
	if err != nil {
		t.Fatalf("process once: %v", err)
	}
	if stats.Claimed != 1 || stats.Succeeded != 1 {
		t.Fatalf("expected one scoped claim, got %+v", stats)
	}
	if state.hubspotRequests[1].Status != hubSpotRequestStatusPending || state.hubspotRequests[1].AttemptCount != 0 {
		t.Fatalf("unsupported operation was touched: %#v", state.hubspotRequests[1])
	}
	if state.hubspotRequests[2].Status != hubSpotRequestStatusPending || state.hubspotRequests[2].AttemptCount != 0 {
		t.Fatalf("unsupported entity was touched: %#v", state.hubspotRequests[2])
	}
}

func TestClaimDueHubSpotRequestsIncludesAttachAndExcludesUnsupported(t *testing.T) {
	state := hubSpotWorkerState()
	now := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
	state.hubspotRequests = []raenadTestHubSpotRequest{
		workerAttachRequest(1, 101, 501, `{"quote_id":101,"export_id":501}`),
		{ID: 2, Status: hubSpotRequestStatusPending, Operation: hubSpotOperationAttachPDF, EntityType: hubSpotQueueEntityQuote, EntityID: "101", DedupeKey: "wrong-entity", Payload: `{}`, NextAttemptAt: &now},
		{ID: 3, Status: hubSpotRequestStatusPending, Operation: "hubspot.unsupported", EntityType: hubSpotQueueEntityQuotePDFExport, EntityID: "501", DedupeKey: "wrong-op", Payload: `{}`, NextAttemptAt: &now},
	}
	db := openRaenadTestDBWithState(t, state)
	conn, err := db.Conn(context.Background())
	if err != nil {
		t.Fatalf("conn: %v", err)
	}
	defer conn.Close()

	claims, err := claimDueHubSpotRequests(context.Background(), conn, 10, "claim-worker")
	if err != nil {
		t.Fatalf("claim due: %v", err)
	}
	if len(claims) != 1 || claims[0].ID != 1 || claims[0].Operation != hubSpotOperationAttachPDF {
		t.Fatalf("expected only attach pdf claim, got %#v", claims)
	}
	if state.hubspotRequests[0].Status != hubSpotRequestStatusLocked || state.hubspotRequests[0].AttemptCount != 1 {
		t.Fatalf("supported attach was not claimed: %#v", state.hubspotRequests[0])
	}
	if state.hubspotRequests[1].Status != hubSpotRequestStatusPending || state.hubspotRequests[2].Status != hubSpotRequestStatusPending {
		t.Fatalf("unsupported requests were touched: %#v", state.hubspotRequests)
	}
}

func TestHubSpotQueueWorkerSuccessfulCreateDeal(t *testing.T) {
	state := hubSpotWorkerState()
	seedQuote(state, authoringStatusReady, ptr("030"), []raenadTestQuoteLine{validStoredLine(101, 1)})
	state.quotes[0].HubSpotContactID = ptr("contact-1")
	state.quotes[0].HubSpotPipelineID = ptr("pipeline-1")
	state.quotes[0].HubSpotDealstageID = ptr("stage-1")
	state.hubspotRequests = []raenadTestHubSpotRequest{
		workerRequest(1, hubSpotOperationCreateDeal, 101, payloadJSON(t, hubSpotOperationCreateDeal, state.quotes[0], "actor@example.com")),
	}
	hs := &fakeHubSpotDealClient{createdID: "deal-1"}

	stats, err := newTestHubSpotQueueWorker(t, state, hs).ProcessOnce(context.Background())
	if err != nil {
		t.Fatalf("process once: %v", err)
	}
	if stats.Succeeded != 1 || len(hs.createProps) != 1 {
		t.Fatalf("expected successful create, stats=%+v create_calls=%d", stats, len(hs.createProps))
	}
	props := hs.createProps[0]
	if props["dealname"] != "AE-101/2026 - Customer SpA" || props["amount"] != "20.0000" || props["closedate"] != "2026-07-14" {
		t.Fatalf("unexpected create properties: %#v", props)
	}
	if props["hubspot_owner_id"] != "owner-1" || props["pipeline"] != "pipeline-1" || props["dealstage"] != "stage-1" {
		t.Fatalf("missing create owner/stage properties: %#v", props)
	}
	if len(hs.createAssocs[0]) != 2 {
		t.Fatalf("expected company and contact associations, got %#v", hs.createAssocs[0])
	}
	if state.quotes[0].HubSpotSyncStatus != hubSpotSyncStatusSucceeded || deref(state.quotes[0].HubSpotDealID) != "deal-1" {
		t.Fatalf("quote not marked succeeded: %#v", state.quotes[0])
	}
	if !hasRaenadEvent(state, 101, quoteEventHubSpotSyncSucceeded) {
		t.Fatalf("expected success event")
	}
}

func TestHubSpotQueueWorkerUpdateDealOmitsStageProperties(t *testing.T) {
	state := hubSpotWorkerState()
	seedQuote(state, authoringStatusReady, ptr("030"), []raenadTestQuoteLine{validStoredLine(101, 1)})
	state.quotes[0].HubSpotDealID = ptr("deal-1")
	state.quotes[0].HubSpotContactID = ptr("contact-1")
	state.quotes[0].HubSpotPipelineID = ptr("pipeline-1")
	state.quotes[0].HubSpotDealstageID = ptr("stage-1")
	state.hubspotRequests = []raenadTestHubSpotRequest{
		workerRequest(1, hubSpotOperationUpdateDeal, 101, payloadJSON(t, hubSpotOperationUpdateDeal, state.quotes[0], "actor@example.com")),
	}
	hs := &fakeHubSpotDealClient{}

	stats, err := newTestHubSpotQueueWorker(t, state, hs).ProcessOnce(context.Background())
	if err != nil {
		t.Fatalf("process once: %v", err)
	}
	if stats.Succeeded != 1 || len(hs.updateProps) != 1 {
		t.Fatalf("expected successful update, stats=%+v update_calls=%d", stats, len(hs.updateProps))
	}
	props := hs.updateProps[0]
	if _, ok := props["pipeline"]; ok {
		t.Fatalf("update must not include pipeline: %#v", props)
	}
	if _, ok := props["dealstage"]; ok {
		t.Fatalf("update must not include dealstage: %#v", props)
	}
	if len(hs.assocCompanies) != 1 || len(hs.assocContacts) != 1 {
		t.Fatalf("expected refreshed associations, companies=%#v contacts=%#v", hs.assocCompanies, hs.assocContacts)
	}
}

func TestHubSpotQueueWorkerDefersUpdateWithoutDealID(t *testing.T) {
	state := hubSpotWorkerState()
	seedQuote(state, authoringStatusReady, ptr("030"), []raenadTestQuoteLine{validStoredLine(101, 1)})
	state.hubspotRequests = []raenadTestHubSpotRequest{
		workerRequest(1, hubSpotOperationUpdateDeal, 101, payloadJSON(t, hubSpotOperationUpdateDeal, state.quotes[0], "actor@example.com")),
	}
	hs := &fakeHubSpotDealClient{}

	stats, err := newTestHubSpotQueueWorker(t, state, hs).ProcessOnce(context.Background())
	if err != nil {
		t.Fatalf("process once: %v", err)
	}
	if stats.Retried != 1 || len(hs.updateProps) != 0 {
		t.Fatalf("expected deferred update without hubspot call, stats=%+v updates=%d", stats, len(hs.updateProps))
	}
	req := state.hubspotRequests[0]
	if req.Status != hubSpotRequestStatusPending || req.LastError == nil || !strings.Contains(*req.LastError, "deal id missing") {
		t.Fatalf("request not deferred correctly: %#v", req)
	}
}

func TestHubSpotQueueWorkerSuccessfulAttachPDF(t *testing.T) {
	state := hubSpotWorkerState()
	seedQuote(state, authoringStatusReady, ptr("030"), []raenadTestQuoteLine{validStoredLine(101, 1)})
	state.quotes[0].HubSpotDealID = ptr("deal-1")
	state.quotes[0].HubSpotSyncStatus = hubSpotSyncStatusSucceeded
	state.pdfExports = []raenadTestPDFExport{storedPDFExport(501, 101)}
	state.hubspotRequests = []raenadTestHubSpotRequest{
		workerAttachRequest(1, 101, 501, attachPayloadJSON(t, state.quotes[0], state.pdfExports[0], "actor@example.com")),
	}
	renderer := &recordingPDFRenderer{pdf: []byte("%PDF saved")}
	hs := &fakeHubSpotDealClient{uploadID: "file-1", noteID: "note-1"}
	beforeStatus := state.quotes[0].HubSpotSyncStatus
	beforeDealID := deref(state.quotes[0].HubSpotDealID)

	stats, err := newTestHubSpotQueueWorkerWithRenderer(t, state, hs, renderer).ProcessOnce(context.Background())
	if err != nil {
		t.Fatalf("process once: %v", err)
	}
	if stats.Succeeded != 1 || len(hs.uploads) != 1 || len(hs.notes) != 1 {
		t.Fatalf("expected successful attach, stats=%+v uploads=%d notes=%d", stats, len(hs.uploads), len(hs.notes))
	}
	if renderer.callCount() != 1 || renderer.calls[0].templateID != "tmpl-raenad" || renderer.calls[0].payloadJSON != `{"saved":true}` {
		t.Fatalf("worker should render saved payload with raenad template, calls=%#v", renderer.calls)
	}
	upload := hs.uploads[0]
	if upload.filename != "Preventivo saved.pdf" || upload.folderPath != hubSpotPDFExportFolderPath || string(upload.content) != "%PDF saved" || upload.options["access"] != "PRIVATE" {
		t.Fatalf("unexpected upload call: %#v", upload)
	}
	note := hs.notes[0]
	if note.TargetObjectType != hubspot.ObjectTypeDeal || note.TargetObjectID != "deal-1" || note.AssociationTypeID != hubspot.AssocTypeNoteToDeal {
		t.Fatalf("unexpected note association: %#v", note)
	}
	if len(note.AttachmentIDs) != 1 || note.AttachmentIDs[0] != "file-1" {
		t.Fatalf("unexpected note attachment ids: %#v", note.AttachmentIDs)
	}
	export := state.pdfExports[0]
	if export.HubSpotAttachmentStatus != "attached" ||
		deref(export.HubSpotFileID) != "file-1" ||
		deref(export.HubSpotNoteID) != "note-1" ||
		deref(export.HubSpotDealID) != "deal-1" ||
		export.HubSpotAttachedAt == nil ||
		export.HubSpotError != nil {
		t.Fatalf("export not marked attached: %#v", export)
	}
	if state.quotes[0].HubSpotSyncStatus != beforeStatus || deref(state.quotes[0].HubSpotDealID) != beforeDealID {
		t.Fatalf("attach must not mutate quote sync fields: before=%s/%s after=%#v", beforeStatus, beforeDealID, state.quotes[0])
	}
}

func TestHubSpotQueueWorkerAttachPDFDeadMarksOnlyExportFailed(t *testing.T) {
	state := hubSpotWorkerState()
	seedQuote(state, authoringStatusReady, ptr("030"), []raenadTestQuoteLine{validStoredLine(101, 1)})
	syncedAt := time.Date(2026, 6, 14, 11, 30, 0, 0, time.UTC)
	state.quotes[0].HubSpotDealID = ptr("deal-1")
	state.quotes[0].HubSpotSyncStatus = hubSpotSyncStatusSucceeded
	state.quotes[0].HubSpotSyncedAt = &syncedAt
	state.pdfExports = []raenadTestPDFExport{storedPDFExport(501, 101)}
	req := workerAttachRequest(1, 101, 501, attachPayloadJSON(t, state.quotes[0], state.pdfExports[0], "actor@example.com"))
	req.AttemptCount = 2
	state.hubspotRequests = []raenadTestHubSpotRequest{req}
	hs := &fakeHubSpotDealClient{uploadErr: errors.New("hubspot upload unavailable\nwith detail")}

	stats, err := newTestHubSpotQueueWorker(t, state, hs).ProcessOnce(context.Background())
	if err != nil {
		t.Fatalf("process once: %v", err)
	}
	if stats.Dead != 1 || state.hubspotRequests[0].Status != hubSpotRequestStatusDead {
		t.Fatalf("expected dead attach request, stats=%+v request=%#v", stats, state.hubspotRequests[0])
	}
	if state.pdfExports[0].HubSpotAttachmentStatus != "failed" || state.pdfExports[0].HubSpotError == nil {
		t.Fatalf("export failure not recorded: %#v", state.pdfExports[0])
	}
	if strings.Contains(*state.pdfExports[0].HubSpotError, "\n") {
		t.Fatalf("export error not sanitized: %#v", state.pdfExports[0].HubSpotError)
	}
	if state.quotes[0].HubSpotSyncStatus != hubSpotSyncStatusSucceeded || state.quotes[0].HubSpotSyncError != nil || state.quotes[0].HubSpotSyncedAt == nil {
		t.Fatalf("attach failure must not mutate quote sync fields: %#v", state.quotes[0])
	}
}

func TestHubSpotQueueWorkerAttachPDFMissingTemplateRetries(t *testing.T) {
	state := hubSpotWorkerState()
	delete(state.config, "raenad.carbone_quote_template")
	seedQuote(state, authoringStatusReady, ptr("030"), []raenadTestQuoteLine{validStoredLine(101, 1)})
	state.quotes[0].HubSpotDealID = ptr("deal-1")
	state.quotes[0].HubSpotSyncStatus = hubSpotSyncStatusSucceeded
	state.pdfExports = []raenadTestPDFExport{storedPDFExport(501, 101)}
	state.hubspotRequests = []raenadTestHubSpotRequest{
		workerAttachRequest(1, 101, 501, attachPayloadJSON(t, state.quotes[0], state.pdfExports[0], "actor@example.com")),
	}
	renderer := &recordingPDFRenderer{pdf: []byte("%PDF saved")}
	hs := &fakeHubSpotDealClient{}

	stats, err := newTestHubSpotQueueWorkerWithRenderer(t, state, hs, renderer).ProcessOnce(context.Background())
	if err != nil {
		t.Fatalf("process once: %v", err)
	}
	if stats.Retried != 1 || state.hubspotRequests[0].Status != hubSpotRequestStatusPending {
		t.Fatalf("expected missing template retry, stats=%+v request=%#v", stats, state.hubspotRequests[0])
	}
	if renderer.callCount() != 0 || len(hs.uploads) != 0 || len(hs.notes) != 0 {
		t.Fatalf("missing template should not render/upload/note, calls=%#v uploads=%d notes=%d", renderer.calls, len(hs.uploads), len(hs.notes))
	}
	if state.pdfExports[0].HubSpotAttachmentStatus != "pending" || state.pdfExports[0].HubSpotError != nil {
		t.Fatalf("missing template should not mark export attached/failed: %#v", state.pdfExports[0])
	}
}

func TestHubSpotQueueWorkerOwnerFallbackAndMisconfig(t *testing.T) {
	t.Run("uses fallback owner", func(t *testing.T) {
		state := hubSpotWorkerState()
		state.config["raenad.hubspot_deal_owner"] = []byte(`{"fallback_owner_email":"fallback@example.com"}`)
		state.owners = append(state.owners, raenadTestOwner{ID: "owner-fallback", Email: "fallback@example.com"})
		seedQuote(state, authoringStatusReady, ptr("030"), []raenadTestQuoteLine{validStoredLine(101, 1)})
		state.hubspotRequests = []raenadTestHubSpotRequest{
			workerRequest(1, hubSpotOperationCreateDeal, 101, payloadJSON(t, hubSpotOperationCreateDeal, state.quotes[0], "missing@example.com")),
		}
		hs := &fakeHubSpotDealClient{createdID: "deal-1"}

		stats, err := newTestHubSpotQueueWorker(t, state, hs).ProcessOnce(context.Background())
		if err != nil {
			t.Fatalf("process once: %v", err)
		}
		if stats.Succeeded != 1 || hs.createProps[0]["hubspot_owner_id"] != "owner-fallback" {
			t.Fatalf("fallback owner not used, stats=%+v props=%#v", stats, hs.createProps)
		}
	})

	t.Run("defers when owner unresolved", func(t *testing.T) {
		state := hubSpotWorkerState()
		seedQuote(state, authoringStatusReady, ptr("030"), []raenadTestQuoteLine{validStoredLine(101, 1)})
		state.hubspotRequests = []raenadTestHubSpotRequest{
			workerRequest(1, hubSpotOperationCreateDeal, 101, payloadJSON(t, hubSpotOperationCreateDeal, state.quotes[0], "missing@example.com")),
		}
		hs := &fakeHubSpotDealClient{createdID: "deal-1"}

		stats, err := newTestHubSpotQueueWorker(t, state, hs).ProcessOnce(context.Background())
		if err != nil {
			t.Fatalf("process once: %v", err)
		}
		if stats.Retried != 1 || len(hs.createProps) != 0 {
			t.Fatalf("expected owner misconfig deferral without create, stats=%+v create_calls=%d", stats, len(hs.createProps))
		}
	})
}

func TestHubSpotQueueWorkerStaleLockRecoveryIsScoped(t *testing.T) {
	state := hubSpotWorkerState()
	oldLock := time.Date(2026, 6, 14, 11, 0, 0, 0, time.UTC)
	youngLock := time.Date(2026, 6, 14, 11, 59, 30, 0, time.UTC)
	worker := "worker-a"
	state.hubspotRequests = []raenadTestHubSpotRequest{
		{ID: 1, Status: hubSpotRequestStatusLocked, Operation: hubSpotOperationCreateDeal, EntityType: hubSpotQueueEntityQuote, EntityID: "101", DedupeKey: "stale", Payload: `{}`, LockedAt: &oldLock, LockedBy: &worker},
		{ID: 2, Status: hubSpotRequestStatusLocked, Operation: hubSpotOperationCreateDeal, EntityType: hubSpotQueueEntityQuote, EntityID: "102", DedupeKey: "young", Payload: `{}`, LockedAt: &youngLock, LockedBy: &worker},
		{ID: 3, Status: hubSpotRequestStatusLocked, Operation: hubSpotOperationAttachPDF, EntityType: hubSpotQueueEntityQuotePDFExport, EntityID: "501", DedupeKey: "stale-attach", Payload: `{"quote_id":101,"export_id":501}`, LockedAt: &oldLock, LockedBy: &worker},
		{ID: 4, Status: hubSpotRequestStatusLocked, Operation: hubSpotOperationAttachPDF, EntityType: hubSpotQueueEntityQuote, EntityID: "103", DedupeKey: "unsupported", Payload: `{}`, LockedAt: &oldLock, LockedBy: &worker},
	}

	stats, err := newTestHubSpotQueueWorker(t, state, &fakeHubSpotDealClient{}).ProcessOnce(context.Background())
	if err != nil {
		t.Fatalf("process once: %v", err)
	}
	if stats.Recovered != 2 {
		t.Fatalf("expected two stale recoveries, got %+v", stats)
	}
	if state.hubspotRequests[0].Status != hubSpotRequestStatusPending || state.hubspotRequests[0].LockedAt != nil {
		t.Fatalf("stale supported request not recovered: %#v", state.hubspotRequests[0])
	}
	if state.hubspotRequests[2].Status != hubSpotRequestStatusPending || state.hubspotRequests[2].LockedAt != nil {
		t.Fatalf("stale attach request not recovered: %#v", state.hubspotRequests[2])
	}
	if state.hubspotRequests[1].Status != hubSpotRequestStatusLocked || state.hubspotRequests[3].Status != hubSpotRequestStatusLocked {
		t.Fatalf("young or unsupported locks were touched: %#v", state.hubspotRequests)
	}
}

func TestHubSpotQueueWorkerDeadRequestPropagatesQuoteFailure(t *testing.T) {
	state := hubSpotWorkerState()
	seedQuote(state, authoringStatusReady, ptr("030"), []raenadTestQuoteLine{validStoredLine(101, 1)})
	req := workerRequest(1, hubSpotOperationCreateDeal, 101, payloadJSON(t, hubSpotOperationCreateDeal, state.quotes[0], "actor@example.com"))
	req.AttemptCount = 2
	state.hubspotRequests = []raenadTestHubSpotRequest{req}
	hs := &fakeHubSpotDealClient{createErr: errors.New("hubspot unavailable\nwith detail"), createdID: "deal-1"}

	stats, err := newTestHubSpotQueueWorker(t, state, hs).ProcessOnce(context.Background())
	if err != nil {
		t.Fatalf("process once: %v", err)
	}
	if stats.Dead != 1 || state.hubspotRequests[0].Status != hubSpotRequestStatusDead {
		t.Fatalf("expected dead request, stats=%+v request=%#v", stats, state.hubspotRequests[0])
	}
	if state.quotes[0].HubSpotSyncStatus != hubSpotSyncStatusFailed || state.quotes[0].HubSpotSyncError == nil {
		t.Fatalf("quote failure not propagated: %#v", state.quotes[0])
	}
	if strings.Contains(*state.quotes[0].HubSpotSyncError, "\n") || state.quotes[0].HubSpotSyncedAt != nil {
		t.Fatalf("quote error not sanitized or synced_at not cleared: %#v", state.quotes[0])
	}
	if !hasRaenadEvent(state, 101, quoteEventHubSpotSyncFailed) {
		t.Fatalf("expected failure event")
	}
}

func TestHubSpotQueueWorkerLatestWinsRearmsStaleUpdatePayload(t *testing.T) {
	state := hubSpotWorkerState()
	seedQuote(state, authoringStatusReady, ptr("030"), []raenadTestQuoteLine{validStoredLine(101, 1)})
	state.quotes[0].UpdatedAt = time.Date(2026, 6, 14, 12, 5, 0, 0, time.UTC)
	state.quotes[0].HubSpotDealID = ptr("deal-1")
	req := workerRequest(1, hubSpotOperationUpdateDeal, 101, payloadJSONWithSource(t, hubSpotOperationUpdateDeal, state.quotes[0], "actor@example.com", "2026-06-14T12:00:00Z"))
	state.hubspotRequests = []raenadTestHubSpotRequest{req}
	hs := &fakeHubSpotDealClient{}

	stats, err := newTestHubSpotQueueWorker(t, state, hs).ProcessOnce(context.Background())
	if err != nil {
		t.Fatalf("process once: %v", err)
	}
	if stats.Succeeded != 1 || len(hs.updateProps) != 1 {
		t.Fatalf("expected stale update to process once, stats=%+v updates=%d", stats, len(hs.updateProps))
	}
	if state.hubspotRequests[0].Status != hubSpotRequestStatusPending || state.hubspotRequests[0].AttemptCount != 0 {
		t.Fatalf("request was not rearmed with latest payload: %#v", state.hubspotRequests[0])
	}
	if state.quotes[0].HubSpotSyncStatus != hubSpotSyncStatusPending {
		t.Fatalf("quote should remain pending after latest-wins rearm: %#v", state.quotes[0])
	}
	if !strings.Contains(state.hubspotRequests[0].Payload, `"source_quote_updated_at":"2026-06-14T12:05:00Z"`) {
		t.Fatalf("latest payload not stored: %s", state.hubspotRequests[0].Payload)
	}
}

func TestHubSpotQueueWorkerReconcilerEnqueuesMissingLiveRequest(t *testing.T) {
	state := hubSpotWorkerState()
	seedQuote(state, authoringStatusReady, ptr("030"), []raenadTestQuoteLine{validStoredLine(101, 1)})
	future := time.Date(2026, 6, 14, 13, 0, 0, 0, time.UTC)
	state.hubspotRequests = []raenadTestHubSpotRequest{
		{ID: 9, Status: hubSpotRequestStatusPending, Operation: hubSpotOperationAttachPDF, EntityType: hubSpotQueueEntityQuotePDFExport, EntityID: "501", DedupeKey: "attach-live", Payload: `{"quote_id":101,"export_id":501}`, NextAttemptAt: &future},
	}

	stats, err := newTestHubSpotQueueWorker(t, state, &fakeHubSpotDealClient{}).ProcessOnce(context.Background())
	if err != nil {
		t.Fatalf("process once: %v", err)
	}
	if stats.Reconciled != 1 || len(state.hubspotRequests) != 2 {
		t.Fatalf("expected reconciler enqueue, stats=%+v requests=%#v", stats, state.hubspotRequests)
	}
	req := state.hubspotRequests[1]
	if req.Operation != hubSpotOperationCreateDeal || req.DedupeKey != hubSpotCreateDealDedupeKey(101) {
		t.Fatalf("unexpected reconciled request: %#v", req)
	}
}

func newTestHubSpotQueueWorker(t *testing.T, state *raenadTestState, hs HubSpotDealClient) *HubSpotQueueWorker {
	t.Helper()
	return newTestHubSpotQueueWorkerWithRenderer(t, state, hs, &recordingPDFRenderer{pdf: []byte("%PDF worker")})
}

func newTestHubSpotQueueWorkerWithRenderer(t *testing.T, state *raenadTestState, hs HubSpotDealClient, renderer PDFRenderer) *HubSpotQueueWorker {
	t.Helper()
	db := openRaenadTestDBWithState(t, state)
	worker := NewHubSpotQueueWorker(HubSpotQueueWorkerDeps{
		Mistra:   db,
		Anisetta: db,
		HubSpot:  hs,
		Carbone:  renderer,
	})
	worker.now = func() time.Time { return time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC) }
	worker.workerID = "test-worker"
	return worker
}

func hubSpotWorkerState() *raenadTestState {
	state := lifecycleMistraState()
	state.config = map[string][]byte{
		"raenad.hubspot_queue_worker":   []byte(`{"enabled":true,"interval_seconds":1,"batch_size":10,"max_attempts":3,"backoff_base_seconds":1,"backoff_max_seconds":60,"lock_timeout_seconds":60}`),
		"raenad.carbone_quote_template": []byte(`{"template_id":"tmpl-raenad"}`),
	}
	state.owners = []raenadTestOwner{{ID: "owner-1", Email: "actor@example.com"}}
	return state
}

func workerRequest(id int64, operation string, quoteID int64, payload string) raenadTestHubSpotRequest {
	now := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
	dedupeKey := hubSpotCreateDealDedupeKey(quoteID)
	if operation == hubSpotOperationUpdateDeal {
		dedupeKey = hubSpotUpdateDealDedupeKey(quoteID)
	}
	return raenadTestHubSpotRequest{
		ID:            id,
		Status:        hubSpotRequestStatusPending,
		Operation:     operation,
		EntityType:    hubSpotQueueEntityQuote,
		EntityID:      strconv.FormatInt(quoteID, 10),
		DedupeKey:     dedupeKey,
		Payload:       payload,
		NextAttemptAt: &now,
	}
}

func workerAttachRequest(id, quoteID, exportID int64, payload string) raenadTestHubSpotRequest {
	now := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
	return raenadTestHubSpotRequest{
		ID:            id,
		Status:        hubSpotRequestStatusPending,
		Operation:     hubSpotOperationAttachPDF,
		EntityType:    hubSpotQueueEntityQuotePDFExport,
		EntityID:      strconv.FormatInt(exportID, 10),
		DedupeKey:     hubSpotAttachPDFDedupeKey(quoteID, exportID),
		Payload:       payload,
		NextAttemptAt: &now,
	}
}

func payloadJSON(t *testing.T, operation string, quote raenadTestQuote, actorEmail string) string {
	t.Helper()
	return payloadJSONWithSource(t, operation, quote, actorEmail, formatTimestamp(quote.UpdatedAt))
}

func payloadJSONWithSource(t *testing.T, operation string, quote raenadTestQuote, actorEmail, sourceUpdatedAt string) string {
	t.Helper()
	resp := quoteResponse{
		quoteSummary: quoteSummary{
			ID:                 quote.ID,
			QuoteNumber:        quote.QuoteNumber,
			UpdatedAt:          sourceUpdatedAt,
			HubSpotSyncStatus:  quote.HubSpotSyncStatus,
			HubSpotCompanyID:   quote.HubSpotCompanyID,
			HubSpotContactID:   quote.HubSpotContactID,
			HubSpotDealID:      quote.HubSpotDealID,
			HubSpotPipelineID:  quote.HubSpotPipelineID,
			HubSpotDealstageID: quote.HubSpotDealstageID,
			CustomerName:       quote.Customer.Name,
			DocumentDate:       quote.DocumentDate,
			Description:        quote.Description,
			TotalNet:           quote.TotalNet,
			TotalVAT:           quote.TotalVAT,
			TotalGross:         quote.TotalGross,
			TotalPurchase:      quote.TotalPurchase,
			TotalGain:          quote.TotalGain,
		},
		Customer: customerSnapshot{quote.Customer},
		Contact:  contactSnapshot{quote.Contact},
		Payment:  quote.Payment,
	}
	payload := hubSpotPayload(operation, resp, actor{Subject: "actor", Email: actorEmail, Name: "Actor"})
	payload.SourceQuoteUpdatedAt = sourceUpdatedAt
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	return string(raw)
}

func attachPayloadJSON(t *testing.T, quote raenadTestQuote, export raenadTestPDFExport, actorEmail string) string {
	t.Helper()
	payload := hubSpotQueuePayload{
		Source:                "raenad",
		Operation:             hubSpotOperationAttachPDF,
		QuoteID:               quote.ID,
		QuoteNumber:           quote.QuoteNumber,
		HubSpotDealID:         quote.HubSpotDealID,
		ExportID:              export.ID,
		ExportRevision:        export.Revision,
		ExportFilename:        export.Filename,
		ExportContentType:     export.ContentType,
		ExportChecksumSHA256:  export.ChecksumSHA256,
		SourceExportCreatedAt: formatTimestamp(export.CreatedAt),
		Actor: hubSpotPayloadActor{
			Subject: "actor",
			Email:   actorEmail,
			Name:    "Actor",
		},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal attach payload: %v", err)
	}
	return string(raw)
}

type fakeHubSpotDealClient struct {
	createdID      string
	createErr      error
	updateErr      error
	uploadID       string
	uploadErr      error
	noteID         string
	noteErr        error
	createProps    []map[string]any
	updateProps    []map[string]any
	createAssocs   [][]hubspot.ObjectAssociation
	assocCompanies [][2]string
	assocContacts  [][2]string
	uploads        []fakeHubSpotUpload
	notes          []hubspot.NoteWithAttachmentRequest
}

type fakeHubSpotUpload struct {
	filename   string
	content    []byte
	folderPath string
	options    map[string]any
}

func (f *fakeHubSpotDealClient) CreateDeal(ctx context.Context, properties map[string]any, associations []hubspot.ObjectAssociation) (*hubspot.CRMObject, error) {
	if f.createErr != nil {
		return nil, f.createErr
	}
	f.createProps = append(f.createProps, cloneAnyMap(properties))
	f.createAssocs = append(f.createAssocs, append([]hubspot.ObjectAssociation(nil), associations...))
	id := f.createdID
	if id == "" {
		id = "deal-created"
	}
	return &hubspot.CRMObject{ID: id}, nil
}

func (f *fakeHubSpotDealClient) UpdateDeal(ctx context.Context, dealID string, properties map[string]any) (*hubspot.CRMObject, error) {
	if f.updateErr != nil {
		return nil, f.updateErr
	}
	f.updateProps = append(f.updateProps, cloneAnyMap(properties))
	return &hubspot.CRMObject{ID: dealID}, nil
}

func (f *fakeHubSpotDealClient) AssociateDealToCompany(ctx context.Context, dealID, companyID string) error {
	f.assocCompanies = append(f.assocCompanies, [2]string{dealID, companyID})
	return nil
}

func (f *fakeHubSpotDealClient) AssociateDealToContact(ctx context.Context, dealID, contactID string) error {
	f.assocContacts = append(f.assocContacts, [2]string{dealID, contactID})
	return nil
}

func (f *fakeHubSpotDealClient) UploadFile(ctx context.Context, filename string, content []byte, folderPath string, options map[string]any) (*hubspot.UploadedFile, error) {
	if f.uploadErr != nil {
		return nil, f.uploadErr
	}
	f.uploads = append(f.uploads, fakeHubSpotUpload{
		filename:   filename,
		content:    append([]byte(nil), content...),
		folderPath: folderPath,
		options:    cloneAnyMap(options),
	})
	id := f.uploadID
	if id == "" {
		id = "file-created"
	}
	return &hubspot.UploadedFile{ID: id}, nil
}

func (f *fakeHubSpotDealClient) CreateGenericNoteWithAttachment(ctx context.Context, req hubspot.NoteWithAttachmentRequest) (string, error) {
	if f.noteErr != nil {
		return "", f.noteErr
	}
	copied := req
	copied.AttachmentIDs = append([]string(nil), req.AttachmentIDs...)
	f.notes = append(f.notes, copied)
	id := f.noteID
	if id == "" {
		id = "note-created"
	}
	return id, nil
}

func cloneAnyMap(in map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range in {
		out[k] = v
	}
	return out
}
