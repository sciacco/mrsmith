import { useState, type FormEvent } from 'react';
import { ApiError } from '@mrsmith/api-client';
import { useMutation, useQuery } from '@tanstack/react-query';
import { Button, Icon } from '@mrsmith/ui';
import { useApiClient } from '../api/client';
import type {
  MABMFamily,
  MACompanyDossier,
  MADeepBrief,
  MADeepMetric,
  MADeepQualityFlag,
  MADeepScorecard,
  MADeepValuation,
} from '../api/types';
import iicLegendRaw from '../data/iicLegend.json';
import styles from './CompanyDossierPage.module.css';

const iicLegend = iicLegendRaw as Record<string, { description: string; section: string }>;

const COST_LABEL = '0,30 €';
const eur0 = new Intl.NumberFormat('it-IT', { style: 'currency', currency: 'EUR', maximumFractionDigits: 0 });
const num1 = new Intl.NumberFormat('it-IT', { maximumFractionDigits: 1 });
const num2 = new Intl.NumberFormat('it-IT', { maximumFractionDigits: 2 });

function isValidVatOrTax(v: string): boolean {
  if (/^[0-9]{11}$/.test(v)) return true;
  if (/^[A-Z0-9]{16}$/.test(v)) return true;
  return false;
}

function eurCompact(value?: number): string {
  if (value == null || !Number.isFinite(value)) return '—';
  const abs = Math.abs(value);
  if (abs >= 1_000_000) return `${num2.format(value / 1_000_000)} M€`;
  if (abs >= 1_000) return `${num1.format(value / 1_000)} k€`;
  return eur0.format(value);
}

function ragClass(rag?: string): string {
  switch (rag) {
    case 'green':
      return styles.ragGreen ?? '';
    case 'amber':
      return styles.ragAmber ?? '';
    case 'red':
      return styles.ragRed ?? '';
    default:
      return styles.ragNa ?? '';
  }
}

const RAG_LABEL: Record<string, string> = { green: 'Solido', amber: 'Intermedio', red: 'Debole', na: 'N/D' };

function metricDisplay(m: MADeepMetric): string {
  if (m.value == null) return '—';
  const v = m.unit === 'x' || m.unit === 'gg' ? num2.format(m.value) : num1.format(m.value);
  if (m.unit === 'gg') return `${v} gg`;
  if (m.unit === '%') return `${v}%`;
  if (m.unit === 'x') return `${v}×`;
  return v;
}

function year(d?: string): string {
  if (!d) return '—';
  const m = /^(\d{4})/.exec(d);
  return m?.[1] ?? '—';
}

function errorLabel(error: unknown): string {
  if (error instanceof ApiError) {
    if (error.status === 400) return 'Partita IVA non valida.';
    if (error.status === 402) return 'Credito OpenAPI.it esaurito.';
    if (error.status === 403) return 'Non hai accesso a Binocolo.';
    if (error.status === 502) return 'OpenAPI.it non ha risposto correttamente.';
    if (error.status === 503) return 'Servizio non configurato in questo ambiente.';
    return `Richiesta non riuscita (${error.status}).`;
  }
  return 'Richiesta non riuscita.';
}

// ── raw IT-full shape (only the fields we render) ──────────────────────────────
interface CodeValue {
  code: string;
  value: number;
}
interface ItFull {
  companyDetails?: { companyName?: string };
  pec?: string;
  mail?: { email?: string };
  contacts?: { telephoneNumber?: string };
  webAndSocial?: { website?: string; linkedin?: string };
  legalForm?: { legalForm?: { description?: string }; detailedLegalForm?: { description?: string } };
  companyDates?: { startDate?: string; incorporationDate?: string };
  companyStatus?: { activityStatus?: { description?: string } };
  address?: { streetName?: string; zipCode?: string; town?: string; province?: { code?: string }; region?: { description?: string } };
  atecoClassification?: {
    ateco?: { code?: string; description?: string };
    ateco2022?: { code?: string; description?: string };
    secondaryAteco?: string;
  };
  ecofin?: {
    turnover?: number;
    turnoverYear?: number;
    turnoverTrend?: number;
    netWorth?: number;
    shareCapital?: number;
    enterpriseSize?: { description?: string };
    balanceSheetDate?: string;
  };
  employees?: { employee?: number; employeeRange?: { description?: string }; employeeTrend?: number };
  managers?: Array<{
    name?: string;
    surname?: string;
    age?: number;
    taxCode?: string;
    isLegalRepresentative?: boolean;
    roles?: Array<{ role?: { description?: string } }>;
  }>;
  shareholders?: Array<{
    percentShare?: number;
    shareholdersInformation?: Array<{ name?: string; surname?: string; companyName?: string; taxCode?: string; town?: string }>;
  }>;
  corporateGroups?: {
    groupName?: string;
    belongsToGroup?: boolean;
    holdingCompanyName?: string;
    totalGroupSubsidiaries?: number;
    hasForeignSubsidiaries?: boolean;
  };
  subsidiaries?: Array<{ companyName?: string; town?: string; province?: { description?: string } }>;
  allOffices?: Array<{
    address?: { town?: string; province?: { code?: string } };
    companyStatus?: { activityStatus?: { description?: string } };
    companyDetails?: { officeType?: { description?: string }; reaCode?: string };
  }>;
  publicTenders?: Array<{ year?: string; won?: number; applied?: number; value?: number }>;
  assetsAggregateValues?: CodeValue[];
  liabilitiesAggregateValues?: CodeValue[];
  incomeStatementAggregateValues?: CodeValue[];
  annualResult?: CodeValue[];
}

interface BilancioRow {
  code: string;
  description: string;
  value: number;
}

function bilancioRows(entries?: CodeValue[]): BilancioRow[] {
  if (!Array.isArray(entries)) return [];
  const out: BilancioRow[] = [];
  for (const e of entries) {
    if (!e || typeof e.value !== 'number' || e.value === 0) continue;
    const legend = iicLegend[e.code];
    if (!legend) continue;
    out.push({ code: e.code, description: legend.description, value: e.value });
  }
  return out;
}

// ── sections ───────────────────────────────────────────────────────────────────

const SECTIONS = [
  { id: 'sintesi', label: 'Sintesi' },
  { id: 'valutazione', label: 'Valutazione' },
  { id: 'famiglia', label: 'Business model' },
  { id: 'kpi', label: 'Indicatori' },
  { id: 'bilancio', label: 'Bilancio' },
  { id: 'soci', label: 'Soci e gruppo' },
  { id: 'sedi', label: 'Sedi e cariche' },
  { id: 'anagrafica', label: 'Anagrafica' },
  { id: 'dati', label: 'Dati completi' },
] as const;

function Provenance({ kind }: { kind: 'elab' | 'fonte' }) {
  return (
    <span className={`${styles.prov} ${kind === 'elab' ? styles.provElab : styles.provFonte}`}>
      {kind === 'elab' ? 'Elaborazione Binocolo' : 'Fonte: registro / IT-full'}
    </span>
  );
}

function SectionHead({ id, title, prov }: { id: string; title: string; prov: 'elab' | 'fonte' }) {
  return (
    <div className={styles.sectionHead}>
      <h2 id={id} className={styles.sectionTitle}>
        {title}
      </h2>
      <Provenance kind={prov} />
    </div>
  );
}

function Hero({ dossier, itf }: { dossier: MACompanyDossier; itf: ItFull }) {
  const sc = dossier.scorecard;
  const val = dossier.valuation;
  const rag = sc?.overallRag ?? dossier.brief?.rag;
  const name = itf.companyDetails?.companyName ?? '—';
  const ateco = itf.atecoClassification?.ateco?.code ?? sc?.atecoCode;
  const town = itf.address?.town;
  const trend = itf.ecofin?.turnoverTrend;
  const equity = val?.equityLow != null && val?.equityHigh != null
    ? `${eurCompact(val.equityLow)} – ${eurCompact(val.equityHigh)}`
    : null;
  return (
    <header className={styles.hero}>
      <div className={styles.heroTop}>
        <div>
          <div className={styles.heroName}>{name}</div>
          <div className={styles.heroMeta}>
            <span className={styles.mono}>{dossier.vatCode}</span>
            {ateco ? <span>ATECO {ateco}</span> : null}
            {town ? <span>{town}</span> : null}
            {itf.companyStatus?.activityStatus?.description ? (
              <span>{itf.companyStatus.activityStatus.description}</span>
            ) : null}
          </div>
        </div>
        {rag ? <span className={`${styles.ragPill} ${ragClass(rag)}`}>{RAG_LABEL[rag] ?? rag}</span> : null}
      </div>

      <div className={styles.heroFigures}>
        {equity ? (
          <div className={styles.heroEquity}>
            <span className={styles.heroEquityLabel}>Equity stimata</span>
            <span className={styles.heroEquityValue}>{equity}</span>
            {val && val.evLow != null ? (
              <span className={styles.heroEquityNote}>
                EV {eurCompact(val.evLow)} – {eurCompact(val.evHigh)} · {val.method === 'ev_sales' ? 'EV/Sales' : 'EV/EBITDA'} {num2.format(val.multiple)}×
              </span>
            ) : null}
          </div>
        ) : null}
        <div className={styles.heroKpis}>
          <Kpi label="Fatturato" value={eurCompact(sc?.turnover)} sub={trend != null ? `${trend > 0 ? '▲' : trend < 0 ? '▼' : ''} ${num1.format(Math.abs(trend))}%` : undefined} trend={trend} />
          <Kpi label="EBITDA" value={eurCompact(sc?.ebitda)} />
          <Kpi label="PFN" value={eurCompact(sc?.pfn)} />
          <Kpi label="Patrimonio netto" value={eurCompact(sc?.netWorth)} />
        </div>
      </div>

      {dossier.brief?.verdict ? <p className={styles.heroVerdict}>{dossier.brief.verdict}</p> : null}
    </header>
  );
}

function Kpi({ label, value, sub, trend }: { label: string; value: string; sub?: string; trend?: number }) {
  const trendClass = trend == null ? '' : trend > 0 ? styles.kpiUp : trend < 0 ? styles.kpiDown : '';
  return (
    <div className={styles.kpi}>
      <span className={styles.kpiLabel}>{label}</span>
      <span className={styles.kpiValue}>{value}</span>
      {sub ? <span className={`${styles.kpiSub} ${trendClass}`}>{sub}</span> : null}
    </div>
  );
}

function BriefSection({ brief }: { brief?: MADeepBrief }) {
  if (!brief) return null;
  return (
    <section className={styles.section}>
      <SectionHead id="sintesi" title="Sintesi e lettura" prov="elab" />
      {brief.businessProfile ? (
        <div className={styles.briefBlock}>
          <h3 className={styles.briefH}>Profilo</h3>
          <p>{brief.businessProfile}</p>
        </div>
      ) : null}
      {brief.thesisReading ? (
        <div className={styles.briefBlock}>
          <h3 className={styles.briefH}>Lettura finanziaria</h3>
          <p>{brief.thesisReading}</p>
        </div>
      ) : null}
      {brief.strengths && brief.strengths.length > 0 ? (
        <div className={styles.briefBlock}>
          <h3 className={styles.briefH}>Punti di forza</h3>
          <ul className={styles.bullets}>
            {brief.strengths.map((s, i) => (
              <li key={i}>{s}</li>
            ))}
          </ul>
        </div>
      ) : null}
      {brief.redFlags && brief.redFlags.length > 0 ? (
        <div className={styles.briefBlock}>
          <h3 className={styles.briefH}>Rischi e red flag</h3>
          <div className={styles.flags}>
            {brief.redFlags.map((f, i) => (
              <div key={i} className={styles.flag}>
                <div className={styles.flagTop}>
                  <span className={`${styles.flagSev} ${f.severity === 'warning' ? styles.sevWarn : styles.sevInfo}`}>
                    {f.severity === 'warning' ? 'Attenzione' : 'Info'}
                  </span>
                  {f.category ? <span className={styles.flagCat}>{f.category}</span> : null}
                </div>
                <p className={styles.flagClaim}>{f.claim}</p>
                {f.ddQuestion ? <p className={styles.flagDd}>DD — {f.ddQuestion}</p> : null}
              </div>
            ))}
          </div>
        </div>
      ) : null}
      {brief.ddQuestions && brief.ddQuestions.length > 0 ? (
        <div className={styles.briefBlock}>
          <h3 className={styles.briefH}>Cosa indagare</h3>
          <ul className={styles.bullets}>
            {brief.ddQuestions.map((q, i) => (
              <li key={i}>{q}</li>
            ))}
          </ul>
        </div>
      ) : null}
    </section>
  );
}

function ValuationSection({
  valuation,
  brief,
  flags,
}: {
  valuation?: MADeepValuation;
  brief?: MADeepBrief;
  flags?: MADeepQualityFlag[];
}) {
  if (!valuation) return null;
  const v = valuation;
  const bridge = v.bridge;
  return (
    <section className={styles.section}>
      <SectionHead id="valutazione" title="Valutazione" prov="elab" />
      <div className={styles.valGrid}>
        <ValItem label="Metodo" value={v.method === 'ev_sales' ? 'EV / Sales' : 'EV / EBITDA'} />
        <ValItem label="Multiplo" value={`${num2.format(v.multiple)}×`} />
        <ValItem label="Haircut PMI" value={`${num1.format(v.haircutPct)}%`} />
        <ValItem label="Enterprise Value" value={`${eurCompact(v.evLow)} – ${eurCompact(v.evHigh)}`} />
        {bridge?.pfn ? <ValItem label="− PFN" value={eurCompact(bridge.pfn.value)} /> : null}
        {bridge?.tfr ? <ValItem label="− TFR" value={eurCompact(bridge.tfr.value)} /> : null}
        {bridge?.taxFund ? <ValItem label="− Fondo imposte" value={eurCompact(bridge.taxFund.value)} /> : null}
        {v.equityLow != null ? (
          <ValItem label="Equity" value={`${eurCompact(v.equityLow)} – ${eurCompact(v.equityHigh)}`} strong />
        ) : (
          <ValItem label="Equity" value="non calcolabile (PFN assente)" />
        )}
        {bridge?.shareholderLoans ? (
          <ValItem label="di cui finanz. soci (negoziale)" value={eurCompact(bridge.shareholderLoans.value)} />
        ) : null}
        {v.sector ? <ValItem label="Settore" value={`${v.sector}${v.nFirms ? ` · ${v.nFirms} comp.` : ''}`} /> : null}
      </div>
      {v.lowMethod === 'ev_sales' ? (
        <p className={styles.valSource}>Estremo basso su EV/Sales: EBITDA prudenziale sotto soglia.</p>
      ) : null}
      {flags && flags.length > 0 ? (
        <ul className={styles.valFlags}>
          {flags.map((flag) => (
            <li key={flag.code}>
              <strong>{flag.label}</strong> — {flag.evidence}
              {flag.ddQuestion ? <span> · DD: {flag.ddQuestion}</span> : null}
            </li>
          ))}
        </ul>
      ) : null}
      {brief?.valuationRationale ? <p className={styles.valRationale}>{brief.valuationRationale}</p> : null}
      <p className={styles.valSource}>
        {v.source}
        {v.sourceDate ? ` · ${v.sourceDate}` : ''}
        {v.caveat ? ` — ${v.caveat}` : ''}
      </p>
    </section>
  );
}

function ValItem({ label, value, strong }: { label: string; value: string; strong?: boolean }) {
  return (
    <div className={`${styles.valItem} ${strong ? styles.valItemStrong : ''}`}>
      <span className={styles.valLabel}>{label}</span>
      <span className={styles.valValue}>{value}</span>
    </div>
  );
}

const FAMILY_LABEL: Record<string, string> = {
  servizi_ricorrenti: 'Servizi ricorrenti',
  progetto_integrazione: 'Progetto & integrazione',
  rivendita_var: 'Rivendita / VAR',
  software_prodotto: 'Software prodotto',
};

// FamilySection — famiglia di business model (Fase 3): suggerimento del motore
// con evidenza + ratifica/override dell'analista. La famiglia guida soglie RAG e
// riga Damodaran; entra in vigore al prossimo ricalcolo dell'azienda.
function FamilySection({ vatCode, bmFamily }: { vatCode: string; bmFamily?: MABMFamily }) {
  const api = useApiClient();
  const [current, setCurrent] = useState<MABMFamily | undefined>(bmFamily);
  const [selected, setSelected] = useState<string>(bmFamily?.family ?? '');
  const ratify = useMutation({
    mutationFn: (family: string) =>
      api.put<MABMFamily>(`/binocolo/v1/ma/companies/${encodeURIComponent(vatCode)}/bm-family`, { family }),
    onSuccess: (data) => {
      setCurrent(data ?? undefined);
      setSelected(data?.family ?? '');
    },
  });
  const effective = current?.family || current?.suggestedFamily || '';
  return (
    <section className={styles.section}>
      <SectionHead id="famiglia" title="Business model" prov="elab" />
      <div className={styles.valGrid}>
        <ValItem
          label="Famiglia"
          value={effective ? `${FAMILY_LABEL[effective] ?? effective}${current?.family ? ' (ratificata)' : ' (suggerita)'}` : 'non classificata'}
          strong
        />
        {current?.suggestedFamily && !current.family ? (
          <ValItem label="Evidenza" value={current.suggestedEvidence || current.suggestedSource || '—'} />
        ) : null}
        {current?.family && current.suggestedFamily && current.family !== current.suggestedFamily ? (
          <ValItem
            label="Suggerita dal motore"
            value={`${FAMILY_LABEL[current.suggestedFamily] ?? current.suggestedFamily} — ${current.suggestedEvidence}`}
          />
        ) : null}
      </div>
      <div className={styles.familyControls}>
        <select
          className={styles.familySelect}
          value={selected}
          onChange={(event) => setSelected(event.target.value)}
          aria-label="Famiglia di business model"
        >
          <option value="">— nessuna ratifica —</option>
          {Object.entries(FAMILY_LABEL).map(([value, label]) => (
            <option key={value} value={value}>
              {label}
            </option>
          ))}
        </select>
        <Button
          variant="secondary"
          onClick={() => ratify.mutate(selected)}
          loading={ratify.isPending}
          disabled={selected === (current?.family ?? '')}
        >
          {selected ? 'Ratifica' : 'Revoca ratifica'}
        </Button>
      </div>
      <p className={styles.valSource}>
        La famiglia guida le soglie degli indicatori e il multiplo di settore; entra in vigore al prossimo ricalcolo
        dell'analisi.
      </p>
      {ratify.isError ? <p className={styles.valSource}>Errore nel salvataggio: riprova.</p> : null}
    </section>
  );
}

const GROUP_LABEL: Record<string, string> = {
  redditivita: 'Redditività',
  leva: 'Leva e struttura',
  liquidita: 'Liquidità',
  efficienza: 'Efficienza',
  crescita: 'Crescita',
  contorno: 'Struttura finanziaria del venditore',
};

function ScorecardSection({ scorecard }: { scorecard?: MADeepScorecard }) {
  if (!scorecard || scorecard.metrics.length === 0) return null;
  const groups = new Map<string, MADeepMetric[]>();
  for (const m of scorecard.metrics) {
    if (m.tier === 'contorno') continue;
    const arr = groups.get(m.group) ?? [];
    arr.push(m);
    groups.set(m.group, arr);
  }
  const contorno = scorecard.metrics.filter((m) => m.tier === 'contorno');
  if (contorno.length > 0) {
    groups.set('contorno', contorno);
  }
  return (
    <section className={styles.section}>
      <SectionHead id="kpi" title="Indicatori finanziari" prov="elab" />
      <div className={styles.metricGroups}>
        {[...groups.entries()].map(([group, metrics]) => (
          <div key={group} className={`${styles.metricGroup} ${group === 'contorno' ? styles.metricGroupContorno : ''}`}>
            <h3 className={styles.metricGroupTitle}>{GROUP_LABEL[group] ?? group}</h3>
            <div className={styles.metricGrid}>
              {metrics.map((m) => (
                <div key={m.key} className={styles.metric}>
                  <span className={`${styles.metricDot} ${ragClass(m.rag)}`} aria-hidden="true" />
                  <span className={styles.metricLabel}>{m.label}</span>
                  <span className={styles.metricValue}>{metricDisplay(m)}</span>
                </div>
              ))}
            </div>
            {group === 'contorno' ? (
              <p className={styles.valSource}>
                Fuori dal giudizio complessivo: il compratore sostituisce la struttura del capitale al closing.
              </p>
            ) : null}
          </div>
        ))}
      </div>
    </section>
  );
}

function BilancioSection({ itf }: { itf: ItFull }) {
  const blocks: Array<{ title: string; rows: BilancioRow[] }> = [
    { title: 'Stato patrimoniale — attivo', rows: bilancioRows(itf.assetsAggregateValues) },
    { title: 'Stato patrimoniale — passivo', rows: bilancioRows(itf.liabilitiesAggregateValues) },
    { title: 'Conto economico', rows: bilancioRows(itf.incomeStatementAggregateValues) },
    { title: 'Risultato', rows: bilancioRows(itf.annualResult) },
  ].filter((b) => b.rows.length > 0);
  if (blocks.length === 0) return null;
  return (
    <section className={styles.section}>
      <SectionHead id="bilancio" title="Bilancio riclassificato" prov="fonte" />
      <div className={styles.bilancioGrid}>
        {blocks.map((b) => (
          <div key={b.title} className={styles.bilancioBlock}>
            <h3 className={styles.bilancioH}>{b.title}</h3>
            <table className={styles.kvTable}>
              <tbody>
                {b.rows.map((r) => (
                  <tr key={r.code}>
                    <td>{r.description}</td>
                    <td className={styles.kvNum}>{eur0.format(r.value)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        ))}
      </div>
    </section>
  );
}

function SociSection({ itf }: { itf: ItFull }) {
  const shareholders = itf.shareholders ?? [];
  const group = itf.corporateGroups;
  const subs = itf.subsidiaries ?? [];
  if (shareholders.length === 0 && !group?.belongsToGroup && subs.length === 0) return null;
  return (
    <section className={styles.section}>
      <SectionHead id="soci" title="Soci e gruppo" prov="fonte" />
      {shareholders.length > 0 ? (
        <div className={styles.subBlock}>
          <h3 className={styles.bilancioH}>Compagine sociale</h3>
          <table className={styles.kvTable}>
            <tbody>
              {shareholders.map((s, i) => {
                const info = s.shareholdersInformation?.[0];
                const who = info?.companyName || [info?.name, info?.surname].filter(Boolean).join(' ') || '—';
                return (
                  <tr key={i}>
                    <td>{who}</td>
                    <td className={styles.kvNum}>{s.percentShare != null ? `${num1.format(s.percentShare)}%` : '—'}</td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      ) : null}
      {group?.belongsToGroup ? (
        <div className={styles.subBlock}>
          <h3 className={styles.bilancioH}>Gruppo</h3>
          <p className={styles.factText}>
            Gruppo {group.groupName ?? '—'}
            {group.holdingCompanyName ? ` · holding ${group.holdingCompanyName}` : ''}
            {group.totalGroupSubsidiaries ? ` · ${group.totalGroupSubsidiaries} controllate` : ''}
            {group.hasForeignSubsidiaries ? ' · presenza estera' : ''}
          </p>
        </div>
      ) : null}
      {subs.length > 0 ? (
        <div className={styles.subBlock}>
          <h3 className={styles.bilancioH}>Controllate</h3>
          <ul className={styles.bullets}>
            {subs.map((s, i) => (
              <li key={i}>
                {s.companyName ?? '—'}
                {s.town ? ` — ${s.town}${s.province?.description ? ` (${s.province.description})` : ''}` : ''}
              </li>
            ))}
          </ul>
        </div>
      ) : null}
    </section>
  );
}

function SediSection({ itf }: { itf: ItFull }) {
  const offices = itf.allOffices ?? [];
  const managers = itf.managers ?? [];
  const tenders = (itf.publicTenders ?? []).filter((t) => (t.value ?? 0) > 0);
  if (offices.length === 0 && managers.length === 0 && tenders.length === 0) return null;
  return (
    <section className={styles.section}>
      <SectionHead id="sedi" title="Sedi, cariche e gare" prov="fonte" />
      {offices.length > 0 ? (
        <div className={styles.subBlock}>
          <h3 className={styles.bilancioH}>Sedi ({offices.length})</h3>
          <ul className={styles.bullets}>
            {offices.map((o, i) => (
              <li key={i}>
                {o.companyDetails?.officeType?.description ?? 'Unità'} — {o.address?.town ?? '—'}
                {o.address?.province?.code ? ` (${o.address.province.code})` : ''}
                {o.companyStatus?.activityStatus?.description ? ` · ${o.companyStatus.activityStatus.description}` : ''}
              </li>
            ))}
          </ul>
        </div>
      ) : null}
      {managers.length > 0 ? (
        <div className={styles.subBlock}>
          <h3 className={styles.bilancioH}>Cariche</h3>
          <ul className={styles.bullets}>
            {managers.map((m, i) => (
              <li key={i}>
                {[m.name, m.surname].filter(Boolean).join(' ')}
                {m.roles?.[0]?.role?.description ? ` — ${m.roles[0].role.description}` : ''}
                {m.age ? ` · ${m.age} anni` : ''}
                {m.isLegalRepresentative ? ' · rappr. legale' : ''}
              </li>
            ))}
          </ul>
        </div>
      ) : null}
      {tenders.length > 0 ? (
        <div className={styles.subBlock}>
          <h3 className={styles.bilancioH}>Gare pubbliche</h3>
          <ul className={styles.bullets}>
            {tenders.map((t, i) => (
              <li key={i}>
                {t.year}: {t.won ?? 0}/{t.applied ?? 0} vinte · {eur0.format(t.value ?? 0)}
              </li>
            ))}
          </ul>
        </div>
      ) : null}
    </section>
  );
}

function AnagraficaSection({ itf }: { itf: ItFull }) {
  const facts: Array<[string, string | undefined]> = [
    ['Forma giuridica', itf.legalForm?.detailedLegalForm?.description ?? itf.legalForm?.legalForm?.description],
    ['ATECO', itf.atecoClassification?.ateco?.description],
    ['Costituzione', year(itf.companyDates?.incorporationDate)],
    ['Capitale sociale', itf.ecofin?.shareCapital != null ? eur0.format(itf.ecofin.shareCapital) : undefined],
    ['Dimensione', itf.ecofin?.enterpriseSize?.description],
    ['Dipendenti', itf.employees?.employee != null ? `${itf.employees.employee}${itf.employees.employeeRange?.description ? ` (${itf.employees.employeeRange.description})` : ''}` : undefined],
    ['Sede', [itf.address?.streetName, itf.address?.town].filter(Boolean).join(', ') || undefined],
    ['PEC', itf.pec],
    ['Telefono', itf.contacts?.telephoneNumber],
    ['Web', itf.webAndSocial?.website],
    ['Bilancio al', itf.ecofin?.balanceSheetDate ? year(itf.ecofin.balanceSheetDate) : undefined],
  ];
  const rows = facts.filter(([, v]) => v && v.trim());
  if (rows.length === 0) return null;
  return (
    <section className={styles.section}>
      <SectionHead id="anagrafica" title="Anagrafica" prov="fonte" />
      <table className={styles.kvTable}>
        <tbody>
          {rows.map(([k, v]) => (
            <tr key={k}>
              <td className={styles.kvKey}>{k}</td>
              <td>{v}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </section>
  );
}

function RawSection({ raw }: { raw: unknown }) {
  const [open, setOpen] = useState(false);
  return (
    <section className={styles.section}>
      <SectionHead id="dati" title="Dati completi" prov="fonte" />
      <button type="button" className={styles.rawToggle} onClick={() => setOpen((o) => !o)}>
        <Icon name={open ? 'chevron-down' : 'chevron-right'} size={16} />
        {open ? 'Nascondi payload IT-full' : 'Mostra payload IT-full grezzo'}
      </button>
      {open ? <pre className={styles.rawPre}>{JSON.stringify(raw, null, 2)}</pre> : null}
    </section>
  );
}

function Dossier({ dossier }: { dossier: MACompanyDossier }) {
  const itf = (dossier.raw ?? {}) as ItFull;
  return (
    <div className={styles.dossier}>
      <Hero dossier={dossier} itf={itf} />
      <div className={styles.dossierLayout}>
        <nav className={styles.rail} aria-label="Sezioni">
          {SECTIONS.map((s) => (
            <a key={s.id} href={`#${s.id}`} className={styles.railLink}>
              {s.label}
            </a>
          ))}
        </nav>
        <div className={styles.body}>
          <BriefSection brief={dossier.brief} />
          <ValuationSection valuation={dossier.valuation} brief={dossier.brief} flags={dossier.scorecard?.qualityFlags} />
          <FamilySection vatCode={dossier.vatCode} bmFamily={dossier.bmFamily} />
          <ScorecardSection scorecard={dossier.scorecard} />
          <BilancioSection itf={itf} />
          <SociSection itf={itf} />
          <SediSection itf={itf} />
          <AnagraficaSection itf={itf} />
          <RawSection raw={dossier.raw} />
        </div>
      </div>
    </div>
  );
}

const ANALYZING_STAGES = ['Anagrafica', 'Bilanci', 'Indicatori', 'Valutazione', 'Brief'];

export function CompanyDossierPage() {
  const api = useApiClient();
  const [vatInput, setVatInput] = useState('');
  const [activeVat, setActiveVat] = useState<string | null>(null);
  const [costPrompt, setCostPrompt] = useState<{ vat: string; costEur?: number } | null>(null);
  const [localError, setLocalError] = useState<string | null>(null);

  const start = useMutation({
    mutationFn: ({ v, ack }: { v: string; ack: boolean }) =>
      api.post<MACompanyDossier>(`/binocolo/v1/companies/${encodeURIComponent(v)}/dossier`, { acknowledgeCost: ack }),
    onSuccess: (data) => {
      if (data.status === 'cost_required') {
        setCostPrompt({ vat: data.vatCode, costEur: data.costEur });
        setActiveVat(null);
      } else {
        setCostPrompt(null);
        setActiveVat(data.vatCode);
      }
    },
  });

  const poll = useQuery({
    queryKey: ['dossier', activeVat],
    enabled: !!activeVat,
    queryFn: () => api.get<MACompanyDossier>(`/binocolo/v1/companies/${encodeURIComponent(activeVat as string)}/dossier`),
    refetchInterval: (query) => {
      const s = query.state.data?.status;
      return s === 'queued' || s === 'running' ? 3000 : false;
    },
  });

  const dossier =
    poll.data ?? (start.data && start.data.status !== 'cost_required' ? start.data : undefined);
  const status = dossier?.status;

  function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const v = vatInput.trim().toUpperCase();
    if (!isValidVatOrTax(v)) {
      setLocalError('Inserisci una partita IVA (11 cifre) o un codice fiscale (16 caratteri).');
      return;
    }
    setLocalError(null);
    setActiveVat(null);
    setCostPrompt(null);
    start.mutate({ v, ack: false });
  }

  const busy = start.isPending || status === 'queued' || status === 'running';

  return (
    <div className={styles.page}>
      <div className={styles.pageHeader}>
        <div>
          <p className={styles.eyebrow}>Dossier</p>
          <h1 className={styles.pageTitle}>Dossier azienda</h1>
          <p className={styles.pageSubtitle}>Partita IVA → profilo completo, indicatori e valutazione.</p>
        </div>
      </div>

      <form className={styles.lookupForm} onSubmit={handleSubmit}>
        <input
          type="text"
          className={styles.vatInput}
          value={vatInput}
          onChange={(e) => setVatInput(e.target.value)}
          placeholder="Partita IVA, es. 01522380193"
          inputMode="numeric"
          autoComplete="off"
          spellCheck={false}
          aria-label="Partita IVA"
          maxLength={16}
        />
        <Button type="submit" loading={start.isPending} leftIcon={<Icon name="search" />}>
          Cerca
        </Button>
      </form>
      {localError ? <p className={styles.localError}>{localError}</p> : null}

      {costPrompt ? (
        <div className={styles.costPanel} role="alert">
          <div className={styles.stateIcon}>
            <Icon name="circle-dollar-sign" size={22} />
          </div>
          <p className={styles.stateTitle}>Prima analisi di {costPrompt.vat}</p>
          <p className={styles.stateText}>
            Questa azienda non è ancora in archivio. Confermi l'acquisizione del fascicolo?
          </p>
          <div className={styles.costActions}>
            <Button variant="secondary" onClick={() => setCostPrompt(null)}>
              Annulla
            </Button>
            <Button loading={start.isPending} onClick={() => start.mutate({ v: costPrompt.vat, ack: true })}>
              Analizza
            </Button>
          </div>
        </div>
      ) : null}

      {start.isError ? (
        <div className={styles.statePanel} role="alert">
          <div className={styles.stateIcon}>
            <Icon name="triangle-alert" size={22} />
          </div>
          <p className={styles.stateTitle}>Ricerca non riuscita</p>
          <p className={styles.stateText}>{errorLabel(start.error)}</p>
        </div>
      ) : null}

      {!costPrompt && busy && status !== 'ready' ? (
        <div className={styles.analyzing}>
          <div className={styles.spinner} aria-hidden="true" />
          <p className={styles.stateTitle}>Analisi in corso…</p>
          <p className={styles.stateText}>Recupero IT-full e calcolo del dossier. Può richiedere qualche minuto.</p>
          <div className={styles.stageList}>
            {ANALYZING_STAGES.map((s) => (
              <span key={s} className={styles.stage}>
                {s}
              </span>
            ))}
          </div>
        </div>
      ) : null}

      {status === 'failed' ? (
        <div className={styles.statePanel} role="alert">
          <div className={styles.stateIcon}>
            <Icon name="triangle-alert" size={22} />
          </div>
          <p className={styles.stateTitle}>Analisi non riuscita</p>
          <p className={styles.stateText}>
            {dossier?.errorCode ? `Errore: ${dossier.errorCode}.` : 'IT-full non ha restituito dati.'}
          </p>
          <Button variant="secondary" onClick={() => activeVat && start.mutate({ v: activeVat, ack: true })}>
            Riprova ({COST_LABEL})
          </Button>
        </div>
      ) : null}

      {status === 'ready' && dossier ? <Dossier dossier={dossier} /> : null}
    </div>
  );
}
