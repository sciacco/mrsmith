import { useRef, useState, type CSSProperties } from 'react';
import { Link } from 'react-router-dom';
import { Button, Drawer, Icon, Skeleton } from '@mrsmith/ui';
import type { MAInitiativeCardView } from '../../../api/types';
import { errorLabel } from '../../ricerche/helpers';
import { ActivityTimeline } from '../../../components/company/activity/ActivityTimeline';
import { useAnnotationMutations, useCompanyActivity } from '../../../hooks/useCompanyActivity';
import { writeCohort } from '../../../components/scheda/cohort';
import { ACTIVE_STATES, isTerminalState, stateLabel, esitoLabel, stateVars } from '../../../lib/cardStates';
import { dossierState } from './useBoardData';
import type { TerminalTarget } from './TerminalModal';
import styles from './board.module.css';

const REGISTRY_LABELS: Record<string, { label: string; kind: 'info' | 'warn' }> = {
  non_vende: { label: 'Non vende', kind: 'warn' },
  in_trattativa_altrui: { label: 'In trattativa con altri', kind: 'warn' },
  da_evitare: { label: 'Da evitare', kind: 'warn' },
  gia_cliente: { label: 'Già cliente', kind: 'info' },
  partner: { label: 'Partner', kind: 'info' },
};

function schedaHref(companyKey: string, initiativeId: string) {
  return `/aziende/${encodeURIComponent(companyKey)}?iniziativa=${encodeURIComponent(initiativeId)}`;
}

export function CardDrawer({
  initiativeId,
  initiativeTitle,
  card,
  cohortKeys,
  onClose,
  onChanged,
  onSetState,
  onOpenTerminal,
  onReopen,
  onRemove,
  onDeepDive,
  onOpenDossier,
}: {
  initiativeId: string;
  initiativeTitle: string;
  card: MAInitiativeCardView;
  cohortKeys: string[];
  onClose: () => void;
  onChanged: () => void;
  onSetState: (companyKey: string, state: string, recontactOn?: string | null) => void;
  onOpenTerminal: (card: MAInitiativeCardView, target: TerminalTarget) => void;
  onReopen: (companyKey: string) => void;
  onRemove: (card: MAInitiativeCardView) => void;
  onDeepDive: (companyKey: string) => Promise<void>;
  onOpenDossier: (card: MAInitiativeCardView) => void;
}) {
  const activity = useCompanyActivity(card.companyKey);
  const annotations = useAnnotationMutations(card.companyKey, initiativeId);
  const [note, setNote] = useState('');
  const [noteError, setNoteError] = useState('');
  const [contextExpanded, setContextExpanded] = useState(false);
  const [timelineEditing, setTimelineEditing] = useState(false);
  const [dismissError, setDismissError] = useState('');
  const [announcement, setAnnouncement] = useState('');
  const [launching, setLaunching] = useState(false);
  const composerRef = useRef<HTMLTextAreaElement>(null);

  const terminal = isTerminalState(card.state);
  const removed = card.state === 'rimossa';
  const ds = dossierState(card.dossierStatus);
  const canLaunch = ds === 'none' || ds === 'failed';

  const announce = (message: string) => {
    setAnnouncement('');
    requestAnimationFrame(() => setAnnouncement(message));
  };

  const discardDraft = () => {
    setNote('');
    setNoteError('');
    setDismissError('');
    composerRef.current?.focus();
  };

  const submitNote = async () => {
    const body = note.trim();
    if (!body) return;
    setNoteError('');
    try {
      await annotations.create.mutateAsync(body);
      setNote('');
      announce('Annotazione aggiunta.');
      onChanged();
    } catch (e) {
      setNoteError(errorLabel(e));
    }
  };

  const launch = async () => {
    setLaunching(true);
    try {
      await onDeepDive(card.companyKey);
    } catch {
      /* toast a monte */
    } finally {
      setLaunching(false);
    }
  };

  const facts = card.registryFacts ?? [];
  const activityItems = activity.data?.items ?? [];
  const sessionInitiative = new Map((activity.data?.sessions ?? []).map((session) => [session.id, session.initiativeId]));
  const belongsToCurrentInitiative = (item: (typeof activityItems)[number]) =>
    item.initiativeId === initiativeId || Boolean(item.sessionId && sessionInitiative.get(item.sessionId) === initiativeId);
  const currentItems = activityItems.filter(belongsToCurrentInitiative);
  const contextAnnotations = activityItems.filter((item) => item.event === 'nota' && !belongsToCurrentInitiative(item));
  const visibleContextAnnotations = contextExpanded ? contextAnnotations : contextAnnotations.slice(0, 2);

  return (
    <Drawer
      open
      onClose={() => { setDismissError(''); onClose(); }}
      onDismissAttempt={() => {
        if (!note.trim() && !timelineEditing) return true;
        setDismissError('Salva o annulla le modifiche prima di chiudere.');
        return false;
      }}
      title={card.companyName}
      subtitle={
        <div className={styles.drawerMeta}>
          {card.vatCode ? <span>P.IVA {card.vatCode}</span> : null}
          {card.province ? <span> · {card.province}</span> : null}
          {card.origin === 'direct' ? <span> · Diretta</span> : null}
        </div>
      }
      headerExtra={
        <div className={styles.drawerActions}>
          <Link
            className={styles.actionLink}
            to={schedaHref(card.companyKey, initiativeId)}
            target="_blank"
            rel="noopener noreferrer"
            onClick={() => writeCohort({ lensType: 'iniziativa', lensId: initiativeId, companyKeys: cohortKeys })}
            onAuxClick={() => writeCohort({ lensType: 'iniziativa', lensId: initiativeId, companyKeys: cohortKeys })}
          >
            Apri scheda ↗
          </Link>
          <Button variant="secondary" size="sm" onClick={() => onOpenDossier(card)}>
            Dossier ↗
          </Button>
          {terminal || removed ? (
            <Button variant="secondary" size="sm" onClick={() => onReopen(card.companyKey)}>
              Riapri
            </Button>
          ) : (
            <Button variant="secondary" size="sm" onClick={() => onRemove(card)} aria-label="Rimuovi">
              <Icon name="trash" size={14} />
            </Button>
          )}
        </div>
      }
      size="lg"
    >
      <div className={styles.drawerBody}>
        <div className={styles.drawerScrollArea}>
          {!terminal && !removed ? (
            <>
              <div className={styles.pipeline}>
                {ACTIVE_STATES.map((state, idx) => (
                  <div key={state.key} className={styles.pipelineStep}>
                    <button
                      type="button"
                      className={[styles.stationBtn, card.state === state.key ? styles.stationOn : ''].filter(Boolean).join(' ')}
                      style={stateVars(state.key) as CSSProperties}
                      onClick={() => onSetState(card.companyKey, state.key)}
                    >
                      <span className={styles.stepCircle}>{idx + 1}</span>
                      <span className={styles.stepLabel}>{state.label}</span>
                    </button>
                    {idx < ACTIVE_STATES.length - 1 ? <div className={styles.stepConnector} /> : null}
                  </div>
                ))}
              </div>

              {card.state === 'ricontattare' ? (
                <div className={styles.field}>
                  <label>Data di ricontatto (opzionale)</label>
                  <input
                    className={styles.input}
                    type="date"
                    value={card.recontactOn ? card.recontactOn.slice(0, 10) : ''}
                    onChange={(e) => onSetState(card.companyKey, 'ricontattare', e.target.value || null)}
                  />
                </div>
              ) : null}

              <div className={styles.terminalRow}>
                <Button variant="secondary" size="sm" onClick={() => onOpenTerminal(card, 'won')}>
                  WON
                </Button>
                <Button variant="danger" size="sm" onClick={() => onOpenTerminal(card, 'ko_nostro')}>
                  KO nostro
                </Button>
                <Button variant="danger" size="sm" onClick={() => onOpenTerminal(card, 'ko_target')}>
                  KO target
                </Button>
              </div>
            </>
          ) : (
            <div className={styles.drawerSec} style={{ borderTop: 0, paddingTop: 0, marginTop: 0 }}>
              <p className={styles.lab}>Stato</p>
              <p>
                {stateLabel(card.state)}
                {card.esito ? ` · ${esitoLabel(card.esito)}` : ''}
              </p>
            </div>
          )}

          <div className={styles.drawerSec}>
            <p className={styles.lab}>Analisi approfondita</p>
            {ds === 'ready' ? (
              <Button variant="secondary" size="sm" onClick={() => onOpenDossier(card)}>
                Apri dossier
              </Button>
            ) : ds === 'working' ? (
              <p className={styles.hint}>Analisi in elaborazione. La board si aggiorna automaticamente.</p>
            ) : canLaunch ? (
              <Button variant="primary" size="sm" onClick={() => void launch()} loading={launching}>
                Avvia analisi
              </Button>
            ) : (
              <p className={styles.hint}>Stato analisi non disponibile.</p>
            )}
          </div>

          <div className={styles.drawerSec}>
            <p className={styles.lab}>Registro azienda</p>
            {facts.length > 0 ? (
              <div className={styles.cardMeta}>
                {facts.map((kind) => {
                  const info = REGISTRY_LABELS[kind] ?? { label: kind, kind: 'info' as const };
                  return (
                    <span key={kind} className={`${styles.chip} ${info.kind === 'warn' ? styles.chipRegWarn : styles.chipRegInfo}`}>
                      {info.label}
                    </span>
                  );
                })}
              </div>
            ) : (
              <p className={styles.hint}>Nessun fatto registrato.</p>
            )}
          </div>

          {activity.isLoading ? (
            <div className={styles.drawerSec}>
              <p className={styles.lab}>Attività · {initiativeTitle}</p>
              <Skeleton rows={3} />
            </div>
          ) : activity.isError ? (
            <div className={styles.drawerSec}>
              <p className={styles.lab}>Attività</p>
              <div className={styles.activityError} role="alert">
                <Icon name="triangle-alert" size={16} />
                <div>
                  <p>Attività non disponibile.</p>
                  <Button variant="secondary" size="sm" onClick={() => void activity.refetch()}>Riprova</Button>
                </div>
              </div>
            </div>
          ) : (
            <>
              <div className={styles.drawerSec}>
                <p className={styles.lab}>Attività · {initiativeTitle}</p>
                <ActivityTimeline
                  items={currentItems}
                  initiatives={activity.data?.initiatives}
                  sessions={activity.data?.sessions}
                  companyKey={card.companyKey}
                  editableInitiativeId={initiativeId}
                  onEditingChange={setTimelineEditing}
                />
              </div>

              <div className={styles.drawerSec}>
                <p className={styles.lab}>Contesto azienda</p>
                {contextAnnotations.length > 0 ? (
                  <p className={styles.hint}>{contextAnnotations.length} {contextAnnotations.length === 1 ? 'annotazione' : 'annotazioni'} da altri contesti</p>
                ) : null}
                <ActivityTimeline
                  items={visibleContextAnnotations}
                  initiatives={activity.data?.initiatives}
                  sessions={activity.data?.sessions}
                  companyKey={card.companyKey}
                  emptyLabel="Nessuna annotazione da altri contesti."
                />
                {contextAnnotations.length > 2 ? (
                  <Button variant="ghost" size="sm" onClick={() => setContextExpanded((value) => !value)}>
                    {contextExpanded ? 'Mostra meno' : 'Mostra tutte'}
                  </Button>
                ) : null}
              </div>
            </>
          )}
        </div>

        <div className={styles.composer}>
          <div className={styles.composerField}>
            <textarea
              ref={composerRef}
              className={styles.composerInput}
              placeholder="Aggiungi annotazione…"
              value={note}
              aria-invalid={Boolean(noteError)}
              aria-describedby={noteError ? 'card-annotation-error' : undefined}
              onChange={(e) => { setNote(e.target.value); setNoteError(''); setDismissError(''); }}
              maxLength={1000}
              rows={2}
            />
            <div className={styles.composerMeta}>
              <span>Contesto: {initiativeTitle}</span>
              <span>{note.length}/1.000</span>
            </div>
            {noteError ? <p id="card-annotation-error" className={styles.composerError} role="alert">{noteError}</p> : null}
            {dismissError ? <p className={styles.composerError} role="alert">{dismissError}</p> : null}
            <span className={styles.srStatus} role="status">{announcement}</span>
          </div>
          <div className={styles.composerActions}>
            {note ? <Button variant="ghost" size="sm" onClick={discardDraft}>Annulla</Button> : null}
            <Button variant="primary" size="sm" onClick={() => void submitNote()} loading={annotations.create.isPending} disabled={!note.trim()}>
              Aggiungi
            </Button>
          </div>
        </div>
      </div>
    </Drawer>
  );
}
