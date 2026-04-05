package budget

import (
	"encoding/json"
	"net/http"
	"net/url"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

func RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /users-int/v1/user", handleGetAllUsers)
	mux.HandleFunc("GET /budget/v1/group", handleGetAllGroups)
	mux.HandleFunc("GET /budget/v1/group/{group_id}", handleGetGroupDetails)
	mux.HandleFunc("POST /budget/v1/group", handleNewGroup)
	mux.HandleFunc("PUT /budget/v1/group/{group_id}", handleEditGroup)
	mux.HandleFunc("DELETE /budget/v1/group/{group_id}", handleDeleteGroup)
	mux.HandleFunc("GET /budget/v1/cost-center", handleGetAllCostCenters)
	mux.HandleFunc("GET /budget/v1/cost-center/{cost_center_id}", handleGetCostCenterDetails)
	mux.HandleFunc("POST /budget/v1/cost-center", handleNewCostCenter)
	mux.HandleFunc("PUT /budget/v1/cost-center/{cost_center_id}", handleEditCostCenter)
}

func handleGetAllUsers(w http.ResponseWriter, r *http.Request) {
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

func handleGetAllGroups(w http.ResponseWriter, r *http.Request) {
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
	details, ok := db.getGroupDetails(groupID)
	if !ok {
		httputil.Error(w, http.StatusNotFound, "group not found")
		return
	}
	httputil.JSON(w, http.StatusOK, details)
}

func handleNewGroup(w http.ResponseWriter, r *http.Request) {
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
	if !db.deleteGroup(groupID) {
		httputil.Error(w, http.StatusNotFound, "group not found")
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]string{"message": "group deleted"})
}

func handleGetAllCostCenters(w http.ResponseWriter, r *http.Request) {
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
	details, ok := db.getCostCenterDetails(ccID)
	if !ok {
		httputil.Error(w, http.StatusNotFound, "cost center not found")
		return
	}
	httputil.JSON(w, http.StatusOK, details)
}

func handleNewCostCenter(w http.ResponseWriter, r *http.Request) {
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
