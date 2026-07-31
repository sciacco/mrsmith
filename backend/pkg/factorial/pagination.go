package factorial

import (
	"context"
	"fmt"
	"iter"
)

// PaginateOption configures the behavior of paginate and paginatePages.
type PaginateOption func(*paginateConfig)

// paginateConfig holds resolved pagination options. It is unexported; callers
// configure it via PaginateOption values (WithMaxItems / WithPageSize).
type paginateConfig struct {
	// maxItems caps the number of ITEMS (not pages) an iterator yields. A
	// value of 0 means unlimited.
	maxItems int
	// pageSize, when > 0, is injected as the Limit query parameter on every
	// request via params.setPagination. A value of 0 leaves Limit unset (the
	// server default applies).
	pageSize int
}

// newPaginateConfig applies opts to a zeroed config.
func newPaginateConfig(opts []PaginateOption) paginateConfig {
	cfg := paginateConfig{}
	for _, opt := range opts {
		opt(&cfg)
	}
	return cfg
}

// WithMaxItems caps the total number of items (not pages) an iterator yields.
// n <= 0 disables the cap (iterate until the server says there is no next
// page). This is the primary tool for bounding large or unbounded walks.
func WithMaxItems(n int) PaginateOption {
	return func(cfg *paginateConfig) { cfg.maxItems = n }
}

// WithPageSize injects the given page size as the Limit query parameter on
// every request. n <= 0 leaves Limit unset and the server default applies.
func WithPageSize(n int) PaginateOption {
	return func(cfg *paginateConfig) { cfg.pageSize = n }
}

// lister is the constraint for paginated list params: they must encode to
// query values (so doList can hand them to the transport) AND accept injected
// cursor/limit controls. Every generated List params struct implements both:
// encode() comes from the params emitter and setPagination() is injected by
// the generator (see docs/golden/namespaces.go.golden). By embedding
// queryEncoder (defined in client.go) here, listFetcher can pass the params
// straight to doList.
type lister interface {
	queryEncoder
	setPagination(afterID *string, limit *int)
}

// listFetcher fetches a single page via doList. It is the fetch function
// passed to paginate / paginatePages by the generated List methods, e.g.
// `paginate(ctx, c, path, params, listFetcher[T], opts...)`. It takes a single
// type argument (the item type) so it is assignable to the fetch parameter of
// paginate/paginatePages, which is typed against the lister interface.
func listFetcher[T any](ctx context.Context, c *Client, path string, params lister) (*Page[T], error) {
	return doList[T](ctx, c, path, params)
}

// paginate returns an item-level iterator that walks a cursor-paginated List
// endpoint. It threads Meta.EndCursor from one page into the next request's
// AfterID via params.setPagination, yields each item in page.Data, honors
// WithMaxItems / WithPageSize, and stops on: server end-of-results
// (!HasNextPage or nil EndCursor), a consumer break (yield returns false), an
// error (yielded once as (zero, err)), or a repeated cursor (the server failed
// to advance — yielded once as (zero, err) to avoid an infinite loop).
func paginate[T any, P lister](ctx context.Context, c *Client, path string, params P, fetch func(context.Context, *Client, string, lister) (*Page[T], error), opts ...PaginateOption) iter.Seq2[T, error] {
	cfg := newPaginateConfig(opts)
	return func(yield func(T, error) bool) {
		var afterID *string
		yielded := 0
		for {
			var limit *int
			if cfg.pageSize > 0 {
				ps := cfg.pageSize
				limit = &ps
			}
			params.setPagination(afterID, limit)

			page, err := fetch(ctx, c, path, params)
			if err != nil {
				yield(zero[T](), err)
				return
			}

			for _, it := range page.Data {
				if !yield(it, nil) {
					return
				}
				yielded++
				if cfg.maxItems > 0 && yielded >= cfg.maxItems {
					return
				}
			}

			if !page.Meta.HasNextPage || page.Meta.EndCursor == nil {
				return
			}
			if afterID != nil && *page.Meta.EndCursor == *afterID {
				yield(zero[T](), fmt.Errorf("factorial: pagination stalled: server returned the same end_cursor %q twice", *afterID))
				return
			}
			cur := *page.Meta.EndCursor
			afterID = &cur
		}
	}
}

// paginatePages returns a page-level iterator that yields whole *Page[T]
// values. maxItems still counts ITEMS (the sum of len(page.Data) across
// yielded pages), so a WithMaxItems cap that falls between page boundaries
// stops as soon as the cumulative item count reaches the cap. The same
// error / stop / repeated-cursor rules as paginate apply.
func paginatePages[T any, P lister](ctx context.Context, c *Client, path string, params P, fetch func(context.Context, *Client, string, lister) (*Page[T], error), opts ...PaginateOption) iter.Seq2[*Page[T], error] {
	cfg := newPaginateConfig(opts)
	return func(yield func(*Page[T], error) bool) {
		var afterID *string
		yielded := 0
		for {
			var limit *int
			if cfg.pageSize > 0 {
				ps := cfg.pageSize
				limit = &ps
			}
			params.setPagination(afterID, limit)

			page, err := fetch(ctx, c, path, params)
			if err != nil {
				yield(nil, err)
				return
			}

			if !yield(page, nil) {
				return
			}
			yielded += len(page.Data)
			if cfg.maxItems > 0 && yielded >= cfg.maxItems {
				return
			}

			if !page.Meta.HasNextPage || page.Meta.EndCursor == nil {
				return
			}
			if afterID != nil && *page.Meta.EndCursor == *afterID {
				yield(nil, fmt.Errorf("factorial: pagination stalled: server returned the same end_cursor %q twice", *afterID))
				return
			}
			cur := *page.Meta.EndCursor
			afterID = &cur
		}
	}
}

// collectAll drains an item iterator into a slice, returning the accumulated
// items and the first error encountered. On error the partial items collected
// so far are returned alongside err.
func collectAll[T any](seq iter.Seq2[T, error]) ([]T, error) {
	var all []T
	for item, err := range seq {
		if err != nil {
			return all, err
		}
		all = append(all, item)
	}
	return all, nil
}

// zero returns the zero value of T. It is used for the single error yield
// emitted by paginate / paginatePages on a fetch error or stall.
func zero[T any]() T { var z T; return z }
