import { type CSSProperties } from 'react';
import { MACROFASI, CARD_STATES, stateVars } from '../../../lib/cardStates';
import styles from './board.module.css';

/** Strip funnel: chip per stato raggruppati per macrofase, SEMPRE tutti gli stati
 *  anche a 0 (posizioni stabili). Il chip è un filtro (toggle). L'Esito compare
 *  solo quando `showEsito` (scope "Vista completa"). Sigla + label piena in tooltip. */
export function FunnelStrip({
  counts,
  activeState,
  onToggle,
  showEsito,
}: {
  counts: Record<string, number>;
  activeState: string | null;
  onToggle: (state: string) => void;
  showEsito: boolean;
}) {
  const groups = MACROFASI.filter((m) => showEsito || m.key !== 'esito');
  return (
    <div className={styles.funnel} role="group" aria-label="Funnel per stato">
      {groups.map((macro) => (
        <div key={macro.key} className={styles.fgroup}>
          <span className={styles.glabel}>
            {macro.num} · {macro.label}
          </span>
          <div className={styles.grow}>
            {macro.states.map((sk) => {
              const meta = CARD_STATES.find((s) => s.key === sk);
              if (!meta) return null;
              const n = counts[sk] ?? 0;
              const on = activeState === sk;
              return (
                <button
                  key={sk}
                  type="button"
                  className={[styles.fchip, n === 0 ? styles.fzero : '', on ? styles.fon : ''].filter(Boolean).join(' ')}
                  style={stateVars(sk) as CSSProperties}
                  onClick={() => onToggle(sk)}
                  aria-pressed={on}
                  title={meta.label}
                >
                  <span className={styles.dot} />
                  <span className={styles.fn}>{meta.abbr}</span>
                  <span className={styles.fc}>{n}</span>
                </button>
              );
            })}
          </div>
        </div>
      ))}
    </div>
  );
}
