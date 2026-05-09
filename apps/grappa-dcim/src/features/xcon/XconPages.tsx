import { Button, Modal } from '@mrsmith/ui';
import { useEffect, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { useGrappaDCIMMeta, useRacks, useXcon, useXconDetail, useXconMutations, useXconProducts } from '../../api/queries';
import type { Xcon, XconHop, XconInput } from '../../api/types';
import { Detail, errorText, NumberField, SelectField, TextField, valueOrDash } from '../equipment/assetPageUtils';
import styles from '../facilities/workspace.module.css';

export function XconPage() {
  const { xconId } = useParams();
  const selectedId = xconId ? Number(xconId) : null;
  const navigate = useNavigate();
  const meta = useGrappaDCIMMeta();
  const canOperate = Boolean(meta.data?.canOperate);
  const [tab, setTab] = useState<'active' | 'ceased'>('active');
  const [q, setQ] = useState('');
  const [editing, setEditing] = useState<Xcon | null | 'new'>(null);
  const [editingHops, setEditingHops] = useState(false);
  const list = useXcon({ tab, q });
  const detail = useXconDetail(selectedId);
  const products = useXconProducts();
  const mutations = useXconMutations();
  const selected = selectedId ? detail.data ?? list.data?.find((item) => item.id === selectedId) : null;

  return (
    <main className={styles.page}>
      <header className={styles.header}>
        <div className={styles.titleBlock}>
          <span className={styles.eyebrow}>Connettivita</span>
          <h1 className={styles.title}>Cross connect</h1>
          <p className={styles.subtitle}>Registro circuiti, endpoint A/Z, LOA/MMR e percorso ordinato.</p>
        </div>
        {canOperate ? <Button onClick={() => setEditing('new')}>Nuovo cross connect</Button> : null}
      </header>

      <section className={styles.toolbar}>
        <div className={styles.tabs}>
          <button className={`${styles.tab} ${tab === 'active' ? styles.tabActive : ''}`} onClick={() => setTab('active')}>Attivi</button>
          <button className={`${styles.tab} ${tab === 'ceased' ? styles.tabActive : ''}`} onClick={() => setTab('ceased')}>Cessati</button>
        </div>
        <input value={q} onChange={(event) => setQ(event.target.value)} placeholder="Cerca cross connect" />
      </section>

      <section className={styles.split}>
        <div className={styles.tableWrap}>
          <table className={styles.table}>
            <thead><tr><th>Ticket</th><th>Cliente</th><th>Tipo</th><th>Stato</th><th>Codice Ordine</th><th>Serial Number</th></tr></thead>
            <tbody>{list.data?.map((item) => (
              <tr key={item.id} className={`${styles.clickable} ${selectedId === item.id ? styles.selectedRow : ''}`} onClick={() => navigate(`/cross-connect/${item.id}`)}>
                <td>{item.extendedTicket ?? item.ticket}</td><td>{item.customerId}</td><td>{item.type}</td><td><span className={item.status === 'cessata' ? styles.badgeDanger : styles.badgeMuted}>{item.status}</span></td><td>{valueOrDash(item.orderCode)}</td><td>{valueOrDash(item.serialNumber)}</td>
              </tr>
            ))}</tbody>
          </table>
        </div>
        <aside className={styles.panel}>
          {selected ? (
            <>
              <div className={styles.detailGrid}>
                <Detail label="Ticket Esteso" value={selected.extendedTicket ?? selected.ticket} />
                <Detail label="Codice Ordine" value={selected.orderCode} />
                <Detail label="Serial Number" value={selected.serialNumber} />
                <Detail label="Tipo" value={selected.type} />
                <Detail label="Stato" value={selected.status} />
                <Detail label="Sorgente" value={selected.source} />
                <Detail label="LOA" value={selected.loaName ?? selected.loaId} />
                <Detail label="MMR" value={selected.mmrPort} />
              </div>
              <div className={styles.tabs}>
                <button className={`${styles.tab} ${styles.tabActive}`}>Riepilogo</button>
                <button className={styles.tab}>Endpoint</button>
                <button className={styles.tab}>Percorso</button>
                <button className={styles.tab}>LOA/MMR</button>
              </div>
              <div className={styles.detailGrid}>
                <Detail label="A apparato" value={selected.aEndEquipment} />
                <Detail label="A unita" value={selected.aEndUnit} />
                <Detail label="A fibre" value={selected.aEndFibers} />
                <Detail label="Z apparato" value={selected.zEndEquipment} />
                <Detail label="Z unita" value={selected.zEndUnit} />
                <Detail label="Z fibre" value={selected.zEndFibers} />
              </div>
              <div className={styles.socketList}>
                {(selected.hops ?? []).length === 0 ? <p className={styles.emptyText}>Nessun hop intermedio.</p> : selected.hops?.map((hop) => <div key={`${hop.order}-${hop.id ?? hop.rackId}`} className={styles.socketRow}><span>{hop.order}. {hop.room} - {hop.rack} - {hop.unit}</span><span>{hop.fibers}</span></div>)}
              </div>
              {canOperate ? <div className={styles.actions}><Button variant="secondary" onClick={() => setEditing(selected)}>Modifica</Button><Button variant="secondary" onClick={() => setEditingHops(true)}>Percorso</Button></div> : null}
            </>
          ) : <div className={styles.emptyPanel}><div className={styles.emptyTitle}>Seleziona un cross connect</div><p className={styles.emptyText}>Apri un circuito per vedere endpoint e percorso.</p></div>}
        </aside>
      </section>

      <XconModal open={Boolean(editing)} value={editing === 'new' ? null : editing} productOptions={products.data?.map((item) => String(item.label)) ?? []} onClose={() => setEditing(null)} onSave={(value) => mutations.saveXcon.mutate(value, { onSuccess: () => setEditing(null) })} loading={mutations.saveXcon.isPending} />
      <HopsModal open={editingHops} value={selected?.hops ?? []} onClose={() => setEditingHops(false)} onSave={(items) => selected && mutations.replaceHops.mutate({ id: selected.id, items }, { onSuccess: () => setEditingHops(false) })} loading={mutations.replaceHops.isPending} />
      {mutations.saveXcon.error ? <p className={styles.emptyText}>{errorText(mutations.saveXcon.error, 'Salvataggio non riuscito.')}</p> : null}
      {mutations.replaceHops.error ? <p className={styles.emptyText}>{errorText(mutations.replaceHops.error, 'Aggiornamento percorso non riuscito.')}</p> : null}
    </main>
  );
}

function XconModal({ open, value, productOptions, onClose, onSave, loading }: { open: boolean; value: Xcon | null; productOptions: string[]; onClose: () => void; onSave: (value: XconInput & { id?: number }) => void; loading: boolean }) {
  const initial = (): XconInput => ({
    ticket: value?.ticket ?? '',
    pa: value?.pa ?? '',
    customerId: value?.customerId ?? 0,
    status: value?.status ?? 'attiva',
    orderCode: value?.orderCode ?? '',
    serialNumber: value?.serialNumber ?? '',
    type: value?.type ?? productOptions[0] ?? 'CDL-X',
    activatedAt: value?.activatedAt?.slice(0, 10) ?? '',
    ceasedAt: value?.ceasedAt?.slice(0, 10) ?? '',
    aEndUnit: value?.aEndUnit ?? '',
    aEndSlot: value?.aEndSlot ?? '',
    aEndFibers: value?.aEndFibers ?? '',
    aEndEquipment: value?.aEndEquipment ?? '',
    zEndUnit: value?.zEndUnit ?? '',
    zEndSlot: value?.zEndSlot ?? '',
    zEndFibers: value?.zEndFibers ?? '',
    zEndEquipment: value?.zEndEquipment ?? '',
    extendedTicket: value?.extendedTicket ?? '',
    source: value?.source ?? 'AssetManager',
    loaName: value?.loaName ?? '',
    mmrPort: value?.mmrPort ?? '',
    note: value?.note ?? '',
  });
  const [draft, setDraft] = useState<XconInput>(initial);
  useEffect(() => setDraft(initial()), [value, productOptions.join('|')]);
  const typeOptions = productOptions.includes(draft.type) ? productOptions : [draft.type, ...productOptions].filter(Boolean);
  return (
    <Modal open={open} onClose={onClose} title={value ? 'Modifica cross connect' : 'Nuovo cross connect'} size="wide">
      <div className={styles.formGrid}>
        <TextField label="Ticket" value={draft.ticket} onChange={(ticket) => setDraft({ ...draft, ticket })} />
        <TextField label="Ticket Esteso" value={draft.extendedTicket ?? ''} onChange={(extendedTicket) => setDraft({ ...draft, extendedTicket })} />
        <NumberField label="Cliente" value={draft.customerId} onChange={(customerId) => setDraft({ ...draft, customerId })} />
        <SelectField label="Stato" value={draft.status} onChange={(status) => setDraft({ ...draft, status })} options={['attiva', 'annullato', 'cessata']} />
        <SelectField label="Tipo" value={draft.type} onChange={(type) => setDraft({ ...draft, type })} options={typeOptions.length ? typeOptions : ['CDL-X']} />
        <TextField label="Codice Ordine" value={draft.orderCode ?? ''} onChange={(orderCode) => setDraft({ ...draft, orderCode })} />
        <TextField label="Serial Number" value={draft.serialNumber ?? ''} onChange={(serialNumber) => setDraft({ ...draft, serialNumber })} />
        <TextField label="Sorgente" value={draft.source ?? ''} onChange={(source) => setDraft({ ...draft, source })} />
        <TextField label="A apparato" value={draft.aEndEquipment} onChange={(aEndEquipment) => setDraft({ ...draft, aEndEquipment })} />
        <TextField label="A unita" value={draft.aEndUnit} onChange={(aEndUnit) => setDraft({ ...draft, aEndUnit })} />
        <TextField label="A slot" value={draft.aEndSlot ?? ''} onChange={(aEndSlot) => setDraft({ ...draft, aEndSlot })} />
        <TextField label="A fibre" value={draft.aEndFibers} onChange={(aEndFibers) => setDraft({ ...draft, aEndFibers })} />
        <TextField label="Z apparato" value={draft.zEndEquipment} onChange={(zEndEquipment) => setDraft({ ...draft, zEndEquipment })} />
        <TextField label="Z unita" value={draft.zEndUnit} onChange={(zEndUnit) => setDraft({ ...draft, zEndUnit })} />
        <TextField label="Z slot" value={draft.zEndSlot ?? ''} onChange={(zEndSlot) => setDraft({ ...draft, zEndSlot })} />
        <TextField label="Z fibre" value={draft.zEndFibers} onChange={(zEndFibers) => setDraft({ ...draft, zEndFibers })} />
        <TextField label="LOA" value={draft.loaName ?? ''} onChange={(loaName) => setDraft({ ...draft, loaName })} />
        <TextField label="MMR" value={draft.mmrPort ?? ''} onChange={(mmrPort) => setDraft({ ...draft, mmrPort })} />
      </div>
      <div className={styles.modalActions}><Button variant="secondary" onClick={onClose}>Annulla</Button><Button loading={loading} disabled={!draft.ticket || !draft.customerId || !draft.type || !draft.aEndEquipment || !draft.zEndEquipment} onClick={() => onSave({ ...draft, id: value?.id })}>Salva</Button></div>
    </Modal>
  );
}

function HopsModal({ open, value, onClose, onSave, loading }: { open: boolean; value: XconHop[]; onClose: () => void; onSave: (items: XconHop[]) => void; loading: boolean }) {
  const racks = useRacks({ status: 'all' });
  const [items, setItems] = useState<XconHop[]>(value);
  useEffect(() => setItems(value), [value]);
  const update = (index: number, patch: Partial<XconHop>) => setItems((current) => current.map((item, i) => i === index ? { ...item, ...patch } : item));
  return (
    <Modal open={open} onClose={onClose} title="Percorso cross connect" size="wide">
      <div className={styles.socketList}>
        {items.map((item, index) => (
          <div key={index} className={styles.socketRow}>
            <div className={styles.formGrid}>
              <TextField label="Sala" value={item.room} onChange={(room) => update(index, { room })} />
              <TextField label="Rack" value={item.rack} onChange={(rack) => update(index, { rack })} />
              <TextField label="Unita" value={item.unit} onChange={(unit) => update(index, { unit })} />
              <TextField label="Fibre" value={item.fibers} onChange={(fibers) => update(index, { fibers })} />
              <div className={styles.field}><label>Rack collegato</label><select value={item.rackId} onChange={(event) => update(index, { rackId: Number(event.target.value) })}><option value={0}>Seleziona rack</option>{racks.data?.map((rack) => <option key={rack.id} value={rack.id}>{rack.name}</option>)}</select></div>
            </div>
            <Button variant="secondary" onClick={() => setItems((current) => current.filter((_, i) => i !== index))}>Rimuovi</Button>
          </div>
        ))}
      </div>
      <div className={styles.modalActions}><Button variant="secondary" onClick={() => setItems([...items, { room: '', rack: '', unit: '', fibers: '', rackId: 0, order: items.length + 1 }])}>Aggiungi hop</Button><Button variant="secondary" onClick={onClose}>Annulla</Button><Button loading={loading} onClick={() => onSave(items.map((item, index) => ({ ...item, order: index + 1 })))}>Salva</Button></div>
    </Modal>
  );
}
