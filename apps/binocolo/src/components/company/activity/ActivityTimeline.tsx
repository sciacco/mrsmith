import { useEffect, useMemo, useRef, useState } from 'react';
import { Button, Modal } from '@mrsmith/ui';
import type { MACompanyActivityLookup, MACompanyActivitySession, MATargetOutcome } from '../../../api/types';
import { useAnnotationMutations } from '../../../hooks/useCompanyActivity';
import { eventLabel, TECHNICAL_ACTIVITY_EVENTS } from './eventLabel';
import styles from './ActivityTimeline.module.css';

function shortAuthor(email?: string) {
  if (!email) return 'Autore non disponibile';
  return email.split('@')[0] || email;
}

function dateTime(value?: string) {
  if (!value) return '';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '';
  return new Intl.DateTimeFormat('it-IT', { dateStyle: 'short', timeStyle: 'short' }).format(date);
}

function relativeDate(value?: string) {
  if (!value) return '';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '';
  const minutes = Math.max(0, Math.floor((Date.now() - date.getTime()) / 60_000));
  if (minutes < 1) return 'ora';
  if (minutes < 60) return `${minutes} min fa`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours} h fa`;
  const days = Math.floor(hours / 24);
  if (days === 1) return 'ieri';
  if (days < 7) return `${days} giorni fa`;
  return new Intl.DateTimeFormat('it-IT', { day: '2-digit', month: 'short' }).format(date);
}

function contextLabel(
  item: MATargetOutcome,
  initiativeMap: Map<string, string>,
  sessionMap: Map<string, MACompanyActivitySession>,
) {
  const payload = item.payload as Record<string, unknown> | undefined;
  if (payload?.annotationOrigin === 'legacy_registry') return 'Registro azienda';
  if (item.initiativeId) return initiativeMap.get(item.initiativeId) ?? 'Iniziativa';
  if (item.sessionId) {
    const session = sessionMap.get(item.sessionId);
    if (session) return `Ricerca ${session.title}`;
    return 'Ricerca';
  }
  return 'Scheda azienda';
}

function authorLabel(item: MATargetOutcome) {
  const payload = item.payload as Record<string, unknown> | undefined;
  if (payload?.annotationOrigin === 'system_domain') return 'Verifica automatica del dominio';
  return shortAuthor(item.createdByEmail);
}

function eventKindLabel(item: MATargetOutcome) {
  switch (item.event) {
    case 'nota': return 'Annotazione';
    case 'stato': return 'Stato';
    case 'chiusura':
    case 'contattato':
    case 'buon_lead':
    case 'no_go': return 'Esito';
    case 'card_creata': return 'Inserimento';
    case 'card_riaperta': return 'Riapertura';
    case 'card_rimossa': return 'Rimozione';
    case 'dominio_verificato': return 'Verifica';
    default: return 'Attività';
  }
}

export function ActivityTimeline({
  items,
  initiatives = [],
  sessions = [],
  companyKey,
  editableInitiativeId,
  allowAllAnnotations = false,
  showTechnical = false,
  relativeDates = false,
  compact = false,
  onEditingChange,
  emptyLabel = 'Nessuna attività registrata.',
}: {
  items: MATargetOutcome[];
  initiatives?: MACompanyActivityLookup[];
  sessions?: MACompanyActivitySession[];
  companyKey: string;
  editableInitiativeId?: string;
  allowAllAnnotations?: boolean;
  showTechnical?: boolean;
  relativeDates?: boolean;
  compact?: boolean;
  onEditingChange?: (editing: boolean) => void;
  emptyLabel?: string;
}) {
  const mutations = useAnnotationMutations(companyKey, editableInitiativeId);
  const [editing, setEditing] = useState<string | null>(null);
  const [draft, setDraft] = useState('');
  const [deleteTarget, setDeleteTarget] = useState<MATargetOutcome | null>(null);
  const [deleteError, setDeleteError] = useState('');
  const [error, setError] = useState('');
  const [announcement, setAnnouncement] = useState('');
  const timelineRef = useRef<HTMLElement | null>(null);
  const editButtonRefs = useRef(new Map<string, HTMLButtonElement>());
  const initiativeMap = useMemo(() => new Map(initiatives.map((item) => [item.id, item.title])), [initiatives]);
  const sessionMap = useMemo(() => new Map(sessions.map((item) => [item.id, item])), [sessions]);
  const sessionTitles = useMemo(() => new Map(sessions.map((item) => [item.id, item.title])), [sessions]);
  const visible = showTechnical ? items : items.filter((item) => !TECHNICAL_ACTIVITY_EVENTS.has(item.event));
  const announce = (message: string) => {
    setAnnouncement('');
    requestAnimationFrame(() => setAnnouncement(message));
  };

  useEffect(() => {
    onEditingChange?.(editing !== null);
    return () => onEditingChange?.(false);
  }, [editing, onEditingChange]);

  const cancelEdit = (itemId: string) => {
    setEditing(null);
    setDraft('');
    setError('');
    requestAnimationFrame(() => editButtonRefs.current.get(itemId)?.focus());
  };

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
      announce('Annotazione aggiornata.');
      requestAnimationFrame(() => editButtonRefs.current.get(item.id)?.focus());
    } catch {
      setError('Modifica non salvata. Riprova.');
    }
  };

  const remove = async () => {
    if (!deleteTarget) return;
    setDeleteError('');
    try {
      await mutations.remove.mutateAsync(deleteTarget.id);
      setDeleteTarget(null);
      announce('Annotazione eliminata.');
      requestAnimationFrame(() => timelineRef.current?.focus());
    } catch {
      setDeleteError('Annotazione non eliminata. Riprova.');
    }
  };

  if (visible.length === 0) return (
    <p ref={(element) => { timelineRef.current = element; }} className={styles.empty} tabIndex={-1}>
      <span className={styles.srStatus} role="status">{announcement}</span>
      {emptyLabel}
    </p>
  );

  return (
    <>
    <div ref={(element) => { timelineRef.current = element; }} className={`${styles.timeline} ${compact ? styles.compact : ''}`} tabIndex={-1}>
      <span className={styles.srStatus} role="status">{announcement}</span>
      {error ? <p className={styles.error} role="alert">{error}</p> : null}
      {visible.map((item) => {
        const annotation = item.event === 'nota';
        const editable = annotation && !item.deletedAt && (allowAllAnnotations || (Boolean(editableInitiativeId) && item.initiativeId === editableInitiativeId));
        const context = contextLabel(item, initiativeMap, sessionMap);
        const kind = eventKindLabel(item);
        return (
          <article
            key={item.id}
            className={`${styles.item} ${annotation ? styles.annotation : styles.event} ${item.deletedAt ? styles.deleted : ''}`}
            aria-label={`${kind}: ${eventLabel(item, sessionTitles)}`}
          >
            <span className={styles.dot} aria-hidden="true" />
            <div className={styles.content}>
              <p className={styles.eyebrow}>{kind.toUpperCase()} · {context}</p>
              {item.deletedAt ? <span className={styles.deletedLabel}>Annotazione eliminata</span> : null}
              {editing === item.id ? (
                <div className={styles.editor}>
                  <textarea
                    value={draft}
                    maxLength={1000}
                    rows={4}
                    onChange={(event) => setDraft(event.target.value)}
                    onKeyDown={(event) => {
                      if (event.key === 'Escape') {
                        event.preventDefault();
                        event.stopPropagation();
                        cancelEdit(item.id);
                      }
                    }}
                    autoFocus
                  />
                  <div className={styles.actions}>
                    <Button size="sm" variant="primary" loading={mutations.update.isPending} onClick={() => void save(item)}>Salva</Button>
                    <Button size="sm" variant="ghost" onClick={() => cancelEdit(item.id)}>Annulla</Button>
                  </div>
                </div>
              ) : (
                <p className={styles.text}>{eventLabel(item, sessionTitles)}</p>
              )}
              <p className={styles.meta}>
                {authorLabel(item)} · {relativeDates ? relativeDate(item.createdAt) : dateTime(item.createdAt)}
                {item.updatedAt ? ` · modificata ${dateTime(item.updatedAt)} da ${shortAuthor(item.updatedByEmail)}` : ''}
                {item.deletedAt ? ` · eliminata ${dateTime(item.deletedAt)} da ${shortAuthor(item.deletedByEmail)}` : ''}
              </p>
              {editable && editing !== item.id ? (
                <div className={styles.actions}>
                  <Button
                    ref={(element) => { if (element) editButtonRefs.current.set(item.id, element); else editButtonRefs.current.delete(item.id); }}
                    size="sm"
                    variant="ghost"
                    onClick={() => { setEditing(item.id); setDraft(item.note ?? ''); setError(''); }}
                  >
                    Modifica
                  </Button>
                  <Button size="sm" variant="ghost" onClick={() => { setDeleteError(''); setDeleteTarget(item); }}>Elimina</Button>
                </div>
              ) : null}
            </div>
          </article>
        );
      })}
    </div>
    <Modal
      open={deleteTarget !== null}
      onClose={() => { setDeleteTarget(null); setDeleteError(''); }}
      title="Eliminare questa annotazione?"
      size="sm"
      dismissible={!mutations.remove.isPending}
    >
      <div className={styles.confirmation}>
        <p>Non sarà più visibile nella Scheda e nelle iniziative.</p>
        <p>Resterà consultabile nello storico read-only.</p>
        {deleteError ? <p className={styles.error} role="alert">{deleteError}</p> : null}
        <div className={styles.confirmationActions}>
          <Button variant="secondary" onClick={() => { setDeleteTarget(null); setDeleteError(''); }} disabled={mutations.remove.isPending}>Annulla</Button>
          <Button variant="danger" onClick={() => void remove()} loading={mutations.remove.isPending}>Elimina</Button>
        </div>
      </div>
    </Modal>
    </>
  );
}
