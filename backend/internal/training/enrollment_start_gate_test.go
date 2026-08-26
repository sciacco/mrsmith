package training

import (
	"context"
	"net/http"
	"testing"
)

func TestEnrollmentStartEconomicGate(t *testing.T) {
	t.Run("no coverage allows without fetch", func(t *testing.T) {
		upstream := &poResolverUpstream{}
		h := newPOResolverHandler(upstream)
		if err := h.requireApprovedEnrollmentPurchaseOrders(context.Background(), "people@example.com", nil); err != nil {
			t.Fatalf("gate without coverage: %v", err)
		}
		assertGateRequests(t, upstream, nil)
	})

	t.Run("one approved PO allows start", func(t *testing.T) {
		upstream := &poResolverUpstream{detail: poJSON(7, "PO-7", "APPROVED")}
		h := newPOResolverHandler(upstream)
		if err := h.requireApprovedEnrollmentPurchaseOrders(context.Background(), "people@example.com", []int64{7}); err != nil {
			t.Fatalf("approved gate: %v", err)
		}
		assertGateRequests(t, upstream, []string{"/arak/rda/v1/po/7"})
	})

	t.Run("all covered POs must be approved and are fetched once", func(t *testing.T) {
		upstream := &poResolverUpstream{details: map[int64]string{
			7:  poJSON(7, "PO-7", "APPROVED"),
			42: poJSON(42, "PO-42", "SENT"),
		}}
		h := newPOResolverHandler(upstream)
		if err := h.requireApprovedEnrollmentPurchaseOrders(context.Background(), "people@example.com", []int64{42, 7, 42}); err != nil {
			t.Fatalf("all-approved gate: %v", err)
		}
		assertGateRequests(t, upstream, []string{"/arak/rda/v1/po/7", "/arak/rda/v1/po/42"})
	})

	t.Run("pending rejected and unknown all block deterministically", func(t *testing.T) {
		upstream := &poResolverUpstream{details: map[int64]string{
			7:  poJSON(7, "PO-7", "APPROVED"),
			19: poJSON(19, "PO-19", "FUTURE_STATE"),
			42: poJSON(42, "PO-42", "REJECTED"),
		}}
		h := newPOResolverHandler(upstream)
		err := h.requireApprovedEnrollmentPurchaseOrders(context.Background(), "people@example.com", []int64{42, 7, 19})
		assertPOResolverAppError(t, err, http.StatusConflict, "event_expense_po_not_approved")
		appErr, _ := asAppError(err)
		want := "avvio iscrizione bloccato: PO 19 (PO-19): stato FUTURE_STATE, economico pending; PO 42 (PO-42): stato REJECTED, economico rejected"
		if appErr.message != want {
			t.Fatalf("blocker message = %q, want %q", appErr.message, want)
		}
		assertGateRequests(t, upstream, []string{"/arak/rda/v1/po/7", "/arak/rda/v1/po/19", "/arak/rda/v1/po/42"})
	})
}

func TestRequiresEnrollmentStartGate(t *testing.T) {
	cases := []struct {
		name              string
		current, computed string
		want              bool
	}{
		{"real PATCH start", deliveryPlanned, deliveryInProgress, true},
		{"already in progress is not retroactive", deliveryInProgress, deliveryCompleted, false},
		{"planned directly completed is historical", deliveryPlanned, deliveryCompleted, false},
		{"planned directly partially completed is historical", deliveryPlanned, deliveryPartiallyCompleted, false},
		{"planned directly not attended is historical", deliveryPlanned, deliveryNotAttended, false},
		{"reopen correction is not start", deliveryInProgress, deliveryPlanned, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := requiresEnrollmentStartGate(tc.current, tc.computed); got != tc.want {
				t.Fatalf("requiresEnrollmentStartGate(%q, %q) = %t, want %t", tc.current, tc.computed, got, tc.want)
			}
		})
	}
}

func assertGateRequests(t *testing.T, upstream *poResolverUpstream, wantPaths []string) {
	t.Helper()
	requests := upstream.capturedRequests()
	if len(requests) != len(wantPaths) {
		t.Fatalf("requests = %#v, want paths %#v", requests, wantPaths)
	}
	for i, wantPath := range wantPaths {
		if requests[i].path != wantPath {
			t.Fatalf("request %d path = %q, want %q", i, requests[i].path, wantPath)
		}
		if got := requests[i].header.Get("Requester-Email"); got != "people@example.com" {
			t.Fatalf("request %d Requester-Email = %q", i, got)
		}
	}
}
