import { Button, Drawer, Icon, Modal, Skeleton, useToast } from '@mrsmith/ui';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import type { ReactNode } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { useApiClient } from '../../api/client';
import type {
  MACardCloseResponse,
  MACardEvent,
  MACardEventListResponse,
  MACardRemoveResponse,
  MAInitiativeBoard,
  MAInitiativeCardView,
  MACompanyRegistry,
  MASessionListResponse,
  MASessionSummary,
} from '../../api/types';
import { dateLabel, relativeDate, shortAuthor, errorLabel } from '../ricerche/helpers';
import styles from './Iniziative.module.css';

const STATES: Array<{ key: string; label: string }> = [
  { key: 'approfondimento', label: 'Approfondimento' },
  { key: 'da_contattare', label: 'Da contattare' },
  { key: 'contattata', label: 'Contattata' },
  { key: 'in_dialogo', label: 'In dialogo' },
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

function stateDisplayLabel(key: string): string {
  return STATES.find((s) => s.key === key)?.label ?? key;
}

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

const STATE_COLORS: Record<string, string> = {
  da_contattare: styles.statusGrey ?? '',
  contattata: styles.statusBlue ?? '',
  in_dialogo: styles.statusIndigo ?? '',
  approfondimento: styles.statusPurple ?? '',
  offerta: styles.statusOrange ?? '',
  chiusa: styles.statusGreen ?? '',
  rimossa: styles.statusMuted ?? '',
};

// Etichetta di stato per la vista tabella. `rimossa` non è una colonna kanban
// (STATES) ma compare tra le righe: gli serve un'etichetta leggibile e una
// pill distinta ("non più in lavorazione"), coerente col wireframe S3 che
// prescrive pill di stato colorate anche per le card uscite di scena.
function stateTableLabel(key: string): string {
  if (key === 'rimossa') return 'Rimossa';
  return STATES.find((s) => s.key === key)?.label ?? key;
}

function stateBadgeClass(state: string) {
  return `${styles.statusBadge} ${STATE_COLORS[state] ?? styles.statusGrey ?? ''}`;
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

  // Mantiene selectedCard sincronizzato con i dati freschi della board
  // (es. dopo cambio stato, chiusura, riapertura nel drawer).
  useEffect(() => {
    if (!selectedCard || !board) return;
    const updated = board.cards.find((c) => c.companyKey === selectedCard.companyKey);
    if (updated && updated !== selectedCard) {
      setSelectedCard(updated);
    }
  }, [board, selectedCard]);
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
        setCollapsed(new Set(stored));
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
    navigate(`/iniziative/${id}/dossier/${encodeURIComponent(card.companyKey)}`);
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
          <h1>{board.initiative.title} <span className={styles.hint}>({board.cards.length})</span></h1>
          <div className={styles.viewToggle} role="tablist" aria-label="Vista">
            <button
              type="button"
              role="tab"
              aria-selected={view === 'kanban'}
              className={`${styles.viewToggleBtn} ${view === 'kanban' ? styles.on : ''}`}
              onClick={() => setView('kanban')}
            >
              <Icon name="list" size={14} style={{ marginRight: 6 }} />
              Kanban
            </button>
            <button
              type="button"
              role="tab"
              aria-selected={view === 'tabella'}
              className={`${styles.viewToggleBtn} ${view === 'tabella' ? styles.on : ''}`}
              onClick={() => setView('tabella')}
            >
              <Icon name="file-text" size={14} style={{ marginRight: 6 }} />
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
                        <span className={styles.kcardName} title={card.companyName}>{card.companyName}</span>
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
                        <div className={styles.kcardHead}>
                          <span className={styles.kcardName} title={card.companyName}>{card.companyName}</span>
                          <Icon name="more-vertical" size={14} className={styles.hint} />
                        </div>
                        <div className={styles.kcardMeta}>
                          <span className={styles.provinceBadge}>{card.province}</span>
                          {(card.registryFacts ?? []).map((kind) => {
                            const info = registryLabel(kind);
                            return (
                              <span key={kind} className={`${styles.badge} ${info.kind === 'warn' ? styles.badgeWarn : styles.badgeInfo}`} title={info.label}>
                                <Icon name={info.kind === 'warn' ? 'triangle-alert' : 'info'} size={10} style={{ marginRight: 4 }} />
                                {info.label}
                              </span>
                            );
                          })}
                          {(card.collisions ?? []).map((collision) => (
                            <span key={collision.initiativeId} className={`${styles.badge} ${styles.badgeLav}`} title={`Anche in: ${collision.initiativeTitle}`}>
                              <Icon name="git-branch" size={10} style={{ marginRight: 4 }} />
                              {collision.initiativeTitle}
                            </span>
                          ))}
                        </div>
                        <div className={styles.kcardFoot}>
                          <DossierButton
                            card={card}
                            onStart={() => void startDeepDive(card.companyKey)}
                            onOpen={() => openDossier(card)}
                          />
                        </div>
                      </div>
                    ),
                  )}
                </div>
              </div>
            );
          })}
        </div>
      ) : (
        <div className={styles.tableContainer}>
          <div className={styles.filters}>
            <div className={styles.filterGroup}>
              <Icon name="filter" size={14} className={styles.hint} />
              <select className={styles.select} value={tableStateFilter} onChange={(e) => setTableStateFilter(e.target.value)}>
                <option value="">Tutti gli stati</option>
                {STATES.map((state) => (
                  <option key={state.key} value={state.key}>
                    {state.label}
                  </option>
                ))}
              </select>
              <select className={styles.select} value={tableEsitoFilter} onChange={(e) => setTableEsitoFilter(e.target.value)}>
                <option value="">Tutti gli esiti</option>
                {Object.entries(ESITO_LABELS).map(([key, label]) => (
                  <option key={key} value={key}>
                    {label}
                  </option>
                ))}
              </select>
            </div>
            <div className={styles.searchWrapper}>
              <Icon name="search" size={14} className={styles.searchIcon} />
              <input
                className={`${styles.input} ${styles.searchInput}`}
                placeholder="Cerca per nome azienda…"
                value={tableQuery}
                onChange={(e) => setTableQuery(e.target.value)}
              />
            </div>
          </div>
          <table className={styles.table}>
            <thead>
              <tr>
                <th>Azienda</th>
                <th>Prov.</th>
                <th>Stato Lavorazione</th>
                <th>Dossier & Analisi</th>
                <th>Registro Fatti</th>
                <th>Aggiornamento</th>
              </tr>
            </thead>
            <tbody>
              {filteredTableCards.length === 0 ? (
                <tr>
                  <td colSpan={6} style={{ textAlign: 'center', padding: '48px 0' }}>
                    <p className={styles.hint}>Nessuna azienda corrisponde ai filtri impostati.</p>
                  </td>
                </tr>
              ) : (
                filteredTableCards.map((card) => (
                  <tr key={card.companyKey} className={styles.tableRow} onClick={() => setSelectedCard(card)}>
                    <td className={styles.cellMain}>
                      <span className={styles.companyNameText} title={card.companyName}>{card.companyName}</span>
                    </td>
                    <td>
                      <span className={styles.provinceBadge}>{card.province}</span>
                    </td>
                    <td>
                      <div className={styles.statusCell}>
                        <span className={stateBadgeClass(card.state)}>
                          {stateTableLabel(card.state)}
                        </span>
                        {card.esito ? (
                          <span className={`${styles.badge} ${styles.badgeEsito}`}>
                            <Icon name="check" size={10} style={{ marginRight: 4 }} />
                            {ESITO_LABELS[card.esito] ?? card.esito}
                          </span>
                        ) : null}
                      </div>
                    </td>
                    <td>
                      <DossierButton
                        card={card}
                        onStart={() => void startDeepDive(card.companyKey)}
                        onOpen={() => openDossier(card)}
                      />
                    </td>
                    <td>
                      <div className={styles.registryCell}>
                        {(card.registryFacts ?? []).length > 0 ? (
                          card.registryFacts!.map((kind) => {
                            const info = registryLabel(kind);
                            return (
                              <span key={kind} className={`${styles.badge} ${info.kind === 'warn' ? styles.badgeWarn : styles.badgeInfo}`} title={info.label}>
                                <Icon name={info.kind === 'warn' ? 'triangle-alert' : 'info'} size={10} />
                              </span>
                            );
                          })
                        ) : (
                          <span className={styles.dash}>—</span>
                        )}
                      </div>
                    </td>
                    <td>
                      <span className={styles.dateCell}>
                        <Icon name="clock" size={12} style={{ marginRight: 6 }} />
                        {card.lastEvent || dateLabel(card.updatedAt)}
                      </span>
                    </td>
                  </tr>
                ))
              )}
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
          sessions={board.sessions}
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
  sessions,
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
  sessions: MASessionSummary[];
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

  const sessionMap = useMemo(() => {
    const m = new Map<string, string>();
    for (const s of sessions) m.set(s.id, s.title);
    return m;
  }, [sessions]);

  const [registry, setRegistry] = useState<MACompanyRegistry | null>(null);
  useEffect(() => {
    api
      .get<MACompanyRegistry>(`/binocolo/v1/ma/companies/${encodeURIComponent(card.companyKey)}/registry`)
      .then(setRegistry)
      .catch(() => setRegistry(null));
  }, [api, card.companyKey]);

  const activeStates = STATES.filter((s) => s.key !== 'chiusa');

  return (
    <Drawer
      open
      onClose={onClose}
      title={card.companyName}
      subtitle={
        <div className={styles.drawerMeta}>
          {card.vatCode && <span>P.IVA {card.vatCode}</span>}
          {card.province && <span> · {card.province}</span>}
        </div>
      }
      headerExtra={
        <div className={styles.drawerActions}>
          <Button variant="secondary" size="sm" onClick={() => onOpenDossier(card)}>
            Dossier ↗
          </Button>
          {card.state === 'chiusa' ? (
            <Button variant="secondary" size="sm" onClick={() => void onReopen(card.companyKey)}>
              Riapri
            </Button>
          ) : null}
          {card.state !== 'chiusa' && card.state !== 'rimossa' && (
            <Button variant="secondary" size="sm" onClick={() => onOpenRemoveModal(card)}>
              <Icon name="trash" size={14} />
            </Button>
          )}
        </div>
      }
      size="xl"
    >
      <div className={styles.drawerBody}>
        <div className={styles.drawerScrollArea}>
          <div className={styles.pipeline}>
            {activeStates.map((state, idx) => (
              <div key={state.key} className={styles.pipelineStep}>
                <button
                  type="button"
                  className={`${styles.stationBtn} ${card.state === state.key ? styles.on : ''}`}
                  onClick={() => void onSetState(card.companyKey, state.key)}
                >
                  <span className={styles.stepCircle}>{idx + 1}</span>
                  <span className={styles.stepLabel}>{state.label}</span>
                </button>
                {idx < activeStates.length - 1 && <div className={styles.stepConnector} />}
              </div>
            ))}
            <div className={styles.pipelineStep}>
              <button type="button" className={styles.stationBtn} onClick={() => onOpenCloseModal(card)}>
                <span className={styles.stepCircle}>6</span>
                <span className={styles.stepLabel}>Chiudi…</span>
              </button>
            </div>
          </div>

          {(card.collisions ?? []).length > 0 && (
            <div className={styles.drawerSec} style={{ borderTop: 0, paddingTop: 0 }}>
              {card.collisions!.map((collision) => (
                <span key={collision.initiativeId} className={`${styles.badge} ${styles.badgeLav}`}>
                  In lavorazione anche in: {collision.initiativeTitle}
                </span>
              ))}
            </div>
          )}

          <Accordion title="Provenienze e Dettagli" initialOpen={false}>
            {(card.provenances ?? []).length > 0 && (
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
            )}

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
              {(() => {
                const lastNote = registry?.notes?.[0];
                if (!lastNote) return null;
                return (
                  <p className={styles.hint} style={{ marginTop: 8 }}>
                    Ultima nota: &ldquo;{lastNote.body}&rdquo; · {shortAuthor(lastNote.createdByEmail)} &middot; {relativeDate(lastNote.createdAt)}
                  </p>
                );
              })()}
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
              </div>
            </div>
          </Accordion>

          <div className={styles.drawerSec}>
            <p className={styles.lab}>Diario Attività</p>
            {loadingEvents ? (
              <Skeleton rows={3} />
            ) : (
              <ul className={styles.timeline}>
                {events.map((event) => {
                  const isNote = event.event === 'nota';
                  return (
                    <li key={event.id} className={styles.timelineItem}>
                      <div className={styles.timelineDot} />
                      <div className={isNote ? styles.timelineNote : styles.timelineEvent}>{eventLabel(event, sessionMap)}</div>
                      <span className={styles.timelineWho}>
                        {shortAuthor(event.createdByEmail)} · {relativeDate(event.createdAt)}
                      </span>
                    </li>
                  );
                })}
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

function Accordion({ title, children, initialOpen = false }: { title: string; children: ReactNode; initialOpen?: boolean }) {
  const [open, setOpen] = useState(initialOpen);
  return (
    <div className={styles.accordion}>
      <div className={styles.accordionHead} onClick={() => setOpen(!open)}>
        <span className={styles.lab} style={{ margin: 0 }}>{title}</span>
        <Icon name={open ? 'chevron-up' : 'chevron-down'} size={14} />
      </div>
      {open && <div className={styles.accordionBody}>{children}</div>}
    </div>
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

function eventLabel(event: MACardEvent, sessionMap?: Map<string, string>): string {
  const p = event.payload as Record<string, unknown> | undefined;
  switch (event.event) {
    case 'stato': {
      const from = p && typeof p.from === 'string' ? stateDisplayLabel(p.from) : null;
      const to = p && typeof p.to === 'string' ? stateDisplayLabel(p.to) : null;
      if (from && to) return `${from} → ${to}`;
      return `Stato aggiornato${event.note ? ` — ${event.note}` : ''}`;
    }
    case 'chiusura': {
      const esito = p && typeof p.esito === 'string' ? (ESITO_LABELS[p.esito] ?? p.esito) : null;
      const base = esito ? `Chiusura — ${esito}` : 'Chiusura';
      return event.note ? `${base}: ${event.note}` : base;
    }
    case 'card_creata': {
      const rating = p && typeof p.rating === 'number' ? p.rating : 0;
      const stars = rating > 0 ? ' ★'.repeat(Math.min(rating, 3)) : '';
      const sessionId = p && typeof p.sessionId === 'string' ? p.sessionId : null;
      const sessionTitle = sessionId && sessionMap ? (sessionMap.get(sessionId) ?? null) : null;
      const provenance = sessionTitle ? ` da ${sessionTitle}` : '';
      return `Card creata${provenance}${stars}${event.note ? ` — ${event.note}` : ''}`;
    }
    case 'card_riaperta':
      return 'Card riaperta';
    case 'card_rimossa':
      return 'Card rimossa';
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
