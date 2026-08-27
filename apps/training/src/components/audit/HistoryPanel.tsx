// Pannello storia condiviso (#164, slice 5 del task 7): lettura di
// training.audit_log dietro un unico selettore (entità singola, persona o
// evento aggregati). Collassato di default — la query si abilita solo
// all'apertura (caricamento pigro, come da contratto). Riga "chi · quando ·
// cosa" con dettaglio campi cambiati espandibile per riga; azioni e tipi
// entità tradotti via lib/labels.ts, mai il gergo grezzo del backend.

import { useState } from 'react';
import { Icon, Skeleton } from '@mrsmith/ui';
import { useAuditHistory, type AuditSelector } from '../../api/queries';
import type { AuditEntry } from '../../api/types';
import { describeApiError } from '../events/apiErrors';
import { formatInstantDateTime } from '../events/eventFormat';
import { AUDIT_ACTION_LABELS, AUDIT_ENTITY_TYPE_LABELS } from '../../lib/labels';
import styles from './HistoryPanel.module.css';

interface HistoryPanelProps {
  selector: AuditSelector;
  title?: string;
  className?: string;
  /** 'section': montaggio fra fratelli di pagina impaddati a --space-5
   *  (EventDetailPage .card/.section) — alza il padding interno del pannello
   *  in coerenza, senza sommare un padding esterno. Omesso altrove: il
   *  pannello resta al suo padding proprio (--space-4). */
  variant?: 'section';
}

export function HistoryPanel({ selector, title = 'Storia', className, variant }: HistoryPanelProps) {
  const [open, setOpen] = useState(false);
  const history = useAuditHistory(selector, open);
  const entries = history.data?.entries ?? [];
  const panelClassName = [styles.panel, variant === 'section' ? styles.panelSection : null, className]
    .filter(Boolean)
    .join(' ');

  return (
    <section className={panelClassName}>
      <button type="button" className={styles.toggle} aria-expanded={open} onClick={() => setOpen((v) => !v)}>
        <Icon name={open ? 'chevron-up' : 'chevron-down'} size={16} />
        <span className={styles.toggleLabel}>{title}</span>
      </button>
      {open && (
        <div className={styles.body}>
          {history.isLoading && <Skeleton rows={3} />}
          {history.isError && (
            <p className={styles.errorNotice}>{describeApiError(history.error, 'Lettura della storia non riuscita')}</p>
          )}
          {history.isSuccess && entries.length === 0 && <p className={styles.empty}>Nessuna modifica registrata.</p>}
          {history.isSuccess && entries.length > 0 && (
            <ul className={styles.list}>
              {entries.map((entry, index) => (
                <HistoryRow key={`${entry.entityType}-${entry.entityId}-${entry.occurredAt}-${index}`} entry={entry} />
              ))}
            </ul>
          )}
        </div>
      )}
    </section>
  );
}

function formatFieldValue(value: unknown): string {
  if (value === undefined || value === null) return '—';
  if (typeof value === 'object') return JSON.stringify(value);
  return String(value);
}

// Campo sintetico scritto dal backend (changedAuditFields) quando before/
// after sono JSON validi ma non oggetti — es. json_agg(...) di
// pathStepsSnapshot per learning_path/replace_steps: non ci sono chiavi da
// confrontare per campo, ma il prima/dopo grezzo esiste davvero. In questo
// caso il valore da mostrare e' l'intero before/after, non una sua chiave.
const RAW_VALUE_FIELD = 'valori';

function fieldValueFor(entry: AuditEntry, side: 'before' | 'after', field: string): unknown {
  const container = entry[side];
  if (field === RAW_VALUE_FIELD) return container;
  if (container === null || Array.isArray(container)) return undefined;
  return container[field];
}

// Colonne strutturali senza valore informativo per «chi ha cambiato cosa»:
// presenti su ogni riga di creazione (id, created_at) o di modifica
// (updated_at) a prescindere da cosa e' realmente cambiato.
const STRUCTURAL_FIELDS = new Set(['id', 'created_at', 'updated_at']);

function HistoryRow({ entry }: { entry: AuditEntry }) {
  const [expanded, setExpanded] = useState(false);
  const actionLabel = AUDIT_ACTION_LABELS[entry.action] ?? entry.action;
  const entityLabel = AUDIT_ENTITY_TYPE_LABELS[entry.entityType] ?? entry.entityType;
  const visibleFields = entry.changedFields.filter((field) => !STRUCTURAL_FIELDS.has(field));
  const hasDetail = visibleFields.length > 0;

  return (
    <li className={styles.row}>
      <button
        type="button"
        className={styles.rowHead}
        aria-expanded={expanded}
        disabled={!hasDetail}
        onClick={() => hasDetail && setExpanded((v) => !v)}
      >
        <span className={styles.rowWho}>{entry.actorName}</span>
        <span className={styles.rowWhen}>{formatInstantDateTime(entry.occurredAt)}</span>
        <span className={styles.rowWhat}>
          {actionLabel} — {entityLabel}
        </span>
        {hasDetail && <Icon name={expanded ? 'chevron-up' : 'chevron-down'} size={14} />}
      </button>
      {expanded && hasDetail && (
        <dl className={styles.fields}>
          {visibleFields.map((field) => (
            <div key={field} className={styles.fieldItem}>
              <dt>{field}</dt>
              <dd>
                {entry.valuesRecorded
                  ? `${formatFieldValue(fieldValueFor(entry, 'before', field))} → ${formatFieldValue(fieldValueFor(entry, 'after', field))}`
                  : 'Valore non registrato'}
              </dd>
            </div>
          ))}
        </dl>
      )}
    </li>
  );
}
