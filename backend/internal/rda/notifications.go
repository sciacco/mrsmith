package rda

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/sciacco/mrsmith/internal/auth"
	"github.com/sciacco/mrsmith/internal/notifications"
)

const (
	rdaApprovalNotificationType = "rda_approval_requested"
	rdaNotificationEntityType   = "rda_po"
)

func (h *Handler) afterRDAWorkflowTransition(ctx context.Context, logger *slog.Logger, email string, poID string, previous *poDetail) {
	if h.notifier == nil {
		return
	}
	if err := h.notifier.Resolve(ctx, notifications.ResolveInput{
		TypeKey:    rdaApprovalNotificationType,
		EntityType: rdaNotificationEntityType,
		EntityID:   poID,
	}); err != nil {
		logger.Warn("failed to resolve stale RDA notifications", "po_id", poID, "error", err)
	}

	updated, err := h.fetchPODetailForNotifications(ctx, email, poID)
	if err != nil {
		logger.Warn("failed to fetch updated PO for notifications", "po_id", poID, "error", err)
		return
	}
	if !shouldNotifyRDAApproval(previous, updated) {
		return
	}
	if err := h.notifyRDAApprovalRequested(ctx, updated); err != nil {
		logger.Warn("failed to create RDA approval notification", "po_id", poID, "error", err)
	}
}

func (h *Handler) notifyRDAApprovalRequested(ctx context.Context, po poDetail) error {
	if h.notifier == nil {
		return nil
	}
	poID := poIDString(po.ID)
	if poID == "" {
		return nil
	}
	recipients := rdaApprovalRecipients(po)
	if len(recipients) == 0 {
		return nil
	}
	level := normalizeApprovalLevel(po.CurrentApprovalLevel)
	if level == "" {
		level = "current"
	}
	code := strings.TrimSpace(po.Code)
	if code == "" {
		code = poID
	}
	title := fmt.Sprintf("RDA %s waiting for approval", code)
	body := "A purchase request is waiting for your approval."
	if requester := strings.TrimSpace(po.Requester.Email); requester != "" {
		body = fmt.Sprintf("A purchase request from %s is waiting for your approval.", requester)
	}
	claims, _ := auth.GetClaims(ctx)
	_, err := h.notifier.Notify(ctx, notifications.NotifyInput{
		TypeKey:    rdaApprovalNotificationType,
		Title:      title,
		Body:       body,
		EntityType: rdaNotificationEntityType,
		EntityID:   poID,
		DedupeKey:  fmt.Sprintf("rda:po:%s:approval:%s:requested", poID, level),
		DeepLink:   h.rdaPODetailDeepLink(poID),
		Metadata: map[string]any{
			"po_id":                  poID,
			"po_code":                code,
			"state":                  po.State,
			"current_approval_level": level,
			"requester_email":        strings.TrimSpace(po.Requester.Email),
			"total_price":            strings.TrimSpace(po.TotalPrice),
			"currency":               strings.TrimSpace(po.Currency),
		},
		Recipients:       recipients,
		CreatedBySubject: claims.Subject,
		CreatedByEmail:   claims.Email,
	})
	return err
}

func rdaApprovalRecipients(po poDetail) []notifications.Recipient {
	recipients := make([]notifications.Recipient, 0, len(po.Approvers))
	seen := map[string]struct{}{}
	for _, approver := range po.Approvers {
		if !approverLevelMatches(po, approver) {
			continue
		}
		email := strings.TrimSpace(approver.User.Email)
		if email == "" {
			continue
		}
		key := strings.ToLower(email)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		recipients = append(recipients, notifications.Recipient{Email: email})
	}
	return recipients
}

func shouldNotifyRDAApproval(previous *poDetail, updated poDetail) bool {
	if updated.State != "PENDING_APPROVAL" || len(rdaApprovalRecipients(updated)) == 0 {
		return false
	}
	if previous == nil {
		return true
	}
	return previous.State != "PENDING_APPROVAL" ||
		normalizeApprovalLevel(previous.CurrentApprovalLevel) != normalizeApprovalLevel(updated.CurrentApprovalLevel)
}

func (h *Handler) rdaPODetailDeepLink(poID string) string {
	escapedID := url.PathEscape(strings.TrimSpace(poID))
	if escapedID == "" {
		return "/apps/rda/rda"
	}
	appURL := strings.TrimRight(strings.TrimSpace(h.rdaAppURL), "/")
	if appURL != "" {
		return appURL + "/rda/po/" + escapedID
	}
	if strings.TrimSpace(h.staticDir) == "" {
		return "http://localhost:5190/rda/po/" + escapedID
	}
	return "/apps/rda/rda/po/" + escapedID
}

func poIDString(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(typed)
	default:
		return strings.TrimSpace(fmt.Sprint(typed))
	}
}

func (h *Handler) fetchPODetailForNotifications(ctx context.Context, email string, id string) (poDetail, error) {
	if h.arak == nil {
		return poDetail{}, errors.New("arak client not configured")
	}
	select {
	case <-ctx.Done():
		return poDetail{}, ctx.Err()
	default:
	}
	path := arakRDARoot + "/po/" + url.PathEscape(id)
	resp, err := h.arak.DoWithHeaders(http.MethodGet, path, "", nil, requesterHeaders(email))
	if err != nil {
		return poDetail{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return poDetail{}, &upstreamStatusError{status: resp.StatusCode, body: body}
	}
	var po poDetail
	decoder := json.NewDecoder(resp.Body)
	decoder.UseNumber()
	if err := decoder.Decode(&po); err != nil {
		return poDetail{}, err
	}
	return po, nil
}
