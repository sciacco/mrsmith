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
	var po poDetail
	if err := json.NewDecoder(resp.Body).Decode(&po); err != nil {
		return poDetail{}, err
	}
	return po, nil
}

func isRequester(po poDetail, email string) bool {
	return strings.EqualFold(strings.TrimSpace(po.Requester.Email), strings.TrimSpace(email))
}

func isApprover(po poDetail, email string) bool {
	for _, approver := range po.Approvers {
		if strings.EqualFold(strings.TrimSpace(approver.User.Email), strings.TrimSpace(email)) {
			return true
		}
	}
	return false
}

type inboxRoute struct {
	upstreamPath string
	roles        []string
}

func inboxConfig(kind string) (inboxRoute, bool) {
	switch kind {
	case "level1-2":
		return inboxRoute{upstreamPath: "/po/pending-approval", roles: applaunchRDAApproverL1L2Roles()}, true
	case "leasing":
		return inboxRoute{upstreamPath: "/po/pending-leasing", roles: applaunchRDAApproverAFCRoles()}, true
	case "no-leasing":
		return inboxRoute{upstreamPath: "/po/pending-approval-no-leasing", roles: applaunchRDAApproverNoLeasingRoles()}, true
	case "payment-method":
		return inboxRoute{upstreamPath: "/po/pending-approval-payment-method", roles: applaunchRDAApproverAFCRoles()}, true
	case "budget-increment":
		return inboxRoute{upstreamPath: "/po-pending-budget-increment", roles: applaunchRDAApproverExtraBudgetRoles()}, true
	default:
		return inboxRoute{}, false
	}
}

func inboxKindFromRequest(r *http.Request) string {
	return strings.TrimPrefix(r.URL.Path, "/rda/v1/pos/inbox/")
}
