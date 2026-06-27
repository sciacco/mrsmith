package binocolo

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"unicode"

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

type DomainResolutionRequest struct {
	CompanyName string   `json:"companyName"`
	VATCode     string   `json:"vatCode,omitempty"`
	TaxCode     string   `json:"taxCode,omitempty"`
	Town        string   `json:"town,omitempty"`
	Province    string   `json:"province,omitempty"`
	Keywords    []string `json:"keywords,omitempty"`
	Count       int      `json:"count"`
}

type DomainResolutionCandidate struct {
	Domain     string            `json:"domain"`
	Score      int               `json:"score"`
	Confidence string            `json:"confidence"`
	Reasons    []string          `json:"reasons"`
	Results    []WebSearchResult `json:"results"`
}

type DomainResolutionResponse struct {
	Query      string                      `json:"query"`
	Count      int                         `json:"count"`
	Candidates []DomainResolutionCandidate `json:"candidates"`
	Results    []WebSearchResult           `json:"results"`
}

const (
	webSearchDefaultCount        = 25
	webSearchMaxCount            = 50
	webSearchMaxQueryLen         = 400 // Brave: q is 1-400 chars
	webSearchMaxWords            = 50  // Brave: q is max 50 words
	webSearchScoreTextCap        = 400 // chars of snippet text sent to the scorer per result
	webSearchScoreMaxToken       = 512
	domainResolutionDefaultCount = 10
	domainResolutionMaxCount     = 20
)

var (
	domainResolutionURLPattern   = regexp.MustCompile(`https?://[^[:space:]"'<>\\)]+`)
	domainResolutionEmailPattern = regexp.MustCompile(`[A-Za-z0-9._%+\-]+@([A-Za-z0-9.\-]+\.[A-Za-z]{2,})`)
)

// handleTestDomainResolution is a lab-only helper used by the Binocolo Test page.
// It tries to infer a target's official domain from Brave results, returning the
// raw candidate evidence and deterministic reasons. It deliberately does not
// persist anything and should not be treated as an authoritative source.
func (h *Handler) handleTestDomainResolution(w http.ResponseWriter, r *http.Request) {
	if !h.requireBrave(w) {
		return
	}

	var body DomainResolutionRequest
	if err := decodeMABody(r, &body); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_json")
		return
	}

	companyName := strings.Join(strings.Fields(strings.TrimSpace(body.CompanyName)), " ")
	if companyName == "" {
		httputil.Error(w, http.StatusBadRequest, "missing_company_name")
		return
	}

	count := body.Count
	if count <= 0 {
		count = domainResolutionDefaultCount
	}
	if count > domainResolutionMaxCount {
		count = domainResolutionMaxCount
	}

	query := buildDomainResolutionQuery(body, companyName)
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
		h.braveFailure(w, r, "domain_resolution", err)
		return
	}

	results := make([]WebSearchResult, 0, len(res.Generic))
	for _, g := range res.Generic {
		host := resultHostname(g.URL, res.Sources)
		if strings.TrimSpace(host) == "" {
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

	candidates := rankDomainResolutionCandidates(body, companyName, results)
	httputil.JSON(w, http.StatusOK, DomainResolutionResponse{
		Query:      query,
		Count:      count,
		Candidates: candidates,
		Results:    results,
	})
}

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

func buildDomainResolutionQuery(body DomainResolutionRequest, companyName string) string {
	parts := []string{quoteBraveTerm(companyName), "sito ufficiale"}
	if value := sanitizeDomainResolutionIdentifier(body.VATCode); value != "" {
		parts = append(parts, quoteBraveTerm(value))
	}
	if value := sanitizeDomainResolutionIdentifier(body.TaxCode); value != "" && value != sanitizeDomainResolutionIdentifier(body.VATCode) {
		parts = append(parts, quoteBraveTerm(value))
	}
	if town := strings.TrimSpace(body.Town); town != "" {
		parts = append(parts, quoteBraveTerm(town))
	}
	if province := strings.TrimSpace(body.Province); province != "" {
		parts = append(parts, strings.ToUpper(province))
	}
	for _, keyword := range body.Keywords {
		keyword = strings.TrimSpace(keyword)
		if keyword == "" {
			continue
		}
		parts = append(parts, quoteBraveTerm(keyword))
		if len(parts) >= 10 {
			break
		}
	}
	return strings.Join(parts, " ")
}

func quoteBraveTerm(value string) string {
	value = strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
	if value == "" {
		return ""
	}
	if strings.ContainsAny(value, " \t") {
		return `"` + strings.ReplaceAll(value, `"`, "") + `"`
	}
	return strings.ReplaceAll(value, `"`, "")
}

type domainResolutionAccum struct {
	domain  string
	score   int
	reasons map[string]struct{}
	results []WebSearchResult
}

type domainResolutionHint struct {
	domain  string
	bonus   int
	reason  string
	reasons []string
}

func rankDomainResolutionCandidates(body DomainResolutionRequest, companyName string, results []WebSearchResult) []DomainResolutionCandidate {
	accums := map[string]*domainResolutionAccum{}
	for _, result := range results {
		sourceDomain, _ := normalizeDomain(result.Hostname)
		for _, hint := range extractDomainResolutionHints(companyName, sourceDomain, result) {
			score, reasons := scoreDomainResolutionResult(body, companyName, hint.domain, result)
			score += hint.bonus
			if hint.reason != "" {
				reasons = append(reasons, hint.reason)
			}
			reasons = append(reasons, hint.reasons...)
			addDomainResolutionAccum(accums, hint.domain, min(100, score), reasons, result)
		}

		domain, ok := normalizeDomain(result.Hostname)
		if !ok || isDomainResolutionExcludedDomain(domain) {
			continue
		}
		score, reasons := scoreDomainResolutionResult(body, companyName, domain, result)
		addDomainResolutionAccum(accums, domain, score, reasons, result)
	}

	candidates := make([]DomainResolutionCandidate, 0, len(accums))
	for _, accum := range accums {
		score := min(100, accum.score+min(15, (len(accum.results)-1)*3))
		reasons := make([]string, 0, len(accum.reasons))
		for reason := range accum.reasons {
			reasons = append(reasons, reason)
		}
		sort.Strings(reasons)
		candidates = append(candidates, DomainResolutionCandidate{
			Domain:     accum.domain,
			Score:      score,
			Confidence: domainResolutionConfidence(score, reasons),
			Reasons:    reasons,
			Results:    accum.results,
		})
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].Score == candidates[j].Score {
			return candidates[i].Domain < candidates[j].Domain
		}
		return candidates[i].Score > candidates[j].Score
	})
	if len(candidates) > 8 {
		candidates = candidates[:8]
	}
	return candidates
}

func addDomainResolutionAccum(
	accums map[string]*domainResolutionAccum,
	domain string,
	score int,
	reasons []string,
	result WebSearchResult,
) {
	if score <= 0 {
		return
	}
	accum := accums[domain]
	if accum == nil {
		accum = &domainResolutionAccum{
			domain:  domain,
			reasons: map[string]struct{}{},
		}
		accums[domain] = accum
	}
	if score > accum.score {
		accum.score = score
	}
	for _, reason := range reasons {
		if reason = strings.TrimSpace(reason); reason != "" {
			accum.reasons[reason] = struct{}{}
		}
	}
	for _, existing := range accum.results {
		if existing.URL == result.URL {
			return
		}
	}
	accum.results = append(accum.results, result)
}

func extractDomainResolutionHints(
	companyName string,
	sourceDomain string,
	result WebSearchResult,
) []domainResolutionHint {
	text := result.Title + " " + result.URL + " " + strings.Join(result.Snippets, " ")
	tokens := companyResolutionTokens(companyName)
	hintsByDomain := map[string]*domainResolutionHint{}

	addHint := func(domain string, bonus int, reason string) {
		domain, ok := normalizeDomain(domain)
		if !ok || domain == "" {
			return
		}
		if sourceDomain != "" && hostMatchesDomain(domain, sourceDomain) {
			return
		}
		if isDomainResolutionExcludedDomain(domain) || isGenericEmailDomain(domain) {
			return
		}
		hint := hintsByDomain[domain]
		if hint == nil {
			hint = &domainResolutionHint{domain: domain}
			hintsByDomain[domain] = hint
		}
		if bonus > hint.bonus {
			hint.bonus = bonus
			hint.reason = reason
		}
		if reason != "" {
			hint.reasons = append(hint.reasons, reason)
		}
	}

	for _, match := range domainResolutionURLPattern.FindAllStringIndex(text, -1) {
		raw := text[match[0]:match[1]]
		domain, ok := normalizeDomain(raw)
		if !ok {
			continue
		}
		context := strings.ToLower(text[max(0, match[0]-96):min(len(text), match[1]+96)])
		if isDomainResolutionExampleContext(context) {
			continue
		}

		brandCompatible := domainLooksCompanyOwned(domain, tokens)
		switch {
		case strings.Contains(context, "sameas"):
			addHint(domain, 30, "dominio citato come sameAs")
		case strings.Contains(context, "sito web ufficiale") ||
			strings.Contains(context, "pagina web ufficiale") ||
			strings.Contains(context, "sito ufficiale"):
			addHint(domain, 28, "dominio citato come sito ufficiale")
		case brandCompatible:
			addHint(domain, 18, "dominio citato da fonte terza")
		}
	}

	for _, match := range domainResolutionEmailPattern.FindAllStringSubmatchIndex(text, -1) {
		if len(match) < 4 || match[2] < 0 || match[3] < 0 {
			continue
		}
		domain := text[match[2]:match[3]]
		normalized, ok := normalizeDomain(domain)
		if !ok || !domainLooksCompanyOwned(normalized, tokens) {
			continue
		}
		addHint(normalized, 24, "email aziendale su dominio")
	}

	hints := make([]domainResolutionHint, 0, len(hintsByDomain))
	for _, hint := range hintsByDomain {
		hint.reasons = cleanStringList(hint.reasons, 6, 80)
		hints = append(hints, *hint)
	}
	sort.SliceStable(hints, func(i, j int) bool {
		if hints[i].bonus == hints[j].bonus {
			return hints[i].domain < hints[j].domain
		}
		return hints[i].bonus > hints[j].bonus
	})
	return hints
}

func isDomainResolutionExampleContext(context string) bool {
	for _, marker := range []string{"non ha fornito", "es.", "esempio", "example"} {
		if strings.Contains(context, marker) {
			return true
		}
	}
	return false
}

func scoreDomainResolutionResult(body DomainResolutionRequest, companyName, domain string, result WebSearchResult) (int, []string) {
	text := strings.ToLower(result.Title + " " + result.URL + " " + strings.Join(result.Snippets, " "))
	compactText := compactAlnum(text)
	tokens := companyResolutionTokens(companyName)
	score := 0
	reasons := []string{}

	matchedNameTokens := 0
	for _, token := range tokens {
		if strings.Contains(text, token) || strings.Contains(compactAlnum(domain), token) {
			matchedNameTokens++
		}
	}
	if matchedNameTokens > 0 {
		score += min(35, matchedNameTokens*8)
		reasons = append(reasons, "nome azienda presente")
	}
	if domainLooksCompanyOwned(domain, tokens) {
		score += 30
		reasons = append(reasons, "host compatibile con ragione sociale")
	}
	if value := sanitizeDomainResolutionIdentifier(body.VATCode); value != "" && strings.Contains(compactText, strings.ToLower(value)) {
		score += 35
		reasons = append(reasons, "partita IVA trovata")
	}
	if value := sanitizeDomainResolutionIdentifier(body.TaxCode); value != "" && strings.Contains(compactText, strings.ToLower(value)) {
		score += 35
		reasons = append(reasons, "codice fiscale trovato")
	}
	if town := strings.ToLower(strings.TrimSpace(body.Town)); town != "" && strings.Contains(text, town) {
		score += 8
		reasons = append(reasons, "localita coerente")
	}
	keywordMatches := 0
	for _, keyword := range body.Keywords {
		for _, token := range companyResolutionTokens(keyword) {
			if strings.Contains(text, token) {
				keywordMatches++
			}
		}
	}
	if keywordMatches > 0 {
		score += min(12, keywordMatches*4)
		reasons = append(reasons, "keyword settore presenti")
	}
	for _, marker := range []string{"sito ufficiale", "homepage", "chi siamo", "contatti", "azienda"} {
		if strings.Contains(text, marker) {
			score += 5
			reasons = append(reasons, "indicatori sito aziendale")
			break
		}
	}
	if isHostedSiteDomain(domain) {
		score -= 12
		reasons = append(reasons, "dominio hosted da verificare")
	}
	if score < 0 {
		score = 0
	}
	if len(reasons) == 0 {
		reasons = append(reasons, "evidenza debole")
	}
	return min(100, score), cleanStringList(reasons, 8, 80)
}

func domainResolutionConfidence(score int, reasons []string) string {
	hasHostMatch := false
	hasCitedDomain := false
	for _, reason := range reasons {
		if reason == "host compatibile con ragione sociale" {
			hasHostMatch = true
		}
		if reason == "dominio citato come sameAs" ||
			reason == "dominio citato come sito ufficiale" ||
			reason == "email aziendale su dominio" {
			hasCitedDomain = true
		}
	}
	switch {
	case score >= 75 && (hasHostMatch || hasCitedDomain):
		return "alta"
	case score >= 45 && (hasHostMatch || hasCitedDomain):
		return "media"
	default:
		return "bassa"
	}
}

func companyResolutionTokens(value string) []string {
	seen := map[string]struct{}{}
	out := []string{}
	for _, token := range strings.FieldsFunc(strings.ToLower(value), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	}) {
		token = strings.TrimSpace(token)
		if len([]rune(token)) < 3 || domainResolutionStopword(token) {
			continue
		}
		if _, exists := seen[token]; exists {
			continue
		}
		seen[token] = struct{}{}
		out = append(out, token)
	}
	return out
}

func domainLooksCompanyOwned(domain string, tokens []string) bool {
	host := strings.TrimSuffix(strings.TrimPrefix(strings.ToLower(domain), "www."), ".")
	labels := strings.Split(host, ".")
	if len(labels) == 0 {
		return false
	}
	brand := compactAlnum(labels[0])
	if brand == "" {
		return false
	}
	strongMatches := 0
	for _, token := range tokens {
		compactToken := compactAlnum(token)
		if len(compactToken) < 3 {
			continue
		}
		if len(compactToken) == 3 {
			if brand == compactToken || (strings.HasPrefix(brand, compactToken) && len(brand) <= 16) {
				strongMatches++
			}
			continue
		}
		if strings.Contains(brand, compactToken) {
			strongMatches++
		}
	}
	return strongMatches >= 1 && (len(tokens) == 1 || strongMatches >= 2 || len(brand) <= 18)
}

func sanitizeDomainResolutionIdentifier(value string) string {
	return strings.ToUpper(compactAlnum(value))
}

func compactAlnum(value string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(value) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func domainResolutionStopword(token string) bool {
	switch token {
	case "srl", "spa", "srls", "soc", "societa", "cooperativa", "coop", "consorzio", "azienda", "italia", "italiana", "group", "holding":
		return true
	default:
		return false
	}
}

func isDomainResolutionExcludedDomain(domain string) bool {
	excluded := []string{
		"facebook.com",
		"instagram.com",
		"linkedin.com",
		"twitter.com",
		"x.com",
		"youtube.com",
		"google.com",
		"paginegialle.it",
		"registroimprese.it",
		"ufficiocamerali.it",
		"ufficio-camerale.it",
		"reportaziende.it",
		"informazione-aziende.it",
		"aziende.it",
		"misterimprese.it",
		"cylex-italia.it",
		"europages.it",
		"kompass.com",
		"kompassitalia.com",
		"indeed.com",
		"glassdoor.it",
		"crif.it",
		"cerved.com",
	}
	for _, item := range excluded {
		if hostMatchesDomain(domain, item) {
			return true
		}
	}
	return false
}

func isHostedSiteDomain(domain string) bool {
	for _, item := range []string{"wixsite.com", "wordpress.com", "blogspot.com", "weebly.com", "jimdosite.com", "business.site"} {
		if hostMatchesDomain(domain, item) {
			return true
		}
	}
	return false
}

func isGenericEmailDomain(domain string) bool {
	for _, item := range []string{
		"pec.it",
		"pec-mail.eu",
		"gmail.com",
		"googlemail.com",
		"outlook.com",
		"hotmail.com",
		"live.com",
		"icloud.com",
		"yahoo.com",
		"libero.it",
		"virgilio.it",
		"alice.it",
		"tim.it",
		"tiscali.it",
	} {
		if hostMatchesDomain(domain, item) {
			return true
		}
	}
	return false
}
