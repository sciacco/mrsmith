package applaunch

import "strings"

// URLResolver resolves environment-aware base URLs and deep links for any app,
// using the same href resolution as Catalog (prod defaults overlaid with the
// hrefOverrides map main.go builds from *_APP_URL env vars / dev port defaults).
//
// It deliberately takes no StaticDir and hardcodes no ports: the dev-vs-prod
// decision is already encoded in the override map by the composition root.
type URLResolver struct {
	hrefs map[string]string
}

// NewURLResolver snapshots the resolved href for every catalog app. Passing nil
// yields the production defaults (handy for tests).
func NewURLResolver(hrefOverrides map[string]string) *URLResolver {
	r := &URLResolver{hrefs: make(map[string]string)}
	for _, d := range Catalog(hrefOverrides) {
		r.hrefs[d.ID] = d.Href
	}
	return r
}

// Base returns the environment-aware base URL for appID — e.g.
// "http://localhost:5192" in split-server dev, "/apps/ordini/" in production.
// Returns "" for an unknown appID.
func (r *URLResolver) Base(appID string) string {
	if r == nil {
		return ""
	}
	return r.hrefs[appID]
}

// Link composes a deep link into appID: the app base (without trailing slash)
// joined to path (without leading slash). Example:
// Link(OrdiniAppID, "ordini/123") -> "/apps/ordini/ordini/123" (prod) or
// "http://localhost:5192/ordini/123" (dev).
func (r *URLResolver) Link(appID, path string) string {
	return strings.TrimRight(r.Base(appID), "/") + "/" + strings.TrimLeft(path, "/")
}
