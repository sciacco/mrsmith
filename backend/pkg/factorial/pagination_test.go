package factorial

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
)

// item is a throwaway element type for pagination tests.
type item struct {
	V int
}

// testParams is a minimal lister implementation: it records the cursor/limit
// injected by the helpers and encodes to an empty query string (irrelevant to
// these tests, which use a fake fetch).
type testParams struct {
	AfterID *string
	Limit   *int
}

func (p *testParams) encode() url.Values { return url.Values{} }

func (p *testParams) setPagination(afterID *string, limit *int) {
	p.AfterID = afterID
	if limit != nil {
		p.Limit = limit
	}
}

// fetchRecorder captures the (afterID, limit) pairs a fetch func is called
// with so tests can assert cursor threading and option application.
type fetchRecorder struct {
	calls    int
	afterIDs []string // "" when the incoming AfterID was nil
	limits   []*int
}

func (rec *fetchRecorder) fetch(pages map[string]*Page[item], errOn map[string]string) func(context.Context, *Client, string, lister) (*Page[item], error) {
	return func(_ context.Context, _ *Client, _ string, params lister) (*Page[item], error) {
		rec.calls++
		tp := params.(*testParams)
		key := ""
		if tp.AfterID != nil {
			key = *tp.AfterID
		}
		rec.afterIDs = append(rec.afterIDs, key)
		rec.limits = append(rec.limits, tp.Limit)
		if msg, ok := errOn[key]; ok {
			return nil, errors.New(msg)
		}
		p, ok := pages[key]
		if !ok {
			panic("fake fetch: unexpected afterID key " + key)
		}
		return p, nil
	}
}

func strPtr(s string) *string { return &s }
func intPtr(i int) *int       { return &i }

// makePage builds a canned *Page[item].
func makePage(items []item, endCursor *string, hasNext bool) *Page[item] {
	return &Page[item]{Data: items, Meta: PagedMeta{EndCursor: endCursor, HasNextPage: hasNext}}
}

// threePageWalk is the shared multi-page scenario: 3 pages of sizes 2, 2, 1.
func threePageWalk() map[string]*Page[item] {
	return map[string]*Page[item]{
		"":   makePage([]item{{1}, {2}}, strPtr("c1"), true),
		"c1": makePage([]item{{3}, {4}}, strPtr("c2"), true),
		"c2": makePage([]item{{5}}, nil, false),
	}
}

func TestPaginateMultiPageWalk(t *testing.T) {
	ctx := context.Background()
	c := &Client{}
	rec := &fetchRecorder{}
	seq := paginate[item](ctx, c, "/x", &testParams{}, rec.fetch(threePageWalk(), nil))
	got, err := collectAll(seq)
	if err != nil {
		t.Fatalf("collectAll err = %v, want nil", err)
	}
	if len(got) != 5 {
		t.Fatalf("len(got) = %d, want 5", len(got))
	}
	for i, v := range got {
		if v.V != i+1 {
			t.Errorf("got[%d].V = %d, want %d", i, v.V, i+1)
		}
	}
	if rec.calls != 3 {
		t.Errorf("fetch calls = %d, want 3", rec.calls)
	}
	wantAfter := []string{"", "c1", "c2"}
	if !equalSlices(rec.afterIDs, wantAfter) {
		t.Errorf("afterID threading = %v, want %v", rec.afterIDs, wantAfter)
	}
}

func TestPaginateMaxItemsMidPage(t *testing.T) {
	ctx := context.Background()
	c := &Client{}
	pages := map[string]*Page[item]{
		"": makePage([]item{{1}, {2}, {3}}, strPtr("c1"), true),
	}
	rec := &fetchRecorder{}
	var got []item
	for x, err := range paginate[item](ctx, c, "/x", &testParams{}, rec.fetch(pages, nil), WithMaxItems(2)) {
		if err != nil {
			t.Fatalf("unexpected err = %v", err)
		}
		got = append(got, x)
	}
	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2", len(got))
	}
	if got[0].V != 1 || got[1].V != 2 {
		t.Errorf("got = %+v, want [{1} {2}]", got)
	}
	if rec.calls != 1 {
		t.Errorf("fetch calls = %d, want 1 (must stop inside first page)", rec.calls)
	}
}

func TestPaginateConsumerBreak(t *testing.T) {
	ctx := context.Background()
	c := &Client{}
	rec := &fetchRecorder{}
	for x, err := range paginate[item](ctx, c, "/x", &testParams{}, rec.fetch(threePageWalk(), nil)) {
		_ = x
		_ = err
		break
	}
	if rec.calls != 1 {
		t.Errorf("fetch calls = %d, want 1 (no second fetch after break)", rec.calls)
	}
}

func TestPaginateErrorPropagation(t *testing.T) {
	ctx := context.Background()
	c := &Client{}
	pages := map[string]*Page[item]{
		"": makePage([]item{{1}, {2}}, strPtr("c1"), true),
	}
	rec := &fetchRecorder{}
	got, err := collectAll(paginate[item](ctx, c, "/x", &testParams{}, rec.fetch(pages, map[string]string{"c1": "boom"})))
	if err == nil || err.Error() != "boom" {
		t.Fatalf("err = %v, want error %q", err, "boom")
	}
	if len(got) != 2 {
		t.Errorf("partial items = %d, want 2 (page 1 items before error)", len(got))
	}
	if rec.calls != 2 {
		t.Errorf("fetch calls = %d, want 2", rec.calls)
	}
}

func TestPaginateRepeatedCursorGuard(t *testing.T) {
	ctx := context.Background()
	c := &Client{}
	// The server never advances: both pages return the same EndCursor "X"
	// with HasNextPage=true. The guard must detect the stall and terminate.
	pages := map[string]*Page[item]{
		"":  makePage([]item{{1}}, strPtr("X"), true),
		"X": makePage([]item{{2}}, strPtr("X"), true),
	}
	rec := &fetchRecorder{}
	got, err := collectAll(paginate[item](ctx, c, "/x", &testParams{}, rec.fetch(pages, nil)))
	if err == nil {
		t.Fatal("err = nil, want stall error")
	}
	if !strings.Contains(err.Error(), "stalled") || !strings.Contains(err.Error(), "end_cursor") {
		t.Errorf("err = %q, want message containing 'stalled' and 'end_cursor'", err.Error())
	}
	if len(got) != 2 {
		t.Errorf("items before stall = %d, want 2", len(got))
	}
	if rec.calls != 2 {
		t.Errorf("fetch calls = %d, want 2", rec.calls)
	}
}

func TestPaginateWithPageSize(t *testing.T) {
	ctx := context.Background()
	c := &Client{}
	pages := map[string]*Page[item]{
		"": makePage([]item{{1}}, nil, false),
	}

	t.Run("sets limit", func(t *testing.T) {
		rec := &fetchRecorder{}
		_, _ = collectAll(paginate[item](ctx, c, "/x", &testParams{}, rec.fetch(pages, nil), WithPageSize(50)))
		if rec.calls != 1 || len(rec.limits) != 1 {
			t.Fatalf("calls = %d, limits = %v", rec.calls, rec.limits)
		}
		if rec.limits[0] == nil || *rec.limits[0] != 50 {
			t.Errorf("limit = %v, want 50", rec.limits[0])
		}
	})

	t.Run("omits limit when unset", func(t *testing.T) {
		rec := &fetchRecorder{}
		_, _ = collectAll(paginate[item](ctx, c, "/x", &testParams{}, rec.fetch(pages, nil)))
		if rec.calls != 1 || len(rec.limits) != 1 {
			t.Fatalf("calls = %d, limits = %v", rec.calls, rec.limits)
		}
		if rec.limits[0] != nil {
			t.Errorf("limit = %v, want nil when WithPageSize not used", rec.limits[0])
		}
	})
}

func TestPaginatePagesCleanWalk(t *testing.T) {
	ctx := context.Background()
	c := &Client{}
	rec := &fetchRecorder{}
	var got []*Page[item]
	for pg, err := range paginatePages[item](ctx, c, "/x", &testParams{}, rec.fetch(threePageWalk(), nil)) {
		if err != nil {
			t.Fatalf("unexpected err = %v", err)
		}
		got = append(got, pg)
	}
	if len(got) != 3 {
		t.Fatalf("pages yielded = %d, want 3", len(got))
	}
	if len(got[0].Data) != 2 || len(got[1].Data) != 2 || len(got[2].Data) != 1 {
		t.Errorf("page sizes = %d %d %d, want 2 2 1", len(got[0].Data), len(got[1].Data), len(got[2].Data))
	}
	if rec.calls != 3 {
		t.Errorf("fetch calls = %d, want 3", rec.calls)
	}
}

func TestPaginatePagesMaxItems(t *testing.T) {
	ctx := context.Background()
	c := &Client{}

	// Cap of 3 items: page1 has 2 (cumulative 2 < 3, continue), page2 has 2
	// (cumulative 4 >= 3, stop). Exactly 2 pages yielded.
	rec := &fetchRecorder{}
	var got []*Page[item]
	for pg, err := range paginatePages[item](ctx, c, "/x", &testParams{}, rec.fetch(threePageWalk(), nil), WithMaxItems(3)) {
		if err != nil {
			t.Fatalf("unexpected err = %v", err)
		}
		got = append(got, pg)
	}
	if len(got) != 2 {
		t.Fatalf("pages yielded = %d, want 2 (cap reached after page 2)", len(got))
	}
	if rec.calls != 2 {
		t.Errorf("fetch calls = %d, want 2", rec.calls)
	}

	// Cap of 5 items: page1(2)+page2(2)+page3(1) = 5 -> stops at end normally;
	// also yields exactly 3 pages.
	rec2 := &fetchRecorder{}
	var got2 []*Page[item]
	for pg, err := range paginatePages[item](ctx, c, "/x", &testParams{}, rec2.fetch(threePageWalk(), nil), WithMaxItems(5)) {
		if err != nil {
			t.Fatalf("unexpected err = %v", err)
		}
		got2 = append(got2, pg)
	}
	if len(got2) != 3 {
		t.Errorf("pages yielded with cap 5 = %d, want 3", len(got2))
	}
}

func TestPaginatePagesConsumerBreak(t *testing.T) {
	ctx := context.Background()
	c := &Client{}
	rec := &fetchRecorder{}
	for pg, err := range paginatePages[item](ctx, c, "/x", &testParams{}, rec.fetch(threePageWalk(), nil)) {
		_ = pg
		_ = err
		break
	}
	if rec.calls != 1 {
		t.Errorf("fetch calls = %d, want 1 (no second fetch after break)", rec.calls)
	}
}

func TestPaginatePagesErrorPropagation(t *testing.T) {
	ctx := context.Background()
	c := &Client{}
	pages := map[string]*Page[item]{
		"": makePage([]item{{1}, {2}}, strPtr("c1"), true),
	}
	rec := &fetchRecorder{}
	var got []*Page[item]
	var err error
	for pg, e := range paginatePages[item](ctx, c, "/x", &testParams{}, rec.fetch(pages, map[string]string{"c1": "boom"})) {
		if e != nil {
			err = e
			break
		}
		got = append(got, pg)
	}
	if err == nil || err.Error() != "boom" {
		t.Fatalf("err = %v, want error %q", err, "boom")
	}
	if len(got) != 1 {
		t.Errorf("pages before error = %d, want 1", len(got))
	}
}

func TestPaginatePagesRepeatedCursorGuard(t *testing.T) {
	ctx := context.Background()
	c := &Client{}
	pages := map[string]*Page[item]{
		"":  makePage([]item{{1}}, strPtr("X"), true),
		"X": makePage([]item{{2}}, strPtr("X"), true),
	}
	rec := &fetchRecorder{}
	var got []*Page[item]
	var err error
	for pg, e := range paginatePages[item](ctx, c, "/x", &testParams{}, rec.fetch(pages, nil)) {
		if e != nil {
			err = e
			break
		}
		got = append(got, pg)
	}
	if err == nil {
		t.Fatal("err = nil, want stall error")
	}
	if !strings.Contains(err.Error(), "stalled") || !strings.Contains(err.Error(), "end_cursor") {
		t.Errorf("err = %q, want message containing 'stalled' and 'end_cursor'", err.Error())
	}
	if len(got) != 2 {
		t.Errorf("pages before stall = %d, want 2", len(got))
	}
	if rec.calls != 2 {
		t.Errorf("fetch calls = %d, want 2", rec.calls)
	}
}

func TestPaginatePagesWithPageSize(t *testing.T) {
	ctx := context.Background()
	c := &Client{}
	pages := map[string]*Page[item]{
		"": makePage([]item{{1}}, nil, false),
	}
	rec := &fetchRecorder{}
	for range paginatePages[item](ctx, c, "/x", &testParams{}, rec.fetch(pages, nil), WithPageSize(7)) {
	}
	if rec.calls != 1 || len(rec.limits) != 1 {
		t.Fatalf("calls = %d, limits = %v", rec.calls, rec.limits)
	}
	if rec.limits[0] == nil || *rec.limits[0] != 7 {
		t.Errorf("limit = %v, want 7", rec.limits[0])
	}
}

func TestCollectAllClean(t *testing.T) {
	ctx := context.Background()
	c := &Client{}
	rec := &fetchRecorder{}
	got, err := collectAll(paginate[item](ctx, c, "/x", &testParams{}, rec.fetch(threePageWalk(), nil)))
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if len(got) != 5 {
		t.Errorf("len(got) = %d, want 5", len(got))
	}
}

// encParams is a lister whose encode() actually emits after_id/limit, so the
// listFetcher integration test can assert those controls reach the wire.
type encParams struct {
	AfterID *string
	Limit   *int
}

func (p *encParams) encode() url.Values {
	v := url.Values{}
	if p == nil {
		return v
	}
	if p.AfterID != nil {
		v.Set("after_id", *p.AfterID)
	}
	if p.Limit != nil {
		v.Set("limit", strconv.Itoa(*p.Limit))
	}
	return v
}

func (p *encParams) setPagination(afterID *string, limit *int) {
	p.AfterID = afterID
	if limit != nil {
		p.Limit = limit
	}
}

// TestListFetcher drives listFetcher through the real Client / doList path
// with an httptest server, covering the doList pass-through and confirming
// that injected after_id/limit reach the request query.
func TestListFetcher(t *testing.T) {
	var gotReq *http.Request
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotReq = r
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"data":[{"V":1},{"V":2}],"meta":{"has_next_page":false,"limit":50,"total":2}}`)
	}))
	t.Cleanup(srv.Close)

	c := New(WithBaseURL(srv.URL))
	params := &encParams{}
	params.setPagination(strPtr("abc"), intPtr(50))

	page, err := listFetcher[item](context.Background(), c, "/list", params)
	if err != nil {
		t.Fatalf("listFetcher err = %v, want nil", err)
	}
	if len(page.Data) != 2 || page.Data[0].V != 1 || page.Data[1].V != 2 {
		t.Errorf("page.Data = %+v, want [{1} {2}]", page.Data)
	}
	if page.Meta.HasNextPage {
		t.Errorf("HasNextPage = true, want false")
	}
	if gotReq == nil {
		t.Fatal("server never received a request")
	}
	if q := gotReq.URL.Query().Get("after_id"); q != "abc" {
		t.Errorf("after_id query = %q, want %q", q, "abc")
	}
	if q := gotReq.URL.Query().Get("limit"); q != "50" {
		t.Errorf("limit query = %q, want %q", q, "50")
	}
}

// equalSlices compares two string slices for equality.
func equalSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
