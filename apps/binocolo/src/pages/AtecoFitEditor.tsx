import { useEffect, useMemo, useState } from 'react';
import { Icon } from '@mrsmith/ui';
import { useApiClient } from '../api/client';
import type { MAAtecoCandidate, MAAtecoFit, MAAtecoSearchItem, MAAtecoSearchResponse } from '../api/types';
import styles from './AtecoFitEditor.module.css';

const TIERS: { value: MAAtecoFit; label: string }[] = [
  { value: 'core', label: 'Core' },
  { value: 'weak', label: 'Adiacente' },
  { value: 'excluded', label: 'Escluso' },
];
const FIT_ORDER: Record<MAAtecoFit, number> = { core: 0, weak: 1, excluded: 2 };

function fitOf(candidate: MAAtecoCandidate): MAAtecoFit {
  return candidate.fit ?? 'core';
}

function dedupeKey(code: string): string {
  return code.replace(/\./g, '').toUpperCase();
}

// AtecoFitEditor is the strategy editor's ATECO control: one row per code with its
// description always visible and an inline 3-way tier (core/adiacente/escluso),
// plus a catalog typeahead to add codes. It edits the atecoCandidates array
// directly (no text round-trip), so descriptions and rationale survive every edit.
export function AtecoFitEditor({
  value,
  onChange,
  disabled,
}: {
  value: MAAtecoCandidate[];
  onChange: (next: MAAtecoCandidate[]) => void;
  disabled?: boolean;
}) {
  const api = useApiClient();
  const [query, setQuery] = useState('');
  const [results, setResults] = useState<MAAtecoSearchItem[]>([]);
  const [open, setOpen] = useState(false);

  useEffect(() => {
    const q = query.trim();
    if (q.length < 2) {
      setResults([]);
      return;
    }
    let active = true;
    const timer = setTimeout(() => {
      api
        .get<MAAtecoSearchResponse>(`/binocolo/v1/ma/ateco/search?q=${encodeURIComponent(q)}&limit=8`)
        .then((data) => {
          if (active) {
            setResults(data.items ?? []);
            setOpen(true);
          }
        })
        .catch(() => {
          if (active) setResults([]);
        });
    }, 250);
    return () => {
      active = false;
      clearTimeout(timer);
    };
  }, [query, api]);

  const existing = useMemo(() => new Set(value.map((item) => dedupeKey(item.code))), [value]);
  const rows = useMemo(
    () => [...value].sort((a, b) => FIT_ORDER[fitOf(a)] - FIT_ORDER[fitOf(b)] || a.code.localeCompare(b.code)),
    [value],
  );
  const counts = useMemo(() => {
    const tally: Record<MAAtecoFit, number> = { core: 0, weak: 0, excluded: 0 };
    for (const item of value) tally[fitOf(item)] += 1;
    return tally;
  }, [value]);

  function addItem(item: MAAtecoSearchItem) {
    if (existing.has(dedupeKey(item.code))) return;
    onChange([...value, { code: item.code, description: item.description, rationale: '', fit: 'core' }]);
    setQuery('');
    setResults([]);
    setOpen(false);
  }

  function setFit(code: string, fit: MAAtecoFit) {
    onChange(value.map((item) => (item.code === code ? { ...item, fit } : item)));
  }

  function remove(code: string) {
    onChange(value.filter((item) => item.code !== code));
  }

  return (
    <div className={styles.editor}>
      <div className={styles.header}>
        <span className={styles.title}>Codici ATECO &amp; rilevanza</span>
        <span className={styles.counts}>
          {value.length} codici · {counts.core} core · {counts.weak} adiacenti · {counts.excluded} esclusi
        </span>
      </div>

      <div className={styles.searchWrap}>
        <Icon name="search" size={14} />
        <input
          className={styles.search}
          value={query}
          placeholder="Cerca per codice o descrizione…"
          disabled={disabled}
          onChange={(event) => setQuery(event.target.value)}
          onFocus={() => results.length > 0 && setOpen(true)}
          onBlur={() => window.setTimeout(() => setOpen(false), 150)}
        />
        {open && results.length > 0 ? (
          <ul className={styles.dropdown}>
            {results.map((item) => {
              const added = existing.has(dedupeKey(item.code));
              return (
                <li key={item.code}>
                  <button
                    type="button"
                    className={styles.option}
                    disabled={added}
                    onMouseDown={(event) => {
                      event.preventDefault();
                      addItem(item);
                    }}
                  >
                    <span className={styles.optCode}>{item.code}</span>
                    <span className={styles.optDesc}>{item.description}</span>
                    {added ? <span className={styles.optAdded}>già nel perimetro</span> : null}
                  </button>
                </li>
              );
            })}
          </ul>
        ) : null}
      </div>

      {rows.length === 0 ? (
        <p className={styles.empty}>Nessun codice nel perimetro — cerca un codice o una descrizione per aggiungerlo.</p>
      ) : (
        <ul className={styles.rows}>
          {rows.map((candidate) => {
            const fit = fitOf(candidate);
            return (
              <li key={candidate.code} className={`${styles.row} ${styles[`fit_${fit}`]}`}>
                <span className={styles.rail} aria-hidden="true" />
                <span className={styles.code}>{candidate.code}</span>
                <span className={styles.desc} title={candidate.rationale || candidate.description || undefined}>
                  {candidate.description || '—'}
                </span>
                <span className={styles.tiers} role="group" aria-label={`Rilevanza ${candidate.code}`}>
                  {TIERS.map((tier) => (
                    <button
                      key={tier.value}
                      type="button"
                      className={`${styles.tier} ${fit === tier.value ? styles.tierActive : ''}`}
                      aria-pressed={fit === tier.value}
                      disabled={disabled}
                      onClick={() => setFit(candidate.code, tier.value)}
                    >
                      {tier.label}
                    </button>
                  ))}
                </span>
                <button
                  type="button"
                  className={styles.remove}
                  aria-label={`Rimuovi ${candidate.code}`}
                  disabled={disabled}
                  onClick={() => remove(candidate.code)}
                >
                  <Icon name="x-circle" size={14} />
                </button>
              </li>
            );
          })}
        </ul>
      )}

      <p className={styles.legend}>
        <span className={styles.legendCore}>● core</span> piena aderenza
        <span className={styles.legendWeak}>● adiacente</span> incluso, declassato
        <span className={styles.legendExcl}>● escluso</span> fuori perimetro, nascosto
      </p>
    </div>
  );
}
