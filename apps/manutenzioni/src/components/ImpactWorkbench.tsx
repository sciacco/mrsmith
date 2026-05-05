import { useEffect, useMemo, useState } from 'react';
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
import { EffectsSection } from './EffectsSection';
import { OperatedDetailPanel } from './OperatedDetailPanel';
import { errorMessage } from '../lib/format';
import type { ImpactSelectionView } from './impactTypes';
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
  const [selectedOperatedId, setSelectedOperatedId] = useState<number | null>(null);

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

  useEffect(() => {
    if (operatedSelections.length === 0) {
      if (selectedOperatedId !== null) setSelectedOperatedId(null);
      return;
    }
    const stillSelected = operatedSelections.some((item) => item.reference.id === selectedOperatedId);
    if (!stillSelected) {
      const primary = operatedSelections.find((item) => item.isPrimary);
      setSelectedOperatedId(primary?.reference.id ?? operatedSelections[0]?.reference.id ?? null);
    }
  }, [operatedSelections, selectedOperatedId]);

  const selectedOperated = useMemo(
    () => operatedSelections.find((item) => item.reference.id === selectedOperatedId) ?? null,
    [operatedSelections, selectedOperatedId],
  );

  const operatedCatalog = useMemo(
    () =>
      reference.service_taxonomy.filter(
        (item) => item.technical_domain_id === detail.technical_domain.id,
      ),
    [detail.technical_domain.id, reference.service_taxonomy],
  );

  const allDependencies = dependencies.data ?? [];

  const operatedIdSet = useMemo(
    () => new Set(operatedSelections.map((item) => item.reference.id)),
    [operatedSelections],
  );

  const dependentIdSet = useMemo(
    () => new Set(dependentSelections.map((item) => item.reference.id)),
    [dependentSelections],
  );

  const suggestionsForMaintenance = useMemo(
    () => dedupeSuggestions(allDependencies, operatedIdSet, dependentIdSet, ignoredSuggestionIds),
    [allDependencies, dependentIdSet, ignoredSuggestionIds, operatedIdSet],
  );

  const ignoredSuggestionsCount = useMemo(() => {
    let count = 0;
    for (const dep of allDependencies) {
      if (!operatedIdSet.has(dep.upstream_service_id)) continue;
      if (dependentIdSet.has(dep.downstream_service_id)) continue;
      if (ignoredSuggestionIds.has(dep.service_dependency_id)) count += 1;
    }
    return count;
  }, [allDependencies, dependentIdSet, ignoredSuggestionIds, operatedIdSet]);

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
    if (role === 'operated') setSelectedOperatedId(service.id);
    void saveSelections(next, role === 'operated' ? 'Servizio in manutenzione aggiunto.' : 'Effetto aggiunto.');
  }

  function requestCreateTaxonomy(initialName: string, role: ImpactRole) {
    setCreateTaxonomyContext({ initialName, role });
  }

  function handleTaxonomyCreated(item: ReferenceItem) {
    if (!createTaxonomyContext) return;
    const role = createTaxonomyContext.role;
    addService(item, role, 'manual', role === 'operated' ? 'unavailable' : 'degraded');
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

  function resetIgnoredSuggestions() {
    setIgnoredSuggestionIds(new Set());
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
      toast.toast('Istanza rimossa.');
    } catch (error) {
      toast.toast(errorMessage(error, 'Rimozione istanza non riuscita.'), 'error');
    }
  }

  return (
    <div className={styles.workbench}>
      <header className={styles.workbenchHeader}>
        <div>
          <h2>Impatto operativo</h2>
          <p>
            Modella servizi in manutenzione e effetti propagati ad altri sistemi.
          </p>
        </div>
        <div className={styles.summaryChips} aria-label="Sintesi impatto">
          <span>{operatedSelections.length} in manutenzione</span>
          <span>+{dependentSelections.length} impattati</span>
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

      <div className={styles.shell}>
        <aside className={styles.rail} aria-label="Servizi in manutenzione">
          <div className={styles.railHeader}>
            <span>In manutenzione</span>
            <span className={styles.railCount}>{operatedSelections.length}</span>
          </div>
          {canOperate ? (
            <CatalogCombobox
              options={operatedCatalog}
              excludedIds={selectedIds}
              domainHintId={detail.technical_domain.id}
              placeholder="Aggiungi servizio in manutenzione"
              onSelect={(item) => addService(item, 'operated', 'manual', 'unavailable')}
              onCreateRequest={(name) => requestCreateTaxonomy(name, 'operated')}
            />
          ) : null}
          {operatedSelections.length > 0 ? (
            <ul className={styles.railList}>
              {operatedSelections.map((item) => {
                const isSelected = item.reference.id === selectedOperatedId;
                const crossDomain =
                  item.reference.technical_domain_id != null &&
                  item.reference.technical_domain_id !== detail.technical_domain.id;
                return (
                  <li key={item.reference.id}>
                    <button
                      type="button"
                      className={`${styles.railItem} ${isSelected ? styles.railItemSelected : ''} ${crossDomain ? styles.railItemWarning : ''}`}
                      onClick={() => setSelectedOperatedId(item.reference.id)}
                    >
                      <span className={styles.railItemName}>{item.reference.name_it}</span>
                      <span className={styles.railItemMeta}>
                        {item.reference.technical_domain_name ?? 'Dominio non indicato'}
                      </span>
                    </button>
                  </li>
                );
              })}
            </ul>
          ) : (
            <div className={styles.railEmpty}>
              <Icon name="package" size={18} />
              <p>Aggiungi un servizio per iniziare.</p>
            </div>
          )}
        </aside>

        <section className={styles.detail} aria-label="Dettaglio servizio in manutenzione">
          {selectedOperated ? (
            <OperatedDetailPanel
              key={selectedOperated.reference.id}
              selection={selectedOperated}
              targets={targetsByService.get(selectedOperated.reference.id) ?? []}
              maintenanceDomainId={detail.technical_domain.id}
              canOperate={canOperate}
              busy={busy}
              onSelectionAudienceChange={(audience) =>
                updateSelection(
                  selectedOperated.reference.id,
                  { expectedAudience: audience },
                  'Destinatari aggiornati.',
                )
              }
              onSelectionSeverityChange={(severity) =>
                updateSelection(
                  selectedOperated.reference.id,
                  { expectedSeverity: severity },
                  'Severità aggiornata.',
                )
              }
              onRemoveOperated={() => removeService(selectedOperated.reference.id)}
              onCreateTarget={(displayName) =>
                void createTarget(selectedOperated.reference, displayName)
              }
              onRequestRemoveTarget={setRemoveTarget}
            />
          ) : (
            <div className={styles.detailEmpty}>
              <Icon name="box" size={28} />
              <strong>Nessun servizio selezionato</strong>
              <p>
                Aggiungi un servizio in manutenzione dalla colonna a sinistra per modellare istanze
                e severità.
              </p>
            </div>
          )}
        </section>
      </div>

      <EffectsSection
        dependentSelections={dependentSelections}
        targetsByService={targetsByService}
        dependencies={allDependencies}
        operatedSelections={operatedSelections}
        suggestions={suggestionsForMaintenance}
        suggestionsLoading={dependencies.isLoading}
        suggestionsUnavailable={Boolean(dependencies.error)}
        ignoredCount={ignoredSuggestionsCount}
        manualCatalog={reference.service_taxonomy}
        excludedIds={selectedIds}
        maintenanceDomainId={detail.technical_domain.id}
        canOperate={canOperate}
        busy={busy}
        onAcceptSuggestions={acceptSuggestions}
        onIgnoreSuggestion={ignoreSuggestion}
        onResetIgnored={resetIgnoredSuggestions}
        onAddManual={(item) => addService(item, 'dependent', 'manual', 'degraded')}
        onCreateManualRequest={(name) => requestCreateTaxonomy(name, 'dependent')}
        onSeverityChange={(serviceId, severity) =>
          updateSelection(serviceId, { expectedSeverity: severity }, 'Severità aggiornata.')
        }
        onAudienceChange={(serviceId, audience) =>
          updateSelection(serviceId, { expectedAudience: audience }, 'Destinatari aggiornati.')
        }
        onCreateTarget={(service, displayName) => void createTarget(service, displayName)}
        onRequestRemoveTarget={setRemoveTarget}
        onRemove={removeService}
      />

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
        title="Rimuovi istanza"
        message={
          removeTarget
            ? `L'istanza "${removeTarget.display_name}" sarà rimossa da questa manutenzione.`
            : "L'istanza sarà rimossa da questa manutenzione."
        }
        confirmLabel="Rimuovi"
        busy={targetMutations.remove.isPending}
        onClose={() => setRemoveTarget(null)}
        onConfirm={confirmRemoveTarget}
      />
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

function dedupeSuggestions(
  dependencies: ServiceDependency[],
  upstreamIds: Set<number>,
  takenDownstreamIds: Set<number>,
  ignoredIds: Set<number>,
): ServiceDependency[] {
  const byDownstream = new Map<number, ServiceDependency>();
  for (const dependency of dependencies) {
    if (!upstreamIds.has(dependency.upstream_service_id)) continue;
    if (takenDownstreamIds.has(dependency.downstream_service_id)) continue;
    if (ignoredIds.has(dependency.service_dependency_id)) continue;
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
