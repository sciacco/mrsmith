import { Button, Icon, MultiSelect, Skeleton, useToast } from '@mrsmith/ui';
import { useEffect, useMemo, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useCreateMaintenance, useReferenceData } from '../api/queries';
import type {
  AdhocSiteInput,
  ClassificationInput,
  MaintenanceFormBody,
  ReferenceItem,
  WindowBody,
} from '../api/types';
import { SiteSelectField } from '../components/SiteSelectField';
import { errorMessage } from '../lib/format';
import shared from './shared.module.css';

interface FormState {
  summary_it: string;
  assistance_context: string;
  maintenance_kind_id: string;
  technical_domain_id: string;
  customer_scope_id: string;
  site_id: string;
  adhoc_site: AdhocSiteInput | null;
  service_taxonomy_ids: number[];
  reason_class_ids: number[];
  impact_effect_ids: number[];
  quality_flag_ids: number[];
  residual_service_it: string;
  scheduled_start_at: string;
  scheduled_end_at: string;
  expected_downtime_minutes: string;
}

const initialForm: FormState = {
  summary_it: '',
  assistance_context: '',
  maintenance_kind_id: '',
  technical_domain_id: '',
  customer_scope_id: '',
  site_id: '',
  adhoc_site: null,
  service_taxonomy_ids: [],
  reason_class_ids: [],
  impact_effect_ids: [],
  quality_flag_ids: [],
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
  const [attemptedSubmit, setAttemptedSubmit] = useState(false);
  const dirty = useMemo(() => JSON.stringify(form) !== JSON.stringify(initialForm), [form]);

  useEffect(() => {
    const onBeforeUnload = (event: BeforeUnloadEvent) => {
      if (!dirty || create.isPending) return;
      event.preventDefault();
      event.returnValue = '';
    };
    window.addEventListener('beforeunload', onBeforeUnload);
    return () => window.removeEventListener('beforeunload', onBeforeUnload);
  }, [create.isPending, dirty]);

  const selectedDomainId = form.technical_domain_id ? Number(form.technical_domain_id) : null;
  const serviceItems = useMemo(() => {
    const items = reference.data?.service_taxonomy ?? [];
    if (!selectedDomainId) return items;
    return items.filter((item) => item.technical_domain_id === selectedDomainId);
  }, [reference.data?.service_taxonomy, selectedDomainId]);

  const missing = useMemo(() => {
    const reasons: string[] = [];
    if (!form.summary_it.trim()) reasons.push('sintesi operativa');
    if (!form.maintenance_kind_id) reasons.push('tipo');
    if (!form.technical_domain_id) reasons.push('dominio tecnico');
    if (!form.customer_scope_id) reasons.push('ambito clienti');
    return reasons;
  }, [form.customer_scope_id, form.maintenance_kind_id, form.summary_it, form.technical_domain_id]);

  const canSubmit = missing.length === 0;

  function update<K extends keyof FormState>(key: K, value: FormState[K]) {
    setForm((current) => ({ ...current, [key]: value }));
  }

  function updateDomain(value: string) {
    setForm((current) => {
      const domainId = value ? Number(value) : null;
      const nextServices = domainId
        ? current.service_taxonomy_ids.filter((id) => {
            const item = reference.data?.service_taxonomy.find((service) => service.id === id);
            return item?.technical_domain_id === domainId;
          })
        : current.service_taxonomy_ids;
      return { ...current, technical_domain_id: value, service_taxonomy_ids: nextServices };
    });
  }

  async function submit() {
    setAttemptedSubmit(true);
    if (!canSubmit) {
      toast(`Per creare la bozza completa: ${missing.join(', ')}.`, 'error');
      return;
    }
    if ((form.scheduled_start_at && !form.scheduled_end_at) || (!form.scheduled_start_at && form.scheduled_end_at)) {
      toast('Indica inizio e fine della prima finestra.', 'error');
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
    if (form.adhoc_site && !form.adhoc_site.name.trim()) {
      toast('Indica il nome del sito ad-hoc.', 'error');
      return;
    }
    const body: MaintenanceFormBody = {
      title_it: form.summary_it.trim(),
      description_it: form.assistance_context.trim() || null,
      maintenance_kind_id: Number(form.maintenance_kind_id),
      technical_domain_id: Number(form.technical_domain_id),
      customer_scope_id: Number(form.customer_scope_id),
      site_id: form.adhoc_site ? null : form.site_id ? Number(form.site_id) : null,
      adhoc_site: form.adhoc_site
        ? {
            name: form.adhoc_site.name.trim(),
            city: form.adhoc_site.city?.trim() || null,
            country_code: form.adhoc_site.country_code?.trim() || null,
            code: form.adhoc_site.code?.trim() || null,
          }
        : null,
      residual_service_it: form.residual_service_it.trim() || null,
      first_window: firstWindow,
      initial_service_taxonomy: classificationInputs(form.service_taxonomy_ids, true),
      initial_reason_classes: classificationInputs(form.reason_class_ids, true),
      initial_impact_effects: classificationInputs(form.impact_effect_ids, true),
      initial_quality_flags: classificationInputs(form.quality_flag_ids, false),
      metadata: {
        ai_intake: {
          summary_it: form.summary_it.trim(),
          context_it: form.assistance_context.trim() || null,
          service_taxonomy_ids: form.service_taxonomy_ids,
          reason_class_ids: form.reason_class_ids,
          impact_effect_ids: form.impact_effect_ids,
          quality_flag_ids: form.quality_flag_ids,
        },
      },
    };
    try {
      const result = await create.mutateAsync(body);
      toast('Bozza creata.');
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
            Raccogli il contesto essenziale e crea una bozza da completare nel dettaglio.
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
        <>
          <div className={shared.wizardLayout}>
            <aside className={`${shared.panel} ${shared.wizardAside}`}>
              <div className={shared.stepList} aria-label="Avanzamento creazione">
                <span className={shared.stepItemActive}>1. Contesto</span>
                <span className={shared.stepItem}>2. Classificazione</span>
                <span className={shared.stepItem}>3. Finestra</span>
              </div>
              <div className={shared.summaryList}>
                <div>
                  <span>Pronto per bozza</span>
                  <strong>{canSubmit ? 'Sì' : 'No'}</strong>
                </div>
                <div>
                  <span>Servizi</span>
                  <strong>{selectionLabel(form.service_taxonomy_ids, reference.data.service_taxonomy)}</strong>
                </div>
                <div>
                  <span>Prima finestra</span>
                  <strong>{form.scheduled_start_at && form.scheduled_end_at ? 'Indicata' : 'Da definire'}</strong>
                </div>
              </div>
              {!canSubmit && attemptedSubmit && (
                <p className={shared.fieldError}>Completa: {missing.join(', ')}.</p>
              )}
            </aside>

            <div className={shared.wizardMain}>
              <div className={shared.panel}>
                <div className={shared.sectionHeader}>
                  <h2 className={shared.sectionTitle}>Contesto</h2>
                  <span className={shared.small}>Dati obbligatori</span>
                </div>
                <div className={shared.formGrid}>
                  <label className={`${shared.label} ${shared.formGridSpan}`}>
                    <span className={shared.labelText}>
                      Sintesi operativa <span className={shared.required}>*</span>
                    </span>
                    <input
                      className={`${shared.field} ${attemptedSubmit && !form.summary_it.trim() ? shared.fieldInvalid : ''}`}
                      value={form.summary_it}
                      onChange={(event) => update('summary_it', event.target.value)}
                      required
                    />
                  </label>
                  <label className={`${shared.label} ${shared.formGridSpan}`}>
                    Contesto per assistenza
                    <textarea
                      className={shared.textarea}
                      value={form.assistance_context}
                      onChange={(event) => update('assistance_context', event.target.value)}
                    />
                  </label>
                  <SelectField
                    label="Tipo"
                    required
                    value={form.maintenance_kind_id}
                    items={reference.data.maintenance_kinds}
                    invalid={attemptedSubmit && !form.maintenance_kind_id}
                    onChange={(value) => update('maintenance_kind_id', value)}
                  />
                  <SelectField
                    label="Dominio tecnico"
                    required
                    value={form.technical_domain_id}
                    items={reference.data.technical_domains}
                    invalid={attemptedSubmit && !form.technical_domain_id}
                    onChange={updateDomain}
                  />
                  <SelectField
                    label="Ambito clienti"
                    required
                    value={form.customer_scope_id}
                    items={reference.data.customer_scopes}
                    invalid={attemptedSubmit && !form.customer_scope_id}
                    onChange={(value) => update('customer_scope_id', value)}
                  />
                  <SiteSelectField
                    sites={reference.data.sites}
                    value={{
                      site_id: form.site_id ? Number(form.site_id) : null,
                      adhoc_site: form.adhoc_site,
                    }}
                    onChange={(next) =>
                      setForm((current) => ({
                        ...current,
                        site_id: next.site_id != null ? String(next.site_id) : '',
                        adhoc_site: next.adhoc_site,
                      }))
                    }
                  />
                </div>
              </div>

              <div className={shared.panel}>
                <div className={shared.sectionHeader}>
                  <h2 className={shared.sectionTitle}>Classificazione iniziale</h2>
                  <span className={shared.small}>Facoltativa</span>
                </div>
                <div className={shared.formGrid}>
                  <MultiSelectField
                    label="Servizi coinvolti"
                    options={toOptions(serviceItems)}
                    selected={form.service_taxonomy_ids}
                    onChange={(value) => update('service_taxonomy_ids', value)}
                  />
                  <MultiSelectField
                    label="Motivi"
                    options={toOptions(reference.data.reason_classes)}
                    selected={form.reason_class_ids}
                    onChange={(value) => update('reason_class_ids', value)}
                  />
                  <MultiSelectField
                    label="Effetti attesi"
                    options={toOptions(reference.data.impact_effects)}
                    selected={form.impact_effect_ids}
                    onChange={(value) => update('impact_effect_ids', value)}
                  />
                  <MultiSelectField
                    label="Segnali qualità"
                    options={toOptions(reference.data.quality_flags)}
                    selected={form.quality_flag_ids}
                    onChange={(value) => update('quality_flag_ids', value)}
                  />
                  <label className={`${shared.label} ${shared.formGridSpan}`}>
                    Servizio residuo
                    <textarea
                      className={shared.textarea}
                      value={form.residual_service_it}
                      onChange={(event) => update('residual_service_it', event.target.value)}
                    />
                  </label>
                </div>
              </div>

              <div className={shared.panel}>
                <div className={shared.sectionHeader}>
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
              </div>
            </div>
          </div>

          <div className={shared.stickyActionBar}>
            <span className={shared.small}>
              {canSubmit ? 'Tutto pronto per creare la bozza.' : `Per completare: ${missing.join(', ')}.`}
            </span>
            <div className={shared.formActions}>
              <Button variant="secondary" onClick={() => navigate('/manutenzioni')}>
                Annulla
              </Button>
              <Button onClick={submit} loading={create.isPending}>
                Crea bozza
              </Button>
            </div>
          </div>
        </>
      )}
    </section>
  );
}

function SelectField({
  label,
  value,
  items,
  onChange,
  emptyLabel = 'Seleziona',
  required = false,
  invalid = false,
}: {
  label: string;
  value: string;
  items: ReferenceItem[];
  onChange: (value: string) => void;
  emptyLabel?: string;
  required?: boolean;
  invalid?: boolean;
}) {
  return (
    <label className={shared.label}>
      <span className={shared.labelText}>
        {label} {required && <span className={shared.required}>*</span>}
      </span>
      <select
        className={`${shared.select} ${invalid ? shared.fieldInvalid : ''}`}
        value={value}
        onChange={(event) => onChange(event.target.value)}
        required={required}
      >
        <option value="">{emptyLabel}</option>
        {items.map((item) => (
          <option key={item.id} value={item.id}>
            {item.name_it}
          </option>
        ))}
      </select>
    </label>
  );
}

function MultiSelectField({
  label,
  options,
  selected,
  onChange,
}: {
  label: string;
  options: Array<{ value: number; label: string }>;
  selected: number[];
  onChange: (value: number[]) => void;
}) {
  return (
    <div className={shared.fieldGroup}>
      <span className={shared.fieldLabel}>{label}</span>
      <MultiSelect<number>
        options={options}
        selected={selected}
        onChange={onChange}
        placeholder="Seleziona..."
      />
    </div>
  );
}

function toOptions(items: ReferenceItem[]): Array<{ value: number; label: string }> {
  return items.map((item) => ({ value: item.id, label: item.name_it }));
}

function classificationInputs(ids: number[], hasPrimary: boolean): ClassificationInput[] {
  return ids.map((id, index) => ({
    reference_id: id,
    source: 'manual',
    confidence: null,
    is_primary: hasPrimary && index === 0,
  }));
}

function selectionLabel(ids: number[], items: ReferenceItem[]): string {
  if (ids.length === 0) return '-';
  const names = ids
    .map((id) => items.find((item) => item.id === id)?.name_it)
    .filter(Boolean);
  if (names.length === 0) return `${ids.length}`;
  if (names.length === 1) return names[0] ?? '-';
  return `${names[0]} +${names.length - 1}`;
}
