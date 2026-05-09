import { Button, Icon, Modal, SearchInput, SingleSelect, Skeleton, useToast } from '@mrsmith/ui';
import { useEffect, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { useEquipment, useGrappaDCIMMeta, useStorage, useStorageDetail, useStorageMutations } from '../../api/queries';
import type { StorageInput, StorageItem } from '../../api/types';
import { ViewState } from '../../components/ViewState';
import styles from '../facilities/workspace.module.css';
import { ConfirmModal, Detail, NumberField, SelectField, TextField, destructiveBody, errorText } from '../equipment/assetPageUtils';

export function StoragePage() {
  const params = useParams();
  const navigate = useNavigate();
  const toast = useToast();
  const meta = useGrappaDCIMMeta();
  const canOperate = Boolean(meta.data?.canOperate);
  const [q, setQ] = useState('');
  const [status, setStatus] = useState<'active' | 'all'>('active');
  const [editing, setEditing] = useState<StorageItem | null | 'new'>(null);
  const [archiving, setArchiving] = useState<StorageItem | null>(null);
  const selectedId = params.storageId ? Number(params.storageId) : null;
  const storage = useStorage({ q, status });
  const selected = storage.data?.find((item) => item.id === selectedId) ?? storage.data?.[0] ?? null;
  const detail = useStorageDetail(selected?.id ?? null);
  const mutations = useStorageMutations();

  async function saveStorage(input: StorageInput & { id?: number }) {
    try {
      const result = await mutations.saveStorage.mutateAsync(input);
      toast.toast(result.message || 'Storage salvato.');
      setEditing(null);
    } catch (error) {
      toast.toast(errorText(error, 'Salvataggio storage non riuscito.'), 'error');
    }
  }

  async function archiveStorage() {
    if (!archiving) return;
    try {
      const result = await mutations.archiveStorage.mutateAsync({ id: archiving.id, body: destructiveBody });
      toast.toast(result.message || 'Storage archiviato.');
      setArchiving(null);
    } catch (error) {
      toast.toast(errorText(error, 'Archiviazione storage non riuscita.'), 'error');
    }
  }

  return (
    <section className={styles.page}>
      <div className={styles.header}>
        <div className={styles.titleBlock}><span className={styles.eyebrow}>Asset</span><h1 className={styles.title}>Storage</h1><p className={styles.subtitle}>Allocazioni storage e archivio delle chiusure.</p></div>
        {canOperate ? <Button onClick={() => setEditing('new')} leftIcon={<Icon name="plus" size={16} />}>Nuovo storage</Button> : null}
      </div>
      <div className={styles.toolbar}><SearchInput value={q} onChange={setQ} placeholder="Cerca storage..." /><SingleSelect options={[{ value: 'active', label: 'Solo attivi' }, { value: 'all', label: 'Tutti' }]} selected={status} onChange={(value) => setStatus((value ?? 'active') as 'active' | 'all')} searchable={false} /></div>
      {storage.isLoading ? <div className={styles.panel}><Skeleton rows={8} /></div> : storage.error ? <ViewState title="Storage non disponibile" message="Non e stato possibile caricare le allocazioni storage." tone="error" /> : (storage.data?.length ?? 0) === 0 ? <div className={styles.emptyPanel}><h3 className={styles.emptyTitle}>Nessuno storage trovato</h3><p className={styles.emptyText}>Modifica i filtri o aggiungi una nuova allocazione.</p></div> : (
        <div className={styles.split}>
          <div className={styles.tableWrap}><table className={styles.table}><thead><tr><th>Storage</th><th>Cliente</th><th>Apparato</th><th>Stato</th><th>Azioni</th></tr></thead><tbody>{storage.data?.map((item) => <tr key={item.id} className={`${styles.clickable} ${selected?.id === item.id ? styles.selectedRow : ''}`} onClick={() => navigate(`/storage/${item.id}`)}><td><strong>{item.size ?? '-'} {item.sizeType ?? ''}</strong><br /><span className={styles.muted}>{item.protocol ?? item.serialNumber ?? '-'}</span></td><td>{item.customerId}</td><td>{item.equipment ?? item.equipmentId}</td><td><span className={item.readOnly ? styles.badgeDanger : styles.badge}>{item.status}</span></td><td><div className={styles.actions}>{canOperate && !item.readOnly ? <Button size="sm" variant="secondary" onClick={(event) => { event.stopPropagation(); setEditing(item); }}>Modifica</Button> : null}{canOperate && !item.readOnly ? <Button size="sm" variant="danger" onClick={(event) => { event.stopPropagation(); setArchiving(item); }}>Archivia</Button> : null}</div></td></tr>)}</tbody></table></div>
          <div className={styles.panel}>{detail.isLoading ? <Skeleton rows={8} /> : detail.error || !detail.data ? <div className={styles.emptyPanel}><h3 className={styles.emptyTitle}>Dettaglio non disponibile</h3><p className={styles.emptyText}>Seleziona una riga storage dal registro.</p></div> : <StorageDetail item={detail.data} canOperate={canOperate} onEdit={() => setEditing(detail.data)} onArchive={() => setArchiving(detail.data)} />}</div>
        </div>
      )}
      <StorageModal open={editing !== null} value={editing === 'new' ? null : editing} onClose={() => setEditing(null)} onSave={saveStorage} loading={mutations.saveStorage.isPending} />
      <ConfirmModal open={archiving !== null} title="Archivia storage" message={`Confermi l'archiviazione dello storage ${archiving?.id ?? ''}?`} onClose={() => setArchiving(null)} onConfirm={archiveStorage} loading={mutations.archiveStorage.isPending} />
    </section>
  );
}

function StorageDetail({ item, canOperate, onEdit, onArchive }: { item: StorageItem; canOperate: boolean; onEdit: () => void; onArchive: () => void }) {
  return <><div className={styles.header}><div><h2 className={styles.emptyTitle}>{item.size ?? '-'} {item.sizeType ?? ''}</h2><p className={styles.emptyText}>{item.equipment ?? `Apparato ${item.equipmentId}`} · cliente {item.customerId}</p></div><span className={item.readOnly ? styles.badgeDanger : styles.badge}>{item.status}</span></div><div className={styles.detailGrid}><Detail label="Protocollo" value={item.protocol} /><Detail label="Ordine" value={item.orderCode} /><Detail label="Seriale" value={item.serialNumber} /><Detail label="Creato" value={item.createdAt} /><Detail label="Chiuso" value={item.closedAt} /><Detail label="Note" value={item.note} /></div>{item.readOnly ? <p className={styles.emptyText}>Storage chiuso in sola consultazione.</p> : null}{canOperate && !item.readOnly ? <div className={styles.modalActions}><Button variant="secondary" onClick={onEdit}>Modifica</Button><Button variant="danger" onClick={onArchive}>Archivia</Button></div> : null}</>;
}

function StorageModal({ open, value, onClose, onSave, loading }: { open: boolean; value: StorageItem | null; onClose: () => void; onSave: (value: StorageInput & { id?: number }) => void; loading: boolean }) {
  const equipment = useEquipment({ status: 'active' });
  const [draft, setDraft] = useState<StorageInput>({ customerId: 0, equipmentId: 0, status: 'Attivo', sizeType: 'GB' });
  useEffect(() => setDraft(value ? { protocol: value.protocol, size: value.size, customerId: value.customerId, equipmentId: value.equipmentId, note: value.note, sizeType: value.sizeType, status: value.status, orderCode: value.orderCode, serialNumber: value.serialNumber } : { customerId: 0, equipmentId: equipment.data?.[0]?.id ?? 0, status: 'Attivo', sizeType: 'GB' }), [value, open, equipment.data]);
  return <Modal open={open} onClose={onClose} title={value ? 'Modifica storage' : 'Nuovo storage'}><div className={styles.formGrid}><NumberField label="Cliente" value={draft.customerId} onChange={(customerId) => setDraft({ ...draft, customerId })} /><div className={styles.field}><label>Apparato</label><select value={draft.equipmentId} onChange={(event) => setDraft({ ...draft, equipmentId: Number(event.target.value) })}><option value={0}>Seleziona apparato</option>{equipment.data?.map((item) => <option key={item.id} value={item.id}>{item.name}</option>)}</select></div><NumberField label="Dimensione" value={draft.size ?? 0} onChange={(size) => setDraft({ ...draft, size })} /><SelectField label="Unita" value={draft.sizeType ?? 'GB'} onChange={(sizeType) => setDraft({ ...draft, sizeType })} options={['GB', 'TB']} /><TextField label="Protocollo" value={draft.protocol ?? ''} onChange={(protocol) => setDraft({ ...draft, protocol })} /><TextField label="Codice ordine" value={draft.orderCode ?? ''} onChange={(orderCode) => setDraft({ ...draft, orderCode })} /><TextField label="Seriale" value={draft.serialNumber ?? ''} onChange={(serialNumber) => setDraft({ ...draft, serialNumber })} /></div><div className={styles.modalActions}><Button variant="secondary" onClick={onClose}>Annulla</Button><Button loading={loading} disabled={!draft.customerId || !draft.equipmentId} onClick={() => onSave({ ...draft, id: value?.id, status: value?.status ?? 'Attivo' })}>Salva</Button></div></Modal>;
}
