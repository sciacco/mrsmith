import { useMemo, useState, type CSSProperties } from 'react';
import { useNavigate } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import { Button, Drawer, Icon, MultiSelect, Skeleton, Tooltip } from '@mrsmith/ui';
import { useApiClient } from '../../api/client';
import type { MAInitiativeCardView, MAPipelineCardView, MAPipelineResponse } from '../../api/types';
import { maTagQueryPolicy } from '../../hooks/useMATags';
import { ACTIVE_STATES, CARD_STATES, MACROFASI, TERMINAL_STATES, stateLabel, esitoLabel, stateVars } from '../../lib/cardStates';
import { errorLabel } from '../ricerche/helpers';
import { writeCohort } from '../../components/scheda/cohort';
import { ActivityTimeline } from '../../components/company/activity/ActivityTimeline';
import { CompanyContactsPanel } from '../../components/company/contacts/CompanyContactsPanel';
import { useCompanyActivity } from '../../hooks/useCompanyActivity';
import { FunnelStrip } from '../iniziative/board/FunnelStrip';
import { StatesView } from '../iniziative/board/StatesView';
import { MacroView } from '../iniziative/board/MacroView';
import { BoardTable } from '../iniziative/board/BoardTable';
import { SegnalazioneModal } from '../../components/segnalazioni/SegnalazioneModal';
import styles from '../iniziative/board/board.module.css';

type Layout = 'macro' | 'states' | 'table';
type Scope = 'active' | 'complete';

function readLS(k: string, fb: string) {
  try {
    return localStorage.getItem(k) ?? fb;
  } catch {
    return fb;
  }
}
function writeLS(k: string, v: string) {
  try {
    localStorage.setItem(k, v);
  } catch {
    /* best-effort */
  }
}

export function PipelinePage() {
  const api = useApiClient();
  const navigate = useNavigate();

  const query = useQuery({
    queryKey: ['ma-pipeline'],
    queryFn: () => api.get<MAPipelineResponse>('/binocolo/v1/ma/pipeline'),
    // I tag sulle card sono condivisi tra utenti: riletti a apertura e focus
    // (politica centralizzata, issue #198).
    ...maTagQueryPolicy,
  });
  const data = query.data;

  const [layout, setLayoutState] = useState<Layout>(() => readLS('binocolo.pipeline.layout', 'macro') as Layout);
  const [scope, setScopeState] = useState<Scope>(() => readLS('binocolo.pipeline.scope', 'active') as Scope);
  const [collapsed, setCollapsed] = useState<Set<string>>(new Set());
  const [search, setSearch] = useState('');
  const [funnelFilter, setFunnelFilter] = useState<string | null>(null);
  const [initFilter, setInitFilter] = useState<string[]>([]);
  const [selected, setSelected] = useState<MAPipelineCardView | null>(null);
  const [segnalazioneOpen, setSegnalazioneOpen] = useState(false);

  const setLayout = (l: Layout) => {
    setLayoutState(l);
    writeLS('binocolo.pipeline.layout', l);
  };
  const setScope = (s: Scope) => {
    setScopeState(s);
    writeLS('binocolo.pipeline.scope', s);
  };
  const toggleCollapse = (state: string) =>
    setCollapsed((prev) => {
      const next = new Set(prev);
      if (next.has(state)) next.delete(state);
      else next.add(state);
      return next;
    });

  const allCards = data?.cards ?? [];
  const initiatives = data?.initiatives ?? [];

  // Card ristrette alle iniziative selezionate (se presenti) — base per i conteggi.
  const scopedCards = useMemo(
    () => (initFilter.length === 0 ? allCards : allCards.filter((c) => initFilter.includes(c.initiativeId))),
    [allCards, initFilter],
  );

  const counts = useMemo(() => {
    const c: Record<string, number> = {};
    for (const card of scopedCards) c[card.state] = (c[card.state] ?? 0) + 1;
    return c;
  }, [scopedCards]);

  const cardsByState = useMemo(() => {
    const q = search.trim().toLowerCase();
    const map = new Map<string, MAInitiativeCardView[]>();
    for (const s of CARD_STATES) map.set(s.key, []);
    for (const card of scopedCards) {
      if (q && !card.companyName.toLowerCase().includes(q)) continue;
      if (funnelFilter && card.state !== funnelFilter) continue;
      (map.get(card.state) ?? []).push(card);
    }
    return map;
  }, [scopedCards, search, funnelFilter]);

  const showEsito = scope === 'complete';
  const visibleStates = useMemo(() => [...ACTIVE_STATES, ...(showEsito ? TERMINAL_STATES : [])], [showEsito]);
  const visibleMacrofasi = useMemo(() => MACROFASI.filter((m) => showEsito || m.key !== 'esito'), [showEsito]);

  const titleByInit = useMemo(() => new Map(initiatives.map((i) => [i.id, i.title])), [initiatives]);
  const initiativeChip = (card: MAInitiativeCardView) => titleByInit.get(card.initiativeId);

  const cohortKeys = useMemo(
    () => CARD_STATES.flatMap((s) => (cardsByState.get(s.key) ?? []).map((c) => c.companyKey)),
    [cardsByState],
  );

  const onCardClick = (card: MAInitiativeCardView) => setSelected(card as MAPipelineCardView);

  if (query.isLoading && !data) {
    return (
      <main className={styles.page}>
        <Skeleton rows={6} />
      </main>
    );
  }
  if (!data) {
    return (
      <main className={styles.page}>
        <div role="alert" style={{ color: 'var(--color-danger-hover)' }}>
          <Icon name="triangle-alert" size={18} /> {query.error ? errorLabel(query.error) : 'Pipeline non disponibile.'}
        </div>
      </main>
    );
  }

  return (
    <main className={styles.page}>
      <div className={styles.board}>
        <div className={styles.topbar}>
          <div className={styles.titleBlock}>
            <span className={styles.kicker}>
              Pipeline · {initiatives.length} iniziative attive · {allCards.length} aziende
            </span>
            <h1 className={styles.title}>Pipeline aggregata</h1>
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
            <div className={styles.seg} role="group" aria-label="Disposizione">
              <button className={`${styles.segIcon} ${layout === 'macro' ? styles.segOn : ''}`} aria-pressed={layout === 'macro'} onClick={() => setLayout('macro')} title="Vista Macrofasi" aria-label="Vista Macrofasi">
                <Icon name="layout-grid" size={15} />
              </button>
              <button className={`${styles.segIcon} ${layout === 'states' ? styles.segOn : ''}`} aria-pressed={layout === 'states'} onClick={() => setLayout('states')} title="Tutti gli stati" aria-label="Tutti gli stati">
                <Icon name="kanban-square" size={15} />
              </button>
              <button className={`${styles.segIcon} ${layout === 'table' ? styles.segOn : ''}`} aria-pressed={layout === 'table'} onClick={() => setLayout('table')} title="Vista tabella" aria-label="Vista tabella">
                <Icon name="table" size={15} />
              </button>
            </div>
          </div>
          <div style={{ width: 200 }}>
            <MultiSelect<string>
              options={initiatives.map((i) => ({ value: i.id, label: i.title }))}
              selected={initFilter}
              onChange={setInitFilter}
              placeholder="Tutte le iniziative"
            />
          </div>
          <label className={styles.search}>
            <Icon name="search" size={14} />
            <input placeholder="Cerca azienda…" value={search} onChange={(e) => setSearch(e.target.value)} />
          </label>
          <Tooltip content="Nuova segnalazione">
            <Button variant="secondary" className={styles.topbarIconBtn} aria-label="Nuova segnalazione" onClick={() => setSegnalazioneOpen(true)}>
              <Icon name="lightbulb" size={18} />
            </Button>
          </Tooltip>
        </div>

        {layout !== 'table' ? (
          <FunnelStrip counts={counts} activeState={funnelFilter} onToggle={(s) => setFunnelFilter((p) => (p === s ? null : s))} showEsito={showEsito} />
        ) : null}

        {layout === 'table' ? (
          <BoardTable cards={scopedCards} onRowClick={onCardClick} />
        ) : layout === 'macro' ? (
          <MacroView
            visibleMacrofasi={visibleMacrofasi}
            cardsByState={cardsByState}
            collapsed={collapsed}
            onToggleCollapse={toggleCollapse}
            onCardClick={onCardClick}
            onOpenDossier={(c) => navigate(`/aziende/${encodeURIComponent(c.companyKey)}?iniziativa=${encodeURIComponent(c.initiativeId)}`)}
            onIsolate={(s) => {
              setLayout('states');
              setFunnelFilter(s);
            }}
            readOnly
            initiativeChip={initiativeChip}
          />
        ) : (
          <StatesView
            visibleStates={visibleStates}
            cardsByState={cardsByState}
            collapsed={collapsed}
            onToggleCollapse={toggleCollapse}
            onCardClick={onCardClick}
            onOpenDossier={(c) => navigate(`/aziende/${encodeURIComponent(c.companyKey)}?iniziativa=${encodeURIComponent(c.initiativeId)}`)}
            readOnly
            initiativeChip={initiativeChip}
          />
        )}
      </div>

      {selected ? (
        <PipelineDrawer
          card={selected}
          onClose={() => setSelected(null)}
          onOpenBoard={() => navigate(`/iniziative/${encodeURIComponent(selected.initiativeId)}`)}
          onOpenScheda={() => {
            writeCohort({ lensType: 'iniziativa', lensId: selected.initiativeId, companyKeys: cohortKeys });
            navigate(`/aziende/${encodeURIComponent(selected.companyKey)}?iniziativa=${encodeURIComponent(selected.initiativeId)}`);
          }}
        />
      ) : null}
      <SegnalazioneModal open={segnalazioneOpen} onClose={() => setSegnalazioneOpen(false)} />
    </main>
  );
}

/** Drawer read-only per stato e lavorazione della card; la rubrica aziendale
 * resta modificabile perché non muta la card. */
function PipelineDrawer({
  card,
  onClose,
  onOpenBoard,
  onOpenScheda,
}: {
  card: MAPipelineCardView;
  onClose: () => void;
  onOpenBoard: () => void;
  onOpenScheda: () => void;
}) {
  const activity = useCompanyActivity(card.companyKey);
  const sessionInitiative = new Map((activity.data?.sessions ?? []).map((session) => [session.id, session.initiativeId]));
  const events = (activity.data?.items ?? []).filter((item) => item.initiativeId === card.initiativeId || (item.sessionId && sessionInitiative.get(item.sessionId) === card.initiativeId));

  const prov = card.provenances?.[0];
  const facts = card.registryFacts ?? [];

  return (
    <Drawer
      open
      onClose={onClose}
      title={card.companyName}
      subtitle={
        <div className={styles.drawerMeta}>
          {card.vatCode ? <span>P.IVA {card.vatCode}</span> : null}
          {card.province ? <span> · {card.province}</span> : null}
          <span> · {card.initiativeTitle}</span>
        </div>
      }
      headerExtra={
        <div className={styles.drawerActions}>
          <Button variant="secondary" size="sm" onClick={onOpenBoard}>
            Apri nella board ↗
          </Button>
          <Button variant="secondary" size="sm" onClick={onOpenScheda}>
            Apri scheda ↗
          </Button>
        </div>
      }
      size="lg"
    >
      <div className={styles.drawerBody}>
        <div className={styles.drawerScrollArea}>
          <div className={styles.drawerSec} style={{ borderTop: 0, marginTop: 0, paddingTop: 0 }}>
            <p className={styles.lab}>Stato</p>
            <span className={styles.statePill} style={stateVars(card.state) as CSSProperties}>
              <span className={styles.dot} />
              {stateLabel(card.state)}
              {card.esito ? ` · ${esitoLabel(card.esito)}` : ''}
            </span>
            {prov ? <p className={styles.hint} style={{ marginTop: 8 }}>Giudizio: {'★'.repeat(Math.max(0, Math.min(3, prov.rating)))} · {prov.sessionTitle}</p> : null}
          </div>

          <div className={styles.drawerSec}>
            <CompanyContactsPanel companyKey={card.companyKey} compact />
          </div>

          <div className={styles.drawerSec}>
            <p className={styles.lab}>Registro azienda</p>
            {facts.length > 0 ? (
              <div className={styles.cardMeta}>
                {facts.map((k) => (
                  <span key={k} className={`${styles.chip} ${styles.chipRegWarn}`}>
                    {k}
                  </span>
                ))}
              </div>
            ) : (
              <p className={styles.hint}>Nessun fatto registrato.</p>
            )}
          </div>

          <div className={styles.drawerSec}>
            <p className={styles.lab}>Attività · {card.initiativeTitle}</p>
            {activity.isLoading ? (
              <Skeleton rows={3} />
            ) : activity.isError ? (
              <div className={styles.activityError} role="alert">
                <Icon name="triangle-alert" size={16} />
                <div>
                  <p>Attività non disponibile.</p>
                  <Button variant="secondary" size="sm" onClick={() => void activity.refetch()}>Riprova</Button>
                </div>
              </div>
            ) : (
              <ActivityTimeline
                items={events}
                initiatives={activity.data?.initiatives}
                sessions={activity.data?.sessions}
                companyKey={card.companyKey}
              />
            )}
          </div>
        </div>
      </div>
    </Drawer>
  );
}
