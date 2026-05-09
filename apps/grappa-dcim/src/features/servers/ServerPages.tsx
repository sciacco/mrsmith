import { Button, Icon, Modal, SearchInput, SingleSelect, Skeleton, useToast } from '@mrsmith/ui';
import { useEffect, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { useEquipment, useGrappaDCIMMeta, useServerChildren, useServerCredentials, useServerDetail, useServerMutations, useServers } from '../../api/queries';
import type { ServerCredentialsInput, ServerInput, ServerItem } from '../../api/types';
import { ViewState } from '../../components/ViewState';
import styles from '../facilities/workspace.module.css';
import { Detail, NumberField, SelectField, TextField, errorText } from '../equipment/assetPageUtils';

export function ServersPage() {
  const params = useParams();
  const navigate = useNavigate();
  const toast = useToast();
  const meta = useGrappaDCIMMeta();
  const canOperate = Boolean(meta.data?.canOperate);
  const canViewCredentials = Boolean(meta.data?.canViewCredentials);
  const [q, setQ] = useState('');
  const [status, setStatus] = useState<'active' | 'all'>('active');
  const [kind, setKind] = useState<'all' | 'Fisico' | 'Virtuale'>('all');
  const [editing, setEditing] = useState<ServerItem | null | 'new'>(null);
  const [editingCredentials, setEditingCredentials] = useState(false);
  const [tab, setTab] = useState<'summary' | 'hardware' | 'access' | 'cards' | 'applications' | 'services' | 'ports'>('summary');
  const selectedId = params.serverId ? Number(params.serverId) : null;
  const servers = useServers({ q, status, kind });
  const selected = servers.data?.find((item) => item.id === selectedId) ?? servers.data?.[0] ?? null;
  const detail = useServerDetail(selected?.id ?? null);
  const children = useServerChildren(selected?.id ?? null);
  const credentials = useServerCredentials(selected?.id ?? null, canViewCredentials);
  const mutations = useServerMutations();

  async function saveServer(input: ServerInput & { id?: number }) {
    try {
      const result = await mutations.saveServer.mutateAsync(input);
      toast.toast(result.message || 'Server salvato.');
      setEditing(null);
    } catch (error) {
      toast.toast(errorText(error, 'Salvataggio server non riuscito.'), 'error');
    }
  }

  async function saveCredentials(input: ServerCredentialsInput) {
    if (!selected) return;
    try {
      const result = await mutations.saveCredentials.mutateAsync({ id: selected.id, body: input });
      toast.toast(result.message || 'Credenziali aggiornate.');
      setEditingCredentials(false);
    } catch (error) {
      toast.toast(errorText(error, 'Aggiornamento credenziali non riuscito.'), 'error');
    }
  }

  return (
    <section className={styles.page}>
      <div className={styles.header}>
        <div className={styles.titleBlock}>
          <span className={styles.eyebrow}>Asset</span>
          <h1 className={styles.title}>Server</h1>
          <p className={styles.subtitle}>Server fisici e virtuali con dettagli hardware, applicazioni, servizi e accessi.</p>
        </div>
        {canOperate ? <Button onClick={() => setEditing('new')} leftIcon={<Icon name="plus" size={16} />}>Nuovo server</Button> : null}
      </div>
      <div className={styles.toolbar}>
        <SearchInput value={q} onChange={setQ} placeholder="Cerca server..." />
        <SingleSelect options={[{ value: 'active', label: 'Solo attivi' }, { value: 'all', label: 'Tutti' }]} selected={status} onChange={(value) => setStatus((value ?? 'active') as 'active' | 'all')} searchable={false} />
        <SingleSelect options={[{ value: 'all', label: 'Tutti' }, { value: 'Fisico', label: 'Fisici' }, { value: 'Virtuale', label: 'Virtuali' }]} selected={kind} onChange={(value) => setKind((value ?? 'all') as typeof kind)} searchable={false} />
      </div>
      {servers.isLoading ? (
        <div className={styles.panel}><Skeleton rows={8} /></div>
      ) : servers.error ? (
        <ViewState title="Server non disponibili" message="Non e stato possibile caricare il registro server." tone="error" />
      ) : (servers.data?.length ?? 0) === 0 ? (
        <div className={styles.emptyPanel}><h3 className={styles.emptyTitle}>Nessun server trovato</h3><p className={styles.emptyText}>Modifica i filtri o aggiungi un server operativo.</p></div>
      ) : (
        <div className={styles.split}>
          <div className={styles.tableWrap}>
            <table className={styles.table}>
              <thead><tr><th>Server</th><th>Tipo</th><th>Asset</th><th>Stato</th><th>Azioni</th></tr></thead>
              <tbody>{servers.data?.map((item) => <tr key={item.id} className={`${styles.clickable} ${selected?.id === item.id ? styles.selectedRow : ''}`} onClick={() => navigate(`/server/${item.id}`)}><td><strong>{item.name ?? item.hostname ?? `Server ${item.id}`}</strong><br /><span className={styles.muted}>{item.managementIp ?? item.operatingSystem ?? '-'}</span></td><td>{item.kind}</td><td>{item.equipmentName ?? item.rackName ?? '-'}</td><td><span className={item.status === 'Cessato' ? styles.badgeDanger : styles.badge}>{item.status ?? 'Attivo'}</span></td><td><div className={styles.actions}>{canOperate ? <Button size="sm" variant="secondary" onClick={(event) => { event.stopPropagation(); setEditing(item); }}>Modifica</Button> : null}</div></td></tr>)}</tbody>
            </table>
          </div>
          <div className={styles.panel}>
            {detail.isLoading ? <Skeleton rows={8} /> : detail.error || !detail.data ? (
              <div className={styles.emptyPanel}><h3 className={styles.emptyTitle}>Dettaglio non disponibile</h3><p className={styles.emptyText}>Seleziona un server dal registro.</p></div>
            ) : (
              <>
                <div className={styles.header}><div><h2 className={styles.emptyTitle}>{detail.data.name ?? detail.data.hostname ?? `Server ${detail.data.id}`}</h2><p className={styles.emptyText}>{detail.data.kind} · {detail.data.operatingSystem ?? 'Sistema non indicato'}</p></div><span className={styles.badgeMuted}>{detail.data.status ?? 'Attivo'}</span></div>
                <div className={styles.tabs}>{[['summary', 'Riepilogo'], ['hardware', 'Hardware'], ['access', 'Accessi'], ['cards', 'Schede'], ['applications', 'Applicazioni'], ['services', 'Servizi'], ['ports', 'Porte']].map(([key, label]) => <button key={key} className={`${styles.tab} ${tab === key ? styles.tabActive : ''}`} onClick={() => setTab(key as typeof tab)}>{label}</button>)}</div>
                {tab === 'summary' ? <ServerSummary item={detail.data} /> : null}
                {tab === 'hardware' ? <ServerHardware item={detail.data} /> : null}
                {tab === 'access' ? <CredentialPanel canView={canViewCredentials} canOperate={canOperate} credentials={credentials.data} loading={credentials.isLoading} onEdit={() => setEditingCredentials(true)} /> : null}
                {tab === 'cards' ? <SimpleRows loading={children.isLoading} rows={children.data?.cards} columns={['physicalName', 'osName', 'ip']} labels={['Nome fisico', 'Sistema', 'IP']} /> : null}
                {tab === 'applications' ? <SimpleRows loading={children.isLoading} rows={children.data?.applications} columns={['name', 'managedByCdlan']} labels={['Applicazione', 'Gestito']} /> : null}
                {tab === 'services' ? <SimpleRows loading={children.isLoading} rows={children.data?.services} columns={['name']} labels={['Servizio']} /> : null}
                {tab === 'ports' ? <SimpleRows loading={children.isLoading} rows={children.data?.ports} columns={['interfaceName', 'destinationInterface', 'portType']} labels={['Interfaccia', 'Destinazione', 'Tipo']} /> : null}
              </>
            )}
          </div>
        </div>
      )}
      <ServerModal open={editing !== null} value={editing === 'new' ? null : editing} onClose={() => setEditing(null)} onSave={saveServer} loading={mutations.saveServer.isPending} />
      <CredentialsModal open={editingCredentials} value={credentials.data} onClose={() => setEditingCredentials(false)} onSave={saveCredentials} loading={mutations.saveCredentials.isPending} />
    </section>
  );
}

function ServerSummary({ item }: { item: ServerItem }) {
  return <div className={styles.detailGrid}><Detail label="Cliente" value={item.customerId} /><Detail label="Hostname" value={item.hostname} /><Detail label="Ordine" value={item.orderCode} /><Detail label="Seriale" value={item.serialNumber ?? item.serial} /><Detail label="Rack" value={item.rackName} /><Detail label="Apparato" value={item.equipmentName} /><Detail label="IP gestione" value={item.managementIp} /><Detail label="Note" value={item.note} /></div>;
}

function ServerHardware({ item }: { item: ServerItem }) {
  return <div className={styles.detailGrid}><Detail label="Modello" value={item.model} /><Detail label="CPU" value={item.cpu} /><Detail label="Core" value={item.coreCount} /><Detail label="RAM" value={item.ram} /><Detail label="Dischi" value={item.disks} /><Detail label="Porte" value={item.portCount} /></div>;
}

function CredentialPanel({ canView, canOperate, credentials, loading, onEdit }: { canView: boolean; canOperate: boolean; credentials?: import('../../api/types').ServerCredentials; loading: boolean; onEdit: () => void }) {
  if (!canView) return <div className={styles.emptyPanel}><h3 className={styles.emptyTitle}>Credenziali riservate</h3><p className={styles.emptyText}>Gli accessi non sono disponibili in sola consultazione.</p></div>;
  if (loading) return <Skeleton rows={5} />;
  if (!credentials) return <p className={styles.emptyText}>Credenziali non disponibili.</p>;
  return <><div className={styles.detailGrid}><Detail label="Indirizzo iLO" value={credentials.iloAddress} /><Detail label="Utente iLO" value={credentials.iloUsername} /><Detail label="Accesso cliente" value={credentials.customerRootAccess} /><Detail label="Utenza cliente" value={credentials.customerUsername} /><Detail label="Utenza CDLAN" value={credentials.cdlanUsername} /><Detail label="Accessi registrati" value={[credentials.iloPasswordStored && 'iLO', credentials.rootAdministratorStored && 'root', credentials.customerPasswordStored && 'cliente', credentials.cdlanPasswordStored && 'CDLAN'].filter(Boolean).join(', ') || '-'} /></div>{canOperate ? <div className={styles.modalActions}><Button variant="secondary" onClick={onEdit}>Modifica accessi</Button></div> : null}</>;
}

function SimpleRows({ loading, rows, columns, labels }: { loading: boolean; rows?: Array<Record<string, unknown>>; columns: string[]; labels: string[] }) {
  if (loading) return <Skeleton rows={5} />;
  if (!rows?.length) return <p className={styles.emptyText}>Nessun dettaglio presente.</p>;
  return <div className={styles.tableWrap}><table className={styles.table}><thead><tr>{labels.map((label) => <th key={label}>{label}</th>)}</tr></thead><tbody>{rows.map((row) => <tr key={String(row.id)}>{columns.map((column) => <td key={column}>{String(row[column] ?? '-')}</td>)}</tr>)}</tbody></table></div>;
}

function ServerModal({ open, value, onClose, onSave, loading }: { open: boolean; value: ServerItem | null; onClose: () => void; onSave: (value: ServerInput & { id?: number }) => void; loading: boolean }) {
  const equipment = useEquipment({ status: 'active' });
  const [draft, setDraft] = useState<ServerInput>({ kind: 'Fisico', status: 'Attivo' });
  useEffect(() => {
    setDraft(value ? { kind: value.kind, name: value.name, customerId: value.customerId, status: value.status ?? 'Attivo', operatingSystem: value.operatingSystem, hostname: value.hostname, rackId: value.rackId, model: value.model, serial: value.serial, cpu: value.cpu, coreCount: value.coreCount, ram: value.ram, disks: value.disks, iloAddress: value.iloAddress, customerUsername: value.customerUsername, cdlanUsername: value.cdlanUsername, note: value.note, managementIp: value.managementIp, equipmentId: value.equipmentId, orderCode: value.orderCode, serialNumber: value.serialNumber, portCount: value.portCount } : { kind: 'Fisico', status: 'Attivo' });
  }, [value, open]);
  return <Modal open={open} onClose={onClose} title={value ? 'Modifica server' : 'Nuovo server'} size="wide"><div className={styles.formGrid}><SelectField label="Tipo" value={draft.kind} onChange={(kind) => setDraft({ ...draft, kind })} options={['Fisico', 'Virtuale']} /><TextField label="Nome" value={draft.name ?? ''} onChange={(name) => setDraft({ ...draft, name })} /><TextField label="Hostname" value={draft.hostname ?? ''} onChange={(hostname) => setDraft({ ...draft, hostname })} /><TextField label="Sistema operativo" value={draft.operatingSystem ?? ''} onChange={(operatingSystem) => setDraft({ ...draft, operatingSystem })} /><div className={styles.field}><label>Apparato collegato</label><select value={draft.equipmentId ?? 0} onChange={(event) => setDraft({ ...draft, equipmentId: Number(event.target.value) || undefined })}><option value={0}>Nessun apparato</option>{equipment.data?.map((item) => <option key={item.id} value={item.id}>{item.name}</option>)}</select></div><TextField label="IP gestione" value={draft.managementIp ?? ''} onChange={(managementIp) => setDraft({ ...draft, managementIp })} /><TextField label="Modello" value={draft.model ?? ''} onChange={(model) => setDraft({ ...draft, model })} /><TextField label="Seriale" value={draft.serialNumber ?? draft.serial ?? ''} onChange={(serialNumber) => setDraft({ ...draft, serialNumber })} /><TextField label="Codice ordine" value={draft.orderCode ?? ''} onChange={(orderCode) => setDraft({ ...draft, orderCode })} /><NumberField label="RAM" value={draft.ram ?? 0} onChange={(ram) => setDraft({ ...draft, ram })} /><NumberField label="Core" value={draft.coreCount ?? 0} onChange={(coreCount) => setDraft({ ...draft, coreCount })} /></div><div className={styles.modalActions}><Button variant="secondary" onClick={onClose}>Annulla</Button><Button loading={loading} disabled={!draft.kind} onClick={() => onSave({ ...draft, id: value?.id })}>Salva</Button></div></Modal>;
}

function CredentialsModal({ open, value, onClose, onSave, loading }: { open: boolean; value?: import('../../api/types').ServerCredentials; onClose: () => void; onSave: (value: ServerCredentialsInput) => void; loading: boolean }) {
  const [draft, setDraft] = useState<ServerCredentialsInput>({});
  useEffect(() => setDraft({ iloAddress: value?.iloAddress, iloUsername: value?.iloUsername, customerRootAccess: value?.customerRootAccess, customerUsername: value?.customerUsername, cdlanUsername: value?.cdlanUsername }), [value, open]);
  return <Modal open={open} onClose={onClose} title="Modifica accessi"><div className={styles.formGrid}><TextField label="Indirizzo iLO" value={draft.iloAddress ?? ''} onChange={(iloAddress) => setDraft({ ...draft, iloAddress })} /><TextField label="Utente iLO" value={draft.iloUsername ?? ''} onChange={(iloUsername) => setDraft({ ...draft, iloUsername })} /><TextField label="Accesso cliente" value={draft.customerRootAccess ?? ''} onChange={(customerRootAccess) => setDraft({ ...draft, customerRootAccess })} /><TextField label="Utenza cliente" value={draft.customerUsername ?? ''} onChange={(customerUsername) => setDraft({ ...draft, customerUsername })} /><TextField label="Utenza CDLAN" value={draft.cdlanUsername ?? ''} onChange={(cdlanUsername) => setDraft({ ...draft, cdlanUsername })} /></div><div className={styles.modalActions}><Button variant="secondary" onClick={onClose}>Annulla</Button><Button loading={loading} onClick={() => onSave(draft)}>Salva</Button></div></Modal>;
}
