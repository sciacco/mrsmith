import { Button, Icon, Modal, SearchInput, Skeleton, useToast } from '@mrsmith/ui';
import { useEffect, useMemo, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { useConfigList, useConfigMutations, useReferenceData } from '../api/queries';
import type { ReferenceItem } from '../api/types';
import { ConfirmDialog } from '../components/ConfirmDialog';
import { StatusPill } from '../components/StatusPill';
import { errorMessage } from '../lib/format';
import shared from './shared.module.css';

const titles: Record<string, string> = {
  sites: 'Siti',
  'technical-domains': 'Domini tecnici',
  'maintenance-kinds': 'Tipi manutenzione',
  'customer-scopes': 'Ambiti clienti',
  'service-taxonomy': 'Servizi',
  'reason-classes': 'Motivi',
  'impact-effects': 'Effetti impatto',
  'quality-flags': 'Segnali qualità',
  'target-types': 'Tipi target',
  'notice-channels': 'Canali comunicazione',
};

export function ConfigurationResourcePage() {
  const navigate = useNavigate();
  const params = useParams();
  const resource = params.resource ?? '';
  const title = titles[resource];
  const [active, setActive] = useState('active');
  const [q, setQ] = useState('');
  const [editing, setEditing] = useState<ReferenceItem | null>(null);
  const [modalOpen, setModalOpen] = useState(false);
  const [confirm, setConfirm] = useState<ReferenceItem | null>(null);
  const list = useConfigList(resource, active, q);
  const reference = useReferenceData();
  const mutations = useConfigMutations(resource);
  const toast = useToast();
  const unknown = !title;

  const confirmActive = useMemo(() => {
    if (!confirm) return false;
    return confirm.is_active;
  }, [confirm]);

  if (unknown) {
    return (
      <section className={shared.emptyCard}>
        <div className={shared.emptyIconDanger}>
          <Icon name="triangle-alert" />
        </div>
        <h3>Configurazione non trovata</h3>
        <p>La pagina richiesta non è disponibile.</p>
      </section>
    );
  }

  async function toggleActive() {
    if (!confirm) return;
    try {
      if (confirm.is_active) await mutations.deactivate.mutateAsync(confirm.id);
      else await mutations.reactivate.mutateAsync(confirm.id);
      toast.toast(confirm.is_active ? 'Valore disattivato.' : 'Valore riattivato.');
      setConfirm(null);
    } catch (error) {
      toast.toast(errorMessage(error, 'Aggiornamento non riuscito.'), 'error');
    }
  }

  return (
    <section className={shared.page}>
      <button type="button" className={shared.backLink} onClick={() => navigate('/manutenzioni/configurazione')}>
        <Icon name="chevron-left" size={16} />
        Torna alla configurazione
      </button>
      <div className={shared.header}>
        <div className={shared.titleBlock}>
          <h1 className={shared.pageTitle}>{title}</h1>
          <p className={shared.pageSubtitle}>Crea, modifica, disattiva e riattiva i valori della risorsa.</p>
        </div>
        <div className={shared.headerActions}>
          <Button
            onClick={() => {
              setEditing(null);
              setModalOpen(true);
            }}
            leftIcon={<Icon name="plus" size={16} />}
          >
            Nuovo valore
          </Button>
        </div>
      </div>

      <div className={shared.filterBar} style={{ gridTemplateColumns: 'minmax(240px, 1fr) auto auto' }}>
        <SearchInput value={q} onChange={setQ} placeholder="Cerca per codice, nome o descrizione..." />
        <div className={shared.segmented}>
          {([
            ['active', 'Attivi'],
            ['inactive', 'Non attivi'],
            ['all', 'Tutti'],
          ] as Array<[string, string]>).map(([value, label]) => (
            <button
              key={value}
              type="button"
              className={`${shared.segment} ${active === value ? shared.segmentActive : ''}`}
              onClick={() => setActive(value)}
            >
              {label}
            </button>
          ))}
        </div>
        <Button variant="secondary" onClick={() => list.refetch()} loading={list.isFetching && !list.isLoading}>
          Aggiorna
        </Button>
      </div>

      {list.isLoading ? (
        <div className={shared.panel}>
          <Skeleton rows={6} />
        </div>
      ) : list.error ? (
        <div className={shared.emptyCard}>
          <div className={shared.emptyIconDanger}>
            <Icon name="triangle-alert" />
          </div>
          <h3>Elenco non disponibile</h3>
          <p>{errorMessage(list.error, 'Impossibile caricare i valori.')}</p>
        </div>
      ) : !list.data || list.data.length === 0 ? (
        <div className={shared.emptyCard}>
          <div className={shared.emptyIcon}>
            <Icon name="list" />
          </div>
          <h3>Nessun valore</h3>
          <p>Non ci sono valori da mostrare.</p>
        </div>
      ) : (
        <div className={shared.tableCard}>
          <div className={shared.tableScroll}>
            <table className={shared.table}>
              <thead>
                <tr>
                  <th>Codice</th>
                  <th>Nome</th>
                  {resource === 'service-taxonomy' && <th>Dominio</th>}
                  {resource === 'sites' && <th>Città</th>}
                  <th>Stato</th>
                  <th className={shared.actionsCell}>Azioni</th>
                </tr>
              </thead>
              <tbody>
                {list.data.map((item) => (
                  <tr key={item.id}>
                    <td className={shared.mono}>{item.code}</td>
                    <td>
                      <div className={shared.rowTitle}>
                        <strong>{item.name_it}</strong>
                        {item.description && <span className={shared.small}>{item.description}</span>}
                      </div>
                    </td>
                    {resource === 'service-taxonomy' && <td>{item.technical_domain_name ?? '-'}</td>}
                    {resource === 'sites' && <td>{item.city ?? '-'}</td>}
                    <td>
                      <StatusPill tone={item.is_active ? 'success' : 'neutral'}>
                        {item.is_active ? 'Attivo' : 'Non attivo'}
                      </StatusPill>
                    </td>
                    <td className={shared.actionsCell}>
                      <div className={shared.inlineActions}>
                        <Button
                          size="sm"
                          variant="secondary"
                          onClick={() => {
                            setEditing(item);
                            setModalOpen(true);
                          }}
                        >
                          Modifica
                        </Button>
                        <Button
                          size="sm"
                          variant={item.is_active ? 'danger' : 'secondary'}
                          onClick={() => setConfirm(item)}
                        >
                          {item.is_active ? 'Disattiva' : 'Riattiva'}
                        </Button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      <ConfigModal
        open={modalOpen}
        resource={resource}
        item={editing}
        technicalDomains={reference.data?.technical_domains ?? []}
        onClose={() => setModalOpen(false)}
        onSaved={() => {
          setModalOpen(false);
          setEditing(null);
        }}
      />
      <ConfirmDialog
        open={confirm !== null}
        title={confirmActive ? 'Disattiva valore' : 'Riattiva valore'}
        message={confirmActive ? 'Il valore non sarà più proposto nei nuovi inserimenti.' : 'Il valore tornerà disponibile.'}
        confirmLabel={confirmActive ? 'Disattiva' : 'Riattiva'}
        busy={mutations.deactivate.isPending || mutations.reactivate.isPending}
        onClose={() => setConfirm(null)}
        onConfirm={toggleActive}
      />
    </section>
  );
}

function ConfigModal({
  open,
  resource,
  item,
  technicalDomains,
  onClose,
  onSaved,
}: {
  open: boolean;
  resource: string;
  item: ReferenceItem | null;
  technicalDomains: ReferenceItem[];
  onClose: () => void;
  onSaved: () => void;
}) {
  const mutations = useConfigMutations(resource);
  const toast = useToast();
  const [form, setForm] = useState(() => formFromItem(item));

  useEffect(() => {
    if (open) setForm(formFromItem(item));
  }, [item, open]);

  async function save() {
    if (!form.name_it.trim() || (!item && !form.code.trim())) {
      toast.toast('Completa codice e nome.', 'error');
      return;
    }
    const body = {
      code: form.code.trim(),
      name_it: form.name_it.trim(),
      name_en: form.name_en.trim() || null,
      description: form.description.trim() || null,
      city: form.city.trim() || null,
      country_code: form.country_code.trim() || null,
      sort_order: form.sort_order ? Number(form.sort_order) : 100,
      technical_domain_id: form.technical_domain_id ? Number(form.technical_domain_id) : undefined,
      is_active: true,
    };
    try {
      if (item) await mutations.update.mutateAsync({ id: item.id, body });
      else await mutations.create.mutateAsync(body);
      toast.toast('Valore salvato.');
      onSaved();
    } catch (error) {
      toast.toast(errorMessage(error, 'Salvataggio non riuscito.'), 'error');
    }
  }

  return (
    <Modal open={open} onClose={onClose} title={item ? 'Modifica valore' : 'Nuovo valore'} size="lg">
      <div className={shared.formGrid}>
        <label className={shared.label}>
          Codice
          <input
            className={shared.field}
            value={form.code}
            disabled={Boolean(item)}
            onChange={(event) => setForm({ ...form, code: event.target.value })}
          />
        </label>
        <label className={shared.label}>
          Nome
          <input
            className={shared.field}
            value={form.name_it}
            onChange={(event) => setForm({ ...form, name_it: event.target.value })}
          />
        </label>
        <label className={shared.label}>
          Nome inglese
          <input
            className={shared.field}
            value={form.name_en}
            onChange={(event) => setForm({ ...form, name_en: event.target.value })}
          />
        </label>
        {resource === 'service-taxonomy' && (
          <label className={shared.label}>
            Dominio
            <select
              className={shared.select}
              value={form.technical_domain_id}
              onChange={(event) => setForm({ ...form, technical_domain_id: event.target.value })}
            >
              <option value="">Seleziona dominio</option>
              {technicalDomains.map((domain) => (
                <option key={domain.id} value={domain.id}>
                  {domain.name_it}
                </option>
              ))}
            </select>
          </label>
        )}
        {resource === 'sites' && (
          <>
            <label className={shared.label}>
              Città
              <input
                className={shared.field}
                value={form.city}
                onChange={(event) => setForm({ ...form, city: event.target.value })}
              />
            </label>
            <label className={shared.label}>
              Paese
              <input
                className={shared.field}
                value={form.country_code}
                onChange={(event) => setForm({ ...form, country_code: event.target.value })}
              />
            </label>
          </>
        )}
        {resource !== 'sites' && (
          <label className={shared.label}>
            Ordinamento
            <input
              className={shared.field}
              type="number"
              value={form.sort_order}
              onChange={(event) => setForm({ ...form, sort_order: event.target.value })}
            />
          </label>
        )}
        <label className={shared.label}>
          Descrizione
          <textarea
            className={shared.textarea}
            value={form.description}
            onChange={(event) => setForm({ ...form, description: event.target.value })}
          />
        </label>
      </div>
      <div className={shared.formActions} style={{ marginTop: '1rem' }}>
        <Button variant="secondary" onClick={onClose}>
          Annulla
        </Button>
        <Button onClick={save} loading={mutations.create.isPending || mutations.update.isPending}>
          Salva
        </Button>
      </div>
    </Modal>
  );
}

function formFromItem(item: ReferenceItem | null) {
  return {
    code: item?.code ?? '',
    name_it: item?.name_it ?? '',
    name_en: item?.name_en ?? '',
    description: item?.description ?? '',
    city: item?.city ?? '',
    country_code: item?.country_code ?? '',
    sort_order: item ? String(item.sort_order) : '100',
    technical_domain_id: item?.technical_domain_id ? String(item.technical_domain_id) : '',
  };
}
