import { Button, Icon, Modal, Skeleton, TabNav, type TabNavItem, useToast } from '@mrsmith/ui';
import { hasAnyRole } from '@mrsmith/auth-client';
import { useState, type ReactNode } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import {
  useCustomerImpactMutations,
  useCustomerSearch,
  useMaintenance,
  useMaintenanceCockpit,
  useMaintenanceAssistanceDraft,
  useNoticeMutations,
  useReferenceData,
  useReplaceClassifications,
  useStatusAction,
  useTargetMutations,
  useUpdateMaintenance,
  useWindowMutations,
} from '../api/queries';
import type {
  AdhocSiteInput,
  ClassificationInput,
  AssistanceClassificationProposal,
  MaintenanceAssistanceDraft,
  ImpactedCustomerBody,
  JsonObject,
  MaintenanceDetail,
  MaintenanceWindow,
  NoticeBody,
  ReferenceItem,
  ReferenceData,
  TargetBody,
  WindowBody,
} from '../api/types';
import { ConfirmDialog } from '../components/ConfirmDialog';
import { MaintenanceCockpit } from '../components/MaintenanceCockpit';
import { SiteSelectField } from '../components/SiteSelectField';
import { StatusPill, statusTone } from '../components/StatusPill';
import { useOptionalAuth } from '../hooks/useOptionalAuth';
import {
  audienceLabel,
  confidenceLabel,
  errorMessage,
  formatDateInput,
  formatDateTime,
  impactScopeLabel,
  minutesLabel,
  noticeTypeLabel,
  parsePositiveId,
  sendStatusLabel,
  serviceRoleLabel,
  severityLabel,
  sourceLabel,
  statusLabel,
  windowStatusLabel,
} from '../lib/format';
import { MANUTENZIONI_APPROVER_ROLES, MANUTENZIONI_MANAGER_ROLES } from '../lib/roles';
import { SEVERITY_OPTIONS } from '../lib/severity';
import { validateWindowTiming, windowDurationMinutes } from '../lib/windowValidation';
import shared from './shared.module.css';

type TabKey = 'cockpit' | 'riepilogo' | 'finestre' | 'impatto' | 'target' | 'clienti' | 'comunicazioni' | 'storico';

const tabs: TabNavItem[] = [
  { key: 'cockpit', label: 'Cruscotto' },
  { key: 'riepilogo', label: 'Riepilogo' },
  { key: 'finestre', label: 'Finestre' },
  { key: 'impatto', label: 'Impatto' },
  { key: 'target', label: 'Target' },
  { key: 'clienti', label: 'Clienti' },
  { key: 'comunicazioni', label: 'Comunicazioni' },
  { key: 'storico', label: 'Storico' },
];

export function MaintenanceDetailPage() {
  const navigate = useNavigate();
  const params = useParams();
  const id = parsePositiveId(params.id);
  const { user } = useOptionalAuth();
  const canManage = hasAnyRole(user?.roles, MANUTENZIONI_MANAGER_ROLES);
  const canApprove = hasAnyRole(user?.roles, MANUTENZIONI_APPROVER_ROLES);
  const [activeTab, setActiveTab] = useState<TabKey>('cockpit');
  const detail = useMaintenance(id);
  const cockpit = useMaintenanceCockpit(id);
  const reference = useReferenceData(id ?? undefined);
  const statusAction = useStatusAction();
  const toast = useToast();

  if (id === null) {
    return (
      <section className={shared.emptyCard}>
        <div className={shared.emptyIconDanger}>
          <Icon name="triangle-alert" />
        </div>
        <h3>Indirizzo non valido</h3>
        <p>Il dettaglio richiesto non può essere aperto.</p>
      </section>
    );
  }

  async function runStatus(action: string) {
    if (!id) return;
    const reason_it =
      action === 'cancel' ? window.prompt('Motivo annullamento')?.trim() : undefined;
    if (action === 'cancel' && !reason_it) return;
    try {
      await statusAction.mutateAsync({ id, action, reason_it });
      toast.toast('Stato aggiornato.');
    } catch (error) {
      toast.toast(errorMessage(error, 'Cambio stato non riuscito.'), 'error');
    }
  }

  const data = detail.data;
  const statusButtons = data
    ? lifecycleButtons(data.status, canManage, canApprove).map((item) => (
        <Button
          key={item.action}
          size="sm"
          variant={item.action === 'cancel' ? 'danger' : 'secondary'}
          onClick={() => runStatus(item.action)}
          loading={statusAction.isPending}
        >
          {item.label}
        </Button>
      ))
    : null;

  function changeTab(key: string) {
    if (tabs.some((item) => item.key === key)) {
      setActiveTab(key as TabKey);
    }
  }

  return (
    <section className={shared.page}>
      <button type="button" className={shared.backLink} onClick={() => navigate('/manutenzioni')}>
        <Icon name="chevron-left" size={16} />
        Torna al registro
      </button>

      {detail.isLoading ? (
        <div className={shared.panel}>
          <Skeleton rows={8} />
        </div>
      ) : detail.error || !data ? (
        <div className={shared.emptyCard}>
          <div className={shared.emptyIconDanger}>
            <Icon name="triangle-alert" />
          </div>
          <h3>Dettaglio non disponibile</h3>
          <p>{errorMessage(detail.error, 'Impossibile caricare la manutenzione.')}</p>
        </div>
      ) : (
        <>
          <div className={shared.detailShell}>
            <div className={shared.detailShellTop}>
              <div className={shared.titleBlock}>
                <div className={shared.detailIdentity}>
                  <span className={shared.detailCode}>{data.code || `MNT #${data.maintenance_id}`}</span>
                  <h1 className={shared.pageTitle}>{data.title_it}</h1>
                </div>
                <div className={shared.detailMeta} aria-label="Metadati manutenzione">
                  <span>{data.technical_domain.name_it}</span>
                  <span>{data.maintenance_kind.name_it}</span>
                  <span>{customerScopeLabel(data.customer_scope)}</span>
                  {data.site ? <span>{data.site.name_it}</span> : null}
                </div>
              </div>
              <div className={shared.headerActions}>
                <StatusPill tone={statusTone(data.status)}>{statusLabel(data.status)}</StatusPill>
                {statusButtons}
              </div>
            </div>

            <div className={shared.detailShellTabs}>
              <div className={shared.tabScroller}>
                <TabNav items={tabs} activeKey={activeTab} onTabChange={(key) => setActiveTab(key as TabKey)} />
              </div>
            </div>
          </div>

          <div className={shared.tabsSpacer}>
            {activeTab === 'cockpit' && (
              <MaintenanceCockpit
                detail={data}
                cockpit={cockpit.data}
                isLoading={cockpit.isLoading}
                error={cockpit.error}
                onTabChange={changeTab}
              />
            )}
            {activeTab === 'riepilogo' && (
              <SummaryTab
                detail={data}
                reference={reference.data}
                canManage={canManage}
              />
            )}
            {activeTab === 'finestre' && (
              <WindowsTab detail={data} canManage={canManage} />
            )}
            {activeTab === 'impatto' && (
              <ImpactTab detail={data} reference={reference.data} canManage={canManage} />
            )}
            {activeTab === 'target' && (
              <TargetsTab detail={data} reference={reference.data} canManage={canManage} />
            )}
            {activeTab === 'clienti' && <CustomersTab detail={data} canManage={canManage} />}
            {activeTab === 'comunicazioni' && (
              <NoticesTab detail={data} reference={reference.data} canManage={canManage} />
            )}
            {activeTab === 'storico' && <EventsTab detail={data} />}
          </div>
        </>
      )}
    </section>
  );
}

function lifecycleButtons(status: string, canManage: boolean, canApprove: boolean) {
  const items: Array<{ action: string; label: string }> = [];
  if (status === 'draft' && canApprove) items.push({ action: 'approve', label: 'Approva' });
  if ((status === 'approved' || status === 'announced') && canManage) {
    items.push({ action: 'schedule', label: 'Pianifica' });
  }
  if ((status === 'approved' || status === 'scheduled') && canManage) {
    items.push({ action: 'announce', label: 'Annuncia' });
  }
  if ((status === 'scheduled' || status === 'announced') && canManage) {
    items.push({ action: 'start', label: 'Avvia' });
  }
  if (status === 'in_progress' && canManage) items.push({ action: 'complete', label: 'Completa' });
  if (['draft', 'approved', 'scheduled', 'announced'].includes(status) && canManage) {
    items.push({ action: 'cancel', label: 'Annulla' });
  }
  return items;
}

function SummaryTab({
  detail,
  reference,
  canManage,
}: {
  detail: MaintenanceDetail;
  reference?: ReferenceData;
  canManage: boolean;
}) {
  const update = useUpdateMaintenance();
  const assistance = useMaintenanceAssistanceDraft(detail.maintenance_id);
  const serviceTaxonomyMutation = useReplaceClassifications(detail.maintenance_id, 'service-taxonomy');
  const reasonClassesMutation = useReplaceClassifications(detail.maintenance_id, 'reason-classes');
  const impactEffectsMutation = useReplaceClassifications(detail.maintenance_id, 'impact-effects');
  const qualityFlagsMutation = useReplaceClassifications(detail.maintenance_id, 'quality-flags');
  const toast = useToast();
  const [editing, setEditing] = useState(false);
  const [assistanceNote, setAssistanceNote] = useState('');
  const [assistanceDraft, setAssistanceDraft] = useState<MaintenanceAssistanceDraft | null>(null);
  const [form, setForm] = useState(() => ({
    title_it: detail.title_it,
    title_en: detail.title_en ?? '',
    description_it: detail.description_it ?? '',
    description_en: detail.description_en ?? '',
    maintenance_kind_id: String(detail.maintenance_kind.id),
    technical_domain_id: String(detail.technical_domain.id),
    customer_scope_id: detail.customer_scope ? String(detail.customer_scope.id) : '',
    site_id: detail.site ? String(detail.site.id) : '',
    adhoc_site: null as AdhocSiteInput | null,
    reason_it: detail.reason_it ?? '',
    residual_service_it: detail.residual_service_it ?? '',
  }));

  async function save() {
    if (form.adhoc_site && !form.adhoc_site.name.trim()) {
      toast.toast('Indica il nome del sito ad-hoc.', 'error');
      return;
    }
    const sitePatch = buildSitePatch({
      previousSiteId: detail.site?.id ?? null,
      siteIdRaw: form.site_id,
      adhocSite: form.adhoc_site,
    });
    const customerScopePatch = buildCustomerScopePatch({
      status: detail.status,
      previousCustomerScopeId: detail.customer_scope?.id ?? null,
      customerScopeIdRaw: form.customer_scope_id,
    });
    if (customerScopePatch === null) {
      toast.toast("Definisci l'ambito clienti prima di continuare.", 'error');
      return;
    }
    try {
      await update.mutateAsync({
        id: detail.maintenance_id,
        body: {
          title_it: form.title_it,
          title_en: form.title_en || null,
          description_it: form.description_it || null,
          description_en: form.description_en || null,
          maintenance_kind_id: Number(form.maintenance_kind_id),
          technical_domain_id: Number(form.technical_domain_id),
          ...customerScopePatch,
          ...sitePatch,
          reason_it: form.reason_it || null,
          residual_service_it: form.residual_service_it || null,
        },
      });
      setEditing(false);
      toast.toast('Riepilogo aggiornato.');
    } catch (error) {
      toast.toast(errorMessage(error, 'Salvataggio non riuscito.'), 'error');
    }
  }

  async function generateAssistance() {
    try {
      const result = await assistance.mutateAsync({
        regenerate: true,
        note: assistanceNote.trim() || null,
      });
      setAssistanceDraft(result);
      toast.toast('Proposte generate.');
    } catch (error) {
      toast.toast(errorMessage(error, 'Assistenza non disponibile.'), 'error');
    }
  }

  async function applyAssistance() {
    if (!assistanceDraft) return;
    const texts = assistanceDraft.texts;
    const nextTitleIT = texts.title_it?.trim() || detail.title_it;
    const nextTitleEN = proposedOrCurrent(texts.title_en, detail.title_en ?? null);
    const nextDescriptionIT = proposedOrCurrent(texts.description_it, detail.description_it ?? null);
    const nextDescriptionEN = proposedOrCurrent(texts.description_en, detail.description_en ?? null);
    const nextReasonEN = proposedOrCurrent(texts.reason_en, detail.reason_en ?? null);
    const nextResidualServiceEN = proposedOrCurrent(texts.residual_service_en, detail.residual_service_en ?? null);
    const approvedAt = new Date().toISOString();
    try {
      await update.mutateAsync({
        id: detail.maintenance_id,
        body: {
          title_it: nextTitleIT,
          title_en: nextTitleEN,
          description_it: nextDescriptionIT,
          description_en: nextDescriptionEN,
          reason_en: nextReasonEN,
          residual_service_en: nextResidualServiceEN,
          metadata: mergeMetadata(detail.metadata, {
            assistance: {
              last_approved_at: approvedAt,
              last_audit: {
                generated_at: assistanceDraft.audit.generated_at,
                model: assistanceDraft.audit.model,
                summary: assistanceDraft.audit.summary,
              },
              approved_texts: assistanceTextsMetadata(texts),
            },
          }),
        },
      });
      if (assistanceDraft.service_taxonomy.length > 0) {
        await serviceTaxonomyMutation.mutateAsync(assistanceInputs(assistanceDraft.service_taxonomy));
      }
      if (assistanceDraft.reason_classes.length > 0) {
        await reasonClassesMutation.mutateAsync(assistanceInputs(assistanceDraft.reason_classes));
      }
      if (assistanceDraft.impact_effects.length > 0) {
        await impactEffectsMutation.mutateAsync(assistanceInputs(assistanceDraft.impact_effects));
      }
      if (assistanceDraft.quality_flags.length > 0) {
        await qualityFlagsMutation.mutateAsync(assistanceInputs(assistanceDraft.quality_flags));
      }
      setForm((current) => ({
        ...current,
        title_it: nextTitleIT,
        title_en: nextTitleEN ?? '',
        description_it: nextDescriptionIT ?? '',
        description_en: nextDescriptionEN ?? '',
      }));
      setAssistanceDraft(null);
      setEditing(false);
      toast.toast('Proposte applicate.');
    } catch (error) {
      toast.toast(errorMessage(error, 'Applicazione non riuscita.'), 'error');
    }
  }

  const applyPending =
    update.isPending ||
    serviceTaxonomyMutation.isPending ||
    reasonClassesMutation.isPending ||
    impactEffectsMutation.isPending ||
    qualityFlagsMutation.isPending;

  return (
    <div className={shared.panel}>
      <div className={shared.sectionHeader}>
        <h2 className={shared.sectionTitle}>Riepilogo</h2>
        {canManage && (
          <div className={shared.inlineActions}>
            {!editing && (
              <Button
                size="sm"
                variant="secondary"
                onClick={generateAssistance}
                loading={assistance.isPending}
              >
                Genera testi e classificazione
              </Button>
            )}
            <Button size="sm" variant="secondary" onClick={() => setEditing((value) => !value)}>
              {editing ? 'Chiudi' : 'Modifica'}
            </Button>
          </div>
        )}
      </div>
      {editing && reference ? (
        <>
          <div className={shared.formGrid}>
            <label className={shared.label}>
              Titolo
              <input
                className={shared.field}
                value={form.title_it}
                onChange={(event) => setForm({ ...form, title_it: event.target.value })}
              />
            </label>
            <label className={shared.label}>
              Titolo inglese
              <input
                className={shared.field}
                value={form.title_en}
                onChange={(event) => setForm({ ...form, title_en: event.target.value })}
              />
            </label>
            <SelectField
              label="Tipo"
              value={form.maintenance_kind_id}
              items={reference.maintenance_kinds}
              onChange={(value) => setForm({ ...form, maintenance_kind_id: value })}
            />
            <SelectField
              label="Dominio"
              value={form.technical_domain_id}
              items={reference.technical_domains}
              onChange={(value) => setForm({ ...form, technical_domain_id: value })}
            />
            <SelectField
              label="Ambito clienti"
              value={form.customer_scope_id}
              items={reference.customer_scopes}
              emptyLabel="Da definire"
              includeEmpty={canClearCustomerScope(detail.status)}
              onChange={(value) => setForm({ ...form, customer_scope_id: value })}
            />
            <SiteSelectField
              sites={reference.sites}
              currentScope={detail.site?.scope ?? null}
              value={{
                site_id: form.site_id ? Number(form.site_id) : null,
                adhoc_site: form.adhoc_site,
              }}
              onChange={(next) =>
                setForm({
                  ...form,
                  site_id: next.site_id != null ? String(next.site_id) : '',
                  adhoc_site: next.adhoc_site,
                })
              }
            />
            <label className={shared.label}>
              Descrizione
              <textarea
                className={shared.textarea}
                value={form.description_it}
                onChange={(event) => setForm({ ...form, description_it: event.target.value })}
              />
            </label>
            <label className={shared.label}>
              Motivo
              <textarea
                className={shared.textarea}
                value={form.reason_it}
                onChange={(event) => setForm({ ...form, reason_it: event.target.value })}
              />
            </label>
          </div>
          <div className={shared.formActions} style={{ marginTop: '1rem' }}>
            <Button variant="secondary" onClick={() => setEditing(false)}>
              Annulla
            </Button>
            <Button onClick={save} loading={update.isPending}>
              Salva
            </Button>
          </div>
        </>
      ) : (
        <div className={shared.detailsGrid}>
          <DetailItem label="Tipo" value={detail.maintenance_kind.name_it} />
          <DetailItem label="Dominio" value={detail.technical_domain.name_it} />
          <DetailItem label="Ambito clienti" value={customerScopeLabel(detail.customer_scope)} />
          <div className={shared.detailItem}>
            <span>Sito</span>
            <strong>
              {detail.site?.name_it ?? '-'}
              {detail.site?.scope === 'scoped' ? (
                <span
                  style={{
                    marginLeft: '0.4rem',
                    padding: '0.05rem 0.45rem',
                    borderRadius: '999px',
                    background: 'rgba(56, 189, 248, 0.15)',
                    color: 'rgb(125, 211, 252)',
                    fontSize: '0.7rem',
                    fontWeight: 600,
                    textTransform: 'uppercase',
                    letterSpacing: '0.04em',
                  }}
                >
                  Ad-hoc
                </span>
              ) : null}
            </strong>
          </div>
          <DetailItem label="Creata" value={formatDateTime(detail.created_at)} />
          <DetailItem label="Aggiornata" value={formatDateTime(detail.updated_at)} />
          <DetailItem label="Descrizione" value={detail.description_it ?? '-'} />
          <DetailItem label="Motivo" value={detail.reason_it ?? '-'} />
          <DetailItem
            label="Servizio garantito durante l'intervento"
            value={detail.residual_service_it ?? '-'}
          />
        </div>
      )}
      {canManage && !editing && (
        <AssistancePanel
          draft={assistanceDraft}
          note={assistanceNote}
          onNoteChange={setAssistanceNote}
          onGenerate={generateAssistance}
          generating={assistance.isPending}
          applying={applyPending}
          onApply={applyAssistance}
          onDiscard={() => setAssistanceDraft(null)}
        />
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
  includeEmpty = true,
}: {
  label: string;
  value: string;
  items: Array<{ id: number; name_it: string }>;
  onChange: (value: string) => void;
  emptyLabel?: string;
  includeEmpty?: boolean;
}) {
  return (
    <label className={shared.label}>
      {label}
      <select className={shared.select} value={value} onChange={(event) => onChange(event.target.value)}>
        {includeEmpty && <option value="">{emptyLabel}</option>}
        {items.map((item) => (
          <option key={item.id} value={item.id}>
            {item.name_it}
          </option>
        ))}
      </select>
    </label>
  );
}

function DetailItem({ label, value }: { label: string; value: string }) {
  return (
    <div className={shared.detailItem}>
      <span>{label}</span>
      <strong>{value}</strong>
    </div>
  );
}

function customerScopeLabel(scope: ReferenceItem | null): string {
  return scope?.name_it ?? 'Da definire';
}

function AssistancePanel({
  draft,
  note,
  onNoteChange,
  onGenerate,
  generating,
  applying,
  onApply,
  onDiscard,
}: {
  draft: MaintenanceAssistanceDraft | null;
  note: string;
  onNoteChange: (value: string) => void;
  onGenerate: () => void;
  generating: boolean;
  applying: boolean;
  onApply: () => void;
  onDiscard: () => void;
}) {
  return (
    <div className={shared.assistancePanel}>
      <div className={shared.sectionHeader}>
        <div>
          <h3 className={shared.sectionTitle}>Assistenza</h3>
          {draft && <p className={shared.small}>{draft.audit.summary}</p>}
        </div>
        <div className={shared.inlineActions}>
          {draft && (
            <Button size="sm" variant="secondary" onClick={onDiscard}>
              Scarta
            </Button>
          )}
          {draft ? (
            <Button size="sm" onClick={onApply} loading={applying}>
              Applica proposte
            </Button>
          ) : (
            <Button size="sm" variant="secondary" onClick={onGenerate} loading={generating}>
              Genera testi e classificazione
            </Button>
          )}
        </div>
      </div>
      {!draft && (
        <label className={shared.label}>
          Indicazioni aggiuntive
          <textarea
            className={shared.textarea}
            value={note}
            onChange={(event) => onNoteChange(event.target.value)}
          />
        </label>
      )}
      {draft && (
        <div className={shared.proposalGrid}>
          <ProposalBlock
            title="Testi proposti"
            rows={textProposalRows(draft)}
          />
          <ClassificationProposalBlock title="Servizi coinvolti" items={draft.service_taxonomy} />
          <ClassificationProposalBlock title="Motivi" items={draft.reason_classes} />
          <ClassificationProposalBlock title="Effetti attesi" items={draft.impact_effects} />
          <ClassificationProposalBlock title="Segnali qualità" items={draft.quality_flags} />
        </div>
      )}
    </div>
  );
}

function ProposalBlock({ title, rows }: { title: string; rows: Array<{ label: string; value: string }> }) {
  return (
    <div className={shared.proposalBlock}>
      <h4>{title}</h4>
      {rows.length === 0 ? (
        <p className={shared.small}>Nessuna proposta.</p>
      ) : (
        <div className={shared.proposalList}>
          {rows.map((row) => (
            <div className={shared.proposalItem} key={row.label}>
              <span>{row.label}</span>
              <strong>{row.value}</strong>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

function ClassificationProposalBlock({
  title,
  items,
}: {
  title: string;
  items: AssistanceClassificationProposal[];
}) {
  return (
    <div className={shared.proposalBlock}>
      <h4>{title}</h4>
      {items.length === 0 ? (
        <p className={shared.small}>Nessuna proposta.</p>
      ) : (
        <div className={shared.proposalList}>
          {items.map((item) => (
            <div className={shared.proposalItem} key={`${title}-${item.reference_id}`}>
              <span>{item.label}</span>
              <strong>{sourceLabel(item.source)}</strong>
              <p className={shared.proposalMeta}>
                Affidabilità {confidenceLabel(item.confidence)}
                {item.is_primary ? ' · Principale' : ''}
              </p>
              {item.rationale && <p className={shared.small}>{item.rationale}</p>}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

function textProposalRows(draft: MaintenanceAssistanceDraft): Array<{ label: string; value: string }> {
  const rows: Array<{ label: string; value: string }> = [];
  addTextRow(rows, 'Titolo', draft.texts.title_it);
  addTextRow(rows, 'Titolo EN', draft.texts.title_en);
  addTextRow(rows, 'Descrizione', draft.texts.description_it);
  addTextRow(rows, 'Descrizione EN', draft.texts.description_en);
  addTextRow(rows, 'Motivo EN', draft.texts.reason_en);
  addTextRow(rows, "Servizio garantito durante l'intervento EN", draft.texts.residual_service_en);
  return rows;
}

function addTextRow(rows: Array<{ label: string; value: string }>, label: string, value?: string | null) {
  const clean = value?.trim();
  if (clean) rows.push({ label, value: clean });
}

function assistanceInputs(items: AssistanceClassificationProposal[]): ClassificationInput[] {
  return items.map((item) => ({
    reference_id: item.reference_id,
    source: 'ai_extracted',
    confidence: item.confidence ?? null,
    is_primary: item.is_primary,
    metadata: {
      assistance: {
        rationale: item.rationale ?? null,
      },
    },
  }));
}

function proposedOrCurrent(value: string | null | undefined, current: string | null): string | null {
  const trimmed = value?.trim();
  return trimmed || current;
}

function buildSitePatch(input: {
  previousSiteId: number | null;
  siteIdRaw: string;
  adhocSite: AdhocSiteInput | null;
}): {
  site_id?: number | null;
  adhoc_site?: AdhocSiteInput;
  clear_site?: boolean;
} {
  if (input.adhocSite) {
    return {
      adhoc_site: {
        ...input.adhocSite,
        name: input.adhocSite.name.trim(),
        city: input.adhocSite.city?.trim() || null,
        country_code: input.adhocSite.country_code?.trim() || null,
        code: input.adhocSite.code?.trim() || null,
      },
    };
  }
  if (input.siteIdRaw) {
    return { site_id: Number(input.siteIdRaw) };
  }
  // Niente sito scelto: clear solo se prima ce n'era uno.
  if (input.previousSiteId != null) {
    return { clear_site: true };
  }
  return {};
}

function canClearCustomerScope(status: string): boolean {
  return status === 'draft' || status === 'cancelled';
}

function buildCustomerScopePatch(input: {
  status: string;
  previousCustomerScopeId: number | null;
  customerScopeIdRaw: string;
}): {
  customer_scope_id?: number;
  clear_customer_scope?: boolean;
} | null {
  if (input.customerScopeIdRaw) {
    return { customer_scope_id: Number(input.customerScopeIdRaw) };
  }
  if (!canClearCustomerScope(input.status)) {
    return null;
  }
  if (input.previousCustomerScopeId != null) {
    return { clear_customer_scope: true };
  }
  return {};
}

function assistanceTextsMetadata(texts: MaintenanceAssistanceDraft['texts']): JsonObject {
  return {
    title_it: texts.title_it ?? null,
    title_en: texts.title_en ?? null,
    description_it: texts.description_it ?? null,
    description_en: texts.description_en ?? null,
    reason_en: texts.reason_en ?? null,
    residual_service_en: texts.residual_service_en ?? null,
  };
}

function mergeMetadata(current: JsonObject | undefined, patch: JsonObject): JsonObject {
  return {
    ...(current ?? {}),
    ...patch,
  };
}

interface WindowFormState {
  startDate: string;
  startTime: string;
  endDate: string;
  endTime: string;
  expectedDowntimeMinutes: string;
}

const emptyWindowForm: WindowFormState = {
  startDate: '',
  startTime: '',
  endDate: '',
  endTime: '',
  expectedDowntimeMinutes: '',
};

function splitDateTimeInput(value: string): { date: string; time: string } {
  if (!value) return { date: '', time: '' };
  const [date = '', rawTime = ''] = value.split('T');
  return { date, time: rawTime.slice(0, 5) };
}

function windowFormFromWindow(window: MaintenanceWindow): WindowFormState {
  const start = splitDateTimeInput(formatDateInput(window.scheduled_start_at));
  const end = splitDateTimeInput(formatDateInput(window.scheduled_end_at));
  return {
    startDate: start.date,
    startTime: start.time,
    endDate: end.date,
    endTime: end.time,
    expectedDowntimeMinutes:
      window.expected_downtime_minutes === null || window.expected_downtime_minutes === undefined
        ? ''
        : String(window.expected_downtime_minutes),
  };
}

function buildWindowBody(form: WindowFormState): WindowBody | null {
  if (!form.startDate || !form.startTime || !form.endDate || !form.endTime) return null;
  return {
    scheduled_start_at: `${form.startDate}T${form.startTime}`,
    scheduled_end_at: `${form.endDate}T${form.endTime}`,
    expected_downtime_minutes: form.expectedDowntimeMinutes
      ? Number(form.expectedDowntimeMinutes)
      : null,
  };
}

function windowFormValidationMessage(form: WindowFormState, missingMessage: string): string | null {
  const body = buildWindowBody(form);
  if (!body) return missingMessage;
  return validateWindowTiming(body);
}

function durationHelper(form: WindowFormState): string {
  const body = buildWindowBody(form);
  if (!body) return 'Durata finestra: -';
  const minutes = windowDurationMinutes(body);
  return minutes === null
    ? 'Durata finestra: verifica inizio e fine'
    : `Durata finestra: ${minutes} min`;
}

function compareWindowsBySchedule(a: MaintenanceWindow, b: MaintenanceWindow): number {
  const aStart = new Date(a.scheduled_start_at).getTime();
  const bStart = new Date(b.scheduled_start_at).getTime();
  if (aStart !== bStart) return aStart - bStart;
  return a.seq_no - b.seq_no;
}

function WindowsTab({ detail, canManage }: { detail: MaintenanceDetail; canManage: boolean }) {
  const mutations = useWindowMutations(detail.maintenance_id);
  const toast = useToast();
  const [form, setForm] = useState<WindowFormState>(emptyWindowForm);
  const [rescheduleWindow, setRescheduleWindow] = useState<MaintenanceWindow | null>(null);
  const [rescheduleForm, setRescheduleForm] = useState<WindowFormState>(emptyWindowForm);
  const sortedWindows = [...detail.windows].sort(compareWindowsBySchedule);

  async function addWindow() {
    const validationMessage = windowFormValidationMessage(form, 'Indica inizio e fine previsti.');
    if (validationMessage) {
      toast.toast(validationMessage, 'error');
      return;
    }
    const body = buildWindowBody(form);
    if (!body) {
      toast.toast('Indica inizio e fine previsti.', 'error');
      return;
    }
    try {
      await mutations.create.mutateAsync(body);
      setForm(emptyWindowForm);
      toast.toast('Finestra aggiunta.');
    } catch (error) {
      toast.toast(errorMessage(error, 'Salvataggio finestra non riuscito.'), 'error');
    }
  }

  function openReschedule(window: MaintenanceWindow) {
    setRescheduleWindow(window);
    setRescheduleForm(windowFormFromWindow(window));
  }

  function closeReschedule() {
    setRescheduleWindow(null);
    setRescheduleForm(emptyWindowForm);
  }

  async function submitReschedule() {
    const validationMessage = windowFormValidationMessage(rescheduleForm, 'Indica inizio e fine previsti.');
    if (validationMessage) {
      toast.toast(validationMessage, 'error');
      return;
    }
    const body = buildWindowBody(rescheduleForm);
    if (!body) {
      toast.toast('Indica inizio e fine previsti.', 'error');
      return;
    }
    if (!rescheduleWindow) return;
    try {
      await mutations.reschedule.mutateAsync({
        windowId: rescheduleWindow.maintenance_window_id,
        body,
      });
      closeReschedule();
      toast.toast('Finestra ripianificata.');
    } catch (error) {
      toast.toast(errorMessage(error, 'Ripianificazione non riuscita.'), 'error');
    }
  }

  return (
    <div className={shared.panel}>
      <div className={shared.sectionHeader}>
        <h2 className={shared.sectionTitle}>Finestre di attività</h2>
      </div>
      {canManage && (
        <div className={shared.windowFormPanel}>
          <WindowFields form={form} onChange={setForm} />
          <div className={shared.windowFormFooter}>
            <span className={shared.durationHint}>{durationHelper(form)}</span>
            <div className={shared.formActions}>
              <Button
                onClick={addWindow}
                loading={mutations.create.isPending}
                leftIcon={<Icon name="plus" size={16} />}
              >
                Aggiungi finestra
              </Button>
            </div>
          </div>
        </div>
      )}
      <div className={shared.tableCard}>
        <div className={shared.tableScroll}>
          <table className={shared.table}>
            <thead>
              <tr>
                <th>Stato</th>
                <th>Inizio</th>
                <th>Fine</th>
                <th>Downtime previsto</th>
                <th className={shared.actionsCell}>Azioni</th>
              </tr>
            </thead>
            <tbody>
              {sortedWindows.length === 0 ? (
                <tr>
                  <td colSpan={5}>Nessuna finestra registrata.</td>
                </tr>
              ) : (
                sortedWindows.map((window) => (
                  <tr key={window.maintenance_window_id}>
                    <td>{windowStatusLabel(window.window_status)}</td>
                    <td>{formatDateTime(window.scheduled_start_at)}</td>
                    <td>{formatDateTime(window.scheduled_end_at)}</td>
                    <td>{minutesLabel(window.expected_downtime_minutes)}</td>
                    <td className={shared.actionsCell}>
                      {canManage && window.window_status === 'planned' ? (
                        <Button
                          size="sm"
                          variant="secondary"
                          onClick={() => openReschedule(window)}
                          leftIcon={<Icon name="calendar" size={16} />}
                        >
                          Ripianifica
                        </Button>
                      ) : (
                        '-'
                      )}
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </div>
      <Modal
        open={rescheduleWindow !== null}
        onClose={closeReschedule}
        title="Ripianifica finestra"
        size="md"
      >
        <div className={shared.modalBody}>
          <p className={shared.modalIntro}>
            Crea una nuova finestra sostitutiva. La finestra attuale verrà marcata come Sostituita.
          </p>
          {rescheduleWindow ? (
            <div className={shared.inlineSummary}>
              <span>Finestra attuale</span>
              <strong>
                {formatDateTime(rescheduleWindow.scheduled_start_at)} -{' '}
                {formatDateTime(rescheduleWindow.scheduled_end_at)}
              </strong>
            </div>
          ) : null}
          <WindowFields form={rescheduleForm} onChange={setRescheduleForm} />
          <div className={shared.windowFormFooter}>
            <span className={shared.durationHint}>{durationHelper(rescheduleForm)}</span>
            <div className={shared.formActions}>
              <Button
                variant="secondary"
                onClick={closeReschedule}
                disabled={mutations.reschedule.isPending}
              >
                Annulla
              </Button>
              <Button
                onClick={submitReschedule}
                loading={mutations.reschedule.isPending}
                leftIcon={<Icon name="calendar" size={16} />}
              >
                Ripianifica
              </Button>
            </div>
          </div>
        </div>
      </Modal>
    </div>
  );
}

function WindowFields({
  form,
  onChange,
}: {
  form: WindowFormState;
  onChange: (form: WindowFormState) => void;
}) {
  return (
    <div className={shared.windowFormGrid}>
      <label className={shared.label}>
        Data inizio
        <input
          className={shared.field}
          type="date"
          value={form.startDate}
          onChange={(event) => onChange({ ...form, startDate: event.target.value })}
        />
      </label>
      <label className={shared.label}>
        Ora inizio
        <input
          className={shared.field}
          type="time"
          value={form.startTime}
          onChange={(event) => onChange({ ...form, startTime: event.target.value })}
        />
      </label>
      <label className={shared.label}>
        Data fine
        <input
          className={shared.field}
          type="date"
          min={form.startDate || undefined}
          value={form.endDate}
          onChange={(event) => onChange({ ...form, endDate: event.target.value })}
        />
      </label>
      <label className={shared.label}>
        Ora fine
        <input
          className={shared.field}
          type="time"
          min={form.endDate && form.endDate === form.startDate ? form.startTime || undefined : undefined}
          value={form.endTime}
          onChange={(event) => onChange({ ...form, endTime: event.target.value })}
        />
      </label>
      <label className={shared.label}>
        Downtime previsto (minuti)
        <input
          className={shared.field}
          type="number"
          min="0"
          value={form.expectedDowntimeMinutes}
          onChange={(event) => onChange({ ...form, expectedDowntimeMinutes: event.target.value })}
        />
        <span className={shared.fieldHelper}>Tempo stimato di indisponibilità, espresso in minuti.</span>
      </label>
    </div>
  );
}

function ImpactTab({
  detail,
  reference,
  canManage,
}: {
  detail: MaintenanceDetail;
  reference?: ReferenceData;
  canManage: boolean;
}) {
  if (!reference) return <Skeleton rows={5} />;
  return (
    <>
      <ClassificationSection
        title="Servizi"
        resource="service-taxonomy"
        items={detail.service_taxonomy}
        options={reference.service_taxonomy}
        technicalDomainId={detail.technical_domain.id}
        maintenanceId={detail.maintenance_id}
        canManage={canManage}
        primary
      />
      <ClassificationSection
        title="Motivi"
        resource="reason-classes"
        items={detail.reason_classes}
        options={reference.reason_classes}
        maintenanceId={detail.maintenance_id}
        canManage={canManage}
        primary
      />
      <ClassificationSection
        title="Effetti"
        resource="impact-effects"
        items={detail.impact_effects}
        options={reference.impact_effects}
        maintenanceId={detail.maintenance_id}
        canManage={canManage}
        primary
      />
      <ClassificationSection
        title="Segnali qualità"
        resource="quality-flags"
        items={detail.quality_flags}
        options={reference.quality_flags}
        maintenanceId={detail.maintenance_id}
        canManage={canManage}
      />
    </>
  );
}

function ClassificationSection({
  title,
  resource,
  items,
  options,
  technicalDomainId,
  maintenanceId,
  canManage,
  primary = false,
}: {
  title: string;
  resource: string;
  items: MaintenanceDetail['service_taxonomy'];
  options: ReferenceItem[];
  technicalDomainId?: number;
  maintenanceId: number;
  canManage: boolean;
  primary?: boolean;
}) {
  const mutation = useReplaceClassifications(maintenanceId, resource);
  const toast = useToast();
  const [selected, setSelected] = useState('');
  const [isPrimary, setIsPrimary] = useState(false);
  const [role, setRole] = useState<'operated' | 'dependent'>('operated');
  const [severity, setSeverity] = useState<'none' | 'degraded' | 'unavailable'>('unavailable');
  const [audience, setAudience] = useState('');
  const isServiceSection = resource === 'service-taxonomy';
  const selectableOptions =
    isServiceSection && role === 'operated' && technicalDomainId
      ? options.filter((item) => item.technical_domain_id === technicalDomainId)
      : options;

  function canOperateSelection(id: number): boolean {
    if (!technicalDomainId) return true;
    const option = options.find((item) => item.id === id);
    return option?.technical_domain_id === technicalDomainId;
  }

  function changeRole(nextRole: 'operated' | 'dependent') {
    setRole(nextRole);
    if (nextRole === 'operated' && selected && !canOperateSelection(Number(selected))) {
      setSelected('');
      toast.toast('Solo i servizi del dominio tecnico possono essere operati.', 'warning');
    }
  }

  async function add() {
    if (!selected) return;
    if (isServiceSection && role === 'operated' && !canOperateSelection(Number(selected))) {
      toast.toast('Solo i servizi del dominio tecnico possono essere operati.', 'error');
      return;
    }
    const next: ClassificationInput[] = items.map((item) => ({
      reference_id: item.reference.id,
      source: item.source,
      confidence: item.confidence ?? null,
      is_primary: primary ? item.is_primary && !isPrimary : false,
      role: item.role ?? undefined,
      expected_severity: item.expected_severity ?? undefined,
      expected_audience: item.expected_audience ?? null,
    }));
    next.push({
      reference_id: Number(selected),
      service_taxonomy_id: Number(selected),
      source: 'manual',
      is_primary: primary ? isPrimary : false,
      role: isServiceSection ? role : undefined,
      expected_severity: isServiceSection ? severity : undefined,
      expected_audience: isServiceSection ? (audience || null) as 'internal' | 'external' | 'both' | null : null,
    });
    try {
      await mutation.mutateAsync(next);
      setSelected('');
      setIsPrimary(false);
      setRole('operated');
      setSeverity('unavailable');
      setAudience('');
      toast.toast('Classificazione aggiornata.');
    } catch (error) {
      toast.toast(errorMessage(error, 'Aggiornamento non riuscito.'), 'error');
    }
  }

  return (
    <div className={shared.panel}>
      <div className={shared.sectionHeader}>
        <h2 className={shared.sectionTitle}>{title}</h2>
      </div>
      {canManage && (
        <div className={shared.formGridThree} style={{ marginBottom: '1rem' }}>
          <select className={shared.select} value={selected} onChange={(event) => setSelected(event.target.value)}>
            <option value="">Seleziona</option>
            {selectableOptions.map((item) => (
              <option key={item.id} value={item.id}>
                {item.name_it}
              </option>
            ))}
          </select>
          {primary && (
            <label className={shared.label}>
              Principale
              <input type="checkbox" checked={isPrimary} onChange={(event) => setIsPrimary(event.target.checked)} />
            </label>
          )}
          {isServiceSection && (
            <>
              <select className={shared.select} value={role} onChange={(event) => changeRole(event.target.value as typeof role)}>
                <option value="operated">Operato</option>
                <option value="dependent">Impattato</option>
              </select>
              <select className={shared.select} value={severity} onChange={(event) => setSeverity(event.target.value as typeof severity)}>
                {SEVERITY_OPTIONS.map((option) => (
                  <option key={option.value} value={option.value}>
                    {option.label}
                  </option>
                ))}
              </select>
              <select className={shared.select} value={audience} onChange={(event) => setAudience(event.target.value)}>
                <option value="">Audience catalogo</option>
                <option value="internal">Interna</option>
                <option value="external">Esterna</option>
                <option value="both">Interna ed esterna</option>
              </select>
            </>
          )}
          <Button onClick={add} loading={mutation.isPending}>
            Aggiungi
          </Button>
        </div>
      )}
      <SimpleTable
        headers={
          isServiceSection
            ? ['Valore', 'Ruolo', 'Severità', 'Audience', 'Origine', 'Principale']
            : ['Valore', 'Origine', 'Confidenza', 'Principale']
        }
        rows={items.map((item) => [
          ...(isServiceSection
            ? [
                item.reference.name_it,
                serviceRoleLabel(item.role),
                severityLabel(item.expected_severity),
                item.expected_audience ? audienceLabel(item.expected_audience) : audienceLabel(item.reference.audience ?? ''),
                sourceLabel(item.source),
                item.is_primary ? 'Sì' : '-',
              ]
            : [
                item.reference.name_it,
                sourceLabel(item.source),
                confidenceLabel(item.confidence),
                item.is_primary ? 'Sì' : '-',
              ]),
        ])}
        empty="Nessun valore registrato."
      />
    </div>
  );
}

function TargetsTab({
  detail,
  reference,
  canManage,
}: {
  detail: MaintenanceDetail;
  reference?: ReferenceData;
  canManage: boolean;
}) {
  const mutations = useTargetMutations(detail.maintenance_id);
  const toast = useToast();
  const [removeId, setRemoveId] = useState<number | null>(null);
  const [form, setForm] = useState<TargetBody>({
    target_type_id: 0,
    service_taxonomy_id: null,
    display_name: '',
    source: 'manual',
    is_primary: false,
  });

  async function save() {
    if (!form.target_type_id || !form.display_name.trim()) {
      toast.toast('Completa tipo e nome target.', 'error');
      return;
    }
    try {
      await mutations.create.mutateAsync(form);
      setForm({ target_type_id: 0, service_taxonomy_id: null, display_name: '', source: 'manual', is_primary: false });
      toast.toast('Target aggiunto.');
    } catch (error) {
      toast.toast(errorMessage(error, 'Salvataggio target non riuscito.'), 'error');
    }
  }

  return (
    <div className={shared.panel}>
      <div className={shared.sectionHeader}>
        <h2 className={shared.sectionTitle}>Target</h2>
      </div>
      {canManage && reference && (
        <div className={shared.formGridThree} style={{ marginBottom: '1rem' }}>
          <select
            className={shared.select}
            value={form.target_type_id || ''}
            onChange={(event) => setForm({ ...form, target_type_id: Number(event.target.value) })}
          >
            <option value="">Tipo target</option>
            {reference.target_types.map((item) => (
              <option key={item.id} value={item.id}>
                {item.name_it}
              </option>
            ))}
          </select>
          <input
            className={shared.field}
            value={form.display_name}
            onChange={(event) => setForm({ ...form, display_name: event.target.value })}
            placeholder="Nome target"
          />
          <select
            className={shared.select}
            value={form.service_taxonomy_id ?? ''}
            onChange={(event) =>
              setForm({ ...form, service_taxonomy_id: event.target.value ? Number(event.target.value) : null })
            }
          >
            <option value="">Servizio collegato</option>
            {reference.service_taxonomy.map((item) => (
              <option key={item.id} value={item.id}>
                {item.name_it}
              </option>
            ))}
          </select>
          <Button onClick={save} loading={mutations.create.isPending}>
            Aggiungi
          </Button>
        </div>
      )}
      <div className={shared.tableCard}>
        <div className={shared.tableScroll}>
          <table className={shared.table}>
            <thead>
              <tr>
                <th>Nome</th>
                <th>Tipo</th>
                <th>Servizio</th>
                <th>Origine</th>
                <th>Principale</th>
                <th className={shared.actionsCell}>Azioni</th>
              </tr>
            </thead>
            <tbody>
              {detail.targets.length === 0 ? (
                <tr>
                  <td colSpan={6}>Nessun target registrato.</td>
                </tr>
              ) : (
                detail.targets.map((item) => (
                  <tr key={item.maintenance_target_id}>
                    <td>{item.display_name}</td>
                    <td>{item.target_type.name_it}</td>
                    <td>{item.service_taxonomy?.name_it ?? '-'}</td>
                    <td>{sourceLabel(item.source)}</td>
                    <td>{item.is_primary ? 'Sì' : '-'}</td>
                    <td className={shared.actionsCell}>
                      {canManage && (
                        <Button
                          size="sm"
                          variant="danger"
                          onClick={() => setRemoveId(item.maintenance_target_id)}
                        >
                          Rimuovi
                        </Button>
                      )}
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </div>
      <ConfirmDialog
        open={removeId !== null}
        title="Rimuovi target"
        message="Il target sarà rimosso da questa manutenzione."
        confirmLabel="Rimuovi"
        busy={mutations.remove.isPending}
        onClose={() => setRemoveId(null)}
        onConfirm={async () => {
          if (removeId === null) return;
          try {
            await mutations.remove.mutateAsync(removeId);
            setRemoveId(null);
            toast.toast('Target rimosso.');
          } catch (error) {
            toast.toast(errorMessage(error, 'Rimozione non riuscita.'), 'error');
          }
        }}
      />
    </div>
  );
}

function CustomersTab({ detail, canManage }: { detail: MaintenanceDetail; canManage: boolean }) {
  const mutations = useCustomerImpactMutations(detail.maintenance_id);
  const toast = useToast();
  const [search, setSearch] = useState('');
  const [removeId, setRemoveId] = useState<number | null>(null);
  const customers = useCustomerSearch(search, canManage);
  const [form, setForm] = useState<ImpactedCustomerBody>({
    customer_id: 0,
    impact_scope: 'possible',
    derivation_source: 'manual',
  });

  async function save() {
    if (!form.customer_id) {
      toast.toast('Seleziona un cliente.', 'error');
      return;
    }
    try {
      await mutations.create.mutateAsync(form);
      setForm({ customer_id: 0, impact_scope: 'possible', derivation_source: 'manual' });
      setSearch('');
      toast.toast('Cliente aggiunto.');
    } catch (error) {
      toast.toast(errorMessage(error, 'Salvataggio cliente non riuscito.'), 'error');
    }
  }

  return (
    <div className={shared.panel}>
      <div className={shared.sectionHeader}>
        <h2 className={shared.sectionTitle}>Clienti impattati</h2>
      </div>
      {canManage && (
        <div className={shared.formGridThree} style={{ marginBottom: '1rem' }}>
          <input
            className={shared.field}
            value={search}
            onChange={(event) => setSearch(event.target.value)}
            placeholder="Cerca cliente"
          />
          <select
            className={shared.select}
            value={form.customer_id || ''}
            onChange={(event) => setForm({ ...form, customer_id: Number(event.target.value) })}
          >
            <option value="">Seleziona cliente</option>
            {customers.data?.map((item) => (
              <option key={item.id} value={item.id}>
                {item.name} · {item.id}
              </option>
            ))}
          </select>
          <select
            className={shared.select}
            value={form.impact_scope}
            onChange={(event) => setForm({ ...form, impact_scope: event.target.value })}
          >
            <option value="direct">Diretto</option>
            <option value="indirect">Indiretto</option>
            <option value="possible">Possibile</option>
          </select>
          <Button onClick={save} loading={mutations.create.isPending}>
            Aggiungi
          </Button>
        </div>
      )}
      <div className={shared.tableCard}>
        <div className={shared.tableScroll}>
          <table className={shared.table}>
            <thead>
              <tr>
                <th>Cliente</th>
                <th>Ambito</th>
                <th>Origine</th>
                <th>Confidenza</th>
                <th className={shared.actionsCell}>Azioni</th>
              </tr>
            </thead>
            <tbody>
              {detail.impacted_customers.length === 0 ? (
                <tr>
                  <td colSpan={5}>Nessun cliente registrato.</td>
                </tr>
              ) : (
                detail.impacted_customers.map((item) => (
                  <tr key={item.maintenance_impacted_customer_id}>
                    <td>{item.customer_name}</td>
                    <td>{impactScopeLabel(item.impact_scope)}</td>
                    <td>{sourceLabel(item.derivation_source)}</td>
                    <td>{confidenceLabel(item.confidence)}</td>
                    <td className={shared.actionsCell}>
                      {canManage && (
                        <Button
                          size="sm"
                          variant="danger"
                          onClick={() => setRemoveId(item.maintenance_impacted_customer_id)}
                        >
                          Rimuovi
                        </Button>
                      )}
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </div>
      <ConfirmDialog
        open={removeId !== null}
        title="Rimuovi cliente"
        message="Il cliente sarà rimosso dall'elenco degli impatti."
        confirmLabel="Rimuovi"
        busy={mutations.remove.isPending}
        onClose={() => setRemoveId(null)}
        onConfirm={async () => {
          if (removeId === null) return;
          try {
            await mutations.remove.mutateAsync(removeId);
            setRemoveId(null);
            toast.toast('Cliente rimosso.');
          } catch (error) {
            toast.toast(errorMessage(error, 'Rimozione non riuscita.'), 'error');
          }
        }}
      />
    </div>
  );
}

function NoticesTab({
  detail,
  reference,
  canManage,
}: {
  detail: MaintenanceDetail;
  reference?: ReferenceData;
  canManage: boolean;
}) {
  const mutations = useNoticeMutations(detail.maintenance_id);
  const toast = useToast();
  const [form, setForm] = useState<NoticeBody>({
    notice_type: 'announcement',
    audience: 'internal',
    notice_channel_id: 0,
    generation_source: 'manual',
    send_status: 'draft',
    locales: [{ locale: 'it', subject: '', body_text: '' }],
  });

  async function save() {
    if (!form.notice_channel_id) {
      toast.toast('Seleziona un canale.', 'error');
      return;
    }
    try {
      await mutations.create.mutateAsync(form);
      setForm({
        notice_type: 'announcement',
        audience: 'internal',
        notice_channel_id: 0,
        generation_source: 'manual',
        send_status: 'draft',
        locales: [{ locale: 'it', subject: '', body_text: '' }],
      });
      toast.toast('Comunicazione aggiunta.');
    } catch (error) {
      toast.toast(errorMessage(error, 'Salvataggio comunicazione non riuscito.'), 'error');
    }
  }

  return (
    <div className={shared.panel}>
      <div className={shared.sectionHeader}>
        <h2 className={shared.sectionTitle}>Comunicazioni</h2>
      </div>
      {canManage && reference && (
        <div className={shared.formGridThree} style={{ marginBottom: '1rem' }}>
          <select
            className={shared.select}
            value={form.notice_type}
            onChange={(event) => setForm({ ...form, notice_type: event.target.value })}
          >
            <option value="announcement">Annuncio</option>
            <option value="reminder">Promemoria</option>
            <option value="reschedule">Riprogrammazione</option>
            <option value="cancellation">Annullamento</option>
            <option value="internal_update">Aggiornamento interno</option>
          </select>
          <select
            className={shared.select}
            value={form.audience}
            onChange={(event) => setForm({ ...form, audience: event.target.value })}
          >
            <option value="internal">Interna</option>
            <option value="external">Esterna</option>
          </select>
          <select
            className={shared.select}
            value={form.notice_channel_id || ''}
            onChange={(event) => setForm({ ...form, notice_channel_id: Number(event.target.value) })}
          >
            <option value="">Canale</option>
            {reference.notice_channels.map((item) => (
              <option key={item.id} value={item.id}>
                {item.name_it}
              </option>
            ))}
          </select>
          <input
            className={shared.field}
            value={form.locales?.[0]?.subject ?? ''}
            onChange={(event) =>
              setForm({ ...form, locales: [{ locale: 'it', subject: event.target.value, body_text: form.locales?.[0]?.body_text ?? '' }] })
            }
            placeholder="Oggetto"
          />
          <textarea
            className={shared.textarea}
            value={form.locales?.[0]?.body_text ?? ''}
            onChange={(event) =>
              setForm({ ...form, locales: [{ locale: 'it', subject: form.locales?.[0]?.subject ?? '', body_text: event.target.value }] })
            }
            placeholder="Testo"
          />
          <Button onClick={save} loading={mutations.create.isPending}>
            Aggiungi
          </Button>
        </div>
      )}
      <SimpleTable
        headers={['Tipo', 'Destinatari', 'Canale', 'Stato', 'Creata']}
        rows={detail.notices.map((item) => [
          noticeTypeLabel(item.notice_type),
          audienceLabel(item.audience),
          item.notice_channel.name_it,
          sendStatusLabel(item.send_status),
          formatDateTime(item.created_at),
        ])}
        empty="Nessuna comunicazione registrata."
      />
    </div>
  );
}

function EventsTab({ detail }: { detail: MaintenanceDetail }) {
  return (
    <div className={shared.panel}>
      <div className={shared.sectionHeader}>
        <h2 className={shared.sectionTitle}>Storico</h2>
      </div>
      <SimpleTable
        headers={['Data', 'Evento', 'Sintesi']}
        rows={detail.events.map((item) => [
          formatDateTime(item.event_at),
          item.event_type,
          item.summary ?? '-',
        ])}
        empty="Nessun evento registrato."
      />
    </div>
  );
}

function SimpleTable({
  headers,
  rows,
  empty,
}: {
  headers: string[];
  rows: ReactNode[][];
  empty: string;
}) {
  return (
    <div className={shared.tableCard}>
      <div className={shared.tableScroll}>
        <table className={shared.table}>
          <thead>
            <tr>
              {headers.map((header) => (
                <th key={header}>{header}</th>
              ))}
            </tr>
          </thead>
          <tbody>
            {rows.length === 0 ? (
              <tr>
                <td colSpan={headers.length}>{empty}</td>
              </tr>
            ) : (
              rows.map((row, rowIndex) => (
                <tr key={rowIndex}>
                  {row.map((cell, cellIndex) => (
                    <td key={`${rowIndex}-${cellIndex}`}>{cell}</td>
                  ))}
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
}
