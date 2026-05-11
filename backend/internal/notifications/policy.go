package notifications

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

const (
	channelPortal = "portal"
	channelEmail  = "email"

	deliveryStatusPending   = "pending"
	deliveryStatusSent      = "sent"
	deliveryStatusSkipped   = "skipped"
	deliveryStatusFailed    = "failed"
	deliveryStatusCancelled = "cancelled"

	portalCreatedStep = "portal_created"
)

type DeliveryPolicy struct {
	Portal PortalPolicy `json:"portal"`
	Email  EmailPolicy  `json:"email"`
}

type PortalPolicy struct {
	Enabled bool `json:"enabled"`
}

type EmailPolicy struct {
	Enabled bool              `json:"enabled"`
	Steps   []EmailPolicyStep `json:"steps"`
}

type EmailPolicyStep struct {
	Step  string `json:"step"`
	Delay string `json:"delay"`
}

func effectivePolicy(defaultPolicy json.RawMessage, override map[string]any) (DeliveryPolicy, error) {
	base := map[string]any{}
	if len(defaultPolicy) > 0 {
		if err := json.Unmarshal(defaultPolicy, &base); err != nil {
			return DeliveryPolicy{}, fmt.Errorf("decode default policy: %w", err)
		}
	}
	merged := mergeObjects(base, override)
	raw, err := json.Marshal(merged)
	if err != nil {
		return DeliveryPolicy{}, fmt.Errorf("marshal effective policy: %w", err)
	}
	var policy DeliveryPolicy
	if err := json.Unmarshal(raw, &policy); err != nil {
		return DeliveryPolicy{}, fmt.Errorf("decode effective policy: %w", err)
	}
	return policy, nil
}

func mergeObjects(base map[string]any, override map[string]any) map[string]any {
	out := make(map[string]any, len(base)+len(override))
	for key, value := range base {
		if nested, ok := value.(map[string]any); ok {
			out[key] = mergeObjects(nested, nil)
			continue
		}
		out[key] = value
	}
	for key, value := range override {
		baseNested, baseOK := out[key].(map[string]any)
		overrideNested, overrideOK := value.(map[string]any)
		if baseOK && overrideOK {
			out[key] = mergeObjects(baseNested, overrideNested)
			continue
		}
		out[key] = value
	}
	return out
}

func deliverySpecsForPolicy(policy DeliveryPolicy, now time.Time) ([]DeliverySpec, error) {
	specs := make([]DeliverySpec, 0, 1+len(policy.Email.Steps))
	if policy.Portal.Enabled {
		specs = append(specs, DeliverySpec{
			Channel:    channelPortal,
			PolicyStep: portalCreatedStep,
			Status:     deliveryStatusSent,
			DueAt:      now,
		})
	}
	if !policy.Email.Enabled {
		return specs, nil
	}
	for _, step := range policy.Email.Steps {
		if step.Step == "" {
			return nil, errors.New("email policy step is required")
		}
		delay, err := time.ParseDuration(step.Delay)
		if err != nil {
			return nil, fmt.Errorf("invalid delay for policy step %q: %w", step.Step, err)
		}
		specs = append(specs, DeliverySpec{
			Channel:    channelEmail,
			PolicyStep: step.Step,
			Status:     deliveryStatusPending,
			DueAt:      now.Add(delay),
		})
	}
	return specs, nil
}
