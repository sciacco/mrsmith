import { ApiError } from '@mrsmith/api-client';
import { Button, Icon, Modal, MultiSelect, SearchInput, Skeleton, ToggleSwitch, useToast } from '@mrsmith/ui';
import { useMemo, useState } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import {
  useArticleCategories,
  useCategories,
  useDashboard,
  useDocumentTypes,
  useFornitoriMutations,
  usePaymentMethods,
  useProvider,
  useProviderCategories,
  useProviderDocuments,
  useProviders,
} from './api/queries';
import type { Category, DocumentType, Provider, ProviderPayload, ProviderReference } from './api/types';
import { countries } from './lib/countries';
import { provinces } from './lib/provinces';
import { providerTabs, referenceTypeLabel, referenceTypes, stateLabel, type ProviderTab } from './lib/reference';
import { saveBlob } from './lib/download';
import { useHasRole } from './hooks/useHasRole';

function errorTitle(error: unknown) {
  if (error instanceof ApiError) {
    if (error.status === 403) return 'Accesso non consentito';
    if (error.status === 503) return 'Servizio temporaneamente non disponibile';
  }
  return 'Dati non disponibili';
}

function stateBlock(title: string, message: string) {
  return (
    <div className="emptyState">
      <p className="emptyTitle">{title}</p>
      <p className="emptyText">{message}</p>
    </div>
  );
}

function value(value?: string | number | boolean | null) {
  if (value === null || value === undefined || value === '') return '-';
  return String(value);
}

function dateLabel(raw?: string | null) {
  if (!raw) return '-';
  const parsed = new Date(raw);
  if (Number.isNaN(parsed.getTime())) return raw.slice(0, 10);
  return new Intl.DateTimeFormat('it-IT', { dateStyle: 'medium' }).format(parsed);
}

function getProviderRefs(provider?: Provider): ProviderReference[] {
  if (!provider) return [];
  if (provider.refs?.length) return provider.refs;
  return provider.ref ? [provider.ref] : [];
}

function qualificationRef(provider?: Provider) {
  return getProviderRefs(provider).find((ref) => ref.reference_type === 'QUALIFICATION_REF');
}

function selectOptions(items: Category[] | DocumentType[] | undefined) {
  return items?.map((item) => ({ value: item.id, label: item.name })) ?? [];
}

function providerPayload(form: HTMLFormElement): ProviderPayload {
  const data = new FormData(form);
  const erp = String(data.get('erp_id') ?? '').trim();
  const country = String(data.get('country') || 'IT');
  const payload: ProviderPayload = {
    company_name: String(data.get('company_name') ?? '').trim(),
    state: String(data.get('state') || 'DRAFT'),
    vat_number: String(data.get('vat_number') ?? '').trim() || undefined,
    cf: String(data.get('cf') ?? '').trim() || undefined,
    address: String(data.get('address') ?? '').trim() || undefined,
    city: String(data.get('city') ?? '').trim() || undefined,
    postal_code: String(data.get('postal_code') ?? '').trim() || undefined,
    province: String(data.get('province') ?? '').trim() || undefined,
    erp_id: erp ? Number(erp) : null,
    language: String(data.get('language') || 'it'),
    country,
    default_payment_method: String(data.get('default_payment_method') ?? '').trim() || null,
    ref: {
      first_name: String(data.get('ref_first_name') ?? '').trim(),
      last_name: String(data.get('ref_last_name') ?? '').trim(),
      email: String(data.get('ref_email') ?? '').trim(),
      phone: String(data.get('ref_phone') ?? '').trim(),
      reference_type: 'QUALIFICATION_REF',
    },
  };
  if (data.get('skip_qualification_validation') === 'on') {
    payload.skip_qualification_validation = true;
  }
  return payload;
}

function validateProvider(payload: ProviderPayload): string | null {
  if (!payload.company_name) return 'Inserisci la ragione sociale';
  if (payload.country === 'IT' && !payload.cf && !payload.vat_number) return 'Per i fornitori italiani inserisci CF o P.IVA';
  if (payload.country === 'IT' && (payload.postal_code?.length ?? 0) < 5) return 'Per i fornitori italiani inserisci un CAP valido';
  if (payload.country === 'IT' && !payload.province) return 'Per i fornitori italiani seleziona la provincia';
  if ((payload.state === 'ACTIVE' || payload.state === 'INACTIVE') && !payload.erp_id) return 'Inserisci il codice Alyante prima di cambiare stato';
  return null;
}

export function DashboardPage() {
  const navigate = useNavigate();
  const { drafts, documents, categories } = useDashboard();
  const mutations = useFornitoriMutations();
  const loading = drafts.isLoading || documents.isLoading || categories.isLoading;
  const error = drafts.error ?? documents.error ?? categories.error;

  async function download(id: number) {
    const blob = await mutations.downloadDocument(id);
    saveBlob(blob, `documento-${id}`);
  }

  return (
    <main className="page">
      <header className="pageHeader">
        <div>
          <h1>Dashboard</h1>
          <p>Fornitori da qualificare, documenti in scadenza e categorie da gestire.</p>
        </div>
      </header>
      {loading ? <Skeleton rows={8} /> : error ? stateBlock(errorTitle(error), 'Le attivita fornitori non possono essere caricate.') : (
        <>
          <section className="metricGrid">
            <Metric label="Bozze" value={drafts.data?.length ?? 0} />
            <Metric label="Documenti in scadenza" value={documents.data?.length ?? 0} />
            <Metric label="Categorie da gestire" value={categories.data?.length ?? 0} />
          </section>
          <section className="threeTables">
            <Panel title="Fornitori da qualificare">
              <table className="table">
                <thead><tr><th>Fornitore</th><th>Stato</th><th>Codice Alyante</th></tr></thead>
                <tbody>{(drafts.data ?? []).map((row) => (
                  <tr key={row.id} onClick={() => navigate(`/fornitori?id_provider=${row.id}&tab=Dati`)}>
                    <td>{value(row.company_name)}</td><td>{value(row.state)}</td><td>{value(row.erp_id)}</td>
                  </tr>
                ))}</tbody>
              </table>
            </Panel>
            <Panel title="Documenti in scadenza">
              <table className="table">
                <thead><tr><th>Fornitore</th><th>Documento</th><th>Scadenza</th><th /></tr></thead>
                <tbody>{(documents.data ?? []).map((row) => (
                  <tr key={row.id}>
                    <td>{value(row.company_name)}</td><td>{value(row.document_type)}</td><td>{dateLabel(row.expire_date)}</td>
                    <td><Button size="sm" variant="ghost" leftIcon={<Icon name="download" />} onClick={() => void download(row.id)}>Scarica file</Button></td>
                  </tr>
                ))}</tbody>
              </table>
            </Panel>
            <Panel title="Categorie da gestire">
              <table className="table">
                <thead><tr><th>Fornitore</th><th>Categoria</th><th>Stato</th></tr></thead>
                <tbody>{(categories.data ?? []).map((row) => (
                  <tr key={`${row.provider_id}-${row.category_id}`} onClick={() => navigate(`/fornitori?id_provider=${row.provider_id}&tab=Qualifica`)}>
                    <td>{value(row.company_name)}</td><td>{value(row.category_name)}</td><td>{value(row.state)}</td>
                  </tr>
                ))}</tbody>
              </table>
            </Panel>
          </section>
        </>
      )}
    </main>
  );
}

function Metric({ label, value: count }: { label: string; value: number }) {
  return <div className="metric"><span>{label}</span><strong>{count}</strong></div>;
}

function Panel({ title, children }: { title: string; children: React.ReactNode }) {
  return <section className="panel"><header className="panelHeader"><h2>{title}</h2></header>{children}</section>;
}

export function FornitoriPage() {
  const [params, setParams] = useSearchParams();
  const selectedId = Number(params.get('id_provider') ?? '') || null;
  const tab = providerTabs.includes(params.get('tab') as ProviderTab) ? (params.get('tab') as ProviderTab) : 'Dati';
  const [query, setQuery] = useState('');
  const [showCreate, setShowCreate] = useState(false);
  const providers = useProviders();
  const provider = useProvider(selectedId);
  const filtered = useMemo(() => (providers.data ?? []).filter((item) => (item.company_name ?? '').toLowerCase().includes(query.toLowerCase())), [providers.data, query]);

  function selectProvider(id: number) {
    setParams({ id_provider: String(id), tab });
  }

  function setTab(next: ProviderTab) {
    const nextParams: Record<string, string> = { tab: next };
    if (selectedId) nextParams.id_provider = String(selectedId);
    setParams(nextParams);
  }

  return (
    <main className="page">
      <header className="pageHeader">
        <div><h1>Fornitori</h1><p>Anagrafica, contatti, qualifica e documenti.</p></div>
        <Button leftIcon={<Icon name="plus" />} onClick={() => setShowCreate(true)}>Nuovo fornitore</Button>
      </header>
      <div className="workspace">
        <section className="master panel">
          <div className="toolbar"><SearchInput value={query} onChange={setQuery} placeholder="Cerca fornitore" /></div>
          {providers.isLoading ? <Skeleton rows={8} /> : providers.error ? stateBlock(errorTitle(providers.error), 'Elenco fornitori non disponibile.') : (
            <div className="listRows">
              {filtered.map((item) => (
                <button key={item.id} className={`listRow ${item.id === selectedId ? 'selected' : ''}`} onClick={() => selectProvider(item.id)}>
                  <span><strong>{item.company_name}</strong><small>{stateLabel(item.state)} · {value(item.erp_id)}</small></span>
                  <Icon name="chevron-right" size={16} />
                </button>
              ))}
              {filtered.length === 0 ? stateBlock('Nessun fornitore trovato', 'Modifica la ricerca o crea un nuovo fornitore.') : null}
            </div>
          )}
        </section>
        <section className="detail panel">
          {!selectedId ? stateBlock('Seleziona un fornitore', 'Scegli un fornitore dalla lista per vedere i dettagli.') : provider.isLoading ? <Skeleton rows={8} /> : provider.error ? stateBlock(errorTitle(provider.error), 'Il dettaglio fornitore non puo essere caricato.') : (
            <>
              <header className="detailHeader"><h2>{provider.data?.company_name}</h2><span>{stateLabel(provider.data?.state)}</span></header>
              <div className="segments">{providerTabs.map((item) => <button key={item} className={item === tab ? 'active' : ''} onClick={() => setTab(item)}>{item}</button>)}</div>
              {tab === 'Dati' ? <ProviderData provider={provider.data} /> : null}
              {tab === 'Contatti' ? <ProviderContacts provider={provider.data} /> : null}
              {tab === 'Qualifica' ? <ProviderQualification providerId={selectedId} /> : null}
              {tab === 'Documenti' ? <ProviderDocuments providerId={selectedId} /> : null}
            </>
          )}
        </section>
      </div>
      <ProviderCreateModal open={showCreate} onClose={() => setShowCreate(false)} />
    </main>
  );
}

function ProviderData({ provider }: { provider?: Provider }) {
  const { toast } = useToast();
  const skipRole = useHasRole('app_fornitori_skip_qualification');
  const [confirmDelete, setConfirmDelete] = useState(false);
  const mutations = useFornitoriMutations();
  const ref = qualificationRef(provider);

  async function submit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!provider) return;
    const body = providerPayload(event.currentTarget);
    const validation = validateProvider(body);
    if (validation) {
      toast(validation, 'warning');
      return;
    }
    await mutations.updateProvider.mutateAsync({ id: provider.id, body });
    toast('Aggiornamento completato');
  }

  async function remove() {
    if (!provider) return;
    await mutations.deleteProvider.mutateAsync(provider.id);
    setConfirmDelete(false);
    toast('Aggiornamento completato');
  }

  return (
    <>
      <form className="formGrid" onSubmit={(event) => void submit(event)}>
        <Input name="company_name" label="Ragione sociale" defaultValue={provider?.company_name} />
        <Select name="state" label="Stato" defaultValue={provider?.state ?? 'DRAFT'} options={['DRAFT', 'ACTIVE', 'INACTIVE']} />
        <Input name="vat_number" label="P.IVA" defaultValue={provider?.vat_number} />
        <Input name="cf" label="CF" defaultValue={provider?.cf} />
        <Input name="erp_id" label="Codice Alyante" type="number" defaultValue={provider?.erp_id ?? ''} />
        <Select name="language" label="Lingua" defaultValue={provider?.language ?? 'it'} options={['it', 'en']} />
        <Select name="country" label="Paese" defaultValue={provider?.country ?? 'IT'} options={countries.map((item) => item.code)} />
        <Select name="province" label="Provincia" defaultValue={provider?.province ?? ''} options={['', ...provinces]} />
        <Input name="city" label="Citta" defaultValue={provider?.city} />
        <Input name="postal_code" label="CAP" defaultValue={provider?.postal_code} />
        <Input name="address" label="Indirizzo" defaultValue={provider?.address} wide />
        <Input name="default_payment_method" label="Pagamento predefinito" defaultValue={typeof provider?.default_payment_method === 'object' ? provider.default_payment_method?.code : provider?.default_payment_method ?? ''} />
        <Input name="ref_first_name" label="Nome qualifica" defaultValue={ref?.first_name} />
        <Input name="ref_last_name" label="Cognome qualifica" defaultValue={ref?.last_name} />
        <Input name="ref_email" label="Email qualifica" defaultValue={ref?.email} />
        <Input name="ref_phone" label="Telefono qualifica" defaultValue={ref?.phone} />
        {skipRole ? <label className="checkLine"><input name="skip_qualification_validation" type="checkbox" /> Salta controllo qualifica</label> : null}
        <div className="formActions">
          <Button type="submit" leftIcon={<Icon name="check" />} loading={mutations.updateProvider.isPending}>Salva</Button>
          <Button variant="danger" leftIcon={<Icon name="trash" />} onClick={() => setConfirmDelete(true)}>Elimina</Button>
        </div>
      </form>
      <Modal open={confirmDelete} onClose={() => setConfirmDelete(false)} title="Elimina fornitore" size="sm">
        <p className="modalText">Confermi eliminazione del fornitore selezionato?</p>
        <div className="modalActions">
          <Button variant="secondary" onClick={() => setConfirmDelete(false)}>Annulla</Button>
          <Button variant="danger" onClick={() => void remove()} loading={mutations.deleteProvider.isPending}>Elimina</Button>
        </div>
      </Modal>
    </>
  );
}

function ProviderContacts({ provider }: { provider?: Provider }) {
  const { toast } = useToast();
  const mutations = useFornitoriMutations();
  const refs = getProviderRefs(provider).filter((ref) => ref.reference_type !== 'QUALIFICATION_REF');
  const [newType, setNewType] = useState('ADMINISTRATIVE_REF');

  async function save(ref: ProviderReference, form: HTMLFormElement) {
    if (!provider || !ref.id) return;
    const body = refPayload(form, ref.reference_type);
    await mutations.updateReference.mutateAsync({ providerId: provider.id, refId: ref.id, body });
    toast('Aggiornamento completato');
  }

  async function add(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!provider) return;
    await mutations.createReference.mutateAsync({ providerId: provider.id, body: refPayload(event.currentTarget, newType) });
    event.currentTarget.reset();
    toast('Aggiornamento completato');
  }

  return (
    <div className="stack">
      <table className="table">
        <thead><tr><th>Tipo</th><th>Nome</th><th>Email</th><th>Telefono</th><th /></tr></thead>
        <tbody>{refs.map((ref) => (
          <tr key={ref.id}>
            <td>{referenceTypeLabel(ref.reference_type)}</td>
            <td colSpan={4}>
              <form className="inlineForm" onSubmit={(event) => { event.preventDefault(); void save(ref, event.currentTarget); }}>
                <input name="first_name" defaultValue={ref.first_name} placeholder="Nome" />
                <input name="last_name" defaultValue={ref.last_name} placeholder="Cognome" />
                <input name="email" defaultValue={ref.email} placeholder="Email" />
                <input name="phone" defaultValue={ref.phone} placeholder="Telefono" />
                <Button size="sm" type="submit">Salva</Button>
              </form>
            </td>
          </tr>
        ))}</tbody>
      </table>
      <form className="inlineForm addLine" onSubmit={(event) => void add(event)}>
        <select value={newType} onChange={(event) => setNewType(event.target.value)}>{referenceTypes.map((item) => <option key={item.value} value={item.value}>{item.label}</option>)}</select>
        <input name="first_name" placeholder="Nome" />
        <input name="last_name" placeholder="Cognome" />
        <input name="email" placeholder="Email" />
        <input name="phone" placeholder="Telefono" />
        <Button type="submit" leftIcon={<Icon name="plus" />}>Salva</Button>
      </form>
    </div>
  );
}

function refPayload(form: HTMLFormElement, type?: string): ProviderReference {
  const data = new FormData(form);
  return {
    first_name: String(data.get('first_name') ?? ''),
    last_name: String(data.get('last_name') ?? ''),
    email: String(data.get('email') ?? ''),
    phone: String(data.get('phone') ?? ''),
    reference_type: type,
  };
}

function ProviderQualification({ providerId }: { providerId: number }) {
  const { toast } = useToast();
  const categories = useCategories();
  const providerCategories = useProviderCategories(providerId);
  const [selectedCategory, setSelectedCategory] = useState<number | null>(null);
  const [newCategories, setNewCategories] = useState<number[]>([]);
  const [critical, setCritical] = useState(false);
  const mutations = useFornitoriMutations();
  const documents = useProviderDocuments(providerId, selectedCategory);

  async function add() {
    if (newCategories.length === 0) return;
    await mutations.addProviderCategories.mutateAsync({ providerId, categoryIds: newCategories, critical });
    setNewCategories([]);
    toast('Aggiornamento completato');
  }

  return (
    <div className="twoCols">
      <section>
        <div className="toolbar compact">
          <MultiSelect options={selectOptions(categories.data)} selected={newCategories} onChange={setNewCategories} placeholder="Aggiungi categorie" />
          <ToggleSwitch id="critical-category" checked={critical} onChange={setCritical} label="Critica" />
          <Button onClick={() => void add()} leftIcon={<Icon name="plus" />}>Salva</Button>
        </div>
        <table className="table">
          <thead><tr><th>Categoria</th><th>Stato</th><th>Critica</th></tr></thead>
          <tbody>{(providerCategories.data ?? []).map((row) => (
            <tr key={row.category?.id} className={selectedCategory === row.category?.id ? 'selectedRow' : ''} onClick={() => setSelectedCategory(row.category?.id ?? null)}>
              <td>{row.category?.name}</td><td>{value(row.status ?? row.state)}</td><td>{row.critical ? 'Si' : 'No'}</td>
            </tr>
          ))}</tbody>
        </table>
      </section>
      <Panel title="Documenti categoria">
        {selectedCategory == null ? stateBlock('Seleziona una categoria', 'I documenti verranno mostrati dopo la selezione.') : documents.isLoading ? <Skeleton rows={4} /> : (
          <table className="table">
            <thead><tr><th>Tipo</th><th>Stato</th><th>Scadenza</th></tr></thead>
            <tbody>{(documents.data ?? []).map((doc) => <tr key={doc.id}><td>{doc.document_type?.name}</td><td>{doc.state}</td><td>{dateLabel(doc.expire_date)}</td></tr>)}</tbody>
          </table>
        )}
      </Panel>
    </div>
  );
}

function ProviderDocuments({ providerId }: { providerId: number }) {
  const { toast } = useToast();
  const documents = useProviderDocuments(providerId);
  const types = useDocumentTypes();
  const mutations = useFornitoriMutations();
  const [uploadOpen, setUploadOpen] = useState(false);
  const [editId, setEditId] = useState<number | null>(null);

  async function download(id: number) {
    const blob = await mutations.downloadDocument(id);
    saveBlob(blob, `documento-${id}`);
  }

  return (
    <>
      <div className="toolbar"><Button leftIcon={<Icon name="plus" />} onClick={() => setUploadOpen(true)}>Salva</Button></div>
      <table className="table">
        <thead><tr><th>Tipo</th><th>Stato</th><th>Scadenza</th><th /></tr></thead>
        <tbody>{documents.isLoading ? <tr><td colSpan={4}><Skeleton rows={4} /></td></tr> : (documents.data ?? []).map((doc) => (
          <tr key={doc.id}>
            <td>{doc.document_type?.name}</td><td>{doc.state}</td><td>{dateLabel(doc.expire_date)}</td>
            <td className="rowActions">
              <Button size="sm" variant="ghost" leftIcon={<Icon name="download" />} onClick={() => void download(doc.id)}>Scarica file</Button>
              <Button size="sm" variant="secondary" leftIcon={<Icon name="pencil" />} onClick={() => setEditId(doc.id)}>Salva</Button>
            </td>
          </tr>
        ))}</tbody>
      </table>
      <DocumentModal open={uploadOpen} onClose={() => setUploadOpen(false)} providerId={providerId} documentTypes={types.data ?? []} onSaved={() => toast('Aggiornamento completato')} />
      <DocumentModal open={editId != null} onClose={() => setEditId(null)} providerId={providerId} documentId={editId ?? undefined} documentTypes={types.data ?? []} onSaved={() => toast('Aggiornamento completato')} />
    </>
  );
}

function DocumentModal({ open, onClose, providerId, documentId, documentTypes, onSaved }: { open: boolean; onClose: () => void; providerId: number; documentId?: number; documentTypes: DocumentType[]; onSaved: () => void }) {
  const { toast } = useToast();
  const mutations = useFornitoriMutations();

  async function submit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const data = new FormData(event.currentTarget);
    const file = data.get('file');
    if (!(file instanceof File) || file.size === 0) {
      toast('Seleziona un file da caricare', 'warning');
      return;
    }
    data.set('provider_id', String(providerId));
    if (documentId) await mutations.updateDocument.mutateAsync({ id: documentId, body: data });
    else await mutations.uploadDocument.mutateAsync(data);
    onClose();
    onSaved();
  }

  return (
    <Modal open={open} onClose={onClose} title={documentId ? 'Salva documento' : 'Salva documento'} size="md">
      <form className="modalForm" onSubmit={(event) => void submit(event)}>
        {!documentId ? <Select name="document_type_id" label="Tipo documento" options={documentTypes.map((item) => String(item.id))} /> : null}
        <Input name="expire_date" label="Scadenza" type="date" />
        <label className="field"><span>File</span><input name="file" type="file" /></label>
        <div className="modalActions"><Button variant="secondary" onClick={onClose}>Annulla</Button><Button type="submit">Salva</Button></div>
      </form>
    </Modal>
  );
}

function ProviderCreateModal({ open, onClose }: { open: boolean; onClose: () => void }) {
  const { toast } = useToast();
  const categories = useCategories();
  const mutations = useFornitoriMutations();
  const [selectedCategories, setSelectedCategories] = useState<number[]>([]);
  const [critical, setCritical] = useState(false);

  async function submit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const body = providerPayload(event.currentTarget);
    const validation = validateProvider(body);
    if (validation) {
      toast(validation, 'warning');
      return;
    }
    const created = await mutations.createProvider.mutateAsync(body);
    if (selectedCategories.length > 0) {
      await mutations.addProviderCategories.mutateAsync({ providerId: created.id, categoryIds: selectedCategories, critical });
    }
    onClose();
    setSelectedCategories([]);
    toast('Aggiornamento completato');
  }

  return (
    <Modal open={open} onClose={onClose} title="Nuovo fornitore" size="wide">
      <form className="formGrid" onSubmit={(event) => void submit(event)}>
        <Input name="company_name" label="Ragione sociale" />
        <Select name="state" label="Stato" defaultValue="DRAFT" options={['DRAFT', 'ACTIVE', 'INACTIVE']} />
        <Input name="vat_number" label="P.IVA" />
        <Input name="cf" label="CF" />
        <Input name="erp_id" label="Codice Alyante" type="number" />
        <Select name="language" label="Lingua" defaultValue="it" options={['it', 'en']} />
        <Select name="country" label="Paese" defaultValue="IT" options={countries.map((item) => item.code)} />
        <Select name="province" label="Provincia" options={['', ...provinces]} />
        <Input name="city" label="Citta" />
        <Input name="postal_code" label="CAP" />
        <Input name="address" label="Indirizzo" wide />
        <Input name="default_payment_method" label="Pagamento predefinito" />
        <Input name="ref_first_name" label="Nome qualifica" />
        <Input name="ref_last_name" label="Cognome qualifica" />
        <Input name="ref_email" label="Email qualifica" />
        <Input name="ref_phone" label="Telefono qualifica" />
        <div className="wideField">
          <span className="fieldLabel">Categorie</span>
          <MultiSelect options={selectOptions(categories.data)} selected={selectedCategories} onChange={setSelectedCategories} placeholder="Seleziona categorie" />
        </div>
        <ToggleSwitch id="new-critical" checked={critical} onChange={setCritical} label="Categoria critica" />
        <div className="formActions"><Button variant="secondary" onClick={onClose}>Annulla</Button><Button type="submit">Salva</Button></div>
      </form>
    </Modal>
  );
}

export function QualificationSettingsPage() {
  const readonly = useHasRole('app_fornitori_readonly');
  const { toast } = useToast();
  const categories = useCategories();
  const documentTypes = useDocumentTypes();
  const mutations = useFornitoriMutations();
  const [required, setRequired] = useState<number[]>([]);
  const [optional, setOptional] = useState<number[]>([]);
  const [selectedCategory, setSelectedCategory] = useState<Category | null>(null);
  const [selectedDocType, setSelectedDocType] = useState<DocumentType | null>(null);

  async function saveCategory(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const overlap = required.find((id) => optional.includes(id));
    if (overlap) {
      toast('Lo stesso tipo documento non puo essere obbligatorio e facoltativo', 'warning');
      return;
    }
    const name = String(new FormData(event.currentTarget).get('name') ?? '').trim();
    const body = { name, required_document_type_ids: required, optional_document_type_ids: optional };
    if (selectedCategory) await mutations.updateCategory.mutateAsync({ id: selectedCategory.id, body });
    else await mutations.createCategory.mutateAsync(body);
    setSelectedCategory(null);
    setRequired([]);
    setOptional([]);
    toast('Aggiornamento completato');
  }

  async function saveDocType(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const name = String(new FormData(event.currentTarget).get('name') ?? '').trim();
    if (selectedDocType) await mutations.updateDocumentType.mutateAsync({ id: selectedDocType.id, name });
    else await mutations.createDocumentType.mutateAsync({ name });
    setSelectedDocType(null);
    toast('Aggiornamento completato');
  }

  return (
    <main className="page">
      <header className="pageHeader"><div><h1>Impostazioni qualifica</h1><p>Categorie e tipi documento.</p></div></header>
      <div className="twoCols">
        <Panel title="Categorie">
          <table className="table"><tbody>{(categories.data ?? []).map((item) => <tr key={item.id} onClick={() => setSelectedCategory(item)}><td>{item.name}</td><td className="rightText"><Icon name="chevron-right" size={16} /></td></tr>)}</tbody></table>
          <form className="modalForm" onSubmit={(event) => void saveCategory(event)}>
            <Input name="name" label="Categoria" defaultValue={selectedCategory?.name ?? ''} />
            <span className="fieldLabel">Doc obbligatori</span><MultiSelect options={selectOptions(documentTypes.data)} selected={required} onChange={setRequired} />
            <span className="fieldLabel">Doc facoltativi</span><MultiSelect options={selectOptions(documentTypes.data)} selected={optional} onChange={setOptional} />
            <div className="formActions">
              <Button type="submit" disabled={readonly}>Salva</Button>
              {selectedCategory ? <Button variant="danger" disabled={readonly} onClick={() => void mutations.deleteCategory.mutateAsync(selectedCategory.id)}>Elimina</Button> : null}
            </div>
          </form>
        </Panel>
        <Panel title="Tipi documento">
          <table className="table"><tbody>{(documentTypes.data ?? []).map((item) => <tr key={item.id} onClick={() => setSelectedDocType(item)}><td>{item.name}</td><td className="rightText"><Icon name="chevron-right" size={16} /></td></tr>)}</tbody></table>
          <form className="modalForm" onSubmit={(event) => void saveDocType(event)}>
            <Input name="name" label="Tipo documento" defaultValue={selectedDocType?.name ?? ''} />
            <div className="formActions">
              <Button type="submit" disabled={readonly}>Salva</Button>
              {selectedDocType ? <Button variant="danger" disabled={readonly} onClick={() => void mutations.deleteDocumentType.mutateAsync(selectedDocType.id)}>Elimina</Button> : null}
            </div>
          </form>
        </Panel>
      </div>
    </main>
  );
}

export function PaymentMethodsPage() {
  const readonly = useHasRole('app_fornitori_readonly');
  const { toast } = useToast();
  const methods = usePaymentMethods();
  const mutations = useFornitoriMutations();

  async function toggle(code: string, checked: boolean) {
    await mutations.setPaymentRda.mutateAsync({ code, rda_available: checked });
    toast('Aggiornamento completato');
  }

  return (
    <main className="page">
      <header className="pageHeader"><div><h1>Pagamenti RDA</h1><p>Disponibilita dei metodi di pagamento per RDA.</p></div></header>
      <section className="panel">
        {methods.isLoading ? <Skeleton rows={8} /> : methods.error ? stateBlock(errorTitle(methods.error), 'I metodi di pagamento non possono essere caricati.') : (
          <table className="table">
            <thead><tr><th>Codice</th><th>Descrizione</th><th>Disponibile RDA</th></tr></thead>
            <tbody>{(methods.data ?? []).map((item) => (
              <tr key={item.code}><td>{item.code}</td><td>{item.description}</td><td><ToggleSwitch id={`rda-${item.code}`} checked={Boolean(item.rda_available)} disabled={readonly} onChange={(checked) => void toggle(item.code, checked)} /></td></tr>
            ))}</tbody>
          </table>
        )}
      </section>
    </main>
  );
}

export function ArticleCategoriesPage() {
  const readonly = useHasRole('app_fornitori_readonly');
  const { toast } = useToast();
  const articles = useArticleCategories();
  const categories = useCategories();
  const mutations = useFornitoriMutations();
  const [selected, setSelected] = useState<string | null>(null);
  const selectedItem = articles.data?.find((item) => item.article_code === selected);

  async function submit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!selectedItem) return;
    const categoryId = Number(new FormData(event.currentTarget).get('category_id'));
    await mutations.setArticleCategory.mutateAsync({ articleCode: selectedItem.article_code, categoryId });
    toast('Aggiornamento completato');
  }

  return (
    <main className="page">
      <header className="pageHeader"><div><h1>Articoli-categorie</h1><p>Associazione tra articoli e categorie di qualifica.</p></div></header>
      <div className="workspace">
        <section className="master panel">
          {articles.isLoading ? <Skeleton rows={8} /> : articles.error ? stateBlock(errorTitle(articles.error), 'Gli articoli non possono essere caricati.') : (
            <table className="table">
              <thead><tr><th>Articolo</th><th>Categoria</th></tr></thead>
              <tbody>{(articles.data ?? []).map((item) => <tr key={item.article_code} className={selected === item.article_code ? 'selectedRow' : ''} onClick={() => setSelected(item.article_code)}><td>{item.article_code}<small>{item.description}</small></td><td>{item.category_name}</td></tr>)}</tbody>
            </table>
          )}
        </section>
        <section className="detail panel">
          {!selectedItem ? stateBlock('Seleziona un articolo', 'La categoria potra essere modificata dopo la selezione.') : (
            <form className="modalForm" onSubmit={(event) => void submit(event)}>
              <h2>{selectedItem.article_code}</h2>
              <p className="muted">{selectedItem.description}</p>
              <label className="field"><span>Categoria</span><select name="category_id" defaultValue={selectedItem.category_id}>{(categories.data ?? []).map((item) => <option key={item.id} value={item.id}>{item.name}</option>)}</select></label>
              <Button type="submit" disabled={readonly}>Salva</Button>
            </form>
          )}
        </section>
      </div>
    </main>
  );
}

function Input({ label, wide, ...props }: React.InputHTMLAttributes<HTMLInputElement> & { label: string; wide?: boolean }) {
  return <label className={`field ${wide ? 'wideField' : ''}`}><span>{label}</span><input {...props} /></label>;
}

function Select({ label, options, ...props }: React.SelectHTMLAttributes<HTMLSelectElement> & { label: string; options: string[] }) {
  return <label className="field"><span>{label}</span><select {...props}>{options.map((item) => <option key={item} value={item}>{item || '-'}</option>)}</select></label>;
}
