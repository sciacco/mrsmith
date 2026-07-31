package factorial

import (
	"context"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// --- local test types --------------------------------------------------------

// testStatus is a `type X string` enum; its presence here proves that enum
// fields serialize via stringify as their string value, not via fmt's default
// "%!..." rendering.
type testStatus string

const (
	statusActive   testStatus = "active"
	statusArchived testStatus = "archived"
)

// uploadBody mixes every field kind the multipart encoder must handle.
type uploadBody struct {
	Name        string     `json:"name"`
	Count       int        `json:"count"`
	On          bool       `json:"on"`
	Tags        []string   `json:"tags[]"`
	Doc         File       `json:"file"`
	Attachments []File     `json:"attachments[]"`
	Omitted     *string    `json:"omitted,omitempty"`
	Status      testStatus `json:"status"`
}

type testSchema struct {
	OK bool `json:"ok"`
}

// --- helpers -----------------------------------------------------------------

func assertFileContent(t *testing.T, fh *multipart.FileHeader, name, want string) {
	t.Helper()
	if fh.Filename != name {
		t.Errorf("filename = %q, want %q", fh.Filename, name)
	}
	f, err := fh.Open()
	if err != nil {
		t.Fatalf("open file part %q: %v", name, err)
	}
	defer f.Close()
	got, err := io.ReadAll(f)
	if err != nil {
		t.Fatalf("read file part %q: %v", name, err)
	}
	if string(got) != want {
		t.Errorf("file %q content = %q, want %q", name, got, want)
	}
}

// --- doMultipart -----------------------------------------------------------

func TestDoMultipart(t *testing.T) {
	const payload = `{"ok":true}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		if r.URL.Path != "/api/2026-07-01/resources/x/y" {
			t.Errorf("URL path = %q, want /api/2026-07-01/resources/x/y", r.URL.Path)
		}
		if ct := r.Header.Get("Content-Type"); !strings.HasPrefix(ct, "multipart/form-data; boundary=") {
			t.Errorf("Content-Type = %q, want multipart/form-data; boundary=…", ct)
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Errorf("ParseMultipartForm: %v", err)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		f := r.MultipartForm

		if got := f.Value["name"]; len(got) != 1 || got[0] != "widget" {
			t.Errorf("name = %v, want [widget]", got)
		}
		if got := f.Value["count"]; len(got) != 1 || got[0] != "3" {
			t.Errorf("count = %v, want [3]", got)
		}
		if got := f.Value["on"]; len(got) != 1 || got[0] != "true" {
			t.Errorf("on = %v, want [true]", got)
		}
		if got := f.Value["status"]; len(got) != 1 || got[0] != "active" {
			t.Errorf("status = %v, want [active] (enum serialized as string)", got)
		}
		if got := f.Value["tags[]"]; len(got) != 2 || got[0] != "a" || got[1] != "b" {
			t.Errorf("tags[] = %v, want [a b]", got)
		}
		if _, ok := f.Value["omitted"]; ok {
			t.Errorf("omitted must be absent (nil pointer skipped)")
		}

		fileParts := f.File["file"]
		if len(fileParts) != 1 {
			t.Fatalf("file parts for \"file\" = %d, want 1", len(fileParts))
		}
		assertFileContent(t, fileParts[0], "doc.txt", "doc-content")

		attParts := f.File["attachments[]"]
		if len(attParts) != 2 {
			t.Fatalf("attachments[] parts = %d, want 2 (both named attachments[])", len(attParts))
		}
		assertFileContent(t, attParts[0], "a1.txt", "content-1")
		assertFileContent(t, attParts[1], "a2.txt", "content-2")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(payload))
	}))
	t.Cleanup(srv.Close)

	c := New(WithBaseURL(srv.URL))
	body := uploadBody{
		Name:  "widget",
		Count: 3,
		On:    true,
		Tags:  []string{"a", "b"},
		Doc:   File{Name: "doc.txt", Reader: strings.NewReader("doc-content")},
		Attachments: []File{
			{Name: "a1.txt", Reader: strings.NewReader("content-1")},
			{Name: "a2.txt", Reader: strings.NewReader("content-2")},
		},
		Status: statusActive,
	}
	got, err := doMultipart[testSchema](context.Background(), c, "/resources/x/y", &body)
	if err != nil {
		t.Fatalf("doMultipart: %v", err)
	}
	if !got.OK {
		t.Errorf("decoded response = %+v, want {OK:true}", got)
	}
}

// --- doMultipartUpdate -----------------------------------------------------

func TestDoMultipartUpdate(t *testing.T) {
	const payload = `{"ok":true}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("method = %q, want PUT", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/resources/x/y/42") {
			t.Errorf("URL path = %q, want suffix /resources/x/y/42", r.URL.Path)
		}
		if ct := r.Header.Get("Content-Type"); !strings.HasPrefix(ct, "multipart/form-data; boundary=") {
			t.Errorf("Content-Type = %q, want multipart/form-data; boundary=…", ct)
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Errorf("ParseMultipartForm: %v", err)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		f := r.MultipartForm

		// The runtime must inject id == "42" (path id) even though the body
		// struct omits it — mirroring doUpdate's JSON id-injection.
		if got := f.Value["id"]; len(got) != 1 || got[0] != "42" {
			t.Errorf("id = %v, want [42] (injected)", got)
		}
		if got := f.Value["name"]; len(got) != 1 || got[0] != "widget" {
			t.Errorf("name = %v, want [widget] (body fields still present)", got)
		}
		if got := f.Value["status"]; len(got) != 1 || got[0] != "active" {
			t.Errorf("status = %v, want [active]", got)
		}
		fileParts := f.File["file"]
		if len(fileParts) != 1 {
			t.Errorf("file parts = %d, want 1", len(fileParts))
		} else {
			assertFileContent(t, fileParts[0], "doc.txt", "doc-content")
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(payload))
	}))
	t.Cleanup(srv.Close)

	c := New(WithBaseURL(srv.URL))
	body := uploadBody{
		Name:   "widget",
		Doc:    File{Name: "doc.txt", Reader: strings.NewReader("doc-content")},
		Status: statusActive,
	}
	got, err := doMultipartUpdate[testSchema](context.Background(), c, "/resources/x/y", "42", &body)
	if err != nil {
		t.Fatalf("doMultipartUpdate: %v", err)
	}
	if !got.OK {
		t.Errorf("decoded response = %+v", got)
	}
}

// --- error mapping ---------------------------------------------------------

func TestDoMultipartErrorMapping(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"error":"validation failed"}`))
	}))
	t.Cleanup(srv.Close)

	c := New(WithBaseURL(srv.URL))
	body := uploadBody{Name: "x"}
	_, err := doMultipart[testSchema](context.Background(), c, "/resources/x/y", &body)

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError via errors.As, got %T: %v", err, err)
	}
	if apiErr.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("StatusCode = %d, want 422", apiErr.StatusCode)
	}
	if apiErr.Method != http.MethodPost {
		t.Errorf("Method = %q, want POST", apiErr.Method)
	}
	if !strings.HasSuffix(apiErr.URL, "/resources/x/y") {
		t.Errorf("URL = %q", apiErr.URL)
	}
}

func TestDoMultipartUpdateErrorMapping(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"error":"bad"}`))
	}))
	t.Cleanup(srv.Close)

	c := New(WithBaseURL(srv.URL))
	body := uploadBody{Name: "x"}
	_, err := doMultipartUpdate[testSchema](context.Background(), c, "/resources/x/y", "9", &body)

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError via errors.As, got %T: %v", err, err)
	}
	if apiErr.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("StatusCode = %d, want 422", apiErr.StatusCode)
	}
	if apiErr.Method != http.MethodPut {
		t.Errorf("Method = %q, want PUT", apiErr.Method)
	}
	if !strings.HasSuffix(apiErr.URL, "/resources/x/y/9") {
		t.Errorf("URL = %q", apiErr.URL)
	}
}

// A Reader that always fails surfaces as a wrapped multipart error and never
// reaches the wire. This exercises the io.Copy error branch in writeFile and
// its propagation through the []File and encodeStruct loops.

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errors.New("disk gone") }

func TestDoMultipartFileCopyError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("request must not reach the server when the body fails to encode")
	}))
	t.Cleanup(srv.Close)

	c := New(WithBaseURL(srv.URL))
	body := struct {
		Files []File `json:"files[]"`
	}{
		Files: []File{{Name: "broken.txt", Reader: errReader{}}},
	}
	_, err := doMultipart[testSchema](context.Background(), c, "/resources/x/y", &body)
	if err == nil {
		t.Fatal("expected file-copy error, got nil")
	}
	if !strings.Contains(err.Error(), "writing file part") {
		t.Errorf("error must wrap the file-part copy failure, got %q", err.Error())
	}
}

// --- nil / typed-nil body --------------------------------------------------

func TestDoMultipartNilBody(t *testing.T) {
	var sawCT bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data; boundary=") {
			sawCT = true
		}
		// An empty multipart body (just the closing boundary) must still parse.
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Errorf("ParseMultipartForm on empty body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(srv.Close)

	c := New(WithBaseURL(srv.URL))

	// Untyped nil interface.
	got, err := doMultipart[testSchema](context.Background(), c, "/resources/x/y", nil)
	if err != nil {
		t.Fatalf("doMultipart nil body: %v", err)
	}
	if !got.OK {
		t.Errorf("decoded = %+v, want {OK:true}", got)
	}
	if !sawCT {
		t.Errorf("nil body must still send a multipart Content-Type")
	}

	// Typed-nil pointer (a nil *uploadBody arriving as a non-nil `any`).
	var nilBody *uploadBody
	if _, err := doMultipart[testSchema](context.Background(), c, "/resources/x/y", nilBody); err != nil {
		t.Fatalf("doMultipart typed-nil body: %v", err)
	}
}

// --- all scalar kinds + edge-case field names -----------------------------
//
// Exercises the stringify branches not covered by uploadBody (uint, float, the
// default fmt.Sprint fallback for non-scalar values), the json:",omitempty"
// empty-name fallback to the Go field name, and the unexported-field skip in
// encodeStruct. Also exercises a nil element inside []File (writeFile skip).

type innerThing struct{ X int }

type scalarBody struct {
	Amount   float64    `json:"amount"`
	Quantity uint64     `json:"quantity"`
	Inner    innerThing `json:"inner"`      // non-scalar → stringify default branch
	Omit     string     `json:",omitempty"` // empty tag name → Go field name
	Hidden   string     `json:"-"`          // skipped by fieldName
	Note     *string    `json:"note"`       // non-nil pointer → deref + stringify
	unexp    string     // unexported → skipped
	Files    []File     `json:"files[]"`
}

func TestDoMultipartScalarKindsAndEdges(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Errorf("ParseMultipartForm: %v", err)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		f := r.MultipartForm
		if got := f.Value["amount"]; len(got) != 1 || got[0] != "12.5" {
			t.Errorf("amount = %v, want [12.5]", got)
		}
		if got := f.Value["quantity"]; len(got) != 1 || got[0] != "7" {
			t.Errorf("quantity = %v, want [7] (uint)", got)
		}
		if got := f.Value["inner"]; len(got) != 1 || got[0] != "{5}" {
			t.Errorf("inner = %v, want [{5}] (default fmt.Sprint)", got)
		}
		// json:",omitempty" has an empty name → falls back to Go field name.
		if got := f.Value["Omit"]; len(got) != 1 || got[0] != "kept" {
			t.Errorf("Omit = %v, want [kept] (empty-tag name fallback)", got)
		}
		// A non-nil *string is dereferenced and stringified.
		if got := f.Value["note"]; len(got) != 1 || got[0] != "kept-note" {
			t.Errorf("note = %v, want [kept-note] (non-nil pointer deref)", got)
		}
		if _, ok := f.Value["Hidden"]; ok {
			t.Errorf("json:\"-\" field must be skipped")
		}
		if _, ok := f.Value["unexp"]; ok {
			t.Errorf("unexported field must be skipped")
		}
		// []File with one nil-reader element: only the one real file arrives.
		if parts := f.File["files[]"]; len(parts) != 1 {
			t.Errorf("files[] = %d parts, want 1 (nil-reader element skipped)", len(parts))
		} else {
			assertFileContent(t, parts[0], "only.txt", "payload")
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(srv.Close)

	c := New(WithBaseURL(srv.URL))
	note := "kept-note"
	body := scalarBody{
		Amount:   12.5,
		Quantity: 7,
		Inner:    innerThing{X: 5},
		Omit:     "kept",
		Hidden:   "must-not-send",
		Note:     &note,
		unexp:    "secret",
		Files: []File{
			{Name: "skipped.txt"}, // nil Reader → skipped
			{Name: "only.txt", Reader: strings.NewReader("payload")},
		},
	}
	got, err := doMultipart[testSchema](context.Background(), c, "/resources/x/y", &body)
	if err != nil {
		t.Fatalf("doMultipart: %v", err)
	}
	if !got.OK {
		t.Errorf("decoded = %+v", got)
	}
}

// --- non-struct body is a safe no-op --------------------------------------

func TestDoMultipartNonStructBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Errorf("ParseMultipartForm: %v", err)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		// A non-struct body encodes nothing; the form is empty.
		if len(r.MultipartForm.Value) != 0 || len(r.MultipartForm.File) != 0 {
			t.Errorf("non-struct body must encode no parts, got value=%v file=%v",
				r.MultipartForm.Value, r.MultipartForm.File)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(srv.Close)

	c := New(WithBaseURL(srv.URL))
	// A nil map arrives as a non-nil interface wrapping a zero Map value; the
	// reflection walk must not panic and must encode nothing.
	var m map[string]int
	if _, err := doMultipart[testSchema](context.Background(), c, "/resources/x/y", m); err != nil {
		t.Fatalf("doMultipart non-struct body: %v", err)
	}
}

// --- nil Reader inside a File is skipped -----------------------------------

func TestDoMultipartNilReaderSkipped(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Errorf("ParseMultipartForm: %v", err)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		f := r.MultipartForm
		if parts, ok := f.File["file"]; ok && len(parts) > 0 {
			t.Errorf("file part must be absent (nil Reader skipped), got %d", len(parts))
		}
		if got := f.Value["name"]; len(got) != 1 || got[0] != "x" {
			t.Errorf("name = %v, want [x] (non-file fields still sent)", got)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(srv.Close)

	c := New(WithBaseURL(srv.URL))
	body := struct {
		Name string `json:"name"`
		Doc  File   `json:"file"`
	}{
		Name: "x",
		Doc:  File{Name: "ignored.txt"}, // Reader nil → skipped
	}
	if _, err := doMultipart[testSchema](context.Background(), c, "/resources/x/y", &body); err != nil {
		t.Fatalf("doMultipart nil reader: %v", err)
	}
}
