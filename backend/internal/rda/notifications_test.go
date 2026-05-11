package rda

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sciacco/mrsmith/internal/notifications"
	"github.com/sciacco/mrsmith/internal/platform/arak"
)

func TestSubmitCreatesApprovalNotificationOnlyAfterUpstreamSuccess(t *testing.T) {
	upstream := &rdaNotificationArakState{}
	server := httptest.NewServer(upstream)
	t.Cleanup(server.Close)
	notifier := &fakeRDANotifier{}
	h := &Handler{
		arak:           arak.New(arak.Config{BaseURL: server.URL, TokenURL: server.URL + "/token", ClientID: "client", ClientSecret: "secret"}),
		logger:         slog.Default().With("component", component),
		notifier:       notifier,
		staticDir:      "/static",
		quoteThreshold: 3000,
	}

	req := authedRDARequest(http.MethodPost, "/rda/v1/pos/42/submit", nil)
	req.SetPathValue("id", "42")
	res := httptest.NewRecorder()
	h.handleSubmitPO(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected submit success, got %d body=%s", res.Code, res.Body.String())
	}
	if !upstream.submitted {
		t.Fatalf("upstream submit was not called")
	}
	if len(notifier.notifies) != 1 {
		t.Fatalf("expected one notification, got %#v", notifier.notifies)
	}
	notify := notifier.notifies[0]
	if notify.TypeKey != "rda_approval_requested" ||
		notify.EntityType != "rda_po" ||
		notify.EntityID != "42" ||
		notify.DedupeKey != "rda:po:42:approval:2:requested" {
		t.Fatalf("unexpected notification contract: %#v", notify)
	}
	if notify.DeepLink != "/apps/rda/rda/po/42" {
		t.Fatalf("unexpected deep link: %q", notify.DeepLink)
	}
	if len(notify.Recipients) != 1 || notify.Recipients[0].Email != "approver@example.com" {
		t.Fatalf("unexpected recipients: %#v", notify.Recipients)
	}
}

func TestSubmitFailureDoesNotCreateNotification(t *testing.T) {
	upstream := &rdaNotificationArakState{failSubmit: true}
	server := httptest.NewServer(upstream)
	t.Cleanup(server.Close)
	notifier := &fakeRDANotifier{}
	h := &Handler{
		arak:           arak.New(arak.Config{BaseURL: server.URL, TokenURL: server.URL + "/token", ClientID: "client", ClientSecret: "secret"}),
		logger:         slog.Default().With("component", component),
		notifier:       notifier,
		quoteThreshold: 3000,
	}

	req := authedRDARequest(http.MethodPost, "/rda/v1/pos/42/submit", nil)
	req.SetPathValue("id", "42")
	res := httptest.NewRecorder()
	h.handleSubmitPO(res, req)

	if res.Code != http.StatusInternalServerError {
		t.Fatalf("expected upstream failure, got %d body=%s", res.Code, res.Body.String())
	}
	if len(notifier.notifies) != 0 || len(notifier.resolves) != 0 {
		t.Fatalf("notification side effects should not run on failed submit: notifies=%#v resolves=%#v", notifier.notifies, notifier.resolves)
	}
}

type fakeRDANotifier struct {
	notifies []notifications.NotifyInput
	resolves []notifications.ResolveInput
}

func (n *fakeRDANotifier) Notify(_ context.Context, input notifications.NotifyInput) (notifications.NotifyResult, error) {
	n.notifies = append(n.notifies, input)
	return notifications.NotifyResult{NotificationID: 1, RecipientCount: len(input.Recipients), Created: true}, nil
}

func (n *fakeRDANotifier) Resolve(_ context.Context, input notifications.ResolveInput) error {
	n.resolves = append(n.resolves, input)
	return nil
}

type rdaNotificationArakState struct {
	submitted  bool
	failSubmit bool
}

func (s *rdaNotificationArakState) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.URL.Path == "/token" {
		_, _ = w.Write([]byte(`{"access_token":"arak-token","expires_in":3600}`))
		return
	}
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/arak/rda/v1/po/42":
		_ = json.NewEncoder(w).Encode(s.poDetail())
	case r.Method == http.MethodPost && r.URL.Path == "/arak/rda/v1/po/42/submit":
		if s.failSubmit {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":"submit failed"}`))
			return
		}
		s.submitted = true
		_, _ = w.Write([]byte(`{"ok":true}`))
	default:
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"not found"}`))
	}
}

func (s *rdaNotificationArakState) poDetail() map[string]any {
	if !s.submitted {
		return map[string]any{
			"id":          42,
			"code":        "RDA-42",
			"state":       "DRAFT",
			"requester":   map[string]any{"email": "user@example.com"},
			"rows":        []map[string]any{{"id": 1, "type": "good", "qty": 1, "price": "10", "total_price": "10"}},
			"attachments": []map[string]any{},
			"total_price": "10",
		}
	}
	return map[string]any{
		"id":                     42,
		"code":                   "RDA-42",
		"state":                  "PENDING_APPROVAL",
		"current_approval_level": 2,
		"requester":              map[string]any{"email": "user@example.com"},
		"rows":                   []map[string]any{{"id": 1, "type": "good", "qty": 1, "price": "10", "total_price": "10"}},
		"approvers":              []map[string]any{{"level": 2, "user": map[string]any{"email": "approver@example.com"}}},
		"total_price":            "10",
	}
}
