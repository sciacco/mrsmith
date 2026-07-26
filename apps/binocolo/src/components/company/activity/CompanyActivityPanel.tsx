import { useEffect, useMemo, useRef, useState } from 'react';
import { Button, Drawer, SingleSelect, Skeleton } from '@mrsmith/ui';
import { useAnnotationMutations, useCompanyActivity } from '../../../hooks/useCompanyActivity';
import { ActivityTimeline } from './ActivityTimeline';
import styles from './CompanyActivityPanel.module.css';

export function CompanyActivityPanel({ companyKey, companyName, vatCode }: { companyKey: string; companyName: string; vatCode?: string }) {
  const activity = useCompanyActivity(companyKey);
  const mutations = useAnnotationMutations(companyKey);
  const [open, setOpen] = useState(false);
  const [focusComposer, setFocusComposer] = useState(false);
  const [annotationsOnly, setAnnotationsOnly] = useState(false);
  const [initiativeId, setInitiativeId] = useState('');
  const [body, setBody] = useState('');
  const [error, setError] = useState('');
  const [dismissError, setDismissError] = useState('');
  const [timelineEditing, setTimelineEditing] = useState(false);
  const [announcement, setAnnouncement] = useState('');
  const composerRef = useRef<HTMLTextAreaElement>(null);
  const items = activity.data?.items ?? [];
  const activeAnnotations = items.filter((item) => item.event === 'nota' && !item.deletedAt);
  const initiativeOptions = (activity.data?.initiatives ?? []).map((initiative) => ({ value: initiative.id, label: initiative.title }));

  useEffect(() => {
    if (open && focusComposer) requestAnimationFrame(() => composerRef.current?.focus());
  }, [focusComposer, open]);

  const openForAnnotation = () => {
    setFocusComposer(true);
    setOpen(true);
  };
  const openHistory = () => {
    setFocusComposer(false);
    setOpen(true);
  };

  const filtered = useMemo(() => items.filter((item) => {
    if (annotationsOnly && item.event !== 'nota') return false;
    if (!initiativeId) return true;
    if (item.initiativeId === initiativeId) return true;
    const session = activity.data?.sessions.find((candidate) => candidate.id === item.sessionId);
    return session?.initiativeId === initiativeId;
  }), [activity.data?.sessions, annotationsOnly, initiativeId, items]);

  const submit = async () => {
    const value = body.trim();
    if (!value) return;
    setError('');
    try {
      await mutations.create.mutateAsync(value);
      setBody('');
      setAnnouncement('Annotazione aggiunta.');
      composerRef.current?.focus();
    } catch {
      setError('Annotazione non salvata. Riprova.');
    }
  };

  return (
    <>
      <section className={styles.summary}>
        <div className={styles.heading}>
          <div>
            <h3>Attività e annotazioni</h3>
            {!activity.isLoading && !activity.isError ? (
              <p>{activeAnnotations.length} {activeAnnotations.length === 1 ? 'annotazione attiva' : 'annotazioni attive'}</p>
            ) : null}
          </div>
          <div className={styles.headingActions}>
            <Button size="sm" variant="secondary" onClick={openForAnnotation}>Aggiungi annotazione</Button>
            {items.length > 0 || activity.isError ? <Button size="sm" variant="ghost" onClick={openHistory}>Apri cronologia</Button> : null}
          </div>
        </div>
        {activity.isLoading ? <Skeleton rows={3} /> : activity.isError ? (
          <div className={styles.errorState} role="alert">
            <span>Cronologia non disponibile.</span>
            <Button size="sm" variant="secondary" onClick={() => void activity.refetch()}>Riprova</Button>
          </div>
        ) : activeAnnotations.length === 0 ? (
          <div className={styles.empty}>
            <p>Nessuna annotazione registrata.</p>
            <p>Le annotazioni nate dalla Scheda, dalle iniziative o da processi automatici saranno visibili qui.</p>
          </div>
        ) : (
          <ActivityTimeline
            items={activeAnnotations.slice(0, 3)}
            initiatives={activity.data?.initiatives}
            sessions={activity.data?.sessions}
            companyKey={companyKey}
            relativeDates
            compact
          />
        )}
      </section>

      {open ? (
        <Drawer
          open
          onClose={() => { setOpen(false); setDismissError(''); }}
          onDismissAttempt={() => {
            if (!body.trim() && !timelineEditing) return true;
            setDismissError('Salva o annulla le modifiche prima di chiudere.');
            return false;
          }}
          title="Attività e annotazioni"
          subtitle={`${companyName}${vatCode ? ` · P.IVA ${vatCode}` : ''}`}
          size="lg"
        >
          <div className={styles.drawerBody}>
            <div className={styles.toolbar}>
              <div className={styles.segmented} role="group" aria-label="Tipo di attività">
                <button type="button" className={!annotationsOnly ? styles.active : ''} aria-pressed={!annotationsOnly} onClick={() => setAnnotationsOnly(false)}>Tutto</button>
                <button type="button" className={annotationsOnly ? styles.active : ''} aria-pressed={annotationsOnly} onClick={() => setAnnotationsOnly(true)}>Annotazioni</button>
              </div>
              {initiativeOptions.length > 1 ? (
                <div className={styles.initiativeFilter} role="group" aria-label="Filtra per iniziativa">
                  <SingleSelect
                    options={initiativeOptions}
                    selected={initiativeId || null}
                    onChange={(value) => setInitiativeId(value ?? '')}
                    placeholder="Tutte le iniziative"
                    allowClear
                    clearLabel="Tutte le iniziative"
                    ariaLabel="Filtra per iniziativa"
                  />
                </div>
              ) : null}
            </div>
            <div className={styles.dismissSlot}>
              {dismissError ? <p className={styles.dismissError} role="alert">{dismissError}</p> : null}
            </div>
            <div className={styles.scroll}>
              {activity.isLoading ? <Skeleton rows={5} /> : activity.isError ? (
                <div className={styles.errorState} role="alert">
                  <span>Cronologia non disponibile.</span>
                  <Button size="sm" variant="secondary" onClick={() => void activity.refetch()}>Riprova</Button>
                </div>
              ) : (
                <ActivityTimeline
                  items={filtered}
                  initiatives={activity.data?.initiatives}
                  sessions={activity.data?.sessions}
                  companyKey={companyKey}
                  allowAllAnnotations
                  onEditingChange={setTimelineEditing}
                />
              )}
            </div>
            <div className={styles.composer}>
              <textarea
                ref={composerRef}
                value={body}
                rows={3}
                maxLength={1000}
                placeholder="Aggiungi annotazione…"
                aria-invalid={Boolean(error)}
                aria-describedby={error ? 'company-annotation-error' : undefined}
                onChange={(event) => { setBody(event.target.value); setError(''); setDismissError(''); }}
              />
              <div className={styles.composerFooter}>
                <span>{body.length}/1.000</span>
                <Button variant="primary" size="sm" disabled={!body.trim()} loading={mutations.create.isPending} onClick={() => void submit()}>Aggiungi</Button>
              </div>
              {error ? <p id="company-annotation-error" className={styles.error} role="alert">{error}</p> : null}
              <span className={styles.srStatus} role="status">{announcement}</span>
            </div>
          </div>
        </Drawer>
      ) : null}
    </>
  );
}
