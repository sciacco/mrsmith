package factorial

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

// namespaces_integration_test.go is a hand-written end-to-end test of the
// generated namespaces.gen.go against an httptest.Server. It drives a full
// CRUD resource (teams.teams: List/Create/Update/Delete) and a bulk action
// returning a top-level array (tasks.tasks/bulk_create), asserting method,
// path, query string, request body (including the Update id-injection), and
// decoded response shapes. This is the only test that exercises the generated
// accessors through the real Client transport, so it is the integration
// guarantee for T10.

// nsRequest records one captured request for the integration server.
type nsRequest struct {
	method string
	path   string // full path, incl. /api/<version>/resources/...
	query  string // raw query string
	body   []byte
}

// nsServer is a routing httptest.Server that records every request and serves
// canned JSON for the teams CRUD + tasks bulk_create endpoints the test drives.
// Any unexpected route returns 404 so a wrong path fails loudly.
func nsServer(t *testing.T, rec *[]nsRequest) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		*rec = append(*rec, nsRequest{method: r.Method, path: r.URL.Path, query: r.URL.RawQuery, body: b})
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/2026-07-01/resources/teams/teams":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"data":[{"id":"1","name":"Alpha"},{"id":"2","name":"Beta"}],"meta":{"limit":5,"total":2,"has_next_page":false}}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/2026-07-01/resources/teams/teams":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":"10","name":"Gamma","description":"created"}`))
		case r.Method == http.MethodPut && r.URL.Path == "/api/2026-07-01/resources/teams/teams/42":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":"42","name":"Updated","description":"now updated"}`))
		case r.Method == http.MethodDelete && r.URL.Path == "/api/2026-07-01/resources/teams/teams/9":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":"9","name":"Deleted"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/2026-07-01/resources/tasks/tasks/bulk_create":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"id":"100","name":"T1","status":"todo"},{"id":"101","name":"T2","status":"done"}]`))
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":"unexpected route ` + r.Method + ` ` + r.URL.Path + `"}`))
		}
	}))
}

// TestNamespacesIntegration_TeamsCRUDAndTasksBulk drives the generated
// client.Teams.Teams and client.Tasks.Tasks accessors against an httptest.Server,
// asserting every request the runtime emits and every response it decodes.
func TestNamespacesIntegration_TeamsCRUDAndTasksBulk(t *testing.T) {
	var rec []nsRequest
	srv := nsServer(t, &rec)
	t.Cleanup(srv.Close)

	client := New(WithBaseURL(srv.URL), WithAPIKey("k"))
	ctx := context.Background()

	// --- List: GET teams/teams with ids[]=1&ids[]=2&limit=5 ------------------
	page, err := client.Teams.Teams.List(ctx, &TeamsTeamsListParams{IDs: []string{"1", "2"}, Limit: intPtr(5)})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	last := rec[len(rec)-1]
	if last.method != http.MethodGet {
		t.Errorf("List method = %q, want GET", last.method)
	}
	if got := last.path; got != "/api/2026-07-01/resources/teams/teams" {
		t.Errorf("List path = %q", got)
	}
	if got, want := last.query, "ids%5B%5D=1&ids%5B%5D=2&limit=5"; got != want {
		t.Errorf("List query = %q, want %q", got, want)
	}
	if len(page.Data) != 2 {
		t.Fatalf("List decoded %d items, want 2", len(page.Data))
	}
	if id := page.Data[0].ID; id == nil || *id != "1" {
		t.Errorf("List Data[0].ID = %v, want 1", page.Data[0].ID)
	}
	if page.Meta.Total != 2 || page.Meta.Limit != 5 {
		t.Errorf("List meta = {total:%d, limit:%d}, want {2,5}", page.Meta.Total, page.Meta.Limit)
	}

	// --- Create: POST teams/teams with a JSON body; 201 ----------------------
	created, err := client.Teams.Teams.Create(ctx, &TeamsTeamsCreateBody{Name: "Gamma", Description: strPtr("created")})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	last = rec[len(rec)-1]
	if last.method != http.MethodPost {
		t.Errorf("Create method = %q, want POST", last.method)
	}
	var gotBody map[string]any
	if err := json.Unmarshal(last.body, &gotBody); err != nil {
		t.Fatalf("Create body not JSON: %v\n%s", err, last.body)
	}
	if gotBody["name"] != "Gamma" {
		t.Errorf("Create body name = %v, want Gamma", gotBody["name"])
	}
	if created.ID == nil || *created.ID != "10" {
		t.Errorf("Create decoded ID = %v, want 10", created.ID)
	}

	// --- Update: PUT teams/teams/42; body must contain "id":"42" (injected) ---
	updated, err := client.Teams.Teams.Update(ctx, "42", &TeamsTeamsUpdateBody{Name: strPtr("Updated")})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	last = rec[len(rec)-1]
	if last.method != http.MethodPut {
		t.Errorf("Update method = %q, want PUT", last.method)
	}
	if got := last.path; got != "/api/2026-07-01/resources/teams/teams/42" {
		t.Errorf("Update path = %q, want .../teams/teams/42", got)
	}
	var updBody map[string]any
	if err := json.Unmarshal(last.body, &updBody); err != nil {
		t.Fatalf("Update body not JSON: %v\n%s", err, last.body)
	}
	if updBody["id"] != "42" {
		t.Errorf("Update body id = %v, want 42 (runtime id-injection)", updBody["id"])
	}
	if updBody["name"] != "Updated" {
		t.Errorf("Update body name = %v, want Updated", updBody["name"])
	}
	if updated.ID == nil || *updated.ID != "42" {
		t.Errorf("Update decoded ID = %v, want 42", updated.ID)
	}

	// --- Delete: DELETE teams/teams/9 ----------------------------------------
	deleted, err := client.Teams.Teams.Delete(ctx, "9")
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
	last = rec[len(rec)-1]
	if last.method != http.MethodDelete {
		t.Errorf("Delete method = %q, want DELETE", last.method)
	}
	if got := last.path; got != "/api/2026-07-01/resources/teams/teams/9" {
		t.Errorf("Delete path = %q, want .../teams/teams/9", got)
	}
	if deleted.ID == nil || *deleted.ID != "9" {
		t.Errorf("Delete decoded ID = %v, want 9", deleted.ID)
	}

	// --- Tasks bulk_create: POST returns a top-level JSON array ---------------
	tasks, err := client.Tasks.Tasks.BulkCreate(ctx, &TasksTasksBulkCreateBody{
		Name:   "T1",
		Status: TasksTasksBulkCreateBodyStatusTodo,
	})
	if err != nil {
		t.Fatalf("BulkCreate: %v", err)
	}
	last = rec[len(rec)-1]
	if last.method != http.MethodPost {
		t.Errorf("BulkCreate method = %q, want POST", last.method)
	}
	if got := last.path; got != "/api/2026-07-01/resources/tasks/tasks/bulk_create" {
		t.Errorf("BulkCreate path = %q", got)
	}
	if len(tasks) != 2 {
		t.Fatalf("BulkCreate decoded %d tasks, want 2", len(tasks))
	}
	if tasks[0].ID == nil || *tasks[0].ID != "100" {
		t.Errorf("BulkCreate tasks[0].ID = %v, want 100", tasks[0].ID)
	}
	if tasks[1].Status == nil || string(*tasks[1].Status) != "done" {
		t.Errorf("BulkCreate tasks[1].Status = %v, want done", tasks[1].Status)
	}
}

// TestNamespacesIntegration_Multipart drives an ATS messages Create — a
// multipart body whose Attachments field is an array-of-binary ([]File) —
// end-to-end through the real transport. It asserts the request is
// multipart/form-data, the server's ParseMultipartForm sees a file part named
// "attachments[]" with filename a.txt and content AAA, and the 201 JSON
// response decodes into *ATSMessage.
func TestNamespacesIntegration_Multipart(t *testing.T) {
	var (
		gotPath        string
		gotMethod      string
		gotContentType string
		gotFileName    string
		gotFileContent string
		sawAttachments bool
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		gotContentType = r.Header.Get("Content-Type")
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			t.Errorf("ParseMultipartForm: %v", err)
		}
		if r.MultipartForm == nil {
			t.Errorf("MultipartForm is nil after ParseMultipartForm")
			http.Error(w, "no multipart form", http.StatusBadRequest)
			return
		}
		files, ok := r.MultipartForm.File["attachments[]"]
		if !ok || len(files) == 0 {
			var keys []string
			for k := range r.MultipartForm.File {
				keys = append(keys, k)
			}
			t.Errorf("no file part named attachments[]; file keys = %v", keys)
			http.Error(w, "no attachments[] file", http.StatusBadRequest)
			return
		}
		sawAttachments = true
		gotFileName = files[0].Filename
		if f, err := files[0].Open(); err == nil {
			b, _ := io.ReadAll(f)
			f.Close()
			gotFileContent = string(b)
		} else {
			t.Errorf("open attachments[] file part: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"7","content":"hi","topic":"t"}`))
	}))
	t.Cleanup(srv.Close)

	client := New(WithBaseURL(srv.URL), WithAPIKey("k"))
	ctx := context.Background()

	msg, err := client.ATS.Messages.Create(ctx, &ATSMessagesCreateBody{
		Content:              "hi",
		SentByID:             "1",
		SentByType:           ATSMessagesCreateBodySentByTypeUser,
		ATSApplicationID:     "2",
		Attachments:          []File{{Name: "a.txt", Reader: strings.NewReader("AAA")}},
		Topic:                "t",
		SendAsCorporateEmail: true,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if want := "/api/2026-07-01/resources/ats/messages"; gotPath != want {
		t.Errorf("path = %q, want %q", gotPath, want)
	}
	if !strings.HasPrefix(gotContentType, "multipart/form-data") {
		t.Errorf("Content-Type = %q, want prefix multipart/form-data", gotContentType)
	}
	if !sawAttachments {
		t.Fatalf("no attachments[] file part observed by server")
	}
	if gotFileName != "a.txt" {
		t.Errorf("attachments[] filename = %q, want a.txt", gotFileName)
	}
	if gotFileContent != "AAA" {
		t.Errorf("attachments[] content = %q, want AAA", gotFileContent)
	}
	if msg == nil || msg.ID == nil || *msg.ID != "7" {
		t.Errorf("decoded ATSMessage ID = %v, want 7", msg)
	}
}

// TestNamespacesIntegration_Paginate asserts the generated Paginate helper
// threads Meta.EndCursor into the next request's after_id query param across a
// two-page walk, end-to-end through the real transport.
func TestNamespacesIntegration_Paginate(t *testing.T) {
	page := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		switch page {
		case 1:
			_, _ = w.Write([]byte(`{"data":[{"id":"1"}],"meta":{"end_cursor":"c1","has_next_page":true,"limit":100,"total":2}}`))
		case 2:
			if q := r.URL.Query().Get("after_id"); q != "c1" {
				t.Errorf("page 2 after_id = %q, want c1", q)
			}
			_, _ = w.Write([]byte(`{"data":[{"id":"2"}],"meta":{"has_next_page":false,"limit":100,"total":2}}`))
		default:
			t.Errorf("unexpected page %d", page)
		}
	}))
	t.Cleanup(srv.Close)

	client := New(WithBaseURL(srv.URL), WithAPIKey("k"))
	all, err := client.Teams.Teams.All(context.Background(), &TeamsTeamsListParams{})
	if err != nil {
		t.Fatalf("All: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("All collected %d, want 2", len(all))
	}
	got := []string{}
	for _, tm := range all {
		if tm.ID != nil {
			got = append(got, *tm.ID)
		}
	}
	if !reflect.DeepEqual(got, []string{"1", "2"}) {
		t.Errorf("All ids = %v, want [1 2]", got)
	}
}

// TestNamespacesIntegration_ObjectParamFormExplode asserts that the two
// object query params of the performance namespace are form-explode
// flattened: their properties are sent as TOP-LEVEL query keys and the param
// name itself never reaches the wire (matching the official TS SDK's
// style:'form' serializer override and the Python SDK's params.update()).
func TestNamespacesIntegration_ObjectParamFormExplode(t *testing.T) {
	var query string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/2026-07-01/resources/performance/review_process_targets" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		query = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[],"meta":{"has_next_page":false}}`))
	}))
	t.Cleanup(srv.Close)

	client := New(WithBaseURL(srv.URL), WithAPIKey("k"))
	_, err := client.Performance.ReviewProcessTargets.List(context.Background(), &PerformanceReviewProcessTargetsListParams{
		IDs: []string{"7"},
		ManagedByFilter: &PerformanceReviewProcessTargetsListParamsManagedByFilter{
			ManagerEmployeeID: "42",
			OnlyDirectReports: true,
		},
	})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	for _, want := range []string{"ids%5B%5D=7", "manager_employee_id=42", "only_direct_reports=true"} {
		if !strings.Contains(query, want) {
			t.Errorf("query %q does not contain %q", query, want)
		}
	}
	if strings.Contains(query, "managed_by_filter") {
		t.Errorf("query %q leaks the object param name managed_by_filter", query)
	}
}
