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

  const suggestionsForSelected = useMemo(() => {
    if (!selectedOperated) return [] as ServiceDependency[];
    return dedupeSuggestions(
      allDependencies,
      selectedOperated.reference.id,
      selectedIds,
      ignoredSuggestionIds,
    );
  }, [allDependencies, ignoredSuggestionIds, selectedIds, selectedOperated]);

  const declaredForSelected = useMemo(() => {
    if (!selectedOperated) return [] as ImpactSelectionView[];
    const operatedId = selectedOperated.reference.id;
    return dependentSelections.filter((sel) =>
      allDependencies.some(
        (d) => d.upstream_service_id === operatedId && d.downstream_service_id === sel.reference.id,
      ),
    );
  }, [allDependencies, dependentSelections, selectedOperated]);

  const operatedIdSet = useMemo(
    () => new Set(operatedSelections.map((item) => item.reference.id)),
    [operatedSelections],
  );

  const orphanDependents = useMemo(() => {
    return dependentSelections.filter(
      (sel) =>
        !allDependencies.some(
          (d) => operatedIdSet.has(d.upstream_service_id) && d.downstream_service_id === sel.reference.id,
        ),
    );
  }, [allDependencies, dependentSelections, operatedIdSet]);

  const effectsCountByOperated = useMemo(() => {
    const counts = new Map<number, number>();
    const dependentIds = new Set(dependentSelections.map((sel) => sel.reference.id));
    for (const dep of allDependencies) {
      if (!operatedIdSet.has(dep.upstream_service_id)) continue;
      if (!dependentIds.has(dep.downstream_service_id)) continue;
      counts.set(dep.upstream_service_id, (counts.get(dep.upstream_service_id) ?? 0) + 1);
    }
    return counts;
  }, [allDependencies, dependentSelections, operatedIdSet]);

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
            Seleziona un servizio in manutenzione per modellarne istanze, effetti propagati e
            override.
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
                const targetsCount = (targetsByService.get(item.reference.id) ?? []).length;
                const effectsCount = effectsCountByOperated.get(item.reference.id) ?? 0;
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
                      <span className={styles.railItemStats}>
                        <span>{effectsCount} effetti</span>
                        <span>{targetsCount} istanze</span>
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
              suggestions={suggestionsForSelected}
              suggestionsLoading={dependencies.isLoading}
              suggestionsUnavailable={Boolean(dependencies.error)}
              ignoredCount={countIgnoredForOperated(
                allDependencies,
                selectedOperated.reference.id,
                ignoredSuggestionIds,
              )}
              declaredEffects={declaredForSelected.map((sel) => ({
                selection: sel,
                dependency: allDependencies.find(
                  (d) =>
                    d.upstream_service_id === selectedOperated.reference.id &&
                    d.downstream_service_id === sel.reference.id,
                ),
                targets: targetsByService.get(sel.reference.id) ?? [],
              }))}
              orphanEffects={orphanDependents.map((sel) => ({
                selection: sel,
                targets: targetsByService.get(sel.reference.id) ?? [],
              }))}
              manualEffectCatalog={reference.service_taxonomy}
              excludedEffectIds={selectedIds}
              canOperate={canOperate}
              busy={busy}
              onSelectionAudienceChange={(audience) =>
                updateSelection(
                  selectedOperated.reference.id,
                  { expectedAudience: audience },
                  'Audience aggiornata.',
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
              onAcceptSuggestions={acceptSuggestions}
              onIgnoreSuggestion={ignoreSuggestion}
              onResetIgnored={resetIgnoredSuggestions}
              onAddManualEffect={(item) => addService(item, 'dependent', 'manual', 'degraded')}
              onCreateManualEffectRequest={(name) => requestCreateTaxonomy(name, 'dependent')}
              onEffectSeverityChange={(serviceId, severity) =>
                updateSelection(serviceId, { expectedSeverity: severity }, 'Severità aggiornata.')
              }
              onEffectAudienceChange={(serviceId, audience) =>
                updateSelection(serviceId, { expectedAudience: audience }, 'Audience aggiornata.')
              }
              onEffectCreateTarget={(service, displayName) => void createTarget(service, displayName)}
              onEffectRemove={removeService}
            />
          ) : (
            <div className={styles.detailEmpty}>
              <Icon name="box" size={28} />
              <strong>Nessun servizio selezionato</strong>
              <p>
                Aggiungi un servizio in manutenzione dalla colonna a sinistra per modellare istanze
                e propagazione degli effetti.
              </p>
            </div>
          )}
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
  upstreamId: number,
  selectedIds: Set<number>,
  ignoredIds: Set<number>,
): ServiceDependency[] {
  const byDownstream = new Map<number, ServiceDependency>();
  for (const dependency of dependencies) {
    if (dependency.upstream_service_id !== upstreamId) continue;
    if (selectedIds.has(dependency.downstream_service_id)) continue;
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

function countIgnoredForOperated(
  dependencies: ServiceDependency[],
  upstreamId: number,
  ignoredIds: Set<number>,
): number {
  let count = 0;
  for (const dep of dependencies) {
    if (dep.upstream_service_id === upstreamId && ignoredIds.has(dep.service_dependency_id)) count += 1;
  }
  return count;
}

function severityRank(value: SeverityValue): number {
  if (value === 'unavailable') return 3;
  if (value === 'degraded') return 2;
  return 1;
}
