import { useMemo, useState } from 'react';
import { Icon, useToast } from '@mrsmith/ui';
import {
  useReplaceClassifications,
  useServiceDependencies,
  useTargetMutations,
} from '../api/queries';
import type {
  ClassificationInput,
  ClassificationItem,
  MaintenanceDetail,
  MaintenanceTarget,
  ReferenceData,
  ReferenceItem,
  ServiceDependency,
  SeverityValue,
  TargetBody,
} from '../api/types';
import { CatalogCombobox } from './CatalogCombobox';
import { ConfirmDialog } from './ConfirmDialog';
import { CreateServiceTaxonomyModal } from './CreateServiceTaxonomyModal';
import { ImpactRelationRail } from './ImpactRelationRail';
import { ImpactServiceCard, type ImpactSelectionView } from './ImpactServiceCard';
import { errorMessage, severityLabel } from '../lib/format';
import styles from './ImpactWorkbench.module.css';

interface Props {
  detail: MaintenanceDetail;
  reference: ReferenceData;
  canOperate: boolean;
}

type ImpactRole = ImpactSelectionView['role'];

export function ImpactWorkbench({ detail, reference, canOperate }: Props) {
  const toast = useToast();
  const serviceMutation = useReplaceClassifications(detail.maintenance_id, 'service-taxonomy');
  const targetMutations = useTargetMutations(detail.maintenance_id);
  const dependencies = useServiceDependencies('active');
  const [ignoredSuggestionIds, setIgnoredSuggestionIds] = useState<Set<number>>(new Set());
  const [createTaxonomyContext, setCreateTaxonomyContext] = useState<{
    role: ImpactRole;
    initialName: string;
  } | null>(null);
  const [removeTarget, setRemoveTarget] = useState<MaintenanceTarget | null>(null);

  const selections = useMemo(
    () => normalizeImpactSelections(detail.service_taxonomy, detail.technical_domain.id),
    [detail.service_taxonomy, detail.technical_domain.id],
  );
  const operatedSelections = useMemo(
    () => selections.filter((item) => item.role === 'operated'),
    [selections],
  );
  const dependentSelections = useMemo(
    () => selections.filter((item) => item.role === 'dependent'),
    [selections],
  );
  const selectedIds = useMemo(
    () => new Set(selections.map((item) => item.reference.id)),
    [selections],
  );
  const targetsByService = useMemo(() => targetsGroupedByService(detail.targets), [detail.targets]);
  const operatedCatalog = useMemo(
    () =>
      reference.service_taxonomy.filter(
        (item) => item.technical_domain_id === detail.technical_domain.id,
      ),
    [detail.technical_domain.id, reference.service_taxonomy],
  );
  const directSuggestions = useMemo(
    () =>
      dedupeDirectSuggestions(
        dependencies.data ?? [],
        new Set(operatedSelections.map((item) => item.reference.id)),
        selectedIds,
      ),
    [dependencies.data, operatedSelections, selectedIds],
  );
  const crossDomainOperatedCount = operatedSelections.filter(
    (item) =>
      item.reference.technical_domain_id != null &&
      item.reference.technical_domain_id !== detail.technical_domain.id,
  ).length;

  const busy = serviceMutation.isPending || targetMutations.create.isPending || targetMutations.remove.isPending;

  async function saveSelections(next: ImpactSelectionView[], successMessage: string) {
    try {
      await serviceMutation.mutateAsync(selectionInputs(next));
      toast.toast(successMessage);
    } catch (error) {
      toast.toast(errorMessage(error, 'Aggiornamento impatto non riuscito.'), 'error');
    }
  }

  function addService(service: ReferenceItem, role: ImpactRole, source: string, severity: SeverityValue) {
    if (role === 'operated' && service.technical_domain_id !== detail.technical_domain.id) {
      toast.toast('Solo i servizi del dominio tecnico possono essere in manutenzione.', 'error');
      return;
    }
    const existing = selections.find((item) => item.reference.id === service.id);
    if (existing?.role === 'operated' && role === 'dependent') {
      toast.toast('La voce è già presente tra quelle in manutenzione.', 'warning');
      return;
    }
    const nextSelection: ImpactSelectionView = {
      reference: service,
      source,
      confidence: null,
      isPrimary: role === 'operated' && operatedSelections.length === 0,
      role,
      expectedSeverity: severity,
      expectedAudience: null,
    };
    const next = mergeSelection(selections, nextSelection);
    void saveSelections(next, role === 'operated' ? 'Servizio in manutenzione aggiunto.' : 'Effetto aggiunto.');
  }

  function requestCreateTaxonomy(initialName: string, role: ImpactRole) {
    setCreateTaxonomyContext({ initialName, role });
  }

  function handleTaxonomyCreated(item: ReferenceItem) {
    if (!createTaxonomyContext) return;
    addService(item, createTaxonomyContext.role, 'manual', 'unavailable');
    setCreateTaxonomyContext(null);
  }

  function updateSelection(
    serviceId: number,
    patch: Partial<Pick<ImpactSelectionView, 'role' | 'expectedSeverity' | 'expectedAudience'>>,
    successMessage: string,
  ) {
    const current = selections.find((item) => item.reference.id === serviceId);
    if (!current) return;
    if (
      patch.role === 'operated' &&
      current.reference.technical_domain_id !== detail.technical_domain.id
    ) {
      toast.toast('Solo i servizi del dominio tecnico possono essere in manutenzione.', 'error');
      return;
    }
    const next = selections.map((item) =>
      item.reference.id === serviceId ? { ...item, ...patch } : item,
    );
    void saveSelections(next, successMessage);
  }

  function removeService(serviceId: number) {
    const next = selections.filter((item) => item.reference.id !== serviceId);
    void saveSelections(next, "Servizio rimosso dall'impatto.");
  }

  function acceptSuggestions(items: ServiceDependency[]) {
    if (items.length === 0) return;
    const incoming = items.map(
      (item): ImpactSelectionView => ({
        reference: item.downstream_service,
        source: 'dependency_graph',
        confidence: null,
        isPrimary: false,
        role: 'dependent',
        expectedSeverity: item.default_severity,
        expectedAudience: null,
      }),
    );
    const next = incoming.reduce(
      (current, item) => mergeSelection(current, item),
      selections,
    );
    void saveSelections(next, 'Suggerimenti aggiunti agli effetti.');
  }

  function ignoreSuggestion(dependencyId: number) {
    setIgnoredSuggestionIds((current) => {
      const next = new Set(current);
      next.add(dependencyId);
      return next;
    });
  }

  async function createTarget(service: ReferenceItem, displayName: string) {
    if (!service.target_type_id) {
      toast.toast('Tipo target mancante: aggiunta istanza non disponibile.', 'error');
      return;
    }
    const body: TargetBody = {
      target_type_id: service.target_type_id,
      service_taxonomy_id: service.id,
      display_name: displayName,
      source: 'manual',
      is_primary: false,
    };
    try {
      await targetMutations.create.mutateAsync(body);
      toast.toast('Istanza aggiunta.');
    } catch (error) {
      toast.toast(errorMessage(error, 'Salvataggio istanza non riuscito.'), 'error');
    }
  }

  async function confirmRemoveTarget() {
    if (!removeTarget) return;
    try {
      await targetMutations.remove.mutateAsync(removeTarget.maintenance_target_id);
      setRemoveTarget(null);
      toast.toast('Target rimosso.');
    } catch (error) {
      toast.toast(errorMessage(error, 'Rimozione target non riuscita.'), 'error');
    }
  }

  return (
    <div className={styles.workbench}>
      <header className={styles.workbenchHeader}>
        <div>
          <h2>Impatto operativo</h2>
          <p>
            Modella cosa entra in manutenzione, cosa ne subisce l'effetto e quali istanze sono coinvolte.
          </p>
        </div>
        <div className={styles.summaryChips} aria-label="Sintesi impatto">
          <span>{operatedSelections.length} in manutenzione</span>
          <span>{dependentSelections.length} effetti</span>
          <span>{detail.targets.length} istanze</span>
        </div>
      </header>

      {crossDomainOperatedCount > 0 ? (
        <div className={styles.warningBanner}>
          <Icon name="triangle-alert" size={16} />
          <span>
            {crossDomainOperatedCount === 1
              ? 'Una voce in manutenzione appartiene a un altro dominio.'
              : `${crossDomainOperatedCount} voci in manutenzione appartengono a un altro dominio.`}
          </span>
        </div>
      ) : null}

      <div className={styles.workspaceGrid}>
        <section className={styles.serviceColumn}>
          <ColumnHeader
            title="In manutenzione"
            subtitle={detail.technical_domain.name_it}
            count={operatedSelections.length}
          />
          {canOperate ? (
            <CatalogCombobox
              options={operatedCatalog}
              excludedIds={selectedIds}
              domainHintId={detail.technical_domain.id}
              placeholder="Cerca servizio in manutenzione"
              onSelect={(item) => addService(item, 'operated', 'manual', 'unavailable')}
              onCreateRequest={(name) => requestCreateTaxonomy(name, 'operated')}
            />
          ) : null}
          <div className={styles.cardStack}>
            {operatedSelections.length > 0 ? (
              operatedSelections.map((selection) => (
                <ImpactServiceCard
                  key={selection.reference.id}
                  selection={selection}
                  targets={targetsByService.get(selection.reference.id) ?? []}
                  maintenanceDomainId={detail.technical_domain.id}
                  canOperate={canOperate}
                  busy={busy}
                  onRoleChange={(serviceId, role) =>
                    updateSelection(serviceId, { role }, 'Ruolo aggiornato.')
                  }
                  onSeverityChange={(serviceId, severity) =>
                    updateSelection(serviceId, { expectedSeverity: severity }, 'Severità aggiornata.')
                  }
                  onAudienceChange={(serviceId, audience) =>
                    updateSelection(serviceId, { expectedAudience: audience }, 'Audience aggiornata.')
                  }
                  onCreateTarget={(service, displayName) => void createTarget(service, displayName)}
                  onRequestRemoveTarget={setRemoveTarget}
                  onRemoveService={removeService}
                />
              ))
            ) : (
              <EmptyColumn
                icon="package"
                title="Nessun servizio in manutenzione"
                text="Aggiungi l'oggetto su cui interviene questa finestra."
              />
            )}
          </div>
        </section>

        <ImpactRelationRail
          operated={operatedSelections}
          dependent={dependentSelections}
          suggestions={directSuggestions}
          ignoredSuggestionIds={ignoredSuggestionIds}
          dependenciesLoading={dependencies.isLoading}
          dependenciesUnavailable={Boolean(dependencies.error)}
          canOperate={canOperate}
          onAcceptSuggestions={acceptSuggestions}
          onIgnoreSuggestion={ignoreSuggestion}
        />

        <section className={styles.serviceColumn}>
          <ColumnHeader
            title="Effetti su altri sistemi"
            subtitle="Cross-dominio"
            count={dependentSelections.length}
          />
          {canOperate ? (
            <CatalogCombobox
              options={reference.service_taxonomy}
              excludedIds={selectedIds}
              domainHintId={detail.technical_domain.id}
              placeholder="Cerca sistema impattato"
              onSelect={(item) => addService(item, 'dependent', 'manual', 'degraded')}
              onCreateRequest={(name) => requestCreateTaxonomy(name, 'dependent')}
            />
          ) : null}
          <div className={styles.cardStack}>
            {dependentSelections.length > 0 ? (
              severityGroups(dependentSelections).map((group) => (
                <div key={group.key} className={styles.severityGroup}>
                  <div className={styles.severityGroupHeader}>
                    <span>{group.label}</span>
                    <span>{group.items.length}</span>
                  </div>
                  {group.items.map((selection) => (
                    <ImpactServiceCard
                      key={selection.reference.id}
                      selection={selection}
                      targets={targetsByService.get(selection.reference.id) ?? []}
                      maintenanceDomainId={detail.technical_domain.id}
                      canOperate={canOperate}
                      busy={busy}
                      onRoleChange={(serviceId, role) =>
                        updateSelection(serviceId, { role }, 'Ruolo aggiornato.')
                      }
                      onSeverityChange={(serviceId, severity) =>
                        updateSelection(serviceId, { expectedSeverity: severity }, 'Severità aggiornata.')
                      }
                      onAudienceChange={(serviceId, audience) =>
                        updateSelection(serviceId, { expectedAudience: audience }, 'Audience aggiornata.')
                      }
                      onCreateTarget={(service, displayName) => void createTarget(service, displayName)}
                      onRequestRemoveTarget={setRemoveTarget}
                      onRemoveService={removeService}
                    />
                  ))}
                </div>
              ))
            ) : (
              <EmptyColumn
                icon="link"
                title="Nessun effetto indicato"
                text="Aggiungi sistemi collegati o accetta un suggerimento dalle dipendenze."
              />
            )}
          </div>
        </section>
      </div>

      <CreateServiceTaxonomyModal
        open={createTaxonomyContext !== null}
        initialName={createTaxonomyContext?.initialName ?? ''}
        initialDomainId={detail.technical_domain.id}
        domains={reference.technical_domains}
        targetTypes={reference.target_types}
        onClose={() => setCreateTaxonomyContext(null)}
        onCreated={handleTaxonomyCreated}
      />

      <ConfirmDialog
        open={removeTarget !== null}
        title="Rimuovi target"
        message={
          removeTarget
            ? `Il target "${removeTarget.display_name}" sarà rimosso da questa manutenzione.`
            : 'Il target sarà rimosso da questa manutenzione.'
        }
        confirmLabel="Rimuovi"
        busy={targetMutations.remove.isPending}
        onClose={() => setRemoveTarget(null)}
        onConfirm={confirmRemoveTarget}
      />
    </div>
  );
}

function ColumnHeader({ title, subtitle, count }: { title: string; subtitle: string; count: number }) {
  return (
    <div className={styles.columnHeader}>
      <div>
        <h3>{title}</h3>
        <p>{subtitle}</p>
      </div>
      <span>{count}</span>
    </div>
  );
}

function EmptyColumn({
  icon,
  title,
  text,
}: {
  icon: 'package' | 'link';
  title: string;
  text: string;
}) {
  return (
    <div className={styles.emptyColumn}>
      <Icon name={icon} size={19} />
      <strong>{title}</strong>
      <p>{text}</p>
    </div>
  );
}

function normalizeImpactSelections(
  items: ClassificationItem[],
  maintenanceDomainId: number,
): ImpactSelectionView[] {
  const byId = new Map<number, ImpactSelectionView>();
  for (const item of items) {
    const role = effectiveRole(item, maintenanceDomainId);
    const next: ImpactSelectionView = {
      reference: item.reference,
      source: item.source,
      confidence: item.confidence ?? null,
      isPrimary: item.is_primary,
      role,
      expectedSeverity: item.expected_severity ?? null,
      expectedAudience: item.expected_audience ?? null,
    };
    const current = byId.get(item.reference.id);
    byId.set(item.reference.id, current ? mergeSelectionItem(current, next) : next);
  }
  return Array.from(byId.values()).sort(compareSelections);
}

function effectiveRole(item: ClassificationItem, maintenanceDomainId: number): ImpactRole {
  if (item.role === 'operated' || item.role === 'dependent') return item.role;
  return item.reference.technical_domain_id === maintenanceDomainId ? 'operated' : 'dependent';
}

function mergeSelection(
  current: ImpactSelectionView[],
  incoming: ImpactSelectionView,
): ImpactSelectionView[] {
  const byId = new Map(current.map((item) => [item.reference.id, item]));
  const existing = byId.get(incoming.reference.id);
  byId.set(incoming.reference.id, existing ? mergeSelectionItem(existing, incoming) : incoming);
  return Array.from(byId.values()).sort(compareSelections);
}

function mergeSelectionItem(
  current: ImpactSelectionView,
  incoming: ImpactSelectionView,
): ImpactSelectionView {
  const operatedWins = current.role === 'operated' || incoming.role === 'operated';
  const winner = incoming.role === 'operated' && current.role !== 'operated' ? incoming : current;
  const fallback = winner === current ? incoming : current;
  return {
    ...winner,
    role: operatedWins ? 'operated' : 'dependent',
    isPrimary: current.isPrimary || incoming.isPrimary,
    expectedSeverity: winner.expectedSeverity ?? fallback.expectedSeverity,
    expectedAudience: winner.expectedAudience ?? fallback.expectedAudience,
  };
}

function compareSelections(a: ImpactSelectionView, b: ImpactSelectionView): number {
  if (a.role !== b.role) return a.role === 'operated' ? -1 : 1;
  return a.reference.name_it.localeCompare(b.reference.name_it);
}

function selectionInputs(items: ImpactSelectionView[]): ClassificationInput[] {
  const existingOperatedPrimaryId =
    items.find((item) => item.role === 'operated' && item.isPrimary)?.reference.id ?? null;
  const firstOperatedId = items.find((item) => item.role === 'operated')?.reference.id ?? null;
  const existingPrimaryId = items.find((item) => item.isPrimary)?.reference.id ?? null;
  const primaryId = existingOperatedPrimaryId ?? firstOperatedId ?? existingPrimaryId;

  return items.map((item) => ({
    reference_id: item.reference.id,
    service_taxonomy_id: item.reference.id,
    source: item.source,
    confidence: item.confidence ?? null,
    is_primary: item.reference.id === primaryId,
    role: item.role,
    expected_severity: item.expectedSeverity ?? undefined,
    expected_audience: item.expectedAudience,
  }));
}

function targetsGroupedByService(targets: MaintenanceTarget[]): Map<number, MaintenanceTarget[]> {
  const map = new Map<number, MaintenanceTarget[]>();
  for (const target of targets) {
    if (!target.service_taxonomy_id) continue;
    const list = map.get(target.service_taxonomy_id) ?? [];
    list.push(target);
    map.set(target.service_taxonomy_id, list);
  }
  return map;
}

function dedupeDirectSuggestions(
  dependencies: ServiceDependency[],
  operatedIds: Set<number>,
  selectedIds: Set<number>,
): ServiceDependency[] {
  const byDownstream = new Map<number, ServiceDependency>();
  for (const dependency of dependencies) {
    if (!operatedIds.has(dependency.upstream_service_id)) continue;
    if (selectedIds.has(dependency.downstream_service_id)) continue;
    const current = byDownstream.get(dependency.downstream_service_id);
    if (!current || severityRank(dependency.default_severity) > severityRank(current.default_severity)) {
      byDownstream.set(dependency.downstream_service_id, dependency);
    }
  }
  return Array.from(byDownstream.values()).sort((a, b) =>
    a.downstream_service.name_it.localeCompare(b.downstream_service.name_it),
  );
}

function severityRank(value: SeverityValue): number {
  if (value === 'unavailable') return 3;
  if (value === 'degraded') return 2;
  return 1;
}

function severityGroups(items: ImpactSelectionView[]): Array<{
  key: string;
  label: string;
  items: ImpactSelectionView[];
}> {
  const order: Array<SeverityValue | 'unset'> = ['unavailable', 'degraded', 'none', 'unset'];
  return order
    .map((key) => {
      const groupItems = items.filter((item) => (item.expectedSeverity ?? 'unset') === key);
      return {
        key,
        label: key === 'unset' ? 'Severità da definire' : severityLabel(key),
        items: groupItems,
      };
    })
    .filter((group) => group.items.length > 0);
}
