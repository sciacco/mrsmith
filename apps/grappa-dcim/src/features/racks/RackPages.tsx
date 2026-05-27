import { Button, Icon, Modal, SearchInput, SingleSelect, Skeleton, useToast } from '@mrsmith/ui';
import { useEffect, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import {
  useDatacenters,
  useGrappaDCIMMeta,
  useIslets,
  usePositions,
  useRackDetail,
  useRackMutations,
  useRackPowerReadings,
  useRackPowerSummary,
  useRacks,
} from '../../api/queries';
import type { RackInput, RackListItem, RackMediaWrite, RackMoveInput, RackSocket, RackSocketInput } from '../../api/types';
import { ViewState } from '../../components/ViewState';
import styles from '../facilities/workspace.module.css';

const destructiveBody = { confirmPrimary: true, confirmSecondary: true };

function valueOrDash(value: unknown) {
  if (value === undefined || value === null || value === '') return '-';
  return String(value);
}

function errorText(error: unknown, fallback: string) {
  if (typeof error === 'object' && error && 'body' in error) {
    const body = (error as { body?: unknown }).body;
    if (typeof body === 'object' && body && 'message' in body) return String((body as { message?: unknown }).message);
  }
  return fallback;
}

export function RacksPage() {
  const params = useParams();
  const navigate = useNavigate();
  const toast = useToast();
  const meta = useGrappaDCIMMeta();
  const canOperate = Boolean(meta.data?.canOperate);
  const [q, setQ] = useState('');
  const [status, setStatus] = useState<'active' | 'all'>('active');
  const [editing, setEditing] = useState<RackListItem | null | 'new'>(null);
  const [moving, setMoving] = useState<RackListItem | null>(null);
  const [ceasing, setCeasing] = useState<RackListItem | null>(null);
  const [deleting, setDeleting] = useState<RackListItem | null>(null);
  const [editingSocket, setEditingSocket] = useState<RackSocket | null | 'new'>(null);
  const [deletingSocket, setDeletingSocket] = useState<RackSocket | null>(null);
  const [replacingMedia, setReplacingMedia] = useState<RackMediaWrite | null | 'new'>(null);
  const [tab, setTab] = useState<'summary' | 'units' | 'sockets' | 'media' | 'power' | 'history'>(
    params.rackId && window.location.pathname.endsWith('/potenza') ? 'power' : 'summary',
  );
  const selectedId = params.rackId ? Number(params.rackId) : null;
  const racks = useRacks({ q, status });
  const selected = racks.data?.find((item) => item.id === selectedId) ?? racks.data?.[0] ?? null;
  const detail = useRackDetail(selected?.id ?? null);
  const powerReadings = useRackPowerReadings(selected?.id ?? null, 1);
  const powerSummary = useRackPowerSummary(selected?.id ?? null);
  const mutations = useRackMutations();

  async function saveRack(input: RackInput & { id?: number }) {
    try {
      const result = await mutations.saveRack.mutateAsync(input);
      toast.toast(result.message || 'Rack salvato.');
      setEditing(null);
    } catch (error) {
      toast.toast(errorText(error, 'Salvataggio rack non riuscito.'), 'error');
    }
  }

  async function moveRack(input: RackMoveInput) {
    if (!moving) return;
    try {
      const result = await mutations.moveRack.mutateAsync({ id: moving.id, body: input });
      toast.toast(result.message || 'Rack spostato.');
      setMoving(null);
    } catch (error) {
      toast.toast(errorText(error, 'La posizione selezionata non e disponibile.'), 'error');
    }
  }

  async function ceaseRack() {
    if (!ceasing) return;
    try {
      const result = await mutations.ceaseRack.mutateAsync({ id: ceasing.id, body: destructiveBody });
      toast.toast(result.message || 'Rack cessato.');
      setCeasing(null);
    } catch (error) {
      toast.toast(errorText(error, 'Azione bloccata da dipendenze operative.'), 'error');
    }
  }

  async function deleteRack() {
    if (!deleting) return;
    try {
      const result = await mutations.deleteRack.mutateAsync({ id: deleting.id, body: destructiveBody });
      toast.toast(result.message || 'Rack eliminato.');
      setDeleting(null);
    } catch (error) {
      toast.toast(errorText(error, 'Eliminazione bloccata da dipendenze operative.'), 'error');
    }
  }

  async function saveSocket(input: RackSocketInput & { id?: number }) {
    if (!detail.data) return;
    try {
      const result = await mutations.saveRackSocket.mutateAsync({ rackId: detail.data.id, body: input });
      toast.toast(result.message || 'Socket salvato.');
      setEditingSocket(null);
    } catch (error) {
      toast.toast(errorText(error, 'Salvataggio socket non riuscito.'), 'error');
    }
  }

  async function deleteSocket() {
    if (!deletingSocket) return;
    try {
      const result = await mutations.deleteRackSocket.mutateAsync({ id: deletingSocket.id, body: destructiveBody });
      toast.toast(result.message || 'Socket eliminato.');
      setDeletingSocket(null);
    } catch (error) {
      toast.toast(errorText(error, 'Eliminazione bloccata da letture o dipendenze operative.'), 'error');
    }
  }

  async function replaceMedia(input: RackMediaWrite) {
    if (!detail.data) return;
    try {
      const result = await mutations.replaceRackMedia.mutateAsync({ rackId: detail.data.id, body: { items: [input] } });
      toast.toast(result.message || 'Media aggiornato.');
      setReplacingMedia(null);
    } catch (error) {
      toast.toast(errorText(error, 'Aggiornamento media non riuscito.'), 'error');
    }
  }

  return (
    <section className={styles.page}>
      <div className={styles.header}>
        <div className={styles.titleBlock}>
          <span className={styles.eyebrow}>Infrastruttura</span>
          <h1 className={styles.title}>Rack</h1>
        </div>
        {canOperate ? <Button onClick={() => setEditing('new')} leftIcon={<Icon name="plus" size={16} />}>Nuovo rack</Button> : null}
      </div>
      <div className={styles.inlineToolbar}>
        <SearchInput value={q} onChange={setQ} placeholder="Cerca rack..." />
        <SingleSelect
          options={[{ value: 'active', label: 'Solo attivi' }, { value: 'all', label: 'Tutti' }]}
          selected={status}
          onChange={(value) => setStatus((value ?? 'active') as 'active' | 'all')}
          searchable={false}
        />
      </div>
      {racks.isLoading ? (
        <div className={styles.panel}><Skeleton rows={8} /></div>
      ) : racks.error ? (
        <ViewState title="Rack non disponibili" message="Non e stato possibile caricare il registro rack." tone="error" />
      ) : (racks.data?.length ?? 0) === 0 ? (
        <div className={styles.emptyPanel}><h3 className={styles.emptyTitle}>Nessun rack trovato</h3><p className={styles.emptyText}>Modifica i filtri o aggiungi un rack operativo.</p></div>
      ) : (
        <div className={styles.split}>
          <div className={styles.tableWrap}>
            <table className={styles.table}>
              <thead><tr><th>Rack</th><th>Sala</th><th>Formato</th><th>Unita</th><th>Socket</th><th>Azioni</th></tr></thead>
              <tbody>
                {racks.data?.map((item) => (
                  <tr key={item.id} className={`${styles.clickable} ${selected?.id === item.id ? styles.selectedRow : ''}`} onClick={() => navigate(`/rack/${item.id}`)}>
                    <td><strong>{item.name}</strong><br /><span className={styles.muted}>{item.serialNumber ?? item.orderCode ?? '-'}</span></td>
                    <td>{item.datacenterName ?? '-'}<br /><span className={styles.muted}>{item.buildingName ?? ''}</span></td>
                    <td>{rackPositionLabel(item.type, item.position)}</td>
                    <td>{item.unitCount}</td>
                    <td>{item.socketCount}</td>
                    <td>
                      <div className={styles.actions}>
                        {canOperate ? <Button size="sm" variant="secondary" onClick={(event) => { event.stopPropagation(); setEditing(item); }}>Modifica</Button> : null}
                        {canOperate ? <Button size="sm" variant="secondary" onClick={(event) => { event.stopPropagation(); setMoving(item); }}>Sposta</Button> : null}
                        {canOperate ? <Button size="sm" variant="danger" onClick={(event) => { event.stopPropagation(); setCeasing(item); }}>Cessa</Button> : null}
                        {canOperate ? <Button size="sm" variant="danger" onClick={(event) => { event.stopPropagation(); setDeleting(item); }}>Elimina</Button> : null}
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          <div className={styles.panel}>
            {detail.isLoading ? <Skeleton rows={8} /> : detail.error || !detail.data ? (
              <div className={styles.emptyPanel}><h3 className={styles.emptyTitle}>Dettaglio non disponibile</h3><p className={styles.emptyText}>Seleziona un rack dal registro.</p></div>
            ) : (
              <>
                <div className={styles.header}>
                  <div>
                    <h2 className={styles.emptyTitle}>{detail.data.name}</h2>
                    <p className={styles.emptyText}>{detail.data.datacenterName ?? '-'} · {rackPositionLabel(detail.data.type, detail.data.position)}</p>
                  </div>
                  <span className={styles.badgeMuted}>{detail.data.status ?? 'Stato non indicato'}</span>
                </div>
                <div className={styles.tabs}>
                  {[
                    ['summary', 'Riepilogo'],
                    ['units', 'Unita rack'],
                    ['sockets', 'Socket'],
                    ['media', 'Media'],
                    ['power', 'Potenza'],
                    ['history', 'Storico'],
                  ].map(([key, label]) => (
                    <button key={key} className={`${styles.tab} ${tab === key ? styles.tabActive : ''}`} onClick={() => setTab(key as typeof tab)}>{label}</button>
                  ))}
                </div>
                {tab === 'summary' ? <RackSummary rack={detail.data} /> : null}
                {tab === 'units' ? <RackUnits rack={detail.data} /> : null}
                {tab === 'sockets' ? <RackSockets rack={detail.data} canOperate={canOperate} onCreate={() => setEditingSocket('new')} onEdit={setEditingSocket} onDelete={setDeletingSocket} /> : null}
                {tab === 'media' ? <RackMedia rack={detail.data} canOperate={canOperate} onReplace={setReplacingMedia} /> : null}
                {tab === 'power' ? <RackPower summary={powerSummary.data} loading={powerSummary.isLoading} /> : null}
                {tab === 'history' ? <RackHistory data={powerReadings.data} loading={powerReadings.isLoading} /> : null}
              </>
            )}
          </div>
        </div>
      )}
      <RackModal open={editing !== null} value={editing === 'new' ? null : editing} onClose={() => setEditing(null)} onSave={saveRack} loading={mutations.saveRack.isPending} />
      <MoveRackModal open={moving !== null} rack={moving} onClose={() => setMoving(null)} onSave={moveRack} loading={mutations.moveRack.isPending} />
      <ConfirmModal open={ceasing !== null} title="Cessa rack" message={`Confermi la cessazione di ${ceasing?.name ?? 'questo rack'}?`} onClose={() => setCeasing(null)} onConfirm={ceaseRack} loading={mutations.ceaseRack.isPending} />
      <ConfirmModal open={deleting !== null} title="Elimina rack" message={`Confermi l'eliminazione definitiva di ${deleting?.name ?? 'questo rack'}?`} onClose={() => setDeleting(null)} onConfirm={deleteRack} loading={mutations.deleteRack.isPending} />
      <RackSocketModal open={editingSocket !== null} value={editingSocket === 'new' ? null : editingSocket} onClose={() => setEditingSocket(null)} onSave={saveSocket} loading={mutations.saveRackSocket.isPending} />
      <RackMediaModal
        open={replacingMedia !== null}
        value={replacingMedia === 'new' ? null : replacingMedia}
        rack={detail.data ?? null}
        onClose={() => setReplacingMedia(null)}
        onSave={replaceMedia}
        loading={mutations.replaceRackMedia.isPending}
      />
      <ConfirmModal open={deletingSocket !== null} title="Elimina socket" message={`Confermi l'eliminazione definitiva di ${deletingSocket?.position || 'questo socket'}?`} onClose={() => setDeletingSocket(null)} onConfirm={deleteSocket} loading={mutations.deleteRackSocket.isPending} />
    </section>
  );
}

function RackSummary({ rack }: { rack: RackListItem }) {
  return (
    <div className={styles.detailGrid}>
      <Detail label="Cliente" value={rack.customerId} />
      <Detail label="Ordine" value={rack.orderCode} />
      <Detail label="Seriale" value={rack.serialNumber} />
      <Detail label="Magnetotermico" value={rack.magnetotermico} />
      <Detail label="Ampere" value={rack.ampere} />
      <Detail label="Potenza venduta" value={rack.soldPower} />
      <Detail label="Potenza impegnata" value={rack.committedPower} />
      <Detail label="Fatturazione variabile" value={rack.variableBilling === 1 ? 'Si' : rack.variableBilling === 0 ? 'No' : '-'} />
    </div>
  );
}

function RackUnits({ rack }: { rack: import('../../api/types').RackDetail }) {
  const occupied = new Set(rack.units.filter((unit) => unit.deviceId).map((unit) => unit.num));
  return (
    <div className={styles.unitGrid}>
      {rack.units.map((unit) => (
        <div key={unit.id} className={`${styles.unitCell} ${occupied.has(unit.num) ? styles.occupied : styles.free}`}>
          <strong>U{unit.num ?? '-'}</strong>
          <span>{unit.deviceId ? 'occupata' : 'libera'}</span>
        </div>
      ))}
    </div>
  );
}

function RackSockets({
  rack,
  canOperate,
  onCreate,
  onEdit,
  onDelete,
}: {
  rack: import('../../api/types').RackDetail;
  canOperate: boolean;
  onCreate: () => void;
  onEdit: (socket: RackSocket) => void;
  onDelete: (socket: RackSocket) => void;
}) {
  return (
    <div className={styles.stack}>
      {canOperate ? (
        <div className={styles.sectionHeader}>
          <span className={styles.emptyText}>{rack.sockets.length} socket configurati</span>
          <Button size="sm" variant="secondary" onClick={onCreate}>Nuovo socket</Button>
        </div>
      ) : null}
      {rack.sockets.length === 0 ? <p className={styles.emptyText}>Nessun socket configurato.</p> : null}
      <div className={styles.socketList}>
        {rack.sockets.map((socket) => (
          <div key={socket.id} className={styles.socketRow}>
            <div>
              <strong>{socket.position || `Socket ${socket.id}`}</strong>
              <p className={styles.emptyText}>{socket.magnetotermico || 'Magnetotermico non indicato'} · {socket.snmpMonitoringDevice || 'Monitor non indicato'}</p>
              <small className={styles.muted}>{[socket.oid, socket.oid2, socket.oid3, socket.oid4].filter(Boolean).join(' · ') || 'OID non indicati'}</small>
            </div>
            <div className={styles.rowEnd}>
              <span className={socket.status === 'Spento' ? styles.badgeDanger : styles.badge}>{socket.status || '-'}</span>
              {canOperate ? (
                <div className={styles.actions}>
                  <Button size="sm" variant="secondary" onClick={() => onEdit(socket)}>Modifica</Button>
                  <Button size="sm" variant="danger" onClick={() => onDelete(socket)}>Elimina</Button>
                </div>
              ) : null}
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

function RackMedia({
  rack,
  canOperate,
  onReplace,
}: {
  rack: import('../../api/types').RackDetail;
  canOperate: boolean;
  onReplace: (media: RackMediaWrite | 'new') => void;
}) {
  return (
    <div className={styles.stack}>
      {canOperate ? (
        <div className={styles.sectionHeader}>
          <span className={styles.emptyText}>{rack.media.length} media collegati</span>
          <Button size="sm" variant="secondary" onClick={() => onReplace('new')}>Sostituisci media</Button>
        </div>
      ) : null}
      {rack.media.length === 0 ? <p className={styles.emptyText}>Nessun media collegato alle unita.</p> : (
        <div className={styles.tableWrap}>
          <table className={styles.table}>
            <thead><tr><th>Unita</th><th>Lato</th><th>Percorso</th><th>Azioni</th></tr></thead>
            <tbody>{rack.media.map((item) => (
              <tr key={item.id}>
                <td>{item.unitId}</td>
                <td>{mediaSideLabel(item.side)}</td>
                <td>{item.path ?? '-'}</td>
                <td>
                  {canOperate && item.unitId ? (
                    <Button size="sm" variant="secondary" onClick={() => onReplace({ unitId: item.unitId!, side: item.side ?? 'front', path: item.path ?? '' })}>
                      Sostituisci
                    </Button>
                  ) : null}
                </td>
              </tr>
            ))}</tbody>
          </table>
        </div>
      )}
    </div>
  );
}

function RackPower({ summary, loading }: { summary?: import('../../api/types').RackPowerSummaryPoint[]; loading: boolean }) {
  if (loading) return <Skeleton rows={5} />;
  if (!summary?.length) return <p className={styles.emptyText}>Nessun riepilogo potenza disponibile.</p>;
  return (
    <div className={styles.tableWrap}>
      <table className={styles.table}>
        <thead><tr><th>Giorno</th><th>Kilowatt</th></tr></thead>
        <tbody>{summary.slice(0, 12).map((item, index) => <tr key={`${item.day}-${index}`}><td>{item.day ?? '-'}</td><td>{item.kilowatt ?? '-'}</td></tr>)}</tbody>
      </table>
    </div>
  );
}

function RackHistory({ data, loading }: { data?: import('../../api/types').RackPowerReadingsResponse; loading: boolean }) {
  if (loading) return <Skeleton rows={6} />;
  if (!data?.items.length) return <p className={styles.emptyText}>Nessuna lettura disponibile.</p>;
  return (
    <div className={styles.tableWrap}>
      <table className={styles.table}>
        <thead><tr><th>Data</th><th>OID</th><th>Ampere</th></tr></thead>
        <tbody>{data.items.map((item) => <tr key={item.id}><td>{item.date}</td><td>{item.oid}</td><td>{item.ampere}</td></tr>)}</tbody>
      </table>
    </div>
  );
}

function RackModal({ open, value, onClose, onSave, loading }: { open: boolean; value: RackListItem | null; onClose: () => void; onSave: (value: RackInput & { id?: number }) => void; loading: boolean }) {
  const datacenters = useDatacenters({ kind: 'all', status: 'active' });
  const [draft, setDraft] = useState<RackInput>({ name: '', unitCount: 42, datacenterId: 0, type: 'Full', position: 'F', status: 'Attivo', socketCount: 0 });

  useEffect(() => {
    setDraft(value ? {
      name: value.name,
      unitCount: value.unitCount,
      customerId: value.customerId,
      datacenterId: value.datacenterId,
      status: value.status ?? 'Attivo',
      magnetotermico: value.magnetotermico,
      ampere: value.ampere,
      floor: value.floor,
      island: value.island,
      type: value.type ?? 'Full',
      position: value.position ?? 'F',
      rackNumber: value.rackNumber,
      positionId: value.positionId,
      isletId: value.isletId,
      shared: value.shared,
      reserved: value.reserved,
      note: value.note,
      orderCode: value.orderCode,
      soldPower: value.soldPower,
      serialNumber: value.serialNumber,
      committedPower: value.committedPower,
      variableBilling: value.variableBilling,
      socketCount: value.socketCount,
    } : { name: '', unitCount: 42, datacenterId: datacenters.data?.[0]?.id ?? 0, type: 'Full', position: 'F', status: 'Attivo', socketCount: 0 });
  }, [value, open, datacenters.data]);

  return (
    <Modal open={open} onClose={onClose} title={value ? 'Modifica rack' : 'Nuovo rack'} size="wide">
      <div className={styles.formGrid}>
        <TextField label="Nome" value={draft.name} onChange={(name) => setDraft({ ...draft, name })} />
        <div className={styles.field}>
          <label>Sala</label>
          <select value={draft.datacenterId} onChange={(event) => setDraft({ ...draft, datacenterId: Number(event.target.value) })}>
            <option value={0}>Seleziona sala</option>
            {datacenters.data?.map((item) => <option key={item.id} value={item.id}>{item.name}</option>)}
          </select>
        </div>
        <NumberField label="Unita" value={draft.unitCount} onChange={(unitCount) => setDraft({ ...draft, unitCount })} />
        <SelectField label="Formato" value={draft.type} onChange={(type) => setDraft({ ...draft, type, position: type === 'Full' ? 'F' : 'A' })} options={['Full', 'Half']} />
        <SelectField label="Posizione" value={draft.position} onChange={(position) => setDraft({ ...draft, position })} options={draft.type === 'Full' ? ['F'] : ['A', 'B']} />
        <NumberField label="Socket iniziali" value={draft.socketCount ?? 0} onChange={(socketCount) => setDraft({ ...draft, socketCount })} />
        <TextField label="Seriale" value={draft.serialNumber ?? ''} onChange={(serialNumber) => setDraft({ ...draft, serialNumber })} />
        <TextField label="Codice ordine" value={draft.orderCode ?? ''} onChange={(orderCode) => setDraft({ ...draft, orderCode })} />
      </div>
      <div className={styles.modalActions}>
        <Button variant="secondary" onClick={onClose}>Annulla</Button>
        <Button loading={loading} onClick={() => onSave({ ...draft, id: value?.id })}>Salva</Button>
      </div>
    </Modal>
  );
}

function MoveRackModal({ open, rack, onClose, onSave, loading }: { open: boolean; rack: RackListItem | null; onClose: () => void; onSave: (value: RackMoveInput) => void; loading: boolean }) {
  const [datacenterId, setDatacenterId] = useState<number | null>(rack?.datacenterId ?? null);
  const [isletId, setIsletId] = useState<number | null>(rack?.isletId ?? null);
  const [positionId, setPositionId] = useState<number | null>(rack?.positionId ?? null);
  const [type, setType] = useState(rack?.type ?? 'Full');
  const [position, setPosition] = useState(rack?.position ?? 'F');
  const datacenters = useDatacenters({ kind: 'all', status: 'active' });
  const islets = useIslets(datacenterId);
  const positions = usePositions(isletId);

  useEffect(() => {
    setDatacenterId(rack?.datacenterId ?? null);
    setIsletId(rack?.isletId ?? null);
    setPositionId(rack?.positionId ?? null);
    setType(rack?.type ?? 'Full');
    setPosition(rack?.position ?? 'F');
  }, [rack, open]);

  return (
    <Modal open={open} onClose={onClose} title="Sposta rack">
      <div className={styles.formGrid}>
        <div className={styles.field}>
          <label>Sala</label>
          <select value={datacenterId ?? 0} onChange={(event) => { setDatacenterId(Number(event.target.value)); setIsletId(null); setPositionId(null); }}>
            <option value={0}>Seleziona sala</option>
            {datacenters.data?.map((item) => <option key={item.id} value={item.id}>{item.name}</option>)}
          </select>
        </div>
        <div className={styles.field}>
          <label>Isola</label>
          <select value={isletId ?? 0} onChange={(event) => { setIsletId(Number(event.target.value)); setPositionId(null); }}>
            <option value={0}>Seleziona isola</option>
            {islets.data?.map((item) => <option key={item.id} value={item.id}>{item.name}</option>)}
          </select>
        </div>
        <div className={styles.field}>
          <label>Posizione</label>
          <select value={positionId ?? 0} onChange={(event) => setPositionId(Number(event.target.value))}>
            <option value={0}>Seleziona posizione</option>
            {positions.data?.map((item) => <option key={item.id} value={item.id}>{item.num} - {item.status}</option>)}
          </select>
        </div>
        <SelectField label="Formato" value={type} onChange={(next) => { setType(next); setPosition(next === 'Full' ? 'F' : 'A'); }} options={['Full', 'Half']} />
        <SelectField label="Posizione verticale" value={position} onChange={setPosition} options={type === 'Full' ? ['F'] : ['A', 'B']} />
      </div>
      <div className={styles.modalActions}>
        <Button variant="secondary" onClick={onClose}>Annulla</Button>
        <Button loading={loading} disabled={!datacenterId || !positionId} onClick={() => onSave({ datacenterId: datacenterId!, positionId: positionId!, isletId: isletId ?? undefined, type, position })}>Sposta</Button>
      </div>
    </Modal>
  );
}

function RackSocketModal({
  open,
  value,
  onClose,
  onSave,
  loading,
}: {
  open: boolean;
  value: RackSocket | null;
  onClose: () => void;
  onSave: (value: RackSocketInput & { id?: number }) => void;
  loading: boolean;
}) {
  const [draft, setDraft] = useState<RackSocketInput>({ position: '', status: 'Acceso' });

  useEffect(() => {
    setDraft(value ? {
      magnetotermico: value.magnetotermico,
      snmpMonitoringDevice: value.snmpMonitoringDevice,
      detectorIp: value.detectorIp,
      oid: value.oid,
      oid2: value.oid2,
      oid3: value.oid3,
      oid4: value.oid4,
      position: value.position,
      position2: value.position2,
      position3: value.position3,
      position4: value.position4,
      status: value.status,
    } : { position: '', status: 'Acceso' });
  }, [value, open]);

  return (
    <Modal open={open} onClose={onClose} title={value ? 'Modifica socket' : 'Nuovo socket'} size="wide">
      <div className={styles.formGrid}>
        <TextField label="Posizione" value={draft.position ?? ''} onChange={(position) => setDraft({ ...draft, position })} />
        <SelectField label="Stato" value={draft.status ?? 'Acceso'} onChange={(status) => setDraft({ ...draft, status })} options={['Acceso', 'Spento']} />
        <TextField label="Magnetotermico" value={draft.magnetotermico ?? ''} onChange={(magnetotermico) => setDraft({ ...draft, magnetotermico })} />
        <TextField label="Monitor SNMP" value={draft.snmpMonitoringDevice ?? ''} onChange={(snmpMonitoringDevice) => setDraft({ ...draft, snmpMonitoringDevice })} />
        <TextField label="IP rilevatore" value={draft.detectorIp ?? ''} onChange={(detectorIp) => setDraft({ ...draft, detectorIp })} />
        <TextField label="OID 1" value={draft.oid ?? ''} onChange={(oid) => setDraft({ ...draft, oid })} />
        <TextField label="OID 2" value={draft.oid2 ?? ''} onChange={(oid2) => setDraft({ ...draft, oid2 })} />
        <TextField label="OID 3" value={draft.oid3 ?? ''} onChange={(oid3) => setDraft({ ...draft, oid3 })} />
        <TextField label="OID 4" value={draft.oid4 ?? ''} onChange={(oid4) => setDraft({ ...draft, oid4 })} />
        <TextField label="Posizione 2" value={draft.position2 ?? ''} onChange={(position2) => setDraft({ ...draft, position2 })} />
        <TextField label="Posizione 3" value={draft.position3 ?? ''} onChange={(position3) => setDraft({ ...draft, position3 })} />
        <TextField label="Posizione 4" value={draft.position4 ?? ''} onChange={(position4) => setDraft({ ...draft, position4 })} />
      </div>
      <div className={styles.modalActions}>
        <Button variant="secondary" onClick={onClose}>Annulla</Button>
        <Button loading={loading} onClick={() => onSave({ ...draft, id: value?.id })}>Salva</Button>
      </div>
    </Modal>
  );
}

function RackMediaModal({
  open,
  value,
  rack,
  onClose,
  onSave,
  loading,
}: {
  open: boolean;
  value: RackMediaWrite | null;
  rack: import('../../api/types').RackDetail | null;
  onClose: () => void;
  onSave: (value: RackMediaWrite) => void;
  loading: boolean;
}) {
  const [draft, setDraft] = useState<RackMediaWrite>({ unitId: 0, side: 'front', path: '' });

  useEffect(() => {
    const firstUnit = rack?.units[0]?.id ?? 0;
    setDraft(value ?? { unitId: firstUnit, side: 'front', path: '' });
  }, [value, open, rack]);

  return (
    <Modal open={open} onClose={onClose} title="Sostituisci media rack">
      <div className={styles.formGrid}>
        <div className={styles.field}>
          <label>Unita</label>
          <select value={draft.unitId} onChange={(event) => setDraft({ ...draft, unitId: Number(event.target.value) })}>
            <option value={0}>Seleziona unita</option>
            {rack?.units.map((unit) => <option key={unit.id} value={unit.id}>U{unit.num ?? unit.id}</option>)}
          </select>
        </div>
        <SelectField label="Lato" value={draft.side} onChange={(side) => setDraft({ ...draft, side })} options={['front', 'back']} />
        <div className={`${styles.field} ${styles.fieldFull}`}>
          <label>Percorso file</label>
          <input value={draft.path} onChange={(event) => setDraft({ ...draft, path: event.target.value })} />
        </div>
      </div>
      <div className={styles.modalActions}>
        <Button variant="secondary" onClick={onClose}>Annulla</Button>
        <Button loading={loading} disabled={!draft.unitId} onClick={() => onSave(draft)}>Salva</Button>
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

function Detail({ label, value }: { label: string; value: unknown }) {
  return <div className={styles.detailItem}><span className={styles.detailLabel}>{label}</span><span className={styles.detailValue}>{valueOrDash(value)}</span></div>;
}

function TextField({ label, value, onChange }: { label: string; value: string; onChange: (value: string) => void }) {
  return <div className={styles.field}><label>{label}</label><input value={value} onChange={(event) => onChange(event.target.value)} /></div>;
}

function NumberField({ label, value, onChange }: { label: string; value: number; onChange: (value: number) => void }) {
  return <div className={styles.field}><label>{label}</label><input type="number" value={value} onChange={(event) => onChange(Number(event.target.value))} /></div>;
}

function SelectField({ label, value, onChange, options }: { label: string; value: string; onChange: (value: string) => void; options: string[] }) {
  return <div className={styles.field}><label>{label}</label><select value={value} onChange={(event) => onChange(event.target.value)}>{options.map((option) => <option key={option} value={option}>{selectOptionLabel(option)}</option>)}</select></div>;
}

function rackPositionLabel(type?: string, position?: string) {
  if (type === 'Half' && position === 'A') return 'Half, posizione alta';
  if (type === 'Half' && position === 'B') return 'Half, posizione bassa';
  if (type === 'Full') return 'Full';
  return `${valueOrDash(type)} ${valueOrDash(position)}`;
}

function mediaSideLabel(side?: string) {
  if (side === 'front') return 'Fronte';
  if (side === 'back') return 'Retro';
  return valueOrDash(side);
}

function selectOptionLabel(option: string) {
  if (option === 'A') return 'A - posizione alta';
  if (option === 'B') return 'B - posizione bassa';
  if (option === 'front') return 'Fronte';
  if (option === 'back') return 'Retro';
  return option;
}
