package binocolo

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/sciacco/mrsmith/internal/platform/llm"
)

// MADeepBriefEvalOptions configures the lightweight variance harness for the
// ma_deep_brief LLM. It intentionally writes file artifacts only: no product DB
// schema, no endpoint, no overwrite of production briefs.
type MADeepBriefEvalOptions struct {
	SampleSize  int
	Iterations  int
	OutputDir   string
	ModelID     string
	PromptID    string
	CompanyKeys []string // optional: reuse an existing manifest/sample instead of DB auto-selection
	DryRun      bool
	Progress    io.Writer
}

// MADeepBriefEvalReport points to the generated local artifacts.
type MADeepBriefEvalReport struct {
	ExperimentID   string
	OutputDir      string
	DryRun         bool
	Cases          int
	Runs           int
	ManifestPath   string
	OutputsPath    string
	SummaryCSVPath string
	DashboardPath  string
	NotesPath      string
}

type maDeepBriefEvalCase struct {
	Index           int       `json:"index"`
	CompanyKey      string    `json:"companyKey"`
	CompanyName     string    `json:"companyName,omitempty"`
	VATCode         string    `json:"vatCode,omitempty"`
	TaxCode         string    `json:"taxCode,omitempty"`
	UpdatedAt       time.Time `json:"updatedAt"`
	OverallRAG      string    `json:"overallRag,omitempty"`
	TurnoverYear    *int      `json:"turnoverYear,omitempty"`
	Turnover        *float64  `json:"turnover,omitempty"`
	Ebitda          *float64  `json:"ebitda,omitempty"`
	PFN             *float64  `json:"pfn,omitempty"`
	ValuationMethod string    `json:"valuationMethod,omitempty"`
	ValuationSector string    `json:"valuationSector,omitempty"`
	QualityFlags    []string  `json:"qualityFlags,omitempty"`
	InputFile       string    `json:"inputFile"`

	payload   json.RawMessage
	scorecard *MADeepScorecard
	valuation *MADeepValuation
	inputRaw  json.RawMessage
}

type maDeepBriefEvalManifest struct {
	ExperimentID string                `json:"experimentId"`
	CreatedAt    time.Time             `json:"createdAt"`
	Scope        string                `json:"scope"`
	Model        maDeepBriefEvalModel  `json:"model"`
	Prompt       maDeepBriefEvalPrompt `json:"prompt"`
	SampleSize   int                   `json:"sampleSize"`
	Iterations   int                   `json:"iterations"`
	DryRun       bool                  `json:"dryRun"`
	SelectionSQL string                `json:"selectionSql"`
	CompanyKeys  []string              `json:"companyKeys,omitempty"`
	Params       map[string]any        `json:"params"`
	Cases        []maDeepBriefEvalCase `json:"cases"`
	Rubric       map[string]int        `json:"rubric"`
	Notes        []string              `json:"notes"`
}

type maDeepBriefEvalModel struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Model     string `json:"model"`
	Provider  string `json:"providerId"`
	ParamsSHA string `json:"paramsSha"`
}

type maDeepBriefEvalPrompt struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	PromptSHA string `json:"promptSha"`
}

type maDeepBriefEvalRunRecord struct {
	ExperimentID string               `json:"experimentId"`
	Scope        string               `json:"scope"`
	CompanyKey   string               `json:"companyKey"`
	CompanyName  string               `json:"companyName,omitempty"`
	CaseIndex    int                  `json:"caseIndex"`
	Iteration    int                  `json:"iteration"`
	ModelID      string               `json:"modelId"`
	Model        string               `json:"model"`
	PromptID     string               `json:"promptId"`
	StartedAt    time.Time            `json:"startedAt"`
	LatencyMS    int                  `json:"latencyMs"`
	Usage        llm.Usage            `json:"usage"`
	Status       string               `json:"status"`
	Error        string               `json:"error,omitempty"`
	RawResponse  string               `json:"rawResponse,omitempty"`
	ParsedBrief  *MADeepBrief         `json:"parsedBrief,omitempty"`
	Score        maDeepBriefEvalScore `json:"score"`
	RequestBody  map[string]any       `json:"requestBody,omitempty"`
}

type maDeepBriefEvalScore struct {
	Total                  int             `json:"total"`
	Breakdown              map[string]int  `json:"breakdown"`
	Warnings               []string        `json:"warnings,omitempty"`
	SuspiciousNumbers      []string        `json:"suspiciousNumbers,omitempty"`
	JSONValid              bool            `json:"jsonValid"`
	RAGFlip                bool            `json:"ragFlip"`
	HallucinationCandidate bool            `json:"hallucinationCandidate"`
	FlagCoverage           map[string]bool `json:"flagCoverage,omitempty"`
}

type maDeepBriefEvalSummary struct {
	CaseIndex         int
	CompanyKey        string
	CompanyName       string
	Runs              int
	AvgScore          float64
	StdDevScore       float64
	MinScore          int
	MaxScore          int
	JSONFailRate      float64
	RAGFlipRate       float64
	HallucinationRate float64
	AvgLatencyMS      float64
	TotalTokens       int
}

const maDeepBriefEvalSelectionSQL = `
SELECT company_key, COALESCE(vat_code, ''), COALESCE(tax_code, ''), scorecard, valuation, itfull_payload, updated_at
FROM binocolo.ma_deep_analysis
WHERE status = 'ready'
  AND itfull_payload IS NOT NULL AND itfull_payload <> 'null'::jsonb
  AND scorecard IS NOT NULL AND scorecard <> 'null'::jsonb
  AND valuation IS NOT NULL AND valuation <> 'null'::jsonb
ORDER BY updated_at DESC, company_key ASC
LIMIT $1`

var maDeepBriefEvalRubric = map[string]int{
	"json_schema":           10,
	"rag_coherence":         10,
	"valuation_consistency": 25,
	"no_invented_numbers":   20,
	"caveat_coverage":       15,
	"dd_questions":          10,
	"clarity":               10,
}

func normalizedEvalCompanyKeys(keys []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(keys))
	for _, key := range keys {
		key = normalizeMACompanyKey(key)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, key)
	}
	return out
}

func evalSelectionDescription(keys []string) string {
	if len(normalizedEvalCompanyKeys(keys)) > 0 {
		return "company_key list from manifest/flag; production artifacts reloaded from binocolo.ma_deep_analysis"
	}
	return strings.TrimSpace(maDeepBriefEvalSelectionSQL)
}

// RunMADeepBriefEval executes the current ma_deep_brief prompt/model several times
// over a deterministic DB-selected sample and writes a local JSONL/CSV/HTML bundle.
func RunMADeepBriefEval(ctx context.Context, db *sql.DB, llmSvc *llm.Service, opts MADeepBriefEvalOptions) (*MADeepBriefEvalReport, error) {
	if db == nil {
		return nil, errors.New("binocolo llm eval: nil db")
	}
	if llmSvc == nil && !opts.DryRun {
		return nil, errors.New("binocolo llm eval: nil llm service")
	}
	if opts.SampleSize <= 0 {
		opts.SampleSize = 10
	}
	if opts.Iterations <= 0 {
		opts.Iterations = 10
	}
	if opts.Progress == nil {
		opts.Progress = io.Discard
	}

	provider := newMALLMProvider(llmSvc)
	if provider == nil {
		return nil, errors.New("binocolo llm eval: llm provider not configured")
	}
	model, err := provider.ResolveModel(ctx, maModelScopeDeepBrief, opts.ModelID)
	if err != nil {
		return nil, fmt.Errorf("resolve eval model: %w", err)
	}
	prompt, err := provider.ResolvePrompt(ctx, maModelScopeDeepBrief, opts.PromptID)
	if err != nil {
		return nil, fmt.Errorf("resolve eval prompt: %w", err)
	}

	cases, err := loadMADeepBriefEvalCases(ctx, db, opts.SampleSize, opts.CompanyKeys)
	if err != nil {
		return nil, err
	}
	if len(cases) == 0 {
		return nil, errors.New("binocolo llm eval: nessuna analisi ready con payload/scorecard/valuation")
	}

	experimentID := fmt.Sprintf("%s-%s", time.Now().UTC().Format("20060102T150405Z"), shortSHA(model.Model, 8))
	if strings.TrimSpace(opts.OutputDir) == "" {
		opts.OutputDir = defaultMADeepBriefEvalOutputDir(experimentID, model.Model)
	}
	if err := os.MkdirAll(filepath.Join(opts.OutputDir, "inputs"), 0o755); err != nil {
		return nil, fmt.Errorf("create eval output dir: %w", err)
	}

	paramsForManifest := model.RawParams()
	if _, ok := paramsForManifest["max_tokens"]; !ok {
		paramsForManifest["max_tokens"] = 5000
	}

	for i := range cases {
		inputRaw, err := buildMADeepBriefEvalInput(cases[i].payload, cases[i].scorecard, cases[i].valuation)
		if err != nil {
			return nil, fmt.Errorf("build eval input %s: %w", cases[i].CompanyKey, err)
		}
		cases[i].inputRaw = inputRaw
		inputName := fmt.Sprintf("%02d_%s.json", cases[i].Index, safeArtifactName(cases[i].CompanyKey))
		cases[i].InputFile = filepath.ToSlash(filepath.Join("inputs", inputName))
		if err := os.WriteFile(filepath.Join(opts.OutputDir, cases[i].InputFile), prettyJSON(inputRaw), 0o644); err != nil {
			return nil, fmt.Errorf("write eval input %s: %w", cases[i].CompanyKey, err)
		}
	}

	manifest := maDeepBriefEvalManifest{
		ExperimentID: experimentID,
		CreatedAt:    time.Now().UTC(),
		Scope:        maModelScopeDeepBrief,
		Model: maDeepBriefEvalModel{
			ID:        model.ID,
			Name:      model.Name,
			Model:     model.Model,
			Provider:  model.ProviderID,
			ParamsSHA: shortSHA(string(model.Params), 12),
		},
		Prompt: maDeepBriefEvalPrompt{
			ID:        prompt.ID,
			Name:      prompt.Name,
			PromptSHA: shortSHA(prompt.Prompt, 12),
		},
		SampleSize:   len(cases),
		Iterations:   opts.Iterations,
		DryRun:       opts.DryRun,
		SelectionSQL: evalSelectionDescription(opts.CompanyKeys),
		CompanyKeys:  normalizedEvalCompanyKeys(opts.CompanyKeys),
		Params:       paramsForManifest,
		Cases:        stripEvalCasePayloads(cases),
		Rubric:       maDeepBriefEvalRubric,
		Notes: []string{
			"La valuation numerica è deterministica: il test misura stabilità e fedeltà della narrativa LLM ma_deep_brief.",
			"Score euristico 0-100 per triage; la revisione FDD umana resta il giudice finale.",
			"Nessuna chiamata IT-full, nessuna scrittura sui brief di produzione.",
		},
	}

	report := &MADeepBriefEvalReport{
		ExperimentID: experimentID,
		OutputDir:    opts.OutputDir,
		DryRun:       opts.DryRun,
		Cases:        len(cases),
		ManifestPath: filepath.Join(opts.OutputDir, "case_manifest.json"),
		OutputsPath:  filepath.Join(opts.OutputDir, "outputs.jsonl"),
		NotesPath:    filepath.Join(opts.OutputDir, "notes.md"),
	}
	if err := writeJSONFile(report.ManifestPath, manifest); err != nil {
		return nil, err
	}
	if err := writeMADeepBriefEvalNotes(report.NotesPath); err != nil {
		return nil, err
	}
	if opts.DryRun {
		fmt.Fprintf(opts.Progress, "dry-run: manifest scritto in %s\n", report.ManifestPath)
		return report, nil
	}

	outputs, err := os.Create(report.OutputsPath)
	if err != nil {
		return nil, fmt.Errorf("create outputs.jsonl: %w", err)
	}
	defer outputs.Close()
	enc := json.NewEncoder(outputs)
	enc.SetEscapeHTML(false)

	client, err := provider.ClientForModel(ctx, model)
	if err != nil {
		return nil, fmt.Errorf("build eval client: %w", err)
	}

	runs := make([]maDeepBriefEvalRunRecord, 0, len(cases)*opts.Iterations)
	for ci := range cases {
		c := cases[ci]
		for iter := 1; iter <= opts.Iterations; iter++ {
			fmt.Fprintf(opts.Progress, "[%d/%d] %s iterazione %d/%d\n", ci+1, len(cases), c.CompanyKey, iter, opts.Iterations)
			rec := runMADeepBriefEvalIteration(ctx, provider, client, model, prompt, experimentID, c, iter)
			if err := enc.Encode(rec); err != nil {
				return nil, fmt.Errorf("write eval run: %w", err)
			}
			runs = append(runs, rec)
			report.Runs++
		}
	}

	report.SummaryCSVPath = filepath.Join(opts.OutputDir, "summary.csv")
	report.DashboardPath = filepath.Join(opts.OutputDir, "dashboard.html")
	summaries := summarizeMADeepBriefEval(cases, runs)
	if err := writeMADeepBriefEvalSummaryCSV(report.SummaryCSVPath, summaries); err != nil {
		return nil, err
	}
	if err := writeMADeepBriefEvalDashboard(report.DashboardPath, manifest, summaries, runs); err != nil {
		return nil, err
	}
	return report, nil
}

func loadMADeepBriefEvalCases(ctx context.Context, db *sql.DB, sampleSize int, companyKeys []string) ([]maDeepBriefEvalCase, error) {
	keys := normalizedEvalCompanyKeys(companyKeys)
	if len(keys) > 0 {
		return loadMADeepBriefEvalCasesByKey(ctx, db, keys)
	}
	rows, err := db.QueryContext(ctx, maDeepBriefEvalSelectionSQL, sampleSize)
	if err != nil {
		return nil, fmt.Errorf("select eval cases: %w", err)
	}
	defer rows.Close()
	return scanMADeepBriefEvalCases(rows)
}

func loadMADeepBriefEvalCasesByKey(ctx context.Context, db *sql.DB, companyKeys []string) ([]maDeepBriefEvalCase, error) {
	placeholders := make([]string, 0, len(companyKeys))
	args := make([]any, 0, len(companyKeys))
	for i, key := range companyKeys {
		placeholders = append(placeholders, "$"+strconv.Itoa(i+1))
		args = append(args, key)
	}
	query := `
SELECT company_key, COALESCE(vat_code, ''), COALESCE(tax_code, ''), scorecard, valuation, itfull_payload, updated_at
FROM binocolo.ma_deep_analysis
WHERE company_key IN (` + strings.Join(placeholders, ",") + `)
  AND status = 'ready'
  AND itfull_payload IS NOT NULL AND itfull_payload <> 'null'::jsonb
  AND scorecard IS NOT NULL AND scorecard <> 'null'::jsonb
  AND valuation IS NOT NULL AND valuation <> 'null'::jsonb`
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("select eval cases by key: %w", err)
	}
	defer rows.Close()
	items, err := scanMADeepBriefEvalCases(rows)
	if err != nil {
		return nil, err
	}
	byKey := make(map[string]maDeepBriefEvalCase, len(items))
	for _, item := range items {
		byKey[item.CompanyKey] = item
	}
	out := make([]maDeepBriefEvalCase, 0, len(companyKeys))
	for _, key := range companyKeys {
		item, ok := byKey[key]
		if !ok {
			return nil, fmt.Errorf("eval case %s non trovata o non ready con payload/scorecard/valuation", key)
		}
		item.Index = len(out) + 1
		out = append(out, item)
	}
	return out, nil
}

func scanMADeepBriefEvalCases(rows *sql.Rows) ([]maDeepBriefEvalCase, error) {
	var out []maDeepBriefEvalCase
	for rows.Next() {
		var c maDeepBriefEvalCase
		var scorecardRaw, valuationRaw, payloadRaw []byte
		if err := rows.Scan(&c.CompanyKey, &c.VATCode, &c.TaxCode, &scorecardRaw, &valuationRaw, &payloadRaw, &c.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan eval case: %w", err)
		}
		var sc MADeepScorecard
		if err := json.Unmarshal(scorecardRaw, &sc); err != nil {
			return nil, fmt.Errorf("decode scorecard %s: %w", c.CompanyKey, err)
		}
		var val MADeepValuation
		if err := json.Unmarshal(valuationRaw, &val); err != nil {
			return nil, fmt.Errorf("decode valuation %s: %w", c.CompanyKey, err)
		}
		c.Index = len(out) + 1
		c.scorecard = &sc
		c.valuation = &val
		c.payload = json.RawMessage(payloadRaw)
		c.OverallRAG = sc.OverallRAG
		c.TurnoverYear = sc.TurnoverYear
		c.Turnover = sc.Turnover
		c.Ebitda = sc.Ebitda
		c.PFN = sc.PFN
		c.ValuationMethod = val.Method
		c.ValuationSector = val.Sector
		for _, flag := range sc.QualityFlags {
			if strings.TrimSpace(flag.Code) != "" {
				c.QualityFlags = append(c.QualityFlags, flag.Code)
			}
		}
		if object, err := decodeVendorObject(c.payload); err == nil {
			root := deepFullRoot(object)
			c.CompanyName = firstVendorString(root, "companyDetails.companyName", "companyName")
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate eval cases: %w", err)
	}
	return out, nil
}

func buildMADeepBriefEvalInput(rawPayload json.RawMessage, scorecard *MADeepScorecard, valuation *MADeepValuation) (json.RawMessage, error) {
	briefInput := map[string]any{"scorecard": scorecard, "valuation": valuation}
	if company := curateITFullForBrief(rawPayload); company != nil {
		briefInput["company"] = company
	}
	input, err := json.Marshal(briefInput)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(input), nil
}

func runMADeepBriefEvalIteration(ctx context.Context, provider maLLMProvider, client maAIClient, model llm.Model, prompt llm.Prompt, experimentID string, c maDeepBriefEvalCase, iter int) maDeepBriefEvalRunRecord {
	params := model.RawParams()
	if _, ok := params["max_tokens"]; !ok {
		params["max_tokens"] = 5000
	}
	chatReq := llm.ChatRequest{
		Model:          model.Model,
		Params:         params,
		ResponseFormat: &llm.ResponseFormat{Type: "json_object"},
		Messages: []llm.Message{
			{Role: "system", Content: prompt.Prompt},
			{Role: "user", Content: string(c.inputRaw)},
		},
	}
	requestBody, _ := llm.BuildRequestBody(chatReq)
	requestRaw, _ := json.Marshal(requestBody)
	started := time.Now().UTC()
	resp, err := client.Chat(ctx, chatReq)
	latency := int(time.Since(started).Milliseconds())

	rec := maDeepBriefEvalRunRecord{
		ExperimentID: experimentID,
		Scope:        maModelScopeDeepBrief,
		CompanyKey:   c.CompanyKey,
		CompanyName:  c.CompanyName,
		CaseIndex:    c.Index,
		Iteration:    iter,
		ModelID:      model.ID,
		Model:        model.Model,
		PromptID:     prompt.ID,
		StartedAt:    started,
		LatencyMS:    latency,
		Usage:        resp.Usage,
		RequestBody:  requestBody,
	}
	usageRaw, _ := json.Marshal(resp.Usage)
	auditCtx, _ := json.Marshal(map[string]any{
		"eval":          true,
		"experiment_id": experimentID,
		"company_key":   c.CompanyKey,
		"iteration":     iter,
	})
	audit := llm.CallAudit{
		App:        maApp,
		Scope:      maModelScopeDeepBrief,
		ProviderID: model.ProviderID,
		ModelID:    model.ID,
		PromptID:   prompt.ID,
		Model:      model.Model,
		Request:    requestRaw,
		Usage:      usageRaw,
		Context:    auditCtx,
		DurationMS: &latency,
	}
	if err != nil {
		rec.Status = "failed"
		rec.Error = err.Error()
		rec.Score = scoreMADeepBriefEval(c, string(c.inputRaw), "", nil, err)
		audit.Status = "failed"
		audit.ErrorMessage = err.Error()
		_ = provider.RecordAudit(ctx, audit)
		return rec
	}
	rec.RawResponse = resp.Content
	respRaw, _ := json.Marshal(map[string]any{"content": resp.Content})
	audit.Response = respRaw
	_ = provider.RecordAudit(ctx, audit)

	brief, parseErr := parseMADeepBrief(resp.Content)
	if parseErr != nil {
		rec.Status = "parse_failed"
		rec.Error = parseErr.Error()
	} else {
		rec.Status = "succeeded"
		rec.ParsedBrief = brief
	}
	rec.Score = scoreMADeepBriefEval(c, string(c.inputRaw), resp.Content, brief, parseErr)
	return rec
}

func scoreMADeepBriefEval(c maDeepBriefEvalCase, inputText, rawResponse string, brief *MADeepBrief, parseErr error) maDeepBriefEvalScore {
	score := maDeepBriefEvalScore{Breakdown: map[string]int{}, FlagCoverage: map[string]bool{}}
	if parseErr != nil || brief == nil {
		score.JSONValid = false
		reason := "brief vuoto"
		if parseErr != nil {
			reason = parseErr.Error()
		}
		score.Warnings = append(score.Warnings, "JSON/schema non valido: "+reason)
		score.Breakdown["json_schema"] = 0
		score.Total = 0
		return score
	}
	score.JSONValid = true
	score.Breakdown["json_schema"] = 10

	expectedRAG := strings.ToLower(strings.TrimSpace(c.OverallRAG))
	actualRAG := strings.ToLower(strings.TrimSpace(brief.RAG))
	switch {
	case expectedRAG != "" && actualRAG == expectedRAG:
		score.Breakdown["rag_coherence"] = 10
	case actualRAG == "":
		score.Breakdown["rag_coherence"] = 3
		score.Warnings = append(score.Warnings, "RAG assente nel brief")
	default:
		score.Breakdown["rag_coherence"] = 0
		score.RAGFlip = true
		score.Warnings = append(score.Warnings, fmt.Sprintf("RAG flip: atteso %s, ricevuto %s", expectedRAG, actualRAG))
	}

	combined := maDeepBriefEvalText(brief)
	allowed := collectAllowedNumbersFromJSON([]byte(inputText))
	suspiciousAll := suspiciousNumberMentions(combined, allowed)
	suspiciousValuation := suspiciousNumberMentions(brief.ValuationRationale, allowed)
	score.SuspiciousNumbers = suspiciousAll
	if len(suspiciousAll) > 0 {
		score.HallucinationCandidate = true
		score.Warnings = append(score.Warnings, fmt.Sprintf("numeri sospetti fuori input: %s", strings.Join(suspiciousAll, "; ")))
	}

	valuationScore := 0
	if strings.TrimSpace(brief.ValuationRationale) != "" {
		valuationScore += 8
	}
	valuationText := strings.ToLower(brief.ValuationRationale)
	if methodMentioned(valuationText, c.valuation) {
		valuationScore += 5
	}
	if strings.Contains(valuationText, "ev") || strings.Contains(valuationText, "equity") || strings.Contains(valuationText, "pfn") || strings.Contains(valuationText, "multip") || strings.Contains(valuationText, "haircut") || strings.Contains(valuationText, "ponte") {
		valuationScore += 7
	}
	if len(suspiciousValuation) == 0 {
		valuationScore += 5
	}
	if valuationScore > 25 {
		valuationScore = 25
	}
	score.Breakdown["valuation_consistency"] = valuationScore

	inventedScore := 20 - int(math.Min(20, float64(len(suspiciousAll)*5)))
	if inventedScore < 0 {
		inventedScore = 0
	}
	score.Breakdown["no_invented_numbers"] = inventedScore

	caveatScore, coverage := scoreCaveatCoverage(c.scorecard, combined)
	score.Breakdown["caveat_coverage"] = caveatScore
	for k, v := range coverage {
		score.FlagCoverage[k] = v
		if !v {
			score.Warnings = append(score.Warnings, "caveat non citato: "+k)
		}
	}
	if len(score.FlagCoverage) == 0 {
		score.FlagCoverage = nil
	}

	ddScore := 0
	if len(brief.DDQuestions) >= 3 {
		ddScore += 5
	} else if len(brief.DDQuestions) > 0 {
		ddScore += 3
	}
	if avgQuestionLength(brief.DDQuestions) >= 45 {
		ddScore += 3
	}
	if redFlagsHaveQuestions(brief.RedFlags) {
		ddScore += 2
	}
	if ddScore > 10 {
		ddScore = 10
	}
	score.Breakdown["dd_questions"] = ddScore

	clarity := 0
	if strings.TrimSpace(brief.Verdict) != "" {
		clarity += 3
	}
	if strings.TrimSpace(brief.BusinessProfile) != "" {
		clarity += 2
	}
	if strings.TrimSpace(brief.FinancialReading) != "" {
		clarity += 3
	}
	if len(brief.Strengths) > 0 || len(brief.RedFlags) > 0 {
		clarity += 2
	}
	if len(combined) > 5000 && clarity > 0 {
		clarity--
		score.Warnings = append(score.Warnings, "output molto prolisso")
	}
	score.Breakdown["clarity"] = clarity

	for _, v := range score.Breakdown {
		score.Total += v
	}
	if score.Total > 100 {
		score.Total = 100
	}
	return score
}

func maDeepBriefEvalText(brief *MADeepBrief) string {
	if brief == nil {
		return ""
	}
	parts := []string{brief.Verdict, brief.RAG, brief.BusinessProfile, brief.FinancialReading, brief.ValuationRationale}
	parts = append(parts, brief.Strengths...)
	parts = append(parts, brief.DDQuestions...)
	for _, rf := range brief.RedFlags {
		parts = append(parts, rf.Severity, rf.Category, rf.Claim, rf.DDQuestion)
	}
	return strings.Join(parts, "\n")
}

func methodMentioned(text string, valuation *MADeepValuation) bool {
	if valuation == nil {
		return true
	}
	method := strings.ToLower(strings.TrimSpace(valuation.Method))
	lowMethod := strings.ToLower(strings.TrimSpace(valuation.LowMethod))
	if method == "" && lowMethod == "" {
		return true
	}
	aliases := []string{method, lowMethod}
	if strings.Contains(method, "ebitda") || strings.Contains(lowMethod, "ebitda") {
		aliases = append(aliases, "ev/ebitda", "ev ebitda")
	}
	if strings.Contains(method, "sales") || strings.Contains(lowMethod, "sales") {
		aliases = append(aliases, "ev/sales", "ev sales", "ricavi")
	}
	for _, a := range aliases {
		a = strings.TrimSpace(a)
		if a != "" && strings.Contains(text, a) {
			return true
		}
	}
	return false
}

func scoreCaveatCoverage(sc *MADeepScorecard, text string) (int, map[string]bool) {
	if sc == nil || len(sc.QualityFlags) == 0 {
		return 15, nil
	}
	coverage := map[string]bool{}
	text = strings.ToLower(text)
	matched := 0
	for _, flag := range sc.QualityFlags {
		code := strings.TrimSpace(flag.Code)
		if code == "" {
			continue
		}
		ok := flagMentioned(code, text)
		coverage[code] = ok
		if ok {
			matched++
		}
	}
	if len(coverage) == 0 {
		return 15, nil
	}
	return int(math.Round(15 * float64(matched) / float64(len(coverage)))), coverage
}

func flagMentioned(code, text string) bool {
	switch code {
	case "a4_capitalizzazioni":
		return strings.Contains(text, "a.4") || strings.Contains(text, "capitalizz") || strings.Contains(text, "lavori interni")
	case "a5_altri_ricavi":
		return strings.Contains(text, "a.5") || strings.Contains(text, "altri ricavi") || strings.Contains(text, "contribut")
	case "b8_beni_terzi":
		return strings.Contains(text, "b.8") || strings.Contains(text, "godimento") || strings.Contains(text, "leasing") || strings.Contains(text, "nolegg") || strings.Contains(text, "affitt")
	case "perimetro_standalone":
		return strings.Contains(text, "standalone") || strings.Contains(text, "partecip") || strings.Contains(text, "controllat") || strings.Contains(text, "consolidat") || strings.Contains(text, "somma delle parti")
	case "scarto_vendor_cee":
		return strings.Contains(text, "vendor") || strings.Contains(text, "cee") || strings.Contains(text, "riconcili")
	case "patrimonio_eroso":
		return strings.Contains(text, "patrimonio") || strings.Contains(text, "pn negativo") || strings.Contains(text, "equity negativa")
	case "leva_in_peggioramento":
		return strings.Contains(text, "leva") || (strings.Contains(text, "debito") && strings.Contains(text, "ebitda"))
	case "deriva_valore_aggiunto":
		return strings.Contains(text, "valore aggiunto") || strings.Contains(text, "pass-through") || strings.Contains(text, "rivendita")
	default:
		return strings.Contains(text, strings.ReplaceAll(code, "_", " "))
	}
}

func avgQuestionLength(items []string) int {
	if len(items) == 0 {
		return 0
	}
	total := 0
	for _, item := range items {
		total += len([]rune(strings.TrimSpace(item)))
	}
	return total / len(items)
}

func redFlagsHaveQuestions(flags []MADeepBriefFlag) bool {
	for _, flag := range flags {
		if strings.TrimSpace(flag.DDQuestion) != "" {
			return true
		}
	}
	return false
}

var maEvalNumberRE = regexp.MustCompile(`(?i)(?:€\s*)?[-+]?\d+(?:[\.\s]\d{3})*(?:[,.]\d+)?\s*(?:%|x|×|k|m|mln|milioni|mila)?`)

type maEvalNumberMention struct {
	Raw     string
	Value   float64
	HasUnit bool
}

func suspiciousNumberMentions(text string, allowed []float64) []string {
	mentions := extractEvalNumbers(text)
	seen := map[string]bool{}
	var out []string
	for _, m := range mentions {
		if shouldIgnoreEvalNumber(m) {
			continue
		}
		if closeToAnyAllowedNumber(m.Value, allowed) {
			continue
		}
		key := strings.TrimSpace(m.Raw)
		if !seen[key] {
			seen[key] = true
			out = append(out, key)
		}
		if len(out) >= 8 {
			break
		}
	}
	return out
}

func extractEvalNumbers(text string) []maEvalNumberMention {
	matches := maEvalNumberRE.FindAllString(text, -1)
	out := make([]maEvalNumberMention, 0, len(matches))
	for _, raw := range matches {
		m, ok := parseEvalNumberMention(raw)
		if ok {
			out = append(out, m)
		}
	}
	return out
}

func parseEvalNumberMention(raw string) (maEvalNumberMention, bool) {
	orig := strings.TrimSpace(raw)
	lower := strings.ToLower(orig)
	hasUnit := strings.Contains(lower, "€") || strings.Contains(lower, "%") || strings.Contains(lower, "x") || strings.Contains(lower, "×") || strings.Contains(lower, "mln") || strings.Contains(lower, "milion") || strings.Contains(lower, "mila") || strings.HasSuffix(strings.TrimSpace(lower), "m") || strings.HasSuffix(strings.TrimSpace(lower), "k")
	mult := 1.0
	trimmed := strings.TrimSpace(lower)
	for _, suffix := range []string{"milioni", "mln", "mila", "k", "m", "%", "x", "×"} {
		if strings.HasSuffix(trimmed, suffix) {
			switch suffix {
			case "milioni", "mln", "m":
				mult = 1_000_000
			case "mila", "k":
				mult = 1_000
			}
			trimmed = strings.TrimSpace(strings.TrimSuffix(trimmed, suffix))
			break
		}
	}
	trimmed = strings.ReplaceAll(trimmed, "€", "")
	trimmed = strings.ReplaceAll(trimmed, " ", "")
	if trimmed == "" || trimmed == "+" || trimmed == "-" {
		return maEvalNumberMention{}, false
	}
	if strings.Contains(trimmed, ",") {
		trimmed = strings.ReplaceAll(trimmed, ".", "")
		trimmed = strings.ReplaceAll(trimmed, ",", ".")
	} else if strings.Count(trimmed, ".") > 1 {
		trimmed = strings.ReplaceAll(trimmed, ".", "")
	} else if idx := strings.Index(trimmed, "."); idx >= 0 {
		fracLen := len(trimmed) - idx - 1
		intLen := idx
		if fracLen == 3 && intLen <= 3 {
			trimmed = strings.ReplaceAll(trimmed, ".", "")
		}
	}
	v, err := strconv.ParseFloat(trimmed, 64)
	if err != nil {
		return maEvalNumberMention{}, false
	}
	return maEvalNumberMention{Raw: orig, Value: v * mult, HasUnit: hasUnit}, true
}

func shouldIgnoreEvalNumber(m maEvalNumberMention) bool {
	abs := math.Abs(m.Value)
	if !m.HasUnit {
		if abs >= 1900 && abs <= 2100 && math.Abs(abs-math.Round(abs)) < 0.0001 {
			return true
		}
		if abs <= 5 {
			return true
		}
	}
	return false
}

func collectAllowedNumbersFromJSON(raw []byte) []float64 {
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil
	}
	seen := map[string]bool{}
	var out []float64
	var walk func(any)
	add := func(v float64) {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return
		}
		variants := []float64{v}
		if math.Abs(v) <= 2 && v != 0 {
			variants = append(variants, v*100)
		}
		for _, vv := range variants {
			key := strconv.FormatFloat(vv, 'f', 4, 64)
			if !seen[key] {
				seen[key] = true
				out = append(out, vv)
			}
		}
	}
	walk = func(v any) {
		switch t := v.(type) {
		case map[string]any:
			for _, child := range t {
				walk(child)
			}
		case []any:
			for _, child := range t {
				walk(child)
			}
		case float64:
			add(t)
		case json.Number:
			if f, err := t.Float64(); err == nil {
				add(f)
			}
		}
	}
	walk(value)
	return out
}

func closeToAnyAllowedNumber(value float64, allowed []float64) bool {
	for _, a := range allowed {
		if closeEvalNumber(value, a) {
			return true
		}
	}
	return false
}

func closeEvalNumber(value, allowed float64) bool {
	if value == allowed {
		return true
	}
	diff := math.Abs(value - allowed)
	absAllowed := math.Abs(allowed)
	tol := math.Max(1, absAllowed*0.03)
	if absAllowed >= 100_000 {
		tol = math.Max(tol, 5_000)
	}
	if absAllowed >= 1_000_000 {
		tol = math.Max(tol, 50_000)
	}
	return diff <= tol
}

func summarizeMADeepBriefEval(cases []maDeepBriefEvalCase, runs []maDeepBriefEvalRunRecord) []maDeepBriefEvalSummary {
	byCase := map[int][]maDeepBriefEvalRunRecord{}
	for _, run := range runs {
		byCase[run.CaseIndex] = append(byCase[run.CaseIndex], run)
	}
	out := make([]maDeepBriefEvalSummary, 0, len(cases))
	for _, c := range cases {
		items := byCase[c.Index]
		if len(items) == 0 {
			continue
		}
		s := maDeepBriefEvalSummary{CaseIndex: c.Index, CompanyKey: c.CompanyKey, CompanyName: c.CompanyName, Runs: len(items), MinScore: 101}
		var sum, sumSq, latency float64
		var jsonFail, ragFlip, hallucination int
		for _, item := range items {
			score := float64(item.Score.Total)
			sum += score
			sumSq += score * score
			if item.Score.Total < s.MinScore {
				s.MinScore = item.Score.Total
			}
			if item.Score.Total > s.MaxScore {
				s.MaxScore = item.Score.Total
			}
			if !item.Score.JSONValid {
				jsonFail++
			}
			if item.Score.RAGFlip {
				ragFlip++
			}
			if item.Score.HallucinationCandidate {
				hallucination++
			}
			latency += float64(item.LatencyMS)
			s.TotalTokens += item.Usage.TotalTokens
		}
		n := float64(len(items))
		s.AvgScore = sum / n
		variance := sumSq/n - s.AvgScore*s.AvgScore
		if variance < 0 {
			variance = 0
		}
		s.StdDevScore = math.Sqrt(variance)
		s.JSONFailRate = float64(jsonFail) / n
		s.RAGFlipRate = float64(ragFlip) / n
		s.HallucinationRate = float64(hallucination) / n
		s.AvgLatencyMS = latency / n
		if s.MinScore == 101 {
			s.MinScore = 0
		}
		out = append(out, s)
	}
	return out
}

func writeMADeepBriefEvalSummaryCSV(path string, summaries []maDeepBriefEvalSummary) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create summary csv: %w", err)
	}
	defer f.Close()
	w := csv.NewWriter(f)
	defer w.Flush()
	if err := w.Write([]string{"case_index", "company_key", "company_name", "runs", "avg_score", "stddev_score", "min_score", "max_score", "json_fail_rate", "rag_flip_rate", "hallucination_rate", "avg_latency_ms", "total_tokens"}); err != nil {
		return err
	}
	for _, s := range summaries {
		row := []string{
			strconv.Itoa(s.CaseIndex), s.CompanyKey, s.CompanyName, strconv.Itoa(s.Runs),
			fmt.Sprintf("%.2f", s.AvgScore), fmt.Sprintf("%.2f", s.StdDevScore), strconv.Itoa(s.MinScore), strconv.Itoa(s.MaxScore),
			fmt.Sprintf("%.2f", s.JSONFailRate), fmt.Sprintf("%.2f", s.RAGFlipRate), fmt.Sprintf("%.2f", s.HallucinationRate),
			fmt.Sprintf("%.0f", s.AvgLatencyMS), strconv.Itoa(s.TotalTokens),
		}
		if err := w.Write(row); err != nil {
			return err
		}
	}
	return w.Error()
}

func writeMADeepBriefEvalDashboard(path string, manifest maDeepBriefEvalManifest, summaries []maDeepBriefEvalSummary, runs []maDeepBriefEvalRunRecord) error {
	global := aggregateMADeepBriefEvalSummary(summaries)
	byCase := map[int][]maDeepBriefEvalRunRecord{}
	for _, run := range runs {
		byCase[run.CaseIndex] = append(byCase[run.CaseIndex], run)
	}
	var b strings.Builder
	b.WriteString(`<!doctype html><html lang="it"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>Binocolo LLM Eval</title><style>`)
	b.WriteString(`:root{font-family:"DM Sans",-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif;color:#0f172a;background:radial-gradient(circle at top left,rgba(99,91,255,.06),transparent 28%),linear-gradient(180deg,#f8fafc 0%,#eef2ff 100%)}body{margin:0;padding:32px}.page{max-width:1280px;margin:0 auto}.hero,.card{background:#fff;border:1px solid #e2e8f0;border-radius:20px;box-shadow:0 10px 15px -3px rgba(15,23,42,.08),0 4px 6px -4px rgba(15,23,42,.04)}.hero{padding:28px;margin-bottom:20px}.eyebrow{font-size:12px;text-transform:uppercase;letter-spacing:.08em;color:#635bff;font-weight:700}.title{font-size:28px;line-height:1.1;margin:8px 0 6px;font-weight:800;letter-spacing:-.04em}.muted{color:#64748b;font-size:13px}.grid{display:grid;grid-template-columns:repeat(5,minmax(0,1fr));gap:12px;margin:18px 0}.kpi{background:#f8fafc;border:1px solid #e2e8f0;border-radius:16px;padding:14px}.kpi strong{display:block;font-size:24px}.card{padding:18px;margin:16px 0}table{width:100%;border-collapse:collapse;font-size:13px}th{font-size:11px;text-transform:uppercase;letter-spacing:.06em;color:#64748b;text-align:left;border-bottom:1px solid #e2e8f0;padding:10px}td{border-bottom:1px solid #f1f5f9;padding:10px;vertical-align:top}.score{font-weight:800}.good{color:#059669}.warn{color:#b45309}.bad{color:#dc2626}.pill{display:inline-flex;border-radius:999px;padding:3px 8px;font-weight:700;font-size:12px;background:#eef2ff;color:#5046e5}.mono{font-family:"JetBrains Mono","SF Mono",monospace;font-size:12px}details{margin:6px 0}summary{cursor:pointer;color:#635bff;font-weight:700}pre{white-space:pre-wrap;background:#0f172a;color:#e2e8f0;border-radius:12px;padding:12px;max-height:380px;overflow:auto}.warnings{color:#b45309;font-size:12px} @media(max-width:900px){.grid{grid-template-columns:1fr 1fr}body{padding:16px}}`)
	b.WriteString(`</style></head><body><main class="page">`)
	b.WriteString(`<section class="hero"><div class="eyebrow">Binocolo · ma_deep_brief variance eval</div>`)
	b.WriteString(`<h1 class="title">Dashboard valutazione LLM</h1>`)
	b.WriteString(`<div class="muted">Esperimento <span class="mono">` + html.EscapeString(manifest.ExperimentID) + `</span> · Modello <span class="mono">` + html.EscapeString(manifest.Model.Model) + `</span> · Prompt <span class="mono">` + html.EscapeString(manifest.Prompt.PromptSHA) + `</span></div>`)
	b.WriteString(`<div class="grid">`)
	writeKPI(&b, "Score medio", fmt.Sprintf("%.1f", global.AvgScore), "")
	writeKPI(&b, "Dev. std media", fmt.Sprintf("%.1f", global.StdDevScore), "")
	writeKPI(&b, "JSON fail", fmt.Sprintf("%.0f%%", global.JSONFailRate*100), "")
	writeKPI(&b, "RAG flip", fmt.Sprintf("%.0f%%", global.RAGFlipRate*100), "")
	writeKPI(&b, "Hallucination cand.", fmt.Sprintf("%.0f%%", global.HallucinationRate*100), "")
	b.WriteString(`</div><p class="muted">Score euristico: serve a confrontare stabilità e fedeltà tra modelli, non sostituisce review FDD.</p></section>`)

	b.WriteString(`<section class="card"><h2>Riepilogo per azienda</h2><table><thead><tr><th>Azienda</th><th>Run</th><th>Avg</th><th>Std</th><th>Min/Max</th><th>JSON fail</th><th>RAG flip</th><th>Numeri sospetti</th><th>Token</th></tr></thead><tbody>`)
	for _, s := range summaries {
		b.WriteString(`<tr><td><strong>` + html.EscapeString(labelCompany(s.CompanyName, s.CompanyKey)) + `</strong><br><span class="mono">` + html.EscapeString(s.CompanyKey) + `</span></td>`)
		b.WriteString(`<td>` + strconv.Itoa(s.Runs) + `</td>`)
		b.WriteString(`<td class="score ` + scoreClass(s.AvgScore) + `">` + fmt.Sprintf("%.1f", s.AvgScore) + `</td>`)
		b.WriteString(`<td>` + fmt.Sprintf("%.1f", s.StdDevScore) + `</td>`)
		b.WriteString(`<td>` + strconv.Itoa(s.MinScore) + ` / ` + strconv.Itoa(s.MaxScore) + `</td>`)
		b.WriteString(`<td>` + fmt.Sprintf("%.0f%%", s.JSONFailRate*100) + `</td>`)
		b.WriteString(`<td>` + fmt.Sprintf("%.0f%%", s.RAGFlipRate*100) + `</td>`)
		b.WriteString(`<td>` + fmt.Sprintf("%.0f%%", s.HallucinationRate*100) + `</td>`)
		b.WriteString(`<td>` + strconv.Itoa(s.TotalTokens) + `</td></tr>`)
	}
	b.WriteString(`</tbody></table></section>`)

	b.WriteString(`<section class="card"><h2>Dettaglio iterazioni</h2>`)
	for _, s := range summaries {
		items := byCase[s.CaseIndex]
		sort.Slice(items, func(i, j int) bool { return items[i].Iteration < items[j].Iteration })
		b.WriteString(`<h3>` + html.EscapeString(labelCompany(s.CompanyName, s.CompanyKey)) + ` <span class="mono">` + html.EscapeString(s.CompanyKey) + `</span></h3>`)
		b.WriteString(`<table><thead><tr><th>Iter</th><th>Status</th><th>Score</th><th>RAG</th><th>Warn</th><th>Latenza</th><th>Output</th></tr></thead><tbody>`)
		for _, run := range items {
			rag := ""
			if run.ParsedBrief != nil {
				rag = run.ParsedBrief.RAG
			}
			b.WriteString(`<tr><td>` + strconv.Itoa(run.Iteration) + `</td><td><span class="pill">` + html.EscapeString(run.Status) + `</span></td>`)
			b.WriteString(`<td class="score ` + scoreClass(float64(run.Score.Total)) + `">` + strconv.Itoa(run.Score.Total) + `</td><td>` + html.EscapeString(rag) + `</td>`)
			b.WriteString(`<td class="warnings">` + html.EscapeString(strings.Join(run.Score.Warnings, " · ")) + `</td><td>` + strconv.Itoa(run.LatencyMS) + ` ms</td><td>`)
			b.WriteString(`<details><summary>vedi</summary><pre>` + html.EscapeString(run.RawResponse) + `</pre></details>`)
			b.WriteString(`</td></tr>`)
		}
		b.WriteString(`</tbody></table>`)
	}
	b.WriteString(`</section></main></body></html>`)
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

func writeKPI(b *strings.Builder, label, value, extra string) {
	b.WriteString(`<div class="kpi"><span class="muted">` + html.EscapeString(label) + `</span><strong>` + html.EscapeString(value) + `</strong>`)
	if extra != "" {
		b.WriteString(`<span class="muted">` + html.EscapeString(extra) + `</span>`)
	}
	b.WriteString(`</div>`)
}

func aggregateMADeepBriefEvalSummary(items []maDeepBriefEvalSummary) maDeepBriefEvalSummary {
	if len(items) == 0 {
		return maDeepBriefEvalSummary{}
	}
	var out maDeepBriefEvalSummary
	for _, item := range items {
		out.AvgScore += item.AvgScore
		out.StdDevScore += item.StdDevScore
		out.JSONFailRate += item.JSONFailRate
		out.RAGFlipRate += item.RAGFlipRate
		out.HallucinationRate += item.HallucinationRate
	}
	n := float64(len(items))
	out.AvgScore /= n
	out.StdDevScore /= n
	out.JSONFailRate /= n
	out.RAGFlipRate /= n
	out.HallucinationRate /= n
	return out
}

func writeMADeepBriefEvalNotes(path string) error {
	content := `# Binocolo ma_deep_brief LLM variance eval

Artefatti generati localmente per testare la varianza del modello sul brief neutro.

- ` + "`case_manifest.json`" + `: campione deterministico dal DB e snapshot di modello/prompt.
- ` + "`inputs/*.json`" + `: input esatto inviato al modello per ogni azienda.
- ` + "`outputs.jsonl`" + `: una riga per iterazione, con raw output, parse, usage e score.
- ` + "`summary.csv`" + `: riepilogo per azienda.
- ` + "`dashboard.html`" + `: lettura navigabile dei risultati.

Rubrica: JSON/schema 10, coerenza RAG 10, coerenza valuation 25, assenza numeri inventati 20, copertura caveat 15, domande DD 10, chiarezza 10.

Nota: lo scoring è euristico e serve per confronti modello-vs-modello; non sostituisce il giudizio dell'analista FDD.
`
	return os.WriteFile(path, []byte(content), 0o644)
}

func writeJSONFile(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal %s: %w", path, err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func stripEvalCasePayloads(cases []maDeepBriefEvalCase) []maDeepBriefEvalCase {
	out := make([]maDeepBriefEvalCase, len(cases))
	for i, c := range cases {
		c.payload = nil
		c.scorecard = nil
		c.valuation = nil
		c.inputRaw = nil
		out[i] = c
	}
	return out
}

func prettyJSON(raw json.RawMessage) []byte {
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return append([]byte(raw), '\n')
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return append([]byte(raw), '\n')
	}
	return append(data, '\n')
}

func defaultMADeepBriefEvalOutputDir(experimentID, model string) string {
	base := filepath.Join("artifacts", "llm-evals", "ma-deep-brief")
	if cwd, err := os.Getwd(); err == nil && filepath.Base(cwd) == "backend" {
		if _, statErr := os.Stat(filepath.Join("..", "artifacts")); statErr == nil {
			base = filepath.Join("..", "artifacts", "llm-evals", "ma-deep-brief")
		}
	}
	return filepath.Join(base, experimentID+"-"+safeArtifactName(model))
}

func safeArtifactName(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	lastDash := false
	for _, r := range value {
		ok := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		if ok {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteRune('-')
			lastDash = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		out = "item"
	}
	if len(out) > 80 {
		out = out[:80]
		out = strings.Trim(out, "-")
	}
	return out
}

func shortSHA(value string, n int) string {
	sum := sha256.Sum256([]byte(value))
	hexValue := fmt.Sprintf("%x", sum[:])
	if n > 0 && n < len(hexValue) {
		return hexValue[:n]
	}
	return hexValue
}

func labelCompany(name, key string) string {
	if strings.TrimSpace(name) != "" {
		return name
	}
	return key
}

func scoreClass(score float64) string {
	switch {
	case score >= 85:
		return "good"
	case score >= 70:
		return "warn"
	default:
		return "bad"
	}
}
