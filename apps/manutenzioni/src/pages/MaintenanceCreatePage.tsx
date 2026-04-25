import { Button, Icon, MultiSelect, Skeleton, Tooltip, useToast } from '@mrsmith/ui';
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
import { CatalogCombobox } from '../components/CatalogCombobox';
import { ConfirmDialog } from '../components/ConfirmDialog';
import { CreateServiceTaxonomyModal } from '../components/CreateServiceTaxonomyModal';
import { GraphSuggestionsBanner } from '../components/GraphSuggestionsBanner';
import { RequiredMark } from '../components/RequiredMark';
import { SeverityDropdown } from '../components/SeverityDropdown';
import { SiteSelectField } from '../components/SiteSelectField';
import { errorMessage } from '../lib/format';
import { suggestedSeverityForKindId } from '../lib/smartDefaults';
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
  expected_severity: SeverityValue | null;
  severity_confirmed: boolean;
  expected_audience: AudienceOverride | null;
  source: 'manual' | 'dependency_graph' | 'ai_extracted';
}

interface ManualTarget {
  id: string;
  target_type_id: number;
  display_name: string;
  service_taxonomy_id: number;
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
  const [ignoredSuggestions, setIgnoredSuggestions] = useState<Set<number>>(new Set());
  const [createTaxonomyContext, setCreateTaxonomyContext] = useState<{
    role: 'operated' | 'dependent';
    initialName: string;
  } | null>(null);
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
  const operatedIds = useMemo(
    () => form.service_selections.filter((item) => item.role === 'operated').map((item) => item.service_taxonomy_id),
    [form.service_selections],
  );
  const directSuggestions = useMemo(() => {
    const selected = new Set(form.service_selections.map((item) => item.service_taxonomy_id));
    const operated = new Set(operatedIds);
    return (dependencies.data ?? [])
      .filter((item) => operated.has(item.upstream_service_id) && !selected.has(item.downstream_service_id))
      .sort((a, b) => a.downstream_service.name_it.localeCompare(b.downstream_service.name_it));
  }, [dependencies.data, form.service_selections, operatedIds]);

  const kindList = reference.data?.maintenance_kinds ?? [];
  const suggestedSeverity = useMemo(
    () =>
      suggestedSeverityForKindId(
        form.maintenance_kind_id ? Number(form.maintenance_kind_id) : null,
        kindList,
      ),
    [form.maintenance_kind_id, kindList],
  );

  const undefinedSeverityCount = useMemo(
    () =>
      form.service_selections.filter((item) => !item.severity_confirmed || item.expected_severity === null)
        .length,
    [form.service_selections],
  );

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
    setForm((current) => {
      const existing = new Set(current.service_selections.map((item) => item.service_taxonomy_id));
      const incoming: ServiceSelection[] = response.service_taxonomy_ids
        .filter((id) => !existing.has(id))
        .map((id) => ({
          service_taxonomy_id: id,
          role: 'operated' as const,
          expected_severity: 'unavailable' as const,
          severity_confirmed: true,
          expected_audience: null,
          source: 'ai_extracted' as const,
        }));
      return {
        ...current,
        summary_it: response.texts.title_it?.trim() || current.summary_it,
        service_selections:
          incoming.length > 0
            ? [...current.service_selections, ...incoming]
            : current.service_selections,
        reason_class_ids:
          response.reason_class_ids.length > 0
            ? response.reason_class_ids
            : current.reason_class_ids,
        impact_effect_ids:
          response.impact_effect_ids.length > 0
            ? response.impact_effect_ids
            : current.impact_effect_ids,
      };
    });
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

  function addServiceSelection(item: ReferenceItem, role: ServiceSelection['role']) {
    if (role === 'operated' && !canOperateService(item.id)) {
      toast('Solo le voci del dominio tecnico possono essere operate.', 'error');
      return;
    }
    setForm((current) => {
      if (current.service_selections.some((sel) => sel.service_taxonomy_id === item.id)) {
        toast(`"${item.name_it}" è già stato aggiunto.`, 'warning');
        return current;
      }
      const next: ServiceSelection = {
        service_taxonomy_id: item.id,
        role,
        expected_severity: suggestedSeverity,
        severity_confirmed: false,
        expected_audience: null,
        source: 'manual',
      };
      return {
        ...current,
        service_selections: [...current.service_selections, next],
      };
    });
  }

  function removeServiceSelection(id: number) {
    setForm((current) => ({
      ...current,
      service_selections: current.service_selections.filter((item) => item.service_taxonomy_id !== id),
      manual_targets: current.manual_targets.filter((target) => target.service_taxonomy_id !== id),
    }));
  }

  function setSeverity(id: number, value: SeverityValue | null) {
    setForm((current) => ({
      ...current,
      service_selections: current.service_selections.map((item) =>
        item.service_taxonomy_id === id
          ? { ...item, expected_severity: value, severity_confirmed: value !== null }
          : item,
      ),
    }));
  }

  function confirmSuggestedSeverity(id: number) {
    setForm((current) => ({
      ...current,
      service_selections: current.service_selections.map((item) =>
        item.service_taxonomy_id === id ? { ...item, severity_confirmed: true } : item,
      ),
    }));
  }

  function setAudience(id: number, value: AudienceOverride | null) {
    setForm((current) => ({
      ...current,
      service_selections: current.service_selections.map((item) =>
        item.service_taxonomy_id === id ? { ...item, expected_audience: value } : item,
      ),
    }));
  }

  function addInstance(serviceTaxonomyId: number, displayName: string) {
    const trimmed = displayName.trim();
    if (!trimmed) {
      toast('Inserisci il nome dell\'istanza.', 'error');
      return;
    }
    const service = reference.data?.service_taxonomy.find((item) => item.id === serviceTaxonomyId);
    if (!service?.target_type_id) {
      toast('Voce di catalogo senza natura definita: impossibile creare l\'istanza.', 'error');
      return;
    }
    setForm((current) => ({
      ...current,
      manual_targets: [
        ...current.manual_targets,
        {
          id: crypto.randomUUID(),
          service_taxonomy_id: serviceTaxonomyId,
          target_type_id: service.target_type_id ?? 0,
          display_name: trimmed,
        },
      ],
    }));
  }

  function removeInstance(targetId: string) {
    setForm((current) => ({
      ...current,
      manual_targets: current.manual_targets.filter((target) => target.id !== targetId),
    }));
  }

  function canOperateService(id: number): boolean {
    if (!selectedDomainId) return true;
    const service = reference.data?.service_taxonomy.find((item) => item.id === id);
    return service?.technical_domain_id === selectedDomainId;
  }

  function ignoreSuggestion(dependencyId: number) {
    setIgnoredSuggestions((current) => {
      const next = new Set(current);
      next.add(dependencyId);
      return next;
    });
  }

  function acceptSuggestions(items: ServiceDependency[]) {
    setForm((current) => {
      const existing = new Set(current.service_selections.map((item) => item.service_taxonomy_id));
      const incoming: ServiceSelection[] = items
        .filter((item) => !existing.has(item.downstream_service_id))
        .map((item) => ({
          service_taxonomy_id: item.downstream_service_id,
          role: 'dependent' as const,
          expected_severity: item.default_severity,
          severity_confirmed: false,
          expected_audience: null,
          source: 'dependency_graph' as const,
        }));
      return {
        ...current,
        service_selections: [...current.service_selections, ...incoming],
      };
    });
  }

  function handleTaxonomyCreated(item: ReferenceItem) {
    if (createTaxonomyContext) {
      addServiceSelection(item, createTaxonomyContext.role);
    }
    setCreateTaxonomyContext(null);
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
            Compila i campi essenziali. I testi potranno essere affinati prima dell'approvazione.
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

            <details className={shared.collapsiblePanel} open>
              <summary className={shared.collapsibleSummary}>
                <span className={shared.collapsibleSummaryLeft}>
                  <Icon name="chevron-right" size={16} className={shared.collapsibleChevron} />
                  <h2 className={shared.sectionTitle}>Dettagli tecnici della manutenzione</h2>
                  <SectionStatusIcon
                    gapCount={undefinedSeverityCount}
                    gaps={selectionGapList(form, reference.data.service_taxonomy)}
                  />
                </span>
              </summary>
              <div className={shared.collapsibleContent}>
                {(() => {
                  const summary = selectionSummary(form);
                  return summary ? (
                    <p className={shared.serviceImpactSummary}>{summary}</p>
                  ) : null;
                })()}
                <ServiceImpactSection
                  serviceCatalog={reference.data.service_taxonomy}
                  selectedDomainId={selectedDomainId}
                  selections={form.service_selections}
                  manualTargets={form.manual_targets}
                  suggestions={directSuggestions}
                  ignoredSuggestionIds={ignoredSuggestions}
                  onAddCatalog={addServiceSelection}
                  onCreateRequest={(initialName, role) =>
                    setCreateTaxonomyContext({ initialName, role })
                  }
                  onRemoveSelection={removeServiceSelection}
                  onSetSeverity={setSeverity}
                  onConfirmSeverity={confirmSuggestedSeverity}
                  onSetAudience={setAudience}
                  onAddInstance={addInstance}
                  onRemoveInstance={removeInstance}
                  onIgnoreSuggestion={ignoreSuggestion}
                  onAcceptSuggestions={acceptSuggestions}
                />
              </div>
            </details>

            <details className={shared.collapsiblePanel}>
              <summary className={shared.collapsibleSummary}>
                <span className={shared.collapsibleSummaryLeft}>
                  <Icon name="chevron-right" size={16} className={shared.collapsibleChevron} />
                  <h2 className={shared.sectionTitle}>Prima finestra</h2>
                </span>
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

          <CreateServiceTaxonomyModal
            open={createTaxonomyContext !== null}
            initialName={createTaxonomyContext?.initialName ?? ''}
            initialDomainId={selectedDomainId}
            domains={reference.data.technical_domains}
            targetTypes={reference.data.target_types}
            onClose={() => setCreateTaxonomyContext(null)}
            onCreated={handleTaxonomyCreated}
          />
        </>
      )}
    </section>
  );
}

function SectionStatusIcon({ gapCount, gaps }: { gapCount: number; gaps: string[] }) {
  if (gapCount === 0) {
    return (
      <Tooltip content="Pronto per la pubblicazione" placement="top">
        <span className={shared.sectionStatusOk} aria-label="Sezione completa">
          <Icon name="check-circle" size={14} />
        </span>
      </Tooltip>
    );
  }
  const tooltip =
    gaps.length > 0
      ? `${gapCount} controlli mancanti:\n• ${gaps.slice(0, 5).join('\n• ')}${gaps.length > 5 ? `\n• …e altri ${gaps.length - 5}` : ''}`
      : `${gapCount} controlli mancanti`;
  return (
    <Tooltip content={tooltip} placement="top">
      <span
        className={shared.sectionStatusWarn}
        aria-label={`${gapCount} controlli mancanti per la pubblicazione`}
      >
        <Icon name="triangle-alert" size={14} />
        <span>{gapCount}</span>
      </span>
    </Tooltip>
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

interface ServiceImpactProps {
  serviceCatalog: ReferenceItem[];
  selectedDomainId: number | null;
  selections: ServiceSelection[];
  manualTargets: ManualTarget[];
  suggestions: ServiceDependency[];
  ignoredSuggestionIds: Set<number>;
  onAddCatalog: (item: ReferenceItem, role: ServiceSelection['role']) => void;
  onCreateRequest: (initialName: string, role: ServiceSelection['role']) => void;
  onRemoveSelection: (id: number) => void;
  onSetSeverity: (id: number, value: SeverityValue | null) => void;
  onConfirmSeverity: (id: number) => void;
  onSetAudience: (id: number, value: AudienceOverride | null) => void;
  onAddInstance: (serviceTaxonomyId: number, displayName: string) => void;
  onRemoveInstance: (targetId: string) => void;
  onIgnoreSuggestion: (dependencyId: number) => void;
  onAcceptSuggestions: (items: ServiceDependency[]) => void;
}

function ServiceImpactSection({
  serviceCatalog,
  selectedDomainId,
  selections,
  manualTargets,
  suggestions,
  ignoredSuggestionIds,
  onAddCatalog,
  onCreateRequest,
  onRemoveSelection,
  onSetSeverity,
  onConfirmSeverity,
  onSetAudience,
  onAddInstance,
  onRemoveInstance,
  onIgnoreSuggestion,
  onAcceptSuggestions,
}: ServiceImpactProps) {
  const serviceById = useMemo(
    () => new Map(serviceCatalog.map((service) => [service.id, service])),
    [serviceCatalog],
  );

  const operatedSelections = useMemo(
    () => selections.filter((selection) => selection.role === 'operated'),
    [selections],
  );
  const impactedSelections = useMemo(
    () => selections.filter((selection) => selection.role === 'dependent'),
    [selections],
  );

  const operatedCatalog = useMemo(
    () =>
      selectedDomainId
        ? serviceCatalog.filter((item) => item.technical_domain_id === selectedDomainId)
        : serviceCatalog,
    [selectedDomainId, serviceCatalog],
  );

  const excludedIds = useMemo(
    () => new Set(selections.map((item) => item.service_taxonomy_id)),
    [selections],
  );

  const instancesBySelection = useMemo(() => {
    const map = new Map<number, ManualTarget[]>();
    for (const target of manualTargets) {
      const list = map.get(target.service_taxonomy_id) ?? [];
      list.push(target);
      map.set(target.service_taxonomy_id, list);
    }
    return map;
  }, [manualTargets]);

  return (
    <div className={shared.serviceImpactBox}>
      <section className={shared.serviceBlock}>
        <header className={shared.serviceBlockHeader}>
          <h3 className={shared.serviceBlockTitle}>In manutenzione</h3>
          <p className={shared.serviceBlockSublabel}>
            Gli oggetti su cui interviene questa finestra
          </p>
        </header>
        <CatalogCombobox
          options={operatedCatalog}
          excludedIds={excludedIds}
          domainHintId={selectedDomainId}
          placeholder="Cerca o crea voce di catalogo…"
          onSelect={(item) => onAddCatalog(item, 'operated')}
          onCreateRequest={(name) => onCreateRequest(name, 'operated')}
        />
        {operatedSelections.length > 0 ? (
          <ul className={shared.compactRowList}>
            {operatedSelections.map((selection) => (
              <CompactSelectionRow
                key={selection.service_taxonomy_id}
                selection={selection}
                service={serviceById.get(selection.service_taxonomy_id)}
                instances={instancesBySelection.get(selection.service_taxonomy_id) ?? []}
                onSetSeverity={onSetSeverity}
                onConfirmSeverity={onConfirmSeverity}
                onSetAudience={onSetAudience}
                onAddInstance={onAddInstance}
                onRemoveInstance={onRemoveInstance}
                onRemove={onRemoveSelection}
              />
            ))}
          </ul>
        ) : (
          <p className={shared.small}>Nessuna voce di catalogo aggiunta.</p>
        )}
      </section>

      <hr className={shared.serviceBlockDivider} />

      <section className={shared.serviceBlock}>
        <header className={shared.serviceBlockHeader}>
          <h3 className={shared.serviceBlockTitle}>Effetti su altri sistemi</h3>
          <p className={shared.serviceBlockSublabel}>
            Altri oggetti che subiscono impatto anche se non vengono operati direttamente
          </p>
        </header>
        <GraphSuggestionsBanner
          suggestions={suggestions}
          ignoredIds={ignoredSuggestionIds}
          onIgnore={onIgnoreSuggestion}
          onAccept={onAcceptSuggestions}
        />
        <CatalogCombobox
          options={serviceCatalog}
          excludedIds={excludedIds}
          domainHintId={selectedDomainId}
          placeholder="Cerca o crea voce di catalogo…"
          onSelect={(item) => onAddCatalog(item, 'dependent')}
          onCreateRequest={(name) => onCreateRequest(name, 'dependent')}
        />
        {impactedSelections.length > 0 ? (
          <ul className={shared.compactRowList}>
            {impactedSelections.map((selection) => (
              <CompactSelectionRow
                key={selection.service_taxonomy_id}
                selection={selection}
                service={serviceById.get(selection.service_taxonomy_id)}
                instances={instancesBySelection.get(selection.service_taxonomy_id) ?? []}
                onSetSeverity={onSetSeverity}
                onConfirmSeverity={onConfirmSeverity}
                onSetAudience={onSetAudience}
                onAddInstance={onAddInstance}
                onRemoveInstance={onRemoveInstance}
                onRemove={onRemoveSelection}
              />
            ))}
          </ul>
        ) : (
          <p className={shared.small}>Nessun effetto su altri sistemi.</p>
        )}
      </section>
    </div>
  );
}

interface CompactSelectionRowProps {
  selection: ServiceSelection;
  service: ReferenceItem | undefined;
  instances: ManualTarget[];
  onSetSeverity: (id: number, value: SeverityValue | null) => void;
  onConfirmSeverity: (id: number) => void;
  onSetAudience: (id: number, value: AudienceOverride | null) => void;
  onAddInstance: (serviceTaxonomyId: number, displayName: string) => void;
  onRemoveInstance: (targetId: string) => void;
  onRemove: (id: number) => void;
}

function CompactSelectionRow({
  selection,
  service,
  instances,
  onSetSeverity,
  onConfirmSeverity,
  onSetAudience,
  onAddInstance,
  onRemoveInstance,
  onRemove,
}: CompactSelectionRowProps) {
  const [instanceDraft, setInstanceDraft] = useState('');
  if (!service) return null;
  const showAudience = service.audience === 'maintenance';
  const isSuggested = !selection.severity_confirmed && selection.expected_severity !== null;
  const fromGraph = selection.source === 'dependency_graph';

  function commitInstance() {
    const value = instanceDraft.trim();
    if (!value) return;
    onAddInstance(selection.service_taxonomy_id, value);
    setInstanceDraft('');
  }

  return (
    <li className={shared.compactRow}>
      <div className={shared.compactRowHead}>
        <span className={shared.compactRowName}>
          <strong>{service.name_it}</strong>
          {service.target_type_name ? (
            <span className={shared.compactRowChip}>{service.target_type_name}</span>
          ) : null}
          {fromGraph ? <span className={shared.compactRowFromGraph}>dal grafo</span> : null}
        </span>
        <span className={shared.compactRowSeverity}>
          <span className={shared.compactRowSeverityLabel}>Impatto atteso</span>
          <SeverityDropdown
            value={selection.expected_severity}
            onChange={(value) => onSetSeverity(selection.service_taxonomy_id, value)}
            isSuggested={isSuggested}
            onConfirmSuggested={() => onConfirmSeverity(selection.service_taxonomy_id)}
          />
        </span>
        <button
          type="button"
          className={shared.compactRowRemove}
          onClick={() => onRemove(selection.service_taxonomy_id)}
          title="Rimuovi"
          aria-label="Rimuovi questa voce"
        >
          <Icon name="x" size={14} />
        </button>
      </div>
      <div className={shared.compactRowInstances}>
        <span className={shared.compactRowInstancesLabel}>Istanze:</span>
        {instances.length > 0 ? (
          <ul className={shared.compactRowInstanceList}>
            {instances.map((instance) => (
              <li key={instance.id} className={shared.compactRowInstanceItem}>
                <span className={shared.compactRowInstanceTree}>└</span>
                <span>{instance.display_name}</span>
                <button
                  type="button"
                  className={shared.compactRowInstanceRemove}
                  onClick={() => onRemoveInstance(instance.id)}
                  title="Rimuovi istanza"
                  aria-label="Rimuovi istanza"
                >
                  <Icon name="x" size={12} />
                </button>
              </li>
            ))}
          </ul>
        ) : null}
        <div className={shared.compactRowInstanceAdd}>
          <input
            className={shared.compactRowInstanceInput}
            value={instanceDraft}
            onChange={(event) => setInstanceDraft(event.target.value)}
            onKeyDown={(event) => {
              if (event.key === 'Enter') {
                event.preventDefault();
                commitInstance();
              }
            }}
            placeholder="+ aggiungi istanza"
          />
          {instanceDraft.trim() ? (
            <button
              type="button"
              className={shared.compactRowInstanceConfirm}
              onClick={commitInstance}
              title="Aggiungi istanza"
            >
              <Icon name="check" size={12} />
            </button>
          ) : null}
        </div>
      </div>
      {showAudience ? (
        <div className={shared.compactRowAudience}>
          <label className={shared.compactRowField}>
            <span className={shared.compactRowFieldLabel}>Audience</span>
            <select
              className={shared.select}
              value={selection.expected_audience ?? ''}
              onChange={(event) =>
                onSetAudience(
                  selection.service_taxonomy_id,
                  (event.target.value || null) as AudienceOverride | null,
                )
              }
            >
              <option value="">Da definire</option>
              <option value="internal">Interna</option>
              <option value="external">Esterna</option>
              <option value="both">Interna ed esterna</option>
            </select>
          </label>
        </div>
      ) : null}
    </li>
  );
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
  return items.map((item, index) => {
    const input: ClassificationInput = {
      reference_id: item.service_taxonomy_id,
      service_taxonomy_id: item.service_taxonomy_id,
      source: item.source,
      confidence: null,
      is_primary:
        item.role === 'operated' &&
        index === items.findIndex((candidate) => candidate.role === 'operated'),
      role: item.role,
      expected_audience: item.expected_audience,
    };
    if (item.severity_confirmed && item.expected_severity !== null) {
      input.expected_severity = item.expected_severity;
    }
    return input;
  });
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

function selectionSummary(form: FormState): string | null {
  const operatedCount = form.service_selections.filter((item) => item.role === 'operated').length;
  const impactedCount = form.service_selections.filter((item) => item.role === 'dependent').length;
  const instancesCount = form.manual_targets.length;
  if (operatedCount === 0 && impactedCount === 0) return null;
  const operatedLabel =
    instancesCount > 0
      ? `${operatedCount} in manutenzione (${instancesCount} ${instancesCount === 1 ? 'istanza' : 'istanze'})`
      : `${operatedCount} ${operatedCount === 1 ? 'in manutenzione' : 'in manutenzione'}`;
  return `${operatedLabel} · ${impactedCount} ${impactedCount === 1 ? 'effetto su altri sistemi' : 'effetti su altri sistemi'}`;
}

function selectionGapList(form: FormState, services: ReferenceItem[]): string[] {
  const byId = new Map(services.map((service) => [service.id, service]));
  return form.service_selections
    .filter((item) => !item.severity_confirmed || item.expected_severity === null)
    .map((item) => {
      const service = byId.get(item.service_taxonomy_id);
      const name = service?.name_it ?? `Voce #${item.service_taxonomy_id}`;
      return `Severità non dichiarata: ${name}`;
    });
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
