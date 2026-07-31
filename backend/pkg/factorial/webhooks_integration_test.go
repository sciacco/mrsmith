package factorial

import (
	"strings"
	"testing"
)

// webhooks_integration_test.go exercises the generated ParseWebhook against the
// real payload types emitted into types.gen.go. It lives in package factorial
// (alongside webhooks.gen.go) so it can reference ParseWebhook, the
// WebhookSubscriptionType consts, and the typed payload structs directly. This
// mirrors the T10 integration-test split (runtime tests next to the runtime,
// generator tests in cmd/generate).

// TestParseWebhook_RoundTrip decodes a crafted APIPublicWebhookSubscription
// payload through ParseWebhook for the api_public/webhook_subscription/create
// subscription type and asserts the result is a *APIPublicWebhookSubscription
// with the decoded id. The body is the resource object at the top level (no
// {type,data} envelope), matching how Factorial delivers webhook payloads.
func TestParseWebhook_RoundTrip(t *testing.T) {
	body := []byte(`{"id":"7","target_url":"https://example.com/hook","type":"webhook_subscription","company_id":"42","name":"my sub","enabled":true}`)

	got, err := ParseWebhook("api_public/webhook_subscription/create", body)
	if err != nil {
		t.Fatalf("ParseWebhook: %v", err)
	}
	p, ok := got.(*APIPublicWebhookSubscription)
	if !ok {
		t.Fatalf("ParseWebhook returned %T, want *APIPublicWebhookSubscription", got)
	}
	if p.ID == nil || *p.ID != "7" {
		t.Errorf("decoded ID = %v, want \"7\"", p.ID)
	}
	if p.TargetURL == nil || *p.TargetURL != "https://example.com/hook" {
		t.Errorf("decoded TargetURL = %v, want the crafted URL", p.TargetURL)
	}

	// The same payload schema backs the create/delete/update subscription types;
	// assert those keys map to the same typed pointer too.
	for _, st := range []string{
		"api_public/webhook_subscription/delete",
		"api_public/webhook_subscription/update",
	} {
		got, err := ParseWebhook(st, body)
		if err != nil {
			t.Errorf("ParseWebhook(%q): %v", st, err)
			continue
		}
		if _, ok := got.(*APIPublicWebhookSubscription); !ok {
			t.Errorf("ParseWebhook(%q) returned %T, want *APIPublicWebhookSubscription", st, got)
		}
	}

	// A typed const used in a switch is exercised end-to-end: passing the
	// string value of WebhookAPIPublicWebhookSubscriptionCreate must route to
	// the same case as the literal above.
	if string(WebhookAPIPublicWebhookSubscriptionCreate) != "api_public/webhook_subscription/create" {
		t.Errorf("const value = %q, want api_public/webhook_subscription/create", WebhookAPIPublicWebhookSubscriptionCreate)
	}
}

// TestParseWebhook_UnknownType asserts an unrecognized subscription type yields
// a non-nil error whose message names the type.
func TestParseWebhook_UnknownType(t *testing.T) {
	_, err := ParseWebhook("not/a/real/type", nil)
	if err == nil {
		t.Fatalf("ParseWebhook: want error for unknown type, got nil")
	}
	if !strings.Contains(err.Error(), "unknown webhook subscription type") {
		t.Errorf("error message = %q, want it to contain \"unknown webhook subscription type\"", err.Error())
	}
	if !strings.Contains(err.Error(), "not/a/real/type") {
		t.Errorf("error message = %q, want it to contain the offending type", err.Error())
	}
}

// TestParseWebhook_BadJSON asserts a malformed body for a known type returns the
// decoding error (wrapping the json error), not an unknown-type error.
func TestParseWebhook_BadJSON(t *testing.T) {
	_, err := ParseWebhook("api_public/webhook_subscription/create", []byte("{not json"))
	if err == nil {
		t.Fatalf("ParseWebhook: want error for malformed body, got nil")
	}
	if !strings.Contains(err.Error(), "decoding webhook payload") {
		t.Errorf("error message = %q, want it to contain \"decoding webhook payload\"", err.Error())
	}
}
