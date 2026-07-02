import { Button, Drawer, Icon, Modal, Skeleton, useToast } from '@mrsmith/ui';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { useApiClient } from '../../api/client';
import type {
  MACardCloseResponse,
  MACardEvent,
  MACardEventListResponse,
  MACardRemoveResponse,
  MAInitiativeBoard,
  MAInitiativeCardView,
  MASessionListResponse,
  MASessionSummary,
} from '../../api/types';
import { dateLabel, errorLabel } from '../ricerche/helpers';
import styles from './Iniziative.module.css';

const STATES: Array<{ key: string; label: string }> = [
  { key: 'da_contattare', label: 'Da contattare' },
  { key: 'contattata', label: 'Contattata' },
  { key: 'in_dialogo', label: 'In dialogo' },
  { key: 'approfondimento', label: 'Approfondimento' },
  { key: 'offerta', label: 'Offerta' },
  { key: 'chiusa', label: 'Chiusa' },
];

const ESITO_LABELS: Record<string, string> = {
  conclusa: 'Conclusa',
  no_go: 'No-go',
  non_idonea: 'Non idonea',
  sfumata: 'Sfumata',
  rimandata: 'Rimandata',
};

const ESITI: Array<{ key: string; label: string; description: string; bridge: boolean }> = [
  { key: 'conclusa', label: 'Conclusa', description: "l'operazione è andata in porto", bridge: false },
  { key: 'no_go', label: 'No-go', description: 'idonea, ma si sceglie di non procedere', bridge: true },
  {
    key: 'non_idonea',
    label: 'Non idonea',
    description: 'alla prova dei fatti non è in profilo per questa iniziativa',
    bridge: false,
  },
  { key: 'sfumata', label: 'Sfumata', description: 'decisione della controparte o di terzi', bridge: false },
  { key: 'rimandata', label: 'Rimandata', description: 'condizioni non mature, da riprendere', bridge: true },
];

const BRIDGE_KINDS: Array<{ key: string; label: string; hint: string }> = [
  { key: 'non_vende', label: 'Non vende', hint: 'il titolare non intende vendere' },
  { key: 'in_trattativa_altrui', label: 'In trattativa con altri', hint: '' },
];

const REGISTRY_LABELS: Record<string, { label: string; kind: 'info' | 'warn' }> = {
  non_vende: { label: 'Non vende', kind: 'warn' },
  in_trattativa_altrui: { label: 'In trattativa con altri', kind: 'warn' },
  da_evitare: { label: 'Da evitare', kind: 'warn' },
  gia_cliente: { label: 'Già cliente', kind: 'info' },
  partner: { label: 'Partner', kind: 'info' },
};

function registryLabel(kind: string) {
  return REGISTRY_LABELS[kind] ?? { label: kind, kind: 'info' as const };
}

function collapseStorageKey(initiativeId: string) {
  return `binocolo.iniziative.${initiativeId}.collapsedColumns`;
}

function loadCollapsed(initiativeId: string): Set<string> {
  try {
    const raw = localStorage.getItem(collapseStorageKey(initiativeId));
    if (!raw) return new Set();
    const parsed = JSON.parse(raw);
    return Array.isArray(parsed) ? new Set(parsed) : new Set();
  } catch {
    return new Set();
  }
}

function saveCollapsed(initiativeId: string, collapsed: Set<string>) {
  try {
    localStorage.setItem(collapseStorageKey(initiativeId), JSON.stringify([...collapsed]));
  } catch {
    // best-effort
  }
}

export function IniziativaBoardPage() {
  const { id } = useParams<{ id: string }>();
  const api = useApiClient();
  const navigate = useNavigate();
  const { toast } = useToast();

  const [board, setBoard] = useState<MAInitiativeBoard | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [view, setView] = useState<'kanban' | 'tabella'>('kanban');
  const [collapsed, setCollapsed] = useState<Set<string>>(new Set());
  const collapsedInitRef = useRef(false);

  const [attachModalOpen, setAttachModalOpen] = useState(false);
  const [availableSessions, setAvailableSessions] = useState<MASessionSummary[]>([]);
  const [attachLoading, setAttachLoading] = useState(false);
  const [attaching, setAttaching] = useState<string | null>(null);

  const [selectedCard, setSelectedCard] = useState<MAInitiativeCardView | null>(null);
  const [closeModalCard, setCloseModalCard] = useState<MAInitiativeCardView | null>(null);
  const [removeModalCard, setRemoveModalCard] = useState<MAInitiativeCardView | null>(null);

  const [tableStateFilter, setTableStateFilter] = useState('');
  const [tableEsitoFilter, setTableEsitoFilter] = useState('');
  const [tableQuery, setTableQuery] = useState('');

  const load = useCallback(async () => {
    if (!id) return;
    try {
      const data = await api.get<MAInitiativeBoard>(`/binocolo/v1/ma/initiatives/${id}`);
      setBoard(data);
      setError(null);
      if (!collapsedInitRef.current) {
        collapsedInitRef.current = true;
        const stored = loadCollapsed(id);
        const next = new Set(stored);
        for (const state of STATES) {
          const count = data.cards.filter((c) => c.state === state.key).length;
          if (count === 0 || state.key === 'chiusa') next.add(state.key);
        }
        setCollapsed(next);
      }
    } catch (err) {
      setError(errorLabel(err));
    } finally {
      setLoading(false);
    }
  }, [api, id]);

  useEffect(() => {
    void load();
  }, [load]);

  const hasWorking = useMemo(() => board?.cards.some((c) => c.dossierStatus === 'working') ?? false, [board]);

  useEffect(() => {
    if (!hasWorking) return;
    const timer = window.setInterval(() => void load(), 5000);
    return () => window.clearInterval(timer);
  }, [hasWorking, load]);

  const toggleCollapsed = (state: string) => {
    if (!id) return;
    setCollapsed((prev) => {
      const next = new Set(prev);
      if (next.has(state)) next.delete(state);
      else next.add(state);
      saveCollapsed(id, next);
      return next;
    });
  };

  const openAttachModal = async () => {
    setAttachModalOpen(true);
    setAttachLoading(true);
    try {
      const data = await api.get<MASessionListResponse>('/binocolo/v1/ma/sessions');
      const anchoredIds = new Set((board?.sessions ?? []).map((s) => s.id));
      setAvailableSessions(data.items.filter((s) => !s.initiativeId && !anchoredIds.has(s.id)));
    } catch (err) {
      toast(errorLabel(err), 'error');
    } finally {
      setAttachLoading(false);
    }
  };

  const attachSession = async (sessionId: string) => {
    if (!id) return;
    setAttaching(sessionId);
    try {
      await api.post(`/binocolo/v1/ma/sessions/${sessionId}/initiative`, { initiativeId: id });
      setAttachModalOpen(false);
      await load();
      toast('Ricerca agganciata.', 'success');
    } catch (err) {
      toast(errorLabel(err), 'error');
    } finally {
      setAttaching(null);
    }
  };

  const setCardState = async (companyKey: string, state: string) => {
    if (!id) return;
    try {
      await api.post(`/binocolo/v1/ma/initiatives/${id}/cards/${encodeURIComponent(companyKey)}/state`, { state });
      await load();
    } catch (err) {
      toast(errorLabel(err), 'error');
    }
  };

  const closeCard = async (companyKey: string, esito: string, note: string, registerFacts: string[]) => {
    if (!id) return;
    const data = await api.post<MACardCloseResponse>(
      `/binocolo/v1/ma/initiatives/${id}/cards/${encodeURIComponent(companyKey)}/close`,
      { esito, note: note || undefined, registerFacts: registerFacts.length > 0 ? registerFacts : undefined },
    );
    await load();
    return data;
  };

  const removeCard = async (companyKey: string, correctRating: boolean, reason: string) => {
    if (!id) return;
    const data = await api.post<MACardRemoveResponse>(
      `/binocolo/v1/ma/initiatives/${id}/cards/${encodeURIComponent(companyKey)}/remove`,
      { correctRating, reason: reason || undefined },
    );
    await load();
    return data;
  };

  const reopenCard = async (companyKey: string) => {
    if (!id) return;
    try {
      await api.post(`/binocolo/v1/ma/initiatives/${id}/cards/${encodeURIComponent(companyKey)}/reopen`, {});
      await load();
      toast('Card riaperta.', 'success');
    } catch (err) {
      toast(errorLabel(err), 'error');
    }
  };

  const startDeepDive = async (companyKey: string) => {
    if (!id) return;
    try {
      await api.post(`/binocolo/v1/ma/initiatives/${id}/cards/${encodeURIComponent(companyKey)}/deep-dive`, {
        acknowledgeCost: true,
      });
      await load();
    } catch (err) {
      toast(errorLabel(err), 'error');
    }
  };

  const openDossier = (card: MAInitiativeCardView) => {
    const query = card.vatCode ? `vat=${encodeURIComponent(card.vatCode)}` : `vat=${encodeURIComponent(card.companyKey)}`;
    navigate(`/azienda?${query}`);
  };

  const cardsByState = useMemo(() => {
    const map = new Map<string, MAInitiativeCardView[]>();
    for (const state of STATES) map.set(state.key, []);
    for (const card of board?.cards ?? []) {
      const list = map.get(card.state) ?? [];
      list.push(card);
      map.set(card.state, list);
    }
    return map;
  }, [board]);

  const filteredTableCards = useMemo(() => {
    const cards = board?.cards ?? [];
    const query = tableQuery.trim().toLowerCase();
    return cards.filter((card) => {
      if (tableStateFilter && card.state !== tableStateFilter) return false;
      if (tableEsitoFilter && card.esito !== tableEsitoFilter) return false;
      if (query && !card.companyName.toLowerCase().includes(query)) return false;
      return true;
    });
  }, [board, tableStateFilter, tableEsitoFilter, tableQuery]);

  if (loading && !board) {
    return (
      <main className={styles.boardPage}>
        <Skeleton rows={6} />
      </main>
    );
  }

  if (error && !board) {
    return (
      <main className={styles.boardPage}>
        <div className={styles.danger} role="alert">
          <Icon name="triangle-alert" size={18} />
          <span>{error}</span>
        </div>
      </main>
    );
  }

  if (!board) return null;

  return (
    <main className={styles.boardPage}>
      <button type="button" className={styles.backLink} onClick={() => navigate('/iniziative')}>
        ← Iniziative
      </button>

      <div className={styles.boardHead}>
        <div className={styles.titleRow}>
          <h1>{board.initiative.title}</h1>
          <div className={styles.viewToggle}>
            <button
              type="button"
              className={`${styles.viewToggleBtn} ${view === 'kanban' ? styles.on : ''}`}
              onClick={() => setView('kanban')}
            >
              Kanban
            </button>
            <button
              type="button"
              className={`${styles.viewToggleBtn} ${view === 'tabella' ? styles.on : ''}`}
              onClick={() => setView('tabella')}
            >
              Tabella
            </button>
          </div>
        </div>
        {board.initiative.description ? <p className={styles.subtitle}>{board.initiative.description}</p> : null}
        <div className={styles.sessChips}>
          <span className={styles.hint}>Ricerche:</span>
          {board.sessions.map((session) => (
            <span key={session.id} className={styles.sessChip}>
              {session.title}
            </span>
          ))}
          <button type="button" className={styles.linkBtn} onClick={() => void openAttachModal()}>
            + Aggancia ricerca
          </button>
        </div>
      </div>

      {view === 'kanban' ? (
        <div className={styles.kanban}>
          {STATES.map((state) => {
            const cards = cardsByState.get(state.key) ?? [];
            const isCollapsed = collapsed.has(state.key);
            if (isCollapsed) {
              return (
                <button
                  type="button"
                  key={state.key}
                  className={styles.kcolCollapsed}
                  onClick={() => toggleCollapsed(state.key)}
                  title={`Espandi ${state.label}`}
                >
                  <span className={styles.kcolCount}>{cards.length}</span>
                  <span className={styles.kcolCollapsedLabel}>{state.label}</span>
                  <Icon name="chevron-right" size={14} />
                </button>
              );
            }
            return (
              <div
                key={state.key}
                className={styles.kcol}
                onDragOver={(e) => e.preventDefault()}
                onDrop={(e) => {
                  e.preventDefault();
                  const companyKey = e.dataTransfer.getData('text/plain');
                  if (!companyKey) return;
                  if (state.key === 'chiusa') {
                    const dropped = (board?.cards ?? []).find((c) => c.companyKey === companyKey);
                    if (dropped) setCloseModalCard(dropped);
                    return;
                  }
                  void setCardState(companyKey, state.key);
                }}
              >
                <div className={styles.kcolHead}>
                  <button type="button" className={styles.kcolHeadBtn} onClick={() => toggleCollapsed(state.key)}>
                    <Icon name="chevron-left" size={14} /> {state.label}
                  </button>
                  <span className={styles.kcolCount}>{cards.length}</span>
                </div>
                <div className={styles.kcolBody}>
                  {cards.map((card) =>
                    state.key === 'chiusa' ? (
                      <div
                        key={card.companyKey}
                        className={`${styles.kcard} ${styles.kcardCompact}`}
                        onClick={() => setSelectedCard(card)}
                      >
                        <span className={styles.kcardName}>{card.companyName}</span>
                        {card.esito ? (
                          <span className={`${styles.badge} ${styles.badgeEsito}`}>{ESITO_LABELS[card.esito] ?? card.esito}</span>
                        ) : null}
                      </div>
                    ) : (
                      <div
                        key={card.companyKey}
                        className={styles.kcard}
                        draggable
                        onDragStart={(e) => e.dataTransfer.setData('text/plain', card.companyKey)}
                        onClick={() => setSelectedCard(card)}
                      >
                        <span className={styles.kcardName}>{card.companyName}</span>
                        <span className={styles.kcardMeta}>
                          {card.province}
                          {(card.registryFacts ?? []).map((kind) => {
                            const info = registryLabel(kind);
                            return (
                              <span key={kind} className={`${styles.badge} ${info.kind === 'warn' ? styles.badgeWarn : styles.badgeInfo}`}>
                                {info.label}
                              </span>
                            );
                          })}
                          {(card.collisions ?? []).map((collision) => (
                            <span key={collision.initiativeId} className={`${styles.badge} ${styles.badgeLav}`}>
                              anche in: {collision.initiativeTitle}
                            </span>
                          ))}
                        </span>
                        <DossierButton
                          card={card}
                          onStart={() => void startDeepDive(card.companyKey)}
                          onOpen={() => openDossier(card)}
                        />
                      </div>
                    ),
                  )}
                </div>
              </div>
            );
          })}
        </div>
      ) : (
        <div className={styles.tableView}>
          <div className={styles.filters}>
            <select className={styles.select} value={tableStateFilter} onChange={(e) => setTableStateFilter(e.target.value)}>
              <option value="">Stato: tutti</option>
              {STATES.map((state) => (
                <option key={state.key} value={state.key}>
                  {state.label}
                </option>
              ))}
            </select>
            <select className={styles.select} value={tableEsitoFilter} onChange={(e) => setTableEsitoFilter(e.target.value)}>
              <option value="">Esito: tutti</option>
              {Object.entries(ESITO_LABELS).map(([key, label]) => (
                <option key={key} value={key}>
                  {label}
                </option>
              ))}
            </select>
            <input
              className={`${styles.input} ${styles.searchInput}`}
              placeholder="Cerca azienda…"
              value={tableQuery}
              onChange={(e) => setTableQuery(e.target.value)}
            />
          </div>
          <table className={styles.table}>
            <thead>
              <tr>
                <th>Azienda</th>
                <th>Prov.</th>
                <th>Stato</th>
                <th>Dossier</th>
                <th>Registro</th>
                <th>Ultima attività</th>
              </tr>
            </thead>
            <tbody>
              {filteredTableCards.map((card) => (
                <tr key={card.companyKey} className={styles.tableRow} onClick={() => setSelectedCard(card)}>
                  <td>
                    <b>{card.companyName}</b>
                  </td>
                  <td>{card.province}</td>
                  <td>
                    {STATES.find((s) => s.key === card.state)?.label ?? card.state}
                    {card.esito ? (
                      <>
                        {' '}
                        <span className={`${styles.badge} ${styles.badgeEsito}`}>{ESITO_LABELS[card.esito] ?? card.esito}</span>
                      </>
                    ) : null}
                  </td>
                  <td>
                    {card.dossierStatus === 'ready' ? (
                      <span className={styles.linkBtn}>Apri dossier</span>
                    ) : card.dossierStatus === 'working' ? (
                      <span className={`${styles.badge} ${styles.badgeEsito}`}>in corso…</span>
                    ) : (
                      <span className={styles.linkBtn}>Avvia analisi</span>
                    )}
                  </td>
                  <td>
                    {(card.registryFacts ?? []).length > 0
                      ? card.registryFacts!.map((kind) => registryLabel(kind).label).join(', ')
                      : '—'}
                  </td>
                  <td className={styles.hint}>{dateLabel(card.updatedAt)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      <Modal open={attachModalOpen} onClose={() => setAttachModalOpen(false)} title="Aggancia ricerca">
        {attachLoading ? (
          <Skeleton rows={3} />
        ) : availableSessions.length === 0 ? (
          <p className={styles.hint}>Nessuna ricerca disponibile da agganciare.</p>
        ) : (
          <div className={styles.list}>
            {availableSessions.map((session) => (
              <div key={session.id} className={styles.card}>
                <div className={styles.cardTop}>
                  <span className={styles.cardTitle}>{session.title}</span>
                  <Button
                    variant="secondary"
                    onClick={() => void attachSession(session.id)}
                    loading={attaching === session.id}
                  >
                    Aggancia
                  </Button>
                </div>
              </div>
            ))}
          </div>
        )}
      </Modal>

      {selectedCard ? (
        <CardDrawer
          initiativeId={id ?? ''}
          card={selectedCard}
          onClose={() => setSelectedCard(null)}
          onChanged={() => void load()}
          onSetState={setCardState}
          onDeepDive={startDeepDive}
          onOpenDossier={openDossier}
          onOpenCloseModal={(card) => setCloseModalCard(card)}
          onOpenRemoveModal={(card) => setRemoveModalCard(card)}
          onReopen={reopenCard}
        />
      ) : null}

      {closeModalCard ? (
        <CloseCardModal
          card={closeModalCard}
          onClose={() => setCloseModalCard(null)}
          onSubmit={closeCard}
          onDone={(result) => {
            setCloseModalCard(null);
            if (selectedCard?.companyKey === closeModalCard.companyKey) setSelectedCard(null);
            if (result?.skippedFacts && result.skippedFacts.length > 0) {
              toast('Card chiusa. Alcuni fatti erano già registrati.', 'success');
            } else {
              toast('Card chiusa.', 'success');
            }
          }}
        />
      ) : null}

      {removeModalCard ? (
        <RemoveCardModal
          card={removeModalCard}
          onClose={() => setRemoveModalCard(null)}
          onSubmit={removeCard}
          onDone={(result) => {
            setRemoveModalCard(null);
            if (selectedCard?.companyKey === removeModalCard.companyKey) setSelectedCard(null);
            if (result?.ratingCorrectionSkipped) {
              toast('Card rimossa. La ricerca di provenienza non è più operativa: correzione stella saltata.', 'warning');
            } else {
              toast('Card rimossa.', 'success');
            }
          }}
        />
      ) : null}
    </main>
  );
}

function DossierButton({
  card,
  onStart,
  onOpen,
}: {
  card: MAInitiativeCardView;
  onStart: () => void;
  onOpen: () => void;
}) {
  const [confirming, setConfirming] = useState(false);

  if (card.dossierStatus === 'ready') {
    return (
      <span
        className={styles.dossierBtn}
        onClick={(e) => {
          e.stopPropagation();
          onOpen();
        }}
      >
        <Button variant="secondary" size="sm">
          Apri dossier
        </Button>
      </span>
    );
  }

  if (card.dossierStatus === 'working') {
    return (
      <span className={styles.dossierBtn} onClick={(e) => e.stopPropagation()}>
        <Button variant="secondary" size="sm" disabled className={styles.pulse}>
          Analisi in corso…
        </Button>
      </span>
    );
  }

  if (confirming) {
    return (
      <span className={styles.actionsRow} onClick={(e) => e.stopPropagation()}>
        <Button
          variant="secondary"
          size="sm"
          onClick={() => {
            setConfirming(false);
            onStart();
          }}
        >
          Conferma
        </Button>
        <Button variant="secondary" size="sm" onClick={() => setConfirming(false)}>
          Annulla
        </Button>
      </span>
    );
  }

  return (
    <span className={styles.dossierBtn} onClick={(e) => e.stopPropagation()}>
      <Button variant="secondary" size="sm" onClick={() => setConfirming(true)}>
        Avvia analisi completa
      </Button>
    </span>
  );
}

function CardDrawer({
  initiativeId,
  card,
  onClose,
  onChanged,
  onSetState,
  onDeepDive,
  onOpenDossier,
  onOpenCloseModal,
  onOpenRemoveModal,
  onReopen,
}: {
  initiativeId: string;
  card: MAInitiativeCardView;
  onClose: () => void;
  onChanged: () => void;
  onSetState: (companyKey: string, state: string) => Promise<void>;
  onDeepDive: (companyKey: string) => Promise<void>;
  onOpenDossier: (card: MAInitiativeCardView) => void;
  onOpenCloseModal: (card: MAInitiativeCardView) => void;
  onOpenRemoveModal: (card: MAInitiativeCardView) => void;
  onReopen: (companyKey: string) => Promise<void>;
}) {
  const api = useApiClient();
  const { toast } = useToast();
  const [events, setEvents] = useState<MACardEvent[]>([]);
  const [loadingEvents, setLoadingEvents] = useState(true);
  const [note, setNote] = useState('');
  const [savingNote, setSavingNote] = useState(false);
  const [confirmingDeepDive, setConfirmingDeepDive] = useState(false);

  const loadEvents = useCallback(async () => {
    setLoadingEvents(true);
    try {
      const data = await api.get<MACardEventListResponse>(
        `/binocolo/v1/ma/initiatives/${initiativeId}/cards/${encodeURIComponent(card.companyKey)}/events`,
      );
      setEvents(data.items);
    } catch (err) {
      toast(errorLabel(err), 'error');
    } finally {
      setLoadingEvents(false);
    }
  }, [api, initiativeId, card.companyKey, toast]);

  useEffect(() => {
    void loadEvents();
  }, [loadEvents]);

  const submitNote = async () => {
    const body = note.trim();
    if (!body) return;
    setSavingNote(true);
    try {
      await api.post(`/binocolo/v1/ma/initiatives/${initiativeId}/cards/${encodeURIComponent(card.companyKey)}/note`, {
        body,
      });
      setNote('');
      await loadEvents();
      onChanged();
    } catch (err) {
      toast(errorLabel(err), 'error');
    } finally {
      setSavingNote(false);
    }
  };

  const activeStates = STATES.filter((s) => s.key !== 'chiusa');

  return (
    <Drawer open onClose={onClose} title={card.companyName} size="lg">
      <div className={styles.drawerSub}>
        <span>
          {card.vatCode ? `P.IVA ${card.vatCode}` : ''}
          {card.province ? ` · ${card.province}` : ''}
        </span>
        <button type="button" className={styles.linkBtn} onClick={() => onOpenDossier(card)}>
          Apri dossier azienda ↗
        </button>
      </div>

      {(card.collisions ?? []).length > 0 ? (
        <div className={styles.drawerSec}>
          {card.collisions!.map((collision) => (
            <span key={collision.initiativeId} className={`${styles.badge} ${styles.badgeLav}`}>
              In lavorazione anche in: {collision.initiativeTitle}
            </span>
          ))}
        </div>
      ) : null}

      <div className={styles.drawerSec}>
        <p className={styles.lab}>Stato</p>
        {card.state === 'chiusa' || card.state === 'rimossa' ? (
          <div className={styles.actionsRow}>
            {card.state === 'chiusa' && card.esito ? (
              <span className={`${styles.badge} ${styles.badgeEsito}`}>{ESITO_LABELS[card.esito] ?? card.esito}</span>
            ) : null}
            {card.state === 'rimossa' ? <span className={styles.hint}>Rimossa dalla lavorazione.</span> : null}
            <Button variant="secondary" size="sm" onClick={() => void onReopen(card.companyKey)}>
              Riapri
            </Button>
          </div>
        ) : (
          <div className={styles.stationSel}>
            {activeStates.map((state) => (
              <button
                key={state.key}
                type="button"
                className={`${styles.stationBtn} ${card.state === state.key ? styles.on : ''}`}
                onClick={() => void onSetState(card.companyKey, state.key)}
              >
                {state.label}
              </button>
            ))}
            <button type="button" className={styles.stationBtn} onClick={() => onOpenCloseModal(card)}>
              Chiusa…
            </button>
          </div>
        )}
        {card.state !== 'chiusa' && card.state !== 'rimossa' ? (
          <button type="button" className={styles.linkBtn} onClick={() => onOpenRemoveModal(card)}>
            Rimuovi dalla lavorazione
          </button>
        ) : null}
      </div>

      {(card.provenances ?? []).length > 0 ? (
        <div className={styles.drawerSec}>
          <p className={styles.lab}>Provenienze</p>
          {card.provenances!.map((prov) => (
            <div key={`${prov.sessionId}-${prov.ratedAt}`} className={styles.provRow}>
              <span className={styles.provSess}>{prov.sessionTitle}</span>
              <span className={styles.stars}>
                {'★'.repeat(Math.max(prov.rating, 0))}
                <span className={styles.starsOff}>{'☆'.repeat(Math.max(3 - prov.rating, 0))}</span>
              </span>
              {prov.scoreAtRating != null ? <span className={styles.provScore}>score {prov.scoreAtRating}</span> : null}
            </div>
          ))}
        </div>
      ) : null}

      <div className={styles.drawerSec}>
        <p className={styles.lab}>Scheda azienda</p>
        {(card.registryFacts ?? []).length > 0 ? (
          <div className={styles.kcardMeta}>
            {card.registryFacts!.map((kind) => {
              const info = registryLabel(kind);
              return (
                <span key={kind} className={`${styles.badge} ${info.kind === 'warn' ? styles.badgeWarn : styles.badgeInfo}`}>
                  {info.label}
                </span>
              );
            })}
          </div>
        ) : (
          <p className={styles.hint}>Nessun fatto registrato.</p>
        )}
        <button type="button" className={styles.linkBtn} onClick={() => onOpenDossier(card)}>
          Gestione dal dossier ↗
        </button>
      </div>

      <div className={styles.drawerSec}>
        <p className={styles.lab}>Analisi completa</p>
        <div className={styles.actionsRow}>
          {card.dossierStatus === 'ready' ? (
            <Button variant="secondary" size="sm" onClick={() => onOpenDossier(card)}>
              Apri dossier
            </Button>
          ) : card.dossierStatus === 'working' ? (
            <Button variant="secondary" size="sm" disabled className={styles.pulse}>
              Analisi in corso…
            </Button>
          ) : confirmingDeepDive ? (
            <>
              <Button
                variant="secondary"
                size="sm"
                onClick={() => {
                  setConfirmingDeepDive(false);
                  void onDeepDive(card.companyKey);
                }}
              >
                Conferma
              </Button>
              <Button variant="secondary" size="sm" onClick={() => setConfirmingDeepDive(false)}>
                Annulla
              </Button>
            </>
          ) : (
            <Button variant="secondary" size="sm" onClick={() => setConfirmingDeepDive(true)}>
              Avvia analisi completa
            </Button>
          )}
          <span className={styles.hint}>Recupero dei dati completi e analisi. Richiede tempo; il dossier resta disponibile qui.</span>
        </div>
      </div>

      <div className={styles.drawerSec}>
        <p className={styles.lab}>Diario</p>
        {loadingEvents ? (
          <Skeleton rows={3} />
        ) : (
          <ul className={styles.timeline}>
            {events.map((event) => (
              <li key={event.id} className={styles.timelineItem}>
                <span>{eventLabel(event)}</span>
                <span className={styles.timelineWho}>
                  {event.createdByEmail ?? ''} · {dateLabel(event.createdAt)}
                </span>
              </li>
            ))}
            {events.length === 0 ? <li className={styles.hint}>Nessun evento ancora.</li> : null}
          </ul>
        )}
        <div className={styles.composer}>
          <input
            className={styles.composerInput}
            placeholder="Aggiungi nota al diario…"
            value={note}
            onChange={(e) => setNote(e.target.value)}
            maxLength={1000}
          />
          <Button variant="secondary" size="sm" onClick={() => void submitNote()} loading={savingNote}>
            Aggiungi
          </Button>
        </div>
      </div>
    </Drawer>
  );
}

function CloseCardModal({
  card,
  onClose,
  onSubmit,
  onDone,
}: {
  card: MAInitiativeCardView;
  onClose: () => void;
  onSubmit: (companyKey: string, esito: string, note: string, registerFacts: string[]) => Promise<MACardCloseResponse | undefined>;
  onDone: (result: MACardCloseResponse | undefined) => void;
}) {
  const { toast } = useToast();
  const [esito, setEsito] = useState('');
  const [note, setNote] = useState('');
  const [facts, setFacts] = useState<Set<string>>(new Set());
  const [saving, setSaving] = useState(false);

  const selected = ESITI.find((e) => e.key === esito);
  const showBridge = selected?.bridge ?? false;

  const toggleFact = (kind: string) => {
    setFacts((prev) => {
      const next = new Set(prev);
      if (next.has(kind)) next.delete(kind);
      else next.add(kind);
      return next;
    });
  };

  const submit = async () => {
    if (!esito) return;
    setSaving(true);
    try {
      const result = await onSubmit(card.companyKey, esito, note.trim(), showBridge ? [...facts] : []);
      onDone(result);
    } catch (err) {
      toast(errorLabel(err), 'error');
    } finally {
      setSaving(false);
    }
  };

  return (
    <Modal open onClose={onClose} title={`Chiudi lavorazione — ${card.companyName}`}>
      <p className={styles.modalSub}>L&apos;esito resta nel diario e alimenta la calibrazione del punteggio.</p>
      <div className={styles.radios}>
        {ESITI.map((e) => (
          <label key={e.key} className={`${styles.radioRow} ${esito === e.key ? styles.on : ''}`}>
            <input type="radio" name="esito" value={e.key} checked={esito === e.key} onChange={() => setEsito(e.key)} />
            <span className={styles.radioText}>
              <b>{e.label}</b>
              <span className={styles.radioSmall}>{e.description}</span>
            </span>
          </label>
        ))}
      </div>
      {showBridge ? (
        <div className={styles.bridge}>
          <p className={styles.lab}>
            Registra anche nel registro azienda <span className={styles.hint}>(vale per tutte le iniziative)</span>
          </p>
          {BRIDGE_KINDS.map((k) => (
            <label key={k.key} className={styles.checkrow}>
              <input type="checkbox" checked={facts.has(k.key)} onChange={() => toggleFact(k.key)} />
              <span>
                <b>{k.label}</b>
                {k.hint ? ` — ${k.hint}` : ''}
              </span>
            </label>
          ))}
        </div>
      ) : null}
      <div className={styles.field}>
        <label>Nota</label>
        <input
          className={styles.input}
          placeholder="Motivo in una riga…"
          value={note}
          onChange={(e) => setNote(e.target.value)}
          maxLength={500}
        />
      </div>
      <div className={styles.modalActions}>
        <Button variant="secondary" onClick={onClose}>
          Annulla
        </Button>
        <Button variant="primary" onClick={() => void submit()} loading={saving} disabled={!esito}>
          Chiudi card
        </Button>
      </div>
    </Modal>
  );
}

function RemoveCardModal({
  card,
  onClose,
  onSubmit,
  onDone,
}: {
  card: MAInitiativeCardView;
  onClose: () => void;
  onSubmit: (companyKey: string, correctRating: boolean, reason: string) => Promise<MACardRemoveResponse | undefined>;
  onDone: (result: MACardRemoveResponse | undefined) => void;
}) {
  const { toast } = useToast();
  const [correctRating, setCorrectRating] = useState(false);
  const [reason, setReason] = useState('');
  const [saving, setSaving] = useState(false);

  const submit = async () => {
    setSaving(true);
    try {
      const result = await onSubmit(card.companyKey, correctRating, reason.trim());
      onDone(result);
    } catch (err) {
      toast(errorLabel(err), 'error');
    } finally {
      setSaving(false);
    }
  };

  return (
    <Modal open onClose={onClose} title={`Rimuovi dalla lavorazione — ${card.companyName}`}>
      <p className={styles.modalSub}>
        La rimozione è per le card create per errore: <b>nessun verdetto</b> viene registrato.
      </p>
      <div className={styles.bridge}>
        <label className={styles.checkrow}>
          <input type="checkbox" checked={correctRating} onChange={(e) => setCorrectRating(e.target.checked)} />
          <span>
            Correggi anche la valutazione nella ricerca di provenienza: <b>escludi (−1)</b>
          </span>
        </label>
      </div>
      <div className={styles.field}>
        <label>Motivo dell&apos;esclusione</label>
        <input
          className={styles.input}
          placeholder="Es. non è un MSP, rivende solo licenze…"
          value={reason}
          onChange={(e) => setReason(e.target.value)}
          maxLength={500}
        />
      </div>
      <div className={styles.modalActions}>
        <Button variant="secondary" onClick={onClose}>
          Annulla
        </Button>
        <Button variant="danger" onClick={() => void submit()} loading={saving}>
          Rimuovi
        </Button>
      </div>
    </Modal>
  );
}

function eventLabel(event: MACardEvent): string {
  switch (event.event) {
    case 'card_creata':
      return `Card creata${event.note ? ` — ${event.note}` : ''}`;
    case 'card_riaperta':
      return 'Card riaperta';
    case 'card_rimossa':
      return 'Card rimossa';
    case 'stato':
      return `Stato aggiornato${event.note ? ` — ${event.note}` : ''}`;
    case 'chiusura':
      return `Chiusura${event.note ? ` — ${event.note}` : ''}`;
    case 'nota':
      return event.note ?? 'Nota';
    case 'contattato':
      return `Contattata${event.note ? ` — ${event.note}` : ''}`;
    case 'buon_lead':
      return `Buon lead${event.note ? ` — ${event.note}` : ''}`;
    case 'no_go':
      return `No-go${event.note ? ` — ${event.note}` : ''}`;
    default:
      return event.note ? `${event.event} — ${event.note}` : event.event;
  }
}
