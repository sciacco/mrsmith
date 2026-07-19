import { useMemo, useState, type CSSProperties } from 'react';
import { Icon } from '@mrsmith/ui';
import type { MAInitiativeCardView, MACardProvenance } from '../../../api/types';
import { CARD_STATES, MACROFASI, stateLabel, esitoLabel, stateVars, stateMacrofase } from '../../../lib/cardStates';
import styles from './board.module.css';

function stars(card: MAInitiativeCardView): string {
  const prov: MACardProvenance | undefined = card.provenances?.[0];
  if (!prov) return '—';
  const r = Math.max(0, Math.min(3, prov.rating));
  return r > 0 ? '★'.repeat(r) : '—';
}

/** Vista tabella: filtri macrofase + stato + ricerca. Righe keyboard-operable
 *  (§13.2): la cella primaria è un <button>. */
export function BoardTable({
  cards,
  onRowClick,
}: {
  cards: MAInitiativeCardView[];
  onRowClick: (card: MAInitiativeCardView) => void;
}) {
  const [macro, setMacro] = useState('');
  const [state, setState] = useState('');
  const [query, setQuery] = useState('');

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    return cards.filter((c) => {
      if (macro && stateMacrofase(c.state) !== macro) return false;
      if (state && c.state !== state) return false;
      if (q && !c.companyName.toLowerCase().includes(q)) return false;
      return true;
    });
  }, [cards, macro, state, query]);

  return (
    <div className={styles.tableWrap}>
      <div className={styles.tableFilters}>
        <Icon name="filter" size={14} />
        <select className={styles.select} value={macro} onChange={(e) => setMacro(e.target.value)} aria-label="Filtra per macrofase">
          <option value="">Tutte le macrofasi</option>
          {MACROFASI.map((m) => (
            <option key={m.key} value={m.key}>
              {m.label}
            </option>
          ))}
        </select>
        <select className={styles.select} value={state} onChange={(e) => setState(e.target.value)} aria-label="Filtra per stato">
          <option value="">Tutti gli stati</option>
          {CARD_STATES.map((s) => (
            <option key={s.key} value={s.key}>
              {s.label}
            </option>
          ))}
        </select>
        <div className={styles.search} style={{ marginLeft: 'auto' }}>
          <Icon name="search" size={14} />
          <input placeholder="Cerca azienda…" value={query} onChange={(e) => setQuery(e.target.value)} />
        </div>
      </div>
      <table className={styles.table}>
        <thead>
          <tr>
            <th>Azienda</th>
            <th>Prov.</th>
            <th>Stato</th>
            <th>Giudizio</th>
            <th>Registro</th>
            <th>Aggiornamento</th>
          </tr>
        </thead>
        <tbody>
          {filtered.length === 0 ? (
            <tr>
              <td colSpan={6} style={{ textAlign: 'center', padding: '48px 0' }}>
                <span className={styles.hint}>Nessuna azienda corrisponde ai filtri.</span>
              </td>
            </tr>
          ) : (
            filtered.map((card) => (
              <tr key={card.companyKey} className={styles.tableRow} onClick={() => onRowClick(card)}>
                <td>
                  <button
                    type="button"
                    onClick={(e) => {
                      e.stopPropagation();
                      onRowClick(card);
                    }}
                    style={{ border: 'none', background: 'none', font: 'inherit', color: 'var(--color-text)', cursor: 'pointer', fontWeight: 600, padding: 0 }}
                  >
                    {card.companyName}
                  </button>
                </td>
                <td>{card.province || '—'}</td>
                <td>
                  <span className={styles.statePill} style={stateVars(card.state) as CSSProperties}>
                    <span className={styles.dot} />
                    {stateLabel(card.state)}
                    {card.esito ? ` · ${esitoLabel(card.esito)}` : ''}
                  </span>
                </td>
                <td style={{ fontVariantNumeric: 'tabular-nums', color: 'var(--color-warning-strong)' }}>{stars(card)}</td>
                <td>{(card.registryFacts ?? []).length > 0 ? (card.registryFacts ?? []).length : '—'}</td>
                <td className={styles.hint}>{card.lastEvent || '—'}</td>
              </tr>
            ))
          )}
        </tbody>
      </table>
    </div>
  );
}
