import type {
  MADeepBrief,
  MADeepMetric,
  MADeepQualityFlag,
  MADeepReconciliation,
  MADeepScorecard,
  MADeepValuation,
  MADeepBridgeRow,
} from '../../../../api/types';
import { HiddenField } from '../HiddenField';
import styles from '../Inspector.module.css';
import { formatCompactEuro, ragClass } from '../tabs/format';

const moneyFormat = new Intl.NumberFormat('it-IT', {
  style: 'currency',
  currency: 'EUR',
  maximumFractionDigits: 0,
});

function formatPercentagesInText(text: string): string {
  return text.replace(/(\d+)\.(\d{2,})%/g, (_, p1, p2) => `${parseFloat(`${p1}.${p2}`).toFixed(1)}%`);
}

function formatMetricValue(value: number, unit: string): string {
  const rounded = Math.round(value * 10) / 10;
  if (unit === '%') return `${rounded.toLocaleString('it-IT', { maximumFractionDigits: 1 })}%`;
  if (unit === 'x') return `${rounded.toLocaleString('it-IT', { maximumFractionDigits: 2 })}×`;
  if (unit === 'gg') return `${Math.round(value)} gg`;
  if (unit === '€') return moneyFormat.format(value);
  return String(rounded);
}

function bridgeProvenanceLabel(provenance?: string): string {
  if (provenance === 'cee_total') return ' (da totali di bilancio)';
  if (provenance === 'vendor_ratio') return ' (stimata da ratio)';
  return '';
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

export function DeepScorecard({ scorecard }: { scorecard: MADeepScorecard }) {
  const metrics = scorecard.metrics ?? [];
  const coreMetrics = metrics.filter((m) => m.tier !== 'contorno');
  const contornoMetrics = metrics.filter((m) => m.tier === 'contorno');

  return (
    <div className={styles.card}>
      <div className={styles.linkRow} style={{ justifyContent: 'flex-start' }}>
        <h3 className={styles.cardTitle} style={{ margin: 0 }}>Scorecard finanziaria</h3>
        <span className={`${styles.rag} ${ragClass(scorecard.overallRag)}`}>overall · {scorecard.overallRag}</span>
        <HiddenField label="deep.scorecard.overallRAG" />
      </div>
      <p className={styles.cardSub}>
        Numeri letti dal payload IT-full + semafori deterministici. Metriche "contorno" opacizzate
        (struttura capitale del venditore, non guida il RAG).
      </p>

      {scorecard.turnover != null || scorecard.ebitda != null || scorecard.netWorth != null || scorecard.pfn != null ? (
        <dl className={styles.kv} style={{ marginBottom: 4 }}>
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
                  <span className={styles.metricValue}>{m.value != null ? formatMetricValue(m.value, m.unit) : 'n.d.'}</span>
                  <span className={`${styles.rag} ${ragClass(m.rag)}`} style={{ alignSelf: 'flex-start' }}>{m.rag}</span>
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
              <div key={m.key} className={`${styles.metric} ${styles.metricContorno}`}>
                <span className={styles.metricGroup}>{m.group}</span>
                <span className={styles.metricLabel}>{m.label}</span>
                <span className={styles.metricValue}>{m.value != null ? formatMetricValue(m.value, m.unit) : 'n.d.'}</span>
              </div>
            ))}
          </div>
        </div>
      ) : null}
    </div>
  );
}

// --- Reconciliation ---
export function DeepReconciliation({ reconciliation }: { reconciliation?: MADeepReconciliation }) {
  if (!reconciliation) return null;
  const empty =
    reconciliation.ebitdaPct == null &&
    reconciliation.pfnPct == null &&
    !reconciliation.pfnProvenance &&
    reconciliation.roePointsDiff == null;
  if (empty) return null;

  return (
    <div className={`${styles.card} ${styles.cardCompact}`}>
      <p className={styles.lab}>Reconciliation vendor ↔ CEE <HiddenField label="deep.scorecard.reconciliation" /></p>
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
export function DeepQualityFlags({ flags }: { flags?: MADeepQualityFlag[] }) {
  if (!flags || flags.length === 0) return null;
  return (
    <div className={`${styles.card} ${styles.cardCompact}`}>
      <p className={styles.lab}>Quality flags <HiddenField label="deep.scorecard.qualityFlags[]" /></p>
      <div className={styles.rowsList}>
        {flags.map((f) => (
          <div key={f.code} className={`${styles.rowline} ${f.severity === 'warning' ? styles.rowlineWarn : styles.rowlineNeutral}`}>
            <div>
              <div className={styles.rlMain}>{f.label}</div>
              <div className={styles.rlSub}>{f.evidence}{f.ddQuestion ? ` · DD: ${f.ddQuestion}` : ''}</div>
            </div>
            <span className={`${styles.rlVal} ${f.severity === 'warning' ? styles.flagValWarn : styles.flagValNeutral}`}>{f.severity}</span>
          </div>
        ))}
      </div>
    </div>
  );
}

// --- Valuation + bridge ---
export function DeepValuation({ valuation }: { valuation: MADeepValuation }) {
  const bridge = valuation.bridge;
  const methodLabel = valuation.method === 'ev_sales' ? 'EV/Sales' : 'EV/EBITDA';
  const equityLow = bridge?.equityLow ?? valuation.equityLow;
  const equityHigh = bridge?.equityHigh ?? valuation.equityHigh;

  return (
    <div className={styles.card}>
      <p className={styles.lab}>
        Valuation — Damodaran sector multiples <HiddenField label="deep.valuation" />
      </p>
      <div className={styles.valBlock}>
        <div>
          <div className={styles.valRange}>
            {equityLow != null && equityHigh != null
              ? <>{formatCompactEuro(equityLow)} – {formatCompactEuro(equityHigh)} <small>equity</small></>
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
            {bridge.pfn ? <BridgeRow label={`− PFN${bridgeProvenanceLabel(bridge.pfn.provenance)}`} row={bridge.pfn} neg /> : null}
            {bridge.tfr ? <BridgeRow label={`− TFR${bridge.tfr.note ? ` (${bridge.tfr.note})` : ''}`} row={bridge.tfr} neg /> : null}
            {bridge.taxFund ? <BridgeRow label="− Fondo imposte" row={bridge.taxFund} neg /> : null}
            {bridge.shareholderLoans ? <BridgeRow label="ℹ Finanziamenti soci (già in PFN)" row={bridge.shareholderLoans} /> : null}
            {equityLow != null && equityHigh != null ? (
              <>
                <span className={styles.bridgeK}>= Equity range</span>
                <span className={`${styles.bridgeV} ${styles.bridgeVEq}`}>{formatCompactEuro(equityLow)} – {formatCompactEuro(equityHigh)}</span>
              </>
            ) : null}
          </div>
        ) : null}
      </div>
      {valuation.caveat ? <p className={styles.cardSub} style={{ margin: '10px 0 0' }}>{valuation.caveat}</p> : null}
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
export function DeepBrief({ brief }: { brief: MADeepBrief }) {
  const hasAnything =
    brief.verdict ||
    brief.businessProfile ||
    brief.financialReading ||
    (brief.strengths && brief.strengths.length) ||
    (brief.redFlags && brief.redFlags.length) ||
    brief.valuationRationale ||
    (brief.ddQuestions && brief.ddQuestions.length);
  if (!hasAnything) return null;

  return (
    <div className={styles.card}>
      <p className={styles.lab}>Brief LLM <HiddenField label="deep.brief" /></p>
      <div className={styles.tabBody} style={{ gap: 'var(--space-4)' }}>
        {brief.verdict ? (
          <div>
            <div className={styles.lab} style={{ margin: '0 0 4px' }}>Verdict</div>
            <p className={styles.briefPara}>{formatPercentagesInText(brief.verdict)}</p>
          </div>
        ) : null}
        {brief.businessProfile ? (
          <div>
            <div className={styles.lab} style={{ margin: '0 0 4px' }}>Business profile</div>
            <p className={styles.briefPara}>{brief.businessProfile}</p>
          </div>
        ) : null}
        {brief.financialReading ? (
          <div>
            <div className={styles.lab} style={{ margin: '0 0 4px' }}>Lettura finanziaria</div>
            <p className={styles.briefPara}>{brief.financialReading}</p>
          </div>
        ) : null}
        {brief.valuationRationale ? (
          <div>
            <div className={styles.lab} style={{ margin: '0 0 4px' }}>Valuation rationale</div>
            <p className={styles.briefPara}>{brief.valuationRationale}</p>
          </div>
        ) : null}
        {(brief.strengths && brief.strengths.length) || (brief.redFlags && brief.redFlags.length) ? (
          <div className={styles.grid2} style={{ gap: 'var(--space-4)' }}>
            {brief.strengths && brief.strengths.length ? (
              <div>
                <div className={styles.lab} style={{ margin: '0 0 6px' }}>Punti di forza</div>
                <ul className={styles.briefList}>{brief.strengths.map((s, i) => <li key={i}>{s}</li>)}</ul>
              </div>
            ) : null}
            {brief.redFlags && brief.redFlags.length ? (
              <div>
                <div className={styles.lab} style={{ margin: '0 0 6px' }}>Red flags</div>
                <ul className={`${styles.briefList} ${styles.briefListRed}`}>
                  {brief.redFlags.map((f, i) => (
                    <li key={i}>
                      {f.claim}{f.ddQuestion ? <><br /><small className={styles.muted}>DD: {f.ddQuestion}</small></> : null}
                    </li>
                  ))}
                </ul>
              </div>
            ) : null}
          </div>
        ) : null}
        {brief.ddQuestions && brief.ddQuestions.length ? (
          <div>
            <div className={styles.lab} style={{ margin: '0 0 6px' }}>DD questions</div>
            <ul className={styles.ddq}>{brief.ddQuestions.map((q, i) => <li key={i}>{q}</li>)}</ul>
          </div>
        ) : null}
      </div>
    </div>
  );
}
