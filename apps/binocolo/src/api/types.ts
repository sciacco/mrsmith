export interface OpenAPIITEnvelope<T> {
  data: T;
  success: boolean;
  message: string;
  error: number | null;
  count?: number;
  cost?: number;
}

export interface Province {
  sigla: string;
  provincia: string;
  superficie: number;
  residenti: number;
  num_comuni: number;
  istat: string;
  regione: string;
}

export interface CompanySearchRow {
  id: string;
  taxCode?: string | null;
  companyName?: string | null;
  vatCode?: string | null;
  activityStatus?: string | null;
  address?: {
    registeredOffice?: {
      town?: string | null;
      province?: string | null;
      zipCode?: string | null;
    };
  };
}

export interface WebSearchRequest {
  domain: string;
  keywords: string[];
  count?: number;
  rank?: boolean;
}

export interface WebSearchResult {
  title: string;
  url: string;
  hostname: string;
  age?: string;
  snippets: string[];
  score?: number;
}

export interface WebSearchResponse {
  query: string;
  count: number;
  ranked: boolean;
  rankError?: string;
  results: WebSearchResult[];
}

export interface DomainResolutionRequest {
  companyName: string;
  vatCode?: string;
  taxCode?: string;
  town?: string;
  province?: string;
  keywords?: string[];
  count?: number;
}

export interface DomainResolutionCandidate {
  domain: string;
  score: number;
  confidence: 'alta' | 'media' | 'bassa' | string;
  reasons: string[];
  results: WebSearchResult[];
}

export interface DomainResolutionResponse {
  query: string;
  count: number;
  candidates: DomainResolutionCandidate[];
  results: WebSearchResult[];
}

export interface CandidateMatchAnalysisRequest {
  target: MATarget;
  keywordSet: {
    intentLabel: string;
    coreTerms: string[];
    adjacentTerms: string[];
    negativeTerms: string[];
    sources: string[];
  };
  domainResponse: DomainResolutionResponse;
  selectedDomain?: DomainResolutionCandidate;
  evidenceRuns: Array<{
    bucket: string;
    term: string;
    response?: WebSearchResponse;
    error?: string;
    resultCount: number;
    bestScore?: number;
    matched: boolean;
  }>;
  summary: {
    score: number;
    confidence: string;
    sectorEvidenceScore: number;
    coverageScore: number;
    domainScore: number;
    negativePenalty: number;
    coreMatches: number;
    adjacentMatches: number;
    negativeMatches: number;
    searchedCoreTerms: number;
    searchedAdjacentTerms: number;
    searchedNegativeTerms: number;
    totalCoreTerms: number;
    totalAdjacentTerms: number;
    totalNegativeTerms: number;
  };
}

export type PipelineWebValidationState =
  | 'confirmed'
  | 'deprioritized'
  | 'domain_unresolved'
  | 'analysis_unavailable'
  | 'rejected'
  | 'unclear';

export type PipelineFinalAction =
  | 'confirm'
  | 'deprioritize'
  | 'reject'
  | 'needs_domain_review'
  | 'needs_business_validation';

export interface CandidateMatchFinalDecision {
  initialMatchState: string;
  deterministicScore: number;
  webScore: number;
  webValidationState: PipelineWebValidationState;
  finalAction: PipelineFinalAction;
  confidence: MAConfidence | string;
  reason: string;
  reasons: string[];
  analystVerdict?: string;
  analystAction?: string;
}

export interface MAWebValidation {
  sessionId: string;
  companyKey: string;
  targetId?: string;
  runId?: string;
  pipelineVersion: string;
  inputHash?: string;
  keywordSetHash?: string;
  llmModelId?: string;
  llmPromptId?: string;
  llmModel?: string;
  freshness: 'fresh' | 'stale' | 'expired' | string;
  staleAfter: string;
  expiresAt: string;
  selectedDomain?: string;
  domainConfidence?: string;
  domainScore?: number;
  webScore: number;
  webConfidence?: string;
  webValidationState: PipelineWebValidationState;
  finalAction: PipelineFinalAction;
  analystVerdict?: string;
  analystAction?: string;
  analystConfidence?: string;
  summary: CandidateMatchAnalysisRequest['summary'];
  keywordSet: CandidateMatchAnalysisRequest['keywordSet'];
  selectedDomainPayload?: DomainResolutionCandidate;
  domainResponse: DomainResolutionResponse;
  evidenceRuns: CandidateMatchAnalysisRequest['evidenceRuns'];
  candidateMatchAnalysis?: CandidateMatchAnalysisResponse;
  candidateMatchError?: string;
  finalDecision: CandidateMatchFinalDecision;
  updatedByEmail?: string;
  updatedAt: string;
}

export interface MAWebValidationEnrichRequest {
  limit?: number;
  force?: boolean;
  includeIdentifiers?: boolean;
  analyzeWithLLM?: boolean;
  /** Force the LLM analyst on EVERY company (not just ambiguous). Used by the sector-eval run. */
  llmOnAll?: boolean;
  domainCount?: number;
  keywordCount?: number;
  rank?: boolean;
}

export interface CandidateMatchAnalysisResponse {
  verdict: 'strong_match' | 'match' | 'weak_match' | 'no_match' | 'unclear' | string;
  confidence: MAConfidence | string;
  sectorFit: string;
  businessFit: string;
  evidenceFor: string[];
  evidenceAgainst: string[];
  negativeSignals: string[];
  missingEvidence: string[];
  conceptAliases: CandidateMatchConceptAlias[];
  recommendedAction: 'confirm' | 'review' | 'downgrade' | 'reject' | string;
  rationale: string;
  finalDecision?: CandidateMatchFinalDecision;
  modelId?: string;
  promptId?: string;
  model?: string;
}

export interface CandidateMatchConceptAlias {
  term: string;
  matchedConcept: string;
  evidence: string;
}

// UC2 sector classification lab probe (POST /binocolo/v1/test/sector-classification).
export interface SectorConceptScore {
  conceptId: string;
  name: string;
  kind: 'target' | 'distractor' | string;
  cosine: number;
  rerankProb: number;
  inStrategy: boolean;
}

export interface SectorClassification {
  companyDescription: string;
  concepts: SectorConceptScore[];
  strategyConcepts: string[];
  verdict: 'confirm' | 'reject' | 'weak' | 'ambiguous' | 'no_signal' | string;
  topProb: number;
  rerankApplied: boolean;
  confidence: MAConfidence | string;
  reason: string;
}

export interface SectorClassificationTestResponse {
  companyName?: string;
  selectedDomain?: string;
  evidence: string[];
  classification: SectorClassification;
  analysis?: CandidateMatchAnalysisResponse;
  finalDecision?: CandidateMatchFinalDecision;
}

export interface MASessionListResponse {
  items: MASessionSummary[];
}

export interface MAParameter {
  key: string;
  value: string;
  valueType: string;
  label: string;
  description?: string;
  updatedByEmail?: string;
  updatedAt?: string;
}

export interface MAParametersResponse {
  items: MAParameter[];
}

export interface MALLMOptionsResponse {
  models: MALLMModelOption[];
  prompts: MALLMPromptOption[];
}

export interface MALLMModelOption {
  id: string;
  scope: string;
  name: string;
  model: string;
  isDefault: boolean;
}

export interface MALLMPromptOption {
  id: string;
  scope: string;
  name: string;
  isDefault: boolean;
}

export interface MASessionSummary {
  id: string;
  title: string;
  prompt: string;
  status: MASessionStatus;
  selectedStrategy?: MAStrategyType;
  estimatedCount: number;
  estimatedCost: number;
  resultCount: number;
  createdAt: string;
  updatedAt: string;
  lastRunAt?: string;
  archivedAt?: string;
  archivedByEmail?: string;
  deletedAt?: string;
  deletedByEmail?: string;
}

export interface MASessionDetail {
  session: MASession;
  strategy?: MAStrategyVersion;
  estimates: MAEstimate[];
  runs: MAExecutionRun[];
  targets: MATarget[];
  budgetEur: number;
  costPerCompanyEur: number;
  costFullEur: number;
}

export interface MADeepAnalysis {
  companyKey: string;
  status: 'queued' | 'running' | 'ready' | 'failed';
  scorecard?: MADeepScorecard;
  valuation?: MADeepValuation;
  brief?: MADeepBrief;
  costEur?: number;
  errorCode?: string;
  updatedAt?: string;
}

export interface MADeepScorecard {
  metrics: MADeepMetric[];
  overallRag: string;
  turnover?: number;
  turnoverYear?: number;
  ebitda?: number;
  netWorth?: number;
  pfn?: number;
  atecoCode?: string;
}

export interface MADeepMetric {
  group: string;
  key: string;
  label: string;
  value?: number;
  unit: string;
  rag: string;
}

export interface MADeepValuation {
  method: string;
  multiple: number;
  haircutPct: number;
  evLow: number;
  evHigh: number;
  equityLow?: number;
  equityHigh?: number;
  pfn?: number;
  sector?: string;
  nFirms?: number;
  source?: string;
  sourceDate?: string;
  caveat?: string;
}

export interface MADeepBrief {
  verdict?: string;
  rag?: string;
  businessProfile?: string;
  thesisReading?: string;
  strengths?: string[];
  redFlags?: { severity: string; category?: string; claim: string; ddQuestion?: string }[];
  valuationRationale?: string;
  ddQuestions?: string[];
  thesisFit?: string;
}

// Standalone P.IVA dossier: our elaborations plus the raw IT-full payload (facts layer).
export interface MACompanyDossier {
  vatCode: string;
  status: 'absent' | 'cost_required' | 'queued' | 'running' | 'ready' | 'failed';
  scorecard?: MADeepScorecard;
  valuation?: MADeepValuation;
  brief?: MADeepBrief;
  raw?: unknown;
  costEur?: number;
  errorCode?: string;
  updatedAt?: string;
}

export type MASessionStatus = 'draft' | 'estimating' | 'estimated' | 'running' | 'completed' | 'failed';
export type MASessionVisibility = 'active' | 'archived' | 'deleted';
export type MAStrategyType = 'ateco' | 'expanded';
export type MAMatchState = 'match' | 'match_parziale' | 'fuori_criterio';
export type MAEstimateSurfaceStatus = 'exact' | 'too_broad';
export type MAThesis = 'successione' | 'crescita' | 'consolidamento' | 'tuck_in' | 'generico';
export type MAConfidence = 'alta' | 'media' | 'bassa';
export type MAFlagSeverity = 'neutral' | 'warning';

export interface MASession {
  id: string;
  title: string;
  prompt: string;
  status: MASessionStatus;
  selectedStrategy?: MAStrategyType;
  activeStrategyId?: string;
  lastEstimatedAt?: string;
  lastExecutedAt?: string;
  createdAt: string;
  updatedAt: string;
  archivedAt?: string;
  archivedBySubject?: string;
  archivedByEmail?: string;
  deletedAt?: string;
  deletedBySubject?: string;
  deletedByEmail?: string;
}

export interface MAStrategyVersion {
  id: string;
  sessionId: string;
  version: number;
  strategy: MAStrategySpec;
  createdByEmail?: string;
  createdAt: string;
}

export interface MAStrategySpec {
  title?: string;
  sectorDescription: string;
  territoryLabel?: string;
  provinces: string[];
  activityStatus: string;
  turnoverAround?: number;
  turnoverMin?: number;
  turnoverMax?: number;
  employeeMin?: number;
  employeeMax?: number;
  revenuePerEmployeeMin?: number;
  maxShareholders?: number;
  searchLimit: number;
  atecoCandidates: MAAtecoCandidate[];
  keywords: string[];
  scoringCriteria?: MAScoringCriterion[];
  rationale: string;
  missingCriteria: string[];
  selectedStrategy?: MAStrategyType;
  expandedClassification?: string;
  thesis?: MAThesis;
  legalForms?: string[];
  signalWeights?: Record<string, number>;
  maxBudgetEur?: number;
  successionMinOwnerAge?: number;
}

export type MAAtecoFit = 'core' | 'weak' | 'excluded';

export interface MAAtecoSearchItem {
  code: string;
  description: string;
  hierarchy?: number;
}

export interface MAAtecoSearchResponse {
  items: MAAtecoSearchItem[];
}

export interface MAAtecoCandidate {
  code: string;
  description: string;
  rationale: string;
  // Curated relevance tier (longest-prefix-wins). Empty defaults to core.
  fit?: MAAtecoFit;
}

export interface MAScoringCriterion {
  id: string;
  label: string;
  description?: string;
  weight: number;
  evaluation: MAScoringEvaluation;
  source?: string;
}

export interface MAScoringEvaluation {
  sourcePath?: string;
  operator?: string;
  value?: string | number | boolean;
  min?: number;
  max?: number;
  tolerance?: number;
  match?: 'any' | 'all';
}

export interface MAEstimate {
  id: string;
  sessionId: string;
  strategyVersionId: string;
  strategyType: MAStrategyType;
  atecoCode?: string;
  atecoDescription?: string;
  province?: string;
  estimatedCount: number;
  estimatedCost: number;
  selected: boolean;
  surfaceStatus?: MAEstimateSurfaceStatus;
  executionLimit?: number;
  probeCount?: number;
  params?: Record<string, string>;
  createdAt: string;
}

export interface MAExecutionRun {
  id: string;
  sessionId: string;
  strategyVersionId: string;
  strategyType: MAStrategyType;
  status: 'running' | 'completed' | 'failed';
  estimatedCount: number;
  resultCount: number;
  errorCode?: string;
  startedAt: string;
  completedAt?: string;
}

export interface MATarget {
  id: string;
  sessionId: string;
  runId: string;
  vendorId?: string;
  companyKey?: string;
  companyName: string;
  vatCode?: string;
  taxCode?: string;
  province?: string;
  town?: string;
  activityStatus?: string;
  turnover?: number;
  turnoverYear?: number;
  employees?: number;
  atecoCode?: string;
  atecoDescription?: string;
  score: number;
  matchState: MAMatchState;
  confidence?: MAConfidence;
  rating?: number;
  flags?: MATargetFlag[];
  rationale: string;
  missingCriteria: string[];
  evidence: MATargetEvidence[];
  adjustments?: MATargetAdjustment[];
  deep?: MADeepAnalysis;
  webValidation?: MAWebValidation;
  vendorPayload?: any;
}

export interface MATargetFlag {
  code: string;
  label: string;
  severity: MAFlagSeverity;
}

export interface MATargetEvidence {
  criterion: string;
  status: 'match' | 'match_parziale' | 'criterio_mancante' | 'fuori_criterio';
  family?: string;
  label: string;
  value?: string;
  points?: number;
  weight?: number;
  sourcePath?: string;
}

// Multiplicative score factor applied outside the additive evidence blend
// (viability, thesis-fit). Surfaced only when factor < 1 so the score reconstructs.
export interface MATargetAdjustment {
  code: string;
  label: string;
  factor: number;
}

// --- UC2 sector-eval harness ---

export type SectorEvalLabel = 'keep' | 'forse' | 'scarta';

export interface SectorEvalConcept {
  conceptId: string;
  name: string;
  kind: string;
  cosine: number;
  rerankProb: number;
  inStrategy: boolean;
}

export interface SectorEvalItem {
  companyKey: string;
  companyName: string;
  domain?: string;
  atecoDescription?: string;
  selfDescription?: string;
  validated: boolean;
  // A — embed+rerank only (no LLM)
  deterministicVerdict?: string;
  deterministicBucket?: SectorEvalLabel;
  // B — LLM analyst on this company (LLM-on-all); absent if it did not run
  llmVerdict?: string;
  llmAction?: string;
  llmBucket?: SectorEvalLabel;
  // C — production hybrid final decision
  finalState?: string;
  finalAction?: string;
  finalBucket?: SectorEvalLabel;
  escalated: boolean;
  distractorBeatsTarget: boolean;
  topConcepts?: SectorEvalConcept[];
  label?: SectorEvalLabel;
  note?: string;
}

export interface SectorEvalPredictorMetrics {
  evaluable: number;
  correct: number;
  accuracy: number;
  confusion: Record<string, Record<string, number>>;
}

export interface SectorEvalMetrics {
  targets: number;
  validated: number;
  labeled: number;
  deterministic: SectorEvalPredictorMetrics;
  llm: SectorEvalPredictorMetrics;
  final: SectorEvalPredictorMetrics;
  escalations: number;
  escalationRate: number;
  distractorBeatsTarget: number;
  llmVsDeterministic: Record<string, number>;
}

export interface SectorEvalReport {
  sessionId: string;
  items: SectorEvalItem[];
  metrics: SectorEvalMetrics;
}

export interface SectorEvalLabelRequest {
  companyKey: string;
  label: SectorEvalLabel | '';
  note?: string;
}
