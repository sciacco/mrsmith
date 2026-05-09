import { Button, Modal } from '@mrsmith/ui';
import { useEffect, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import {
  useCableDetail,
  useCableFibers,
  useCables,
  useCablingMutations,
  useDatacenters,
  useGrappaDCIMMeta,
  usePlenumMatrix,
  usePlenums,
  usePorts,
} from '../../api/queries';
import type { Cable, CableInput, Fiber, FiberAssignmentInput, Plenum, PlenumInput } from '../../api/types';
import { ConfirmModal, destructiveBody, Detail, errorText, NumberField, SelectField, TextField, valueOrDash } from '../equipment/assetPageUtils';
import styles from '../facilities/workspace.module.css';
import cablingStyles from './cabling.module.css';

export function PlenumPage() {
  const { plenumId } = useParams();
  const selectedId = plenumId ? Number(plenumId) : null;
  const navigate = useNavigate();
  const meta = useGrappaDCIMMeta();
  const canOperate = Boolean(meta.data?.canOperate);
  const [q, setQ] = useState('');
  const [datacenterId, setDatacenterId] = useState<number | null>(null);
  const [editing, setEditing] = useState<Plenum | null | 'new'>(null);
  const [deleting, setDeleting] = useState<Plenum | null>(null);
  const plenums = usePlenums({ q, datacenterId });
  const datacenters = useDatacenters({ kind: 'all', status: 'all' });
  const matrix = usePlenumMatrix(selectedId);
  const mutations = useCablingMutations();
  const selected = selectedId ? plenums.data?.find((item) => item.id === selectedId) ?? matrix.data?.plenum : null;

  return (
    <main className={styles.page}>
      <header className={styles.header}>
        <div className={styles.titleBlock}>
          <span className={styles.eyebrow}>Connettivita</span>
          <h1 className={styles.title}>Plenum</h1>
          <p className={styles.subtitle}>Gestione plenum e matrice fibre a 288 celle con inizializzazione esplicita.</p>
        </div>
        {canOperate ? <Button onClick={() => setEditing('new')}>Nuovo plenum</Button> : null}
      </header>

      <section className={styles.toolbar}>
        <input value={q} onChange={(event) => setQ(event.target.value)} placeholder="Cerca plenum" />
        <select value={datacenterId ?? 0} onChange={(event) => setDatacenterId(Number(event.target.value) || null)}>
          <option value={0}>Tutte le sale</option>
          {datacenters.data?.map((item) => <option key={item.id} value={item.id}>{item.name}</option>)}
        </select>
      </section>

      <section className={styles.split}>
        <div className={styles.tableWrap}>
          <table className={styles.table}>
            <thead><tr><th>Nome</th><th>Sala</th><th>Stato</th><th>Terminazioni</th><th>Porte</th></tr></thead>
            <tbody>
              {plenums.data?.map((item) => (
                <tr key={item.id} className={`${styles.clickable} ${selectedId === item.id ? styles.selectedRow : ''}`} onClick={() => navigate(`/plenum/${item.id}`)}>
                  <td>{valueOrDash(item.name)}</td>
                  <td>{valueOrDash(item.datacenterName ?? item.datacenterId)}</td>
                  <td><span className={styles.badgeMuted}>{item.status}</span></td>
                  <td>{item.slotCount}/24</td>
                  <td>{item.linkedPortCount}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>

        <aside className={styles.panel}>
          {selected ? (
            <div className={styles.detailGrid}>
              <Detail label="Nome" value={selected.name} />
              <Detail label="Sala" value={selected.datacenterName ?? selected.datacenterId} />
              <Detail label="Isola" value={selected.isle} />
              <Detail label="Tipo" value={selected.type} />
              <Detail label="Stato" value={selected.status} />
              <Detail label="Terminazioni" value={`${selected.slotCount}/24`} />
              {canOperate ? <div className={styles.actions}><Button variant="secondary" onClick={() => setEditing(selected)}>Modifica</Button><Button variant="danger" onClick={() => setDeleting(selected)}>Elimina</Button></div> : null}
            </div>
          ) : <Empty title="Seleziona un plenum" text="Apri un plenum per visualizzare la matrice fibre." />}
        </aside>
      </section>

      {selectedId ? (
        <section className={styles.panel}>
          <div className={cablingStyles.summaryRow}>
            <span className={styles.badge}>{matrix.data?.freeCells ?? 0} libere</span>
            <span className={styles.badgeMuted}>{matrix.data?.assignedCells ?? 0} occupate</span>
            {matrix.data?.incomplete ? <span className={styles.badgeDanger}>Configurazione incompleta</span> : null}
            {matrix.data?.mapOnlyRecords ? <span className={styles.badgeMuted}>{matrix.data.mapOnlyRecords} riferimenti mappa</span> : null}
            {canOperate && matrix.data?.incomplete ? <Button onClick={() => mutations.initializeMatrix.mutate(selectedId)} loading={mutations.initializeMatrix.isPending}>Inizializza matrice</Button> : null}
          </div>
          {matrix.data ? <PlenumMatrixView matrix={matrix.data} /> : <p className={styles.emptyText}>Caricamento matrice.</p>}
        </section>
      ) : null}

      <PlenumModal
        open={Boolean(editing)}
        value={editing === 'new' ? null : editing}
        datacenters={datacenters.data ?? []}
        onClose={() => setEditing(null)}
        onSave={(value) => mutations.savePlenum.mutate(value, { onSuccess: () => setEditing(null) })}
        loading={mutations.savePlenum.isPending}
      />
      <ConfirmModal
        open={Boolean(deleting)}
        title="Elimina plenum"
        message="Il plenum puo essere eliminato solo se non ha porte collegate."
        onClose={() => setDeleting(null)}
        onConfirm={() => deleting && mutations.deletePlenum.mutate({ id: deleting.id, body: destructiveBody }, { onSuccess: () => { setDeleting(null); navigate('/plenum'); } })}
        loading={mutations.deletePlenum.isPending}
      />
      {mutations.savePlenum.error ? <p className={styles.emptyText}>{errorText(mutations.savePlenum.error, 'Salvataggio non riuscito.')}</p> : null}
      {mutations.deletePlenum.error ? <p className={styles.emptyText}>{errorText(mutations.deletePlenum.error, 'Eliminazione non riuscita.')}</p> : null}
    </main>
  );
}

function PlenumMatrixView({ matrix }: { matrix: import('../../api/types').PlenumMatrix }) {
  return (
    <div className={cablingStyles.matrixWrap}>
      <div className={cablingStyles.matrix}>
        {matrix.slots.map((slot) => (
          <div key={`${slot.cable}-${slot.number}`} className={cablingStyles.slot}>
            <div className={cablingStyles.slotHeader}>
              <span>Cavo {slot.cable} - Terminazione {slot.number}</span>
              <span>{slot.missing ? 'Da inizializzare' : slot.status ?? 'Empty'}</span>
            </div>
            <div className={cablingStyles.cells}>
              {slot.cells.map((cell) => {
                const linked = cell.status !== 'Empty' && cell.status !== 'Missing';
                return <button key={cell.fiber} className={`${cablingStyles.cell} ${linked ? cablingStyles.cellLinked : ''} ${cell.status === 'Missing' ? cablingStyles.cellMissing : ''}`} title={cell.portLabel ?? cell.status}>{cell.fiber}</button>;
              })}
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

export function CableFiberPage() {
  const { cableId } = useParams();
  const selectedId = cableId ? Number(cableId) : null;
  const navigate = useNavigate();
  const meta = useGrappaDCIMMeta();
  const canOperate = Boolean(meta.data?.canOperate);
  const [q, setQ] = useState('');
  const [editing, setEditing] = useState<Cable | null | 'new'>(null);
  const [deleting, setDeleting] = useState<Cable | null>(null);
  const [assigning, setAssigning] = useState<Fiber | null>(null);
  const cables = useCables({ q });
  const cable = useCableDetail(selectedId);
  const fibers = useCableFibers(selectedId);
  const mutations = useCablingMutations();
  const selected = selectedId ? cables.data?.find((item) => item.id === selectedId) ?? cable.data : null;

  return (
    <main className={styles.page}>
      <header className={styles.header}>
        <div className={styles.titleBlock}>
          <span className={styles.eyebrow}>Connettivita</span>
          <h1 className={styles.title}>Cavi e fibre</h1>
          <p className={styles.subtitle}>Inventario cavi, fibre generate e assegnazioni sulle porte disponibili.</p>
        </div>
        {canOperate ? <Button onClick={() => setEditing('new')}>Nuovo cavo</Button> : null}
      </header>
      <section className={styles.toolbar}><input value={q} onChange={(event) => setQ(event.target.value)} placeholder="Cerca cavo" /></section>
      <section className={styles.split}>
        <div className={styles.tableWrap}>
          <table className={styles.table}>
            <thead><tr><th>Cavo</th><th>Descrizione</th><th>Fibre</th><th>Assegnate</th><th>Stato</th></tr></thead>
            <tbody>{cables.data?.map((item) => (
              <tr key={item.id} className={`${styles.clickable} ${selectedId === item.id ? styles.selectedRow : ''}`} onClick={() => navigate(`/cavi-fibre/${item.id}`)}>
                <td>{item.name}</td><td>{item.description}</td><td>{item.fibersNum}</td><td>{item.assignedFibers}</td><td><span className={styles.badgeMuted}>{item.status}</span></td>
              </tr>
            ))}</tbody>
          </table>
        </div>
        <aside className={styles.panel}>
          {selected ? (
            <div className={styles.detailGrid}>
              <Detail label="Cavo" value={selected.name} /><Detail label="Fibre" value={selected.fibersNum} />
              <Detail label="Assegnate" value={selected.assignedFibers} /><Detail label="Stato" value={selected.status} />
              {canOperate ? <div className={styles.actions}><Button variant="secondary" onClick={() => setEditing(selected)}>Modifica</Button><Button variant="danger" onClick={() => setDeleting(selected)}>Elimina</Button></div> : null}
            </div>
          ) : <Empty title="Seleziona un cavo" text="Apri un cavo per gestire le fibre generate." />}
        </aside>
      </section>
      {selectedId ? (
        <section className={styles.panel}>
          <div className={styles.tableWrap}>
            <table className={styles.table}>
              <thead><tr><th>Fibra</th><th>Stato</th><th>Porta A</th><th>Porta Z</th><th></th></tr></thead>
              <tbody>{fibers.data?.map((item) => (
                <tr key={item.id}><td>{item.number}</td><td>{item.status}</td><td>{valueOrDash(item.leftLabel ?? item.leftPortId)}</td><td>{valueOrDash(item.rightLabel ?? item.rightPortId)}</td><td>{canOperate ? <Button variant="secondary" onClick={() => setAssigning(item)}>Assegna fibra</Button> : null}</td></tr>
              ))}</tbody>
            </table>
          </div>
        </section>
      ) : null}
      <CableModal open={Boolean(editing)} value={editing === 'new' ? null : editing} onClose={() => setEditing(null)} onSave={(value) => mutations.saveCable.mutate(value, { onSuccess: () => setEditing(null) })} loading={mutations.saveCable.isPending} />
      <ConfirmModal
        open={Boolean(deleting)}
        title="Elimina cavo"
        message="Il cavo puo essere eliminato solo se tutte le fibre sono libere e non assegnate."
        onClose={() => setDeleting(null)}
        onConfirm={() => deleting && mutations.deleteCable.mutate({ id: deleting.id, body: destructiveBody }, { onSuccess: () => { setDeleting(null); navigate('/cavi-fibre'); } })}
        loading={mutations.deleteCable.isPending}
      />
      <FiberAssignModal open={Boolean(assigning)} fiber={assigning} onClose={() => setAssigning(null)} onSave={(body) => assigning && mutations.assignFiber.mutate({ id: assigning.id, body }, { onSuccess: () => setAssigning(null) })} loading={mutations.assignFiber.isPending} />
      {mutations.assignFiber.error ? <p className={styles.emptyText}>{errorText(mutations.assignFiber.error, 'Assegnazione non riuscita.')}</p> : null}
      {mutations.deleteCable.error ? <p className={styles.emptyText}>{errorText(mutations.deleteCable.error, 'Eliminazione non riuscita.')}</p> : null}
    </main>
  );
}

function Empty({ title, text }: { title: string; text: string }) {
  return <div className={styles.emptyPanel}><div className={styles.emptyTitle}>{title}</div><p className={styles.emptyText}>{text}</p></div>;
}

function PlenumModal({ open, value, datacenters, onClose, onSave, loading }: { open: boolean; value: Plenum | null; datacenters: Array<{ id: number; name: string }>; onClose: () => void; onSave: (value: PlenumInput & { id?: number }) => void; loading: boolean }) {
  const [draft, setDraft] = useState<PlenumInput>(() => ({ name: value?.name ?? '', isle: value?.isle ?? '', type: value?.type ?? '', datacenterId: value?.datacenterId ?? 0, status: value?.status ?? 'Attivo' }));
  useEffect(() => setDraft({ name: value?.name ?? '', isle: value?.isle ?? '', type: value?.type ?? '', datacenterId: value?.datacenterId ?? 0, status: value?.status ?? 'Attivo' }), [value]);
  return <Modal open={open} onClose={onClose} title={value ? 'Modifica plenum' : 'Nuovo plenum'}><div className={styles.formGrid}><TextField label="Nome" value={draft.name ?? ''} onChange={(name) => setDraft({ ...draft, name })} /><div className={styles.field}><label>Sala</label><select value={draft.datacenterId} onChange={(event) => setDraft({ ...draft, datacenterId: Number(event.target.value) })}><option value={0}>Seleziona sala</option>{datacenters.map((item) => <option key={item.id} value={item.id}>{item.name}</option>)}</select></div><TextField label="Isola" value={draft.isle ?? ''} onChange={(isle) => setDraft({ ...draft, isle })} /><TextField label="Tipo" value={draft.type ?? ''} onChange={(type) => setDraft({ ...draft, type })} /><SelectField label="Stato" value={draft.status} onChange={(status) => setDraft({ ...draft, status })} options={['Attivo', 'Cessato']} /></div><div className={styles.modalActions}><Button variant="secondary" onClick={onClose}>Annulla</Button><Button loading={loading} disabled={!draft.datacenterId} onClick={() => onSave({ ...draft, id: value?.id })}>Salva</Button></div></Modal>;
}

function CableModal({ open, value, onClose, onSave, loading }: { open: boolean; value: Cable | null; onClose: () => void; onSave: (value: CableInput & { id?: number }) => void; loading: boolean }) {
  const [draft, setDraft] = useState<CableInput>(() => ({ name: value?.name ?? '', description: value?.description ?? '', fibersNum: value?.fibersNum ?? 12, status: value?.status ?? 'Attivo' }));
  useEffect(() => setDraft({ name: value?.name ?? '', description: value?.description ?? '', fibersNum: value?.fibersNum ?? 12, status: value?.status ?? 'Attivo' }), [value]);
  return <Modal open={open} onClose={onClose} title={value ? 'Modifica cavo' : 'Nuovo cavo'}><div className={styles.formGrid}><TextField label="Nome" value={draft.name} onChange={(name) => setDraft({ ...draft, name })} /><NumberField label="Fibre" value={draft.fibersNum} onChange={(fibersNum) => setDraft({ ...draft, fibersNum })} /><TextField label="Descrizione" value={draft.description} onChange={(description) => setDraft({ ...draft, description })} /><SelectField label="Stato" value={draft.status} onChange={(status) => setDraft({ ...draft, status })} options={['Attivo', 'Cessato']} /></div><p className={cablingStyles.hint}>Le fibre vengono create solo alla creazione del cavo.</p><div className={styles.modalActions}><Button variant="secondary" onClick={onClose}>Annulla</Button><Button loading={loading} disabled={!draft.name || draft.fibersNum <= 0} onClick={() => onSave({ ...draft, id: value?.id })}>Salva</Button></div></Modal>;
}

function FiberAssignModal({ open, fiber, onClose, onSave, loading }: { open: boolean; fiber: Fiber | null; onClose: () => void; onSave: (value: FiberAssignmentInput) => void; loading: boolean }) {
  const [left, setLeft] = useState(fiber?.leftPortId ?? 0);
  const [right, setRight] = useState(fiber?.rightPortId ?? 0);
  useEffect(() => { setLeft(fiber?.leftPortId ?? 0); setRight(fiber?.rightPortId ?? 0); }, [fiber]);
  const ports = usePorts({ status: 'Empty', availableForFiberId: fiber?.id ?? null });
  return <Modal open={open} onClose={onClose} title="Assegna fibra" size="lg"><div className={styles.formGrid}><div className={styles.field}><label>Porta A</label><select value={left} onChange={(event) => setLeft(Number(event.target.value))}><option value={0}>Non assegnata</option>{ports.data?.map((item) => <option key={item.id} value={item.id}>{item.label}</option>)}</select></div><div className={styles.field}><label>Porta Z</label><select value={right} onChange={(event) => setRight(Number(event.target.value))}><option value={0}>Non assegnata</option>{ports.data?.map((item) => <option key={item.id} value={item.id}>{item.label}</option>)}</select></div></div><div className={styles.modalActions}><Button variant="secondary" onClick={onClose}>Annulla</Button><Button loading={loading} disabled={left !== 0 && left === right} onClick={() => onSave({ leftPortId: left || undefined, rightPortId: right || undefined })}>Salva</Button></div></Modal>;
}
