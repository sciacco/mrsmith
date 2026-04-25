import { Button, Icon, SearchInput, Skeleton, useToast } from '@mrsmith/ui';
import { useDeferredValue, useMemo, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  useReferenceData,
  useServiceDependencies,
  useServiceDependencyMutations,
} from '../api/queries';
import type { ReferenceItem, ServiceDependency, ServiceDependencyBody } from '../api/types';
import { ConfirmDialog } from '../components/ConfirmDialog';
import { dependencyTypeLabel, errorMessage, severityLabel } from '../lib/format';
import { SEVERITY_OPTIONS } from '../lib/severity';
import styles from './ConfigurationDependenciesPage.module.css';
import shared from './shared.module.css';

type ActiveFilter = 'active' | 'inactive' | 'all';

const defaultForm: ServiceDependencyBody = {
  upstream_service_id: 0,
  downstream_service_id: 0,
  dependency_type: 'depends_on',
  is_redundant: false,
  default_severity: 'unavailable',
};

export function ConfigurationDependenciesPage() {
  const navigate = useNavigate();
  const [active, setActive] = useState<ActiveFilter>('active');
  const [q, setQ] = useState('');
  const deferredQ = useDeferredValue(q);
  const dependencies = useServiceDependencies(active, deferredQ);
  const reference = useReferenceData();
  const mutations = useServiceDependencyMutations();
  const toast = useToast();
  const [selectedUpstreamId, setSelectedUpstreamId] = useState<number | null>(null);
  const [editing, setEditing] = useState<ServiceDependency | null>(null);
  const [form, setForm] = useState<ServiceDependencyBody>(defaultForm);
  const [confirm, setConfirm] = useState<ServiceDependency | null>(null);

  const rows = dependencies.data ?? [];
  const upstreams = useMemo(() => groupByUpstream(rows), [rows]);
  const selectedUpstream =
    upstreams.find((item) => item.service.id === selectedUpstreamId) ?? upstreams[0] ?? null;
  const selectedRows = selectedUpstream
    ? rows.filter((item) => item.upstream_service_id === selectedUpstream.service.id)
    : [];

  function startCreate(upstream?: ReferenceItem) {
    setEditing(null);
    const defaultServiceId =
      upstream?.id ?? selectedUpstream?.service.id ?? reference.data?.service_taxonomy[0]?.id ?? 0;
    setForm({
      ...defaultForm,
      upstream_service_id: defaultServiceId,
    });
  }

  function startEdit(item: ServiceDependency) {
    setEditing(item);
    setForm({
      upstream_service_id: item.upstream_service_id,
      downstream_service_id: item.downstream_service_id,
      dependency_type: item.dependency_type,
      is_redundant: item.is_redundant,
      default_severity: item.default_severity,
      metadata: item.metadata ?? null,
    });
  }

  async function save() {
    if (!form.upstream_service_id || !form.downstream_service_id) {
      toast.toast('Seleziona servizio di partenza e servizio impattato.', 'error');
      return;
    }
    if (form.upstream_service_id === form.downstream_service_id) {
      toast.toast('Scegli due servizi diversi.', 'error');
      return;
    }
    try {
      if (editing) await mutations.update.mutateAsync({ id: editing.service_dependency_id, body: form });
      else await mutations.create.mutateAsync(form);
      toast.toast('Dipendenza salvata.');
      setEditing(null);
      setForm(defaultForm);
    } catch (error) {
      toast.toast(errorMessage(error, 'Salvataggio non riuscito.'), 'error');
    }
  }

  async function toggleActive() {
    if (!confirm) return;
    try {
      if (confirm.is_active) await mutations.deactivate.mutateAsync(confirm.service_dependency_id);
      else await mutations.reactivate.mutateAsync(confirm.service_dependency_id);
      toast.toast(confirm.is_active ? 'Dipendenza disattivata.' : 'Dipendenza riattivata.');
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
          <h1 className={shared.pageTitle}>Grafo dipendenze</h1>
          <p className={shared.pageSubtitle}>
            Relazioni tra servizi catalogo usate per preparare gli impatti delle manutenzioni.
          </p>
        </div>
        <div className={shared.headerActions}>
          <Button variant="secondary" onClick={() => dependencies.refetch()} leftIcon={<Icon name="loader" size={16} />}>
            Aggiorna
          </Button>
          <Button onClick={() => startCreate()} leftIcon={<Icon name="plus" size={16} />}>
            Nuova dipendenza
          </Button>
        </div>
      </div>

      <div className={styles.filterBar}>
        <SearchInput value={q} onChange={setQ} placeholder="Cerca per servizio o tipo relazione..." />
        <div className={shared.segmented}>
          {([
            ['active', 'Attive'],
            ['inactive', 'Non attive'],
            ['all', 'Tutte'],
          ] as Array<[ActiveFilter, string]>).map(([value, label]) => (
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
      </div>

      {dependencies.isLoading || reference.isLoading ? (
        <div className={shared.panel}>
          <Skeleton rows={8} />
        </div>
      ) : dependencies.error || reference.error || !reference.data ? (
        <div className={shared.emptyCard}>
          <div className={shared.emptyIconDanger}>
            <Icon name="triangle-alert" />
          </div>
          <h3>Dipendenze non disponibili</h3>
          <p>{errorMessage(dependencies.error ?? reference.error, 'Impossibile caricare le relazioni.')}</p>
        </div>
      ) : rows.length === 0 && form.upstream_service_id === 0 ? (
        <div className={shared.emptyCard}>
          <div className={shared.emptyIcon}>
            <Icon name="link" />
          </div>
          <h3>Nessuna dipendenza trovata</h3>
          <p>Aggiungi una relazione tra servizi per usare i suggerimenti nelle nuove manutenzioni.</p>
          <div style={{ marginTop: '1rem' }}>
            <Button onClick={() => startCreate()} leftIcon={<Icon name="plus" size={16} />}>
              Nuova dipendenza
            </Button>
          </div>
        </div>
      ) : (
        <div className={styles.workspace}>
          <aside className={styles.master}>
            <div className={styles.masterHeader}>
              <div>
                <h2 className={shared.sectionTitle}>Servizi di partenza</h2>
                <p className={shared.small}>{rows.length} relazioni visibili</p>
              </div>
            </div>
            <div className={styles.masterList}>
              {upstreams.map((item) => (
                <button
                  key={item.service.id}
                  type="button"
                  className={`${styles.masterRow} ${
                    selectedUpstream?.service.id === item.service.id ? styles.masterRowActive : ''
                  }`}
                  onClick={() => setSelectedUpstreamId(item.service.id)}
                >
                  <span className={styles.rowTitle}>
                    <strong>{item.service.name_it}</strong>
                    <span className={shared.small}>{item.service.technical_domain_name ?? item.service.code}</span>
                  </span>
                  <span className={styles.count}>{item.count}</span>
                </button>
              ))}
            </div>
          </aside>

          <section className={styles.detail}>
            <div className={styles.detailHeader}>
              <div>
                <h2 className={shared.sectionTitle}>
                  {selectedUpstream?.service.name_it ?? 'Seleziona un servizio'}
                </h2>
                <p className={shared.small}>{selectedUpstream?.service.technical_domain_name ?? ''}</p>
              </div>
              {selectedUpstream ? (
                <Button size="sm" variant="secondary" onClick={() => startCreate(selectedUpstream.service)}>
                  Aggiungi impatto
                </Button>
              ) : null}
            </div>
            <div className={styles.detailBody}>
              {(editing || form.upstream_service_id > 0) && (
                <DependencyForm
                  form={form}
                  services={reference.data.service_taxonomy}
                  busy={mutations.create.isPending || mutations.update.isPending}
                  editing={Boolean(editing)}
                  onChange={setForm}
                  onSave={save}
                  onCancel={() => {
                    setEditing(null);
                    setForm(defaultForm);
                  }}
                />
              )}
              <div className={styles.dependencyList}>
                {selectedRows.map((item) => (
                  <div key={item.service_dependency_id} className={styles.dependencyRow}>
                    <div className={styles.rowTitle}>
                      <strong>{item.downstream_service.name_it}</strong>
                      <div className={styles.metaLine}>
                        <span>{dependencyTypeLabel(item.dependency_type)}</span>
                        <span>{severityLabel(item.default_severity)}</span>
                        <span>{item.is_redundant ? 'Ridondato' : 'Non ridondato'}</span>
                        <span>{item.is_active ? 'Attiva' : 'Non attiva'}</span>
                      </div>
                    </div>
                    <div className={shared.inlineActions}>
                      <Button size="sm" variant="secondary" onClick={() => startEdit(item)}>
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
                  </div>
                ))}
              </div>
            </div>
          </section>
        </div>
      )}

      <ConfirmDialog
        open={confirm !== null}
        title={confirm?.is_active ? 'Disattiva dipendenza' : 'Riattiva dipendenza'}
        message={
          confirm?.is_active
            ? 'La relazione non sarà più proposta nelle nuove manutenzioni.'
            : 'La relazione tornerà disponibile nei suggerimenti.'
        }
        confirmLabel={confirm?.is_active ? 'Disattiva' : 'Riattiva'}
        variant={confirm?.is_active ? 'danger' : 'primary'}
        busy={mutations.deactivate.isPending || mutations.reactivate.isPending}
        onClose={() => setConfirm(null)}
        onConfirm={toggleActive}
      />
    </section>
  );
}

function DependencyForm({
  form,
  services,
  editing,
  busy,
  onChange,
  onSave,
  onCancel,
}: {
  form: ServiceDependencyBody;
  services: ReferenceItem[];
  editing: boolean;
  busy: boolean;
  onChange: (value: ServiceDependencyBody) => void;
  onSave: () => void;
  onCancel: () => void;
}) {
  return (
    <div className={styles.formPanel}>
      <div className={shared.formGrid}>
        <ServiceSelect
          label="Servizio di partenza"
          value={form.upstream_service_id}
          services={services}
          onChange={(id) => onChange({ ...form, upstream_service_id: id })}
        />
        <ServiceSelect
          label="Servizio impattato"
          value={form.downstream_service_id}
          services={services}
          onChange={(id) => onChange({ ...form, downstream_service_id: id })}
        />
        <label className={shared.label}>
          Tipo relazione
          <select
            className={shared.select}
            value={form.dependency_type}
            onChange={(event) =>
              onChange({ ...form, dependency_type: event.target.value as ServiceDependencyBody['dependency_type'] })
            }
          >
            <option value="runs_on">Ospita</option>
            <option value="connects_through">Transita da</option>
            <option value="consumes">Consuma</option>
            <option value="depends_on">Dipende da</option>
          </select>
        </label>
        <label className={shared.label}>
          Severità attesa
          <select
            className={shared.select}
            value={form.default_severity}
            onChange={(event) =>
              onChange({ ...form, default_severity: event.target.value as ServiceDependencyBody['default_severity'] })
            }
          >
            {SEVERITY_OPTIONS.map((option) => (
              <option key={option.value} value={option.value}>
                {option.label}
              </option>
            ))}
          </select>
        </label>
        <label className={styles.inlineCheck}>
          <input
            type="checkbox"
            checked={form.is_redundant}
            onChange={(event) => onChange({ ...form, is_redundant: event.target.checked })}
          />
          Ridondato
        </label>
      </div>
      <div className={shared.formActions}>
        <Button variant="secondary" onClick={onCancel}>
          Annulla
        </Button>
        <Button onClick={onSave} loading={busy}>
          {editing ? 'Salva modifiche' : 'Aggiungi'}
        </Button>
      </div>
    </div>
  );
}

function ServiceSelect({
  label,
  value,
  services,
  onChange,
}: {
  label: string;
  value: number;
  services: ReferenceItem[];
  onChange: (id: number) => void;
}) {
  return (
    <label className={shared.label}>
      {label}
      <select className={shared.select} value={value || ''} onChange={(event) => onChange(Number(event.target.value))}>
        <option value="">Seleziona servizio</option>
        {services.map((service) => (
          <option key={service.id} value={service.id}>
            {service.name_it}
          </option>
        ))}
      </select>
    </label>
  );
}

function groupByUpstream(items: ServiceDependency[]): Array<{ service: ReferenceItem; count: number }> {
  const groups = new Map<number, { service: ReferenceItem; count: number }>();
  for (const item of items) {
    const current = groups.get(item.upstream_service_id);
    if (current) current.count += 1;
    else groups.set(item.upstream_service_id, { service: item.upstream_service, count: 1 });
  }
  return [...groups.values()].sort((a, b) => a.service.name_it.localeCompare(b.service.name_it));
}
