import { useMemo, useState } from 'react';
import { Button } from '@mrsmith/ui';
import type { MACompanyActivityLookup, MACompanyActivitySession, MATargetOutcome } from '../../../api/types';
import { useAnnotationMutations } from '../../../hooks/useCompanyActivity';
import { eventLabel, TECHNICAL_ACTIVITY_EVENTS } from './eventLabel';
import styles from './ActivityTimeline.module.css';

function shortAuthor(email?: string) {
  if (!email) return 'Sistema';
  return email.split('@')[0] || email;
}

function dateTime(value?: string) {
  if (!value) return '';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '';
  return new Intl.DateTimeFormat('it-IT', { dateStyle: 'short', timeStyle: 'short' }).format(date);
}

function originLabel(
  item: MATargetOutcome,
  initiativeMap: Map<string, string>,
  sessionMap: Map<string, MACompanyActivitySession>,
) {
  const payload = item.payload as Record<string, unknown> | undefined;
  if (payload?.annotationOrigin === 'legacy_registry') return 'Registro azienda';
  if (payload?.annotationOrigin === 'system_domain') return 'Verifica automatica del dominio';
  if (item.initiativeId && initiativeMap.has(item.initiativeId)) return initiativeMap.get(item.initiativeId) ?? '';
  if (item.sessionId) {
    const session = sessionMap.get(item.sessionId);
    if (session) return `Ricerca ${session.title}`;
  }
  return 'Scheda azienda';
}

export function ActivityTimeline({
  items,
  initiatives = [],
  sessions = [],
  companyKey,
  editableInitiativeId,
  allowAllAnnotations = false,
  showTechnical = false,
  emptyLabel = 'Nessuna attività registrata.',
}: {
  items: MATargetOutcome[];
  initiatives?: MACompanyActivityLookup[];
  sessions?: MACompanyActivitySession[];
  companyKey: string;
  editableInitiativeId?: string;
  allowAllAnnotations?: boolean;
  showTechnical?: boolean;
  emptyLabel?: string;
}) {
  const mutations = useAnnotationMutations(companyKey, editableInitiativeId);
  const [editing, setEditing] = useState<string | null>(null);
  const [draft, setDraft] = useState('');
  const [error, setError] = useState('');
  const initiativeMap = useMemo(() => new Map(initiatives.map((item) => [item.id, item.title])), [initiatives]);
  const sessionMap = useMemo(() => new Map(sessions.map((item) => [item.id, item])), [sessions]);
  const sessionTitles = useMemo(() => new Map(sessions.map((item) => [item.id, item.title])), [sessions]);
  const visible = showTechnical ? items : items.filter((item) => !TECHNICAL_ACTIVITY_EVENTS.has(item.event));

  const save = async (item: MATargetOutcome) => {
    const body = draft.trim();
    if (!body) {
      setError('Inserisci il testo dell’annotazione.');
      return;
    }
    setError('');
    try {
      await mutations.update.mutateAsync({ id: item.id, body });
      setEditing(null);
      setDraft('');
    } catch {
      setError('Modifica non salvata. Riprova.');
    }
  };

  const remove = async (item: MATargetOutcome) => {
    if (!window.confirm('Eliminare questa annotazione? Resterà visibile nella traccia completa dell’Inspector.')) return;
    setError('');
    try {
      await mutations.remove.mutateAsync(item.id);
    } catch {
      setError('Annotazione non eliminata. Riprova.');
    }
  };

  if (visible.length === 0) return <p className={styles.empty}>{emptyLabel}</p>;

  return (
    <div className={styles.timeline}>
      {error ? <p className={styles.error} role="alert">{error}</p> : null}
      {visible.map((item) => {
        const annotation = item.event === 'nota';
        const editable = annotation && !item.deletedAt && (allowAllAnnotations || (Boolean(editableInitiativeId) && item.initiativeId === editableInitiativeId));
        return (
          <article key={item.id} className={`${styles.item} ${annotation ? styles.annotation : styles.event} ${item.deletedAt ? styles.deleted : ''}`}>
            <span className={styles.dot} aria-hidden="true" />
            <div className={styles.content}>
              {item.deletedAt ? <span className={styles.deletedLabel}>Annotazione eliminata</span> : null}
              {editing === item.id ? (
                <div className={styles.editor}>
                  <textarea value={draft} maxLength={1000} rows={4} onChange={(event) => setDraft(event.target.value)} autoFocus />
                  <div className={styles.actions}>
                    <Button size="sm" variant="primary" loading={mutations.update.isPending} onClick={() => void save(item)}>Salva</Button>
                    <Button size="sm" variant="ghost" onClick={() => { setEditing(null); setDraft(''); setError(''); }}>Annulla</Button>
                  </div>
                </div>
              ) : (
                <p className={styles.text}>{eventLabel(item, sessionTitles)}</p>
              )}
              <p className={styles.meta}>
                {originLabel(item, initiativeMap, sessionMap)} · {shortAuthor(item.createdByEmail)} · {dateTime(item.createdAt)}
                {item.updatedAt ? ` · modificata ${dateTime(item.updatedAt)} da ${shortAuthor(item.updatedByEmail)}` : ''}
                {item.deletedAt ? ` · eliminata ${dateTime(item.deletedAt)} da ${shortAuthor(item.deletedByEmail)}` : ''}
              </p>
              {editable && editing !== item.id ? (
                <div className={styles.actions}>
                  <Button size="sm" variant="ghost" onClick={() => { setEditing(item.id); setDraft(item.note ?? ''); setError(''); }}>Modifica</Button>
                  <Button size="sm" variant="ghost" onClick={() => void remove(item)}>Elimina</Button>
                </div>
              ) : null}
            </div>
          </article>
        );
      })}
    </div>
  );
}
