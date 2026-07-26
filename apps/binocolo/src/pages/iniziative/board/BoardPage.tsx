import { useCallback, useMemo, useRef, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import {
  DndContext,
  DragOverlay,
  KeyboardSensor,
  PointerSensor,
  pointerWithin,
  useSensor,
  useSensors,
  type Announcements,
  type DragEndEvent,
  type DragStartEvent,
  type ScreenReaderInstructions,
} from '@dnd-kit/core';
import { Icon, Modal, Skeleton, useToast } from '@mrsmith/ui';
import type { MAInitiativeCardView, MASessionSummary } from '../../../api/types';
import { errorLabel } from '../../ricerche/helpers';
import { ApiError } from '@mrsmith/api-client';
import {
  ACTIVE_STATES,
  CARD_STATES,
  MACROFASI,
  TERMINAL_STATES,
  isTerminalState,
} from '../../../lib/cardStates';
import { DirectCompanyModal } from '../../../components/company/DirectCompanyModal';
import { useBoardData } from './useBoardData';
import { useFlip } from './flip';
import { useBoardKeyboard } from './useBoardKeyboard';
import { FunnelStrip } from './FunnelStrip';
import { StatesView } from './StatesView';
import { MacroView } from './MacroView';
import { BoardTable } from './BoardTable';
import { BoardCard } from './BoardCard';
import { CardDrawer } from './CardDrawer';
import { TerminalModal, RemoveModal, type TerminalTarget } from './TerminalModal';
import styles from './board.module.css';

type Layout = 'macro' | 'states' | 'table';
type Scope = 'active' | 'complete';

function readLS(k: string, fallback: string) {
  try {
    return localStorage.getItem(k) ?? fallback;
  } catch {
    return fallback;
  }
}
function writeLS(k: string, v: string) {
  try {
    localStorage.setItem(k, v);
  } catch {
    /* best-effort */
  }
}
function loadCollapsed(id: string): Set<string> {
  try {
    const raw = localStorage.getItem(`binocolo.iniziative.${id}.collapsedColumns`);
    const parsed = raw ? JSON.parse(raw) : [];
    return Array.isArray(parsed) ? new Set(parsed) : new Set();
  } catch {
    return new Set();
  }
}

export function BoardPage() {
  const { id = '' } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { toast } = useToast();
  const data = useBoardData(id);
  const board = data.board;

  const [layout, setLayoutState] = useState<Layout>(() => readLS(`binocolo.board.${id}.layout`, 'macro') as Layout);
  const [scope, setScopeState] = useState<Scope>(() => readLS(`binocolo.board.${id}.scope`, 'active') as Scope);
  const [collapsed, setCollapsed] = useState<Set<string>>(() => loadCollapsed(id));
  const [query, setQuery] = useState('');
  const [funnelFilter, setFunnelFilter] = useState<string | null>(null);
  const [selectedKey, setSelectedKey] = useState<string | null>(null);
  const [terminal, setTerminal] = useState<{ card: MAInitiativeCardView; target: TerminalTarget } | null>(null);
  const [removeCard, setRemoveCard] = useState<MAInitiativeCardView | null>(null);
  const [addState, setAddState] = useState<string | null>(null);
  const [directOpen, setDirectOpen] = useState(false);
  const [directSubmitting, setDirectSubmitting] = useState(false);
  const [attachOpen, setAttachOpen] = useState(false);
  const [attachable, setAttachable] = useState<MASessionSummary[]>([]);
  const [helpOpen, setHelpOpen] = useState(false);
  const [activeId, setActiveId] = useState<string | null>(null);

  const searchRef = useRef<HTMLInputElement | null>(null);
  const boardRef = useRef<HTMLDivElement | null>(null);

  const setLayout = (l: Layout) => {
    setLayoutState(l);
    writeLS(`binocolo.board.${id}.layout`, l);
  };
  const setScope = (s: Scope) => {
    setScopeState(s);
    writeLS(`binocolo.board.${id}.scope`, s);
  };

  const toggleCollapse = useCallback(
    (state: string) => {
      setCollapsed((prev) => {
        const next = new Set(prev);
        if (next.has(state)) next.delete(state);
        else next.add(state);
        try {
          localStorage.setItem(`binocolo.iniziative.${id}.collapsedColumns`, JSON.stringify([...next]));
        } catch {
          /* best-effort */
        }
        return next;
      });
    },
    [id],
  );

  const cards = board?.cards ?? [];

  // Chiave FLIP: cambio vista + spostamenti optimistic senza drag (1..7, drawer,
  // conferma terminale) + collasso animano. La firma è (companyKey=stato) ordinata:
  // un refetch che non muove nessuna card lascia la firma invariata → nessun FLIP
  // (structural sharing, §8.3); un cambio di stato la fa cambiare → le card volano.
  const flipKey = useMemo(() => {
    const positions = cards
      .map((c) => `${c.companyKey}=${c.state}`)
      .sort()
      .join('|');
    return `${layout}:${scope}:${[...collapsed].sort().join(',')}:${positions}`;
  }, [cards, layout, scope, collapsed]);
  useFlip(boardRef, flipKey);

  // Conteggi per stato (strip funnel) — sul set completo, non filtrato.
  const counts = useMemo(() => {
    const c: Record<string, number> = {};
    for (const card of cards) c[card.state] = (c[card.state] ?? 0) + 1;
    return c;
  }, [cards]);

  // Card filtrate (ricerca + chip funnel), raggruppate per stato.
  const cardsByState = useMemo(() => {
    const q = query.trim().toLowerCase();
    const map = new Map<string, MAInitiativeCardView[]>();
    for (const s of CARD_STATES) map.set(s.key, []);
    for (const card of cards) {
      if (q && !card.companyName.toLowerCase().includes(q)) continue;
      if (funnelFilter && card.state !== funnelFilter) continue;
      (map.get(card.state) ?? []).push(card);
    }
    return map;
  }, [cards, query, funnelFilter]);

  // Filtri attivi ma zero corrispondenze: un solo empty di board, non "Nessuna
  // azienda" ripetuto in ogni colonna.
  const noMatch =
    cards.length > 0 &&
    (query.trim().length > 0 || funnelFilter !== null) &&
    ![...cardsByState.values()].some((a) => a.length > 0);

  const showEsito = scope === 'complete';
  const visibleStates = useMemo(
    () => [...ACTIVE_STATES, ...(showEsito ? TERMINAL_STATES : [])],
    [showEsito],
  );
  const visibleMacrofasi = useMemo(() => MACROFASI.filter((m) => showEsito || m.key !== 'esito'), [showEsito]);

  const selected = selectedKey ? cards.find((c) => c.companyKey === selectedKey) ?? null : null;

  const cohortKeys = useMemo(
    () => CARD_STATES.flatMap((s) => (cardsByState.get(s.key) ?? []).map((c) => c.companyKey)),
    [cardsByState],
  );

  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 8 } }),
    useSensor(KeyboardSensor),
  );

  const openTerminal = useCallback(
    (card: MAInitiativeCardView, target: TerminalTarget) => setTerminal({ card, target }),
    [],
  );

  const moveToState = useCallback(
    (companyKey: string, state: string) => {
      if (isTerminalState(state)) {
        const card = cards.find((c) => c.companyKey === companyKey);
        if (card) setTerminal({ card, target: state as TerminalTarget });
        return;
      }
      data.setState.mutate({ companyKey, state });
    },
    [cards, data.setState],
  );

  useBoardKeyboard({
    searchRef,
    enabled: !selected && !terminal && !removeCard && !directOpen && !attachOpen && !activeId,
    onOpenCard: (companyKey) => setSelectedKey(companyKey),
    onMoveToState: moveToState,
    onTerminal: (companyKey, target) => {
      const card = cards.find((c) => c.companyKey === companyKey);
      if (card) setTerminal({ card, target });
    },
    onToggleCollapse: toggleCollapse,
    onToggleHelp: () => setHelpOpen((v) => !v),
  });

  const onDragStart = (e: DragStartEvent) => setActiveId(String(e.active.id));
  const onDragEnd = (e: DragEndEvent) => {
    setActiveId(null);
    const { active, over } = e;
    if (!over) return;
    const overId = String(over.id);
    if (!overId.startsWith('state:')) return;
    const target = overId.slice('state:'.length);
    const companyKey = String(active.id);
    const card = cards.find((c) => c.companyKey === companyKey);
    if (!card || card.state === target) return;
    if (isTerminalState(target)) {
      setTerminal({ card, target: target as TerminalTarget });
      return;
    }
    data.setState.mutate({ companyKey, state: target });
  };

  const openDossier = useCallback(
    (card: MAInitiativeCardView) => navigate(`/aziende/${encodeURIComponent(card.companyKey)}?iniziativa=${encodeURIComponent(id)}`),
    [navigate, id],
  );

  const onAdd = (state: string) => {
    setAddState(state);
    setDirectOpen(true);
  };

  const openAttach = async () => {
    setAttachOpen(true);
    try {
      setAttachable(await data.listAttachableSessions());
    } catch (e) {
      toast(errorLabel(e), 'error');
    }
  };

  const activeCard = activeId ? cards.find((c) => c.companyKey === activeId) ?? null : null;

  // Annunci aria-live localizzati per il drag da tastiera (companyKey → nome,
  // `state:*` → label stato). Senza, dnd-kit legge gli id grezzi in inglese.
  const dndA11y = useMemo(() => {
    const nameOf = (rawId: unknown) => cards.find((c) => c.companyKey === String(rawId))?.companyName ?? 'azienda';
    const stateOf = (rawId: unknown) => {
      const s = String(rawId).replace(/^state:/, '');
      return CARD_STATES.find((x) => x.key === s)?.label ?? s;
    };
    const announcements: Announcements = {
      onDragStart: ({ active }) => `Presa ${nameOf(active.id)}.`,
      onDragOver: ({ active, over }) => (over ? `${nameOf(active.id)} sopra ${stateOf(over.id)}.` : `${nameOf(active.id)} fuori da ogni colonna.`),
      onDragEnd: ({ active, over }) =>
        over ? `${nameOf(active.id)} rilasciata in ${stateOf(over.id)}.` : `${nameOf(active.id)} rilasciata fuori: nessuna modifica.`,
      onDragCancel: ({ active }) => `Spostamento di ${nameOf(active.id)} annullato.`,
    };
    const screenReaderInstructions: ScreenReaderInstructions = {
      draggable: "Premi Spazio per prendere l'azienda, le frecce per spostarla fra le colonne, Spazio per rilasciare, Esc per annullare.",
    };
    return { announcements, screenReaderInstructions };
  }, [cards]);

  if (data.loading && !board) {
    return (
      <main className={styles.page}>
        <Skeleton rows={6} />
      </main>
    );
  }
  if (!board) {
    return (
      <main className={styles.page}>
        <div role="alert" style={{ color: 'var(--color-danger-hover)' }}>
          <Icon name="triangle-alert" size={18} /> {data.error ? errorLabel(data.error) : 'Iniziativa non disponibile.'}
        </div>
      </main>
    );
  }

  return (
    <main className={styles.page}>
      <div className={styles.board} ref={boardRef}>
        <div className={styles.topbar}>
          <div className={styles.titleBlock}>
            <span className={styles.kicker}>Iniziativa · {board.cards.length} aziende</span>
            <h1 className={styles.title}>{board.initiative.title}</h1>
          </div>
          <div className={styles.viewMatrix}>
            <div className={styles.seg} role="group" aria-label="Ambito della pipeline">
              <button className={scope === 'active' ? styles.segOn : ''} aria-pressed={scope === 'active'} onClick={() => setScope('active')}>
                Attive
              </button>
              <button className={scope === 'complete' ? styles.segOn : ''} aria-pressed={scope === 'complete'} onClick={() => setScope('complete')}>
                Tutte
              </button>
            </div>
            <div className={styles.seg} role="group" aria-label="Disposizione della board">
              <button
                className={`${styles.segIcon} ${layout === 'macro' ? styles.segOn : ''}`}
                aria-pressed={layout === 'macro'}
                onClick={() => setLayout('macro')}
                title="Vista Macrofasi"
                aria-label="Vista Macrofasi"
              >
                <Icon name="layout-grid" size={15} />
              </button>
              <button
                className={`${styles.segIcon} ${layout === 'states' ? styles.segOn : ''}`}
                aria-pressed={layout === 'states'}
                onClick={() => setLayout('states')}
                title="Tutti gli stati"
                aria-label="Tutti gli stati"
              >
                <Icon name="kanban-square" size={15} />
              </button>
              <button
                className={`${styles.segIcon} ${layout === 'table' ? styles.segOn : ''}`}
                aria-pressed={layout === 'table'}
                onClick={() => setLayout('table')}
                title="Vista tabella"
                aria-label="Vista tabella"
              >
                <Icon name="table" size={15} />
              </button>
            </div>
          </div>
          <label className={styles.search}>
            <Icon name="search" size={14} />
            <input ref={searchRef} placeholder="Cerca azienda…" value={query} onChange={(e) => setQuery(e.target.value)} />
            <kbd>/</kbd>
          </label>
        </div>

        <div style={{ display: 'flex', gap: 6, flexWrap: 'wrap', alignItems: 'center', marginBottom: 'var(--space-3)' }}>
          <span className={styles.hint}>Ricerche:</span>
          {board.sessions.map((s) => (
            <button
              key={s.id}
              type="button"
              className={`${styles.chip} ${styles.chipRegInfo}`}
              style={{ cursor: 'pointer', border: 'none' }}
              onClick={() => navigate(`/ricerche/${s.id}`)}
            >
              {s.title}
            </button>
          ))}
          <button
            type="button"
            className={`${styles.chip} ${styles.chipDiretta}`}
            style={{ cursor: 'pointer' }}
            onClick={() => void openAttach()}
            title="Collega una ricerca esistente"
          >
            <Icon name="plus" size={11} /> Ricerca
          </button>
        </div>

        {layout !== 'table' ? (
          <FunnelStrip counts={counts} activeState={funnelFilter} onToggle={(s) => setFunnelFilter((p) => (p === s ? null : s))} showEsito={showEsito} />
        ) : null}

        {layout === 'table' ? (
          <BoardTable cards={cards} onRowClick={(c) => setSelectedKey(c.companyKey)} />
        ) : noMatch ? (
          <div className={styles.colEmpty} style={{ padding: 'var(--space-16) 0' }}>
            Nessuna azienda corrisponde ai filtri.{' '}
            <button
              type="button"
              className={styles.actionLink}
              style={{ border: 'none', background: 'none', cursor: 'pointer' }}
              onClick={() => {
                setQuery('');
                setFunnelFilter(null);
              }}
            >
              Azzera filtri
            </button>
          </div>
        ) : (
          <DndContext
            sensors={sensors}
            accessibility={dndA11y}
            collisionDetection={pointerWithin}
            onDragStart={onDragStart}
            onDragEnd={onDragEnd}
          >
            {layout === 'macro' ? (
              <MacroView
                visibleMacrofasi={visibleMacrofasi}
                cardsByState={cardsByState}
                collapsed={collapsed}
                onToggleCollapse={toggleCollapse}
                onCardClick={(c) => setSelectedKey(c.companyKey)}
                onOpenDossier={openDossier}
                onAdd={onAdd}
                onIsolate={(s) => {
                  setLayout('states');
                  setFunnelFilter(s);
                }}
              />
            ) : (
              <StatesView
                visibleStates={visibleStates}
                cardsByState={cardsByState}
                collapsed={collapsed}
                onToggleCollapse={toggleCollapse}
                onCardClick={(c) => setSelectedKey(c.companyKey)}
                onOpenDossier={openDossier}
                onAdd={onAdd}
              />
            )}
            <DragOverlay dropAnimation={null}>
              {activeCard ? (
                <div className={styles.dragOverlay}>
                  <BoardCard card={activeCard} />
                </div>
              ) : null}
            </DragOverlay>
          </DndContext>
        )}
      </div>

      {selected ? (
        <CardDrawer
          initiativeId={id}
          initiativeTitle={board.initiative.title}
          card={selected}
          cohortKeys={cohortKeys}
          onClose={() => setSelectedKey(null)}
          onChanged={() => void data.refetch()}
          onSetState={(companyKey, state, recontactOn) => data.setState.mutate({ companyKey, state, recontactOn })}
          onOpenTerminal={openTerminal}
          onReopen={(companyKey) => data.reopenCard.mutate(companyKey, { onSuccess: () => toast('Azienda rimessa in lavorazione.', 'success') })}
          onRemove={(card) => setRemoveCard(card)}
          onDeepDive={(companyKey) => data.startDeepDive.mutateAsync(companyKey).then(() => undefined)}
          onOpenDossier={openDossier}
        />
      ) : null}

      {terminal ? (
        <TerminalModal
          card={terminal.card}
          targetState={terminal.target}
          onClose={() => setTerminal(null)}
          onSubmit={(input) => data.closeCard.mutateAsync(input).then(() => undefined)}
        />
      ) : null}

      {removeCard ? (
        <RemoveModal
          card={removeCard}
          onClose={() => setRemoveCard(null)}
          onSubmit={(input) =>
            data.removeCard.mutateAsync(input).then((res) => {
              if (res.ratingCorrectionSkipped) toast('Azienda rimossa. Correzione stella saltata (sessione non operativa).', 'warning');
              else toast('Azienda rimossa dalla lavorazione.', 'success');
              if (selectedKey === input.companyKey) setSelectedKey(null);
            })
          }
        />
      ) : null}

      <DirectCompanyModal
        open={directOpen}
        submitting={directSubmitting}
        onClose={() => {
          setDirectOpen(false);
          setAddState(null);
        }}
        onOpenExisting={(companyKey) => {
          setDirectOpen(false);
          setSelectedKey(companyKey);
        }}
        onSubmit={async (value, setError) => {
          setDirectSubmitting(true);
          try {
            const result = await data.createDirect.mutateAsync({ ...value, initialState: addState ?? undefined });
            setDirectOpen(false);
            setAddState(null);
            toast(result.domainVerification === 'queued' ? 'Azienda aggiunta. Verifica dominio in corso.' : 'Azienda aggiunta.', 'success');
          } catch (err) {
            if (err instanceof ApiError && (err.status === 409 || (err.body as { error?: string })?.error === 'card_already_present')) {
              setError('Azienda già presente in questa iniziativa.', (err.body as { companyKey?: string })?.companyKey);
            } else if (err instanceof ApiError && (err.body as { error?: string })?.error === 'vat_not_found') {
              setError('P.IVA non trovata nel registro. Verifica e riprova.');
            } else {
              setError(errorLabel(err));
            }
          } finally {
            setDirectSubmitting(false);
          }
        }}
      />

      {attachOpen ? (
        <Modal open onClose={() => setAttachOpen(false)} title="Collega una ricerca">
          {attachable.length === 0 ? (
            <p className={styles.hint}>Nessuna ricerca collegabile.</p>
          ) : (
            <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
              {attachable.map((s) => (
                <button
                  key={s.id}
                  type="button"
                  className={styles.kadd}
                  onClick={() =>
                    data.attachSession.mutate(s.id, {
                      onSuccess: () => {
                        setAttachOpen(false);
                        toast('Ricerca collegata.', 'success');
                      },
                      onError: (e) => toast(errorLabel(e), 'error'),
                    })
                  }
                >
                  {s.title}
                </button>
              ))}
            </div>
          )}
        </Modal>
      ) : null}

      {helpOpen ? (
        <Modal open onClose={() => setHelpOpen(false)} title="Scorciatoie">
          <ul style={{ margin: 0, paddingLeft: 18, fontSize: '0.8125rem', lineHeight: 1.9 }}>
            <li><b>/</b> — cerca · <b>j/k</b> — azienda successiva/precedente · <b>Invio</b> — apri</li>
            <li><b>1…7</b> — sposta nello stato · <b>w</b>/<b>x</b> — WON / KO target</li>
            <li><b>c</b> — comprimi stato · <b>Spazio</b> + frecce — trascina da tastiera · <b>Esc</b> — chiudi</li>
          </ul>
        </Modal>
      ) : null}

    </main>
  );
}
