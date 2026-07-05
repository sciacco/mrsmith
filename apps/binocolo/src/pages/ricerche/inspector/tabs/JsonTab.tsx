import { useMemo, useState } from 'react';
import type { MATarget } from '../../../../api/types';
import styles from '../Inspector.module.css';

// Filtra ricorsivamente i campi null/undefined/vuoti (stringa vuota, array vuoto,
// oggetto vuoto). Restituisce una copia "solo non-null" del payload.
function stripEmpty<T>(value: T): T | undefined {
  if (value === null || value === undefined) return undefined;
  if (typeof value === 'string') return value === '' ? undefined : value;
  if (Array.isArray(value)) {
    const mapped = value.map(stripEmpty).filter((v) => v !== undefined);
    return mapped.length === 0 ? undefined : (mapped as unknown as T);
  }
  if (typeof value === 'object') {
    const out: Record<string, unknown> = {};
    let hasAny = false;
    for (const [k, v] of Object.entries(value as Record<string, unknown>)) {
      const cleaned = stripEmpty(v);
      if (cleaned !== undefined) {
        out[k] = cleaned;
        hasAny = true;
      }
    }
    return hasAny ? (out as unknown as T) : undefined;
  }
  return value;
}

function formatBytes(n: number): string {
  if (n < 1024) return `~ ${n} B`;
  return `~ ${(n / 1024).toFixed(1)} KB`;
}

export function JsonTab({ target }: { target: MATarget }) {
  const [nonNullOnly, setNonNullOnly] = useState(false);
  const [copied, setCopied] = useState(false);

  const payload = useMemo(() => (nonNullOnly ? stripEmpty(target) ?? {} : target), [target, nonNullOnly]);
  const jsonText = useMemo(() => JSON.stringify(payload, null, 2), [payload]);
  const size = useMemo(() => new Blob([jsonText]).size, [jsonText]);

  async function copy() {
    try {
      await navigator.clipboard.writeText(jsonText);
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    } catch {
      // fallback no-op (clipboard non disponibile, es. contesto non sicuro)
    }
  }

  return (
    <div className={styles.tabBody}>
      <div className={styles.card}>
        <div className={styles.jsonToggle}>
          <button
            type="button"
            className={`${styles.jsonBtn} ${!nonNullOnly ? styles.jsonBtnOn : ''}`}
            onClick={() => setNonNullOnly(false)}
          >
            MATarget
          </button>
          <button
            type="button"
            className={`${styles.jsonBtn} ${nonNullOnly ? styles.jsonBtnOn : ''}`}
            onClick={() => setNonNullOnly(true)}
          >
            solo non-null
          </button>
          <button type="button" className={styles.jsonBtn} onClick={() => void copy()}>
            {copied ? '✓ copiato' : 'copia'}
          </button>
          <span className={styles.jsonSize}>{formatBytes(size)}</span>
        </div>
        <pre className={styles.jsonBlock}>{jsonText}</pre>
      </div>
    </div>
  );
}
