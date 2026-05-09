import { Button, Icon, Modal, SearchInput, SingleSelect, Skeleton, useToast } from '@mrsmith/ui';
import { useEffect, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { useEquipment, useEquipmentDetail, useEquipmentMutations, useEquipmentNics, useEquipmentTypes, useGrappaDCIMMeta, useRacks } from '../../api/queries';
import type { EquipmentInput, EquipmentItem } from '../../api/types';
import { ViewState } from '../../components/ViewState';
import styles from '../facilities/workspace.module.css';
import { ConfirmModal, Detail, NumberField, SelectField, TextField, destructiveBody, errorText } from './assetPageUtils';

export function EquipmentPage() {
  const params = useParams();
  const navigate = useNavigate();
  const toast = useToast();
  const meta = useGrappaDCIMMeta();
  const canOperate = Boolean(meta.data?.canOperate);
  const [q, setQ] = useState('');
  const [status, setStatus] = useState<'active' | 'all'>('active');
  const [editing, setEditing] = useState<EquipmentItem | null | 'new'>(null);
  const [ceasing, setCeasing] = useState<EquipmentItem | null>(null);
  const [tab, setTab] = useState<'summary' | 'nics' | 'rack' | 'history'>('summary');
  const selectedId = params.apparatoId ? Number(params.apparatoId) : null;
  const equipment = useEquipment({ q, status });
  const selected = equipment.data?.find((item) => item.id === selectedId) ?? equipment.data?.[0] ?? null;
  const detail = useEquipmentDetail(selected?.id ?? null);
  const nics = useEquipmentNics(selected?.id ?? null);
  const mutations = useEquipmentMutations();

  async function saveEquipment(input: EquipmentInput & { id?: number }) {
    try {
      const result = await mutations.saveEquipment.mutateAsync(input);
      toast.toast(result.message || 'Apparato salvato.');
      setEditing(null);
    } catch (error) {
      toast.toast(errorText(error, 'Salvataggio apparato non riuscito.'), 'error');
    }
  }

  async function ceaseEquipment() {
    if (!ceasing) return;
    try {
      const result = await mutations.ceaseEquipment.mutateAsync({ id: ceasing.id, body: destructiveBody });
      toast.toast(result.message || 'Apparato cessato.');
      setCeasing(null);
    } catch (error) {
      toast.toast(errorText(error, 'Azione bloccata da dipendenze operative.'), 'error');
    }
  }

  return (
    <section className={styles.page}>
      <div className={styles.header}>
        <div className={styles.titleBlock}>
          <span className={styles.eyebrow}>Asset</span>
          <h1 className={styles.title}>Apparati</h1>
          <p className={styles.subtitle}>Inventario apparati, porte generate e collocazione rack.</p>
        </div>
        {canOperate ? <Button onClick={() => setEditing('new')} leftIcon={<Icon name="plus" size={16} />}>Nuovo apparato</Button> : null}
      </div>
      <div className={styles.toolbar}>
        <SearchInput value={q} onChange={setQ} placeholder="Cerca apparati..." />
        <SingleSelect options={[{ value: 'active', label: 'Solo attivi' }, { value: 'all', label: 'Tutti' }]} selected={status} onChange={(value) => setStatus((value ?? 'active') as 'active' | 'all')} searchable={false} />
      </div>
      {equipment.isLoading ? (
        <div className={styles.panel}><Skeleton rows={8} /></div>
      ) : equipment.error ? (
        <ViewState title="Apparati non disponibili" message="Non e stato possibile caricare il registro apparati." tone="error" />
      ) : (equipment.data?.length ?? 0) === 0 ? (
        <div className={styles.emptyPanel}><h3 className={styles.emptyTitle}>Nessun apparato trovato</h3><p className={styles.emptyText}>Modifica i filtri o aggiungi un apparato operativo.</p></div>
      ) : (
        <div className={styles.split}>
          <div className={styles.tableWrap}>
            <table className={styles.table}>
              <thead><tr><th>Apparato</th><th>Tipo</th><th>Rack</th><th>Porte</th><th>Stato</th><th>Azioni</th></tr></thead>
              <tbody>
                {equipment.data?.map((item) => (
                  <tr key={item.id} className={`${styles.clickable} ${selected?.id === item.id ? styles.selectedRow : ''}`} onClick={() => navigate(`/apparati/${item.id}`)}>
                    <td><strong>{item.name}</strong><br /><span className={styles.muted}>{item.managementIp ?? item.serialNumber ?? item.serial ?? '-'}</span></td>
                    <td>{item.type}</td>
                    <td>{item.rackName ?? '-'}<br /><span className={styles.muted}>{item.datacenterName ?? ''}</span></td>
                    <td>{item.nicCount}</td>
                    <td><span className={item.status === 'Cessato' ? styles.badgeDanger : styles.badge}>{item.status ?? 'Attivo'}</span></td>
                    <td><div className={styles.actions}>{canOperate ? <Button size="sm" variant="secondary" onClick={(event) => { event.stopPropagation(); setEditing(item); }}>Modifica</Button> : null}{canOperate ? <Button size="sm" variant="danger" onClick={(event) => { event.stopPropagation(); setCeasing(item); }}>Cessa</Button> : null}</div></td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          <div className={styles.panel}>
            {detail.isLoading ? <Skeleton rows={8} /> : detail.error || !detail.data ? (
              <div className={styles.emptyPanel}><h3 className={styles.emptyTitle}>Dettaglio non disponibile</h3><p className={styles.emptyText}>Seleziona un apparato dal registro.</p></div>
            ) : (
              <>
                <div className={styles.header}><div><h2 className={styles.emptyTitle}>{detail.data.name}</h2><p className={styles.emptyText}>{detail.data.type} · {detail.data.rackName ?? 'Rack non indicato'}</p></div><span className={styles.badgeMuted}>{detail.data.status ?? 'Attivo'}</span></div>
                <div className={styles.tabs}>{[['summary', 'Riepilogo'], ['nics', 'NIC'], ['rack', 'Rack'], ['history', 'Storico']].map(([key, label]) => <button key={key} className={`${styles.tab} ${tab === key ? styles.tabActive : ''}`} onClick={() => setTab(key as typeof tab)}>{label}</button>)}</div>
                {tab === 'summary' ? <EquipmentSummary item={detail.data} /> : null}
                {tab === 'nics' ? <NICList loading={nics.isLoading} items={nics.data} /> : null}
                {tab === 'rack' ? <RackInfo item={detail.data} /> : null}
                {tab === 'history' ? <p className={styles.emptyText}>Nessuno storico operativo disponibile per questo apparato.</p> : null}
              </>
            )}
          </div>
        </div>
      )}
      <EquipmentModal open={editing !== null} value={editing === 'new' ? null : editing} onClose={() => setEditing(null)} onSave={saveEquipment} loading={mutations.saveEquipment.isPending} />
      <ConfirmModal open={ceasing !== null} title="Cessa apparato" message={`Confermi la cessazione di ${ceasing?.name ?? 'questo apparato'}?`} onClose={() => setCeasing(null)} onConfirm={ceaseEquipment} loading={mutations.ceaseEquipment.isPending} />
    </section>
  );
}

function EquipmentSummary({ item }: { item: EquipmentItem }) {
  return <div className={styles.detailGrid}><Detail label="Cliente" value={item.customerId} /><Detail label="Ordine" value={item.orderCode} /><Detail label="Seriale" value={item.serialNumber ?? item.serial} /><Detail label="IP gestione" value={item.managementIp} /><Detail label="Sistema" value={item.os} /><Detail label="Modello" value={item.model} /><Detail label="Monitoraggio" value={item.monitoringActive} /><Detail label="Note" value={item.note} /></div>;
}

function RackInfo({ item }: { item: EquipmentItem }) {
  return <div className={styles.detailGrid}><Detail label="Rack" value={item.rackName} /><Detail label="Sala" value={item.datacenterName} /><Detail label="Unita" value={item.unit} /><Detail label="Posizione unita" value={item.unitPosition} /></div>;
}

function NICList({ loading, items }: { loading: boolean; items?: import('../../api/types').NICItem[] }) {
  if (loading) return <Skeleton rows={5} />;
  if (!items?.length) return <p className={styles.emptyText}>Nessuna NIC presente.</p>;
  return <div className={styles.tableWrap}><table className={styles.table}><thead><tr><th>Nome</th><th>Tipo</th><th>Layer</th><th>Stato</th></tr></thead><tbody>{items.map((item) => <tr key={item.id}><td>{item.name}<br /><span className={styles.muted}>{item.identifier}</span></td><td>{item.type ?? '-'}</td><td>{item.layer ?? '-'}</td><td>{item.status ?? '-'}</td></tr>)}</tbody></table></div>;
}

function EquipmentModal({ open, value, onClose, onSave, loading }: { open: boolean; value: EquipmentItem | null; onClose: () => void; onSave: (value: EquipmentInput & { id?: number }) => void; loading: boolean }) {
  const racks = useRacks({ status: 'active' });
  const types = useEquipmentTypes();
  const [draft, setDraft] = useState<EquipmentInput>({ name: '', type: '', status: 'Attivo', portCount: 0 });

  useEffect(() => {
    setDraft(value ? {
      name: value.name,
      rackId: value.rackId,
      unitPosition: value.unitPosition,
      unit: value.unit,
      managementIp: value.managementIp,
      note: value.note,
      type: value.type,
      serial: value.serial,
      os: value.os,
      model: value.model,
      customerId: value.customerId,
      status: value.status ?? 'Attivo',
      bandwidth: value.bandwidth,
      portCount: value.portCount ?? 0,
      portName: value.portName,
      portType: value.portType,
      portLayer: value.portLayer,
      monitoringActive: value.monitoringActive,
      firewallType: value.firewallType,
      serialNumber: value.serialNumber,
      orderCode: value.orderCode,
    } : { name: '', type: types.data?.[0]?.label ?? '', status: 'Attivo', portCount: 0 });
  }, [value, open, types.data]);

  return (
    <Modal open={open} onClose={onClose} title={value ? 'Modifica apparato' : 'Nuovo apparato'} size="wide">
      <div className={styles.formGrid}>
        <TextField label="Nome" value={draft.name} onChange={(name) => setDraft({ ...draft, name })} />
        <div className={styles.field}><label>Tipo</label><input list="equipment-types" value={draft.type} onChange={(event) => setDraft({ ...draft, type: event.target.value })} /><datalist id="equipment-types">{types.data?.map((item) => <option key={String(item.id)} value={item.label} />)}</datalist></div>
        <div className={styles.field}><label>Rack</label><select value={draft.rackId ?? 0} onChange={(event) => setDraft({ ...draft, rackId: Number(event.target.value) || undefined })}><option value={0}>Rack non indicato</option>{racks.data?.map((item) => <option key={item.id} value={item.id}>{item.name}</option>)}</select></div>
        <TextField label="IP gestione" value={draft.managementIp ?? ''} onChange={(managementIp) => setDraft({ ...draft, managementIp })} />
        <TextField label="Modello" value={draft.model ?? ''} onChange={(model) => setDraft({ ...draft, model })} />
        <TextField label="Seriale" value={draft.serialNumber ?? draft.serial ?? ''} onChange={(serialNumber) => setDraft({ ...draft, serialNumber })} />
        <TextField label="Codice ordine" value={draft.orderCode ?? ''} onChange={(orderCode) => setDraft({ ...draft, orderCode })} />
        <SelectField label="Stato" value={draft.status ?? 'Attivo'} onChange={(status) => setDraft({ ...draft, status })} options={['Attivo', 'Cessato']} />
        <NumberField label="Porte da generare" value={draft.portCount ?? 0} onChange={(portCount) => setDraft({ ...draft, portCount })} />
        <TextField label="Nome porte" value={draft.portName ?? ''} onChange={(portName) => setDraft({ ...draft, portName })} />
      </div>
      <div className={styles.modalActions}><Button variant="secondary" onClick={onClose}>Annulla</Button><Button loading={loading} disabled={!draft.name || !draft.type} onClick={() => onSave({ ...draft, id: value?.id })}>Salva</Button></div>
      {value ? <p className={styles.emptyText}>Le porte non vengono rigenerate durante la modifica.</p> : null}
    </Modal>
  );
}
