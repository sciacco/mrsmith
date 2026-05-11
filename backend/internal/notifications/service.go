package notifications

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/mail"
	"strings"
	"time"
)

type Service struct {
	store  Store
	logger *slog.Logger
	now    func() time.Time
}

func NewService(store Store, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{
		store:  store,
		logger: logger.With("component", component),
		now:    time.Now,
	}
}

func (s *Service) Notify(ctx context.Context, input NotifyInput) (NotifyResult, error) {
	if s == nil || s.store == nil {
		return NotifyResult{}, errors.New("notifications store not configured")
	}
	input.TypeKey = strings.TrimSpace(input.TypeKey)
	input.DedupeKey = strings.TrimSpace(input.DedupeKey)
	if input.TypeKey == "" {
		return NotifyResult{}, errors.New("notification type_key is required")
	}
	if input.DedupeKey == "" {
		return NotifyResult{}, errors.New("notification dedupe_key is required")
	}

	notificationType, err := s.store.GetType(ctx, input.TypeKey)
	if err != nil {
		return NotifyResult{}, err
	}
	if !notificationType.Enabled {
		return NotifyResult{}, fmt.Errorf("notification type %q is disabled", input.TypeKey)
	}

	recipients := normalizeRecipients(input.Recipients)
	if len(recipients) == 0 {
		return NotifyResult{}, errors.New("at least one valid recipient is required")
	}

	policy, err := effectivePolicy(notificationType.DefaultPolicy, input.PolicyOverride)
	if err != nil {
		return NotifyResult{}, err
	}
	deliveries, err := deliverySpecsForPolicy(policy, s.now().UTC())
	if err != nil {
		return NotifyResult{}, err
	}

	metadataJSON, err := marshalJSONObject(input.Metadata)
	if err != nil {
		return NotifyResult{}, fmt.Errorf("marshal notification metadata: %w", err)
	}
	policyOverrideJSON, err := marshalJSONObject(input.PolicyOverride)
	if err != nil {
		return NotifyResult{}, fmt.Errorf("marshal notification policy override: %w", err)
	}

	title := strings.TrimSpace(input.Title)
	if title == "" {
		title = notificationType.TitleTemplate
	}
	body := strings.TrimSpace(input.Body)
	if body == "" {
		body = notificationType.BodyTemplate
	}
	severity := strings.TrimSpace(input.Severity)
	if severity == "" {
		severity = notificationType.Severity
	}

	return s.store.CreateNotification(ctx, CreateNotificationInput{
		TypeKey:            notificationType.TypeKey,
		AppID:              notificationType.AppID,
		Severity:           severity,
		Title:              title,
		Body:               body,
		EntityType:         strings.TrimSpace(input.EntityType),
		EntityID:           strings.TrimSpace(input.EntityID),
		DedupeKey:          input.DedupeKey,
		DeepLink:           strings.TrimSpace(input.DeepLink),
		MetadataJSON:       metadataJSON,
		PolicyOverrideJSON: policyOverrideJSON,
		CreatedBySubject:   strings.TrimSpace(input.CreatedBySubject),
		CreatedByEmail:     normalizeEmail(input.CreatedByEmail),
		Recipients:         recipients,
		Deliveries:         deliveries,
	})
}

func (s *Service) Resolve(ctx context.Context, input ResolveInput) error {
	if s == nil || s.store == nil {
		return errors.New("notifications store not configured")
	}
	_, err := s.store.Resolve(ctx, ResolveInput{
		TypeKey:    strings.TrimSpace(input.TypeKey),
		EntityType: strings.TrimSpace(input.EntityType),
		EntityID:   strings.TrimSpace(input.EntityID),
		DedupeKey:  strings.TrimSpace(input.DedupeKey),
	})
	return err
}

func normalizeRecipients(values []Recipient) []Recipient {
	out := make([]Recipient, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		email := normalizeEmail(value.Email)
		if email == "" {
			continue
		}
		if _, exists := seen[email]; exists {
			continue
		}
		seen[email] = struct{}{}
		out = append(out, Recipient{
			Subject: strings.TrimSpace(value.Subject),
			Email:   email,
			Name:    strings.TrimSpace(value.Name),
		})
	}
	return out
}

func normalizeEmail(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	address, err := mail.ParseAddress(value)
	if err != nil || address.Address == "" || !strings.Contains(address.Address, "@") {
		return ""
	}
	return strings.ToLower(address.Address)
}

func marshalJSONObject(value map[string]any) ([]byte, error) {
	if value == nil {
		return []byte(`{}`), nil
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return nil, err
	}
	if decoded == nil {
		return []byte(`{}`), nil
	}
	return raw, nil
}
