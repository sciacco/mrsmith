import { useMemo, useState } from 'react';
import { Button, Icon } from '@mrsmith/ui';
import type {
  AudienceOverride,
  MaintenanceTarget,
  ReferenceItem,
  ServiceDependency,
  SeverityValue,
} from '../api/types';
import { CatalogCombobox } from './CatalogCombobox';
import { EffectRow, type EffectAttribution } from './EffectRow';
import { dependencyTypeLabel, severityLabel } from '../lib/format';
import type { ImpactSelectionView } from './impactTypes';
import styles from './ImpactWorkbench.module.css';

interface Props {
  dependentSelections: ImpactSelectionView[];
  targetsByService: Map<number, MaintenanceTarget[]>;
  dependencies: ServiceDependency[];
  operatedSelections: ImpactSelectionView[];
  suggestions: ServiceDependency[];
  suggestionsLoading: boolean;
  suggestionsUnavailable: boolean;
  ignoredCount: number;
  manualCatalog: ReferenceItem[];
  excludedIds: Set<number>;
  maintenanceDomainId: number;
  canOperate: boolean;
  busy: boolean;
  onAcceptSuggestions: (items: ServiceDependency[]) => void;
  onIgnoreSuggestion: (dependencyId: number) => void;
  onResetIgnored: () => void;
  onAddManual: (item: ReferenceItem) => void;
  onCreateManualRequest: (initialName: string) => void;
  onSeverityChange: (serviceId: number, severity: SeverityValue | null) => void;
  onAudienceChange: (serviceId: number, audience: AudienceOverride | null) => void;
  onCreateTarget: (service: ReferenceItem, displayName: string) => void;
  onRequestRemoveTarget: (target: MaintenanceTarget) => void;
  onRemove: (serviceId: number) => void;
}

export function EffectsSection({
  dependentSelections,
  targetsByService,
  dependencies,
  operatedSelections,
  suggestions,
  suggestionsLoading,
  suggestionsUnavailable,
  ignoredCount,
  manualCatalog,
  excludedIds,
  maintenanceDomainId,
  canOperate,
  busy,
  onAcceptSuggestions,
  onIgnoreSuggestion,
  onResetIgnored,
  onAddManual,
  onCreateManualRequest,
  onSeverityChange,
  onAudienceChange,
  onCreateTarget,
  onRequestRemoveTarget,
  onRemove,
}: Props) {
  const [checkedSuggestions, setCheckedSuggestions] = useState<Record<number, boolean>>({});
  const [manualPickerOpen, setManualPickerOpen] = useState(false);

  const operatedById = useMemo(() => {
    const map = new Map<number, ImpactSelectionView>();
    for (const op of operatedSelections) map.set(op.reference.id, op);
    return map;
  }, [operatedSelections]);

  const orderedEffects = useMemo(() => {
    const items = dependentSelections.map((sel) => {
      const edge = dependencies.find(
        (d) => d.downstream_service_id === sel.reference.id && operatedById.has(d.upstream_service_id),
      );
      const attribution: EffectAttribution | undefined = edge
        ? {
            operatedName:
              operatedById.get(edge.upstream_service_id)?.reference.name_it ??
              edge.upstream_service.name_it,
            dependencyType: edge.dependency_type,
          }
        : undefined;
      return { selection: sel, attribution };
    });
    items.sort((a, b) => {
      const sa = severityRank(a.selection.expectedSeverity);
      const sb = severityRank(b.selection.expectedSeverity);
      if (sa !== sb) return sb - sa;
      return a.selection.reference.name_it.localeCompare(b.selection.reference.name_it);
    });
    return items;
  }, [dependencies, dependentSelections, operatedById]);

  const selectedSuggestions = useMemo(
    () => suggestions.filter((item) => checkedSuggestions[item.service_dependency_id]),
    [checkedSuggestions, suggestions],
  );

  const showSuggestionsBanner =
    suggestionsLoading ||
    suggestionsUnavailable ||
    suggestions.length > 0 ||
    ignoredCount > 0;

  function toggleSuggestion(id: number, value: boolean) {
    setCheckedSuggestions((current) => ({ ...current, [id]: value }));
  }

  function handleAcceptSelected() {
    if (selectedSuggestions.length === 0) return;
    onAcceptSuggestions(selectedSuggestions);
    setCheckedSuggestions({});
  }

  function handleAcceptAll() {
    if (suggestions.length === 0) return;
    onAcceptSuggestions(suggestions);
    setCheckedSuggestions({});
  }

  return (
    <section className={styles.effectsSection} aria-label="La manutenzione può impattare anche">
      <div className={styles.effectsHeader}>
        <div className={styles.effectsTitle}>
          <h3>La manutenzione può impattare anche</h3>
          <span className={styles.blockCount}>{dependentSelections.length}</span>
        </div>
        {canOperate ? (
          manualPickerOpen ? null : (
            <Button
              size="sm"
              variant="secondary"
              leftIcon={<Icon name="plus" size={14} />}
              onClick={() => setManualPickerOpen(true)}
              disabled={busy}
            >
              Aggiungi
            </Button>
          )
        ) : null}
      </div>

      {manualPickerOpen ? (
        <div className={styles.effectsPicker}>
          <CatalogCombobox
            options={manualCatalog}
            excludedIds={excludedIds}
            domainHintId={maintenanceDomainId}
            placeholder="Cerca o crea un servizio impattato"
            onSelect={(item) => {
              onAddManual(item);
              setManualPickerOpen(false);
            }}
            onCreateRequest={(name) => {
              onCreateManualRequest(name);
              setManualPickerOpen(false);
            }}
          />
          <button
            type="button"
            className={styles.linkAction}
            onClick={() => setManualPickerOpen(false)}
          >
            Annulla
          </button>
        </div>
      ) : null}

      {showSuggestionsBanner ? (
        <div className={styles.suggestionsBanner}>
          <div className={styles.blockHeader}>
            <span>Suggeriti dal dependency graph</span>
            <span className={styles.blockCount}>{suggestions.length}</span>
          </div>
          {suggestionsUnavailable ? (
            <p className={styles.emptyInline}>Suggerimenti non disponibili in questo momento.</p>
          ) : suggestionsLoading ? (
            <p className={styles.emptyInline}>Caricamento suggerimenti…</p>
          ) : suggestions.length > 0 ? (
            <>
              <ul className={styles.suggestionList}>
                {suggestions.map((item) => {
                  const id = item.service_dependency_id;
                  const isChecked = checkedSuggestions[id] === true;
                  const upstream =
                    operatedById.get(item.upstream_service_id)?.reference.name_it ??
                    item.upstream_service.name_it;
                  return (
                    <li key={id} className={styles.suggestionItem}>
                      <label className={styles.suggestionMain}>
                        <input
                          type="checkbox"
                          checked={isChecked}
                          disabled={!canOperate}
                          onChange={(event) => toggleSuggestion(id, event.target.checked)}
                        />
                        <span>
                          <strong>{item.downstream_service.name_it}</strong>
                          <small>
                            via {upstream} · {dependencyTypeLabel(item.dependency_type)}
                          </small>
                        </span>
                      </label>
                      <span className={styles.suggestionSeverity}>
                        {severityLabel(item.default_severity)}
                      </span>
                      {canOperate ? (
                        <button
                          type="button"
                          className={styles.iconAction}
                          onClick={() => onIgnoreSuggestion(id)}
                          aria-label="Ignora suggerimento"
                          title="Ignora suggerimento"
                        >
                          <Icon name="x" size={13} />
                        </button>
                      ) : null}
                    </li>
                  );
                })}
              </ul>
              {canOperate ? (
                <div className={styles.suggestionActions}>
                  <Button
                    size="sm"
                    variant="secondary"
                    disabled={selectedSuggestions.length === 0}
                    onClick={handleAcceptSelected}
                  >
                    Accetta selezionati ({selectedSuggestions.length})
                  </Button>
                  <Button size="sm" onClick={handleAcceptAll}>
                    Accetta tutti
                  </Button>
                </div>
              ) : null}
            </>
          ) : null}
          {ignoredCount > 0 ? (
            <button type="button" className={styles.linkAction} onClick={onResetIgnored}>
              {ignoredCount} suggerimenti ignorati · ripristina
            </button>
          ) : null}
        </div>
      ) : null}

      {orderedEffects.length > 0 ? (
        <ul className={styles.effectList}>
          {orderedEffects.map((entry) => (
            <EffectRow
              key={entry.selection.reference.id}
              selection={entry.selection}
              targets={targetsByService.get(entry.selection.reference.id) ?? []}
              attribution={entry.attribution}
              canOperate={canOperate}
              busy={busy}
              onSeverityChange={(severity) => onSeverityChange(entry.selection.reference.id, severity)}
              onAudienceChange={(audience) => onAudienceChange(entry.selection.reference.id, audience)}
              onCreateTarget={(displayName) => onCreateTarget(entry.selection.reference, displayName)}
              onRequestRemoveTarget={onRequestRemoveTarget}
              onRemove={() => onRemove(entry.selection.reference.id)}
            />
          ))}
        </ul>
      ) : (
        <p className={styles.emptyInline}>
          Nessun effetto propagato. Aggiungi i servizi che ritieni impattati dalla manutenzione.
        </p>
      )}
    </section>
  );
}

function severityRank(value: SeverityValue | null | undefined): number {
  if (value === 'unavailable') return 3;
  if (value === 'degraded') return 2;
  if (value === 'none') return 1;
  return 0;
}
