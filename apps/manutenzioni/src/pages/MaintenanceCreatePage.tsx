import { Button, Icon, MultiSelect, Skeleton, useToast } from '@mrsmith/ui';
import { useEffect, useMemo, useRef, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  useCreateMaintenance,
  useMaintenanceAssistancePreview,
  useReferenceData,
  useServiceDependencies,
} from '../api/queries';
import type {
  AdhocSiteInput,
  AssistancePreviewResponse,
  AudienceOverride,
  ClassificationInput,
  MaintenanceFormBody,
  ReferenceItem,
  ServiceDependency,
  SeverityValue,
  TargetBody,
  WindowBody,
} from '../api/types';
import { ConfirmDialog } from '../components/ConfirmDialog';
import { RequiredMark } from '../components/RequiredMark';
import { SiteSelectField } from '../components/SiteSelectField';
import { audienceLabel, dependencyTypeLabel, errorMessage, severityLabel } from '../lib/format';
import shared from './shared.module.css';

const BRIEF_MIN_LENGTH = 30;
const BRIEF_MAX_LENGTH = 2000;
const RESIDUAL_SERVICE_PLACEHOLDER = [
  'Solo se necessario indicare una descrizione che chiarisca lo stato dei servizi residui. Es:',
  '- "Durante l\'intervento resta attivo il ramo B."',
  '- "Il servizio sarà erogato dal sito secondario con capacità ridotta."',
  '- "Il portale resta consultabile, ma non saranno disponibili modifiche di configurazione."',
  '- "Nessun servizio residuo garantito nella finestra 02:00-03:00."',
].join('\n');

type AiState = 'idle' | 'loading' | 'applied' | 'error';

interface FormState {
  summary_it: string;
  assistance_context: string;
  maintenance_kind_id: string;
  technical_domain_id: string;
  customer_scope_id: string;
  site_id: string;
  adhoc_site: AdhocSiteInput | null;
  service_selections: ServiceSelection[];
  manual_targets: ManualTarget[];
  reason_class_ids: number[];
  impact_effect_ids: number[];
  residual_service_it: string;
  scheduled_start_at: string;
  scheduled_end_at: string;
  expected_downtime_minutes: string;
}

interface ServiceSelection {
  service_taxonomy_id: number;
  role: 'operated' | 'dependent';
  expected_severity: SeverityValue;
  expected_audience: AudienceOverride | null;
  source: 'manual' | 'dependency_graph' | 'ai_extracted';
}

interface ManualTarget {
  id: string;
  target_type_id: number;
  display_name: string;
  service_taxonomy_id: number | null;
}

const initialForm: FormState = {
  summary_it: '',
  assistance_context: '',
  maintenance_kind_id: '',
  technical_domain_id: '',
  customer_scope_id: '',
  site_id: '',
  adhoc_site: null,
  service_selections: [],
  manual_targets: [],
  reason_class_ids: [],
  impact_effect_ids: [],
  residual_service_it: '',
  scheduled_start_at: '',
  scheduled_end_at: '',
  expected_downtime_minutes: '',
};

const REQUIRED_FIELD_IDS = {
  summary_it: 'mcp-field-summary',
  maintenance_kind_id: 'mcp-field-kind',
  technical_domain_id: 'mcp-field-domain',
} as const;

export function MaintenanceCreatePage() {
  const navigate = useNavigate();
  const { toast } = useToast();
  const reference = useReferenceData();
  const dependencies = useServiceDependencies('active');
  const create = useCreateMaintenance();
  const assistancePreview = useMaintenanceAssistancePreview();
  const [form, setForm] = useState<FormState>(initialForm);
  const [attemptedSubmit, setAttemptedSubmit] = useState(false);
  const [leaveTarget, setLeaveTarget] = useState<string | null>(null);
  const [aiState, setAiState] = useState<AiState>('idle');
  const [aiError, setAiError] = useState<string | null>(null);
  const [aiOpen, setAiOpen] = useState(false);
  const [preAiSnapshot, setPreAiSnapshot] = useState<FormState | null>(null);
  const [confirmRegenerate, setConfirmRegenerate] = useState(false);
  const [checkedSuggestions, setCheckedSuggestions] = useState<Record<number, boolean>>({});
  const [targetDraft, setTargetDraft] = useState<ManualTarget>({
    id: '',
    target_type_id: 0,
    display_name: '',
    service_taxonomy_id: null,
  });
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
  const operatedIds = useMemo(
    () => form.service_selections.filter((item) => item.role === 'operated').map((item) => item.service_taxonomy_id),
    [form.service_selections],
  );
  const impactedIds = useMemo(
    () => form.service_selections.filter((item) => item.role === 'dependent').map((item) => item.service_taxonomy_id),
    [form.service_selections],
  );
  const directSuggestions = useMemo(() => {
    const selected = new Set(form.service_selections.map((item) => item.service_taxonomy_id));
    const operated = new Set(operatedIds);
    return (dependencies.data ?? [])
      .filter((item) => operated.has(item.upstream_service_id) && !selected.has(item.downstream_service_id))
      .sort((a, b) => a.downstream_service.name_it.localeCompare(b.downstream_service.name_it));
  }, [dependencies.data, form.service_selections, operatedIds]);
  const transitivePreview = useMemo(
    () => dependencyPreview(operatedIds, dependencies.data ?? [], form.service_selections),
    [dependencies.data, form.service_selections, operatedIds],
  );

  useEffect(() => {
    setCheckedSuggestions((current) => {
      let dirtyMap = false;
      const next = { ...current };
      for (const item of directSuggestions) {
        if (!(item.service_dependency_id in next)) {
          next[item.service_dependency_id] = getSuggestionDefaultChecked(item);
          dirtyMap = true;
        }
      }
      return dirtyMap ? next : current;
    });
  }, [directSuggestions]);

  const missing = useMemo(() => {
    const reasons: string[] = [];
    if (!form.summary_it.trim()) reasons.push('titolo');
    if (!form.maintenance_kind_id) reasons.push('tipo');
    if (!form.technical_domain_id) reasons.push('dominio tecnico');
    return reasons;
  }, [form.maintenance_kind_id, form.summary_it, form.technical_domain_id]);

  const canSubmit = missing.length === 0;

  function update<K extends keyof FormState>(key: K, value: FormState[K]) {
    setForm((current) => ({ ...current, [key]: value }));
  }

  function updateDomain(value: string) {
    setForm((current) => {
      const domainId = value ? Number(value) : null;
      const nextServices = domainId
        ? current.service_selections.filter((selection) => {
            if (selection.role !== 'operated') return true;
            const id = selection.service_taxonomy_id;
            const item = reference.data?.service_taxonomy.find((service) => service.id === id);
            return item?.technical_domain_id === domainId;
          })
        : current.service_selections;
      const removed = current.service_selections.length - nextServices.length;
      if (removed > 0) {
        toast(
          `Rimosso ${removed} servizio operato non compatibile con il nuovo dominio.`,
          'warning',
        );
      }
      return { ...current, technical_domain_id: value, service_selections: nextServices };
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
    }
  }

  function hasClassificationsOrTitle(): boolean {
    if (form.summary_it.trim()) return true;
    if (form.service_selections.length > 0) return true;
    if (form.reason_class_ids.length > 0) return true;
    if (form.impact_effect_ids.length > 0) return true;
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
      service_selections:
        response.service_taxonomy_ids.length > 0
          ? mergeServiceSelections(
              current.service_selections,
              response.service_taxonomy_ids.map((id) => ({
                service_taxonomy_id: id,
                role: 'operated' as const,
                expected_severity: 'unavailable' as const,
                expected_audience: null,
                source: 'ai_extracted' as const,
              })),
            )
          : current.service_selections,
      reason_class_ids:
        response.reason_class_ids.length > 0 ? response.reason_class_ids : current.reason_class_ids,
      impact_effect_ids:
        response.impact_effect_ids.length > 0
          ? response.impact_effect_ids
          : current.impact_effect_ids,
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
      customer_scope_id: form.customer_scope_id ? Number(form.customer_scope_id) : null,
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
      initial_targets: form.manual_targets.map(targetInput),
      initial_service_taxonomy: serviceClassificationInputs(form.service_selections),
      initial_reason_classes: classificationInputs(form.reason_class_ids, true),
      initial_impact_effects: classificationInputs(form.impact_effect_ids, true),
      metadata: {
        ai_intake: {
          summary_it: form.summary_it.trim(),
          context_it: form.assistance_context.trim() || null,
          service_taxonomy_ids: form.service_selections.map((item) => item.service_taxonomy_id),
          service_selections: form.service_selections.map((item) => ({
            service_taxonomy_id: item.service_taxonomy_id,
            role: item.role,
            expected_severity: item.expected_severity,
            expected_audience: item.expected_audience,
            source: item.source,
          })),
          reason_class_ids: form.reason_class_ids,
          impact_effect_ids: form.impact_effect_ids,
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

  function updateServiceRole(role: ServiceSelection['role'], ids: number[]) {
    setForm((current) => {
      let acceptedIds = ids;
      if (role === 'dependent') {
        const operatedSet = new Set(
          current.service_selections.filter((item) => item.role === 'operated').map((item) => item.service_taxonomy_id),
        );
        const rejected = ids.filter((id) => operatedSet.has(id));
        if (rejected.length > 0) {
          toast('Già operato — gli operati sono impattati per definizione.', 'warning');
          acceptedIds = ids.filter((id) => !operatedSet.has(id));
        }
      }
      const keep = current.service_selections.filter((item) => item.role !== role);
      const nextSelections = acceptedIds.map((id) => {
        const existing = current.service_selections.find((item) => item.service_taxonomy_id === id);
        return {
          service_taxonomy_id: id,
          role,
          expected_severity: existing?.expected_severity ?? defaultSeverityFor(role),
          expected_audience: existing?.expected_audience ?? null,
          source: existing?.source ?? 'manual',
        } satisfies ServiceSelection;
      });
      return {
        ...current,
        service_selections: mergeServiceSelections(keep, nextSelections),
      };
    });
  }

  function updateServiceSelection(id: number, patch: Partial<ServiceSelection>) {
    if (patch.role === 'operated' && !canOperateService(id)) {
      toast('Solo i servizi del dominio tecnico possono essere operati.', 'error');
      return;
    }
    setForm((current) => ({
      ...current,
      service_selections: current.service_selections.map((item) =>
        item.service_taxonomy_id === id ? { ...item, ...patch } : item,
      ),
    }));
  }

  function canOperateService(id: number): boolean {
    if (!selectedDomainId) return true;
    const service = reference.data?.service_taxonomy.find((item) => item.id === id);
    return service?.technical_domain_id === selectedDomainId;
  }

  function addCheckedSuggestions() {
    const selected = directSuggestions.filter(
      (item) => checkedSuggestions[item.service_dependency_id] === true,
    );
    if (selected.length === 0) {
      toast('Seleziona almeno un suggerimento.', 'error');
      return;
    }
    setForm((current) => ({
      ...current,
      service_selections: mergeServiceSelections(
        current.service_selections,
        selected.map((item) => ({
          service_taxonomy_id: item.downstream_service_id,
          role: 'dependent',
          expected_severity: item.default_severity,
          expected_audience: null,
          source: 'dependency_graph',
        })),
      ),
    }));
  }

  function addManualTarget() {
    if (!targetDraft.target_type_id || !targetDraft.display_name.trim()) {
      toast('Completa tipo e nome target.', 'error');
      return;
    }
    setForm((current) => ({
      ...current,
      manual_targets: [
        ...current.manual_targets,
        {
          ...targetDraft,
          id: targetDraft.id || crypto.randomUUID(),
          display_name: targetDraft.display_name.trim(),
        },
      ],
    }));
    setTargetDraft({ id: '', target_type_id: 0, display_name: '', service_taxonomy_id: null });
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
            Compila i campi essenziali. I testi sono provvisori: si affineranno nei prossimi step.
          </p>
        </div>
        <div className={shared.headerActions}>
          <button
            type="button"
            className={`${shared.aiToggle} ${aiOpen ? shared.aiToggleActive : ''}`}
            onClick={() => setAiOpen((open) => !open)}
            aria-pressed={aiOpen}
            aria-expanded={aiOpen}
            title="Compila da brief (sperimentale)"
          >
            <Icon name="sparkles" size={16} />
          </button>
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
            {aiOpen && (
              <BriefBlock
                value={form.assistance_context}
                onChange={(value) => update('assistance_context', value)}
                aiState={aiState}
                aiError={aiError}
                onApply={handleAiApplyClick}
                onUndo={handleAiUndo}
              />
            )}

            <div className={shared.panel}>
              <div className={shared.sectionHeader}>
                <h2 className={shared.sectionTitle}>Contesto</h2>
                <span className={shared.sectionBadge}>Obbligatorio</span>
              </div>
              <div className={shared.formGrid}>
                <label className={`${shared.label} ${shared.formGridSpan}`}>
                  <span className={shared.labelText}>
                    Titolo provvisorio <RequiredMark />
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
                  label="Dominio tecnico / Team"
                  required
                  id={REQUIRED_FIELD_IDS.technical_domain_id}
                  value={form.technical_domain_id}
                  items={reference.data.technical_domains}
                  invalid={attemptedSubmit && !form.technical_domain_id}
                  onChange={updateDomain}
                />
                <SelectField
                  label="Ambito clienti"
                  value={form.customer_scope_id}
                  items={reference.data.customer_scopes}
                  emptyLabel="Da definire"
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
                  <h2 className={shared.sectionTitle}>Sommario</h2>
                </span>
                <span className={shared.sectionBadge}>Opzionale</span>
              </summary>
              <div className={shared.collapsibleContent}>
                <div className={shared.formGrid}>
                  <MultiSelectField
                    label="Motivazione intervento"
                    options={toOptions(reference.data.reason_classes)}
                    selected={form.reason_class_ids}
                    onChange={(value) => update('reason_class_ids', value)}
                  />
                  <MultiSelectField
                    label="Impatto previsto"
                    options={toOptions(reference.data.impact_effects)}
                    selected={form.impact_effect_ids}
                    onChange={(value) => update('impact_effect_ids', value)}
                  />
                  <label className={`${shared.label} ${shared.formGridSpan}`}>
                    Servizio garantito durante l'intervento
                    <textarea
                      className={shared.textarea}
                      placeholder={RESIDUAL_SERVICE_PLACEHOLDER}
                      value={form.residual_service_it}
                      onChange={(event) => update('residual_service_it', event.target.value)}
                    />
                  </label>
                </div>
              </div>
            </details>

            <details className={shared.collapsiblePanel} open>
              <summary className={shared.collapsibleSummary}>
                <span className={shared.collapsibleSummaryLeft}>
                  <Icon name="chevron-right" size={16} className={shared.collapsibleChevron} />
                  <h2 className={shared.sectionTitle}>Servizi e impatti</h2>
                </span>
                <span className={shared.sectionBadge}>Opzionale</span>
              </summary>
              <div className={shared.collapsibleContent}>
                {(() => {
                  const summary = selectionSummary(form);
                  return summary ? (
                    <p className={shared.serviceImpactSummary}>{summary}</p>
                  ) : null;
                })()}
                <ServiceImpactSection
                  operatedOptions={serviceItems}
                  allServices={reference.data.service_taxonomy}
                  targetTypes={reference.data.target_types}
                  selectedDomainId={selectedDomainId}
                  selections={form.service_selections}
                  operatedIds={operatedIds}
                  impactedIds={impactedIds}
                  suggestions={directSuggestions}
                  checkedSuggestions={checkedSuggestions}
                  transitivePreview={transitivePreview}
                  targets={form.manual_targets}
                  targetDraft={targetDraft}
                  onOperatedChange={(ids) => updateServiceRole('operated', ids)}
                  onImpactedChange={(ids) => updateServiceRole('dependent', ids)}
                  onSelectionChange={updateServiceSelection}
                  onSuggestionCheck={(id, checked) =>
                    setCheckedSuggestions((current) => ({ ...current, [id]: checked }))
                  }
                  onAddSuggestions={addCheckedSuggestions}
                  onTargetDraftChange={setTargetDraft}
                  onAddTarget={addManualTarget}
                  onRemoveTarget={(id) =>
                    setForm((current) => ({
                      ...current,
                      manual_targets: current.manual_targets.filter((target) => target.id !== id),
                    }))
                  }
                />
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
        {label} {required && <RequiredMark />}
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

function ServiceImpactSection({
  operatedOptions,
  allServices,
  targetTypes,
  selectedDomainId,
  selections,
  operatedIds,
  impactedIds,
  suggestions,
  checkedSuggestions,
  transitivePreview,
  targets,
  targetDraft,
  onOperatedChange,
  onImpactedChange,
  onSelectionChange,
  onSuggestionCheck,
  onAddSuggestions,
  onTargetDraftChange,
  onAddTarget,
  onRemoveTarget,
}: {
  operatedOptions: ReferenceItem[];
  allServices: ReferenceItem[];
  targetTypes: ReferenceItem[];
  selectedDomainId: number | null;
  selections: ServiceSelection[];
  operatedIds: number[];
  impactedIds: number[];
  suggestions: ServiceDependency[];
  checkedSuggestions: Record<number, boolean>;
  transitivePreview: ReferenceItem[];
  targets: ManualTarget[];
  targetDraft: ManualTarget;
  onOperatedChange: (ids: number[]) => void;
  onImpactedChange: (ids: number[]) => void;
  onSelectionChange: (id: number, patch: Partial<ServiceSelection>) => void;
  onSuggestionCheck: (id: number, checked: boolean) => void;
  onAddSuggestions: () => void;
  onTargetDraftChange: (target: ManualTarget) => void;
  onAddTarget: () => void;
  onRemoveTarget: (id: string) => void;
}) {
  const serviceById = useMemo(() => new Map(allServices.map((service) => [service.id, service])), [allServices]);
  const [expandedRows, setExpandedRows] = useState<Set<number>>(new Set());
  const [targetFormOpen, setTargetFormOpen] = useState(false);

  const operatedSelections = useMemo(
    () => selections.filter((selection) => selection.role === 'operated'),
    [selections],
  );
  const impactedSelections = useMemo(
    () => selections.filter((selection) => selection.role === 'dependent'),
    [selections],
  );

  function toggleRow(id: number) {
    setExpandedRows((current) => {
      const next = new Set(current);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }

  function handleAddTarget() {
    const valid = targetDraft.target_type_id && targetDraft.display_name.trim();
    onAddTarget();
    if (valid) setTargetFormOpen(false);
  }

  function cancelTargetForm() {
    setTargetFormOpen(false);
    onTargetDraftChange({ id: '', target_type_id: 0, display_name: '', service_taxonomy_id: null });
  }

  return (
    <div className={shared.serviceImpactBox}>
      <section className={shared.serviceBlock}>
        <header className={shared.serviceBlockHeader}>
          <h3 className={shared.serviceBlockTitle}>Servizi su cui intervieni</h3>
        </header>
        <MultiSelectField
          label="Aggiungi servizio operato"
          options={toOptions(operatedOptions)}
          selected={operatedIds}
          onChange={onOperatedChange}
        />
        {operatedSelections.length > 0 ? (
          <ul className={shared.compactRowList}>
            {operatedSelections.map((selection) => (
              <CompactSelectionRow
                key={selection.service_taxonomy_id}
                selection={selection}
                service={serviceById.get(selection.service_taxonomy_id)}
                selectedDomainId={selectedDomainId}
                expanded={expandedRows.has(selection.service_taxonomy_id)}
                onToggle={() => toggleRow(selection.service_taxonomy_id)}
                onSelectionChange={onSelectionChange}
              />
            ))}
          </ul>
        ) : (
          <p className={shared.small}>Nessun servizio operato.</p>
        )}
      </section>

      <hr className={shared.serviceBlockDivider} />

      <section className={shared.serviceBlock}>
        <header className={shared.serviceBlockHeader}>
          <h3 className={shared.serviceBlockTitle}>Altri servizi impattati</h3>
        </header>
        <MultiSelectField
          label="Aggiungi servizio impattato"
          options={toOptions(allServices)}
          selected={impactedIds}
          onChange={onImpactedChange}
        />
        {impactedSelections.length > 0 ? (
          <ul className={shared.compactRowList}>
            {impactedSelections.map((selection) => (
              <CompactSelectionRow
                key={selection.service_taxonomy_id}
                selection={selection}
                service={serviceById.get(selection.service_taxonomy_id)}
                selectedDomainId={selectedDomainId}
                expanded={expandedRows.has(selection.service_taxonomy_id)}
                onToggle={() => toggleRow(selection.service_taxonomy_id)}
                onSelectionChange={onSelectionChange}
              />
            ))}
          </ul>
        ) : (
          <p className={shared.small}>Nessun servizio impattato.</p>
        )}
      </section>

      <hr className={shared.serviceBlockDivider} />

      <section className={shared.serviceBlock}>
        <header className={shared.serviceBlockHeader}>
          <h3 className={shared.serviceBlockTitle}>Apparati e oggetti</h3>
        </header>
        {targets.length > 0 ? (
          <div className={shared.chipList}>
            {targets.map((target) => (
              <button
                key={target.id}
                type="button"
                className={shared.filterChip}
                onClick={() => onRemoveTarget(target.id)}
              >
                {target.display_name}
                <Icon name="x" size={14} />
              </button>
            ))}
          </div>
        ) : null}
        {targetFormOpen ? (
          <div className={shared.targetForm}>
            <select
              className={shared.select}
              value={targetDraft.target_type_id || ''}
              onChange={(event) =>
                onTargetDraftChange({ ...targetDraft, target_type_id: Number(event.target.value) })
              }
            >
              <option value="">Tipo target</option>
              {targetTypes.map((item) => (
                <option key={item.id} value={item.id}>
                  {item.name_it}
                </option>
              ))}
            </select>
            <input
              className={shared.field}
              value={targetDraft.display_name}
              onChange={(event) => onTargetDraftChange({ ...targetDraft, display_name: event.target.value })}
              placeholder="Nome target"
            />
            <select
              className={shared.select}
              value={targetDraft.service_taxonomy_id ?? ''}
              onChange={(event) =>
                onTargetDraftChange({
                  ...targetDraft,
                  service_taxonomy_id: event.target.value ? Number(event.target.value) : null,
                })
              }
            >
              <option value="">Servizio collegato (opzionale)</option>
              {allServices.map((item) => (
                <option key={item.id} value={item.id}>
                  {item.name_it}
                </option>
              ))}
            </select>
            <div className={shared.targetFormActions}>
              <Button size="sm" variant="secondary" onClick={cancelTargetForm}>
                Annulla
              </Button>
              <Button size="sm" onClick={handleAddTarget}>
                Aggiungi
              </Button>
            </div>
          </div>
        ) : (
          <Button size="sm" variant="secondary" onClick={() => setTargetFormOpen(true)}>
            <Icon name="plus" size={14} /> Aggiungi target
          </Button>
        )}
      </section>

      {suggestions.length > 0 ? (
        <div className={shared.suggestionPanel}>
          <div className={shared.sectionHeader}>
            <h4 className={shared.sectionTitle}>Suggerimenti</h4>
            <Button size="sm" variant="secondary" onClick={onAddSuggestions}>
              Aggiungi suggeriti
            </Button>
          </div>
          <p className={shared.suggestionDisclaimer}>
            Proposti dal grafo dipendenze — può essere incompleto, verifica.
          </p>
          <div className={shared.suggestionList}>
            {suggestions.map((item) => {
              const tooltip = `${dependencyTypeLabel(item.dependency_type)} · ${audienceLabel(
                item.downstream_service.audience ?? '',
              )}`;
              return (
                <label
                  key={item.service_dependency_id}
                  className={shared.suggestionRowGrid}
                  title={tooltip}
                >
                  <input
                    type="checkbox"
                    checked={checkedSuggestions[item.service_dependency_id] === true}
                    onChange={(event) =>
                      onSuggestionCheck(item.service_dependency_id, event.target.checked)
                    }
                  />
                  <strong>{item.downstream_service.name_it}</strong>
                  <span className={shared.suggestionUpstream}>
                    ← {item.upstream_service.name_it}
                  </span>
                  <span className={shared.suggestionSeverity}>
                    {severityLabel(item.default_severity)}
                  </span>
                </label>
              );
            })}
          </div>
        </div>
      ) : null}

      {transitivePreview.length > 0 ? (
        <details className={shared.indirectDetails}>
          <summary className={shared.indirectSummary}>
            Possibili effetti a catena ({transitivePreview.length})
          </summary>
          <div className={shared.chipList}>
            {transitivePreview.map((service) => (
              <span key={service.id} className={shared.filterChip}>
                {service.name_it}
              </span>
            ))}
          </div>
        </details>
      ) : null}
    </div>
  );
}

function CompactSelectionRow({
  selection,
  service,
  selectedDomainId,
  expanded,
  onToggle,
  onSelectionChange,
}: {
  selection: ServiceSelection;
  service: ReferenceItem | undefined;
  selectedDomainId: number | null;
  expanded: boolean;
  onToggle: () => void;
  onSelectionChange: (id: number, patch: Partial<ServiceSelection>) => void;
}) {
  if (!service) return null;
  const canOperate = !selectedDomainId || service.technical_domain_id === selectedDomainId;
  const showAudience = service.audience === 'maintenance';
  const iconClass =
    selection.role === 'operated' ? shared.iconOperated : shared.iconImpacted;
  const expandedId = `compact-row-expanded-${selection.service_taxonomy_id}`;
  return (
    <li className={shared.compactRow}>
      <div className={shared.compactRowHead}>
        <Icon name="circle" size={10} className={iconClass} aria-hidden="true" />
        <span className={shared.compactRowName}>
          <strong>{service.name_it}</strong>
        </span>
        <select
          className={`${shared.select} ${shared.compactRowSelect}`}
          value={selection.expected_severity}
          onChange={(event) =>
            onSelectionChange(selection.service_taxonomy_id, {
              expected_severity: event.target.value as SeverityValue,
            })
          }
        >
          <option value="none">Nessun impatto</option>
          <option value="degraded">Degradato</option>
          <option value="unavailable">Non disponibile</option>
        </select>
        <button
          type="button"
          className={shared.compactRowToggle}
          aria-expanded={expanded}
          aria-controls={expandedId}
          onClick={onToggle}
          title={expanded ? 'Comprimi' : 'Espandi'}
        >
          <Icon name={expanded ? 'chevron-up' : 'chevron-down'} size={16} />
        </button>
      </div>
      {expanded ? (
        <div id={expandedId} className={shared.compactRowExpanded}>
          <label className={shared.compactRowField}>
            <span className={shared.compactRowFieldLabel}>Ruolo</span>
            <select
              className={shared.select}
              value={selection.role}
              onChange={(event) => {
                if (event.target.value === 'operated' && !canOperate) return;
                onSelectionChange(selection.service_taxonomy_id, {
                  role: event.target.value as ServiceSelection['role'],
                  source: 'manual',
                });
              }}
            >
              <option value="operated" disabled={!canOperate}>Operato</option>
              <option value="dependent">Impattato</option>
            </select>
          </label>
          {showAudience ? (
            <label className={shared.compactRowField}>
              <span className={shared.compactRowFieldLabel}>Audience</span>
              <select
                className={shared.select}
                value={selection.expected_audience ?? ''}
                onChange={(event) =>
                  onSelectionChange(selection.service_taxonomy_id, {
                    expected_audience: (event.target.value || null) as AudienceOverride | null,
                  })
                }
              >
                <option value="">Da definire</option>
                <option value="internal">Interna</option>
                <option value="external">Esterna</option>
                <option value="both">Interna ed esterna</option>
              </select>
            </label>
          ) : null}
        </div>
      ) : null}
    </li>
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

function serviceClassificationInputs(items: ServiceSelection[]): ClassificationInput[] {
  return items.map((item, index) => ({
    reference_id: item.service_taxonomy_id,
    service_taxonomy_id: item.service_taxonomy_id,
    source: item.source,
    confidence: null,
    is_primary: item.role === 'operated' && index === items.findIndex((candidate) => candidate.role === 'operated'),
    role: item.role,
    expected_severity: item.expected_severity,
    expected_audience: item.expected_audience,
  }));
}

function targetInput(item: ManualTarget): TargetBody {
  return {
    target_type_id: item.target_type_id,
    display_name: item.display_name,
    service_taxonomy_id: item.service_taxonomy_id,
    source: 'manual',
    is_primary: false,
  };
}

function mergeServiceSelections(current: ServiceSelection[], incoming: ServiceSelection[]): ServiceSelection[] {
  const byID = new Map<number, ServiceSelection>();
  for (const item of current) byID.set(item.service_taxonomy_id, item);
  for (const item of incoming) {
    const existing = byID.get(item.service_taxonomy_id);
    if (existing?.role === 'operated' && item.role === 'dependent') {
      byID.set(item.service_taxonomy_id, { ...existing, expected_severity: item.expected_severity });
    } else {
      byID.set(item.service_taxonomy_id, { ...existing, ...item });
    }
  }
  return [...byID.values()].sort((a, b) => {
    if (a.role !== b.role) return a.role === 'operated' ? -1 : 1;
    return a.service_taxonomy_id - b.service_taxonomy_id;
  });
}

function dependencyPreview(
  operatedIds: number[],
  dependencies: ServiceDependency[],
  selected: ServiceSelection[],
): ReferenceItem[] {
  const selectedIds = new Set(selected.map((item) => item.service_taxonomy_id));
  const byUpstream = new Map<number, ServiceDependency[]>();
  for (const dependency of dependencies) {
    const rows = byUpstream.get(dependency.upstream_service_id) ?? [];
    rows.push(dependency);
    byUpstream.set(dependency.upstream_service_id, rows);
  }
  const result = new Map<number, ReferenceItem>();
  const visited = new Set<number>(operatedIds);
  const walk = (ids: number[], depth: number) => {
    if (depth > 2) return;
    const next: number[] = [];
    for (const id of ids) {
      for (const dependency of byUpstream.get(id) ?? []) {
        if (visited.has(dependency.downstream_service_id)) continue;
        visited.add(dependency.downstream_service_id);
        next.push(dependency.downstream_service_id);
        if (depth >= 2 && !selectedIds.has(dependency.downstream_service_id)) {
          result.set(dependency.downstream_service_id, dependency.downstream_service);
        }
      }
    }
    if (next.length > 0) walk(next, depth + 1);
  };
  walk(operatedIds, 1);
  return [...result.values()];
}

function defaultSeverityFor(role: ServiceSelection['role']): SeverityValue {
  return role === 'operated' ? 'unavailable' : 'degraded';
}

function getSuggestionDefaultChecked(item: ServiceDependency): boolean {
  return item.dependency_type === 'runs_on' && !item.is_redundant;
}

function selectionSummary(form: FormState): string | null {
  const operatedCount = form.service_selections.filter((item) => item.role === 'operated').length;
  const impactedCount = form.service_selections.filter((item) => item.role === 'dependent').length;
  const targetCount = form.manual_targets.length;
  if (operatedCount === 0 && impactedCount === 0 && targetCount === 0) return null;
  return `${operatedCount} operati · ${impactedCount} impattati · ${targetCount} apparati`;
}
