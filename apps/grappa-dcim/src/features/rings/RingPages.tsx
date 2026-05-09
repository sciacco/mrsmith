import { Button, Modal } from '@mrsmith/ui';
import { useEffect, useMemo, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import {
  useFiberRingDetail,
  useFiberRingKML,
  useFiberRingMutations,
  useFiberRingTopology,
  useFiberRings,
  useGrappaDCIMMeta,
} from '../../api/queries';
import type { Artifact, FiberRing, FiberRingArc, FiberRingInput, FiberRingNode, FiberRingRoute } from '../../api/types';
import { ConfirmModal, destructiveBody, Detail, errorText, NumberField, SelectField, TextField, valueOrDash } from '../equipment/assetPageUtils';
import styles from '../facilities/workspace.module.css';
import ringStyles from './rings.module.css';

type RingTab = 'riepilogo' | 'topologia' | 'tratte' | 'kml' | 'storico';

const tabs: Array<{ key: RingTab; label: string }> = [
  { key: 'riepilogo', label: 'Riepilogo' },
  { key: 'topologia', label: 'Topologia' },
  { key: 'tratte', label: 'Tratte' },
  { key: 'kml', label: 'KML' },
  { key: 'storico', label: 'Storico' },
];

export function FiberRingsPage() {
  const { ringId } = useParams();
  const selectedId = ringId ? Number(ringId) : null;
  const navigate = useNavigate();
  const meta = useGrappaDCIMMeta();
  const canOperate = Boolean(meta.data?.canOperate);
  const [q, setQ] = useState('');
  const [status, setStatus] = useState('active');
  const [customerFilter, setCustomerFilter] = useState('');
  const [activeTab, setActiveTab] = useState<RingTab>('riepilogo');
  const [editing, setEditing] = useState<FiberRing | null | 'new'>(null);
  const [increasing, setIncreasing] = useState<FiberRing | null>(null);
  const [ceasing, setCeasing] = useState<FiberRing | null>(null);
  const [deleting, setDeleting] = useState<FiberRing | null>(null);
  const [editingNode, setEditingNode] = useState<FiberRingNode | null>(null);
  const [editingArc, setEditingArc] = useState<FiberRingArc | null>(null);
  const [editingRoutes, setEditingRoutes] = useState<FiberRingArc | null>(null);
  const [uploadingKml, setUploadingKml] = useState(false);
  const parsedCustomerFilter = customerFilter.trim() === '' ? null : Number(customerFilter);
  const customerId = parsedCustomerFilter && parsedCustomerFilter > 0 ? parsedCustomerFilter : null;
  const rings = useFiberRings({ q, status, customerId });
  const detail = useFiberRingDetail(selectedId);
  const topology = useFiberRingTopology(selectedId);
  const kml = useFiberRingKML(selectedId);
  const mutations = useFiberRingMutations();
  const selected = selectedId ? detail.data ?? rings.data?.find((item) => item.id === selectedId) : null;
  const nodeById = useMemo(() => {
    const map = new Map<number, FiberRingNode>();
    topology.data?.nodes.forEach((node) => map.set(node.id, node));
    return map;
  }, [topology.data?.nodes]);

  return (
    <main className={styles.page}>
      <header className={styles.header}>
        <div className={styles.titleBlock}>
          <span className={styles.eyebrow}>Topologia</span>
          <h1 className={styles.title}>Anelli fibra</h1>
          <p className={styles.subtitle}>Workspace operativo per anelli, nodi, tratte e tracciati KML protetti.</p>
        </div>
        {canOperate ? <Button onClick={() => setEditing('new')}>Nuovo anello</Button> : null}
      </header>

      <section className={styles.toolbar}>
        <input value={q} onChange={(event) => setQ(event.target.value)} placeholder="Cerca anello" />
        <select value={status} onChange={(event) => setStatus(event.target.value)}>
          <option value="active">Attivi</option>
          <option value="all">Tutti</option>
          <option value="Cessato">Cessati</option>
        </select>
        <input type="number" min="1" value={customerFilter} onChange={(event) => setCustomerFilter(event.target.value)} placeholder="Cliente" />
      </section>

      <section className={styles.split}>
        <div className={styles.tableWrap}>
          <table className={styles.table}>
            <thead><tr><th>Anello</th><th>Cliente</th><th>Nodi</th><th>Tratte</th><th>KML</th><th>Stato</th></tr></thead>
            <tbody>
              {rings.data?.map((item) => (
                <tr key={item.id} className={`${styles.clickable} ${selectedId === item.id ? styles.selectedRow : ''}`} onClick={() => navigate(`/anelli-fibra/${item.id}`)}>
                  <td>{item.name}</td>
                  <td>{valueOrDash(item.customerId)}</td>
                  <td>{item.nodeTotal}/{item.nodeCount}</td>
                  <td>{item.arcTotal}</td>
                  <td>{item.kmlArtifactTotal + (item.kmlFilePresent ? 1 : 0)}</td>
                  <td><span className={item.status.toLowerCase() === 'cessato' ? styles.badgeDanger : styles.badgeMuted}>{item.status}</span></td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>

        <aside className={styles.panel}>
          {selected ? (
            <>
              <div className={styles.detailGrid}>
                <Detail label="Anello" value={selected.name} />
                <Detail label="Cliente" value={selected.customerId} />
                <Detail label="Codice Ordine" value={selected.orderCode} />
                <Detail label="Serial Number" value={selected.serialNumber} />
                <Detail label="Nodi" value={`${selected.nodeTotal}/${selected.nodeCount}`} />
                <Detail label="Tratte" value={selected.arcTotal} />
                <Detail label="KML" value={selected.kmlArtifactTotal + (selected.kmlFilePresent ? 1 : 0)} />
                <Detail label="Stato" value={selected.status} />
              </div>
              <div className={styles.tabs}>
                {tabs.map((tab) => <button key={tab.key} className={`${styles.tab} ${activeTab === tab.key ? styles.tabActive : ''}`} onClick={() => setActiveTab(tab.key)}>{tab.label}</button>)}
              </div>
              {canOperate ? (
                <div className={styles.actions}>
                  <Button variant="secondary" onClick={() => setEditing(selected)}>Modifica</Button>
                  <Button variant="secondary" onClick={() => setIncreasing(selected)}>Aumenta nodi</Button>
                  <Button variant="secondary" onClick={() => setCeasing(selected)}>Cessa anello</Button>
                  <Button variant="danger" onClick={() => setDeleting(selected)} disabled={!selected.deleteCheck?.allowed}>Elimina</Button>
                </div>
              ) : null}
              {!selected.deleteCheck?.allowed ? <p className={styles.emptyText}>Eliminazione bloccata: usa la cessazione per anelli con dati operativi.</p> : null}
              {!selected.topologyConsistent ? <p className={ringStyles.blockedNote}>Topologia da verificare: nodi e tratte non sono allineati.</p> : null}
            </>
          ) : <Empty title="Seleziona un anello" text="Apri un anello per gestire topologia e KML." />}
        </aside>
      </section>

      {selected ? (
        <section className={styles.panel}>
          {activeTab === 'riepilogo' ? <RingSummary ring={selected} /> : null}
          {activeTab === 'topologia' ? <TopologyPanel topology={topology.data} selectedNode={editingNode} selectedArc={editingArc} onNode={setEditingNode} onArc={setEditingArc} /> : null}
          {activeTab === 'tratte' ? <RoutesPanel arcs={topology.data?.arcs ?? []} onEditArc={setEditingArc} onEditRoutes={setEditingRoutes} canOperate={canOperate} /> : null}
          {activeTab === 'kml' ? <KMLPanel artifacts={kml.data?.artifacts ?? []} canOperate={canOperate} onUpload={() => setUploadingKml(true)} onDownload={(artifact) => downloadArtifact(artifact, mutations.downloadArtifact.mutateAsync)} /> : null}
          {activeTab === 'storico' ? <HistoryPanel ring={selected} artifacts={kml.data?.artifacts ?? []} /> : null}
        </section>
      ) : null}

      <RingModal
        open={Boolean(editing)}
        value={editing === 'new' ? null : editing}
        onClose={() => setEditing(null)}
        onSave={(value) => mutations.saveRing.mutate(value, { onSuccess: () => setEditing(null) })}
        loading={mutations.saveRing.isPending}
      />
      <IncreaseNodesModal open={Boolean(increasing)} ring={increasing} onClose={() => setIncreasing(null)} onSave={(ring, nodeCount) => mutations.increaseNodes.mutate({ id: ring.id, nodeCount }, { onSuccess: () => setIncreasing(null) })} loading={mutations.increaseNodes.isPending} />
      <NodeModal open={Boolean(editingNode)} value={editingNode} canOperate={canOperate} onClose={() => setEditingNode(null)} onSave={(node, body) => selectedId && mutations.updateNode.mutate({ ringId: selectedId, nodeId: node.id, body: nodePatch(body) }, { onSuccess: () => setEditingNode(null) })} loading={mutations.updateNode.isPending} />
      <ArcModal open={Boolean(editingArc)} value={editingArc} nodeById={nodeById} canOperate={canOperate} onClose={() => setEditingArc(null)} onSave={(arc, body) => selectedId && mutations.updateArc.mutate({ ringId: selectedId, arcId: arc.id, body: arcPatch(body) }, { onSuccess: () => setEditingArc(null) })} loading={mutations.updateArc.isPending} />
      <RoutesModal open={Boolean(editingRoutes)} arc={editingRoutes} onClose={() => setEditingRoutes(null)} onSave={(arc, routes) => selectedId && mutations.replaceRoutes.mutate({ ringId: selectedId, arcId: arc.id, routes }, { onSuccess: () => setEditingRoutes(null) })} loading={mutations.replaceRoutes.isPending} />
      <KMLUploadModal open={uploadingKml} onClose={() => setUploadingKml(false)} onSave={(file, name, detailText) => selectedId && mutations.uploadKML.mutate({ ringId: selectedId, file, name, detail: detailText }, { onSuccess: () => setUploadingKml(false) })} loading={mutations.uploadKML.isPending} />
      <ConfirmModal open={Boolean(ceasing)} title="Cessa anello" message="La cessazione mantiene topologia, tratte e KML storici." onClose={() => setCeasing(null)} onConfirm={() => ceasing && mutations.ceaseRing.mutate({ id: ceasing.id, body: destructiveBody }, { onSuccess: () => setCeasing(null) })} loading={mutations.ceaseRing.isPending} />
      <ConfirmModal open={Boolean(deleting)} title="Elimina anello" message="L'eliminazione e consentita solo senza dati operativi, KML, tratte, coordinate o riferimenti." onClose={() => setDeleting(null)} onConfirm={() => deleting && mutations.deleteRing.mutate({ id: deleting.id, body: destructiveBody }, { onSuccess: () => { setDeleting(null); navigate('/anelli-fibra'); } })} loading={mutations.deleteRing.isPending} />

      {mutations.saveRing.error ? <p className={styles.emptyText}>{errorText(mutations.saveRing.error, 'Salvataggio non riuscito.')}</p> : null}
      {mutations.increaseNodes.error ? <p className={styles.emptyText}>{errorText(mutations.increaseNodes.error, 'Aumento nodi non riuscito.')}</p> : null}
      {mutations.updateNode.error ? <p className={styles.emptyText}>{errorText(mutations.updateNode.error, 'Aggiornamento nodo non riuscito.')}</p> : null}
      {mutations.updateArc.error ? <p className={styles.emptyText}>{errorText(mutations.updateArc.error, 'Aggiornamento tratta non riuscito.')}</p> : null}
      {mutations.replaceRoutes.error ? <p className={styles.emptyText}>{errorText(mutations.replaceRoutes.error, 'Aggiornamento dettagli non riuscito.')}</p> : null}
      {mutations.uploadKML.error ? <p className={styles.emptyText}>{errorText(mutations.uploadKML.error, 'Caricamento KML non riuscito.')}</p> : null}
      {mutations.deleteRing.error ? <p className={styles.emptyText}>{errorText(mutations.deleteRing.error, 'Eliminazione non riuscita.')}</p> : null}
    </main>
  );
}

function RingSummary({ ring }: { ring: FiberRing }) {
  return (
    <div className={styles.detailGrid}>
      <Detail label="Nodi generati" value={ring.nodeTotal} />
      <Detail label="Tratte generate" value={ring.arcTotal} />
      <Detail label="Dettagli tratte" value={ring.routeTotal} />
      <Detail label="KML conservati" value={ring.kmlArtifactTotal + (ring.kmlFilePresent ? 1 : 0)} />
      <Detail label="Note" value={ring.note} />
      <Detail label="Riduzione nodi" value="Non consentita" />
    </div>
  );
}

function TopologyPanel({ topology, selectedNode, selectedArc, onNode, onArc }: { topology?: import('../../api/types').FiberRingTopology; selectedNode: FiberRingNode | null; selectedArc: FiberRingArc | null; onNode: (node: FiberRingNode) => void; onArc: (arc: FiberRingArc) => void }) {
  const nodes = topology?.nodes ?? [];
  const arcs = topology?.arcs ?? [];
  return (
    <div className={ringStyles.topologyGrid}>
      <div className={ringStyles.ringCanvas}>
        <div className={ringStyles.nodeMap}>
          {nodes.map((node, index) => {
            const angle = (Math.PI * 2 * index) / Math.max(nodes.length, 1) - Math.PI / 2;
            const x = 280 + Math.cos(angle) * 190;
            const y = 180 + Math.sin(angle) * 130;
            return <button key={node.id} className={`${ringStyles.nodeButton} ${selectedNode?.id === node.id ? ringStyles.nodeButtonActive : ''}`} style={{ left: x, top: y }} onClick={() => onNode(node)}>{node.identifier}</button>;
          })}
        </div>
      </div>
      <div className={ringStyles.arcList}>
        {arcs.map((arc) => (
          <button key={arc.id} className={`${ringStyles.arcButton} ${selectedArc?.id === arc.id ? ringStyles.arcButtonActive : ''}`} onClick={() => onArc(arc)}>
            <span><span className={ringStyles.rowTitle}>{valueOrDash(arc.fromIdentifier)} - {valueOrDash(arc.toIdentifier)}</span><br /><span className={ringStyles.rowMeta}>{valueOrDash(arc.distance ?? 0)} km, attenuazione {valueOrDash(arc.attenuation ?? 0)}</span></span>
            <span className={styles.badgeMuted}>{arc.routes?.length ?? 0} dettagli</span>
          </button>
        ))}
      </div>
    </div>
  );
}

function RoutesPanel({ arcs, onEditArc, onEditRoutes, canOperate }: { arcs: FiberRingArc[]; onEditArc: (arc: FiberRingArc) => void; onEditRoutes: (arc: FiberRingArc) => void; canOperate: boolean }) {
  return (
    <div className={ringStyles.arcList}>
      {arcs.map((arc) => (
        <div key={arc.id} className={ringStyles.artifactRow}>
          <div>
            <div className={ringStyles.rowTitle}>{valueOrDash(arc.fromIdentifier)} - {valueOrDash(arc.toIdentifier)}</div>
            <div className={ringStyles.rowMeta}>Riferimento {valueOrDash(arc.reference)} · {arc.routes?.length ?? 0} dettagli conservati</div>
          </div>
          <div className={styles.actions}>
            <Button variant="secondary" onClick={() => onEditArc(arc)}>{canOperate ? 'Tratta' : 'Apri'}</Button>
            {canOperate ? <Button variant="secondary" onClick={() => onEditRoutes(arc)}>Dettagli</Button> : null}
          </div>
        </div>
      ))}
    </div>
  );
}

function KMLPanel({ artifacts, canOperate, onUpload, onDownload }: { artifacts: Artifact[]; canOperate: boolean; onUpload: () => void; onDownload: (artifact: Artifact) => void }) {
  return (
    <div className={ringStyles.artifactList}>
      {canOperate ? <div className={styles.actions}><Button onClick={onUpload}>Carica KML</Button></div> : null}
      {artifacts.length === 0 ? <Empty title="Nessun KML" text="Non ci sono tracciati KML associati a questo anello." /> : null}
      {artifacts.map((artifact) => (
        <div key={artifact.id} className={ringStyles.artifactRow}>
          <div><div className={ringStyles.rowTitle}>{artifact.name || artifact.fileName || 'KML'}</div><div className={ringStyles.rowMeta}>{artifact.available ? 'File disponibile' : 'File storico non disponibile'} · {valueOrDash(artifact.detail)}</div></div>
          {artifact.available ? <Button variant="secondary" onClick={() => onDownload(artifact)}>Scarica</Button> : <span className={styles.badgeMuted}>Non disponibile</span>}
        </div>
      ))}
    </div>
  );
}

function HistoryPanel({ ring, artifacts }: { ring: FiberRing; artifacts: Artifact[] }) {
  return (
    <div className={styles.detailGrid}>
      <Detail label="Stato" value={ring.status} />
      <Detail label="Codice Ordine" value={ring.orderCode} />
      <Detail label="Serial Number" value={ring.serialNumber} />
      <Detail label="KML storici" value={artifacts.length} />
    </div>
  );
}

function RingModal({ open, value, onClose, onSave, loading }: { open: boolean; value: FiberRing | null; onClose: () => void; onSave: (value: FiberRingInput & { id?: number }) => void; loading: boolean }) {
  const initial = (): FiberRingInput => ({ name: value?.name ?? '', customerId: value?.customerId, nodeCount: value?.nodeCount ?? 3, note: value?.note ?? '', serialNumber: value?.serialNumber ?? '', orderCode: value?.orderCode ?? '', status: value?.status ?? 'Attivo' });
  const [draft, setDraft] = useState<FiberRingInput>(initial);
  useEffect(() => setDraft(initial()), [value]);
  const decreaseBlocked = Boolean(value && draft.nodeCount < value.nodeCount);
  return (
    <Modal open={open} onClose={onClose} title={value ? 'Modifica anello fibra' : 'Nuovo anello fibra'} size="wide">
      <div className={styles.formGrid}>
        <TextField label="Anello" value={draft.name} onChange={(name) => setDraft({ ...draft, name })} />
        <NumberField label="Cliente" value={draft.customerId ?? 0} onChange={(customerId) => setDraft({ ...draft, customerId: customerId || undefined })} />
        {value ? (
          <div className={styles.field}><label>Nodi</label><input type="number" value={draft.nodeCount} disabled readOnly /></div>
        ) : (
          <NumberField label="Nodi" value={draft.nodeCount} onChange={(nodeCount) => setDraft({ ...draft, nodeCount })} />
        )}
        <SelectField label="Stato" value={draft.status ?? 'Attivo'} onChange={(status) => setDraft({ ...draft, status })} options={['Attivo', 'Cessato']} />
        <TextField label="Codice Ordine" value={draft.orderCode ?? ''} onChange={(orderCode) => setDraft({ ...draft, orderCode })} />
        <TextField label="Serial Number" value={draft.serialNumber ?? ''} onChange={(serialNumber) => setDraft({ ...draft, serialNumber })} />
        <TextField label="Note" value={draft.note ?? ''} onChange={(note) => setDraft({ ...draft, note })} />
      </div>
      {decreaseBlocked ? <p className={ringStyles.blockedNote}>La riduzione del numero nodi non e consentita.</p> : null}
      {value ? <p className={ringStyles.blockedNote}>Per aggiungere nodi usa l'azione Aumenta nodi.</p> : null}
      <div className={styles.modalActions}><Button variant="secondary" onClick={onClose}>Annulla</Button><Button loading={loading} disabled={!draft.name || draft.nodeCount <= 0 || decreaseBlocked} onClick={() => onSave({ ...draft, id: value?.id })}>Salva</Button></div>
    </Modal>
  );
}

function IncreaseNodesModal({ open, ring, onClose, onSave, loading }: { open: boolean; ring: FiberRing | null; onClose: () => void; onSave: (ring: FiberRing, nodeCount: number) => void; loading: boolean }) {
  const [nodeCount, setNodeCount] = useState(0);
  useEffect(() => setNodeCount((ring?.nodeCount ?? 0) + 1), [ring]);
  if (!ring) return null;
  const valid = nodeCount > ring.nodeCount;
  return (
    <Modal open={open} onClose={onClose} title="Aumenta nodi">
      <div className={styles.formGrid}>
        <Detail label="Anello" value={ring.name} />
        <Detail label="Nodi attuali" value={ring.nodeCount} />
        <NumberField label="Nuovo totale nodi" value={nodeCount} onChange={setNodeCount} />
      </div>
      <p className={ringStyles.confirmNote}>Confermi l'aumento a {nodeCount || '-'} nodi? Verranno aggiunti nodi e tratte alla topologia circolare.</p>
      {!valid ? <p className={ringStyles.blockedNote}>Il nuovo totale deve essere maggiore dei nodi attuali.</p> : null}
      <div className={styles.modalActions}><Button variant="secondary" onClick={onClose}>Annulla</Button><Button loading={loading} disabled={!valid} onClick={() => onSave(ring, nodeCount)}>Conferma aumento</Button></div>
    </Modal>
  );
}

function NodeModal({ open, value, canOperate, onClose, onSave, loading }: { open: boolean; value: FiberRingNode | null; canOperate: boolean; onClose: () => void; onSave: (node: FiberRingNode, body: Partial<FiberRingNode>) => void; loading: boolean }) {
  const [draft, setDraft] = useState<FiberRingNode | null>(value);
  useEffect(() => setDraft(value), [value]);
  if (!draft || !value) return null;
  if (!canOperate) {
    return (
      <Modal open={open} onClose={onClose} title="Nodo fibra" size="wide">
        <div className={styles.detailGrid}>
          <Detail label="Identificativo" value={value.identifier} />
          <Detail label="Indirizzo" value={value.address} />
          <Detail label="Posizione" value={value.position} />
          <Detail label="Cliente" value={value.customerId} />
          <Detail label="Longitudine" value={value.longitude} />
          <Detail label="Latitudine" value={value.latitude} />
          <Detail label="Switch" value={value.switchModel} />
          <Detail label="Seriale switch" value={value.switchSerialNumber} />
          <Detail label="IP" value={value.ipAddress} />
          <Detail label="Note" value={value.note} />
        </div>
        <div className={styles.modalActions}><Button variant="secondary" onClick={onClose}>Chiudi</Button></div>
      </Modal>
    );
  }
  return (
    <Modal open={open} onClose={onClose} title="Nodo fibra" size="wide">
      <div className={styles.formGrid}>
        <TextField label="Identificativo" value={draft.identifier} onChange={(identifier) => setDraft({ ...draft, identifier })} />
        <TextField label="Indirizzo" value={draft.address} onChange={(address) => setDraft({ ...draft, address })} />
        <NumberField label="Posizione" value={draft.position ?? 0} onChange={(position) => setDraft({ ...draft, position })} />
        <NumberField label="Cliente" value={draft.customerId ?? 0} onChange={(customerId) => setDraft({ ...draft, customerId: customerId || undefined })} />
        <NumberField label="Longitudine" value={draft.longitude ?? 0} onChange={(longitude) => setDraft({ ...draft, longitude })} />
        <NumberField label="Latitudine" value={draft.latitude ?? 0} onChange={(latitude) => setDraft({ ...draft, latitude })} />
        <TextField label="Switch" value={draft.switchModel ?? ''} onChange={(switchModel) => setDraft({ ...draft, switchModel })} />
        <TextField label="Seriale switch" value={draft.switchSerialNumber ?? ''} onChange={(switchSerialNumber) => setDraft({ ...draft, switchSerialNumber })} />
        <TextField label="IP" value={draft.ipAddress ?? ''} onChange={(ipAddress) => setDraft({ ...draft, ipAddress })} />
        <TextField label="Note" value={draft.note ?? ''} onChange={(note) => setDraft({ ...draft, note })} />
      </div>
      <div className={styles.modalActions}><Button variant="secondary" onClick={onClose}>Annulla</Button><Button loading={loading} disabled={!draft.identifier} onClick={() => onSave(value, draft)}>Salva</Button></div>
    </Modal>
  );
}

function ArcModal({ open, value, nodeById, canOperate, onClose, onSave, loading }: { open: boolean; value: FiberRingArc | null; nodeById: Map<number, FiberRingNode>; canOperate: boolean; onClose: () => void; onSave: (arc: FiberRingArc, body: Partial<FiberRingArc>) => void; loading: boolean }) {
  const [draft, setDraft] = useState<FiberRingArc | null>(value);
  useEffect(() => setDraft(value), [value]);
  if (!draft || !value) return null;
  if (!canOperate) {
    return (
      <Modal open={open} onClose={onClose} title="Tratta fibra" size="wide">
        <div className={styles.detailGrid}>
          <Detail label="Da" value={nodeById.get(value.fromNodeId)?.identifier ?? value.fromIdentifier} />
          <Detail label="A" value={nodeById.get(value.toNodeId)?.identifier ?? value.toIdentifier} />
          <Detail label="Distanza" value={value.distance ?? 0} />
          <Detail label="Attenuazione" value={value.attenuation ?? 0} />
          <Detail label="Riferimento" value={value.reference} />
          <Detail label="Riferimento Metroweb" value={value.metrowebReference} />
          <Detail label="Data rilascio" value={value.releasedAt?.slice(0, 10)} />
          <Detail label="Dettagli conservati" value={value.routes?.length ?? 0} />
        </div>
        <div className={styles.modalActions}><Button variant="secondary" onClick={onClose}>Chiudi</Button></div>
      </Modal>
    );
  }
  return (
    <Modal open={open} onClose={onClose} title="Tratta fibra" size="wide">
      <div className={styles.detailGrid}><Detail label="Da" value={nodeById.get(value.fromNodeId)?.identifier ?? value.fromIdentifier} /><Detail label="A" value={nodeById.get(value.toNodeId)?.identifier ?? value.toIdentifier} /></div>
      <div className={styles.formGrid}>
        <NumberField label="Distanza" value={draft.distance ?? 0} onChange={(distance) => setDraft({ ...draft, distance })} />
        <NumberField label="Attenuazione" value={draft.attenuation ?? 0} onChange={(attenuation) => setDraft({ ...draft, attenuation })} />
        <TextField label="Riferimento" value={draft.reference ?? ''} onChange={(reference) => setDraft({ ...draft, reference })} />
        <TextField label="Riferimento Metroweb" value={draft.metrowebReference ?? ''} onChange={(metrowebReference) => setDraft({ ...draft, metrowebReference })} />
        <TextField label="Data rilascio" type="date" value={draft.releasedAt?.slice(0, 10) ?? ''} onChange={(releasedAt) => setDraft({ ...draft, releasedAt })} />
      </div>
      <div className={styles.modalActions}><Button variant="secondary" onClick={onClose}>Annulla</Button><Button loading={loading} onClick={() => onSave(value, draft)}>Salva</Button></div>
    </Modal>
  );
}

function RoutesModal({ open, arc, onClose, onSave, loading }: { open: boolean; arc: FiberRingArc | null; onClose: () => void; onSave: (arc: FiberRingArc, routes: FiberRingRoute[]) => void; loading: boolean }) {
  const [items, setItems] = useState<FiberRingRoute[]>(arc?.routes ?? []);
  useEffect(() => setItems(arc?.routes ?? []), [arc]);
  const update = (index: number, patch: Partial<FiberRingRoute>) => setItems((current) => current.map((item, i) => i === index ? { ...item, ...patch } : item));
  if (!arc) return null;
  return (
    <Modal open={open} onClose={onClose} title="Dettagli tratta" size="wide">
      <div className={styles.socketList}>
        {items.map((item, index) => (
          <div key={index} className={styles.socketRow}>
            <div className={styles.formGrid}>
              <TextField label="Identificativo" value={item.identifier ?? ''} onChange={(identifier) => update(index, { identifier })} />
              <TextField label="P armadio" value={item.sourceCabinet ?? ''} onChange={(sourceCabinet) => update(index, { sourceCabinet })} />
              <TextField label="P cavo" value={item.sourceCable ?? ''} onChange={(sourceCable) => update(index, { sourceCable })} />
              <TextField label="P fibre" value={item.sourceFibers ?? ''} onChange={(sourceFibers) => update(index, { sourceFibers })} />
              <TextField label="D armadio" value={item.destinationCabinet ?? ''} onChange={(destinationCabinet) => update(index, { destinationCabinet })} />
              <TextField label="D cavo" value={item.destinationCable ?? ''} onChange={(destinationCable) => update(index, { destinationCable })} />
              <TextField label="D fibre" value={item.destinationFibers ?? ''} onChange={(destinationFibers) => update(index, { destinationFibers })} />
              <NumberField label="Lunghezza tratta" value={item.routeLengthMeters ?? 0} onChange={(routeLengthMeters) => update(index, { routeLengthMeters })} />
            </div>
            <Button variant="secondary" onClick={() => setItems((current) => current.filter((_, i) => i !== index))}>Rimuovi</Button>
          </div>
        ))}
      </div>
      <div className={styles.modalActions}><Button variant="secondary" onClick={() => setItems([...items, {}])}>Aggiungi dettaglio</Button><Button variant="secondary" onClick={onClose}>Annulla</Button><Button loading={loading} onClick={() => onSave(arc, items)}>Salva</Button></div>
    </Modal>
  );
}

function KMLUploadModal({ open, onClose, onSave, loading }: { open: boolean; onClose: () => void; onSave: (file: File, name?: string, detail?: string) => void; loading: boolean }) {
  const [file, setFile] = useState<File | null>(null);
  const [name, setName] = useState('');
  const [detail, setDetail] = useState('');
  return (
    <Modal open={open} onClose={onClose} title="Carica KML">
      <div className={styles.formGrid}>
        <TextField label="Nome" value={name} onChange={setName} />
        <TextField label="Dettaglio" value={detail} onChange={setDetail} />
        <div className={styles.field}><label>KML</label><input type="file" accept=".kml,.kmz" onChange={(event) => setFile(event.target.files?.[0] ?? null)} /></div>
      </div>
      <div className={styles.modalActions}><Button variant="secondary" onClick={onClose}>Annulla</Button><Button loading={loading} disabled={!file} onClick={() => file && onSave(file, name, detail)}>Carica</Button></div>
    </Modal>
  );
}

function Empty({ title, text }: { title: string; text: string }) {
  return <div className={styles.emptyPanel}><div className={styles.emptyTitle}>{title}</div><p className={styles.emptyText}>{text}</p></div>;
}

function nodePatch(node: Partial<FiberRingNode>) {
  return {
    identifier: node.identifier,
    address: node.address,
    customerId: node.customerId,
    longitude: node.longitude,
    latitude: node.latitude,
    position: node.position,
    switchModel: node.switchModel,
    switchSerialNumber: node.switchSerialNumber,
    ipAddress: node.ipAddress,
    note: node.note,
  };
}

function arcPatch(arc: Partial<FiberRingArc>) {
  return {
    distance: arc.distance,
    attenuation: arc.attenuation,
    reference: arc.reference,
    metrowebReference: arc.metrowebReference,
    releasedAt: arc.releasedAt,
  };
}

async function downloadArtifact(artifact: Artifact, runDownload: (artifactId: number) => Promise<Blob>) {
  const blob = await runDownload(artifact.id);
  const url = URL.createObjectURL(blob);
  const anchor = document.createElement('a');
  anchor.href = url;
  anchor.download = artifact.fileName || `${artifact.name || 'kml'}.kml`;
  document.body.appendChild(anchor);
  anchor.click();
  anchor.remove();
  URL.revokeObjectURL(url);
}
