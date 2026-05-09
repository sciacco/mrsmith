import { Button, Icon, Modal, SearchInput, SingleSelect, Skeleton, useToast } from '@mrsmith/ui';
import { useEffect, useMemo, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import {
  useBuildings,
  useDatacenterMap,
  useDatacenters,
  useFacilitiesMutations,
  useGrappaDCIMMeta,
  useIslets,
  useLayoutMutations,
  usePositions,
} from '../../api/queries';
import type { Building, BuildingInput, Datacenter, DatacenterInput, DatacenterMap, Islet, Position } from '../../api/types';
import { ViewState } from '../../components/ViewState';
import styles from './workspace.module.css';

const destructiveBody = { confirmPrimary: true, confirmSecondary: true };

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
          <p className={styles.subtitle}>Registro delle sedi tecniche e della loro esposizione verso il portale clienti.</p>
        </div>
        {canOperate ? (
          <Button onClick={() => setEditing('new')} leftIcon={<Icon name="plus" size={16} />}>Nuovo edificio</Button>
        ) : null}
      </div>
      <div className={styles.toolbar}>
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
                  <td><strong>{item.name}</strong></td>
                  <td>{item.address}</td>
                  <td><StatusBadge value={item.status} /></td>
                  <td>{item.portalEnabled ? 'Si' : 'No'}</td>
                  <td>{item.datacenterCount}</td>
                  <td>{item.rackCount} / {item.rackCapacity}</td>
                  <td>
                    <div className={styles.actions}>
                      {canOperate ? <Button size="sm" variant="secondary" onClick={() => setEditing(item)}>Modifica</Button> : null}
                      {canOperate ? <Button size="sm" variant="danger" onClick={() => setCeasing(item)}>Cessa</Button> : null}
                      {canOperate ? <Button size="sm" variant="danger" onClick={() => setDeleting(item)}>Elimina</Button> : null}
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
  const [editing, setEditing] = useState<Datacenter | null | 'new'>(null);
  const [ceasing, setCeasing] = useState<Datacenter | null>(null);
  const [deleting, setDeleting] = useState<Datacenter | null>(null);
  const selectedId = params.datacenterId ? Number(params.datacenterId) : null;
  const query = useDatacenters({ q, kind, status });
  const selected = query.data?.find((item) => item.id === selectedId) ?? query.data?.[0] ?? null;
  const map = useDatacenterMap(selected?.id ?? null);
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
          <p className={styles.subtitle}>Sale, cage e MMR con mappa fisica di isole, posizioni e rack.</p>
        </div>
        {canOperate ? <Button onClick={() => setEditing('new')} leftIcon={<Icon name="plus" size={16} />}>Nuova sala</Button> : null}
      </div>
      <div className={styles.toolbar}>
        <SearchInput value={q} onChange={setQ} placeholder="Cerca sala o MMR..." />
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
      ) : (query.data?.length ?? 0) === 0 ? (
        <div className={styles.emptyPanel}><h3 className={styles.emptyTitle}>Nessuna sala trovata</h3><p className={styles.emptyText}>Modifica i filtri o aggiungi una nuova sala.</p></div>
      ) : (
        <div className={styles.split}>
          <div className={styles.tableWrap}>
            <table className={styles.table}>
              <thead>
                <tr><th>Nome</th><th>Tipo</th><th>Edificio</th><th>Stato</th><th>Rack</th><th>Azioni</th></tr>
              </thead>
              <tbody>
                {query.data?.map((item) => (
                  <tr key={item.id} className={`${styles.clickable} ${selected?.id === item.id ? styles.selectedRow : ''}`} onClick={() => navigate(`/sale-mmr/${item.id}`)}>
                    <td><strong>{item.name}</strong><br /><span className={styles.muted}>{item.floor ? `Piano ${item.floor}` : item.address}</span></td>
                    <td>{item.isMmr ? <span className={styles.badge}>MMR {item.mmrType ?? ''}</span> : <span className={styles.badgeMuted}>Sala</span>}</td>
                    <td>{item.buildingName ?? '-'}</td>
                    <td><StatusBadge value={item.status} /></td>
                    <td>{item.rackCount} / {item.rackCapacity}</td>
                    <td>
                      <div className={styles.actions}>
                        {canOperate ? <Button size="sm" variant="secondary" onClick={(event) => { event.stopPropagation(); setEditing(item); }}>Modifica</Button> : null}
                        {canOperate ? <Button size="sm" variant="danger" onClick={(event) => { event.stopPropagation(); setCeasing(item); }}>Cessa</Button> : null}
                        {canOperate ? <Button size="sm" variant="danger" onClick={(event) => { event.stopPropagation(); setDeleting(item); }}>Elimina</Button> : null}
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          <DatacenterMapPanel data={map.data} loading={map.isLoading} error={map.error} />
        </div>
      )}
      <DatacenterModal open={editing !== null} value={editing === 'new' ? null : editing} onClose={() => setEditing(null)} onSave={saveDatacenter} loading={mutations.saveDatacenter.isPending} />
      <ConfirmModal open={ceasing !== null} title="Cessa sala" message={`Confermi la cessazione di ${ceasing?.name ?? 'questa sala'}?`} onClose={() => setCeasing(null)} onConfirm={ceaseDatacenter} loading={mutations.ceaseDatacenter.isPending} />
      <ConfirmModal open={deleting !== null} title="Elimina sala" message={`Confermi l'eliminazione definitiva di ${deleting?.name ?? 'questa sala'}?`} onClose={() => setDeleting(null)} onConfirm={deleteDatacenter} loading={mutations.deleteDatacenter.isPending} />
    </section>
  );
}

export function LayoutPage() {
  const toast = useToast();
  const meta = useGrappaDCIMMeta();
  const canOperate = Boolean(meta.data?.canOperate);
  const datacenters = useDatacenters({ kind: 'all', status: 'active' });
  const [datacenterId, setDatacenterId] = useState<number | null>(null);
  const [isletId, setIsletId] = useState<number | null>(null);
  const [editingIslet, setEditingIslet] = useState<Islet | null | 'new'>(null);
  const [deletingIslet, setDeletingIslet] = useState<Islet | null>(null);
  const [editingPosition, setEditingPosition] = useState<Position | null>(null);
  const [deletingPosition, setDeletingPosition] = useState<Position | null>(null);
  const [batchCount, setBatchCount] = useState(0);
  const [batchType, setBatchType] = useState('full');
  const islets = useIslets(datacenterId);
  const positions = usePositions(isletId);
  const mutations = useLayoutMutations();

  const selectedIslet = islets.data?.find((item) => item.id === isletId) ?? null;
  const datacenterOptions = useMemo(
    () => datacenters.data?.map((item) => ({ value: item.id, label: `${item.name}${item.isMmr ? ' - MMR' : ''}` })) ?? [],
    [datacenters.data],
  );

  async function createBatch() {
    if (!isletId) return;
    try {
      const result = await mutations.createPositions.mutateAsync({ isletId, count: batchCount, type: batchType });
      toast.toast(result.message || 'Posizioni create.');
      setBatchCount(0);
    } catch (error) {
      toast.toast(errorText(error, 'Creazione posizioni non riuscita.'), 'error');
    }
  }

  async function saveIslet(input: Partial<Islet> & { id?: number; datacenterId?: number }) {
    try {
      const result = await mutations.saveIslet.mutateAsync(input);
      toast.toast(result.message || 'Isola salvata.');
      setEditingIslet(null);
      if (result.id && !input.id) setIsletId(result.id);
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
      if (isletId === deletingIslet.id) setIsletId(null);
    } catch (error) {
      toast.toast(errorText(error, 'Eliminazione bloccata da dipendenze operative.'), 'error');
    }
  }

  async function savePosition(input: Partial<Position> & { id: number }) {
    try {
      const result = await mutations.savePosition.mutateAsync(input);
      toast.toast(result.message || 'Posizione salvata.');
      setEditingPosition(null);
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
          <p className={styles.subtitle}>Amministrazione delle isole fisiche e delle posizioni rack disponibili.</p>
        </div>
        {canOperate ? <Button onClick={() => setEditingIslet('new')} disabled={!datacenterId} leftIcon={<Icon name="plus" size={16} />}>Nuova isola</Button> : null}
      </div>
      <div className={styles.toolbar}>
        <SingleSelect options={datacenterOptions} selected={datacenterId} onChange={(value) => { setDatacenterId(value); setIsletId(null); }} placeholder="Seleziona sala" />
        <SingleSelect options={islets.data?.map((item) => ({ value: item.id, label: item.name })) ?? []} selected={isletId} onChange={setIsletId} placeholder="Seleziona isola" disabled={!datacenterId} />
      </div>
      <div className={styles.split}>
        <div className={styles.panel}>
          <div className={styles.sectionHeader}>
            <h2 className={styles.emptyTitle}>Isole</h2>
            {canOperate ? <Button size="sm" variant="secondary" onClick={() => setEditingIslet('new')} disabled={!datacenterId}>Nuova</Button> : null}
          </div>
          {islets.isLoading ? <Skeleton rows={5} /> : (islets.data?.length ?? 0) === 0 ? (
            <p className={styles.emptyText}>Nessuna isola configurata per la sala selezionata.</p>
          ) : (
            <div className={styles.tableWrap}>
              <table className={styles.table}>
                <thead><tr><th>Isola</th><th>Tipo</th><th>Piano</th><th>Posizioni</th><th>Azioni</th></tr></thead>
                <tbody>
                  {islets.data?.map((item) => (
                    <tr key={item.id} className={`${styles.clickable} ${isletId === item.id ? styles.selectedRow : ''}`} onClick={() => setIsletId(item.id)}>
                      <td><strong>{item.name}</strong></td>
                      <td>{item.type}</td>
                      <td>{item.floor}</td>
                      <td>{item.occupiedCount} / {item.positionCount || item.rackNum}</td>
                      <td>
                        <div className={styles.actions}>
                          {canOperate ? <Button size="sm" variant="secondary" onClick={(event) => { event.stopPropagation(); setEditingIslet(item); }}>Modifica</Button> : null}
                          {canOperate ? <Button size="sm" variant="danger" onClick={(event) => { event.stopPropagation(); setDeletingIslet(item); }}>Elimina</Button> : null}
                        </div>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>
        <div className={styles.panel}>
          <div className={styles.sectionHeader}>
            <h2 className={styles.emptyTitle}>{selectedIslet ? `Posizioni ${selectedIslet.name}` : 'Posizioni'}</h2>
            {selectedIslet ? <span className={styles.badgeMuted}>{positions.data?.length ?? 0} posizioni</span> : null}
          </div>
          {canOperate && selectedIslet ? (
            <div className={styles.toolbar}>
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
              <Button variant="secondary" onClick={createBatch} disabled={!batchCount || mutations.createPositions.isPending}>Crea posizioni</Button>
            </div>
          ) : null}
          {positions.isLoading ? <Skeleton rows={5} /> : (positions.data?.length ?? 0) === 0 ? (
            <p className={styles.emptyText}>Nessuna posizione configurata.</p>
          ) : (
            <div className={styles.positionGrid}>
              {positions.data?.map((item) => (
                <div key={item.id} className={`${styles.positionCell} ${item.status === 'occupied' ? styles.occupied : item.status === 'reserved' ? styles.reserved : styles.free}`}>
                  <strong>{item.num}</strong>
                  <span>{positionStatusLabel(item.status)}</span>
                  <small>{item.rackName ?? item.type}</small>
                  {canOperate ? (
                    <div className={styles.cellActions}>
                      <Button size="sm" variant="secondary" onClick={() => setEditingPosition(item)}>Modifica</Button>
                      <Button size="sm" variant="danger" onClick={() => setDeletingPosition(item)}>Elimina</Button>
                    </div>
                  ) : null}
                </div>
              ))}
            </div>
          )}
        </div>
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

function DatacenterMapPanel({ data, loading, error }: { data?: DatacenterMap; loading: boolean; error: unknown }) {
  if (loading) return <div className={styles.panel}><Skeleton rows={8} /></div>;
  if (error || !data) return <div className={styles.emptyPanel}><h3 className={styles.emptyTitle}>Mappa non disponibile</h3><p className={styles.emptyText}>Seleziona una sala per visualizzare il layout.</p></div>;
  return (
    <div className={styles.panel}>
      <div className={styles.header}>
        <div>
          <h2 className={styles.emptyTitle}>{data.datacenter.name}</h2>
          <p className={styles.emptyText}>{data.incomplete ? 'Configurazione incompleta: mancano posizioni per le isole presenti.' : `${data.positions.length} posizioni visibili`}</p>
        </div>
        <span className={styles.badgeMuted}>{data.racks.length} rack</span>
      </div>
      <div className={styles.mapGrid}>
        {data.positions.length === 0 ? (
          <p className={styles.emptyText}>Nessuna posizione configurata.</p>
        ) : data.positions.map((position) => (
          <div key={position.id} className={`${styles.mapCell} ${position.status === 'occupied' ? styles.occupied : position.status === 'reserved' ? styles.reserved : styles.free}`}>
            <strong>{position.num}</strong>
            <span>{position.rackName ?? positionStatusLabel(position.status)}</span>
            <small>{position.rackType === 'Half' && position.rackPos === 'A' ? 'posizione alta' : position.rackType === 'Half' && position.rackPos === 'B' ? 'posizione bassa' : position.type}</small>
          </div>
        ))}
      </div>
    </div>
  );
}

function BuildingModal({ open, value, onClose, onSave, loading }: { open: boolean; value: Building | null; onClose: () => void; onSave: (value: BuildingInput & { id?: number }) => void; loading: boolean }) {
  const [draft, setDraft] = useState<BuildingInput>({ name: '', address: '', status: 'Attivo', portalEnabled: false, rackCapacity: 0 });

  useEffect(() => {
    setDraft(value ? { name: value.name, address: value.address, status: value.status, portalEnabled: value.portalEnabled, rackCapacity: value.rackCapacity } : { name: '', address: '', status: 'Attivo', portalEnabled: false, rackCapacity: 0 });
  }, [value, open]);

  return (
    <Modal open={open} onClose={onClose} title={value ? 'Modifica edificio' : 'Nuovo edificio'} size="lg">
      <div className={styles.formGrid}>
        <TextField label="Nome" value={draft.name} onChange={(name) => setDraft({ ...draft, name })} />
        <TextField label="Indirizzo" value={draft.address} onChange={(address) => setDraft({ ...draft, address })} />
        <SelectField label="Stato" value={draft.status} onChange={(status) => setDraft({ ...draft, status })} options={['Attivo', 'Cessato']} />
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

function DatacenterModal({ open, value, onClose, onSave, loading }: { open: boolean; value: Datacenter | null; onClose: () => void; onSave: (value: DatacenterInput & { id?: number }) => void; loading: boolean }) {
  const [draft, setDraft] = useState<DatacenterInput>({ name: '', address: '', rackCapacity: 0, status: 'Attivo', portalEnabled: false, isMmr: false });

  useEffect(() => {
    setDraft(value ? { name: value.name, address: value.address, note: value.note, rackCapacity: value.rackCapacity, status: value.status ?? 'Attivo', customerId: value.customerId, portalEnabled: value.portalEnabled, orderCode: value.orderCode, buildingId: value.buildingId, isMmr: value.isMmr, setOrder: value.setOrder, mmrType: value.mmrType, serialNumber: value.serialNumber, floor: value.floor } : { name: '', address: '', rackCapacity: 0, status: 'Attivo', portalEnabled: false, isMmr: false });
  }, [value, open]);

  return (
    <Modal open={open} onClose={onClose} title={value ? 'Modifica sala' : 'Nuova sala'} size="wide">
      <div className={styles.formGrid}>
        <TextField label="Nome" value={draft.name} onChange={(name) => setDraft({ ...draft, name })} />
        <TextField label="Indirizzo" value={draft.address} onChange={(address) => setDraft({ ...draft, address })} />
        <NumberField label="Capienza rack" value={draft.rackCapacity} onChange={(rackCapacity) => setDraft({ ...draft, rackCapacity })} />
        <SelectField label="Stato" value={draft.status ?? 'Attivo'} onChange={(status) => setDraft({ ...draft, status })} options={['Attivo', 'Cessato']} />
        <TextField label="Piano" value={draft.floor ?? ''} onChange={(floor) => setDraft({ ...draft, floor })} />
        <TextField label="MMR type" value={draft.mmrType ?? ''} onChange={(mmrType) => setDraft({ ...draft, mmrType })} />
        <label className={styles.checkboxLine}><input type="checkbox" checked={draft.isMmr} onChange={(event) => setDraft({ ...draft, isMmr: event.target.checked })} /> MMR</label>
        <label className={styles.checkboxLine}><input type="checkbox" checked={draft.portalEnabled} onChange={(event) => setDraft({ ...draft, portalEnabled: event.target.checked })} /> Portale clienti</label>
        <div className={`${styles.field} ${styles.fieldFull}`}>
          <label>Note</label>
          <textarea value={draft.note ?? ''} onChange={(event) => setDraft({ ...draft, note: event.target.value })} />
        </div>
      </div>
      <div className={styles.modalActions}>
        <Button variant="secondary" onClick={onClose}>Annulla</Button>
        <Button loading={loading} onClick={() => onSave({ ...draft, id: value?.id })}>Salva</Button>
      </div>
    </Modal>
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

function SelectField({ label, value, onChange, options }: { label: string; value: string; onChange: (value: string) => void; options: string[] }) {
  return <div className={styles.field}><label>{label}</label><select value={value} onChange={(event) => onChange(event.target.value)}>{options.map((option) => <option key={option} value={option}>{option}</option>)}</select></div>;
}

function positionStatusLabel(status: string) {
  if (status === 'free') return 'libera';
  if (status === 'occupied') return 'occupata';
  if (status === 'reserved') return 'riservata';
  return valueOrDash(status);
}
