import { useCallback, useEffect, useMemo, useState, type CSSProperties } from 'react';
import { Link } from 'react-router-dom';
import { Button, Drawer, Icon, Skeleton, useToast } from '@mrsmith/ui';
import { useApiClient } from '../../../api/client';
import type {
  MACardEvent,
  MACardEventListResponse,
  MACompanyRegistry,
  MAInitiativeCardView,
  MASessionSummary,
} from '../../../api/types';
import { errorLabel, relativeDate, shortAuthor } from '../../ricerche/helpers';
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

/** Etichetta di un evento del diario. I payload append-only NON si riscrivono: le
 *  chiavi stato/esito storiche risolvono via stateLabel/esitoLabel (legacy-map). */
export function eventLabel(event: MACardEvent, sessionMap: Map<string, string>): string {
  const p = event.payload as Record<string, unknown> | undefined;
  switch (event.event) {
    case 'stato': {
      const from = p && typeof p.from === 'string' ? stateLabel(p.from) : null;
      const to = p && typeof p.to === 'string' ? stateLabel(p.to) : null;
      return from && to ? `${from} → ${to}` : `Stato aggiornato${event.note ? ` — ${event.note}` : ''}`;
    }
    case 'chiusura': {
      // v2: payload {stato, esito}. Storico: {esito} soltanto.
      const stato = p && typeof p.stato === 'string' ? stateLabel(p.stato) : 'Chiusura';
      const esito = p && typeof p.esito === 'string' && p.esito ? ` — ${esitoLabel(p.esito)}` : '';
      const base = `${stato}${esito}`;
      return event.note ? `${base}: ${event.note}` : base;
    }
    case 'card_creata': {
      const rating = p && typeof p.rating === 'number' ? p.rating : 0;
      const stars = rating > 0 ? ` ${'★'.repeat(Math.min(rating, 3))}` : '';
      const sessionId = p && typeof p.sessionId === 'string' ? p.sessionId : null;
      const title = sessionId ? sessionMap.get(sessionId) ?? null : null;
      return `Aggiunta${title ? ` da ${title}` : ''}${stars}${event.note ? ` — ${event.note}` : ''}`;
    }
    case 'card_riaperta':
      return 'Rimessa in lavorazione';
    case 'card_rimossa':
      return 'Rimossa dalla lavorazione';
    case 'dominio_verificato': {
      const esito = p && typeof p.esito === 'string' ? p.esito : 'non_verificabile';
      if (esito === 'confermato') return 'Dominio confermato';
      if (esito === 'non_confermato') return 'Dominio non confermato';
      return 'Dominio non verificabile';
    }
    case 'nota':
      return event.note ?? 'Nota';
    default:
      return event.note ? `${event.event} — ${event.note}` : event.event;
  }
}

export function CardDrawer({
  initiativeId,
  card,
  sessions,
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
  card: MAInitiativeCardView;
  sessions: MASessionSummary[];
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
  const api = useApiClient();
  const { toast } = useToast();
  const [events, setEvents] = useState<MACardEvent[]>([]);
  const [loadingEvents, setLoadingEvents] = useState(true);
  const [registry, setRegistry] = useState<MACompanyRegistry | null>(null);
  const [note, setNote] = useState('');
  const [savingNote, setSavingNote] = useState(false);
  const [launching, setLaunching] = useState(false);

  const terminal = isTerminalState(card.state);
  const removed = card.state === 'rimossa';
  const ds = dossierState(card.dossierStatus);
  const canLaunch = ds === 'none' || ds === 'failed';

  const loadEvents = useCallback(async () => {
    setLoadingEvents(true);
    try {
      const res = await api.get<MACardEventListResponse>(
        `/binocolo/v1/ma/initiatives/${initiativeId}/cards/${encodeURIComponent(card.companyKey)}/events`,
      );
      setEvents(res.items);
    } catch (e) {
      toast(errorLabel(e), 'error');
    } finally {
      setLoadingEvents(false);
    }
  }, [api, initiativeId, card.companyKey, toast]);

  useEffect(() => {
    void loadEvents();
  }, [loadEvents]);

  useEffect(() => {
    api
      .get<MACompanyRegistry>(`/binocolo/v1/ma/companies/${encodeURIComponent(card.companyKey)}/registry`)
      .then(setRegistry)
      .catch(() => setRegistry(null));
  }, [api, card.companyKey]);

  const sessionMap = useMemo(() => {
    const m = new Map<string, string>();
    for (const s of sessions) m.set(s.id, s.title);
    return m;
  }, [sessions]);

  const submitNote = async () => {
    const body = note.trim();
    if (!body) return;
    setSavingNote(true);
    try {
      await api.post(`/binocolo/v1/ma/initiatives/${initiativeId}/cards/${encodeURIComponent(card.companyKey)}/note`, { body });
      setNote('');
      await loadEvents();
      onChanged();
    } catch (e) {
      toast(errorLabel(e), 'error');
    } finally {
      setSavingNote(false);
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
  const lastNote = registry?.notes?.[0];

  return (
    <Drawer
      open
      onClose={onClose}
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
            {lastNote ? (
              <p className={styles.hint} style={{ marginTop: 8 }}>
                Ultima nota: &ldquo;{lastNote.body}&rdquo; · {shortAuthor(lastNote.createdByEmail)} · {relativeDate(lastNote.createdAt)}
              </p>
            ) : null}
          </div>

          <div className={styles.drawerSec}>
            <p className={styles.lab}>Diario attività</p>
            {loadingEvents ? (
              <Skeleton rows={3} />
            ) : (
              <ul className={styles.timeline}>
                {events.map((event) => (
                  <li key={event.id} className={styles.timelineItem}>
                    <div className={styles.timelineDot} />
                    <div className={event.event === 'nota' ? styles.timelineNote : styles.timelineEvent}>
                      {eventLabel(event, sessionMap)}
                    </div>
                    <span className={styles.timelineWho}>
                      {shortAuthor(event.createdByEmail)} · {relativeDate(event.createdAt)}
                    </span>
                  </li>
                ))}
                {events.length === 0 ? <li className={styles.hint}>Nessun evento ancora nel diario.</li> : null}
              </ul>
            )}
          </div>
        </div>

        <div className={styles.composer}>
          <textarea
            className={styles.composerInput}
            placeholder="Aggiungi una nota al diario…"
            value={note}
            onChange={(e) => setNote(e.target.value)}
            maxLength={1000}
            rows={1}
          />
          <Button variant="primary" size="sm" onClick={() => void submitNote()} loading={savingNote} disabled={!note.trim()}>
            Invia
          </Button>
        </div>
      </div>
    </Drawer>
  );
}
