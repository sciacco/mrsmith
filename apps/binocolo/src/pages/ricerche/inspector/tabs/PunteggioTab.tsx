import type {
  MATarget,
  MATargetAdjustment,
  MATargetEvidence,
  MATargetFlag,
} from '../../../../api/types';
import { HiddenField } from '../HiddenField';
import styles from '../Inspector.module.css';

type EvStatus = MATargetEvidence['status'];

function statusChipClass(status: EvStatus): string {
  switch (status) {
    case 'match': return styles.stmatchM ?? '';
    case 'match_parziale': return styles.stmatchP ?? '';
    case 'fuori_criterio': return styles.stmatchX ?? '';
    case 'criterio_mancante': return styles.stmatchO ?? '';
    default: return styles.stmatchO ?? '';
  }
}

function statusLabel(status: EvStatus): string {
  switch (status) {
    case 'match': return 'match';
    case 'match_parziale': return 'parziale';
    case 'fuori_criterio': return 'fuori';
    case 'criterio_mancante': return 'mancante';
    default: return status;
  }
}

function formatPoints(n?: number): string {
  if (n == null) return '—';
  return n.toLocaleString('it-IT', { minimumFractionDigits: 0, maximumFractionDigits: 1 });
}

function formatFactor(f: number): string {
  return `× ${f.toLocaleString('it-IT', { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`;
}

export function PunteggioTab({ target }: { target: MATarget }) {
  const evidence = target.evidence ?? [];
  const adjustments = target.adjustments ?? [];
  const flags = target.flags ?? [];
  const missing = target.missingCriteria ?? [];

  const sumPoints = evidence.reduce((acc, e) => acc + (e.points ?? 0), 0);
  const productFactor = adjustments.reduce((acc, a) => acc * a.factor, 1);
  const reconstructed = Math.round(sumPoints * productFactor);
  const delta = reconstructed - target.score;
  const deltaOk = Math.abs(delta) <= 1; // tolleranza 1 punto (arrotondamenti)

  return (
    <div className={styles.tabBody}>
      {/* evidence table */}
      <div className={styles.card}>
        <h3 className={styles.cardTitle}>Composizione del punteggio</h3>
        <p className={styles.cardSub}>
          {evidence.length} criteri attivi, poi {adjustments.length} fattore moltiplicativi.
        </p>
        {evidence.length === 0 ? (
          <p className={styles.muted}>Nessuna evidence registrata per questo target.</p>
        ) : (
          <table className={styles.ev}>
            <thead>
              <tr>
                <th>Criterion</th>
                <th>Stato</th>
                <th>Value</th>
                <th className={styles.num}>Points</th>
                <th className={styles.num}>Weight</th>
                <th>Source</th>
              </tr>
            </thead>
            <tbody>
              {evidence.map((e, i) => (
                <tr key={`${e.criterion}-${e.label}-${i}`}>
                  <td className={styles.label}>{e.label || e.criterion}</td>
                  <td>
                    <span className={`${styles.stmatch} ${statusChipClass(e.status)}`}>{statusLabel(e.status)}</span>
                  </td>
                  <td className={styles.mono}>{e.value ?? '—'}</td>
                  <td className={styles.num}>{formatPoints(e.points)}</td>
                  <td className={styles.num}>{formatPoints(e.weight)}</td>
                  <td><span className={styles.srcpath}>{e.sourcePath ?? '—'}</span></td>
                </tr>
              ))}
            </tbody>
            <tfoot>
              <tr>
                <td colSpan={3}>Σ punti additivi</td>
                <td className={styles.num}>{formatPoints(sumPoints)}</td>
                <td className={styles.num} style={{ color: 'var(--color-text-muted)' }}>/ 100</td>
                <td></td>
              </tr>
            </tfoot>
          </table>
        )}
      </div>

      <div className={styles.grid2}>
        {/* adjustments */}
        <div className={styles.card}>
          <p className={styles.lab}>
            Adjustments <HiddenField label="adjustments[]" />
          </p>
          {adjustments.length === 0 ? (
            <p className={styles.cardSub} style={{ margin: 0 }}>Nessun fattore applicato.</p>
          ) : (
            <div className={styles.rowsList}>
              {adjustments.map((a: MATargetAdjustment) => (
                <div
                  key={a.code}
                  className={`${styles.rowline} ${a.factor < 1 ? styles.rowlineWarn : styles.rowlineNeutral}`}
                >
                  <div>
                    <div className={styles.rlMain}>{a.label || a.code}</div>
                    <div className={styles.rlSub}>Fattore applicato allo score.</div>
                  </div>
                  <span className={styles.rlVal}>{formatFactor(a.factor)}</span>
                </div>
              ))}
            </div>
          )}
          {evidence.length > 0 ? (
            <div className={styles.recon}>
              {formatPoints(sumPoints)}
              {adjustments.map((a) => ` × ${a.factor.toLocaleString('it-IT', { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`).join('') || ' × 1,00'}
              {' = '}
              <b>{reconstructed}</b>
              <span className={`${styles.reconDelta} ${deltaOk ? styles.reconDeltaOk : styles.reconDeltaWarn}`}>
                {deltaOk ? `✓ coincide con score ${target.score}` : `Δ vs score ${target.score}: ${delta > 0 ? '+' : ''}${delta}`}
              </span>
            </div>
          ) : null}
        </div>

        {/* flags */}
        <div className={styles.card}>
          <p className={styles.lab}>
            Flags non-scoring <HiddenField label="flags[]" />
          </p>
          {flags.length === 0 ? (
            <p className={styles.cardSub} style={{ margin: 0 }}>Nessun flag.</p>
          ) : (
            <div className={styles.rowsList}>
              {flags.map((f: MATargetFlag) => (
                <div
                  key={f.code}
                  className={`${styles.rowline} ${f.severity === 'warning' ? styles.rowlineWarn : styles.rowlineNeutral}`}
                >
                  <div>
                    <div className={styles.rlMain}>{f.label}</div>
                    <div className={styles.rlSub}>{f.code}</div>
                  </div>
                  <span
                    className={`${styles.rlVal} ${f.severity === 'warning' ? styles.flagValWarn : styles.flagValNeutral}`}
                  >
                    {f.severity}
                  </span>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>

      {/* missing criteria */}
      <div className={styles.card}>
        <p className={styles.lab}>
          Criteri mancanti <HiddenField label="missingCriteria[]" />
        </p>
        {missing.length === 0 ? (
          <p className={styles.cardSub} style={{ margin: 0 }}>Nessun criterio mancante.</p>
        ) : (
          <div className={styles.missing}>
            {missing.map((m, i) => (
              <span key={i} className={styles.mchip}>{m}</span>
            ))}
          </div>
        )}
        <p className={styles.cardSub} style={{ margin: '10px 0 0' }}>
          Cosa le manca per un match pieno. Seme naturale di domande DD.
        </p>
      </div>
    </div>
  );
}
