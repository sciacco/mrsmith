package raenad

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestPDFExportCreateRequiresReadyQuote(t *testing.T) {
	state := lifecycleMistraState()
	seedQuote(state, authoringStatusDraft, ptr("030"), []raenadTestQuoteLine{validStoredLine(101, 1)})
	mux := http.NewServeMux()
	RegisterRoutes(mux, Deps{
		Mistra:   openRaenadTestDBWithState(t, state),
		ConfigDB: openRaenadTestDBWithState(t, pdfTemplateConfigState()),
		Carbone:  &recordingPDFRenderer{},
	})

	rec := serveRaenadJSON(mux, http.MethodPost, "/aenad/v1/quotes/101/pdf-exports", nil)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "quote_not_ready") {
		t.Fatalf("expected quote_not_ready, got %d body=%q", rec.Code, rec.Body.String())
	}
	if len(state.pdfExports) != 0 {
		t.Fatalf("draft quote should not create pdf export: %#v", state.pdfExports)
	}
}

func TestPDFExportCreateReusesLatestWhenChecksumMatches(t *testing.T) {
	state := readyPDFQuoteState()
	mux := pdfExportMux(t, state, pdfTemplateConfigState(), &recordingPDFRenderer{})

	first := serveRaenadJSON(mux, http.MethodPost, "/aenad/v1/quotes/101/pdf-exports", nil)
	if first.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%q", first.Code, first.Body.String())
	}
	var created pdfExport
	decodeRaenadResponse(t, first, &created)

	second := serveRaenadJSON(mux, http.MethodPost, "/aenad/v1/quotes/101/pdf-exports", nil)
	if second.Code != http.StatusOK {
		t.Fatalf("expected 200 reuse, got %d body=%q", second.Code, second.Body.String())
	}
	var reused pdfExport
	decodeRaenadResponse(t, second, &reused)
	if reused.ID != created.ID || reused.Revision != created.Revision {
		t.Fatalf("expected latest export reuse, created=%#v reused=%#v", created, reused)
	}
	if len(state.pdfExports) != 1 {
		t.Fatalf("expected one stored export, got %#v", state.pdfExports)
	}
}

func TestPDFExportCreateNewRevisionWhenPayloadChangesAndListMarksStale(t *testing.T) {
	state := readyPDFQuoteState()
	mux := pdfExportMux(t, state, pdfTemplateConfigState(), &recordingPDFRenderer{})

	first := serveRaenadJSON(mux, http.MethodPost, "/aenad/v1/quotes/101/pdf-exports", nil)
	if first.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%q", first.Code, first.Body.String())
	}
	firstChecksum := stringValue(state.pdfExports[0].ChecksumSHA256)

	state.lines[101][0].Description = ptr("Changed description")
	state.lines[101][0].LineNet = ptr("30.0000")
	state.quotes[0].TotalNet = "30.0000"
	second := serveRaenadJSON(mux, http.MethodPost, "/aenad/v1/quotes/101/pdf-exports", nil)
	if second.Code != http.StatusCreated {
		t.Fatalf("expected 201 new revision, got %d body=%q", second.Code, second.Body.String())
	}
	if len(state.pdfExports) != 2 {
		t.Fatalf("expected two stored exports, got %#v", state.pdfExports)
	}
	if state.pdfExports[1].Revision != 2 {
		t.Fatalf("expected revision 2, got %#v", state.pdfExports[1])
	}
	if stringValue(state.pdfExports[1].ChecksumSHA256) == firstChecksum {
		t.Fatalf("expected changed checksum, got %q", firstChecksum)
	}

	list := serveRaenadJSON(mux, http.MethodGet, "/aenad/v1/quotes/101/pdf-exports", nil)
	if list.Code != http.StatusOK {
		t.Fatalf("expected 200 list, got %d body=%q", list.Code, list.Body.String())
	}
	var got pdfExportListResponse
	decodeRaenadResponse(t, list, &got)
	if len(got.Items) != 2 || got.Items[0].Revision != 2 || got.Items[0].IsStale || !got.Items[1].IsStale {
		t.Fatalf("expected newest fresh and older stale, got %#v", got.Items)
	}
}

func TestPDFExportCreateStoresMetadataOnly(t *testing.T) {
	state := readyPDFQuoteState()
	renderer := &recordingPDFRenderer{pdf: []byte("%PDF generated")}
	mux := pdfExportMux(t, state, pdfTemplateConfigState(), renderer)

	rec := serveRaenadJSON(mux, http.MethodPost, "/aenad/v1/quotes/101/pdf-exports", nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%q", rec.Code, rec.Body.String())
	}
	if renderer.callCount() != 0 {
		t.Fatalf("create should not call carbone; calls=%d", renderer.callCount())
	}
	if len(state.pdfExports) != 1 {
		t.Fatalf("expected one export, got %#v", state.pdfExports)
	}
	export := state.pdfExports[0]
	if export.ContentType != "application/pdf" || export.Filename == "" || stringValue(export.ChecksumSHA256) == "" {
		t.Fatalf("metadata was not persisted correctly: %#v", export)
	}
	if strings.Contains(export.RenderPayload, "%PDF") {
		t.Fatalf("pdf bytes must not be persisted: %q", export.RenderPayload)
	}
	if !json.Valid([]byte(export.RenderPayload)) {
		t.Fatalf("render payload should be stored as json: %q", export.RenderPayload)
	}
}

func TestPDFExportDownloadRegeneratesFromSavedRenderPayload(t *testing.T) {
	state := readyPDFQuoteState()
	renderer := &recordingPDFRenderer{pdf: []byte("%PDF saved")}
	mux := pdfExportMux(t, state, pdfTemplateConfigState(), renderer)

	rec := serveRaenadJSON(mux, http.MethodPost, "/aenad/v1/quotes/101/pdf-exports", nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%q", rec.Code, rec.Body.String())
	}
	savedPayload := state.pdfExports[0].RenderPayload
	savedFilename := state.pdfExports[0].Filename
	state.lines[101][0].Description = ptr("Live quote changed after export")

	download := serveRaenadJSON(mux, http.MethodGet, "/aenad/v1/quotes/101/pdf-exports/501/download", nil)
	if download.Code != http.StatusOK {
		t.Fatalf("expected 200 download, got %d body=%q", download.Code, download.Body.String())
	}
	if got := download.Header().Get("Content-Type"); got != "application/pdf" {
		t.Fatalf("unexpected content type %q", got)
	}
	if got := download.Header().Get("Content-Disposition"); !strings.Contains(got, savedFilename) {
		t.Fatalf("download should use saved filename %q, got %q", savedFilename, got)
	}
	if download.Body.String() != "%PDF saved" {
		t.Fatalf("unexpected pdf body %q", download.Body.String())
	}
	if renderer.callCount() != 1 || renderer.calls[0].templateID != "tmpl-raenad" {
		t.Fatalf("unexpected carbone calls: %#v", renderer.calls)
	}
	if renderer.calls[0].payloadJSON != savedPayload {
		t.Fatalf("download should use saved payload\nwant=%s\ngot=%s", savedPayload, renderer.calls[0].payloadJSON)
	}
}

func TestPDFExportMissingTemplateReturnsServiceUnavailable(t *testing.T) {
	t.Run("create", func(t *testing.T) {
		state := readyPDFQuoteState()
		mux := pdfExportMux(t, state, &raenadTestState{raenadTestStateData: raenadTestStateData{config: map[string][]byte{}}}, &recordingPDFRenderer{})

		rec := serveRaenadJSON(mux, http.MethodPost, "/aenad/v1/quotes/101/pdf-exports", nil)
		if rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), "raenad_pdf_not_configured") {
			t.Fatalf("expected missing template 503, got %d body=%q", rec.Code, rec.Body.String())
		}
		if len(state.pdfExports) != 0 {
			t.Fatalf("missing template should not insert export: %#v", state.pdfExports)
		}
	})

	t.Run("download", func(t *testing.T) {
		state := readyPDFQuoteState()
		renderer := &recordingPDFRenderer{}
		mux := pdfExportMux(t, state, pdfTemplateConfigState(), renderer)
		rec := serveRaenadJSON(mux, http.MethodPost, "/aenad/v1/quotes/101/pdf-exports", nil)
		if rec.Code != http.StatusCreated {
			t.Fatalf("expected 201, got %d body=%q", rec.Code, rec.Body.String())
		}

		mux = pdfExportMux(t, state, &raenadTestState{raenadTestStateData: raenadTestStateData{config: map[string][]byte{}}}, renderer)
		download := serveRaenadJSON(mux, http.MethodGet, "/aenad/v1/quotes/101/pdf-exports/501/download", nil)
		if download.Code != http.StatusServiceUnavailable || !strings.Contains(download.Body.String(), "raenad_pdf_not_configured") {
			t.Fatalf("expected missing template 503, got %d body=%q", download.Code, download.Body.String())
		}
		if renderer.callCount() != 0 {
			t.Fatalf("download should not call carbone without template config")
		}
	})
}

func TestPDFExportFilenamesAreStableAndReusedOnDownload(t *testing.T) {
	state := readyPDFQuoteState()
	renderer := &recordingPDFRenderer{pdf: []byte("%PDF")}
	mux := pdfExportMux(t, state, pdfTemplateConfigState(), renderer)

	rec := serveRaenadJSON(mux, http.MethodPost, "/aenad/v1/quotes/101/pdf-exports", nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%q", rec.Code, rec.Body.String())
	}
	filename := state.pdfExports[0].Filename
	if filename != "Preventivo AE-101 2026 del 14-06-2026 Customer SpA rev1.pdf" {
		t.Fatalf("unexpected stable filename %q", filename)
	}
	state.quotes[0].Customer.Name = ptr("Changed Customer")

	download := serveRaenadJSON(mux, http.MethodGet, "/aenad/v1/quotes/101/pdf-exports/501/download", nil)
	if got := download.Header().Get("Content-Disposition"); !strings.Contains(got, filename) {
		t.Fatalf("download should reuse stored filename %q, got %q", filename, got)
	}
}

func TestPDFExportDownloadChecksQuoteOwnership(t *testing.T) {
	state := readyPDFQuoteState()
	mux := pdfExportMux(t, state, pdfTemplateConfigState(), &recordingPDFRenderer{})
	rec := serveRaenadJSON(mux, http.MethodPost, "/aenad/v1/quotes/101/pdf-exports", nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%q", rec.Code, rec.Body.String())
	}
	state.pdfExports[0].QuoteID = 202

	download := serveRaenadJSON(mux, http.MethodGet, "/aenad/v1/quotes/101/pdf-exports/501/download", nil)
	if download.Code != http.StatusNotFound || !strings.Contains(download.Body.String(), "pdf_export_not_found") {
		t.Fatalf("expected ownership not found, got %d body=%q", download.Code, download.Body.String())
	}
}

func TestPDFExportListRequiresOnlyMistra(t *testing.T) {
	state := readyPDFQuoteState()
	mux := http.NewServeMux()
	RegisterRoutes(mux, Deps{Mistra: openRaenadTestDBWithState(t, state)})

	rec := serveRaenadJSON(mux, http.MethodGet, "/aenad/v1/quotes/101/pdf-exports", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected list to work without config/carbone, got %d body=%q", rec.Code, rec.Body.String())
	}
}

func TestPDFExportAttachEnqueuesIdempotentlyWithoutRendering(t *testing.T) {
	mistraState := readyPDFQuoteState()
	mistraState.quotes[0].HubSpotDealID = ptr("deal-1")
	mistraState.pdfExports = []raenadTestPDFExport{storedPDFExport(501, 101)}
	configState := pdfTemplateConfigState()
	mux := http.NewServeMux()
	RegisterRoutes(mux, Deps{
		Mistra:   openRaenadTestDBWithState(t, mistraState),
		ConfigDB: openRaenadTestDBWithState(t, configState),
	})

	first := serveRaenadJSON(mux, http.MethodPost, "/aenad/v1/quotes/101/pdf-exports/501/attach", nil)
	if first.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d body=%q", first.Code, first.Body.String())
	}
	second := serveRaenadJSON(mux, http.MethodPost, "/aenad/v1/quotes/101/pdf-exports/501/attach", nil)
	if second.Code != http.StatusAccepted {
		t.Fatalf("expected idempotent 202, got %d body=%q", second.Code, second.Body.String())
	}
	if len(configState.hubspotRequests) != 1 {
		t.Fatalf("expected one deduped attach request, got %#v", configState.hubspotRequests)
	}
	req := configState.hubspotRequests[0]
	if req.Operation != hubSpotOperationAttachPDF ||
		req.EntityType != hubSpotQueueEntityQuotePDFExport ||
		req.EntityID != "501" ||
		req.DedupeKey != hubSpotAttachPDFDedupeKey(101, 501) {
		t.Fatalf("unexpected attach queue request: %#v", req)
	}
	if strings.Contains(req.Payload, "render_payload") || strings.Contains(req.Payload, "%PDF") {
		t.Fatalf("attach queue payload must not contain render payload or pdf bytes: %s", req.Payload)
	}
	for _, want := range []string{`"quote_id":101`, `"export_id":501`, `"export_revision":1`, `"export_filename":"Preventivo saved.pdf"`, `"export_checksum_sha256":"checksum-1"`, `"subject":"user-1"`} {
		if !strings.Contains(req.Payload, want) {
			t.Fatalf("attach queue payload missing %s: %s", want, req.Payload)
		}
	}
	if mistraState.quotes[0].HubSpotSyncStatus != hubSpotSyncStatusPending || mistraState.quotes[0].AuthoringStatus != authoringStatusReady {
		t.Fatalf("attach enqueue must not mutate quote sync/readiness: %#v", mistraState.quotes[0])
	}
}

func TestPDFExportAttachRequiresQuoteHubSpotDeal(t *testing.T) {
	state := readyPDFQuoteState()
	state.pdfExports = []raenadTestPDFExport{storedPDFExport(501, 101)}
	configState := pdfTemplateConfigState()
	mux := http.NewServeMux()
	RegisterRoutes(mux, Deps{
		Mistra:   openRaenadTestDBWithState(t, state),
		ConfigDB: openRaenadTestDBWithState(t, configState),
	})

	rec := serveRaenadJSON(mux, http.MethodPost, "/aenad/v1/quotes/101/pdf-exports/501/attach", nil)
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "quote_hubspot_deal_required") {
		t.Fatalf("expected missing deal conflict, got %d body=%q", rec.Code, rec.Body.String())
	}
	if len(configState.hubspotRequests) != 0 {
		t.Fatalf("missing deal should not enqueue attach: %#v", configState.hubspotRequests)
	}
}

func TestPDFExportAttachChecksExportOwnership(t *testing.T) {
	state := readyPDFQuoteState()
	state.quotes[0].HubSpotDealID = ptr("deal-1")
	state.pdfExports = []raenadTestPDFExport{storedPDFExport(501, 202)}
	configState := pdfTemplateConfigState()
	mux := http.NewServeMux()
	RegisterRoutes(mux, Deps{
		Mistra:   openRaenadTestDBWithState(t, state),
		ConfigDB: openRaenadTestDBWithState(t, configState),
	})

	rec := serveRaenadJSON(mux, http.MethodPost, "/aenad/v1/quotes/101/pdf-exports/501/attach", nil)
	if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), "pdf_export_not_found") {
		t.Fatalf("expected ownership not found, got %d body=%q", rec.Code, rec.Body.String())
	}
	if len(configState.hubspotRequests) != 0 {
		t.Fatalf("ownership failure should not enqueue attach: %#v", configState.hubspotRequests)
	}
}

func readyPDFQuoteState() *raenadTestState {
	state := lifecycleMistraState()
	seedQuote(state, authoringStatusReady, ptr("030"), []raenadTestQuoteLine{validStoredLine(101, 1)})
	return state
}

func pdfTemplateConfigState() *raenadTestState {
	return &raenadTestState{raenadTestStateData: raenadTestStateData{
		config: map[string][]byte{
			"raenad.carbone_quote_template": []byte(`{"template_id":"tmpl-raenad"}`),
		},
	}}
}

func storedPDFExport(id, quoteID int64) raenadTestPDFExport {
	return raenadTestPDFExport{
		ID:                      id,
		QuoteID:                 quoteID,
		Revision:                1,
		Filename:                "Preventivo saved.pdf",
		ContentType:             raenadPDFContentType,
		ChecksumSHA256:          ptr("checksum-1"),
		RenderPayload:           `{"saved":true}`,
		CreatedAt:               time.Date(2026, 6, 14, 15, 0, 0, 0, time.UTC),
		CreatedBy:               "user-1",
		HubSpotAttachmentStatus: "pending",
	}
}

func pdfExportMux(t *testing.T, mistraState, configState *raenadTestState, renderer PDFRenderer) *http.ServeMux {
	t.Helper()
	mux := http.NewServeMux()
	RegisterRoutes(mux, Deps{
		Mistra:   openRaenadTestDBWithState(t, mistraState),
		ConfigDB: openRaenadTestDBWithState(t, configState),
		Carbone:  renderer,
	})
	return mux
}

type recordingPDFRenderer struct {
	pdf   []byte
	calls []recordingPDFCall
}

type recordingPDFCall struct {
	templateID  string
	payloadJSON string
}

func (r *recordingPDFRenderer) GeneratePDF(_ context.Context, templateID string, payload any) ([]byte, error) {
	raw, _ := json.Marshal(payload)
	r.calls = append(r.calls, recordingPDFCall{templateID: templateID, payloadJSON: string(raw)})
	if r.pdf == nil {
		return []byte("%PDF"), nil
	}
	return r.pdf, nil
}

func (r *recordingPDFRenderer) callCount() int {
	if r == nil {
		return 0
	}
	return len(r.calls)
}
