package budget

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"sync"

	"github.com/sciacco/mrsmith/internal/acl"
	"github.com/sciacco/mrsmith/internal/platform/applaunch"
	"github.com/sciacco/mrsmith/internal/platform/arak"
	"github.com/sciacco/mrsmith/internal/platform/httputil"
	"github.com/sciacco/mrsmith/internal/platform/logging"
)

// arakClient is set by RegisterRoutes when a live Arak client is provided.
// nil means fixture mode (all handlers use in-memory data).
var arakClient *arak.Client

const (
	upstreamAuthFailedCode  = "UPSTREAM_AUTH_FAILED"
	upstreamUnavailableCode = "UPSTREAM_UNAVAILABLE"
)

// RegisterRoutes registers all budget API handlers.
// If client is non-nil, all handlers proxy to the real Arak API;
// otherwise they fall back to fixture data.
func RegisterRoutes(mux *http.ServeMux, client ...*arak.Client) {
	if len(client) > 0 && client[0] != nil {
		arakClient = client[0]
		slog.Default().Info("arak proxy mode enabled", "component", "budget")
	}
	protectBudget := acl.RequireRole(applaunch.BudgetAccessRoles()...)
	handle := func(pattern string, handler http.HandlerFunc) {
		mux.Handle(pattern, protectBudget(http.HandlerFunc(handler)))
	}
	// Users
	handle("GET /users-int/v1/user", handleGetAllUsers)
	// Groups
	handle("GET /budget/v1/group", handleGetAllGroups)
	handle("GET /budget/v1/group/{group_id}", handleGetGroupDetails)
	handle("POST /budget/v1/group", handleNewGroup)
	handle("PUT /budget/v1/group/{group_id}", handleEditGroup)
	handle("DELETE /budget/v1/group/{group_id}", handleDeleteGroup)
	// Cost centers
	handle("GET /budget/v1/cost-center", handleGetAllCostCenters)
	handle("GET /budget/v1/cost-center/{cost_center_id}", handleGetCostCenterDetails)
	handle("POST /budget/v1/cost-center", handleNewCostCenter)
	handle("PUT /budget/v1/cost-center/{cost_center_id}", handleEditCostCenter)
	// Budgets
	handle("GET /budget/v1/budget", handleGetAllBudgets)
	handle("GET /budget/v1/budget/{budget_id}", handleGetBudgetDetails)
	handle("POST /budget/v1/budget", handleNewBudget)
	handle("PUT /budget/v1/budget/{budget_id}", handleEditBudget)
	handle("DELETE /budget/v1/budget/{budget_id}", handleDeleteBudget)
	// Reports
	handle("GET /budget/v1/report/budget-used-over-percentage", handleGetBudgetOverPercent)
	handle("GET /budget/v1/report/unassigned-users", handleGetUnassignedUsers)
	// User allocations
	handle("POST /budget/v1/budget/{budget_id}/user", handleNewUserBudget)
	handle("PUT /budget/v1/budget/{budget_id}/user", handleEditUserBudget)
	// Cost center allocations
	handle("POST /budget/v1/budget/{budget_id}/cost-center", handleNewCcBudget)
	handle("PUT /budget/v1/budget/{budget_id}/cost-center", handleEditCcBudget)
	// Approval rules — user budget
	handle("GET /budget/v1/approval-rules/user-budget", handleGetUserBudgetRules)
	handle("POST /budget/v1/approval-rules/user-budget", handleNewUserBudgetRule)
	handle("PUT /budget/v1/approval-rules/user-budget/{rule_id}", handleEditUserBudgetRule)
	handle("DELETE /budget/v1/approval-rules/user-budget/{rule_id}", handleDeleteUserBudgetRule)
	// Approval rules — cost center budget
	handle("GET /budget/v1/approval-rules/cost-center-budget", handleGetCcBudgetRules)
	handle("POST /budget/v1/approval-rules/cost-center-budget", handleNewCcBudgetRule)
	handle("PUT /budget/v1/approval-rules/cost-center-budget/{rule_id}", handleEditCcBudgetRule)
	handle("DELETE /budget/v1/approval-rules/cost-center-budget/{rule_id}", handleDeleteCcBudgetRule)
}

func requestLogger(r *http.Request, operation string, attrs ...any) *slog.Logger {
	args := []any{"component", "budget", "operation", operation}
	args = append(args, attrs...)
	return logging.FromContext(r.Context()).With(args...)
}

// ═══ Proxy helper ═══

// proxyToArak forwards the request to the real Arak API and streams the
// response back. arakPath is the full Arak path (e.g. "/arak/budget/v1/group").
func proxyToArak(w http.ResponseWriter, r *http.Request, arakPath string) {
	resp, err := arakClient.Do(r.Method, arakPath, r.URL.RawQuery, r.Body)
	if err != nil {
		requestLogger(r, "proxy_to_arak", "upstream_path", arakPath).Error("upstream request failed", "error", err)
		writeUpstreamError(w, http.StatusBadGateway, upstreamUnavailableCode, "upstream API error")
		return
	}
	defer resp.Body.Close()

	if translateUpstreamAuthFailure(w, resp.StatusCode) {
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	if _, err := io.Copy(w, resp.Body); err != nil {
		requestLogger(r, "proxy_to_arak", "upstream_path", arakPath).Warn("failed to stream upstream response", "error", err)
	}
}

func isUpstreamAuthFailure(status int) bool {
	return status == http.StatusUnauthorized || status == http.StatusForbidden
}

func translateUpstreamAuthFailure(w http.ResponseWriter, status int) bool {
	if !isUpstreamAuthFailure(status) {
		return false
	}
	writeUpstreamError(w, http.StatusBadGateway, upstreamAuthFailedCode, "upstream authorization failed")
	return true
}

func writeUpstreamError(w http.ResponseWriter, status int, code, message string) {
	httputil.JSON(w, status, map[string]string{
		"error": message,
		"code":  code,
	})
}

// ═══ Users ═══

func handleGetAllUsers(w http.ResponseWriter, r *http.Request) {
	if arakClient != nil {
		proxyToArak(w, r, "/arak/users-int/v1/user")
		return
	}
	if r.URL.Query().Get("page_number") == "" {
		httputil.Error(w, http.StatusBadRequest, "page_number is required")
		return
	}
	users := db.getUsers()
	httputil.JSON(w, http.StatusOK, paginatedResponse{
		TotalNumber: len(users),
		CurrentPage: 1,
		TotalPages:  1,
		Items:       users,
	})
}

// ═══ Groups ═══

func handleGetAllGroups(w http.ResponseWriter, r *http.Request) {
	if arakClient != nil {
		proxyToArak(w, r, "/arak/budget/v1/group")
		return
	}
	if r.URL.Query().Get("page_number") == "" {
		httputil.Error(w, http.StatusBadRequest, "page_number is required")
		return
	}
	groups := db.listGroups()
	httputil.JSON(w, http.StatusOK, paginatedResponse{
		TotalNumber: len(groups),
		CurrentPage: 1,
		TotalPages:  1,
		Items:       groups,
	})
}

func handleGetGroupDetails(w http.ResponseWriter, r *http.Request) {
	groupID, _ := url.PathUnescape(r.PathValue("group_id"))
	if arakClient != nil {
		proxyToArak(w, r, "/arak/budget/v1/group/"+url.PathEscape(groupID))
		return
	}
	details, ok := db.getGroupDetails(groupID)
	if !ok {
		httputil.Error(w, http.StatusNotFound, "group not found")
		return
	}
	httputil.JSON(w, http.StatusOK, details)
}

func handleNewGroup(w http.ResponseWriter, r *http.Request) {
	if arakClient != nil {
		proxyToArak(w, r, "/arak/budget/v1/group")
		return
	}
	var body struct {
		Name    string  `json:"name"`
		UserIDs []int64 `json:"user_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.UserIDs == nil {
		body.UserIDs = []int64{}
	}
	db.createGroup(body.Name, body.UserIDs)
	httputil.JSON(w, http.StatusOK, map[string]string{"message": "group created"})
}

func handleEditGroup(w http.ResponseWriter, r *http.Request) {
	groupID, _ := url.PathUnescape(r.PathValue("group_id"))
	if arakClient != nil {
		proxyToArak(w, r, "/arak/budget/v1/group/"+url.PathEscape(groupID))
		return
	}
	var body struct {
		NewName *string  `json:"new_name,omitempty"`
		UserIDs *[]int64 `json:"user_ids,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if !db.editGroup(groupID, body.NewName, body.UserIDs) {
		httputil.Error(w, http.StatusNotFound, "group not found")
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]string{"message": "group updated"})
}

func handleDeleteGroup(w http.ResponseWriter, r *http.Request) {
	groupID, _ := url.PathUnescape(r.PathValue("group_id"))
	if arakClient != nil {
		proxyToArak(w, r, "/arak/budget/v1/group/"+url.PathEscape(groupID))
		return
	}
	if !db.deleteGroup(groupID) {
		httputil.Error(w, http.StatusNotFound, "group not found")
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]string{"message": "group deleted"})
}

// ═══ Cost centers ═══

// enrichedCostCenterList fetches the cost-center list from Arak, then fetches
// details for each to compute group_count and group_user_count (which the
// upstream list endpoint does not include).
func enrichedCostCenterList(w http.ResponseWriter, r *http.Request) {
	// 1. Fetch the list from Arak.
	resp, err := arakClient.Do("GET", "/arak/budget/v1/cost-center", r.URL.RawQuery, nil)
	if err != nil {
		requestLogger(r, "list_cost_centers").Error("failed to fetch upstream cost-center list", "error", err)
		writeUpstreamError(w, http.StatusBadGateway, upstreamUnavailableCode, "upstream API error")
		return
	}
	defer resp.Body.Close()

	if translateUpstreamAuthFailure(w, resp.StatusCode) {
		return
	}

	if resp.StatusCode != http.StatusOK {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(resp.StatusCode)
		if _, err := io.Copy(w, resp.Body); err != nil {
			requestLogger(r, "list_cost_centers").Warn("failed to stream upstream cost-center list", "error", err)
		}
		return
	}

	// Parse the paginated list.
	var listResp struct {
		TotalNumber int `json:"total_number"`
		CurrentPage int `json:"current_page"`
		TotalPages  int `json:"total_pages"`
		Items       []struct {
			Name         string `json:"name"`
			ManagerEmail string `json:"manager_email"`
			UserCount    int    `json:"user_count"`
			Enabled      bool   `json:"enabled"`
		} `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		requestLogger(r, "list_cost_centers").Error("failed to decode upstream cost-center list", "error", err)
		httputil.Error(w, http.StatusBadGateway, "failed to parse upstream response")
		return
	}

	// 2. Fetch details concurrently for each cost center.
	type detailResult struct {
		idx            int
		userCount      int
		groupCount     int
		groupUserCount int
	}

	results := make([]detailResult, len(listResp.Items))
	var wg sync.WaitGroup
	var detailAuthFailed bool
	var detailMu sync.Mutex
	for i, item := range listResp.Items {
		wg.Add(1)
		go func(idx int, name string) {
			defer wg.Done()
			dr := detailResult{idx: idx}
			encoded := url.PathEscape(name)
			detResp, err := arakClient.Do("GET", "/arak/budget/v1/cost-center/"+encoded, "", nil)
			if err != nil {
				requestLogger(r, "get_cost_center_detail", "cost_center", name).Error("failed to fetch upstream cost-center detail", "error", err)
				return
			}
			defer detResp.Body.Close()
			if isUpstreamAuthFailure(detResp.StatusCode) {
				detailMu.Lock()
				detailAuthFailed = true
				detailMu.Unlock()
				return
			}
			if detResp.StatusCode != http.StatusOK {
				return
			}
			var det struct {
				Users  []json.RawMessage `json:"users"`
				Groups []struct {
					Name  string            `json:"name"`
					Users []json.RawMessage `json:"users"`
				} `json:"groups"`
			}
			if err := json.NewDecoder(detResp.Body).Decode(&det); err != nil {
				requestLogger(r, "get_cost_center_detail", "cost_center", name).Error("failed to decode upstream cost-center detail", "error", err)
				return
			}
			dr.userCount = len(det.Users)
			dr.groupCount = len(det.Groups)
			for _, g := range det.Groups {
				dr.groupUserCount += len(g.Users)
			}
			results[idx] = dr
		}(i, item.Name)
	}
	wg.Wait()

	if detailAuthFailed {
		writeUpstreamError(w, http.StatusBadGateway, upstreamAuthFailedCode, "upstream authorization failed")
		return
	}

	// 3. Build enriched response.
	type enrichedCC struct {
		Name           string `json:"name"`
		ManagerEmail   string `json:"manager_email"`
		UserCount      int    `json:"user_count"`
		GroupCount     int    `json:"group_count"`
		GroupUserCount int    `json:"group_user_count"`
		Enabled        bool   `json:"enabled"`
	}
	items := make([]enrichedCC, len(listResp.Items))
	for i, item := range listResp.Items {
		items[i] = enrichedCC{
			Name:           item.Name,
			ManagerEmail:   item.ManagerEmail,
			UserCount:      results[i].userCount,
			GroupCount:     results[i].groupCount,
			GroupUserCount: results[i].groupUserCount,
			Enabled:        item.Enabled,
		}
	}

	httputil.JSON(w, http.StatusOK, paginatedResponse{
		TotalNumber: listResp.TotalNumber,
		CurrentPage: listResp.CurrentPage,
		TotalPages:  listResp.TotalPages,
		Items:       items,
	})
}

func handleGetAllCostCenters(w http.ResponseWriter, r *http.Request) {
	if arakClient != nil {
		enrichedCostCenterList(w, r)
		return
	}
	if r.URL.Query().Get("page_number") == "" {
		httputil.Error(w, http.StatusBadRequest, "page_number is required")
		return
	}
	ccs := db.listCostCenters()
	httputil.JSON(w, http.StatusOK, paginatedResponse{
		TotalNumber: len(ccs),
		CurrentPage: 1,
		TotalPages:  1,
		Items:       ccs,
	})
}

func handleGetCostCenterDetails(w http.ResponseWriter, r *http.Request) {
	ccID, _ := url.PathUnescape(r.PathValue("cost_center_id"))
	if arakClient != nil {
		proxyToArak(w, r, "/arak/budget/v1/cost-center/"+url.PathEscape(ccID))
		return
	}
	details, ok := db.getCostCenterDetails(ccID)
	if !ok {
		httputil.Error(w, http.StatusNotFound, "cost center not found")
		return
	}
	httputil.JSON(w, http.StatusOK, details)
}

func handleNewCostCenter(w http.ResponseWriter, r *http.Request) {
	if arakClient != nil {
		proxyToArak(w, r, "/arak/budget/v1/cost-center")
		return
	}
	var body struct {
		Name       string   `json:"name"`
		ManagerID  int64    `json:"manager_id"`
		UserIDs    []int64  `json:"user_ids"`
		GroupNames []string `json:"group_names"`
		Enabled    bool     `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.UserIDs == nil {
		body.UserIDs = []int64{}
	}
	if body.GroupNames == nil {
		body.GroupNames = []string{}
	}
	db.createCostCenter(body.Name, body.ManagerID, body.UserIDs, body.GroupNames, body.Enabled)
	httputil.JSON(w, http.StatusOK, map[string]string{"message": "cost center created"})
}

func handleEditCostCenter(w http.ResponseWriter, r *http.Request) {
	ccID, _ := url.PathUnescape(r.PathValue("cost_center_id"))
	if arakClient != nil {
		proxyToArak(w, r, "/arak/budget/v1/cost-center/"+url.PathEscape(ccID))
		return
	}
	var body struct {
		NewName    *string   `json:"new_name,omitempty"`
		ManagerID  *int64    `json:"manager_id,omitempty"`
		UserIDs    *[]int64  `json:"user_ids,omitempty"`
		GroupNames *[]string `json:"group_names,omitempty"`
		Enabled    *bool     `json:"enabled,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if !db.editCostCenter(ccID, body.NewName, body.ManagerID, body.UserIDs, body.GroupNames, body.Enabled) {
		httputil.Error(w, http.StatusNotFound, "cost center not found")
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]string{"message": "cost center updated"})
}

// ═══ Budgets ═══

func handleGetAllBudgets(w http.ResponseWriter, r *http.Request) {
	if arakClient != nil {
		proxyToArak(w, r, "/arak/budget/v1/budget")
		return
	}
	if r.URL.Query().Get("page_number") == "" {
		httputil.Error(w, http.StatusBadRequest, "page_number is required")
		return
	}
	budgets := db.listBudgets()
	httputil.JSON(w, http.StatusOK, paginatedResponse{
		TotalNumber: len(budgets),
		CurrentPage: 1,
		TotalPages:  1,
		Items:       budgets,
	})
}

func handleGetBudgetDetails(w http.ResponseWriter, r *http.Request) {
	budgetID := r.PathValue("budget_id")
	if arakClient != nil {
		proxyToArak(w, r, "/arak/budget/v1/budget/"+budgetID)
		return
	}
	id, ok := parseBudgetID(budgetID)
	if !ok {
		httputil.Error(w, http.StatusBadRequest, "invalid budget_id")
		return
	}
	details, found := db.getBudgetDetails(id)
	if !found {
		httputil.Error(w, http.StatusNotFound, "budget not found")
		return
	}
	httputil.JSON(w, http.StatusOK, details)
}

func handleNewBudget(w http.ResponseWriter, r *http.Request) {
	if arakClient != nil {
		proxyToArak(w, r, "/arak/budget/v1/budget")
		return
	}
	var body struct {
		Name string `json:"name"`
		Year int    `json:"year"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	id := db.createBudget(body.Name, body.Year)
	httputil.JSON(w, http.StatusOK, map[string]int64{"id": id})
}

func handleEditBudget(w http.ResponseWriter, r *http.Request) {
	budgetID := r.PathValue("budget_id")
	if arakClient != nil {
		proxyToArak(w, r, "/arak/budget/v1/budget/"+budgetID)
		return
	}
	id, ok := parseBudgetID(budgetID)
	if !ok {
		httputil.Error(w, http.StatusBadRequest, "invalid budget_id")
		return
	}
	var body struct {
		Name *string `json:"name,omitempty"`
		Year *int    `json:"year,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if !db.editBudget(id, body.Name, body.Year) {
		httputil.Error(w, http.StatusNotFound, "budget not found")
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]string{"message": "budget updated"})
}

func handleDeleteBudget(w http.ResponseWriter, r *http.Request) {
	budgetID := r.PathValue("budget_id")
	if arakClient != nil {
		proxyToArak(w, r, "/arak/budget/v1/budget/"+budgetID)
		return
	}
	id, ok := parseBudgetID(budgetID)
	if !ok {
		httputil.Error(w, http.StatusBadRequest, "invalid budget_id")
		return
	}
	if !db.deleteBudget(id) {
		httputil.Error(w, http.StatusNotFound, "budget not found")
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]string{"message": "budget deleted"})
}

// ═══ Reports ═══

func handleGetBudgetOverPercent(w http.ResponseWriter, r *http.Request) {
	if arakClient != nil {
		proxyToArak(w, r, "/arak/budget/v1/report/budget-used-over-percentage")
		return
	}
	if r.URL.Query().Get("page_number") == "" {
		httputil.Error(w, http.StatusBadRequest, "page_number is required")
		return
	}
	pctStr := r.URL.Query().Get("percentage")
	if pctStr == "" {
		httputil.Error(w, http.StatusBadRequest, "percentage is required")
		return
	}
	pct, err := strconv.ParseFloat(pctStr, 64)
	if err != nil || pct < 0 || pct > 100 {
		httputil.Error(w, http.StatusBadRequest, "percentage must be a number between 0 and 100")
		return
	}
	budgets := db.listBudgetsOverPercentage(pct)
	httputil.JSON(w, http.StatusOK, paginatedResponse{
		TotalNumber: len(budgets),
		CurrentPage: 1,
		TotalPages:  1,
		Items:       budgets,
	})
}

func handleGetUnassignedUsers(w http.ResponseWriter, r *http.Request) {
	if arakClient != nil {
		proxyToArak(w, r, "/arak/budget/v1/report/unassigned-users")
		return
	}
	if r.URL.Query().Get("page_number") == "" {
		httputil.Error(w, http.StatusBadRequest, "page_number is required")
		return
	}
	users := db.listUnassignedUsers()
	httputil.JSON(w, http.StatusOK, paginatedResponse{
		TotalNumber: len(users),
		CurrentPage: 1,
		TotalPages:  1,
		Items:       users,
	})
}

// ═══ User allocations ═══

func handleNewUserBudget(w http.ResponseWriter, r *http.Request) {
	budgetID := r.PathValue("budget_id")
	if arakClient != nil {
		proxyToArak(w, r, "/arak/budget/v1/budget/"+budgetID+"/user")
		return
	}
	id, ok := parseBudgetID(budgetID)
	if !ok {
		httputil.Error(w, http.StatusBadRequest, "invalid budget_id")
		return
	}
	var body struct {
		UserID int64  `json:"user_id"`
		Limit  string `json:"limit"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if !db.createUserAllocation(id, body.UserID, body.Limit) {
		httputil.Error(w, http.StatusNotFound, "budget not found")
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]string{"message": "user budget created"})
}

func handleEditUserBudget(w http.ResponseWriter, r *http.Request) {
	budgetID := r.PathValue("budget_id")
	if arakClient != nil {
		proxyToArak(w, r, "/arak/budget/v1/budget/"+budgetID+"/user")
		return
	}
	id, ok := parseBudgetID(budgetID)
	if !ok {
		httputil.Error(w, http.StatusBadRequest, "invalid budget_id")
		return
	}
	var body struct {
		UserID  int64   `json:"user_id"`
		Limit   *string `json:"limit,omitempty"`
		Enabled *bool   `json:"enabled,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if !db.editUserAllocation(id, body.UserID, body.Limit, body.Enabled) {
		httputil.Error(w, http.StatusNotFound, "allocation not found")
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]string{"message": "user budget updated"})
}

// ═══ Cost center allocations ═══

func handleNewCcBudget(w http.ResponseWriter, r *http.Request) {
	budgetID := r.PathValue("budget_id")
	if arakClient != nil {
		proxyToArak(w, r, "/arak/budget/v1/budget/"+budgetID+"/cost-center")
		return
	}
	id, ok := parseBudgetID(budgetID)
	if !ok {
		httputil.Error(w, http.StatusBadRequest, "invalid budget_id")
		return
	}
	var body struct {
		CostCenter string `json:"cost_center"`
		Limit      string `json:"limit"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if !db.createCcAllocation(id, body.CostCenter, body.Limit) {
		httputil.Error(w, http.StatusNotFound, "budget not found")
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]string{"message": "cost center budget created"})
}

func handleEditCcBudget(w http.ResponseWriter, r *http.Request) {
	budgetID := r.PathValue("budget_id")
	if arakClient != nil {
		proxyToArak(w, r, "/arak/budget/v1/budget/"+budgetID+"/cost-center")
		return
	}
	id, ok := parseBudgetID(budgetID)
	if !ok {
		httputil.Error(w, http.StatusBadRequest, "invalid budget_id")
		return
	}
	var body struct {
		CostCenter string  `json:"cost_center"`
		Limit      *string `json:"limit,omitempty"`
		Enabled    *bool   `json:"enabled,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if !db.editCcAllocation(id, body.CostCenter, body.Limit, body.Enabled) {
		httputil.Error(w, http.StatusNotFound, "allocation not found")
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]string{"message": "cost center budget updated"})
}

// ═══ User budget approval rules ═══

func handleGetUserBudgetRules(w http.ResponseWriter, r *http.Request) {
	if arakClient != nil {
		proxyToArak(w, r, "/arak/budget/v1/approval-rules/user-budget")
		return
	}
	if r.URL.Query().Get("page_number") == "" {
		httputil.Error(w, http.StatusBadRequest, "page_number is required")
		return
	}
	budgetID, err := strconv.ParseInt(r.URL.Query().Get("budget_id"), 10, 64)
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "budget_id is required")
		return
	}
	userID, err := strconv.ParseInt(r.URL.Query().Get("user_id"), 10, 64)
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "user_id is required")
		return
	}
	rules := db.listUserRules(budgetID, userID)
	httputil.JSON(w, http.StatusOK, paginatedResponse{
		TotalNumber: len(rules),
		CurrentPage: 1,
		TotalPages:  1,
		Items:       rules,
	})
}

func handleNewUserBudgetRule(w http.ResponseWriter, r *http.Request) {
	if arakClient != nil {
		proxyToArak(w, r, "/arak/budget/v1/approval-rules/user-budget")
		return
	}
	var body struct {
		Threshold  string `json:"threshold"`
		ApproverID int64  `json:"approver_id"`
		BudgetID   int64  `json:"budget_id"`
		UserID     int64  `json:"user_id"`
		Level      int    `json:"level"`
		SendEmail  bool   `json:"send_email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	id := db.createUserRule(body.Threshold, body.ApproverID, body.BudgetID, body.UserID, body.Level, body.SendEmail)
	httputil.JSON(w, http.StatusOK, map[string]int64{"id": id})
}

func handleEditUserBudgetRule(w http.ResponseWriter, r *http.Request) {
	ruleIDStr := r.PathValue("rule_id")
	if arakClient != nil {
		proxyToArak(w, r, "/arak/budget/v1/approval-rules/user-budget/"+ruleIDStr)
		return
	}
	ruleID, ok := parseRuleID(ruleIDStr)
	if !ok {
		httputil.Error(w, http.StatusBadRequest, "invalid rule_id")
		return
	}
	var body struct {
		Threshold  *string `json:"threshold,omitempty"`
		ApproverID *int64  `json:"approver_id,omitempty"`
		Level      *int    `json:"level,omitempty"`
		SendEmail  *bool   `json:"send_email,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if !db.editUserRule(ruleID, body.Threshold, body.ApproverID, body.Level, body.SendEmail) {
		httputil.Error(w, http.StatusNotFound, "rule not found")
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]string{"message": "rule updated"})
}

func handleDeleteUserBudgetRule(w http.ResponseWriter, r *http.Request) {
	ruleIDStr := r.PathValue("rule_id")
	if arakClient != nil {
		proxyToArak(w, r, "/arak/budget/v1/approval-rules/user-budget/"+ruleIDStr)
		return
	}
	ruleID, ok := parseRuleID(ruleIDStr)
	if !ok {
		httputil.Error(w, http.StatusBadRequest, "invalid rule_id")
		return
	}
	if !db.deleteUserRule(ruleID) {
		httputil.Error(w, http.StatusNotFound, "rule not found")
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]string{"message": "rule deleted"})
}

// ═══ Cost center budget approval rules ═══

func handleGetCcBudgetRules(w http.ResponseWriter, r *http.Request) {
	if arakClient != nil {
		proxyToArak(w, r, "/arak/budget/v1/approval-rules/cost-center-budget")
		return
	}
	if r.URL.Query().Get("page_number") == "" {
		httputil.Error(w, http.StatusBadRequest, "page_number is required")
		return
	}
	budgetID, err := strconv.ParseInt(r.URL.Query().Get("budget_id"), 10, 64)
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "budget_id is required")
		return
	}
	costCenter := r.URL.Query().Get("cost_center")
	if costCenter == "" {
		httputil.Error(w, http.StatusBadRequest, "cost_center is required")
		return
	}
	rules := db.listCcRules(budgetID, costCenter)
	httputil.JSON(w, http.StatusOK, paginatedResponse{
		TotalNumber: len(rules),
		CurrentPage: 1,
		TotalPages:  1,
		Items:       rules,
	})
}

func handleNewCcBudgetRule(w http.ResponseWriter, r *http.Request) {
	if arakClient != nil {
		proxyToArak(w, r, "/arak/budget/v1/approval-rules/cost-center-budget")
		return
	}
	var body struct {
		Threshold  string `json:"threshold"`
		ApproverID int64  `json:"approver_id"`
		BudgetID   int64  `json:"budget_id"`
		CostCenter string `json:"cost_center"`
		Level      int    `json:"level"`
		SendEmail  bool   `json:"send_email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	id := db.createCcRule(body.Threshold, body.ApproverID, body.BudgetID, body.CostCenter, body.Level, body.SendEmail)
	httputil.JSON(w, http.StatusOK, map[string]int64{"id": id})
}

func handleEditCcBudgetRule(w http.ResponseWriter, r *http.Request) {
	ruleIDStr := r.PathValue("rule_id")
	if arakClient != nil {
		proxyToArak(w, r, "/arak/budget/v1/approval-rules/cost-center-budget/"+ruleIDStr)
		return
	}
	ruleID, ok := parseRuleID(ruleIDStr)
	if !ok {
		httputil.Error(w, http.StatusBadRequest, "invalid rule_id")
		return
	}
	var body struct {
		Threshold  *string `json:"threshold,omitempty"`
		ApproverID *int64  `json:"approver_id,omitempty"`
		Level      *int    `json:"level,omitempty"`
		SendEmail  *bool   `json:"send_email,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if !db.editCcRule(ruleID, body.Threshold, body.ApproverID, body.Level, body.SendEmail) {
		httputil.Error(w, http.StatusNotFound, "rule not found")
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]string{"message": "rule updated"})
}

func handleDeleteCcBudgetRule(w http.ResponseWriter, r *http.Request) {
	ruleIDStr := r.PathValue("rule_id")
	if arakClient != nil {
		proxyToArak(w, r, "/arak/budget/v1/approval-rules/cost-center-budget/"+ruleIDStr)
		return
	}
	ruleID, ok := parseRuleID(ruleIDStr)
	if !ok {
		httputil.Error(w, http.StatusBadRequest, "invalid rule_id")
		return
	}
	if !db.deleteCcRule(ruleID) {
		httputil.Error(w, http.StatusNotFound, "rule not found")
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]string{"message": "rule deleted"})
}
