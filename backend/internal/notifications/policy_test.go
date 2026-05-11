package notifications

import (
	"encoding/json"
	"testing"
	"time"
)

func TestEffectivePolicySchedulesDefaultRDAReminders(t *testing.T) {
	defaultPolicy := json.RawMessage(`{
		"portal": {"enabled": true},
		"email": {
			"enabled": true,
			"steps": [
				{"step": "unread_after_4h", "delay": "4h"},
				{"step": "unread_after_24h", "delay": "24h"},
				{"step": "unread_after_72h", "delay": "72h"}
			]
		}
	}`)
	now := time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC)

	policy, err := effectivePolicy(defaultPolicy, nil)
	if err != nil {
		t.Fatalf("effectivePolicy failed: %v", err)
	}
	specs, err := deliverySpecsForPolicy(policy, now)
	if err != nil {
		t.Fatalf("deliverySpecsForPolicy failed: %v", err)
	}
	if len(specs) != 4 {
		t.Fatalf("expected portal + 3 email specs, got %d: %#v", len(specs), specs)
	}
	if specs[0].Channel != channelPortal || specs[0].PolicyStep != portalCreatedStep || specs[0].Status != deliveryStatusSent {
		t.Fatalf("unexpected portal spec: %#v", specs[0])
	}
	expected := []struct {
		step string
		due  time.Time
	}{
		{"unread_after_4h", now.Add(4 * time.Hour)},
		{"unread_after_24h", now.Add(24 * time.Hour)},
		{"unread_after_72h", now.Add(72 * time.Hour)},
	}
	for i, expected := range expected {
		spec := specs[i+1]
		if spec.Channel != channelEmail || spec.PolicyStep != expected.step || spec.Status != deliveryStatusPending || !spec.DueAt.Equal(expected.due) {
			t.Fatalf("unexpected email spec %d: %#v", i, spec)
		}
	}
}

func TestEffectivePolicyMergesOverrides(t *testing.T) {
	defaultPolicy := json.RawMessage(`{
		"portal": {"enabled": true},
		"email": {
			"enabled": true,
			"steps": [{"step": "unread_after_4h", "delay": "4h"}]
		}
	}`)
	policy, err := effectivePolicy(defaultPolicy, map[string]any{
		"email": map[string]any{"enabled": false},
	})
	if err != nil {
		t.Fatalf("effectivePolicy failed: %v", err)
	}
	specs, err := deliverySpecsForPolicy(policy, time.Now())
	if err != nil {
		t.Fatalf("deliverySpecsForPolicy failed: %v", err)
	}
	if len(specs) != 1 || specs[0].Channel != channelPortal {
		t.Fatalf("expected only portal delivery after email override, got %#v", specs)
	}
}
