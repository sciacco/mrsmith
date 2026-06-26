package binocolo

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/sciacco/mrsmith/internal/platform/brave"
	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

// WebSearchRequest is the body for POST /binocolo/v1/web-search: an ad-hoc,
// site-restricted keyword search backed by Brave LLM-context. The flow is
// stateless — nothing is persisted.
type WebSearchRequest struct {
	Domain   string   `json:"domain"`
	Keywords []string `json:"keywords"`
	Count    int      `json:"count"`
}

// WebSearchResult is one source page with the snippets Brave extracted for the query.
type WebSearchResult struct {
	Title    string   `json:"title"`
	URL      string   `json:"url"`
	Hostname string   `json:"hostname"`
	Age      string   `json:"age,omitempty"`
	Snippets []string `json:"snippets"`
}

type WebSearchResponse struct {
	Query   string            `json:"query"`
	Count   int               `json:"count"`
	Results []WebSearchResult `json:"results"`
}

const (
	webSearchDefaultCount = 25
	webSearchMaxCount     = 50
	webSearchMaxQueryLen  = 400 // Brave: q is 1-400 chars
	webSearchMaxWords     = 50  // Brave: q is max 50 words
)

// handleWebSearch restricts the search to a single domain via the "site:" operator
// in the query and, as a safety net, drops any result whose hostname is not on that
// domain — so the caller only ever sees results from the requested site even if the
// operator is not honoured upstream.
func (h *Handler) handleWebSearch(w http.ResponseWriter, r *http.Request) {
	if !h.requireBrave(w) {
		return
	}

	var body WebSearchRequest
	if err := decodeMABody(r, &body); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_json")
		return
	}

	domain, ok := normalizeDomain(body.Domain)
	if !ok {
		httputil.Error(w, http.StatusBadRequest, "invalid_domain")
		return
	}

	keywords := make([]string, 0, len(body.Keywords))
	for _, k := range body.Keywords {
		if k = strings.TrimSpace(k); k != "" {
			keywords = append(keywords, k)
		}
	}
	if len(keywords) == 0 {
		httputil.Error(w, http.StatusBadRequest, "missing_keywords")
		return
	}

	count := body.Count
	if count <= 0 {
		count = webSearchDefaultCount
	}
	if count > webSearchMaxCount {
		count = webSearchMaxCount
	}

	query := "site:" + domain + " " + strings.Join(keywords, " ")
	if len(query) > webSearchMaxQueryLen || len(strings.Fields(query)) > webSearchMaxWords {
		httputil.Error(w, http.StatusBadRequest, "query_too_long")
		return
	}

	res, err := h.brave.LLMContext(r.Context(), brave.LLMContextParams{
		Query:      query,
		Count:      count,
		Country:    "it",
		SearchLang: "it",
	})
	if err != nil {
		h.braveFailure(w, r, "web_search", err)
		return
	}

	results := make([]WebSearchResult, 0, len(res.Generic))
	for _, g := range res.Generic {
		host := resultHostname(g.URL, res.Sources)
		if !hostMatchesDomain(host, domain) {
			continue
		}
		results = append(results, WebSearchResult{
			Title:    g.Title,
			URL:      g.URL,
			Hostname: host,
			Age:      firstAge(res.Sources[g.URL].Age),
			Snippets: g.Snippets,
		})
	}

	httputil.JSON(w, http.StatusOK, WebSearchResponse{
		Query:   query,
		Count:   count,
		Results: results,
	})
}

// normalizeDomain reduces a pasted value (e.g. "https://www.azienda.it/chi-siamo")
// to a bare registrable host ("azienda.it"). Returns false when the input cannot be
// reduced to a plausible domain.
func normalizeDomain(raw string) (string, bool) {
	s := strings.ToLower(strings.TrimSpace(raw))
	if s == "" {
		return "", false
	}
	if i := strings.Index(s, "://"); i >= 0 {
		s = s[i+3:]
	}
	if i := strings.IndexAny(s, "/?#"); i >= 0 {
		s = s[:i]
	}
	if i := strings.LastIndex(s, "@"); i >= 0 {
		s = s[i+1:]
	}
	if i := strings.LastIndex(s, ":"); i >= 0 {
		s = s[:i]
	}
	s = strings.TrimPrefix(s, "www.")
	s = strings.Trim(s, ".")
	if !strings.Contains(s, ".") {
		return "", false
	}
	for _, c := range s {
		if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '.' || c == '-') {
			return "", false
		}
	}
	return s, true
}

// hostMatchesDomain reports whether host belongs to domain (exact or subdomain).
func hostMatchesDomain(host, domain string) bool {
	host = strings.TrimPrefix(strings.Trim(strings.ToLower(strings.TrimSpace(host)), "."), "www.")
	return host == domain || strings.HasSuffix(host, "."+domain)
}

// resultHostname prefers the hostname from the response "sources" metadata and
// falls back to parsing the URL.
func resultHostname(rawURL string, sources map[string]brave.SourceMeta) string {
	if meta, ok := sources[rawURL]; ok {
		if h := strings.TrimSpace(meta.Hostname); h != "" {
			return h
		}
	}
	if u, err := url.Parse(rawURL); err == nil {
		return u.Hostname()
	}
	return ""
}

func firstAge(age []string) string {
	for _, a := range age {
		if s := strings.TrimSpace(a); s != "" {
			return s
		}
	}
	return ""
}
