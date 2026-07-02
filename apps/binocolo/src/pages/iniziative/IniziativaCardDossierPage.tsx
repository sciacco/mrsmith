import { useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { ApiError } from '@mrsmith/api-client';
import { useQuery } from '@tanstack/react-query';
import { Icon, Skeleton } from '@mrsmith/ui';
import { useApiClient } from '../../api/client';
import type {
  MACardDossier,
  MADeepAnalysis,
  MADeepMetric,
  MADeepValuation,
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

type TabKey = 'overview' | 'deep' | 'financials' | 'shareholders' | 'registry' | 'web';

const TABS: { key: TabKey; label: string; icon: string }[] = [
  { key: 'overview', label: 'Panoramica', icon: 'external-link' },
  { key: 'deep', label: 'Analisi', icon: 'bar-chart-2' },
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
        {activeTab === 'deep' && <DeepAnalysisTab deep={target.deep} />}
        {activeTab === 'financials' && <FinancialsTab target={target} />}
        {activeTab === 'shareholders' && <ShareholdersTab target={target} />}
        {activeTab === 'registry' && <RegistryTab target={target} />}
        {activeTab === 'web' && <WebTab target={target} />}
      </div>

      <CompanyRegistrySection companyKey={target.companyKey ?? companyKey ?? ''} vatCode={target.vatCode} companyName={target.companyName} />
    </main>
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

function ragLabel(rag: string): string {
  switch (rag) {
    case 'green':
      return 'Solido';
    case 'amber':
      return 'Attenzione';
    case 'red':
      return 'Critico';
    default:
      return 'n.d.';
  }
}

function formatMetricValue(value: number, unit: string): string {
  const rounded = Math.round(value * 10) / 10;
  if (unit === '%') return `${rounded}%`;
  if (unit === 'x') return `${rounded}×`;
  if (unit === 'gg') return `${Math.round(value)} gg`;
  return String(rounded);
}

function formatPercentagesInText(text: string): string {
  return text.replace(/(\d+)\.(\d{2,})%/g, (_, p1, p2) => {
    const num = parseFloat(`${p1}.${p2}`);
    return `${num.toFixed(1)}%`;
  });
}

function DeepAnalysisTab({ deep }: { deep?: MADeepAnalysis }) {
  if (!deep) {
    return (
      <EmptyState
        icon="search"
        title="Analisi non ancora eseguita"
        text="Assegna almeno una stella e avvia l'approfondimento dalla card per generare il brief analista."
      />
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
      <EmptyState
        icon="file-text"
        title="Analisi non riuscita"
        text={`Si è verificato un problema (${deep.errorCode || 'errore'}).`}
      />
    );
  }
  const scorecard = deep.scorecard;
  if (!scorecard) {
    return <EmptyState icon="file-text" title="Dati non disponibili" text="L'analisi è pronta ma non contiene dati finanziari." />;
  }
  const groups: { key: string; label: string }[] = [
    { key: 'redditivita', label: 'Redditività' },
    { key: 'leva', label: 'Leva e struttura' },
    { key: 'liquidita', label: 'Liquidità' },
    { key: 'efficienza', label: 'Efficienza' },
    { key: 'crescita', label: 'Crescita' },
  ];
  return (
    <div>
      <div>
        <span>{ragLabel(scorecard.overallRag)}</span>
        {scorecard.turnover != null ? (
          <span>
            Fatturato{scorecard.turnoverYear ? ` (${scorecard.turnoverYear})` : ''}: <strong>{moneyFormat.format(scorecard.turnover)}</strong>
          </span>
        ) : null}
        {scorecard.ebitda != null ? (
          <span>
            EBITDA: <strong>{moneyFormat.format(scorecard.ebitda)}</strong>
          </span>
        ) : null}
        {scorecard.pfn != null ? (
          <span>
            PFN: <strong>{moneyFormat.format(scorecard.pfn)}</strong>
          </span>
        ) : null}
        {scorecard.netWorth != null ? (
          <span>
            Patrimonio netto: <strong>{moneyFormat.format(scorecard.netWorth)}</strong>
          </span>
        ) : null}
      </div>

      {deep.brief ? <DeepBriefBlock brief={deep.brief} /> : null}
      {deep.valuation ? <DeepValuation valuation={deep.valuation} /> : null}

      {groups.map((group) => {
        const metrics = scorecard.metrics.filter((metric) => metric.group === group.key);
        if (metrics.length === 0) return null;
        return (
          <div key={group.key}>
            <h5>{group.label}</h5>
            {metrics.map((metric) => (
              <DeepMetricRow key={metric.key} metric={metric} />
            ))}
          </div>
        );
      })}
    </div>
  );
}

function DeepMetricRow({ metric }: { metric: MADeepMetric }) {
  return (
    <div>
      <span>{metric.label}</span>
      <span>{metric.value != null ? formatMetricValue(metric.value, metric.unit) : 'n.d.'}</span>
    </div>
  );
}

function DeepValuation({ valuation }: { valuation: MADeepValuation }) {
  return (
    <div>
      <h5>Inquadramento di valore</h5>
      <div>
        <span>Enterprise Value stimato</span>
        <strong>
          {moneyFormat.format(valuation.evLow)} – {moneyFormat.format(valuation.evHigh)}
        </strong>
      </div>
      {valuation.equityLow != null && valuation.equityHigh != null ? (
        <div>
          <span>Equity implicito (EV − PFN)</span>
          <strong>
            {moneyFormat.format(valuation.equityLow)} – {moneyFormat.format(valuation.equityHigh)}
          </strong>
        </div>
      ) : null}
      <small>
        {valuation.method === 'ev_sales' ? 'EV/Sales' : 'EV/EBITDA'} {valuation.multiple}× · sconto PMI {valuation.haircutPct}%
        {valuation.sector ? ` · ${valuation.sector}` : ''}
        {valuation.source ? ` · ${valuation.source}` : ''}
        {valuation.sourceDate ? ` ${valuation.sourceDate}` : ''}
        {valuation.nFirms ? ` · ${valuation.nFirms} soc.` : ''}
      </small>
      {valuation.caveat ? <small>{valuation.caveat}</small> : null}
    </div>
  );
}

function DeepBriefBlock({ brief }: { brief: NonNullable<MADeepAnalysis['brief']> }) {
  return (
    <div>
      <h5>Brief analista</h5>
      {brief.verdict ? <p>{formatPercentagesInText(brief.verdict)}</p> : null}
      {brief.thesisReading ? <p>{formatPercentagesInText(brief.thesisReading)}</p> : null}
      {brief.redFlags && brief.redFlags.length > 0 ? (
        <ul>
          {brief.redFlags.map((flag, index) => (
            <li key={index}>
              <strong>{formatPercentagesInText(flag.claim)}</strong>
              {flag.ddQuestion ? <span> — {formatPercentagesInText(flag.ddQuestion)}</span> : null}
            </li>
          ))}
        </ul>
      ) : null}
    </div>
  );
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

  return (
    <div>
      <table className={styles.mono}>
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
    <div>
      {shareholders.map((sh: any, index: number) => {
        const displayName = [sh.name, sh.surname].filter(Boolean).join(' ') || sh.companyName || 'Socio Sconosciuto';
        const percent = sh.percentShare ?? 0;
        return (
          <div key={index}>
            <div>{displayName}</div>
            {sh.taxCode && <div className={styles.mono}>{sh.taxCode}</div>}
            <div>
              <span>Quota societaria:</span>
              <strong>{percent > 0 ? `${percent}%` : 'n.d.'}</strong>
            </div>
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
