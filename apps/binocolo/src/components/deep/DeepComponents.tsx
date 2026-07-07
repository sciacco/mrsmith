import { Tooltip } from '@mrsmith/ui';
import type {
  MADeepAnalysis,
  MADeepBrief,
  MADeepMetric,
  MADeepQualityFlag,
  MADeepReconciliation,
  MADeepScorecard,
  MADeepValuation,
  MADeepBridgeRow,
} from '../../api/types';
import styles from './DeepComponents.module.css';

export type DeepRenderVariant = 'full' | 'compact' | 'summary';

const moneyFormat = new Intl.NumberFormat('it-IT', {
  style: 'currency',
  currency: 'EUR',
  maximumFractionDigits: 0,
});

const numberFormat = new Intl.NumberFormat('it-IT');

export function formatDeepCompactEuro(value?: number): string {
  if (value == null) return '—';
  if (value >= 1_000_000) return `${(value / 1_000_000).toLocaleString('it-IT', { maximumFractionDigits: 1 })} M€`;
  if (value >= 1_000) return `${Math.round(value / 1000)} k€`;
  return `${numberFormat.format(value)} €`;
}

export function formatPercentagesInText(text: string): string {
  return text.replace(/(\d+)\.(\d{2,})%/g, (_, p1, p2) => `${parseFloat(`${p1}.${p2}`).toFixed(1)}%`);
}

export function formatDeepMetricValue(value: number, unit: string): string {
  const rounded = Math.round(value * 10) / 10;
  if (unit === '%') return `${rounded.toLocaleString('it-IT', { maximumFractionDigits: 1 })}%`;
  if (unit === 'x') return `${rounded.toLocaleString('it-IT', { maximumFractionDigits: 2 })}×`;
  if (unit === 'gg') return `${Math.round(value)} gg`;
  if (unit === '€') return moneyFormat.format(value);
  return String(rounded);
}

export function deepBridgeProvenanceLabel(provenance?: string): string {
  if (provenance === 'cee_total') return ' (da totali di bilancio)';
  if (provenance === 'vendor_ratio') return ' (stimata da ratio)';
  return '';
}

export function deepRagClassName(rag?: string): string {
  switch ((rag ?? '').toLowerCase()) {
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

function cx(...classes: Array<string | false | null | undefined>): string {
  return classes.filter(Boolean).join(' ');
}

function HiddenField({ label, enabled }: { label: string; enabled?: boolean }) {
  if (!enabled) return null;
  return (
    <Tooltip content={`${label} — oggi non visibile all'analista`} placement="top" showDelay={150}>
      <span className="ti-hide" role="img" aria-label={`${label} — oggi non visibile all'analista`} />
    </Tooltip>
  );
}

// --- High-level renderer ---
export function DeepAnalysisContent({
  deep,
  variant = 'full',
  showHiddenFields = false,
  className,
}: {
  deep?: MADeepAnalysis;
  variant?: DeepRenderVariant;
  showHiddenFields?: boolean;
  className?: string;
}) {
  if (!deep || deep.status !== 'ready') return null;
  if (variant === 'summary') return <DeepSummary deep={deep} className={className} />;

  const scorecard = deep.scorecard;
  const valuation = deep.valuation;
  const brief = deep.brief;
  const hasContent = Boolean(scorecard || valuation || brief);

  return (
    <div className={cx(styles.root, variant === 'compact' && styles.rootCompact, className)}>
      {scorecard ? <DeepScorecard scorecard={scorecard} showHiddenFields={showHiddenFields} /> : null}
      <div className={styles.grid2}>
        <DeepReconciliation reconciliation={scorecard?.reconciliation} showHiddenFields={showHiddenFields} />
        <DeepQualityFlags flags={scorecard?.qualityFlags} showHiddenFields={showHiddenFields} />
      </div>
      {valuation ? <DeepValuation valuation={valuation} showHiddenFields={showHiddenFields} /> : null}
      {brief ? <DeepBrief brief={brief} showHiddenFields={showHiddenFields} /> : null}
      {!hasContent ? <div className={styles.fallback}>Analisi disponibile, ma senza dati finanziari strutturati.</div> : null}
    </div>
  );
}

function DeepSummary({ className }: { deep: MADeepAnalysis; className?: string }) {
  return (
    <div className={cx(styles.summaryRoot, className)}>
      <div className={styles.fallback}>Analisi disponibile. Apri l'ispezione completa per il dettaglio tecnico.</div>
    </div>
  );
}

// --- Scorecard (metric cards + overall RAG) ---
const METRIC_GROUPS: { key: string; label: string }[] = [
  { key: 'redditivita', label: 'Redditività' },
  { key: 'leva', label: 'Leva e struttura' },
  { key: 'liquidita', label: 'Liquidità' },
  { key: 'efficienza', label: 'Efficienza' },
  { key: 'crescita', label: 'Crescita' },
  { key: 'qualita_margine', label: 'Qualità del margine' },
];

export function DeepScorecard({ scorecard, showHiddenFields = false }: { scorecard: MADeepScorecard; showHiddenFields?: boolean }) {
  const metrics = scorecard.metrics ?? [];
  const coreMetrics = metrics.filter((m) => m.tier !== 'contorno');
  const contornoMetrics = metrics.filter((m) => m.tier === 'contorno');

  return (
    <div className={styles.card}>
      <div className={styles.linkRow}>
        <h3 className={cx(styles.cardTitle, styles.cardTitleInline)}>Scorecard finanziaria</h3>
        <span className={cx(styles.rag, deepRagClassName(scorecard.overallRag))}>overall · {scorecard.overallRag}</span>
        <HiddenField label="deep.scorecard.overallRAG" enabled={showHiddenFields} />
      </div>
      <p className={styles.cardSub}>
        Numeri letti dal payload IT-full + semafori deterministici. Metriche "contorno" opacizzate
        (struttura capitale del venditore, non guida il RAG).
      </p>

      {scorecard.turnover != null || scorecard.ebitda != null || scorecard.netWorth != null || scorecard.pfn != null ? (
        <dl className={cx(styles.kv, styles.kvTight)}>
          {scorecard.turnover != null ? (
            <><dt>Fatturato{scorecard.turnoverYear ? ` (${scorecard.turnoverYear})` : ''}</dt><dd>{moneyFormat.format(scorecard.turnover)}</dd></>
          ) : null}
          {scorecard.ebitda != null ? <><dt>EBITDA</dt><dd>{moneyFormat.format(scorecard.ebitda)}</dd></> : null}
          {scorecard.netWorth != null ? <><dt>Patrimonio netto</dt><dd>{moneyFormat.format(scorecard.netWorth)}</dd></> : null}
          {scorecard.pfn != null ? <><dt>PFN</dt><dd>{moneyFormat.format(scorecard.pfn)}</dd></> : null}
        </dl>
      ) : null}

      {METRIC_GROUPS.map((group) => {
        const items = coreMetrics.filter((m: MADeepMetric) => m.group === group.key);
        if (items.length === 0) return null;
        return (
          <div key={group.key}>
            <div className={styles.metricGroupLabel}>{group.label}</div>
            <div className={styles.metrics}>
              {items.map((m) => (
                <div key={m.key} className={styles.metric}>
                  <span className={styles.metricGroup}>{m.group}</span>
                  <span className={styles.metricLabel}>{m.label}</span>
                  <span className={styles.metricValue}>{m.value != null ? formatDeepMetricValue(m.value, m.unit) : 'n.d.'}</span>
                  <span className={cx(styles.rag, deepRagClassName(m.rag))}>{m.rag}</span>
                </div>
              ))}
            </div>
          </div>
        );
      })}

      {contornoMetrics.length > 0 ? (
        <div>
          <div className={styles.metricGroupLabel}>Struttura finanziaria del venditore</div>
          <div className={styles.metrics}>
            {contornoMetrics.map((m) => (
              <div key={m.key} className={cx(styles.metric, styles.metricContorno)}>
                <span className={styles.metricGroup}>{m.group}</span>
                <span className={styles.metricLabel}>{m.label}</span>
                <span className={styles.metricValue}>{m.value != null ? formatDeepMetricValue(m.value, m.unit) : 'n.d.'}</span>
              </div>
            ))}
          </div>
        </div>
      ) : null}
    </div>
  );
}

// --- Reconciliation ---
export function DeepReconciliation({
  reconciliation,
  showHiddenFields = false,
}: {
  reconciliation?: MADeepReconciliation;
  showHiddenFields?: boolean;
}) {
  if (!reconciliation) return null;
  const empty =
    reconciliation.ebitdaPct == null &&
    reconciliation.pfnPct == null &&
    !reconciliation.pfnProvenance &&
    reconciliation.roePointsDiff == null;
  if (empty) return null;

  return (
    <div className={cx(styles.card, styles.cardCompact)}>
      <p className={styles.lab}>Reconciliation vendor ↔ CEE <HiddenField label="deep.scorecard.reconciliation" enabled={showHiddenFields} /></p>
      <dl className={styles.kv}>
        {reconciliation.ebitdaPct != null ? <><dt>EBITDA Δ</dt><dd>{reconciliation.ebitdaPct.toLocaleString('it-IT', { maximumFractionDigits: 1 })}%</dd></> : null}
        {reconciliation.pfnPct != null ? <><dt>PFN Δ</dt><dd>{reconciliation.pfnPct.toLocaleString('it-IT', { maximumFractionDigits: 1 })}%</dd></> : null}
        {reconciliation.pfnProvenance ? <><dt>PFN provenance</dt><dd className={styles.mono}>{reconciliation.pfnProvenance}</dd></> : null}
        {reconciliation.roePointsDiff != null ? <><dt>ROE Δ</dt><dd>{reconciliation.roePointsDiff.toLocaleString('it-IT', { maximumFractionDigits: 1 })} pp</dd></> : null}
      </dl>
    </div>
  );
}

// --- Quality flags ---
export function DeepQualityFlags({ flags, showHiddenFields = false }: { flags?: MADeepQualityFlag[]; showHiddenFields?: boolean }) {
  if (!flags || flags.length === 0) return null;
  return (
    <div className={cx(styles.card, styles.cardCompact)}>
      <p className={styles.lab}>Quality flags <HiddenField label="deep.scorecard.qualityFlags[]" enabled={showHiddenFields} /></p>
      <div className={styles.rowsList}>
        {flags.map((f) => (
          <div key={f.code} className={cx(styles.rowline, f.severity === 'warning' ? styles.rowlineWarn : styles.rowlineNeutral)}>
            <div>
              <div className={styles.rlMain}>{f.label}</div>
              <div className={styles.rlSub}>{f.evidence}{f.ddQuestion ? ` · DD: ${f.ddQuestion}` : ''}</div>
            </div>
            <span className={cx(styles.rlVal, f.severity === 'warning' ? styles.flagValWarn : styles.flagValNeutral)}>{f.severity}</span>
          </div>
        ))}
      </div>
    </div>
  );
}

// --- Valuation + bridge ---
export function DeepValuation({ valuation, showHiddenFields = false }: { valuation: MADeepValuation; showHiddenFields?: boolean }) {
  const bridge = valuation.bridge;
  const methodLabel = valuation.method === 'ev_sales' ? 'EV/Sales' : 'EV/EBITDA';
  const equityLow = bridge?.equityLow ?? valuation.equityLow;
  const equityHigh = bridge?.equityHigh ?? valuation.equityHigh;

  return (
    <div className={styles.card}>
      <p className={styles.lab}>
        Valuation — Damodaran sector multiples <HiddenField label="deep.valuation" enabled={showHiddenFields} />
      </p>
      <div className={styles.valBlock}>
        <div>
          <div className={styles.valRange}>
            {equityLow != null && equityHigh != null
              ? <>{formatDeepCompactEuro(equityLow)} – {formatDeepCompactEuro(equityHigh)} <small>equity</small></>
              : <>{moneyFormat.format(valuation.evLow)} – {moneyFormat.format(valuation.evHigh)} <small>EV</small></>}
          </div>
          <div className={styles.valMeta}>
            EV {moneyFormat.format(valuation.evLow)} – {moneyFormat.format(valuation.evHigh)} · {methodLabel} {valuation.multiple}×
            {valuation.sector ? ` · ${valuation.sector}` : ''}
            {valuation.nFirms ? ` · n.firms ${valuation.nFirms}` : ''}
            {valuation.source ? ` · ${valuation.source}` : ''}
            {valuation.sourceDate ? ` ${valuation.sourceDate}` : ''}
          </div>
        </div>
        {bridge ? (
          <div className={styles.bridge}>
            <span className={styles.bridgeK}>EV low{valuation.lowMethod === 'ev_sales' ? ' (prudenziale)' : ''}</span>
            <span className={styles.bridgeV}>{moneyFormat.format(valuation.evLow)}</span>
            <span className={styles.bridgeK}>EV high</span>
            <span className={styles.bridgeV}>{moneyFormat.format(valuation.evHigh)}</span>
            {bridge.pfn ? <BridgeRow label={`− PFN${deepBridgeProvenanceLabel(bridge.pfn.provenance)}`} row={bridge.pfn} neg /> : null}
            {bridge.tfr ? <BridgeRow label={`− TFR${bridge.tfr.note ? ` (${bridge.tfr.note})` : ''}`} row={bridge.tfr} neg /> : null}
            {bridge.taxFund ? <BridgeRow label="− Fondo imposte" row={bridge.taxFund} neg /> : null}
            {bridge.shareholderLoans ? <BridgeRow label="ℹ Finanziamenti soci (già in PFN)" row={bridge.shareholderLoans} /> : null}
            {equityLow != null && equityHigh != null ? (
              <>
                <span className={styles.bridgeK}>= Equity range</span>
                <span className={cx(styles.bridgeV, styles.bridgeVEq)}>{formatDeepCompactEuro(equityLow)} – {formatDeepCompactEuro(equityHigh)}</span>
              </>
            ) : null}
          </div>
        ) : null}
      </div>
      {valuation.caveat ? <p className={cx(styles.cardSub, styles.cardSubTight)}>{valuation.caveat}</p> : null}
    </div>
  );
}

function BridgeRow({ label, row, neg }: { label: string; row: MADeepBridgeRow; neg?: boolean }) {
  return (
    <>
      <span className={styles.bridgeK}>{label}</span>
      <span className={styles.bridgeV}>{neg ? '−' : ''}{moneyFormat.format(row.value)}</span>
    </>
  );
}

// --- Brief ---
export function DeepBrief({ brief, showHiddenFields = false }: { brief: MADeepBrief; showHiddenFields?: boolean }) {
  const financialReading = brief.financialReading ?? brief.thesisReading;
  const hasAnything =
    brief.verdict ||
    brief.businessProfile ||
    financialReading ||
    (brief.strengths && brief.strengths.length) ||
    (brief.redFlags && brief.redFlags.length) ||
    brief.valuationRationale ||
    (brief.ddQuestions && brief.ddQuestions.length);
  if (!hasAnything) return null;

  return (
    <div className={styles.card}>
      <p className={styles.lab}>Brief LLM <HiddenField label="deep.brief" enabled={showHiddenFields} /></p>
      <div className={styles.briefBody}>
        {brief.verdict ? (
          <div>
            <div className={cx(styles.lab, styles.labTight)}>Verdict</div>
            <p className={styles.briefPara}>{formatPercentagesInText(brief.verdict)}</p>
          </div>
        ) : null}
        {brief.businessProfile ? (
          <div>
            <div className={cx(styles.lab, styles.labTight)}>Business profile</div>
            <p className={styles.briefPara}>{formatPercentagesInText(brief.businessProfile)}</p>
          </div>
        ) : null}
        {financialReading ? (
          <div>
            <div className={cx(styles.lab, styles.labTight)}>Lettura finanziaria</div>
            <p className={styles.briefPara}>{formatPercentagesInText(financialReading)}</p>
          </div>
        ) : null}
        {brief.valuationRationale ? (
          <div>
            <div className={cx(styles.lab, styles.labTight)}>Valuation rationale</div>
            <p className={styles.briefPara}>{formatPercentagesInText(brief.valuationRationale)}</p>
          </div>
        ) : null}
        {(brief.strengths && brief.strengths.length) || (brief.redFlags && brief.redFlags.length) ? (
          <div className={styles.grid2}>
            {brief.strengths && brief.strengths.length ? (
              <div>
                <div className={cx(styles.lab, styles.labTight)}>Punti di forza</div>
                <ul className={styles.briefList}>{brief.strengths.map((s, i) => <li key={i}>{formatPercentagesInText(s)}</li>)}</ul>
              </div>
            ) : null}
            {brief.redFlags && brief.redFlags.length ? (
              <div>
                <div className={cx(styles.lab, styles.labTight)}>Red flags</div>
                <ul className={cx(styles.briefList, styles.briefListRed)}>
                  {brief.redFlags.map((f, i) => (
                    <li key={i}>
                      {formatPercentagesInText(f.claim)}{f.ddQuestion ? <><br /><small className={styles.muted}>DD: {formatPercentagesInText(f.ddQuestion)}</small></> : null}
                    </li>
                  ))}
                </ul>
              </div>
            ) : null}
          </div>
        ) : null}
        {brief.ddQuestions && brief.ddQuestions.length ? (
          <div>
            <div className={cx(styles.lab, styles.labTight)}>DD questions</div>
            <ul className={styles.ddq}>{brief.ddQuestions.map((q, i) => <li key={i}>{formatPercentagesInText(q)}</li>)}</ul>
          </div>
        ) : null}
      </div>
    </div>
  );
}
