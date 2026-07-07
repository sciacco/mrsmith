import { useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { ApiError } from '@mrsmith/api-client';
import { useMutation, useQuery } from '@tanstack/react-query';
import { Button, Icon, Skeleton } from '@mrsmith/ui';
import { useApiClient } from '../../api/client';
import { DeepAnalysisContent } from '../../components/deep/DeepComponents';
import { ThesisReadingPanel } from '../../components/ThesisReadingPanel/ThesisReadingPanel';
import type {
  MACardDossier,
  MACardIRLItem,
  MACardThesisReading,
  MAIRLSeedReport,
  MAIRLStatus,
  MADeepAnalysis,
  MATarget,
  MATargetAdjustment,
  MATargetEvidence,
  MATargetFlag,
  PipelineFinalAction,
  PipelineWebValidationState,
} from '../../api/types';
import { CompanyRegistrySection } from './CompanyRegistrySection';
import styles from './IniziativaCardDossierPage.module.css';

const numberFormat = new Intl.NumberFormat('it-IT');
const moneyFormat = new Intl.NumberFormat('it-IT', {
  style: 'currency',
  currency: 'EUR',
  maximumFractionDigits: 0,
});

const CARD_STATE_LABELS: Record<string, string> = {
  da_contattare: 'Da contattare',
  contattata: 'Contattata',
  in_dialogo: 'In dialogo',
  approfondimento: 'Approfondimento',
  offerta: 'Offerta',
  chiusa: 'Chiusa',
  rimossa: 'Rimossa',
};

function errorLabel(error: unknown): string {
  if (error instanceof ApiError) {
    if (error.status === 404) return 'Nessun dettaglio disponibile per questa card.';
    if (error.status === 403) return 'Non hai accesso a Binocolo.';
    if (error.status === 503) return 'Servizio non configurato in questo ambiente.';
    return `Richiesta non riuscita (${error.status}).`;
  }
  return 'Richiesta non riuscita.';
}

type TabKey = 'overview' | 'deep' | 'tesi' | 'irl' | 'financials' | 'shareholders' | 'registry' | 'web';

const TABS: { key: TabKey; label: string; icon: string }[] = [
  { key: 'overview', label: 'Panoramica', icon: 'external-link' },
  { key: 'deep', label: 'Analisi', icon: 'bar-chart-2' },
  { key: 'tesi', label: 'Lettura di tesi', icon: 'target' },
  { key: 'irl', label: 'IRL', icon: 'clipboard-check' },
  { key: 'financials', label: 'Finanziari', icon: 'circle-dollar-sign' },
  { key: 'shareholders', label: 'Soci', icon: 'user' },
  { key: 'registry', label: 'Registro camerale', icon: 'file-text' },
  { key: 'web', label: 'Web', icon: 'network' },
];

export function IniziativaCardDossierPage() {
  const { id, companyKey } = useParams<{ id: string; companyKey: string }>();
  const navigate = useNavigate();
  const api = useApiClient();
  const [activeTab, setActiveTab] = useState<TabKey>('overview');

  const query = useQuery({
    queryKey: ['ma-card-dossier', id, companyKey],
    enabled: Boolean(id && companyKey),
    queryFn: () =>
      api.get<MACardDossier>(
        `/binocolo/v1/ma/initiatives/${id}/cards/${encodeURIComponent(companyKey ?? '')}/dossier`,
      ),
    // Il 404 (card assente o sessioni sganciate) è uno stato, non un guasto:
    // niente retry, l'empty state deve comparire subito.
    retry: (failureCount, error) =>
      !(error instanceof ApiError && error.status === 404) && failureCount < 2,
  });

  if (query.isLoading) {
    return (
      <main className={styles.page}>
        <Skeleton rows={8} />
      </main>
    );
  }

  if (query.isError) {
    return (
      <main className={styles.page}>
        <button type="button" className={styles.backLink} onClick={() => navigate(`/iniziative/${id}`)}>
          ← Torna al board
        </button>
        <div className={styles.statePanel} role="alert">
          <Icon name="triangle-alert" size={22} />
          <p>{errorLabel(query.error)}</p>
        </div>
      </main>
    );
  }

  const dossier = query.data;
  if (!dossier) return null;

  const target = dossier.target;
  const card = dossier.card;

  return (
    <main className={styles.page}>
      <button type="button" className={styles.backLink} onClick={() => navigate(`/iniziative/${id}`)}>
        ← Torna al board
      </button>

      <header className={styles.header}>
        <div className={styles.headTop}>
          <h1>{target.companyName}</h1>
          {card ? (
            <span className={styles.statePill}>{CARD_STATE_LABELS[card.state] ?? card.state}</span>
          ) : null}
        </div>
        <div className={styles.meta}>
          {target.vatCode ? <span className={styles.mono}>P.IVA {target.vatCode}</span> : null}
          {target.taxCode ? <span className={styles.mono}>CF {target.taxCode}</span> : null}
          {target.town ? <span>{[target.town, target.province].filter(Boolean).join(' · ')}</span> : null}
          {dossier.sessionTitle ? <span>Ricerca: {dossier.sessionTitle}</span> : null}
        </div>
      </header>

      <div className={styles.section}>
        <div className={styles.tabNav}>
          {TABS.map((tab) => (
            <button
              key={tab.key}
              type="button"
              className={`${styles.tabLink ?? ''} ${activeTab === tab.key ? styles.tabLinkActive ?? '' : ''}`}
              onClick={() => setActiveTab(tab.key)}
            >
              <Icon name={tab.icon as any} size={16} />
              <span>{tab.label}</span>
            </button>
          ))}
        </div>

        {activeTab === 'overview' && <OverviewTab target={target} />}
        {activeTab === 'deep' && (
          <DeepAnalysisTab
            deep={target.deep}
            companyKey={target.companyKey ?? companyKey ?? ''}
            onStarted={() => void query.refetch()}
          />
        )}
        {activeTab === 'tesi' && (
          <ThesisReadingTab
            initiativeId={id ?? ''}
            companyKey={target.companyKey ?? companyKey ?? ''}
            deepReady={target.deep?.status === 'ready'}
          />
        )}
        {activeTab === 'irl' && (
          <IRLTab initiativeId={id ?? ''} companyKey={target.companyKey ?? companyKey ?? ''} companyName={target.companyName} />
        )}
        {activeTab === 'financials' && <FinancialsTab target={target} />}
        {activeTab === 'shareholders' && <ShareholdersTab target={target} />}
        {activeTab === 'registry' && <RegistryTab target={target} />}
        {activeTab === 'web' && <WebTab target={target} />}
      </div>

      <CompanyRegistrySection companyKey={target.companyKey ?? companyKey ?? ''} vatCode={target.vatCode} companyName={target.companyName} />
    </main>
  );
}

// ThesisReadingTab — lettura di tesi context-scoped (Fase 5): il memo che applica
// la tesi della sessione di provenienza al dossier neutro. Generazione on-demand,
// rigenerazione esplicita quando la tesi cambia (staleThesis).
function ThesisReadingTab({
  initiativeId,
  companyKey,
  deepReady,
}: {
  initiativeId: string;
  companyKey: string;
  deepReady: boolean;
}) {
  const api = useApiClient();
  const query = useQuery({
    queryKey: ['ma-thesis-reading', initiativeId, companyKey],
    enabled: Boolean(initiativeId && companyKey),
    queryFn: () =>
      api.get<MACardThesisReading>(
        `/binocolo/v1/ma/initiatives/${initiativeId}/cards/${encodeURIComponent(companyKey)}/thesis-reading`,
      ),
    retry: (failureCount, error) =>
      !(error instanceof ApiError && error.status === 404) && failureCount < 2,
  });
  const generate = useMutation({
    mutationFn: () =>
      api.post<MACardThesisReading>(
        `/binocolo/v1/ma/initiatives/${initiativeId}/cards/${encodeURIComponent(companyKey)}/thesis-reading`,
        {},
      ),
    onSuccess: () => void query.refetch(),
  });

  if (query.isLoading) return <Skeleton rows={5} />;

  const record = query.data;
  const notGenerated = !record && query.isError && query.error instanceof ApiError && query.error.status === 404;

  return (
    <ThesisReadingPanel
      record={record}
      notGenerated={notGenerated}
      queryError={query.isError && !notGenerated ? errorLabel(query.error) : null}
      deepReady={deepReady}
      onGenerate={() => generate.mutate()}
      generating={generate.isPending}
      generationError={generate.isError ? 'Generazione non riuscita: riprova.' : null}
      regenerateRequiresDeepReady={false}
    />
  );
}

const IRL_STATUS_CYCLE: Record<MAIRLStatus, MAIRLStatus> = {
  aperta: 'chiesta',
  chiesta: 'risposta',
  risposta: 'na',
  na: 'aperta',
};

const IRL_SOURCE_LABELS: Record<string, string> = {
  flag: 'flag',
  brief: 'brief',
  thesis: 'tesi',
  template: 'template',
  analyst: 'analista',
};

// IRLTab — Information Request List della card (Fase 6): il kick-off DD che
// esce dal tool. Seminata dalle fonti (flag, brief, lettura di tesi, template
// di famiglia), poi curatela dell'analista; il re-seed è solo additivo.
function IRLTab({
  initiativeId,
  companyKey,
  companyName,
}: {
  initiativeId: string;
  companyKey: string;
  companyName: string;
}) {
  const api = useApiClient();
  const base = `/binocolo/v1/ma/initiatives/${initiativeId}/cards/${encodeURIComponent(companyKey)}/irl`;
  const [seedReport, setSeedReport] = useState<MAIRLSeedReport | null>(null);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [editText, setEditText] = useState('');
  const [newQuestion, setNewQuestion] = useState('');
  const [newCategory, setNewCategory] = useState('');

  const query = useQuery({
    queryKey: ['ma-irl', initiativeId, companyKey],
    enabled: Boolean(initiativeId && companyKey),
    queryFn: () => api.get<{ items: MACardIRLItem[] }>(base),
  });
  const refresh = () => void query.refetch();

  const seed = useMutation({
    mutationFn: () => api.post<MAIRLSeedReport>(`${base}/seed`, {}),
    onSuccess: (report) => {
      setSeedReport(report);
      refresh();
    },
  });
  const addItem = useMutation({
    mutationFn: () => api.post<MACardIRLItem>(`${base}/items`, { category: newCategory.trim(), question: newQuestion.trim() }),
    onSuccess: () => {
      setNewQuestion('');
      refresh();
    },
  });
  const patchItem = useMutation({
    mutationFn: ({ itemId, patch }: { itemId: string; patch: Partial<Pick<MACardIRLItem, 'category' | 'question' | 'status'>> }) =>
      api.patch<MACardIRLItem>(`${base}/items/${itemId}`, patch),
    onSuccess: refresh,
  });
  const deleteItem = useMutation({
    mutationFn: (itemId: string) => api.delete<{ deleted: boolean }>(`${base}/items/${itemId}`),
    onSuccess: refresh,
  });
  const exportIRL = useMutation({
    mutationFn: () => api.postBlob(`${base}/export`),
    onSuccess: (blob) => {
      const url = URL.createObjectURL(blob);
      const anchor = document.createElement('a');
      anchor.href = url;
      anchor.download = `irl-${companyName.replace(/[^\w\s-]/g, '').trim().replace(/\s+/g, '-').toLowerCase() || companyKey}.xlsx`;
      anchor.click();
      URL.revokeObjectURL(url);
    },
  });

  if (query.isLoading) return <Skeleton rows={5} />;
  if (query.isError) {
    return (
      <div className={styles.statePanel} role="alert">
        <Icon name="triangle-alert" size={22} />
        <p>{errorLabel(query.error)}</p>
      </div>
    );
  }

  const items = query.data?.items ?? [];
  const categories: string[] = [];
  const grouped = new Map<string, MACardIRLItem[]>();
  for (const item of items) {
    const category = item.category || 'generale';
    if (!grouped.has(category)) {
      grouped.set(category, []);
      categories.push(category);
    }
    grouped.get(category)?.push(item);
  }

  const startEdit = (item: MACardIRLItem) => {
    setEditingId(item.id);
    setEditText(item.question);
  };
  const commitEdit = (item: MACardIRLItem) => {
    const question = editText.trim();
    setEditingId(null);
    if (question && question !== item.question) {
      patchItem.mutate({ itemId: item.id, patch: { question } });
    }
  };

  return (
    <div>
      <div className={styles.irlToolbar}>
        <Button onClick={() => seed.mutate()} loading={seed.isPending}>
          Semina dalle fonti
        </Button>
        <Button variant="secondary" onClick={() => exportIRL.mutate()} loading={exportIRL.isPending} disabled={items.length === 0}>
          Export XLSX
        </Button>
        {seedReport ? (
          <span className={styles.irlSeedNote}>
            {seedReport.inserted} voci nuove su {seedReport.proposed} proposte
            {seedReport.inserted < seedReport.proposed ? ' (le altre erano già in lista)' : ''}
          </span>
        ) : null}
        {seed.isError ? <span className={styles.irlSeedNote}>Seed non riuscito: riprova.</span> : null}
      </div>

      {items.length === 0 ? (
        <EmptyState
          icon="clipboard-check"
          title="IRL vuota"
          text="Semina dalle fonti del dossier (flag, brief, lettura di tesi, template della famiglia) o aggiungi le voci manualmente."
        />
      ) : (
        categories.map((category) => (
          <div key={category} className={styles.irlGroup}>
            <h4 className={styles.irlGroupTitle}>{category}</h4>
            {(grouped.get(category) ?? []).map((item) => (
              <div key={item.id} className={styles.irlRow}>
                <button
                  type="button"
                  className={`${styles.irlStatusChip} ${
                    item.status === 'chiesta' ? styles.irlStatusChiesta : item.status === 'risposta' ? styles.irlStatusRisposta : ''
                  }`}
                  title="Cambia stato"
                  onClick={() => patchItem.mutate({ itemId: item.id, patch: { status: IRL_STATUS_CYCLE[item.status] } })}
                >
                  {item.status}
                </button>
                {editingId === item.id ? (
                  <input
                    className={styles.irlEditInput}
                    value={editText}
                    autoFocus
                    onChange={(event) => setEditText(event.target.value)}
                    onBlur={() => commitEdit(item)}
                    onKeyDown={(event) => {
                      if (event.key === 'Enter') commitEdit(item);
                      if (event.key === 'Escape') setEditingId(null);
                    }}
                  />
                ) : (
                  <span
                    className={`${styles.irlQuestion} ${item.status === 'na' ? styles.irlQuestionNa : ''}`}
                    onClick={() => startEdit(item)}
                  >
                    {item.question}
                  </span>
                )}
                <span className={styles.irlSource}>{IRL_SOURCE_LABELS[item.source] ?? item.source}</span>
                <button type="button" className={styles.irlDelete} title="Elimina voce" onClick={() => deleteItem.mutate(item.id)}>
                  <Icon name="x-circle" size={16} />
                </button>
              </div>
            ))}
          </div>
        ))
      )}

      <form
        className={styles.irlAddForm}
        onSubmit={(event) => {
          event.preventDefault();
          if (newQuestion.trim()) addItem.mutate();
        }}
      >
        <input
          className={styles.irlAddCategory}
          value={newCategory}
          placeholder="Categoria"
          onChange={(event) => setNewCategory(event.target.value)}
        />
        <input
          className={styles.irlAddInput}
          value={newQuestion}
          placeholder="Nuova richiesta…"
          onChange={(event) => setNewQuestion(event.target.value)}
        />
        <Button type="submit" variant="secondary" loading={addItem.isPending} disabled={!newQuestion.trim()}>
          Aggiungi
        </Button>
      </form>
    </div>
  );
}

function EmptyState({ icon, title, text }: { icon: string; title: string; text: string }) {
  return (
    <div className={styles.emptyState}>
      <Icon name={icon as any} size={28} />
      <h2>{title}</h2>
      <p>{text}</p>
    </div>
  );
}

function OverviewTab({ target }: { target: MATarget }) {
  const evidenceFamilies: { key: string; label: string }[] = [
    { key: 'aderenza', label: 'Aderenza strategia' },
    { key: 'opportunita', label: 'Opportunità deal' },
    { key: 'economico', label: 'Profilo economico' },
  ];
  return (
    <div>
      <div>
        <h3 className={styles.sectionTitle}>Razionale aderenza Strategia</h3>
        <p>{target.rationale || 'Nessun razionale disponibile.'}</p>
      </div>

      {target.missingCriteria.length > 0 ? (
        <p>
          <strong>Criteri mancanti o parziali:</strong> {target.missingCriteria.join(', ')}
        </p>
      ) : null}

      {evidenceFamilies.map((family) => {
        const items = target.evidence.filter((item) => item.family === family.key);
        if (items.length === 0) return null;
        return (
          <div key={family.key}>
            <h4>{family.label}</h4>
            {items.map((item) => (
              <EvidenceRow key={`${item.criterion}-${item.label}`} evidence={item} />
            ))}
          </div>
        );
      })}
      {target.evidence.filter((item) => !item.family).length > 0 && (
        <div>
          <h4>Altre evidenze</h4>
          {target.evidence
            .filter((item) => !item.family)
            .map((item) => (
              <EvidenceRow key={`${item.criterion}-${item.label}`} evidence={item} />
            ))}
        </div>
      )}
      <ScoreAdjustments adjustments={target.adjustments} />
      {target.flags && target.flags.length > 0 ? <FlagChips flags={target.flags} /> : null}
    </div>
  );
}

function EvidenceRow({ evidence }: { evidence: MATargetEvidence }) {
  const weight = evidence.weight ?? 0;
  return (
    <div>
      <div>
        <strong>{evidence.label}</strong>
        {weight > 0 ? (
          <small>
            {' '}
            {Math.round(evidence.points ?? 0)} / {Math.round(weight)}
          </small>
        ) : null}
      </div>
      <p>{evidence.value || evidenceStatusLabel(evidence.status)}</p>
    </div>
  );
}

function evidenceStatusLabel(value: string): string {
  if (value === 'match') return 'criterio soddisfatto';
  if (value === 'fuori_criterio') return 'fuori criterio';
  if (value === 'criterio_mancante') return 'criterio mancante';
  return 'match parziale';
}

function ScoreAdjustments({ adjustments }: { adjustments?: MATargetAdjustment[] }) {
  const items = (adjustments ?? []).filter((adj) => adj.factor < 1);
  if (items.length === 0) return null;
  return (
    <div>
      <span>Rettifiche al punteggio</span>
      {items.map((adj) => (
        <div key={adj.code}>
          <span>{adj.label}</span>
          <span>−{Math.round((1 - adj.factor) * 100)}%</span>
        </div>
      ))}
    </div>
  );
}

function FlagChips({ flags }: { flags?: MATargetFlag[] }) {
  if (!flags || flags.length === 0) return null;
  return (
    <div>
      {flags.map((flag) => (
        <span key={flag.code}>{flag.label}</span>
      ))}
    </div>
  );
}

function DeepAnalysisTab({
  deep,
  companyKey,
  onStarted,
}: {
  deep?: MADeepAnalysis;
  companyKey: string;
  onStarted: () => void;
}) {
  const api = useApiClient();
  const launch = useMutation({
    mutationFn: () => api.post(`/binocolo/v1/ma/companies/${encodeURIComponent(companyKey)}/deep-dive`, {}),
    onSuccess: onStarted,
  });

  const launchButton = (
    <>
      <Button onClick={() => launch.mutate()} loading={launch.isPending} disabled={!companyKey}>
        Avvia analisi
      </Button>
      {launch.isError ? <p>Avvio non riuscito: riprova.</p> : null}
    </>
  );

  if (!deep) {
    return (
      <div>
        <EmptyState
          icon="search"
          title="Analisi non ancora eseguita"
          text="Avvia l’analisi approfondita per generare il brief analista."
        />
        {launchButton}
      </div>
    );
  }
  if (deep.status === 'queued' || deep.status === 'running') {
    return (
      <EmptyState
        icon="clipboard-check"
        title={deep.status === 'queued' ? 'In coda' : 'Analisi in corso'}
        text="L'analisi approfondita è in elaborazione."
      />
    );
  }
  if (deep.status === 'failed') {
    return (
      <div>
        <EmptyState
          icon="file-text"
          title="Analisi non riuscita"
          text={`Si è verificato un problema (${deep.errorCode || 'errore'}).`}
        />
        {launchButton}
      </div>
    );
  }
  return <DeepAnalysisContent deep={deep} variant="full" />;
}

function FinancialsTab({ target }: { target: MATarget }) {
  const rawPayload = target.vendorPayload;
  const sheets = (() => {
    const allSheets = rawPayload?.balanceSheets?.all;
    if (Array.isArray(allSheets) && allSheets.length > 0) {
      return [...allSheets].sort((a, b) => (a.year ?? 0) - (b.year ?? 0));
    }
    if (target.turnoverYear && (target.turnover != null || target.employees != null)) {
      return [
        {
          year: target.turnoverYear,
          turnover: target.turnover,
          employees: target.employees,
          netWorth: null,
          totalAssets: null,
        },
      ];
    }
    return [];
  })();

  if (sheets.length === 0) {
    return <EmptyState icon="file-text" title="Dati finanziari non disponibili" text="Nessun dato di bilancio storico presente per questo target." />;
  }

  const latestTurnoverSheet = [...sheets].reverse().find((s: any) => s.turnover != null);
  const latestNetWorthSheet = [...sheets].reverse().find((s: any) => s.netWorth != null);
  const latestTotalAssetsSheet = [...sheets].reverse().find((s: any) => s.totalAssets != null);
  const latestEmployeesSheet = [...sheets].reverse().find((s: any) => s.employees != null);

  const getTrend = (key: 'turnover' | 'netWorth' | 'employees' | 'totalAssets') => {
    const validSheets = sheets.filter((s: any) => s[key] != null);
    if (validSheets.length < 2) return null;
    const lastIndex = validSheets.length - 1;
    const lastVal = (validSheets[lastIndex] as any)[key];
    const prevVal = (validSheets[lastIndex - 1] as any)[key];
    if (lastVal != null && prevVal != null && prevVal > 0) {
      return ((lastVal - prevVal) / prevVal) * 100;
    }
    return null;
  };

  const formatTrend = (pct: number | null) => {
    if (pct === null) return null;
    const sign = pct >= 0 ? '+' : '';
    const className = pct >= 0 ? styles.finTrendPositive : styles.finTrendNegative;
    return (
      <small className={className}>
        {sign}
        {pct.toFixed(1)}% YoY
      </small>
    );
  };

  const turnoverSheets = sheets.filter((s: any) => s.turnover != null);
  const maxTurnover = Math.max(...turnoverSheets.map((s: any) => s.turnover ?? 0), 1);

  const renderBarChart = () => {
    if (turnoverSheets.length === 0) return null;

    const width = 600;
    const height = 200;
    const paddingLeft = 65;
    const paddingRight = 20;
    const paddingTop = 25;
    const paddingBottom = 35;

    const chartWidth = width - paddingLeft - paddingRight;
    const chartHeight = height - paddingTop - paddingBottom;

    const barSpacing = chartWidth / turnoverSheets.length;
    const barWidth = Math.min(barSpacing * 0.5, 45);

    return (
      <div className={styles.chartContainer}>
        <div className={styles.chartTitle}>Andamento Fatturato (€)</div>
        <svg viewBox={`0 0 ${width} ${height}`} className={styles.chartSvg}>
          <defs>
            <linearGradient id="barGradient" x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stopColor="var(--color-accent)" />
              <stop offset="100%" stopColor="#7c6cff" />
            </linearGradient>
          </defs>

          {/* Grid lines (0%, 50%, 100%) */}
          <line
            x1={paddingLeft}
            y1={paddingTop}
            x2={width - paddingRight}
            y2={paddingTop}
            stroke="var(--color-border-subtle)"
            strokeDasharray="4 4"
          />
          <text
            x={paddingLeft - 10}
            y={paddingTop + 4}
            textAnchor="end"
            style={{ fontSize: '10px', fill: 'var(--color-text-muted)', fontFamily: 'var(--font-mono)' }}
          >
            {moneyFormat.format(maxTurnover)}
          </text>

          <line
            x1={paddingLeft}
            y1={paddingTop + chartHeight / 2}
            x2={width - paddingRight}
            y2={paddingTop + chartHeight / 2}
            stroke="var(--color-border-subtle)"
            strokeDasharray="4 4"
          />
          <text
            x={paddingLeft - 10}
            y={paddingTop + chartHeight / 2 + 4}
            textAnchor="end"
            style={{ fontSize: '10px', fill: 'var(--color-text-muted)', fontFamily: 'var(--font-mono)' }}
          >
            {moneyFormat.format(maxTurnover / 2)}
          </text>

          <line
            x1={paddingLeft}
            y1={paddingTop + chartHeight}
            x2={width - paddingRight}
            y2={paddingTop + chartHeight}
            stroke="var(--color-border)"
          />
          <text
            x={paddingLeft - 10}
            y={paddingTop + chartHeight + 4}
            textAnchor="end"
            style={{ fontSize: '10px', fill: 'var(--color-text-muted)', fontFamily: 'var(--font-mono)' }}
          >
            0 €
          </text>

          {/* Bars and Labels */}
          {turnoverSheets.map((sheet: any, idx: number) => {
            const val = sheet.turnover ?? 0;
            const barHeight = (val / maxTurnover) * chartHeight;
            const x = paddingLeft + idx * barSpacing + (barSpacing - barWidth) / 2;
            const y = paddingTop + chartHeight - barHeight;

            return (
              <g key={sheet.year}>
                <rect
                  x={x}
                  y={y}
                  width={barWidth}
                  height={Math.max(barHeight, 2)}
                  rx={4}
                  ry={4}
                  fill="url(#barGradient)"
                />
                <text
                  x={x + barWidth / 2}
                  y={y - 6}
                  textAnchor="middle"
                  style={{ fontSize: '11px', fill: 'var(--color-text)', fontWeight: 700, fontFamily: 'var(--font-mono)' }}
                >
                  {val >= 1000000 ? `${(val / 1000000).toFixed(2)}M` : `${Math.round(val / 1000).toLocaleString('it-IT')}k`}
                </text>
                <text
                  x={x + barWidth / 2}
                  y={paddingTop + chartHeight + 18}
                  textAnchor="middle"
                  style={{ fontSize: '11px', fill: 'var(--color-text-secondary)', fontWeight: 600 }}
                >
                  {sheet.year}
                </text>
              </g>
            );
          })}
        </svg>
      </div>
    );
  };

  return (
    <div>
      <div className={styles.finCardsGrid}>
        <div className={styles.finCard}>
          <span>Fatturato {latestTurnoverSheet ? `(${(latestTurnoverSheet as any).year})` : ''}</span>
          <strong>{(latestTurnoverSheet as any)?.turnover != null ? moneyFormat.format((latestTurnoverSheet as any).turnover) : '-'}</strong>
          {formatTrend(getTrend('turnover'))}
        </div>
        <div className={styles.finCard}>
          <span>Patrimonio Netto {latestNetWorthSheet ? `(${(latestNetWorthSheet as any).year})` : ''}</span>
          <strong>{(latestNetWorthSheet as any)?.netWorth != null ? moneyFormat.format((latestNetWorthSheet as any).netWorth) : '-'}</strong>
          {formatTrend(getTrend('netWorth'))}
        </div>
        <div className={styles.finCard}>
          <span>Attivo Totale {latestTotalAssetsSheet ? `(${(latestTotalAssetsSheet as any).year})` : ''}</span>
          <strong>
            {(latestTotalAssetsSheet as any)?.totalAssets != null ? moneyFormat.format((latestTotalAssetsSheet as any).totalAssets) : '-'}
          </strong>
          {formatTrend(getTrend('totalAssets'))}
        </div>
        <div className={styles.finCard}>
          <span>Dipendenti {latestEmployeesSheet ? `(${(latestEmployeesSheet as any).year})` : ''}</span>
          <strong>
            {(latestEmployeesSheet as any)?.employees != null ? numberFormat.format((latestEmployeesSheet as any).employees) : '-'}
          </strong>
          {formatTrend(getTrend('employees'))}
        </div>
      </div>

      {renderBarChart()}

      <div className={styles.tableContainer}>
        <table className={styles.finTable}>
          <thead>
            <tr>
              <th>Anno</th>
              <th>Fatturato</th>
              <th>Patrimonio Netto</th>
              <th>Attivo Totale</th>
              <th>Dipendenti</th>
            </tr>
          </thead>
          <tbody>
            {[...sheets].reverse().map((sheet: any) => (
              <tr key={sheet.year}>
                <td>
                  <strong>{sheet.year}</strong>
                </td>
                <td>{sheet.turnover != null ? moneyFormat.format(sheet.turnover) : '-'}</td>
                <td>{sheet.netWorth != null ? moneyFormat.format(sheet.netWorth) : '-'}</td>
                <td>{sheet.totalAssets != null ? moneyFormat.format(sheet.totalAssets) : '-'}</td>
                <td>{sheet.employees != null ? numberFormat.format(sheet.employees) : '-'}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}

function ShareholdersTab({ target }: { target: MATarget }) {
  const rawPayload = target.vendorPayload;
  const shareholders = (() => {
    if (!rawPayload) return [];
    if (Array.isArray(rawPayload.shareHolders)) return rawPayload.shareHolders;
    if (Array.isArray(rawPayload.shareholders)) {
      const list: any[] = [];
      for (const item of rawPayload.shareholders) {
        const percent = item.percentShare ?? 0;
        const info = item.shareholdersInformation;
        if (Array.isArray(info) && info.length > 0) {
          for (const sub of info) {
            list.push({
              name: sub.name,
              surname: sub.surname,
              companyName: sub.companyName,
              percentShare: sub.percentShare ?? percent,
              taxCode: sub.taxCode,
            });
          }
        } else {
          list.push({
            name: item.name,
            surname: item.surname,
            companyName: item.companyName,
            percentShare: percent,
            taxCode: item.taxCode,
          });
        }
      }
      return list;
    }
    return [];
  })();

  if (shareholders.length === 0) {
    return <EmptyState icon="file-text" title="Cap Table non disponibile" text="Nessun dato relativo ai soci presente per questo target." />;
  }

  return (
    <div className={styles.shGrid}>
      {shareholders.map((sh: any, index: number) => {
        const displayName = [sh.name, sh.surname].filter(Boolean).join(' ') || sh.companyName || 'Socio Sconosciuto';
        const percent = sh.percentShare ?? 0;
        return (
          <div key={index} className={styles.shCard}>
            <div className={styles.shName}>{displayName}</div>
            {sh.taxCode && <div className={styles.shTaxCode}>{sh.taxCode}</div>}
            <div className={styles.shShare}>
              <span>Quota societaria:</span>
              <strong>{percent > 0 ? `${percent}%` : 'n.d.'}</strong>
            </div>
            {percent > 0 && (
              <div className={styles.shProgressBarBg}>
                <div className={styles.shProgressBar} style={{ width: `${percent}%` }} />
              </div>
            )}
          </div>
        );
      })}
    </div>
  );
}

function RegistryTab({ target }: { target: MATarget }) {
  const rawPayload = target.vendorPayload;
  const detailedForm = rawPayload?.detailedLegalForm?.description || '-';
  const startDate = rawPayload?.startDate || '-';
  const regDate = rawPayload?.registrationDate || '-';
  const pec = rawPayload?.pec || rawPayload?.PEC || '-';

  return (
    <dl className={styles.dl}>
      <div>
        <dt>Ragione Sociale</dt>
        <dd>{target.companyName}</dd>
      </div>
      <div>
        <dt>Forma Giuridica</dt>
        <dd>{detailedForm}</dd>
      </div>
      <div>
        <dt>Partita IVA</dt>
        <dd>{target.vatCode || '-'}</dd>
      </div>
      <div>
        <dt>Codice Fiscale</dt>
        <dd>{target.taxCode || '-'}</dd>
      </div>
      <div>
        <dt>Inizio Attività</dt>
        <dd>{startDate}</dd>
      </div>
      <div>
        <dt>Data Registrazione</dt>
        <dd>{regDate}</dd>
      </div>
      <div>
        <dt>Stato Attività</dt>
        <dd>{target.activityStatus || '-'}</dd>
      </div>
      <div>
        <dt>PEC</dt>
        <dd>{pec}</dd>
      </div>
      <div className={styles.wide}>
        <dt>Sede Legale</dt>
        <dd>{[target.town, target.province].filter(Boolean).join(' · ') || '-'}</dd>
      </div>
    </dl>
  );
}

function webFinalActionLabel(action: string): string {
  switch (action) {
    case 'confirm':
      return 'conferma';
    case 'deprioritize':
      return 'declassa';
    case 'reject':
      return 'reject';
    case 'needs_domain_review':
      return 'review dominio';
    case 'needs_business_validation':
      return 'validazione business';
    default:
      return action || 'N/D';
  }
}

function candidateVerdictLabel(verdict: string): string {
  switch (verdict) {
    case 'strong_match':
      return 'Strong match';
    case 'match':
      return 'Match';
    case 'weak_match':
      return 'Weak match';
    case 'no_match':
      return 'No match';
    case 'unclear':
      return 'Unclear';
    default:
      return verdict || 'N/D';
  }
}

function candidateActionLabel(action: string): string {
  switch (action) {
    case 'confirm':
      return 'Conferma';
    case 'review':
      return 'Review';
    case 'downgrade':
      return 'Downgrade';
    case 'reject':
      return 'Reject';
    default:
      return action || 'N/D';
  }
}

function finalActionLabel(action: PipelineFinalAction): string {
  switch (action) {
    case 'confirm':
      return 'Conferma';
    case 'deprioritize':
      return 'Deprioritizza';
    case 'reject':
      return 'Reject';
    case 'needs_domain_review':
      return 'Review dominio';
    case 'needs_business_validation':
      return 'Validazione business';
    case 'no_website_structured':
      return 'Soli dati strutturati';
  }
}

function webValidationStateLabel(state: PipelineWebValidationState): string {
  switch (state) {
    case 'confirmed':
      return 'Confermato';
    case 'deprioritized':
      return 'Declassato';
    case 'domain_unresolved':
      return 'Dominio non risolto';
    case 'analysis_unavailable':
      return 'Analyst non disponibile';
    case 'no_website_declared':
      return 'Nessun sito dichiarato';
    case 'rejected':
      return 'Respinto';
    case 'unclear':
      return 'Incerto';
  }
}

function WebTab({ target }: { target: MATarget }) {
  const [isDetailsOpen, setIsDetailsOpen] = useState(false);
  const validation = target.webValidation;

  if (!validation) {
    return <EmptyState icon="network" title="Analisi WEB non disponibile" text="Nessuna analisi web registrata per questo target." />;
  }

  return (
    <div>
      <article>
        <div>
          <h3>Final reconciliation</h3>
          <p>{validation.finalDecision.reason}</p>
        </div>
        <div>
          <span>{finalActionLabel(validation.finalDecision.finalAction)}</span>
          <span>{webValidationStateLabel(validation.webValidationState)}</span>
        </div>
        <div>
          <div>
            <span>Score iniziale</span>
            <p>
              {validation.finalDecision.deterministicScore} · {validation.finalDecision.initialMatchState}
            </p>
          </div>
          <div>
            <span>Validazione web</span>
            <p>
              {validation.finalDecision.webScore} · {webValidationStateLabel(validation.webValidationState)}
            </p>
          </div>
          <div>
            <span>Analyst</span>
            <p>
              {validation.finalDecision.analystVerdict
                ? `${candidateVerdictLabel(validation.finalDecision.analystVerdict)} · ${candidateActionLabel(
                    validation.finalDecision.analystAction ?? '',
                  )}`
                : 'N/D'}
            </p>
          </div>
          <div>
            <span>Dominio</span>
            <p>
              {validation.selectedDomain
                ? `${validation.selectedDomain}${validation.domainConfidence ? ` · ${validation.domainConfidence}` : ''}`
                : 'N/D'}
            </p>
          </div>
        </div>

        <button type="button" onClick={() => setIsDetailsOpen(!isDetailsOpen)}>
          <span>Altri dati</span>
          <Icon name={isDetailsOpen ? 'chevron-up' : 'chevron-down'} size={16} />
        </button>

        {isDetailsOpen && (
          <>
            <div>
              <div>
                <span>Persistenza</span>
                <p>{`salvata · ${new Date(validation.updatedAt).toLocaleString('it-IT')}`}</p>
              </div>
              <div>
                <span>Freshness</span>
                <p>
                  {validation.freshness} · stale dopo {new Date(validation.staleAfter).toLocaleDateString('it-IT')}
                </p>
              </div>
            </div>
            {validation.finalDecision.reasons.length > 0 ? (
              <div>
                {validation.finalDecision.reasons.slice(0, 6).map((reason) => (
                  <span key={reason}>{reason}</span>
                ))}
              </div>
            ) : null}
          </>
        )}
      </article>

      {validation.candidateMatchAnalysis ? (
        <article>
          <div>
            <h3>Candidate match analyst</h3>
            <p>{validation.candidateMatchAnalysis.rationale}</p>
          </div>
          <div>
            <span>{candidateVerdictLabel(validation.candidateMatchAnalysis.verdict)}</span>
            <span>{candidateActionLabel(validation.candidateMatchAnalysis.recommendedAction)}</span>
          </div>
          <div>
            <div>
              <span>Sector fit</span>
              <p>{validation.candidateMatchAnalysis.sectorFit || 'N/D'}</p>
            </div>
            <div>
              <span>Business fit</span>
              <p>{validation.candidateMatchAnalysis.businessFit || 'N/D'}</p>
            </div>
          </div>
          <div>
            <div>
              <span>Prove a favore</span>
              {validation.candidateMatchAnalysis.evidenceFor.length > 0 ? (
                <ul>
                  {validation.candidateMatchAnalysis.evidenceFor.map((item) => (
                    <li key={item}>{item}</li>
                  ))}
                </ul>
              ) : (
                <p>N/D</p>
              )}
            </div>
            <div>
              <span>Contro / lacune</span>
              {[...validation.candidateMatchAnalysis.evidenceAgainst, ...validation.candidateMatchAnalysis.missingEvidence].length > 0 ? (
                <ul>
                  {[...validation.candidateMatchAnalysis.evidenceAgainst, ...validation.candidateMatchAnalysis.missingEvidence].map((item) => (
                    <li key={item}>{item}</li>
                  ))}
                </ul>
              ) : (
                <p>N/D</p>
              )}
            </div>
          </div>
          {validation.candidateMatchAnalysis.negativeSignals.length > 0 ? (
            <div>
              <span>Segnali negativi</span>
              <p>{validation.candidateMatchAnalysis.negativeSignals.join(' · ')}</p>
            </div>
          ) : null}
          {validation.candidateMatchAnalysis.conceptAliases.length > 0 ? (
            <div>
              <span>Alias concettuali</span>
              {validation.candidateMatchAnalysis.conceptAliases.map((alias) => (
                <p key={`${alias.term}-${alias.matchedConcept}`}>
                  <strong>{alias.term}</strong> -&gt; {alias.matchedConcept}
                  {alias.evidence ? ` · ${alias.evidence}` : ''}
                </p>
              ))}
            </div>
          ) : null}
        </article>
      ) : validation.candidateMatchError ? (
        <div>
          <span>Candidate match analyst non disponibile: {validation.candidateMatchError}</span>
        </div>
      ) : null}
    </div>
  );
}

// webFinalActionLabel non è usata dal blocco final reconciliation (che usa
// finalActionLabel), ma è mantenuta per parità col vecchio modale in caso di
// futuri riusi (badge secondario sullo stato pipeline al posto della sola
// finalAction).
void webFinalActionLabel;
