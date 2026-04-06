package budget

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"

	"github.com/sciacco/mrsmith/internal/platform/arak"
	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

// arakClient is set by RegisterRoutes when a live Arak client is provided.
// nil means fixture mode (all handlers use in-memory data).
var arakClient *arak.Client

// RegisterRoutes registers all budget API handlers.
// If client is non-nil, all handlers proxy to the real Arak API;
// otherwise they fall back to fixture data.
func RegisterRoutes(mux *http.ServeMux, client ...*arak.Client) {
	if len(client) > 0 && client[0] != nil {
		arakClient = client[0]
		log.Println("budget: all handlers will proxy to Arak API")
	}
	// Users
	mux.HandleFunc("GET /users-int/v1/user", handleGetAllUsers)
	// Groups
	mux.HandleFunc("GET /budget/v1/group", handleGetAllGroups)
	mux.HandleFunc("GET /budget/v1/group/{group_id}", handleGetGroupDetails)
	mux.HandleFunc("POST /budget/v1/group", handleNewGroup)
	mux.HandleFunc("PUT /budget/v1/group/{group_id}", handleEditGroup)
	mux.HandleFunc("DELETE /budget/v1/group/{group_id}", handleDeleteGroup)
	// Cost centers
	mux.HandleFunc("GET /budget/v1/cost-center", handleGetAllCostCenters)
	mux.HandleFunc("GET /budget/v1/cost-center/{cost_center_id}", handleGetCostCenterDetails)
	mux.HandleFunc("POST /budget/v1/cost-center", handleNewCostCenter)
	mux.HandleFunc("PUT /budget/v1/cost-center/{cost_center_id}", handleEditCostCenter)
	// Budgets
	mux.HandleFunc("GET /budget/v1/budget", handleGetAllBudgets)
	mux.HandleFunc("GET /budget/v1/budget/{budget_id}", handleGetBudgetDetails)
	mux.HandleFunc("POST /budget/v1/budget", handleNewBudget)
	mux.HandleFunc("PUT /budget/v1/budget/{budget_id}", handleEditBudget)
	mux.HandleFunc("DELETE /budget/v1/budget/{budget_id}", handleDeleteBudget)
	// Reports
	mux.HandleFunc("GET /budget/v1/report/budget-used-over-percentage", handleGetBudgetOverPercent)
	mux.HandleFunc("GET /budget/v1/report/unassigned-users", handleGetUnassignedUsers)
	// User allocations
	mux.HandleFunc("POST /budget/v1/budget/{budget_id}/user", handleNewUserBudget)
	mux.HandleFunc("PUT /budget/v1/budget/{budget_id}/user", handleEditUserBudget)
	// Cost center allocations
	mux.HandleFunc("POST /budget/v1/budget/{budget_id}/cost-center", handleNewCcBudget)
	mux.HandleFunc("PUT /budget/v1/budget/{budget_id}/cost-center", handleEditCcBudget)
	// Approval rules — user budget
	mux.HandleFunc("GET /budget/v1/approval-rules/user-budget", handleGetUserBudgetRules)
	mux.HandleFunc("POST /budget/v1/approval-rules/user-budget", handleNewUserBudgetRule)
	mux.HandleFunc("PUT /budget/v1/approval-rules/user-budget/{rule_id}", handleEditUserBudgetRule)
	mux.HandleFunc("DELETE /budget/v1/approval-rules/user-budget/{rule_id}", handleDeleteUserBudgetRule)
	// Approval rules — cost center budget
	mux.HandleFunc("GET /budget/v1/approval-rules/cost-center-budget", handleGetCcBudgetRules)
	mux.HandleFunc("POST /budget/v1/approval-rules/cost-center-budget", handleNewCcBudgetRule)
	mux.HandleFunc("PUT /budget/v1/approval-rules/cost-center-budget/{rule_id}", handleEditCcBudgetRule)
	mux.HandleFunc("DELETE /budget/v1/approval-rules/cost-center-budget/{rule_id}", handleDeleteCcBudgetRule)
}

// ═══ Proxy helper ═══

// proxyToArak forwards the request to the real Arak API and streams the
// response back. arakPath is the full Arak path (e.g. "/arak/budget/v1/group").
func proxyToArak(w http.ResponseWriter, r *http.Request, arakPath string) {
	resp, err := arakClient.Do(r.Method, arakPath, r.URL.RawQuery, r.Body)
	if err != nil {
		log.Printf("budget: arak proxy error: %v", err)
		httputil.Error(w, http.StatusBadGateway, "upstream API error")
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
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

func handleGetAllCostCenters(w http.ResponseWriter, r *http.Request) {
	if arakClient != nil {
		proxyToArak(w, r, "/arak/budget/v1/cost-center")
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
