import { Button, Icon, MultiSelect, Skeleton, useToast } from '@mrsmith/ui';
import { useEffect, useMemo, useRef, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  useCreateMaintenance,
  useMaintenanceAssistancePreview,
  useReferenceData,
} from '../api/queries';
import type {
  AdhocSiteInput,
  AssistancePreviewResponse,
  ClassificationInput,
  MaintenanceFormBody,
  ReferenceItem,
  WindowBody,
} from '../api/types';
import { ConfirmDialog } from '../components/ConfirmDialog';
import { SiteSelectField } from '../components/SiteSelectField';
import { errorMessage } from '../lib/format';
import shared from './shared.module.css';

const BRIEF_MIN_LENGTH = 30;
const BRIEF_MAX_LENGTH = 2000;

type AiState = 'idle' | 'loading' | 'applied' | 'error';

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

const REQUIRED_FIELD_IDS = {
  summary_it: 'mcp-field-summary',
  maintenance_kind_id: 'mcp-field-kind',
  technical_domain_id: 'mcp-field-domain',
  customer_scope_id: 'mcp-field-scope',
} as const;

export function MaintenanceCreatePage() {
  const navigate = useNavigate();
  const { toast } = useToast();
  const reference = useReferenceData();
  const create = useCreateMaintenance();
  const assistancePreview = useMaintenanceAssistancePreview();
  const [form, setForm] = useState<FormState>(initialForm);
  const [attemptedSubmit, setAttemptedSubmit] = useState(false);
  const [leaveTarget, setLeaveTarget] = useState<string | null>(null);
  const [aiState, setAiState] = useState<AiState>('idle');
  const [aiError, setAiError] = useState<string | null>(null);
  const [preAiSnapshot, setPreAiSnapshot] = useState<FormState | null>(null);
  const [confirmRegenerate, setConfirmRegenerate] = useState(false);
  const formRef = useRef<HTMLDivElement>(null);
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
    if (!form.summary_it.trim()) reasons.push('titolo');
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
      const removed = current.service_taxonomy_ids.length - nextServices.length;
      if (removed > 0) {
        toast(
          `Rimosso ${removed} servizio${removed > 1 ? ' non compatibile' : ' non compatibile'} con il nuovo dominio.`,
          'warning',
        );
      }
      return { ...current, technical_domain_id: value, service_taxonomy_ids: nextServices };
    });
  }

  function focusFirstInvalid() {
    if (!form.summary_it.trim()) {
      document.getElementById(REQUIRED_FIELD_IDS.summary_it)?.focus();
      return;
    }
    if (!form.maintenance_kind_id) {
      document.getElementById(REQUIRED_FIELD_IDS.maintenance_kind_id)?.focus();
      return;
    }
    if (!form.technical_domain_id) {
      document.getElementById(REQUIRED_FIELD_IDS.technical_domain_id)?.focus();
      return;
    }
    if (!form.customer_scope_id) {
      document.getElementById(REQUIRED_FIELD_IDS.customer_scope_id)?.focus();
    }
  }

  function hasClassificationsOrTitle(): boolean {
    if (form.summary_it.trim()) return true;
    if (form.service_taxonomy_ids.length > 0) return true;
    if (form.reason_class_ids.length > 0) return true;
    if (form.impact_effect_ids.length > 0) return true;
    if (form.quality_flag_ids.length > 0) return true;
    return false;
  }

  async function runAiApply() {
    const brief = form.assistance_context.trim();
    if (brief.length < BRIEF_MIN_LENGTH) return;
    setAiState('loading');
    setAiError(null);
    try {
      const response = await assistancePreview.mutateAsync({
        brief,
        maintenance_kind_id: form.maintenance_kind_id ? Number(form.maintenance_kind_id) : null,
        technical_domain_id: form.technical_domain_id ? Number(form.technical_domain_id) : null,
        customer_scope_id: form.customer_scope_id ? Number(form.customer_scope_id) : null,
      });
      applyAssistance(response);
      setAiState('applied');
    } catch (error) {
      setAiError(errorMessage(error, 'Compilazione non disponibile. Inserisci i campi manualmente.'));
      setAiState('error');
    }
  }

  function handleAiApplyClick() {
    if (aiState === 'applied' && hasClassificationsOrTitle()) {
      setConfirmRegenerate(true);
      return;
    }
    setPreAiSnapshot(form);
    runAiApply();
  }

  function applyAssistance(response: AssistancePreviewResponse) {
    setForm((current) => ({
      ...current,
      summary_it: response.texts.title_it?.trim() || current.summary_it,
      service_taxonomy_ids:
        response.service_taxonomy_ids.length > 0
          ? response.service_taxonomy_ids
          : current.service_taxonomy_ids,
      reason_class_ids:
        response.reason_class_ids.length > 0 ? response.reason_class_ids : current.reason_class_ids,
      impact_effect_ids:
        response.impact_effect_ids.length > 0
          ? response.impact_effect_ids
          : current.impact_effect_ids,
      quality_flag_ids:
        response.quality_flag_ids.length > 0 ? response.quality_flag_ids : current.quality_flag_ids,
    }));
  }

  function handleAiUndo() {
    if (preAiSnapshot) {
      setForm(preAiSnapshot);
    }
    setAiState('idle');
    setAiError(null);
    setPreAiSnapshot(null);
  }

  function confirmRegenerateNow() {
    setConfirmRegenerate(false);
    setPreAiSnapshot(form);
    runAiApply();
  }

  async function submit() {
    setAttemptedSubmit(true);
    if (!canSubmit) {
      focusFirstInvalid();
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

  function requestLeave(target: string) {
    if (dirty && !create.isPending) {
      setLeaveTarget(target);
      return;
    }
    navigate(target);
  }

  function handleBackClick(event: React.MouseEvent<HTMLAnchorElement>) {
    event.preventDefault();
    requestLeave('/manutenzioni');
  }

  return (
    <section className={`${shared.page} ${shared.pageNarrow}`}>
      <a className={shared.backLink} href="/manutenzioni" onClick={handleBackClick}>
        <Icon name="chevron-left" size={16} />
        Torna al registro
      </a>
      <div className={shared.header}>
        <div className={shared.titleBlock}>
          <h1 className={shared.pageTitle}>Nuova manutenzione</h1>
          <p className={shared.pageSubtitle}>
            Incolla un brief oppure compila i campi essenziali. I testi sono provvisori: si affineranno nei prossimi step.
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
          <div ref={formRef} className={shared.tabsSpacer}>
            <BriefBlock
              value={form.assistance_context}
              onChange={(value) => update('assistance_context', value)}
              aiState={aiState}
              aiError={aiError}
              onApply={handleAiApplyClick}
              onUndo={handleAiUndo}
            />

            <div className={shared.panel}>
              <div className={shared.sectionHeader}>
                <h2 className={shared.sectionTitle}>Contesto</h2>
                <span className={shared.sectionBadge}>Obbligatorio</span>
              </div>
              <div className={shared.formGrid}>
                <label className={`${shared.label} ${shared.formGridSpan}`}>
                  <span className={shared.labelText}>
                    Titolo <span className={shared.required}>*</span>
                  </span>
                  <input
                    id={REQUIRED_FIELD_IDS.summary_it}
                    className={`${shared.field} ${attemptedSubmit && !form.summary_it.trim() ? shared.fieldInvalid : ''}`}
                    value={form.summary_it}
                    onChange={(event) => update('summary_it', event.target.value)}
                    required
                  />
                </label>
                <SelectField
                  label="Tipo"
                  required
                  id={REQUIRED_FIELD_IDS.maintenance_kind_id}
                  value={form.maintenance_kind_id}
                  items={reference.data.maintenance_kinds}
                  invalid={attemptedSubmit && !form.maintenance_kind_id}
                  onChange={(value) => update('maintenance_kind_id', value)}
                />
                <SelectField
                  label="Dominio tecnico"
                  required
                  id={REQUIRED_FIELD_IDS.technical_domain_id}
                  value={form.technical_domain_id}
                  items={reference.data.technical_domains}
                  invalid={attemptedSubmit && !form.technical_domain_id}
                  onChange={updateDomain}
                />
                <SelectField
                  label="Ambito clienti"
                  required
                  id={REQUIRED_FIELD_IDS.customer_scope_id}
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

            <details className={shared.collapsiblePanel} open>
              <summary className={shared.collapsibleSummary}>
                <span className={shared.collapsibleSummaryLeft}>
                  <Icon name="chevron-right" size={16} className={shared.collapsibleChevron} />
                  <h2 className={shared.sectionTitle}>Classificazione</h2>
                </span>
                <span className={shared.sectionBadge}>Opzionale</span>
              </summary>
              <div className={shared.collapsibleContent}>
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
            </details>

            <details className={shared.collapsiblePanel}>
              <summary className={shared.collapsibleSummary}>
                <span className={shared.collapsibleSummaryLeft}>
                  <Icon name="chevron-right" size={16} className={shared.collapsibleChevron} />
                  <h2 className={shared.sectionTitle}>Prima finestra</h2>
                </span>
                <span className={shared.sectionBadge}>Opzionale</span>
              </summary>
              <div className={shared.collapsibleContent}>
                <div className={shared.formGridThree}>
                  <label className={shared.label}>
                    Inizio previsto
                    <input
                      className={shared.field}
                      type="datetime-local"
                      value={form.scheduled_start_at}
                      onChange={(event) => update('scheduled_start_at', event.target.value)}
                    />
                    <span className={shared.fieldHelper}>Ora locale del browser.</span>
                  </label>
                  <label className={shared.label}>
                    Fine prevista
                    <input
                      className={shared.field}
                      type="datetime-local"
                      value={form.scheduled_end_at}
                      onChange={(event) => update('scheduled_end_at', event.target.value)}
                    />
                    <span className={shared.fieldHelper}>Ora locale del browser.</span>
                  </label>
                  <label className={shared.label}>
                    Downtime previsto (minuti)
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
            </details>
          </div>

          <div className={shared.stickyActionBar}>
            <span className={shared.small}>
              {canSubmit ? 'Pronta per la bozza.' : `Mancano: ${missing.join(', ')}.`}
            </span>
            <div className={shared.formActions}>
              <Button variant="secondary" onClick={() => requestLeave('/manutenzioni')}>
                Annulla
              </Button>
              <Button onClick={submit} loading={create.isPending}>
                Crea bozza
              </Button>
            </div>
          </div>

          <ConfirmDialog
            open={leaveTarget !== null}
            title="Uscire senza salvare?"
            message="La bozza non è stata creata. Le modifiche andranno perse."
            confirmLabel="Esci senza salvare"
            variant="danger"
            onConfirm={() => {
              const target = leaveTarget;
              setLeaveTarget(null);
              if (target) navigate(target);
            }}
            onClose={() => setLeaveTarget(null)}
          />

          <ConfirmDialog
            open={confirmRegenerate}
            title="Sovrascrivere i campi compilati?"
            message="Il brief è cambiato. La compilazione sostituirà titolo e classificazioni attuali."
            confirmLabel="Sovrascrivi"
            variant="primary"
            onConfirm={confirmRegenerateNow}
            onClose={() => setConfirmRegenerate(false)}
          />
        </>
      )}
    </section>
  );
}

function BriefBlock({
  value,
  onChange,
  aiState,
  aiError,
  onApply,
  onUndo,
}: {
  value: string;
  onChange: (value: string) => void;
  aiState: AiState;
  aiError: string | null;
  onApply: () => void;
  onUndo: () => void;
}) {
  const length = value.length;
  const canApply = value.trim().length >= BRIEF_MIN_LENGTH && length <= BRIEF_MAX_LENGTH;
  const overLimit = length > BRIEF_MAX_LENGTH;

  return (
    <div className={shared.briefPanel}>
      <div className={shared.briefHeader}>
        <h2 className={shared.briefTitle}>Brief</h2>
        <span className={`${shared.briefCounter} ${overLimit ? shared.briefCounterWarn : ''}`}>
          {length}/{BRIEF_MAX_LENGTH}
        </span>
      </div>
      <textarea
        className={shared.briefTextarea}
        value={value}
        onChange={(event) => onChange(event.target.value)}
        placeholder="Descrizione libera dell'intervento: apparati, finestra prevista, impatto atteso, motivo."
      />
      <div className={shared.briefActions}>
        <Button
          onClick={onApply}
          loading={aiState === 'loading'}
          disabled={!canApply || aiState === 'loading'}
        >
          {aiState === 'loading' ? 'Analisi in corso…' : 'Compila dal brief'}
        </Button>
      </div>
      {aiState === 'applied' && (
        <div className={`${shared.briefBanner} ${shared.briefBannerSuccess}`}>
          <span>Campi compilati dal brief. Verifica e crea la bozza.</span>
          <button type="button" className={shared.briefBannerAction} onClick={onUndo}>
            Annulla compilazione
          </button>
        </div>
      )}
      {aiState === 'error' && aiError && (
        <div className={`${shared.briefBanner} ${shared.briefBannerError}`}>
          <span>{aiError}</span>
        </div>
      )}
    </div>
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
  id,
}: {
  label: string;
  value: string;
  items: ReferenceItem[];
  onChange: (value: string) => void;
  emptyLabel?: string;
  required?: boolean;
  invalid?: boolean;
  id?: string;
}) {
  return (
    <label className={shared.label}>
      <span className={shared.labelText}>
        {label} {required && <span className={shared.required}>*</span>}
      </span>
      <select
        id={id}
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
