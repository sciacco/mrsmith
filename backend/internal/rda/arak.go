package rda

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type upstreamStatusError struct {
	status int
	body   []byte
}

func (e *upstreamStatusError) Error() string {
	return fmt.Sprintf("upstream returned %d", e.status)
}

func (h *Handler) fetchPODetail(r *http.Request, email, id string) (poDetail, error) {
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
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return poDetail{}, err
	}
	rowEconomics, err := h.fetchPORowEconomics(r.Context(), id)
	if err != nil {
		h.requestLogger(r, "rda_po_detail").Warn("failed to load PO row economics", "error", err)
	} else if normalized, err := normalizePODetailRows(body, rowEconomics); err == nil {
		body = normalized
	} else {
		return poDetail{}, err
	}
	var po poDetail
	if err := json.Unmarshal(body, &po); err != nil {
		return poDetail{}, err
	}
	return po, nil
}

func isRequester(po poDetail, email string) bool {
	return strings.EqualFold(strings.TrimSpace(po.Requester.Email), strings.TrimSpace(email))
}

func isApprover(po poDetail, email string) bool {
	for _, approver := range po.Approvers {
		if strings.EqualFold(strings.TrimSpace(approver.User.Email), strings.TrimSpace(email)) && approverLevelMatches(po, approver) {
			return true
		}
	}
	return false
}

func approverLevelMatches(po poDetail, approver approverRef) bool {
	current := normalizeApprovalLevel(po.CurrentApprovalLevel)
	if current == "" {
		return true
	}
	level := normalizeApprovalLevel(approver.Level)
	return level == "" || level == current
}

func normalizeApprovalLevel(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(typed)
	case json.Number:
		return strings.TrimSpace(typed.String())
	default:
		return strings.TrimSpace(fmt.Sprint(typed))
	}
}

type inboxRoute struct {
	upstreamPath       string
	requiredPermission rdaPermissionFlag
}

func inboxConfig(kind string) (inboxRoute, bool) {
	switch kind {
	case "level1-2":
		return inboxRoute{upstreamPath: "/po/pending-approval", requiredPermission: permissionApprover}, true
	case "leasing":
		return inboxRoute{upstreamPath: "/po/pending-leasing", requiredPermission: permissionAFC}, true
	case "no-leasing":
		return inboxRoute{upstreamPath: "/po/pending-approval-no-leasing", requiredPermission: permissionApproverNoLeasing}, true
	case "payment-method":
		return inboxRoute{upstreamPath: "/po/pending-approval-payment-method", requiredPermission: permissionAFC}, true
	case "budget-increment":
		return inboxRoute{upstreamPath: "/po-pending-budget-increment", requiredPermission: permissionExtraBudget}, true
	default:
		return inboxRoute{}, false
	}
}

func inboxKindFromRequest(r *http.Request) string {
	return strings.TrimPrefix(r.URL.Path, "/rda/v1/pos/inbox/")
}
