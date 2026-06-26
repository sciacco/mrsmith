package binocolo

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/sciacco/mrsmith/internal/platform/brave"
	"github.com/sciacco/mrsmith/internal/platform/httputil"
	"github.com/sciacco/mrsmith/internal/platform/llm"
	"github.com/sciacco/mrsmith/internal/platform/logging"
)

// WebSearchRequest is the body for POST /binocolo/v1/web-search: an ad-hoc,
// site-restricted keyword search backed by Brave LLM-context. The flow is
// stateless — nothing is persisted. When Rank is set, the results are reordered
// by an LLM relevance score against the searched keywords.
type WebSearchRequest struct {
	Domain   string   `json:"domain"`
	Keywords []string `json:"keywords"`
	Count    int      `json:"count"`
	Rank     bool     `json:"rank"`
}

// WebSearchResult is one source page with the snippets Brave extracted for the query.
type WebSearchResult struct {
	Title    string   `json:"title"`
	URL      string   `json:"url"`
	Hostname string   `json:"hostname"`
	Age      string   `json:"age,omitempty"`
	Snippets []string `json:"snippets"`
	Score    *int     `json:"score,omitempty"` // 0-100 LLM relevance; nil when not ranked
}

type WebSearchResponse struct {
	Query     string            `json:"query"`
	Count     int               `json:"count"`
	Ranked    bool              `json:"ranked"`              // true when results carry LLM relevance scores
	RankError string            `json:"rankError,omitempty"` // set when ranking was requested but failed
	Results   []WebSearchResult `json:"results"`
}

const (
	webSearchDefaultCount  = 25
	webSearchMaxCount      = 50
	webSearchMaxQueryLen   = 400 // Brave: q is 1-400 chars
	webSearchMaxWords      = 50  // Brave: q is max 50 words
	webSearchScoreTextCap  = 400 // chars of snippet text sent to the scorer per result
	webSearchScoreMaxToken = 512
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

	// Optional LLM relevance ranking. Best-effort: a scoring failure (e.g. scope not
	// configured) must never break the search — we just return the Brave order.
	ranked := false
	rankErr := ""
	if body.Rank && len(results) > 0 {
		subject, email := companySearchRefreshActor(r.Context())
		scores, scoreErr := h.ma.scoreWebSearchResults(r.Context(), strings.Join(keywords, " "), results, subject, email)
		if scoreErr != nil {
			rankErr = scoreErr.Error()
			logging.FromContext(r.Context()).Warn(
				"brave web search scoring failed",
				"component", "binocolo",
				"operation", "web_search_score",
				"error", scoreErr,
			)
		} else {
			for i := range results {
				if sc, ok := scores[i]; ok {
					v := sc
					results[i].Score = &v
				}
			}
			sort.SliceStable(results, func(a, b int) bool {
				return scoreOf(results[a]) > scoreOf(results[b])
			})
			ranked = true
		}
	}

	httputil.JSON(w, http.StatusOK, WebSearchResponse{
		Query:     query,
		Count:     count,
		Ranked:    ranked,
		RankError: rankErr,
		Results:   results,
	})
}

// scoreWebSearchResults asks the configured LLM to rate each result's relevance to
// the searched terms in a SINGLE batched call. Token-frugal by design: only
// {index, title, truncated text} go up, only {index, score} come back — no
// rationale. Returns index→score (0-100). Any failure is returned to the caller,
// which keeps the unranked order.
func (s *maService) scoreWebSearchResults(ctx context.Context, terms string, results []WebSearchResult, subject, email string) (map[int]int, error) {
	model, err := s.llmp.ResolveModel(ctx, maModelScopeWebSearchScorer, "")
	if err != nil {
		return nil, err
	}
	prompt, err := s.llmp.ResolvePrompt(ctx, maModelScopeWebSearchScorer, "")
	if err != nil {
		return nil, err
	}
	client, err := s.llmp.ClientForModel(ctx, model)
	if err != nil {
		return nil, err
	}

	type scoreItem struct {
		I     int    `json:"i"`
		Title string `json:"title"`
		Text  string `json:"text"`
	}
	items := make([]scoreItem, 0, len(results))
	for i, r := range results {
		items = append(items, scoreItem{
			I:     i,
			Title: r.Title,
			Text:  truncateRunes(strings.Join(r.Snippets, " "), webSearchScoreTextCap),
		})
	}
	input, err := json.Marshal(map[string]any{"terms": terms, "results": items})
	if err != nil {
		return nil, err
	}

	// Sampling params are dynamic, sourced from the model's DB config; the scope's
	// default max_tokens applies only when the config omits it.
	reqParams := model.RawParams()
	if _, ok := reqParams["max_tokens"]; !ok {
		reqParams["max_tokens"] = webSearchScoreMaxToken
	}

	chatReq := llm.ChatRequest{
		Model:          model.Model,
		Params:         reqParams,
		ResponseFormat: &llm.ResponseFormat{Type: "json_object"},
		Messages: []llm.Message{
			{Role: "system", Content: prompt.Prompt},
			{Role: "user", Content: string(input)},
		},
	}
	resp, chatErr := client.Chat(ctx, chatReq)

	// Best-effort audit. request is the exact wire body (BuildRequestBody = what Chat
	// sends), so dynamic params (temperature, max_tokens, reasoning_effort, …) are
	// recorded faithfully. Internal FKs live in the model_id/prompt_id columns.
	usageRaw, _ := json.Marshal(resp.Usage)
	requestBody, _ := llm.BuildRequestBody(chatReq)
	requestRaw, _ := json.Marshal(requestBody)
	audit := llm.CallAudit{
		App:          maApp,
		Scope:        maModelScopeWebSearchScorer,
		ProviderID:   model.ProviderID,
		ModelID:      model.ID,
		PromptID:     prompt.ID,
		Model:        model.Model,
		Request:      requestRaw,
		Usage:        usageRaw,
		ActorSubject: subject,
		ActorEmail:   email,
	}
	if chatErr != nil {
		audit.Status = "failed"
		audit.ErrorMessage = chatErr.Error()
	} else if respRaw, mErr := json.Marshal(map[string]any{"content": resp.Content}); mErr == nil {
		audit.Response = respRaw
	}
	_ = s.llmp.RecordAudit(ctx, audit)

	if chatErr != nil {
		return nil, chatErr
	}

	// The model may wrap the JSON in prose, markdown fences, or a reasoning block;
	// extract the object before parsing, and surface the raw content on failure.
	content := extractJSONObject(resp.Content)
	if content == "" {
		return nil, fmt.Errorf("scorer returned non-JSON content: %s", truncateRunes(strings.TrimSpace(resp.Content), 200))
	}
	var parsed struct {
		Scores []struct {
			I     int `json:"i"`
			Score int `json:"score"`
		} `json:"scores"`
	}
	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		return nil, fmt.Errorf("scorer JSON parse: %w (content: %s)", err, truncateRunes(strings.TrimSpace(resp.Content), 200))
	}
	out := make(map[int]int, len(parsed.Scores))
	for _, sc := range parsed.Scores {
		if sc.I < 0 || sc.I >= len(results) {
			continue
		}
		out[sc.I] = max(0, min(100, sc.Score))
	}
	if len(out) == 0 {
		// JSON parsed but no usable indices — treat as a failure so the caller can
		// surface it instead of silently returning an unscored "ranked" list.
		return nil, fmt.Errorf("scorer returned no valid scores (content: %s)", truncateRunes(resp.Content, 200))
	}
	return out, nil
}

// scoreOf returns a result's score, or -1 when unscored so it sinks to the bottom.
func scoreOf(r WebSearchResult) int {
	if r.Score == nil {
		return -1
	}
	return *r.Score
}

// extractJSONObject pulls the JSON object out of an LLM response that may wrap it in
// markdown fences, a <think> reasoning block, or surrounding prose. Returns "" when
// no object is present.
func extractJSONObject(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.Index(s, "</think>"); i >= 0 {
		s = s[i+len("</think>"):]
	}
	if i := strings.Index(s, "```"); i >= 0 {
		rest := s[i+3:]
		rest = strings.TrimPrefix(rest, "json")
		rest = strings.TrimPrefix(rest, "JSON")
		if j := strings.Index(rest, "```"); j >= 0 {
			rest = rest[:j]
		}
		s = rest
	}
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start < 0 || end < 0 || end < start {
		return ""
	}
	return s[start : end+1]
}

func truncateRunes(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max])
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
