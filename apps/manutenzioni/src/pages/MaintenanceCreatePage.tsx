import { Button, Icon, Skeleton, useToast } from '@mrsmith/ui';
import { useEffect, useMemo, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useCreateMaintenance, useReferenceData } from '../api/queries';
import type { MaintenanceFormBody, WindowBody } from '../api/types';
import { errorMessage } from '../lib/format';
import shared from './shared.module.css';

interface FormState {
  title_it: string;
  title_en: string;
  description_it: string;
  description_en: string;
  maintenance_kind_id: string;
  technical_domain_id: string;
  customer_scope_id: string;
  site_id: string;
  reason_it: string;
  residual_service_it: string;
  scheduled_start_at: string;
  scheduled_end_at: string;
  expected_downtime_minutes: string;
}

const initialForm: FormState = {
  title_it: '',
  title_en: '',
  description_it: '',
  description_en: '',
  maintenance_kind_id: '',
  technical_domain_id: '',
  customer_scope_id: '',
  site_id: '',
  reason_it: '',
  residual_service_it: '',
  scheduled_start_at: '',
  scheduled_end_at: '',
  expected_downtime_minutes: '',
};

export function MaintenanceCreatePage() {
  const navigate = useNavigate();
  const { toast } = useToast();
  const reference = useReferenceData();
  const create = useCreateMaintenance();
  const [form, setForm] = useState<FormState>(initialForm);
  const dirty = useMemo(() => JSON.stringify(form) !== JSON.stringify(initialForm), [form]);

  useEffect(() => {
    const onBeforeUnload = (event: BeforeUnloadEvent) => {
      if (!dirty) return;
      event.preventDefault();
      event.returnValue = '';
    };
    window.addEventListener('beforeunload', onBeforeUnload);
    return () => window.removeEventListener('beforeunload', onBeforeUnload);
  }, [dirty]);

  function update<K extends keyof FormState>(key: K, value: FormState[K]) {
    setForm((current) => ({ ...current, [key]: value }));
  }

  async function submit() {
    if (!form.title_it.trim() || !form.maintenance_kind_id || !form.technical_domain_id || !form.customer_scope_id) {
      toast('Completa i campi obbligatori.', 'error');
      return;
    }
    const firstWindow: WindowBody | null =
      form.scheduled_start_at && form.scheduled_end_at
        ? {
            scheduled_start_at: form.scheduled_start_at,
            scheduled_end_at: form.scheduled_end_at,
            expected_downtime_minutes: form.expected_downtime_minutes
              ? Number(form.expected_downtime_minutes)
              : null,
          }
        : null;
    const body: MaintenanceFormBody = {
      title_it: form.title_it.trim(),
      title_en: form.title_en.trim() || null,
      description_it: form.description_it.trim() || null,
      description_en: form.description_en.trim() || null,
      maintenance_kind_id: Number(form.maintenance_kind_id),
      technical_domain_id: Number(form.technical_domain_id),
      customer_scope_id: Number(form.customer_scope_id),
      site_id: form.site_id ? Number(form.site_id) : null,
      reason_it: form.reason_it.trim() || null,
      residual_service_it: form.residual_service_it.trim() || null,
      first_window: firstWindow,
    };
    try {
      const result = await create.mutateAsync(body);
      toast('Manutenzione creata.');
      navigate(`/manutenzioni/${result.maintenance_id}`);
    } catch (error) {
      toast(errorMessage(error, 'Creazione non riuscita.'), 'error');
    }
  }

  return (
    <section className={shared.page}>
      <button type="button" className={shared.backLink} onClick={() => navigate('/manutenzioni')}>
        <Icon name="chevron-left" size={16} />
        Torna al registro
      </button>
      <div className={shared.header}>
        <div className={shared.titleBlock}>
          <h1 className={shared.pageTitle}>Nuova manutenzione</h1>
          <p className={shared.pageSubtitle}>
            Crea una bozza con le informazioni principali. Le finestre e l&apos;impatto si possono completare dal dettaglio.
          </p>
        </div>
      </div>

      {reference.isLoading ? (
        <div className={shared.panel}>
          <Skeleton rows={8} />
        </div>
      ) : reference.error || !reference.data ? (
        <div className={shared.emptyCard}>
          <div className={shared.emptyIconDanger}>
            <Icon name="triangle-alert" />
          </div>
          <h3>Configurazione non disponibile</h3>
          <p>{errorMessage(reference.error, 'Impossibile preparare il modulo.')}</p>
        </div>
      ) : (
        <div className={shared.panel}>
          <div className={shared.formGrid}>
            <label className={shared.label}>
              Titolo
              <input
                className={shared.field}
                value={form.title_it}
                onChange={(event) => update('title_it', event.target.value)}
                required
              />
            </label>
            <label className={shared.label}>
              Titolo inglese
              <input
                className={shared.field}
                value={form.title_en}
                onChange={(event) => update('title_en', event.target.value)}
              />
            </label>
            <label className={shared.label}>
              Tipo
              <select
                className={shared.select}
                value={form.maintenance_kind_id}
                onChange={(event) => update('maintenance_kind_id', event.target.value)}
                required
              >
                <option value="">Seleziona tipo</option>
                {reference.data.maintenance_kinds.map((item) => (
                  <option key={item.id} value={item.id}>
                    {item.name_it}
                  </option>
                ))}
              </select>
            </label>
            <label className={shared.label}>
              Dominio
              <select
                className={shared.select}
                value={form.technical_domain_id}
                onChange={(event) => update('technical_domain_id', event.target.value)}
                required
              >
                <option value="">Seleziona dominio</option>
                {reference.data.technical_domains.map((item) => (
                  <option key={item.id} value={item.id}>
                    {item.name_it}
                  </option>
                ))}
              </select>
            </label>
            <label className={shared.label}>
              Ambito clienti
              <select
                className={shared.select}
                value={form.customer_scope_id}
                onChange={(event) => update('customer_scope_id', event.target.value)}
                required
              >
                <option value="">Seleziona ambito</option>
                {reference.data.customer_scopes.map((item) => (
                  <option key={item.id} value={item.id}>
                    {item.name_it}
                  </option>
                ))}
              </select>
            </label>
            <label className={shared.label}>
              Sito
              <select
                className={shared.select}
                value={form.site_id}
                onChange={(event) => update('site_id', event.target.value)}
              >
                <option value="">Nessun sito</option>
                {reference.data.sites.map((item) => (
                  <option key={item.id} value={item.id}>
                    {item.name_it}
                  </option>
                ))}
              </select>
            </label>
            <label className={shared.label}>
              Descrizione
              <textarea
                className={shared.textarea}
                value={form.description_it}
                onChange={(event) => update('description_it', event.target.value)}
              />
            </label>
            <label className={shared.label}>
              Descrizione inglese
              <textarea
                className={shared.textarea}
                value={form.description_en}
                onChange={(event) => update('description_en', event.target.value)}
              />
            </label>
            <label className={shared.label}>
              Motivo
              <textarea
                className={shared.textarea}
                value={form.reason_it}
                onChange={(event) => update('reason_it', event.target.value)}
              />
            </label>
            <label className={shared.label}>
              Servizio residuo
              <textarea
                className={shared.textarea}
                value={form.residual_service_it}
                onChange={(event) => update('residual_service_it', event.target.value)}
              />
            </label>
          </div>

          <div className={shared.sectionHeader} style={{ marginTop: '1rem' }}>
            <h2 className={shared.sectionTitle}>Prima finestra</h2>
            <span className={shared.small}>Facoltativa</span>
          </div>
          <div className={shared.formGridThree}>
            <label className={shared.label}>
              Inizio previsto
              <input
                className={shared.field}
                type="datetime-local"
                value={form.scheduled_start_at}
                onChange={(event) => update('scheduled_start_at', event.target.value)}
              />
            </label>
            <label className={shared.label}>
              Fine prevista
              <input
                className={shared.field}
                type="datetime-local"
                value={form.scheduled_end_at}
                onChange={(event) => update('scheduled_end_at', event.target.value)}
              />
            </label>
            <label className={shared.label}>
              Downtime previsto
              <input
                className={shared.field}
                type="number"
                min="0"
                value={form.expected_downtime_minutes}
                onChange={(event) => update('expected_downtime_minutes', event.target.value)}
              />
            </label>
          </div>

          <div className={shared.formActions} style={{ marginTop: '1rem' }}>
            <Button variant="secondary" onClick={() => navigate('/manutenzioni')}>
              Annulla
            </Button>
            <Button onClick={submit} loading={create.isPending}>
              Crea bozza
            </Button>
          </div>
        </div>
      )}
    </section>
  );
}
