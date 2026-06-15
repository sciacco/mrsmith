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

export interface MASessionListResponse {
  items: MASessionSummary[];
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
}

export interface MASessionDetail {
  session: MASession;
  strategy?: MAStrategyVersion;
  estimates: MAEstimate[];
  runs: MAExecutionRun[];
  targets: MATarget[];
}

export type MASessionStatus = 'draft' | 'estimated' | 'running' | 'completed' | 'failed';
export type MAStrategyType = 'ateco' | 'expanded';
export type MAMatchState = 'match' | 'match_parziale' | 'fuori_criterio';

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
  atecoCandidates: MAAtecoCandidate[];
  keywords: string[];
  shareholder: MAShareholderSignal;
  shareholderAge: MAAgeSignal;
  rationale: string;
  missingCriteria: string[];
  selectedStrategy?: MAStrategyType;
  expandedClassification?: string;
}

export interface MAAtecoCandidate {
  code: string;
  description: string;
  rationale: string;
}

export interface MAShareholderSignal {
  requiresEqualSplit: boolean;
  tolerance: number;
}

export interface MAAgeSignal {
  required: boolean;
  min?: number;
  max?: number;
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
  rationale: string;
  missingCriteria: string[];
  evidence: MATargetEvidence[];
}

export interface MATargetEvidence {
  criterion: string;
  status: 'match' | 'match_parziale' | 'criterio_mancante' | 'fuori_criterio';
  label: string;
  value?: string;
  sourcePath?: string;
}
