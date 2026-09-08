import { useEffect, useMemo, useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import { Button, Icon, MultiSelect, Skeleton, useToast } from '@mrsmith/ui';
import type { Segnalazione, SegnalazioneState } from '../../api/types';
import { useSegnalazioneMutations, useSegnalazioni } from '../../hooks/useSegnalazioni';
import { SEGNALAZIONE_STATES } from '../../lib/segnalazioneStates';
import { errorLabel } from '../ricerche/helpers';
import { SegnalazioneModal } from '../../components/segnalazioni/SegnalazioneModal';
import { SegnalazioniBoard } from './SegnalazioniBoard';
import { SegnalazioniTable } from './SegnalazioniTable';
import { SegnalazioneDrawer } from './SegnalazioneDrawer';
import board from '../iniziative/board/board.module.css';
import styles from './Segnalazioni.module.css';

type Layout = 'board' | 'table';
const LAYOUT_KEY = 'binocolo.segnalazioni.layout';

function readLayout(): Layout {
  try {
    return localStorage.getItem(LAYOUT_KEY) === 'table' ? 'table' : 'board';
  } catch {
    return 'board';
  }
}
function writeLayout(layout: Layout) {
  try {
    localStorage.setItem(LAYOUT_KEY, layout);
  } catch {
    /* best-effort */
  }
}

function matches(item: Segnalazione, needle: string): boolean {
  if (!needle) return true;
  const haystack = [item.name, item.website, item.location, item.fiscalId, item.contacts, item.notes, item.createdByEmail ?? '']
    .join('\n')
    .toLowerCase();
  return haystack.includes(needle);
}

export function SegnalazioniPage() {
  const query = useSegnalazioni();
  const { setState } = useSegnalazioneMutations();
  const { toast } = useToast();
  const [searchParams, setSearchParams] = useSearchParams();
  const [search, setSearch] = useState('');
  const [layout, setLayoutState] = useState<Layout>(readLayout);
  const [stateFilter, setStateFilter] = useState<SegnalazioneState[]>([]);
  const [createOpen, setCreateOpen] = useState(false);

  const setLayout = (next: Layout) => {
    setLayoutState(next);
    writeLayout(next);
  };

  const items = query.data ?? [];
  const selectedId = searchParams.get('id');
  const selected = selectedId ? items.find((item) => item.id === selectedId) ?? null : null;

  // Link condivisibile: /segnalazioni?id=<uuid>. Se l'id non esiste, avviso e
  // rimozione del parametro una volta arrivati i dati.
  useEffect(() => {
    if (!selectedId || !query.data || selected) return;
    toast('Segnalazione non trovata.', 'error');
    setSearchParams((current) => { current.delete('id'); return current; }, { replace: true });
  }, [selectedId, query.data, selected, toast, setSearchParams]);

  const open = (item: Segnalazione) => setSearchParams((current) => { current.set('id', item.id); return current; });
  const close = () => setSearchParams((current) => { current.delete('id'); return current; });

  const needle = search.trim().toLowerCase();
  const searched = useMemo(() => items.filter((item) => matches(item, needle)), [items, needle]);
  // Il filtro per stato vale solo in tabella: sulla lavagna gli stati sono le colonne.
  const tabled = useMemo(
    () => (stateFilter.length ? searched.filter((item) => stateFilter.includes(item.state)) : searched),
    [searched, stateFilter],
  );
  const visible = layout === 'table' ? tabled : searched;

  const move = (item: Segnalazione, state: SegnalazioneState) => {
    setState.mutate({ id: item.id, state }, { onError: (error) => toast(errorLabel(error), 'error') });
  };

  const showAllInTable = (state: SegnalazioneState) => {
    setStateFilter([state]);
    setLayout('table');
  };

  const clearFilters = () => {
    setSearch('');
    setStateFilter([]);
  };

  return (
    <main className={board.page}>
      <div className={board.board}>
        <div className={board.topbar}>
          <div className={board.titleBlock}>
            <span className={board.kicker}>Segnalazioni · {items.length} totali</span>
            <h1 className={board.title}>Segnalazioni</h1>
          </div>
          <div className={board.viewMatrix}>
            <div className={board.seg} role="group" aria-label="Disposizione">
              <button type="button" className={`${board.segIcon} ${layout === 'board' ? board.segOn : ''}`} aria-pressed={layout === 'board'} onClick={() => setLayout('board')} title="Lavagna" aria-label="Lavagna">
                <Icon name="kanban-square" size={15} />
              </button>
              <button type="button" className={`${board.segIcon} ${layout === 'table' ? board.segOn : ''}`} aria-pressed={layout === 'table'} onClick={() => setLayout('table')} title="Tabella" aria-label="Tabella">
                <Icon name="table" size={15} />
              </button>
            </div>
            {layout === 'table' ? (
              <div style={{ width: 200 }}>
                <MultiSelect<SegnalazioneState>
                  options={SEGNALAZIONE_STATES.map((s) => ({ value: s.key, label: s.label }))}
                  selected={stateFilter}
                  onChange={setStateFilter}
                  placeholder="Tutti gli stati"
                />
              </div>
            ) : null}
            <label className={board.search}>
              <Icon name="search" size={14} />
              <input placeholder="Cerca nelle segnalazioni…" value={search} onChange={(event) => setSearch(event.target.value)} aria-label="Cerca nelle segnalazioni" />
            </label>
            <Button leftIcon={<Icon name="plus" />} onClick={() => setCreateOpen(true)}>Nuova segnalazione</Button>
          </div>
        </div>

        {query.isLoading ? (
          <Skeleton rows={6} />
        ) : query.isError ? (
          <div className={styles.errorBox} role="alert">
            <Icon name="triangle-alert" size={18} />
            <span>{errorLabel(query.error)}</span>
            <Button variant="secondary" size="sm" onClick={() => void query.refetch()}>Riprova</Button>
          </div>
        ) : items.length === 0 ? (
          <div className={styles.empty}>
            <div className={styles.emptyIcon}><Icon name="file-plus" size={32} /></div>
            <p className={styles.emptyTitle}>Nessuna segnalazione</p>
            <p className={styles.emptyText}>Registra la prima azienda o opportunità con le informazioni che conosci.</p>
            <Button leftIcon={<Icon name="plus" />} onClick={() => setCreateOpen(true)}>Nuova segnalazione</Button>
          </div>
        ) : layout === 'table' ? (
          <SegnalazioniTable items={visible} onRowClick={open} />
        ) : visible.length === 0 ? (
          <div className={board.colEmpty} style={{ padding: 'var(--space-16) 0' }}>
            Nessuna segnalazione corrisponde alla ricerca.{' '}
            <button type="button" className={board.actionLink} style={{ border: 'none', background: 'none', cursor: 'pointer' }} onClick={clearFilters}>
              Azzera ricerca
            </button>
          </div>
        ) : (
          <SegnalazioniBoard items={visible} searching={needle !== ''} onOpen={open} onMove={move} onShowAll={showAllInTable} />
        )}
      </div>

      <SegnalazioneModal open={createOpen} onClose={() => setCreateOpen(false)} />
      {selected ? <SegnalazioneDrawer item={selected} onClose={close} onSaved={() => toast('Contenuti salvati', 'success')} /> : null}
    </main>
  );
}
