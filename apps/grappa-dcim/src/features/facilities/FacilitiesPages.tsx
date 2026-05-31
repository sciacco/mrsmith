import { Button, Drawer, Icon, Modal, SearchInput, SingleSelect, Skeleton, useToast } from '@mrsmith/ui';
import { Suspense, lazy, useEffect, useMemo, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import {
  useBuildings,
  useDatacenterLayoutGrid,
  useDatacenterMap,
  useDatacenters,
  useFacilitiesMutations,
  useGrappaDCIMMeta,
  useIslets,
  useLayoutMutations,
} from '../../api/queries';
import type { Building, BuildingInput, Datacenter, DatacenterInput, Islet, Position, PositionRack, RackListItem, LayoutGridResponse, LayoutGridBlock, LayoutGridCell } from '../../api/types';
import { ViewState } from '../../components/ViewState';
import { LayoutGrid } from './LayoutGrid';
import type { LayoutSelection } from './LayoutScene';
import { fullRack, fullSlotStatus, isHalfPosition, rackAt, slotStatus, summarizeSlots, type HalfSide, type SlotStatus } from './positions';
import styles from './workspace.module.css';

const destructiveBody = { confirmPrimary: true, confirmSecondary: true };
const LayoutSceneView = lazy(() => import('./LayoutScene').then((module) => ({ default: module.LayoutScene })));
type LayoutViewMode = '2d' | '3d';

function valueOrDash(value: unknown) {
  if (value === undefined || value === null || value === '') return '-';
  return String(value);
}

function errorText(error: unknown, fallback: string) {
  if (typeof error === 'object' && error && 'body' in error) {
    const body = (error as { body?: unknown }).body;
    if (typeof body === 'object' && body && 'message' in body) {
      return String((body as { message?: unknown }).message);
    }
  }
  return fallback;
}

function StatusBadge({ value }: { value?: string }) {
  const normalized = (value ?? '').toLowerCase();
  const cls = normalized.includes('cess') ? styles.badgeDanger : normalized ? styles.badge : styles.badgeMuted;
  return <span className={cls}>{value || 'Non indicato'}</span>;
}

export function BuildingsPage() {
  const meta = useGrappaDCIMMeta();
  const canOperate = Boolean(meta.data?.canOperate);
  const toast = useToast();
  const [q, setQ] = useState('');
  const [status, setStatus] = useState<'active' | 'all'>('active');
  const [editing, setEditing] = useState<Building | null | 'new'>(null);
  const [ceasing, setCeasing] = useState<Building | null>(null);
  const [deleting, setDeleting] = useState<Building | null>(null);
  const query = useBuildings({ q, status });
  const mutations = useFacilitiesMutations();

  async function saveBuilding(input: BuildingInput & { id?: number }) {
    try {
      const result = await mutations.saveBuilding.mutateAsync(input);
      toast.toast(result.message || 'Edificio aggiornato.');
      setEditing(null);
    } catch (error) {
      toast.toast(errorText(error, 'Salvataggio edificio non riuscito.'), 'error');
    }
  }

  async function ceaseBuilding() {
    if (!ceasing) return;
    try {
      const result = await mutations.ceaseBuilding.mutateAsync({ id: ceasing.id, body: destructiveBody });
      toast.toast(result.message || 'Edificio cessato.');
      setCeasing(null);
    } catch (error) {
      toast.toast(errorText(error, 'Azione bloccata da dipendenze operative.'), 'error');
    }
  }

  async function deleteBuilding() {
    if (!deleting) return;
    try {
      const result = await mutations.deleteBuilding.mutateAsync({ id: deleting.id, body: destructiveBody });
      toast.toast(result.message || 'Edificio eliminato.');
      setDeleting(null);
    } catch (error) {
      toast.toast(errorText(error, 'Eliminazione bloccata da dipendenze operative.'), 'error');
    }
  }

  return (
    <section className={styles.page}>
      <div className={styles.header}>
        <div className={styles.titleBlock}>
          <span className={styles.eyebrow}>Infrastruttura</span>
          <h1 className={styles.title}>Edifici</h1>
        </div>
        {canOperate ? (
          <Button onClick={() => setEditing('new')} leftIcon={<Icon name="plus" size={16} />}>Nuovo edificio</Button>
        ) : null}
      </div>
      <div className={styles.inlineToolbar}>
        <SearchInput value={q} onChange={setQ} placeholder="Cerca edificio..." />
        <SingleSelect
          options={[
            { value: 'active', label: 'Solo attivi' },
            { value: 'all', label: 'Tutti' },
          ]}
          selected={status}
          onChange={(value) => setStatus((value ?? 'active') as 'active' | 'all')}
          searchable={false}
        />
      </div>
      {query.isLoading ? (
        <div className={styles.panel}><Skeleton rows={8} /></div>
      ) : query.error ? (
        <ViewState title="Edifici non disponibili" message="Non e stato possibile caricare il registro edifici." tone="error" />
      ) : (query.data?.length ?? 0) === 0 ? (
        <div className={styles.emptyPanel}>
          <h3 className={styles.emptyTitle}>Nessun edificio trovato</h3>
          <p className={styles.emptyText}>Modifica i filtri o aggiungi un edificio operativo.</p>
        </div>
      ) : (
        <div className={styles.tableWrap}>
          <table className={styles.table}>
            <thead>
              <tr>
                <th className={styles.accentHeader}></th>
                <th>Edificio</th>
                <th>Indirizzo</th>
                <th>Stato</th>
                <th>Portale clienti</th>
                <th>Sale</th>
                <th>Rack</th>
                <th>Azioni</th>
              </tr>
            </thead>
            <tbody>
              {query.data?.map((item) => (
                <tr key={item.id}>
                  <td className={styles.accentCell}>
                    <div className={styles.accentBar} />
                  </td>
                  <td><strong>{item.name}</strong></td>
                  <td>{item.address}</td>
                  <td><StatusBadge value={item.status} /></td>
                  <td>{item.portalEnabled ? 'Si' : 'No'}</td>
                  <td>{item.datacenterCount}</td>
                  <td>{item.rackCount} / {item.rackCapacity}</td>
                  <td>
                    <div className={styles.actions}>
                      {canOperate ? (
                        <Button size="sm" variant="ghost" onClick={() => setEditing(item)} title="Modifica">
                          <Icon name="pencil" size={16} />
                        </Button>
                      ) : null}
                      {canOperate ? (
                        <Button
                          size="sm"
                          variant="ghost"
                          onClick={() => setCeasing(item)}
                          disabled={item.status === 'Cessato' || item.datacenterCount > 0 || item.rackCount > 0}
                          title="Cessa"
                          style={{ color: 'var(--color-warning-strong)' }}
                        >
                          <Icon name="archive" size={16} />
                        </Button>
                      ) : null}
                      {canOperate ? (
                        <Button
                          size="sm"
                          variant="ghost"
                          className={styles.deleteBtn}
                          onClick={() => setDeleting(item)}
                          disabled={item.datacenterCount > 0 || item.rackCount > 0}
                          title="Elimina"
                        >
                          <Icon name="trash" size={16} />
                        </Button>
                      ) : null}
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
      <BuildingModal open={editing !== null} value={editing === 'new' ? null : editing} onClose={() => setEditing(null)} onSave={saveBuilding} loading={mutations.saveBuilding.isPending} />
      <ConfirmModal
        open={ceasing !== null}
        title="Cessa edificio"
        message={`Confermi la cessazione di ${ceasing?.name ?? 'questo edificio'}?`}
        onClose={() => setCeasing(null)}
        onConfirm={ceaseBuilding}
        loading={mutations.ceaseBuilding.isPending}
      />
      <ConfirmModal
        open={deleting !== null}
        title="Elimina edificio"
        message={`Confermi l'eliminazione definitiva di ${deleting?.name ?? 'questo edificio'}?`}
        onClose={() => setDeleting(null)}
        onConfirm={deleteBuilding}
        loading={mutations.deleteBuilding.isPending}
      />
    </section>
  );
}

export function DatacentersPage() {
  const params = useParams();
  const navigate = useNavigate();
  const meta = useGrappaDCIMMeta();
  const canOperate = Boolean(meta.data?.canOperate);
  const toast = useToast();
  const [q, setQ] = useState('');
  const [kind, setKind] = useState<'all' | 'room' | 'mmr'>('all');
  const [status, setStatus] = useState<'active' | 'all'>('active');
  const [buildingFilter, setBuildingFilter] = useState<string | number>('all');
  const [editing, setEditing] = useState<Datacenter | null | 'new'>(null);
  const [ceasing, setCeasing] = useState<Datacenter | null>(null);
  const [deleting, setDeleting] = useState<Datacenter | null>(null);
  const selectedId = params.datacenterId ? Number(params.datacenterId) : null;
  
  const query = useDatacenters({ q, kind, status });
  const buildingsQuery = useBuildings({ status: 'all' });

  const buildingOptions = useMemo(() => {
    const list: Array<{ value: string | number; label: string }> = [
      { value: 'all', label: 'Tutti' },
      { value: 'third', label: 'Edifici terzi' },
    ];
    if (buildingsQuery.data) {
      buildingsQuery.data.forEach((b) => {
        list.push({ value: b.id, label: b.name });
      });
    }
    return list;
  }, [buildingsQuery.data]);

  const filteredDatacenters = useMemo(() => {
    if (!query.data) return [];
    const filtered = query.data.filter((item) => {
      if (buildingFilter === 'all') return true;
      if (buildingFilter === 'third') return !item.buildingId;
      return item.buildingId === buildingFilter;
    });

    return [...filtered].sort((a, b) => {
      const hasBuildingA = !!a.buildingName;
      const hasBuildingB = !!b.buildingName;

      // 1) & 3) Sale con un edificio prima di quelle senza edificio
      if (hasBuildingA && !hasBuildingB) return -1;
      if (!hasBuildingA && hasBuildingB) return 1;

      if (hasBuildingA && hasBuildingB) {
        // Entrambi hanno un edificio: in ordine alfabetico dell'edificio
        const bNameA = a.buildingName || '';
        const bNameB = b.buildingName || '';
        const bComp = bNameA.localeCompare(bNameB);
        if (bComp !== 0) return bComp;

        // 2) Nello stesso edificio: prima le sale (isMmr === false), poi le MMR (isMmr === true)
        if (a.isMmr !== b.isMmr) {
          return a.isMmr ? 1 : -1;
        }

        // Nello stesso edificio e stesso tipo: in ordine alfabetico di nome
        return a.name.localeCompare(b.name);
      }

      // Entrambi senza edificio: in ordine alfabetico di nome
      return a.name.localeCompare(b.name);
    });
  }, [query.data, buildingFilter]);

  const selected = filteredDatacenters.find((item) => item.id === selectedId) ?? filteredDatacenters[0] ?? null;
  const layoutGrid = useDatacenterLayoutGrid(selected?.id ?? null);
  const mutations = useFacilitiesMutations();

  async function saveDatacenter(input: DatacenterInput & { id?: number }) {
    try {
      const result = await mutations.saveDatacenter.mutateAsync(input);
      toast.toast(result.message || 'Sala aggiornata.');
      setEditing(null);
    } catch (error) {
      toast.toast(errorText(error, 'Salvataggio sala non riuscito.'), 'error');
    }
  }

  async function ceaseDatacenter() {
    if (!ceasing) return;
    try {
      const result = await mutations.ceaseDatacenter.mutateAsync({ id: ceasing.id, body: destructiveBody });
      toast.toast(result.message || 'Sala cessata.');
      setCeasing(null);
    } catch (error) {
      toast.toast(errorText(error, 'Azione bloccata da dipendenze operative.'), 'error');
    }
  }

  async function deleteDatacenter() {
    if (!deleting) return;
    try {
      const result = await mutations.deleteDatacenter.mutateAsync({ id: deleting.id, body: destructiveBody });
      toast.toast(result.message || 'Sala eliminata.');
      setDeleting(null);
    } catch (error) {
      toast.toast(errorText(error, 'Eliminazione bloccata da dipendenze operative.'), 'error');
    }
  }

  return (
    <section className={styles.page}>
      <div className={styles.header}>
        <div className={styles.titleBlock}>
          <span className={styles.eyebrow}>Infrastruttura</span>
          <h1 className={styles.title}>Sale e MMR</h1>
        </div>
        {canOperate ? <Button onClick={() => setEditing('new')} leftIcon={<Icon name="plus" size={16} />}>Nuova sala</Button> : null}
      </div>
      <div className={styles.inlineToolbar}>
        <SearchInput value={q} onChange={setQ} placeholder="Cerca sala o MMR..." />
        <SingleSelect
          options={buildingOptions}
          selected={buildingFilter}
          onChange={(value) => setBuildingFilter(value ?? 'all')}
          placeholder="Seleziona edificio"
          searchable={buildingOptions.length > 5}
        />
        <SingleSelect
          options={[{ value: 'all', label: 'Sale e MMR' }, { value: 'room', label: 'Sale' }, { value: 'mmr', label: 'MMR' }]}
          selected={kind}
          onChange={(value) => setKind((value ?? 'all') as 'all' | 'room' | 'mmr')}
          searchable={false}
        />
        <SingleSelect
          options={[{ value: 'active', label: 'Solo attive' }, { value: 'all', label: 'Tutte' }]}
          selected={status}
          onChange={(value) => setStatus((value ?? 'active') as 'active' | 'all')}
          searchable={false}
        />
      </div>
      {query.isLoading ? (
        <div className={styles.panel}><Skeleton rows={8} /></div>
      ) : query.error ? (
        <ViewState title="Sale non disponibili" message="Non e stato possibile caricare sale e MMR." tone="error" />
      ) : filteredDatacenters.length === 0 ? (
        <div className={styles.emptyPanel}><h3 className={styles.emptyTitle}>Nessuna sala trovata</h3><p className={styles.emptyText}>Modifica i filtri o aggiungi una nuova sala.</p></div>
      ) : (
        <div className={styles.split}>
          <div className={styles.sidebarList}>
            {filteredDatacenters.map((item) => {
              const isSelected = selected?.id === item.id;
              return (
                <div
                  key={item.id}
                  className={`${styles.sidebarItem} ${isSelected ? styles.sidebarItemActive : ''}`}
                  onClick={() => navigate(`/sale-mmr/${item.id}`)}
                >
                  <div className={styles.accentBar} />
                  <div className={styles.sidebarItemContent}>
                    <div className={styles.sidebarItemHeader}>
                      <strong className={styles.sidebarItemName}>{item.name}</strong>
                    </div>
                    <div className={styles.sidebarItemMeta}>
                      <span className={styles.sidebarItemBuilding}>
                        {item.buildingName ?? 'Edifici terzi'}
                      </span>
                      <div className={styles.sidebarItemBadges}>
                        {item.isMmr ? (
                          <span className={styles.compactBadge}>MMR</span>
                        ) : (
                          <span className={styles.compactBadgeMuted}>Sala</span>
                        )}
                        {item.status !== 'Attivo' && (
                          <span className={styles.compactBadgeWarning}>Cessata</span>
                        )}
                      </div>
                    </div>
                  </div>
                </div>
              );
            })}
          </div>
          <DatacenterMapPanel 
            data={layoutGrid.data} 
            loading={layoutGrid.isLoading} 
            error={layoutGrid.error} 
            canOperate={canOperate}
            onEdit={() => setEditing(selected)}
          />
        </div>
      )}
      <DatacenterDrawer 
        open={editing !== null} 
        value={editing === 'new' ? null : editing} 
        onClose={() => setEditing(null)} 
        onSave={saveDatacenter} 
        loading={mutations.saveDatacenter.isPending}
        onCease={(item) => setCeasing(item)}
        onDelete={(item) => setDeleting(item)}
      />
      <ConfirmModal open={ceasing !== null} title="Cessa sala" message={`Confermi la cessazione di ${ceasing?.name ?? 'questa sala'}?`} onClose={() => setCeasing(null)} onConfirm={ceaseDatacenter} loading={mutations.ceaseDatacenter.isPending} />
      <ConfirmModal open={deleting !== null} title="Elimina sala" message={`Confermi l'eliminazione definitiva di ${deleting?.name ?? 'questa sala'}?`} onClose={() => setDeleting(null)} onConfirm={deleteDatacenter} loading={mutations.deleteDatacenter.isPending} />
    </section>
  );
}

export function LayoutPage() {
  const toast = useToast();
  const navigate = useNavigate();
  const meta = useGrappaDCIMMeta();
  const canOperate = Boolean(meta.data?.canOperate);
  const datacenters = useDatacenters({ kind: 'all', status: 'active' });
  const [datacenterId, setDatacenterId] = useState<number | null>(null);
  const [selection, setSelection] = useState<LayoutSelection>(null);
  const [viewMode, setViewMode] = useState<LayoutViewMode | null>(null);
  const [editingIslet, setEditingIslet] = useState<Islet | null | 'new'>(null);
  const [deletingIslet, setDeletingIslet] = useState<Islet | null>(null);
  const [editingPosition, setEditingPosition] = useState<Position | null>(null);
  const [deletingPosition, setDeletingPosition] = useState<Position | null>(null);
  const [batchCount, setBatchCount] = useState(0);
  const [batchType, setBatchType] = useState('full');
  const islets = useIslets(datacenterId);
  const map = useDatacenterMap(datacenterId);
  const layoutGrid = useDatacenterLayoutGrid(datacenterId);
  const mutations = useLayoutMutations();

  const selectedDatacenter = datacenters.data?.find((item) => item.id === datacenterId) ?? null;
  const hasLayoutGridBlocks = (layoutGrid.data?.blocks.length ?? 0) > 0;
  const effectiveViewMode: LayoutViewMode = viewMode ?? (hasLayoutGridBlocks ? '2d' : '3d');
  const waitingForDefaultView = datacenterId !== null && viewMode === null && layoutGrid.isLoading;
  const sceneIslets = useMemo(() => map.data?.islets ?? islets.data ?? [], [islets.data, map.data?.islets]);
  const scenePositions = useMemo(() => layoutGrid.data?.positions ?? map.data?.positions ?? [], [layoutGrid.data?.positions, map.data?.positions]);
  const selectedPosition = selection?.type === 'position' ? scenePositions.find((item) => item.id === selection.id) ?? null : null;
  const selectedIslet = selection?.type === 'islet'
    ? sceneIslets.find((item) => item.id === selection.id) ?? null
    : selectedPosition
      ? sceneIslets.find((item) => item.id === selectedPosition.isletId) ?? null
      : null;
  const datacenterOptions = useMemo(
    () => datacenters.data?.map((item) => ({ value: item.id, label: `${item.name}${item.isMmr ? ' - MMR' : ''}` })) ?? [],
    [datacenters.data],
  );
  // Per-slot occupancy (Full = 1 posto, Half = 2), driven by active rack presence so
  // the inspector/header counts match the coloured cells in the 2D/3D layout.
  const occupancy = useMemo(() => summarizeSlots(scenePositions), [scenePositions]);

  useEffect(() => {
    const firstDatacenter = datacenters.data?.[0];
    if (datacenterId !== null || !firstDatacenter) return;
    setDatacenterId(firstDatacenter.id);
  }, [datacenterId, datacenters.data]);

  useEffect(() => {
    if (datacenterId === null || viewMode !== null || layoutGrid.isLoading) return;
    setViewMode(hasLayoutGridBlocks ? '2d' : '3d');
  }, [datacenterId, hasLayoutGridBlocks, layoutGrid.isLoading, viewMode]);

  useEffect(() => {
    if (!selection) return;
    const valid = selection.type === 'islet'
      ? sceneIslets.some((item) => item.id === selection.id)
      : scenePositions.some((item) => item.id === selection.id);
    if (!valid) setSelection(null);
  }, [sceneIslets, scenePositions, selection]);

  async function createBatch() {
    if (!selectedIslet) return;
    try {
      const result = await mutations.createPositions.mutateAsync({ isletId: selectedIslet.id, count: batchCount, type: batchType });
      toast.toast(result.message || 'Posizioni create.');
      setBatchCount(0);
      setSelection({ type: 'islet', id: selectedIslet.id });
    } catch (error) {
      toast.toast(errorText(error, 'Creazione posizioni non riuscita.'), 'error');
    }
  }

  async function saveIslet(input: Partial<Islet> & { id?: number; datacenterId?: number }) {
    try {
      const result = await mutations.saveIslet.mutateAsync(input);
      toast.toast(result.message || 'Isola salvata.');
      setEditingIslet(null);
      if (result.id && !input.id) setSelection({ type: 'islet', id: result.id });
    } catch (error) {
      toast.toast(errorText(error, 'Salvataggio isola non riuscito.'), 'error');
    }
  }

  async function deleteIslet() {
    if (!deletingIslet) return;
    try {
      const result = await mutations.deleteIslet.mutateAsync({ id: deletingIslet.id, body: destructiveBody });
      toast.toast(result.message || 'Isola eliminata.');
      setDeletingIslet(null);
      if (selection?.type === 'islet' && selection.id === deletingIslet.id) setSelection(null);
    } catch (error) {
      toast.toast(errorText(error, 'Eliminazione bloccata da dipendenze operative.'), 'error');
    }
  }

  async function savePosition(input: Partial<Position> & { id: number }) {
    try {
      const result = await mutations.savePosition.mutateAsync(input);
      toast.toast(result.message || 'Posizione salvata.');
      setEditingPosition(null);
      setSelection({ type: 'position', id: input.id });
    } catch (error) {
      toast.toast(errorText(error, 'Salvataggio posizione non riuscito.'), 'error');
    }
  }

  async function deletePosition() {
    if (!deletingPosition) return;
    try {
      const result = await mutations.deletePosition.mutateAsync({ id: deletingPosition.id, body: destructiveBody });
      toast.toast(result.message || 'Posizione eliminata.');
      setDeletingPosition(null);
      if (selection?.type === 'position' && selection.id === deletingPosition.id) setSelection(null);
    } catch (error) {
      toast.toast(errorText(error, 'Eliminazione bloccata da dipendenze operative.'), 'error');
    }
  }

  return (
    <section className={styles.page}>
      <div className={styles.header}>
        <div className={styles.titleBlock}>
          <span className={styles.eyebrow}>Layout</span>
          <h1 className={styles.title}>Isole e posizioni</h1>
          <p className={styles.subtitle}>Vista fisica della sala, con occupazione rack e azioni operative sul punto selezionato.</p>
        </div>
        {canOperate ? <Button onClick={() => setEditingIslet('new')} disabled={!datacenterId} leftIcon={<Icon name="plus" size={16} />}>Nuova isola</Button> : null}
      </div>
      <div className={styles.layoutCommandBar}>
        <div className={styles.layoutSelectField}>
          <label>Sala</label>
          <SingleSelect
            options={datacenterOptions}
            selected={datacenterId}
            onChange={(value) => {
              setDatacenterId(value);
              setSelection(null);
              setViewMode(null);
            }}
            placeholder="Seleziona sala"
          />
        </div>
        <div className={styles.layoutViewSwitch} role="group" aria-label="Selezione vista layout">
          <button
            type="button"
            className={`${styles.layoutViewButton} ${effectiveViewMode === '2d' ? styles.layoutViewButtonActive : ''}`}
            onClick={() => setViewMode('2d')}
            disabled={!hasLayoutGridBlocks && !layoutGrid.isLoading}
          >
            Vista 2D
          </button>
          <button
            type="button"
            className={`${styles.layoutViewButton} ${effectiveViewMode === '3d' ? styles.layoutViewButtonActive : ''}`}
            onClick={() => setViewMode('3d')}
          >
            Vista 3D
          </button>
        </div>
        <div className={styles.layoutStatusStrip} aria-label="Occupazione posti rack">
          <span><strong>{occupancy.total}</strong> posti</span>
          <span><strong>{occupancy.free}</strong> liberi</span>
          <span><strong>{occupancy.occupied}</strong> occupati</span>
          {occupancy.shared > 0 ? <span><strong>{occupancy.shared}</strong> condivisi</span> : null}
          <span><strong>{occupancy.reserved}</strong> riservati</span>
        </div>
      </div>

      <div className={styles.layoutWorkspace}>
        <div className={styles.layoutMain}>
          {datacenterId === null ? (
            <div className={styles.layoutSceneFallback}>
              <h3 className={styles.emptyTitle}>Seleziona una sala</h3>
              <p className={styles.emptyText}>La vista fisica si aggiorna con isole, posizioni e rack disponibili.</p>
            </div>
          ) : waitingForDefaultView || (effectiveViewMode === '2d' && layoutGrid.isLoading) || (effectiveViewMode === '3d' && (map.isLoading || islets.isLoading)) ? (
            <div className={styles.layoutSceneFallback}><Skeleton rows={9} /></div>
          ) : effectiveViewMode === '2d' && layoutGrid.error ? (
            <ViewState title="Layout non disponibile" message="Non e stato possibile caricare la vista 2D della sala." tone="error" />
          ) : effectiveViewMode === '2d' ? (
            <LayoutGrid
              blocks={layoutGrid.data?.blocks ?? []}
              selected={selection}
              onSelect={setSelection}
            />
          ) : map.error ? (
            <ViewState title="Layout non disponibile" message="Non e stato possibile caricare la mappa fisica della sala." tone="error" />
          ) : (
            <Suspense fallback={<div className={styles.layoutSceneFallback}><Skeleton rows={9} /></div>}>
              <LayoutSceneView
                islets={sceneIslets}
                positions={scenePositions}
                selected={selection}
                onSelect={setSelection}
              />
            </Suspense>
          )}
          {effectiveViewMode === '2d' && (layoutGrid.data?.warnings.length ?? 0) > 0 ? (
            <div className={styles.layoutWarningPanel} role="status">
              <strong>Da verificare</strong>
              <ul>
                {layoutGrid.data?.warnings.slice(0, 4).map((warning) => <li key={warning}>{warning}</li>)}
              </ul>
            </div>
          ) : null}
          <LayoutQuickSelect
            islets={sceneIslets}
            positions={scenePositions}
            selected={selection}
            onSelect={setSelection}
          />
        </div>

        <LayoutInspector
          canOperate={canOperate}
          datacenter={selectedDatacenter}
          islets={sceneIslets}
          positions={scenePositions}
          selectedIslet={selectedIslet}
          selectedPosition={selectedPosition}
          occupancy={occupancy}
          batchCount={batchCount}
          batchType={batchType}
          setBatchCount={setBatchCount}
          setBatchType={setBatchType}
          createBatch={createBatch}
          createLoading={mutations.createPositions.isPending}
          onNewIslet={() => setEditingIslet('new')}
          onEditIslet={(item) => setEditingIslet(item)}
          onDeleteIslet={(item) => setDeletingIslet(item)}
          onEditPosition={(item) => setEditingPosition(item)}
          onDeletePosition={(item) => setDeletingPosition(item)}
          onOpenRack={(rackId) => navigate(`/rack/${rackId}`)}
        />
      </div>
      <IsletModal
        open={editingIslet !== null}
        value={editingIslet === 'new' ? null : editingIslet}
        datacenterId={datacenterId}
        onClose={() => setEditingIslet(null)}
        onSave={saveIslet}
        loading={mutations.saveIslet.isPending}
      />
      <PositionModal
        open={editingPosition !== null}
        value={editingPosition}
        onClose={() => setEditingPosition(null)}
        onSave={savePosition}
        loading={mutations.savePosition.isPending}
      />
      <ConfirmModal
        open={deletingIslet !== null}
        title="Elimina isola"
        message={`Confermi l'eliminazione definitiva di ${deletingIslet?.name ?? 'questa isola'}?`}
        onClose={() => setDeletingIslet(null)}
        onConfirm={deleteIslet}
        loading={mutations.deleteIslet.isPending}
      />
      <ConfirmModal
        open={deletingPosition !== null}
        title="Elimina posizione"
        message={`Confermi l'eliminazione definitiva della posizione ${deletingPosition?.num ?? ''}?`}
        onClose={() => setDeletingPosition(null)}
        onConfirm={deletePosition}
        loading={mutations.deletePosition.isPending}
      />
    </section>
  );
}

function LayoutQuickSelect({
  islets,
  positions,
  selected,
  onSelect,
}: {
  islets: Islet[];
  positions: Position[];
  selected: LayoutSelection;
  onSelect: (selection: LayoutSelection) => void;
}) {
  const selectedIsletId = selected?.type === 'islet'
    ? selected.id
    : selected?.type === 'position'
      ? positions.find((item) => item.id === selected.id)?.isletId ?? null
      : null;
  const visiblePositions = selectedIsletId
    ? positions.filter((item) => item.isletId === selectedIsletId).sort((a, b) => a.num - b.num)
    : [];

  return (
    <div className={styles.layoutQuickPanel}>
      <div className={styles.sectionHeader}>
        <h2 className={styles.emptyTitle}>Selezione rapida</h2>
        <span className={styles.badgeMuted}>{islets.length} isole</span>
      </div>
      <div className={styles.layoutRail} aria-label="Isole disponibili">
        {islets.length === 0 ? (
          <span className={styles.emptyText}>Nessuna isola configurata.</span>
        ) : islets.map((item) => (
          <button
            type="button"
            key={item.id}
            className={`${styles.layoutRailButton} ${selectedIsletId === item.id ? styles.layoutRailButtonActive : ''}`}
            onClick={() => onSelect({ type: 'islet', id: item.id })}
          >
            <strong>{item.name}</strong>
            <span>{item.occupiedCount} / {item.positionCount || item.rackNum}</span>
          </button>
        ))}
      </div>
      {selectedIsletId ? (
        <div className={styles.positionRail} aria-label="Posizioni isola selezionata">
          {visiblePositions.length === 0 ? (
            <span className={styles.emptyText}>Nessuna posizione per l'isola selezionata.</span>
          ) : visiblePositions.map((item) => (
            <button
              type="button"
              key={item.id}
              className={`${styles.positionRailButton} ${styles[normalizePositionStatusClass(positionEffectiveStatus(item))]} ${selected?.type === 'position' && selected.id === item.id ? styles.positionRailButtonActive : ''}`}
              onClick={() => onSelect({ type: 'position', id: item.id })}
            >
              <strong>{item.num}</strong>
              <span>{positionStatusLabel(positionEffectiveStatus(item))}</span>
            </button>
          ))}
        </div>
      ) : null}
    </div>
  );
}

function LayoutInspector({
  canOperate,
  datacenter,
  islets,
  positions,
  selectedIslet,
  selectedPosition,
  occupancy,
  batchCount,
  batchType,
  setBatchCount,
  setBatchType,
  createBatch,
  createLoading,
  onNewIslet,
  onEditIslet,
  onDeleteIslet,
  onEditPosition,
  onDeletePosition,
  onOpenRack,
}: {
  canOperate: boolean;
  datacenter: Datacenter | null;
  islets: Islet[];
  positions: Position[];
  selectedIslet: Islet | null;
  selectedPosition: Position | null;
  occupancy: { free: number; occupied: number; reserved: number; shared: number; total: number };
  batchCount: number;
  batchType: string;
  setBatchCount: (value: number) => void;
  setBatchType: (value: string) => void;
  createBatch: () => void;
  createLoading: boolean;
  onNewIslet: () => void;
  onEditIslet: (item: Islet) => void;
  onDeleteIslet: (item: Islet) => void;
  onEditPosition: (item: Position) => void;
  onDeletePosition: (item: Position) => void;
  onOpenRack: (rackId: number) => void;
}) {
  if (!datacenter) {
    return (
      <aside className={styles.layoutInspector} aria-label="Inspector layout">
        <span className={styles.eyebrow}>Inspector</span>
        <h2 className={styles.inspectorTitle}>Nessuna sala selezionata</h2>
        <p className={styles.emptyText}>Seleziona una sala per visualizzare il layout fisico e le azioni disponibili.</p>
      </aside>
    );
  }

  if (selectedPosition) {
    return (
      <aside className={styles.layoutInspector} aria-label="Inspector layout">
        <span className={styles.eyebrow}>Posizione</span>
        <div className={styles.inspectorTitleRow}>
          <h2 className={styles.inspectorTitle}>Posizione {selectedPosition.num}</h2>
          <span className={`${styles.badgeMuted} ${styles[normalizePositionStatusClass(positionEffectiveStatus(selectedPosition))]}`}>{positionStatusLabel(positionEffectiveStatus(selectedPosition))}</span>
        </div>
        <div className={styles.detailGrid}>
          <DetailItem label="Isola" value={selectedIslet?.name} />
          <DetailItem label="Formato" value={selectedPosition.type} />
          <DetailItem label="ID posizione" value={selectedPosition.id} />
        </div>
        {selectedPosition.racks.length === 0 ? (
          <p className={styles.emptyText}>Nessun rack su questa posizione.</p>
        ) : selectedPosition.racks.map((rack) => (
          <div key={rack.id} className={styles.detailGrid}>
            <DetailItem label={positionRackLabel(rack.pos)} value={rack.name} />
            <DetailItem label="Tipo rack" value={rack.type} />
          </div>
        ))}
        <div className={styles.inspectorActions}>
          {selectedPosition.racks.map((rack) => (
            <Button key={rack.id} variant="secondary" onClick={() => onOpenRack(rack.id)}>
              Apri {positionRackLabel(rack.pos).toLowerCase()}
            </Button>
          ))}
          {canOperate ? <Button variant="secondary" onClick={() => onEditPosition(selectedPosition)}>Modifica posizione</Button> : null}
          {canOperate ? <Button variant="danger" onClick={() => onDeletePosition(selectedPosition)}>Elimina posizione</Button> : null}
        </div>
      </aside>
    );
  }

  if (selectedIslet) {
    const isletPositions = positions.filter((item) => item.isletId === selectedIslet.id);
    const free = isletPositions.filter((item) => positionEffectiveStatus(item) === 'free').length;
    const occupied = isletPositions.filter((item) => {
      const status = positionEffectiveStatus(item);
      return status === 'occupied' || status === 'shared';
    }).length;

    return (
      <aside className={styles.layoutInspector} aria-label="Inspector layout">
        <span className={styles.eyebrow}>Isola</span>
        <div className={styles.inspectorTitleRow}>
          <h2 className={styles.inspectorTitle}>{selectedIslet.name}</h2>
          <span className={styles.badgeMuted}>{occupied} / {isletPositions.length || selectedIslet.rackNum}</span>
        </div>
        <div className={styles.detailGrid}>
          <DetailItem label="Tipo" value={selectedIslet.type} />
          <DetailItem label="Piano" value={selectedIslet.floor} />
          <DetailItem label="Libere" value={free} />
          <DetailItem label="Occupate" value={occupied} />
          <DetailItem label="Seriale" value={selectedIslet.serial} />
          <DetailItem label="Ordine" value={selectedIslet.order} />
        </div>
        {canOperate ? (
          <div className={styles.batchBox}>
            <div className={styles.sectionHeader}>
              <h3 className={styles.emptyTitle}>Crea posizioni</h3>
              <span className={styles.badgeMuted}>{selectedIslet.name}</span>
            </div>
            <div className={styles.batchGrid}>
              <div className={styles.field}>
                <label>Numero</label>
                <input type="number" min="1" value={batchCount || ''} onChange={(event) => setBatchCount(Number(event.target.value))} />
              </div>
              <div className={styles.field}>
                <label>Formato</label>
                <select value={batchType} onChange={(event) => setBatchType(event.target.value)}>
                  <option value="full">Full</option>
                  <option value="half">Half</option>
                </select>
              </div>
            </div>
            <Button variant="secondary" onClick={createBatch} disabled={!batchCount || createLoading}>Crea posizioni</Button>
          </div>
        ) : null}
        <div className={styles.inspectorActions}>
          {canOperate ? <Button variant="secondary" onClick={() => onEditIslet(selectedIslet)}>Modifica isola</Button> : null}
          {canOperate ? <Button variant="danger" onClick={() => onDeleteIslet(selectedIslet)}>Elimina isola</Button> : null}
        </div>
      </aside>
    );
  }

  return (
    <aside className={styles.layoutInspector} aria-label="Inspector layout">
      <span className={styles.eyebrow}>Sala</span>
      <h2 className={styles.inspectorTitle}>{datacenter.name}</h2>
      <p className={styles.emptyText}>{datacenter.isMmr ? 'MMR' : 'Sala'} con {islets.length} isole configurate.</p>
      <div className={styles.detailGrid}>
        <DetailItem label="Posti" value={occupancy.total} />
        <DetailItem label="Liberi" value={occupancy.free} />
        <DetailItem label="Occupati" value={occupancy.occupied} />
        {occupancy.shared > 0 ? <DetailItem label="Condivisi" value={occupancy.shared} /> : null}
        <DetailItem label="Riservati" value={occupancy.reserved} />
      </div>
      <div className={styles.inspectorActions}>
        {canOperate ? <Button onClick={onNewIslet} leftIcon={<Icon name="plus" size={16} />}>Nuova isola</Button> : null}
      </div>
    </aside>
  );
}

function DetailItem({ label, value }: { label: string; value: unknown }) {
  return (
    <div className={styles.detailItem}>
      <span className={styles.detailLabel}>{label}</span>
      <span className={styles.detailValue}>{valueOrDash(value)}</span>
    </div>
  );
}

function normalizePositionStatusClass(status: string) {
  if (status === 'occupied') return 'occupied';
  if (status === 'reserved') return 'reserved';
  if (status === 'shared') return 'shared';
  if (status === 'free') return 'free';
  return 'badgeMuted';
}

// Effective per-position status for badges/rails: a shared cabinet is condiviso
// (never free), otherwise Full follows position.status and Half takes the most
// occupied of its two sides (shared > occupied > reserved > free).
function positionEffectiveStatus(position: Position): SlotStatus {
  if (!isHalfPosition(position.type)) return fullSlotStatus(position);
  const rank = (s: SlotStatus) => (s === 'shared' ? 3 : s === 'occupied' ? 2 : s === 'reserved' ? 1 : 0);
  const a = slotStatus(position.status, rackAt(position.racks, 'A'));
  const b = slotStatus(position.status, rackAt(position.racks, 'B'));
  return rank(a) >= rank(b) ? a : b;
}

type HoveredSlot = { position: Position; rack?: PositionRack; side?: HalfSide };

type FallbackRack = {
  id: number;
  name: string;
  type?: string;
  pos?: string;
  shared: boolean;
  positionNum?: number;
  positionStatus?: string;
  rackNumber?: number;
  unitCount?: number;
  customerId?: number;
  orderCode?: string;
  soldPower?: number;
};

function isOperationalRack(rack: RackListItem): boolean {
  const status = (rack.status ?? '').trim().toLowerCase();
  return !rack.ceasedAt && !['cessato', 'cessata', 'spento', 'chiuso'].includes(status);
}

function isSharedValue(value: string | undefined): boolean {
  return (value ?? '').trim().toLowerCase() === 'si';
}

function buildFallbackRacks(data: LayoutGridResponse): FallbackRack[] {
  const detailById = new Map(data.racks.map((rack) => [rack.id, rack]));
  const fromPositions = data.positions.flatMap((position) => position.racks.map((rack) => {
    const detail = detailById.get(rack.id);
    return {
      id: rack.id,
      name: rack.name,
      type: detail?.type ?? rack.type,
      pos: detail?.position ?? rack.pos,
      shared: rack.shared || isSharedValue(detail?.shared),
      positionNum: position.num,
      positionStatus: position.status,
      rackNumber: detail?.rackNumber,
      unitCount: detail?.unitCount,
      customerId: detail?.customerId,
      orderCode: detail?.orderCode,
      soldPower: detail?.soldPower,
    };
  }));

  if (fromPositions.length > 0) return fromPositions;

  return data.racks.filter(isOperationalRack).map((rack) => ({
    id: rack.id,
    name: rack.name,
    type: rack.type,
    pos: rack.position,
    shared: isSharedValue(rack.shared),
    rackNumber: rack.rackNumber,
    unitCount: rack.unitCount,
    customerId: rack.customerId,
    orderCode: rack.orderCode,
    soldPower: rack.soldPower,
  }));
}

function fallbackRackPositionLabel(rack: FallbackRack): string {
  const num = rack.rackNumber ?? rack.positionNum;
  const parts = [
    num === undefined ? null : `Pos. ${num}`,
    rack.type,
    rack.pos,
  ].filter(Boolean);
  return parts.length > 0 ? parts.join(' · ') : '-';
}

function fallbackRackPositionStatusLabel(rack: FallbackRack): string | null {
  if (!rack.positionStatus) return null;
  return positionStatusLabel(fullSlotStatus({
    status: rack.positionStatus,
    racks: [{ id: rack.id, name: rack.name, type: rack.type ?? '', pos: rack.pos ?? '', shared: rack.shared }],
  }));
}

function FallbackRackTable({ racks }: { racks: FallbackRack[] }) {
  return (
    <div className={styles.fallbackRackList}>
      <div className={styles.sectionHeader}>
        <div>
          <h3 className={styles.emptyTitle}>Rack registrati</h3>
          <p className={styles.emptyText}>La sala non ha una mappa spaziale configurata; i rack sono mostrati come elenco operativo.</p>
        </div>
        <span className={styles.badgeMuted}>{racks.length} rack</span>
      </div>
      <div className={styles.tableWrap}>
        <table className={`${styles.table} ${styles.fallbackRackTable}`}>
          <thead>
            <tr>
              <th>Rack</th>
              <th>Posizione</th>
              <th>Unita</th>
              <th>Cliente / Ordine</th>
              <th>Potenza</th>
            </tr>
          </thead>
          <tbody>
            {racks.map((rack) => {
              const positionStatus = fallbackRackPositionStatusLabel(rack);
              return (
                <tr key={rack.id}>
                  <td>
                    <strong>{rack.name}</strong>
                    {rack.shared ? <span className={styles.compactBadgeShared}>Condiviso</span> : null}
                  </td>
                  <td>
                    {fallbackRackPositionLabel(rack)}
                    {positionStatus ? <br /> : null}
                    {positionStatus ? <span className={styles.muted}>{positionStatus}</span> : null}
                  </td>
                  <td>{valueOrDash(rack.unitCount)}</td>
                  <td>
                    {valueOrDash(rack.customerId)}
                    {rack.orderCode ? <br /> : null}
                    {rack.orderCode ? <span className={styles.muted}>{rack.orderCode}</span> : null}
                  </td>
                  <td>{rack.soldPower === undefined ? '-' : `${rack.soldPower} kW`}</td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
    </div>
  );
}

function countUnboundPositionCells(blocks: LayoutGridBlock[]): number {
  return blocks.reduce((total, block) => (
    total + block.grid.reduce((blockTotal, row) => (
      blockTotal + row.filter((cell) => block.isletId !== undefined && cell.type === 'position' && !cell.positionId).length
    ), 0)
  ), 0);
}

function unboundPositionCellsLabel(count: number): string {
  return count === 1 ? '1 posizione mappa non censita' : `${count} posizioni mappa non censite`;
}

function countUnboundPlenumCells(blocks: LayoutGridBlock[]): number {
  return blocks.reduce((total, block) => (
    total + block.grid.reduce((blockTotal, row) => (
      blockTotal + row.filter((cell) => cell.type === 'plenum' && !cell.plenumId).length
    ), 0)
  ), 0);
}

function unboundPlenumCellsLabel(count: number): string {
  return count === 1 ? '1 plenum non collegato' : `${count} plenum non collegati`;
}

function unlinkedIsletsLabel(count: number): string {
  return count === 1 ? '1 isola non collegata' : `${count} isole non collegate`;
}

function otherWarningsLabel(count: number): string {
  return count === 1 ? '1 segnalazione da verificare' : `${count} segnalazioni da verificare`;
}

function buildLayoutIssueLabels(data: LayoutGridResponse, unboundPositionCount: number, unboundPlenumCount: number): string[] {
  const positionWarningCount = data.warnings.filter((warning) => warning.includes('posizione') && warning.includes('non trovata')).length;
  const plenumWarningCount = data.warnings.filter((warning) => warning.includes('plenum') && warning.includes('non collegato')).length;
  const unlinkedIsletCount = data.warnings.filter((warning) => warning.includes('isola non collegata')).length;
  const otherWarningCount = data.warnings.filter((warning) => (
    warning !== 'Mappa non configurata'
    && !(warning.includes('posizione') && warning.includes('non trovata'))
    && !(warning.includes('plenum') && warning.includes('non collegato'))
    && !warning.includes('isola non collegata')
  )).length;

  const labels: string[] = [];
  const positionIssueCount = Math.max(unboundPositionCount, positionWarningCount);
  const plenumIssueCount = Math.max(unboundPlenumCount, plenumWarningCount);
  if (positionIssueCount > 0) labels.push(unboundPositionCellsLabel(positionIssueCount));
  if (plenumIssueCount > 0) labels.push(unboundPlenumCellsLabel(plenumIssueCount));
  if (unlinkedIsletCount > 0) labels.push(unlinkedIsletsLabel(unlinkedIsletCount));
  if (otherWarningCount > 0) labels.push(otherWarningsLabel(otherWarningCount));
  return labels;
}

// The source layout flattened the original nested Bootstrap structure into a flat
// list of blocks with only a `layoutWidth` hint, so we reconstruct the physical
// arrangement heuristically from each block's shape:
//  - a single-row, wide block (e.g. "Sezione 5") is a horizontal strip → bottom;
//  - a narrow tall block ("col-2" or single-column) is a rack column → right;
//  - everything else (the Fila/Isola grids) → main area, top.
function blockCols(block: LayoutGridBlock): number {
  return block.grid.reduce((max, row) => Math.max(max, row.length), 0);
}

function isBottomStrip(block: LayoutGridBlock): boolean {
  return block.grid.length === 1 && blockCols(block) >= 5;
}

function isRightColumn(block: LayoutGridBlock): boolean {
  if (isBottomStrip(block)) return false;
  const width = block.layoutWidth ?? '';
  return width.includes('col-2') || (blockCols(block) === 1 && block.grid.length >= 3);
}

function DatacenterMapPanel({
  data,
  loading,
  error,
  canOperate,
  onEdit,
}: {
  data?: LayoutGridResponse;
  loading: boolean;
  error: unknown;
  canOperate: boolean;
  onEdit: () => void;
}) {
  const [hovered, setHovered] = useState<HoveredSlot | null>(null);

  if (loading) return <div className={styles.panel}><Skeleton rows={8} /></div>;
  if (error || !data) return <div className={styles.emptyPanel}><h3 className={styles.emptyTitle}>Mappa non disponibile</h3><p className={styles.emptyText}>Seleziona una sala per visualizzare il layout.</p></div>;

  const isSpacious = data.blocks.length <= 4;
  const fallbackRacks = data.blocks.length === 0 ? buildFallbackRacks(data) : [];
  const hasRackFallback = fallbackRacks.length > 0;
  const unboundPositionCount = countUnboundPositionCells(data.blocks);
  const unboundPlenumCount = countUnboundPlenumCells(data.blocks);

  // Occupancy is counted per rack slot (posto: Full = 1, Half = 2) and driven by
  // active rack presence, so the numbers match the coloured regions exactly.
  const slots = summarizeSlots(data.positions);
  const subtitle = `${slots.occupied} occupati / ${slots.total} posti`
    + (slots.shared > 0 ? ` · ${slots.shared} condivisi` : '')
    + (slots.reserved > 0 ? ` · ${slots.reserved} riservati` : '');
  const layoutIssueLabels = buildLayoutIssueLabels(data, unboundPositionCount, unboundPlenumCount);
  const panelNote = hasRackFallback
    ? `Mappa non configurata · ${fallbackRacks.length} rack registrati`
    : data.blocks.length === 0
    ? (slots.total > 0 ? `Mappa non configurata · ${subtitle}` : 'Mappa non configurata')
    : layoutIssueLabels.length > 0
    ? `${subtitle} · ${layoutIssueLabels.join(' · ')}`
    : data.incomplete
    ? `${subtitle} · configurazione da verificare`
    : subtitle;
  const hoveredStatus = hovered ? (hovered.side ? slotStatus(hovered.position.status, hovered.rack) : fullSlotStatus(hovered.position)) : 'free';

  // Blocks arrive already sorted by displayOrder. Split them into the three
  // physical regions; rooms (DC) have no right/bottom blocks, so they fall back
  // to the original single flex-wrap container and render exactly as before.
  const bottomBlocks = data.blocks.filter(isBottomStrip);
  const rightBlocks = data.blocks.filter(isRightColumn);
  const mainBlocks = data.blocks.filter((block) => !isBottomStrip(block) && !isRightColumn(block));
  const useRegions = rightBlocks.length > 0 || bottomBlocks.length > 0;

  const renderBlock = (block: LayoutGridBlock) => (
    <section
      key={block.id}
      className={`${styles.mapGridBlock} ${(block.layoutWidth ?? '').includes('align-self-end') ? styles.mapGridBlockBottomAligned : ''}`}
    >
      <div className={styles.mapGridBlockHeader}>
        <strong>{block.title}</strong>
      </div>
      <div className={styles.mapGridTable}>
        {block.grid.map((row: LayoutGridCell[], rowIndex: number) => (
          <div
            key={rowIndex}
            className={styles.mapGridRow}
            style={{ gridTemplateColumns: `repeat(${row.length}, 1fr)` }}
          >
            {row.map((cell: LayoutGridCell, colIndex: number) => {
              const isCellHalf = cell.positionId && isHalfPosition(cell.positionType);
              const fullStatus = cell.positionId ? fullSlotStatus({ status: cell.positionStatus ?? '', racks: cell.racks }) : 'free';

              const cellClassName = [
                styles.mapGridCell,
                cell.type === 'empty' ? styles.mapGridCellEmpty : '',
                cell.type === 'label' ? styles.mapGridCellLabel : '',
                cell.type === 'plenum' ? styles.mapGridCellPlenum : '',
                cell.type === 'position' ? styles.mapGridCellPosition : '',
                cell.type === 'position' && !isCellHalf ? styles[fullStatus] : '',
                cell.type === 'position' && isCellHalf ? styles.mapGridCellHalf : '',
              ].filter(Boolean).join(' ');

              if (cell.type === 'empty') {
                return <div key={colIndex} className={cellClassName} />;
              }

              if (cell.type === 'label') {
                return (
                  <div key={colIndex} className={cellClassName} title={cell.text}>
                    <strong>L</strong>
                  </div>
                );
              }

              if (cell.type === 'plenum') {
                return (
                  <div key={colIndex} className={cellClassName} title={cell.plenumName ?? 'Plenum'}>
                    <strong>P</strong>
                  </div>
                );
              }

              // Position cell
              if (cell.type === 'position' && !cell.positionId) {
                return (
                  <div
                    key={colIndex}
                    className={`${styles.mapGridCell} ${styles.incomplete}`}
                    title={`Posizione ${cell.pos} non configurata nel database Grappa (Isola ID: ${block.isletId ?? 'N/D'})`}
                  >
                    <strong>{cell.pos}</strong>
                  </div>
                );
              }

              if (isCellHalf) {
                const rackA = rackAt(cell.racks, 'A');
                const rackB = rackAt(cell.racks, 'B');
                const posObj: Position = {
                  id: cell.positionId!,
                  status: cell.positionStatus ?? 'Libera',
                  type: 'Half',
                  num: cell.pos ?? 0,
                  isletId: block.isletId ?? 0,
                  racks: cell.racks ?? [],
                };

                return (
                  <div key={colIndex} className={cellClassName} title={`Posizione ${cell.pos} (Divisa)`}>
                    <strong>{cell.pos}</strong>
                    <div className={styles.mapGridHalfStack}>
                      <div
                        className={`${styles.mapGridHalfSlot} ${styles[slotStatus(cell.positionStatus, rackA)] || ''}`}
                        onMouseEnter={() => setHovered({ position: posObj, rack: rackA, side: 'A' })}
                        onMouseLeave={() => setHovered(null)}
                      />
                      <div
                        className={`${styles.mapGridHalfSlot} ${styles[slotStatus(cell.positionStatus, rackB)] || ''}`}
                        onMouseEnter={() => setHovered({ position: posObj, rack: rackB, side: 'B' })}
                        onMouseLeave={() => setHovered(null)}
                      />
                    </div>
                  </div>
                );
              }

              // Full position
              const rack = fullRack(cell.racks);
              const posObj: Position = {
                id: cell.positionId!,
                status: cell.positionStatus ?? 'Libera',
                type: 'Full',
                num: cell.pos ?? 0,
                isletId: block.isletId ?? 0,
                racks: cell.racks ?? [],
              };

              return (
                <div
                  key={colIndex}
                  className={cellClassName}
                  onMouseEnter={() => setHovered({ position: posObj, rack })}
                  onMouseLeave={() => setHovered(null)}
                >
                  <strong>{cell.pos}</strong>
                  {isSpacious && rack?.name && (
                    <span className={styles.mapGridCellStatus}>
                      {rack.name}
                    </span>
                  )}
                </div>
              );
            })}
          </div>
        ))}
      </div>
    </section>
  );

  return (
    <div className={styles.panel}>
      <div className={styles.header}>
        <div>
          <h2 className={styles.panelTitle}>{data.datacenter.name}</h2>
          <p className={styles.panelSubtitle}>
            {data.datacenter.floor ? `Piano ${data.datacenter.floor}` : ''}
            {data.datacenter.floor && data.datacenter.buildingName ? ' · ' : ''}
            {data.datacenter.buildingName ? `${data.datacenter.buildingName}` : 'Edifici terzi'}
            {data.datacenter.address ? ` (${data.datacenter.address})` : ''}
          </p>
        </div>
        {canOperate ? (
          <Button size="sm" variant="secondary" onClick={onEdit} leftIcon={<Icon name="pencil" size={14} />}>
            Modifica
          </Button>
        ) : null}
      </div>

      <div className={styles.emptyText} style={{ fontSize: '0.8125rem', marginBottom: 'var(--space-4)' }}>
        {panelNote}
      </div>

      <div className={`${styles.telemetryBar} ${hovered ? styles.telemetryActive : ''}`}>
        {hovered ? (
          <>
            <div className={`${styles.telemetryStatus} ${styles[hoveredStatus] || ''}`} />
            <div className={styles.telemetryContent}>
              <div className={styles.telemetryHeaderRow}>
                <span className={styles.telemetryLabel}>
                  Posizione {String(hovered.position.num).padStart(2, '0')}{hovered.side ? ` · ${hovered.side}` : ''}
                </span>
                <span className={`${styles.telemetryBadge} ${styles[hoveredStatus] || ''}`}>
                  {positionStatusLabel(hoveredStatus)}
                </span>
              </div>
              <strong className={styles.telemetryTitle}>
                {hovered.rack?.name ?? 'Posizione vuota'}
              </strong>
              <span className={styles.telemetrySubtitle}>
                {hovered.side === 'A'
                  ? 'Mezzo alto (Half)'
                  : hovered.side === 'B'
                  ? 'Mezzo basso (Half)'
                  : hovered.position.type}
              </span>
            </div>
          </>
        ) : (
          <div className={styles.telemetryPlaceholder}>
            <Icon name="info" size={16} />
            <span>{hasRackFallback ? 'Sala senza mappa spaziale: i rack sono mostrati come elenco operativo.' : 'Passa il mouse su una posizione per ispezionare il rack'}</span>
          </div>
        )}
      </div>

      {hasRackFallback ? (
        <div className={`${styles.mapGridBlocks} ${isSpacious ? styles.mapGridBlocksSpacious : ''}`}>
          <FallbackRackTable racks={fallbackRacks} />
        </div>
      ) : data.blocks.length === 0 ? (
        <div className={styles.emptyPanel} style={{ padding: 'var(--space-8) 0' }}>
          <h3 className={styles.emptyTitle}>Mappa non configurata</h3>
          <p className={styles.emptyText} style={{ fontSize: '0.8125rem' }}>
            Questa sala non ha una disposizione spaziale di corridoi o posizioni configurata.
          </p>
        </div>
      ) : useRegions ? (
        <div className={`${styles.mapGridLayout} ${isSpacious ? styles.mapGridBlocksSpacious : ''}`}>
          <div className={styles.mapGridLayoutMain}>{mainBlocks.map(renderBlock)}</div>
          {rightBlocks.length > 0 && (
            <div className={styles.mapGridLayoutRight}>{rightBlocks.map(renderBlock)}</div>
          )}
          {bottomBlocks.length > 0 && (
            <div className={styles.mapGridLayoutBottom}>{bottomBlocks.map(renderBlock)}</div>
          )}
        </div>
      ) : (
        <div className={`${styles.mapGridBlocks} ${isSpacious ? styles.mapGridBlocksSpacious : ''}`}>
          {data.blocks.map(renderBlock)}
        </div>
      )}
    </div>
  );
}

function BuildingModal({ open, value, onClose, onSave, loading }: { open: boolean; value: Building | null; onClose: () => void; onSave: (value: BuildingInput & { id?: number }) => void; loading: boolean }) {
  const [draft, setDraft] = useState<BuildingInput>({ name: '', address: '', status: 'Attivo', portalEnabled: false, rackCapacity: 0 });

  useEffect(() => {
    setDraft(value ? { name: value.name, address: value.address, status: value.status, portalEnabled: value.portalEnabled, rackCapacity: value.rackCapacity } : { name: '', address: '', status: 'Attivo', portalEnabled: false, rackCapacity: 0 });
  }, [value, open]);

  const isCeaseDisabled = value !== null && value.status === 'Attivo' && (value.datacenterCount > 0 || value.rackCount > 0);

  return (
    <Modal open={open} onClose={onClose} title={value ? 'Modifica edificio' : 'Nuovo edificio'} size="lg">
      <div className={styles.formGrid}>
        <TextField label="Nome" value={draft.name} onChange={(name) => setDraft({ ...draft, name })} />
        <TextField label="Indirizzo" value={draft.address} onChange={(address) => setDraft({ ...draft, address })} />
        <SelectField
          label="Stato"
          value={draft.status}
          onChange={(status) => setDraft({ ...draft, status })}
          options={['Attivo', 'Cessato']}
          disabledOptions={isCeaseDisabled ? ['Cessato'] : undefined}
        />
        <NumberField label="Capienza rack" value={draft.rackCapacity} onChange={(rackCapacity) => setDraft({ ...draft, rackCapacity })} />
        <label className={styles.checkboxLine}><input type="checkbox" checked={draft.portalEnabled} onChange={(event) => setDraft({ ...draft, portalEnabled: event.target.checked })} /> Portale clienti</label>
      </div>
      <div className={styles.modalActions}>
        <Button variant="secondary" onClick={onClose}>Annulla</Button>
        <Button loading={loading} onClick={() => onSave({ ...draft, id: value?.id })}>Salva</Button>
      </div>
    </Modal>
  );
}

function DatacenterDrawer({
  open,
  value,
  onClose,
  onSave,
  loading,
  onCease,
  onDelete,
}: {
  open: boolean;
  value: Datacenter | null;
  onClose: () => void;
  onSave: (value: DatacenterInput & { id?: number }) => void;
  loading: boolean;
  onCease: (item: Datacenter) => void;
  onDelete: (item: Datacenter) => void;
}) {
  const [draft, setDraft] = useState<DatacenterInput>({
    name: '',
    address: '',
    rackCapacity: 0,
    status: 'Attivo',
    portalEnabled: false,
    isMmr: false,
  });

  useEffect(() => {
    setDraft(
      value
        ? {
            name: value.name,
            address: value.address,
            note: value.note,
            rackCapacity: value.rackCapacity,
            status: value.status ?? 'Attivo',
            customerId: value.customerId,
            portalEnabled: value.portalEnabled,
            orderCode: value.orderCode,
            buildingId: value.buildingId,
            isMmr: value.isMmr,
            setOrder: value.setOrder,
            mmrType: value.mmrType,
            serialNumber: value.serialNumber,
            floor: value.floor,
          }
        : { name: '', address: '', rackCapacity: 0, status: 'Attivo', portalEnabled: false, isMmr: false }
    );
  }, [value, open]);

  return (
    <Drawer
      open={open}
      onClose={onClose}
      title={value ? 'Modifica sala' : 'Nuova sala'}
      size="md"
      footer={
        <div className={styles.modalActions}>
          <Button variant="secondary" onClick={onClose}>
            Annulla
          </Button>
          <Button loading={loading} onClick={() => onSave({ ...draft, id: value?.id })}>
            Salva
          </Button>
        </div>
      }
    >
      <div className={styles.drawerFormWrap}>
        <div className={styles.formGrid}>
          <TextField label="Nome" value={draft.name} onChange={(name) => setDraft({ ...draft, name })} />
          <TextField label="Indirizzo" value={draft.address} onChange={(address) => setDraft({ ...draft, address })} />
          <NumberField label="Capienza rack" value={draft.rackCapacity} onChange={(rackCapacity) => setDraft({ ...draft, rackCapacity })} />
          <SelectField label="Stato" value={draft.status ?? 'Attivo'} onChange={(status) => setDraft({ ...draft, status })} options={['Attivo', 'Cessato']} />
          <TextField label="Piano" value={draft.floor ?? ''} onChange={(floor) => setDraft({ ...draft, floor })} />
          <TextField label="MMR type" value={draft.mmrType ?? ''} onChange={(mmrType) => setDraft({ ...draft, mmrType })} />
          <label className={styles.checkboxLine}>
            <input type="checkbox" checked={draft.isMmr} onChange={(event) => setDraft({ ...draft, isMmr: event.target.checked })} /> MMR
          </label>
          <label className={styles.checkboxLine}>
            <input
              type="checkbox"
              checked={draft.portalEnabled}
              onChange={(event) => setDraft({ ...draft, portalEnabled: event.target.checked })}
            />{' '}
            Portale clienti
          </label>
          <div className={`${styles.field} ${styles.fieldFull}`}>
            <label>Note</label>
            <textarea value={draft.note ?? ''} onChange={(event) => setDraft({ ...draft, note: event.target.value })} />
          </div>
        </div>

        {value && (
          <div className={styles.dangerZone}>
            <h3 className={styles.dangerZoneTitle}>Zona di Pericolo</h3>
            <p className={styles.dangerZoneText}>Operazioni distruttive ad alto impatto operativo.</p>
            <div className={styles.dangerZoneActions}>
              <Button
                size="sm"
                variant="secondary"
                onClick={() => onCease(value)}
                style={{ borderColor: 'var(--color-warning-strong)', color: 'var(--color-warning-strong)' }}
              >
                Cessa sala
              </Button>
              <Button size="sm" variant="danger" onClick={() => onDelete(value)}>
                Elimina definitivamente
              </Button>
            </div>
          </div>
        )}
      </div>
    </Drawer>
  );
}

function IsletModal({
  open,
  value,
  datacenterId,
  onClose,
  onSave,
  loading,
}: {
  open: boolean;
  value: Islet | null;
  datacenterId: number | null;
  onClose: () => void;
  onSave: (value: Partial<Islet> & { id?: number; datacenterId?: number }) => void;
  loading: boolean;
}) {
  const [draft, setDraft] = useState<Partial<Islet>>({ name: '', rackNum: 0, type: 'rack', floor: 0 });

  useEffect(() => {
    setDraft(value ? {
      name: value.name,
      rackNum: value.rackNum,
      type: value.type,
      floor: value.floor,
      serial: value.serial,
      order: value.order,
      customerId: value.customerId,
    } : { name: '', rackNum: 0, type: 'rack', floor: 0 });
  }, [value, open]);

  return (
    <Modal open={open} onClose={onClose} title={value ? 'Modifica isola' : 'Nuova isola'} size="lg">
      <div className={styles.formGrid}>
        <TextField label="Nome" value={draft.name ?? ''} onChange={(name) => setDraft({ ...draft, name })} />
        <TextField label="Tipo" value={draft.type ?? ''} onChange={(type) => setDraft({ ...draft, type })} />
        <NumberField label="Piano" value={draft.floor ?? 0} onChange={(floor) => setDraft({ ...draft, floor })} />
        <NumberField label="Posizioni previste" value={draft.rackNum ?? 0} onChange={(rackNum) => setDraft({ ...draft, rackNum })} />
        <TextField label="Seriale" value={draft.serial ?? ''} onChange={(serial) => setDraft({ ...draft, serial })} />
        <TextField label="Ordine" value={draft.order ?? ''} onChange={(order) => setDraft({ ...draft, order })} />
      </div>
      <div className={styles.modalActions}>
        <Button variant="secondary" onClick={onClose}>Annulla</Button>
        <Button
          loading={loading}
          disabled={!value && !datacenterId}
          onClick={() => onSave({ ...draft, id: value?.id, datacenterId: value?.datacenterId ?? datacenterId ?? undefined })}
        >
          Salva
        </Button>
      </div>
    </Modal>
  );
}

function PositionModal({
  open,
  value,
  onClose,
  onSave,
  loading,
}: {
  open: boolean;
  value: Position | null;
  onClose: () => void;
  onSave: (value: Partial<Position> & { id: number }) => void;
  loading: boolean;
}) {
  const [draft, setDraft] = useState<Partial<Position>>({ status: 'free', type: 'full', num: 1 });

  useEffect(() => {
    setDraft(value ? { status: value.status, type: value.type, num: value.num } : { status: 'free', type: 'full', num: 1 });
  }, [value, open]);

  return (
    <Modal open={open} onClose={onClose} title="Modifica posizione">
      <div className={styles.formGrid}>
        <NumberField label="Numero" value={draft.num ?? 1} onChange={(num) => setDraft({ ...draft, num })} />
        <TextField label="Formato" value={draft.type ?? ''} onChange={(type) => setDraft({ ...draft, type })} />
        <SelectField label="Stato" value={draft.status ?? 'free'} onChange={(status) => setDraft({ ...draft, status })} options={['free', 'reserved', 'occupied']} />
      </div>
      <div className={styles.modalActions}>
        <Button variant="secondary" onClick={onClose}>Annulla</Button>
        <Button loading={loading} disabled={!value} onClick={() => value && onSave({ ...draft, id: value.id })}>Salva</Button>
      </div>
    </Modal>
  );
}

function ConfirmModal({ open, title, message, onClose, onConfirm, loading }: { open: boolean; title: string; message: string; onClose: () => void; onConfirm: () => void; loading: boolean }) {
  const [first, setFirst] = useState(false);
  const [second, setSecond] = useState(false);
  useEffect(() => {
    if (open) {
      setFirst(false);
      setSecond(false);
    }
  }, [open]);
  return (
    <Modal open={open} onClose={onClose} title={title}>
      <p className={styles.emptyText}>{message}</p>
      <label className={styles.checkboxLine}><input type="checkbox" checked={first} onChange={(event) => setFirst(event.target.checked)} /> Ho verificato le dipendenze operative.</label>
      <label className={styles.checkboxLine}><input type="checkbox" checked={second} onChange={(event) => setSecond(event.target.checked)} /> Confermo l'azione richiesta.</label>
      <div className={styles.modalActions}>
        <Button variant="secondary" onClick={onClose}>Annulla</Button>
        <Button variant="danger" loading={loading} disabled={!first || !second} onClick={onConfirm}>Conferma</Button>
      </div>
    </Modal>
  );
}

function TextField({ label, value, onChange }: { label: string; value: string; onChange: (value: string) => void }) {
  return <div className={styles.field}><label>{label}</label><input value={value} onChange={(event) => onChange(event.target.value)} /></div>;
}

function NumberField({ label, value, onChange }: { label: string; value: number; onChange: (value: number) => void }) {
  return <div className={styles.field}><label>{label}</label><input type="number" value={value} onChange={(event) => onChange(Number(event.target.value))} /></div>;
}

function SelectField({
  label,
  value,
  onChange,
  options,
  disabledOptions,
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
  options: string[];
  disabledOptions?: string[];
}) {
  return (
    <div className={styles.field}>
      <label>{label}</label>
      <select value={value} onChange={(event) => onChange(event.target.value)}>
        {options.map((option) => (
          <option key={option} value={option} disabled={disabledOptions?.includes(option)}>
            {option}
          </option>
        ))}
      </select>
    </div>
  );
}

function positionStatusLabel(status: string) {
  if (status === 'free') return 'libera';
  if (status === 'occupied') return 'occupata';
  if (status === 'reserved') return 'riservata';
  if (status === 'shared') return 'condivisa';
  return valueOrDash(status);
}

function positionRackLabel(pos: string) {
  if (pos === 'A') return 'Rack mezzo alto';
  if (pos === 'B') return 'Rack mezzo basso';
  return 'Rack';
}
