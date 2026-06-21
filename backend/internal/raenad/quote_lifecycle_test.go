package raenad

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestQuoteCreatePersistsPaymentLabelFromDBAndReturnsDBTotals(t *testing.T) {
	mistraState := lifecycleMistraState()
	configState := lifecycleConfigState()
	mux := http.NewServeMux()
	RegisterRoutes(mux, Deps{
		Mistra:   openRaenadTestDBWithState(t, mistraState),
		ConfigDB: openRaenadTestDBWithState(t, configState),
	})

	rec := serveRaenadJSON(mux, http.MethodPost, "/aenad/v1/quotes", validSavePayload(map[string]any{
		"payment_method_code":  "030",
		"payment_method_label": "Client supplied label",
	}))
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%q", rec.Code, rec.Body.String())
	}

	var got quoteResponse
	decodeRaenadResponse(t, rec, &got)
	if got.Payment.MethodCode == nil || *got.Payment.MethodCode != "030" {
		t.Fatalf("payment code not persisted: %#v", got.Payment)
	}
	if got.Payment.MethodLabel == nil || *got.Payment.MethodLabel != "Bonifico DB" {
		t.Fatalf("payment label should come from DB, got %#v", got.Payment.MethodLabel)
	}
	if got.TotalNet != "20.0000" || got.TotalVAT != "4.4000" || got.TotalGross != "24.4000" {
		t.Fatalf("expected DB-owned totals from persisted lines, got net=%s vat=%s gross=%s", got.TotalNet, got.TotalVAT, got.TotalGross)
	}
	if strings.Contains(rec.Body.String(), "pdf_exports") || strings.Contains(rec.Body.String(), "render_payload") || strings.Contains(rec.Body.String(), "is_stale") {
		t.Fatalf("slice 5 response must not expose PDF metadata: %s", rec.Body.String())
	}
	if len(configState.hubspotRequests) != 1 {
		t.Fatalf("expected create enqueue, got %#v", configState.hubspotRequests)
	}
	req := configState.hubspotRequests[0]
	if req.Operation != hubSpotOperationCreateDeal || req.DedupeKey != "raenad:quote:101:deal:create" {
		t.Fatalf("unexpected create queue request: %#v", req)
	}
	if !strings.Contains(req.Payload, `"source_quote_updated_at"`) {
		t.Fatalf("create payload should carry source quote version: %s", req.Payload)
	}
	if !hasRaenadEvent(mistraState, 101, quoteEventHubSpotCreateEnqueued) {
		t.Fatalf("expected create enqueue event, got %#v", mistraState.events[101])
	}
	assertRaenadQueryContains(t, mistraState, "loader.erp_metodi_pagamento")
}

func TestQuoteCreateEnqueueFailureKeepsCommittedQuoteRecoverable(t *testing.T) {
	mistraState := lifecycleMistraState()
	configState := lifecycleConfigState()
	configState.failHubSpotEnqueue = true
	mux := http.NewServeMux()
	RegisterRoutes(mux, Deps{
		Mistra:   openRaenadTestDBWithState(t, mistraState),
		ConfigDB: openRaenadTestDBWithState(t, configState),
	})

	rec := serveRaenadJSON(mux, http.MethodPost, "/aenad/v1/quotes", validSavePayload(nil))
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected committed create response despite enqueue failure, got %d body=%q", rec.Code, rec.Body.String())
	}
	var got quoteResponse
	decodeRaenadResponse(t, rec, &got)
	if got.ID != 101 || got.HubSpotSyncStatus != hubSpotSyncStatusPending {
		t.Fatalf("quote should be recoverable and pending, got %#v", got.quoteSummary)
	}
	if len(configState.hubspotRequests) != 0 {
		t.Fatalf("queue insert should have failed, got %#v", configState.hubspotRequests)
	}
	if !hasRaenadEvent(mistraState, 101, quoteEventHubSpotEnqueueFailed) {
		t.Fatalf("expected enqueue failure event, got %#v", mistraState.events[101])
	}
}

func TestQuoteCreateRejectsClientOwnedTotals(t *testing.T) {
	mistraState := lifecycleMistraState()
	mux := http.NewServeMux()
	RegisterRoutes(mux, Deps{
		Mistra:   openRaenadTestDBWithState(t, mistraState),
		ConfigDB: openRaenadTestDBWithState(t, lifecycleConfigState()),
	})

	rec := serveRaenadJSON(mux, http.MethodPost, "/aenad/v1/quotes", validSavePayload(map[string]any{
		"total_net": "999.0000",
	}))
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "invalid_payload") {
		t.Fatalf("expected invalid_payload, got %d body=%q", rec.Code, rec.Body.String())
	}
	if len(mistraState.quotes) != 0 {
		t.Fatalf("quote should not be inserted when client sends totals: %#v", mistraState.quotes)
	}
}

func TestQuoteCreateRejectsInvalidDecimalBeforeWrite(t *testing.T) {
	mistraState := lifecycleMistraState()
	mux := http.NewServeMux()
	RegisterRoutes(mux, Deps{
		Mistra:   openRaenadTestDBWithState(t, mistraState),
		ConfigDB: openRaenadTestDBWithState(t, lifecycleConfigState()),
	})

	rec := serveRaenadJSON(mux, http.MethodPost, "/aenad/v1/quotes", validSavePayload(map[string]any{
		"lines": []map[string]any{{
			"line_type":  "item",
			"qta":        "1.00000",
			"unit_price": "10.0000",
			"cod_iva":    "22",
		}},
	}))
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "invalid_payload") {
		t.Fatalf("expected invalid_payload, got %d body=%q", rec.Code, rec.Body.String())
	}
	if len(mistraState.quotes) != 0 || mistraState.begins != 0 {
		t.Fatalf("invalid decimal should fail before DB write, quotes=%#v begins=%d", mistraState.quotes, mistraState.begins)
	}
}

func TestQuoteCreateRollsBackWhenLineSaveFails(t *testing.T) {
	mistraState := lifecycleMistraState()
	mistraState.failLineInsert = true
	mux := http.NewServeMux()
	RegisterRoutes(mux, Deps{
		Mistra:   openRaenadTestDBWithState(t, mistraState),
		ConfigDB: openRaenadTestDBWithState(t, lifecycleConfigState()),
	})

	rec := serveRaenadJSON(mux, http.MethodPost, "/aenad/v1/quotes", validSavePayload(nil))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d body=%q", rec.Code, rec.Body.String())
	}
	if len(mistraState.quotes) != 0 || mistraState.commits != 0 || mistraState.rollbacks == 0 {
		t.Fatalf("expected rolled-back transaction, quotes=%#v commits=%d rollbacks=%d", mistraState.quotes, mistraState.commits, mistraState.rollbacks)
	}
}

func TestQuoteReadyValidationAndSuccess(t *testing.T) {
	t.Run("fails without payment", func(t *testing.T) {
		state := lifecycleMistraState()
		seedQuote(state, authoringStatusDraft, nil, []raenadTestQuoteLine{validStoredLine(101, 1)})
		mux := http.NewServeMux()
		RegisterRoutes(mux, Deps{Mistra: openRaenadTestDBWithState(t, state)})

		rec := serveRaenadJSON(mux, http.MethodPost, "/aenad/v1/quotes/101/ready", nil)
		if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "validation_failed") {
			t.Fatalf("expected validation failure, got %d body=%q", rec.Code, rec.Body.String())
		}
	})

	t.Run("fails without valid item line", func(t *testing.T) {
		state := lifecycleMistraState()
		seedQuote(state, authoringStatusDraft, ptr("030"), []raenadTestQuoteLine{{ID: 1, QuoteID: 101, Position: 1, LineType: lineTypeDescription, Description: ptr("Only text")}})
		mux := http.NewServeMux()
		RegisterRoutes(mux, Deps{Mistra: openRaenadTestDBWithState(t, state)})

		rec := serveRaenadJSON(mux, http.MethodPost, "/aenad/v1/quotes/101/ready", nil)
		if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "validation_failed") {
			t.Fatalf("expected validation failure, got %d body=%q", rec.Code, rec.Body.String())
		}
	})

	t.Run("marks ready with valid payment and item", func(t *testing.T) {
		state := lifecycleMistraState()
		seedQuote(state, authoringStatusDraft, ptr("030"), []raenadTestQuoteLine{validStoredLine(101, 1)})
		mux := http.NewServeMux()
		RegisterRoutes(mux, Deps{Mistra: openRaenadTestDBWithState(t, state)})

		rec := serveRaenadJSON(mux, http.MethodPost, "/aenad/v1/quotes/101/ready", nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d body=%q", rec.Code, rec.Body.String())
		}
		var got quoteResponse
		decodeRaenadResponse(t, rec, &got)
		if got.AuthoringStatus != authoringStatusReady {
			t.Fatalf("expected ready quote, got %#v", got.AuthoringStatus)
		}
		if len(got.Events) != 1 || got.Events[0].EventType != "marked_ready" {
			t.Fatalf("ready event not appended: %#v", got.Events)
		}
		if got.Payment.MethodLabel == nil || *got.Payment.MethodLabel != "Bonifico DB" {
			t.Fatalf("ready should refresh payment label from DB: %#v", got.Payment)
		}
	})
}

func TestQuoteUpdateReadyToDraftDetection(t *testing.T) {
	t.Run("internal notes only keeps ready", func(t *testing.T) {
		state := lifecycleMistraState()
		seedQuote(state, authoringStatusReady, ptr("030"), []raenadTestQuoteLine{validStoredLine(101, 1)})
		mux := http.NewServeMux()
		RegisterRoutes(mux, Deps{Mistra: openRaenadTestDBWithState(t, state)})

		rec := serveRaenadJSON(mux, http.MethodPut, "/aenad/v1/quotes/101", validSavePayload(map[string]any{
			"internal_notes": "Operator-only note",
		}))
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d body=%q", rec.Code, rec.Body.String())
		}
		var got quoteResponse
		decodeRaenadResponse(t, rec, &got)
		if got.AuthoringStatus != authoringStatusReady {
			t.Fatalf("internal notes only should not demote ready quote, got %q", got.AuthoringStatus)
		}
	})

	t.Run("commercial change demotes ready to draft", func(t *testing.T) {
		state := lifecycleMistraState()
		seedQuote(state, authoringStatusReady, ptr("030"), []raenadTestQuoteLine{validStoredLine(101, 1)})
		mux := http.NewServeMux()
		RegisterRoutes(mux, Deps{Mistra: openRaenadTestDBWithState(t, state)})

		rec := serveRaenadJSON(mux, http.MethodPut, "/aenad/v1/quotes/101", validSavePayload(map[string]any{
			"description": "Changed commercial title",
		}))
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d body=%q", rec.Code, rec.Body.String())
		}
		var got quoteResponse
		decodeRaenadResponse(t, rec, &got)
		if got.AuthoringStatus != authoringStatusDraft {
			t.Fatalf("commercial change should demote ready quote, got %q", got.AuthoringStatus)
		}
	})
}

func TestQuoteUpdateRelevantChangeEnqueuesAndMarksSyncPending(t *testing.T) {
	state := lifecycleMistraState()
	seedQuote(state, authoringStatusReady, ptr("030"), []raenadTestQuoteLine{validStoredLine(101, 1)})
	state.quotes[0].HubSpotSyncStatus = hubSpotSyncStatusSucceeded
	state.quotes[0].HubSpotSyncError = ptr("old error")
	syncedAt := time.Date(2026, 6, 14, 14, 0, 0, 0, time.UTC)
	state.quotes[0].HubSpotSyncedAt = &syncedAt
	state.quotes[0].HubSpotDealID = ptr("deal-1")
	configState := lifecycleConfigState()
	mux := http.NewServeMux()
	RegisterRoutes(mux, Deps{
		Mistra:   openRaenadTestDBWithState(t, state),
		ConfigDB: openRaenadTestDBWithState(t, configState),
	})

	rec := serveRaenadJSON(mux, http.MethodPut, "/aenad/v1/quotes/101", validSavePayload(map[string]any{
		"description": "Changed commercial title",
	}))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%q", rec.Code, rec.Body.String())
	}
	if state.quotes[0].HubSpotSyncStatus != hubSpotSyncStatusPending || state.quotes[0].HubSpotSyncError != nil || state.quotes[0].HubSpotSyncedAt != nil {
		t.Fatalf("relevant update should reset sync state, got status=%q error=%v synced=%v", state.quotes[0].HubSpotSyncStatus, state.quotes[0].HubSpotSyncError, state.quotes[0].HubSpotSyncedAt)
	}
	if len(configState.hubspotRequests) != 1 {
		t.Fatalf("expected update enqueue, got %#v", configState.hubspotRequests)
	}
	req := configState.hubspotRequests[0]
	if req.Operation != hubSpotOperationUpdateDeal || req.DedupeKey != "raenad:quote:101:deal:update" {
		t.Fatalf("unexpected update request: %#v", req)
	}
	if !strings.Contains(req.Payload, `"source_quote_updated_at"`) {
		t.Fatalf("update payload should carry source quote version: %s", req.Payload)
	}
	if strings.Contains(req.Payload, "hubspot_dealstage_id") {
		t.Fatalf("update payload must not carry dealstage update instruction: %s", req.Payload)
	}
	if !hasRaenadEvent(state, 101, quoteEventHubSpotUpdateEnqueued) {
		t.Fatalf("expected update enqueue event, got %#v", state.events[101])
	}
}

func TestQuoteUpdateInternalNotesOnlyDoesNotEnqueueOrResetSync(t *testing.T) {
	state := lifecycleMistraState()
	seedQuote(state, authoringStatusReady, ptr("030"), []raenadTestQuoteLine{validStoredLine(101, 1)})
	state.quotes[0].HubSpotSyncStatus = hubSpotSyncStatusSucceeded
	state.quotes[0].HubSpotSyncError = ptr("old error")
	syncedAt := time.Date(2026, 6, 14, 14, 0, 0, 0, time.UTC)
	state.quotes[0].HubSpotSyncedAt = &syncedAt
	configState := lifecycleConfigState()
	mux := http.NewServeMux()
	RegisterRoutes(mux, Deps{
		Mistra:   openRaenadTestDBWithState(t, state),
		ConfigDB: openRaenadTestDBWithState(t, configState),
	})

	rec := serveRaenadJSON(mux, http.MethodPut, "/aenad/v1/quotes/101", validSavePayload(map[string]any{
		"internal_notes": "Operator-only note",
	}))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%q", rec.Code, rec.Body.String())
	}
	if state.quotes[0].HubSpotSyncStatus != hubSpotSyncStatusSucceeded || state.quotes[0].HubSpotSyncError == nil || state.quotes[0].HubSpotSyncedAt == nil {
		t.Fatalf("internal notes only should preserve sync state, got status=%q error=%v synced=%v", state.quotes[0].HubSpotSyncStatus, state.quotes[0].HubSpotSyncError, state.quotes[0].HubSpotSyncedAt)
	}
	if len(configState.hubspotRequests) != 0 {
		t.Fatalf("internal notes only should not enqueue, got %#v", configState.hubspotRequests)
	}
}

func TestHubSpotRetrySelectsCreateOrUpdateWithoutHubSpotDependency(t *testing.T) {
	t.Run("missing deal id re-enqueues create", func(t *testing.T) {
		state := lifecycleMistraState()
		seedQuote(state, authoringStatusDraft, ptr("030"), []raenadTestQuoteLine{validStoredLine(101, 1)})
		configState := lifecycleConfigState()
		mux := http.NewServeMux()
		RegisterRoutes(mux, Deps{
			Mistra:   openRaenadTestDBWithState(t, state),
			ConfigDB: openRaenadTestDBWithState(t, configState),
		})

		rec := serveRaenadJSON(mux, http.MethodPost, "/aenad/v1/quotes/101/hubspot/retry", nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d body=%q", rec.Code, rec.Body.String())
		}
		if len(configState.hubspotRequests) != 1 || configState.hubspotRequests[0].Operation != hubSpotOperationCreateDeal {
			t.Fatalf("expected create retry request without HubSpot client, got %#v", configState.hubspotRequests)
		}
		if !hasRaenadEvent(state, 101, quoteEventHubSpotRetryEnqueued) {
			t.Fatalf("expected retry event, got %#v", state.events[101])
		}
	})

	t.Run("existing deal id re-enqueues update", func(t *testing.T) {
		state := lifecycleMistraState()
		seedQuote(state, authoringStatusDraft, ptr("030"), []raenadTestQuoteLine{validStoredLine(101, 1)})
		state.quotes[0].HubSpotDealID = ptr("deal-1")
		configState := lifecycleConfigState()
		mux := http.NewServeMux()
		RegisterRoutes(mux, Deps{
			Mistra:   openRaenadTestDBWithState(t, state),
			ConfigDB: openRaenadTestDBWithState(t, configState),
		})

		rec := serveRaenadJSON(mux, http.MethodPost, "/aenad/v1/quotes/101/hubspot/retry", nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d body=%q", rec.Code, rec.Body.String())
		}
		if len(configState.hubspotRequests) != 1 || configState.hubspotRequests[0].Operation != hubSpotOperationUpdateDeal {
			t.Fatalf("expected update retry request without HubSpot client, got %#v", configState.hubspotRequests)
		}
	})
}

func TestHubSpotQueueStoreDedupeAndStatusHandling(t *testing.T) {
	t.Run("create dedupe inserts once", func(t *testing.T) {
		state := lifecycleConfigState()
		store := newHubSpotQueueStore(openRaenadTestDBWithState(t, state))
		quote := queueTestQuote("2026-06-14T12:00:00Z")

		if _, err := store.enqueueCreateDeal(context.Background(), quote, actor{Subject: "user-1"}, false); err != nil {
			t.Fatalf("enqueue create: %v", err)
		}
		if _, err := store.enqueueCreateDeal(context.Background(), quote, actor{Subject: "user-1"}, false); err != nil {
			t.Fatalf("enqueue duplicate create: %v", err)
		}
		if len(state.hubspotRequests) != 1 || state.hubspotRequests[0].DedupeKey != hubSpotCreateDealDedupeKey(101) {
			t.Fatalf("expected one deduped create request, got %#v", state.hubspotRequests)
		}
	})

	t.Run("non locked update overwrites latest wins", func(t *testing.T) {
		state := lifecycleConfigState()
		state.hubspotRequests = []raenadTestHubSpotRequest{{
			ID:         77,
			Status:     hubSpotRequestStatusFailed,
			Operation:  hubSpotOperationUpdateDeal,
			EntityType: hubSpotQueueEntityQuote,
			EntityID:   "101",
			DedupeKey:  hubSpotUpdateDealDedupeKey(101),
			Payload:    `{"old":true}`,
			LastError:  ptr("old failure"),
		}}
		store := newHubSpotQueueStore(openRaenadTestDBWithState(t, state))

		if _, err := store.enqueueUpdateDeal(context.Background(), queueTestQuote("2026-06-14T12:05:00Z"), actor{Subject: "user-1"}, false); err != nil {
			t.Fatalf("enqueue update: %v", err)
		}
		req := state.hubspotRequests[0]
		if req.Status != hubSpotRequestStatusPending || req.LastError != nil || strings.Contains(req.Payload, `"old":true`) {
			t.Fatalf("expected latest-wins reset, got %#v", req)
		}
		if !strings.Contains(req.Payload, "2026-06-14T12:05:00Z") {
			t.Fatalf("payload was not overwritten with latest source version: %s", req.Payload)
		}
	})

	t.Run("locked update is not overwritten", func(t *testing.T) {
		lockedAt := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
		state := lifecycleConfigState()
		state.hubspotRequests = []raenadTestHubSpotRequest{{
			ID:        78,
			Status:    hubSpotRequestStatusLocked,
			DedupeKey: hubSpotUpdateDealDedupeKey(101),
			Payload:   `{"locked":true}`,
			LockedAt:  &lockedAt,
			LockedBy:  ptr("worker-1"),
		}}
		store := newHubSpotQueueStore(openRaenadTestDBWithState(t, state))

		result, err := store.enqueueUpdateDeal(context.Background(), queueTestQuote("2026-06-14T12:05:00Z"), actor{Subject: "user-1"}, false)
		if err != nil {
			t.Fatalf("enqueue update: %v", err)
		}
		if result.Status != hubSpotRequestStatusLocked || state.hubspotRequests[0].Payload != `{"locked":true}` {
			t.Fatalf("locked request should remain untouched, result=%#v request=%#v", result, state.hubspotRequests[0])
		}
	})

	t.Run("dead and cancelled normal saves are not rearmed", func(t *testing.T) {
		for _, status := range []string{hubSpotRequestStatusDead, hubSpotRequestStatusCancelled} {
			state := lifecycleConfigState()
			state.hubspotRequests = []raenadTestHubSpotRequest{{
				ID:        79,
				Status:    status,
				DedupeKey: hubSpotUpdateDealDedupeKey(101),
				Payload:   `{"terminal":true}`,
			}}
			store := newHubSpotQueueStore(openRaenadTestDBWithState(t, state))

			if _, err := store.enqueueUpdateDeal(context.Background(), queueTestQuote("2026-06-14T12:05:00Z"), actor{Subject: "user-1"}, false); err != nil {
				t.Fatalf("enqueue update for %s: %v", status, err)
			}
			if state.hubspotRequests[0].Status != status || state.hubspotRequests[0].Payload != `{"terminal":true}` {
				t.Fatalf("normal save should not rearm %s, got %#v", status, state.hubspotRequests[0])
			}
		}
	})

	t.Run("retry reactivates terminal rows", func(t *testing.T) {
		for _, status := range []string{hubSpotRequestStatusDead, hubSpotRequestStatusCancelled} {
			state := lifecycleConfigState()
			state.hubspotRequests = []raenadTestHubSpotRequest{{
				ID:        80,
				Status:    status,
				DedupeKey: hubSpotUpdateDealDedupeKey(101),
				Payload:   `{"terminal":true}`,
			}}
			store := newHubSpotQueueStore(openRaenadTestDBWithState(t, state))

			if _, err := store.enqueueUpdateDeal(context.Background(), queueTestQuote("2026-06-14T12:05:00Z"), actor{Subject: "user-1"}, true); err != nil {
				t.Fatalf("retry update for %s: %v", status, err)
			}
			if state.hubspotRequests[0].Status != hubSpotRequestStatusPending || strings.Contains(state.hubspotRequests[0].Payload, `"terminal":true`) {
				t.Fatalf("retry should rearm %s with latest payload, got %#v", status, state.hubspotRequests[0])
			}
		}
	})
}

func TestQuoteUpdatePaymentValidationUsesDBLabel(t *testing.T) {
	state := lifecycleMistraState()
	seedQuote(state, authoringStatusDraft, ptr("030"), []raenadTestQuoteLine{validStoredLine(101, 1)})
	mux := http.NewServeMux()
	RegisterRoutes(mux, Deps{Mistra: openRaenadTestDBWithState(t, state)})

	rec := serveRaenadJSON(mux, http.MethodPut, "/aenad/v1/quotes/101", validSavePayload(map[string]any{
		"payment_method_code":  "404",
		"payment_method_label": "Missing label",
	}))
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "payment_method_not_found") {
		t.Fatalf("expected payment_method_not_found, got %d body=%q", rec.Code, rec.Body.String())
	}
}

func serveRaenadJSON(mux *http.ServeMux, method, path string, payload any) *httptest.ResponseRecorder {
	var body bytes.Buffer
	if payload != nil {
		_ = json.NewEncoder(&body).Encode(payload)
	}
	req := httptest.NewRequest(method, path, &body)
	req = withRoles(req, "app_aenad_access")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func validSavePayload(overrides map[string]any) map[string]any {
	payload := map[string]any{
		"hubspot_company_id": "company-1",
		"customer": map[string]any{
			"name": "Customer SpA",
		},
		"contact":                map[string]any{},
		"document_date":          "2026-06-14",
		"payment_method_code":    "030",
		"payment_bank_details":   "IBAN",
		"description":            "Quote title",
		"internal_notes":         "Internal",
		"hubspot_contact_id":     nil,
		"hubspot_pipeline_id":    nil,
		"hubspot_dealstage_id":   nil,
		"payment_method_label":   nil,
		"hubspot_pipeline_label": nil,
		"lines": []map[string]any{{
			"line_type":        "item",
			"item_code":        "ITEM-1",
			"item_description": "Item",
			"description":      "Item description",
			"unit_of_measure":  "pz",
			"qta":              "2.0000",
			"unit_price":       "10.0000",
			"cod_iva":          "22",
		}},
	}
	for k, v := range overrides {
		payload[k] = v
	}
	return payload
}

func lifecycleMistraState() *raenadTestState {
	return &raenadTestState{
		paymentMethods: []raenadTestPaymentMethod{
			{Code: "030", Description: "Bonifico DB", Selectable: true},
			{Code: "999", Description: "Hidden", Selectable: false},
		},
		stages: []raenadTestStage{
			{ID: "stage-1", Label: ptr("Initial"), Pipeline: "pipeline-1", DisplayOrder: intPtr(1), PipelineLabel: ptr("Pipeline")},
		},
		lines:         map[int64][]raenadTestQuoteLine{},
		events:        map[int64][]raenadTestQuoteEvent{},
		validVATCodes: map[string]string{"22": "22.0000"},
		nextQuoteID:   100,
	}
}

func lifecycleConfigState() *raenadTestState {
	return &raenadTestState{
		config: map[string][]byte{
			"raenad.hubspot_deal_pipeline": []byte(`{"pipeline_id":"pipeline-1","initial_dealstage_id":"stage-1"}`),
		},
	}
}

func seedQuote(state *raenadTestState, status string, paymentCode *string, lines []raenadTestQuoteLine) {
	now := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
	state.quotes = []raenadTestQuote{{
		ID:                101,
		QuoteNumber:       "AE-101/2026",
		CreatedAt:         now,
		UpdatedAt:         now,
		CreatedBy:         "creator",
		UpdatedBy:         "creator",
		AuthoringStatus:   status,
		HubSpotSyncStatus: hubSpotSyncStatusPending,
		HubSpotCompanyID:  ptr("company-1"),
		Customer: customerSnapshotInput{
			Name: ptr("Customer SpA"),
		},
		DocumentDate: ptr("2026-06-14"),
		Payment: paymentSnapshot{
			MethodCode:  paymentCode,
			MethodLabel: ptr("Bonifico DB"),
			BankDetails: ptr("IBAN"),
		},
		Description:   ptr("Quote title"),
		InternalNotes: ptr("Internal"),
		TotalNet:      "20.0000",
		TotalVAT:      "4.4000",
		TotalGross:    "24.4000",
		TotalPurchase: "0.0000",
		TotalGain:     "0.0000",
	}}
	state.lines = map[int64][]raenadTestQuoteLine{101: lines}
	state.events = map[int64][]raenadTestQuoteEvent{}
	state.nextQuoteID = 101
	state.nextLineID = 1000
}

func validStoredLine(quoteID, id int64) raenadTestQuoteLine {
	return raenadTestQuoteLine{
		ID:              id,
		QuoteID:         quoteID,
		Position:        int(id),
		LineType:        lineTypeItem,
		ItemCode:        ptr("ITEM-1"),
		ItemDescription: ptr("Item"),
		Description:     ptr("Item description"),
		UnitOfMeasure:   ptr("pz"),
		Qta:             ptr("2.0000"),
		UnitPrice:       ptr("10.0000"),
		CodIVA:          ptr("22"),
		LineNet:         ptr("20.0000"),
		LineVAT:         ptr("4.4000"),
		LineGross:       ptr("24.4000"),
	}
}

func queueTestQuote(updatedAt string) quoteResponse {
	return quoteResponse{
		quoteSummary: quoteSummary{
			ID:                101,
			QuoteNumber:       "AE-101/2026",
			UpdatedAt:         updatedAt,
			HubSpotSyncStatus: hubSpotSyncStatusPending,
			HubSpotCompanyID:  ptr("company-1"),
			HubSpotDealID:     ptr("deal-1"),
			CustomerName:      ptr("Customer SpA"),
			DocumentDate:      ptr("2026-06-14"),
			Description:       ptr("Quote title"),
			TotalNet:          "20.0000",
			TotalVAT:          "4.4000",
			TotalGross:        "24.4000",
			TotalPurchase:     "0.0000",
			TotalGain:         "0.0000",
		},
		Customer: customerSnapshot{customerSnapshotInput{Name: ptr("Customer SpA")}},
		Payment:  paymentSnapshot{MethodCode: ptr("030"), MethodLabel: ptr("Bonifico DB")},
		Lines: []quoteLine{{
			quoteLineInput: quoteLineInput{
				Position:    1,
				LineType:    lineTypeItem,
				Description: ptr("Item"),
				Qta:         ptr("2.0000"),
				UnitPrice:   ptr("10.0000"),
				CodIVA:      ptr("22"),
			},
			ID:        1,
			QuoteID:   101,
			LineNet:   ptr("20.0000"),
			LineVAT:   ptr("4.4000"),
			LineGross: ptr("24.4000"),
		}},
	}
}

func hasRaenadEvent(state *raenadTestState, quoteID int64, eventType string) bool {
	for _, event := range state.events[quoteID] {
		if event.EventType == eventType {
			return true
		}
	}
	return false
}
