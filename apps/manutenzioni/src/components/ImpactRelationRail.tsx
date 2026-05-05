import { useMemo, useState } from 'react';
import { Button, Icon } from '@mrsmith/ui';
import type { ServiceDependency } from '../api/types';
import { dependencyTypeLabel, severityLabel } from '../lib/format';
import { impactSourceLabel, type ImpactSelectionView } from './ImpactServiceCard';
import styles from './ImpactWorkbench.module.css';

interface Props {
  operated: ImpactSelectionView[];
  dependent: ImpactSelectionView[];
  suggestions: ServiceDependency[];
  ignoredSuggestionIds: Set<number>;
  dependenciesLoading: boolean;
  dependenciesUnavailable: boolean;
  canOperate: boolean;
  onAcceptSuggestions: (items: ServiceDependency[]) => void;
  onIgnoreSuggestion: (dependencyId: number) => void;
}

export function ImpactRelationRail({
  operated,
  dependent,
  suggestions,
  ignoredSuggestionIds,
  dependenciesLoading,
  dependenciesUnavailable,
  canOperate,
  onAcceptSuggestions,
  onIgnoreSuggestion,
}: Props) {
  const [checked, setChecked] = useState<Record<number, boolean>>({});
  const visibleSuggestions = useMemo(
    () => suggestions.filter((item) => !ignoredSuggestionIds.has(item.service_dependency_id)),
    [ignoredSuggestionIds, suggestions],
  );
  const selectedSuggestions = visibleSuggestions.filter(
    (item) => checked[item.service_dependency_id],
  );

  function toggleSuggestion(id: number, value: boolean) {
    setChecked((current) => ({ ...current, [id]: value }));
  }

  function accept(items: ServiceDependency[]) {
    if (items.length === 0) return;
    onAcceptSuggestions(items);
    setChecked({});
  }

  const relations = buildRelations(operated, dependent, suggestions);

  return (
    <section className={styles.relationRail} aria-label="Relazione impatto">
      <div className={styles.railHeader}>
        <span className={styles.railIcon}>
          <Icon name="arrow-right" size={16} />
        </span>
        <div>
          <h3>Relazione</h3>
          <p>Come la manutenzione si propaga sugli altri sistemi.</p>
        </div>
      </div>

      <div className={styles.relationList}>
        {relations.length > 0 ? (
          relations.map((relation) => (
            <article key={relation.key} className={styles.relationItem}>
              <div className={styles.relationPath}>
                <span>{relation.from}</span>
                <Icon name="arrow-right" size={15} />
                <strong>{relation.to}</strong>
              </div>
              <div className={styles.relationMeta}>
                <span className={relation.suggested ? styles.badgeSuggested : styles.badgeManual}>
                  {relation.origin}
                </span>
                <span>{relation.severity}</span>
                <span>{relation.reason}</span>
              </div>
            </article>
          ))
        ) : (
          <div className={styles.relationEmpty}>
            <Icon name="link" size={18} />
            <p>
              Aggiungi almeno una voce in manutenzione e un effetto per leggere la relazione.
            </p>
          </div>
        )}
      </div>

      <div className={styles.suggestionPanel}>
        <div className={styles.suggestionHeader}>
          <h4>Suggeriti dalle dipendenze</h4>
          {dependenciesLoading ? <span>Caricamento...</span> : null}
        </div>
        {dependenciesUnavailable ? (
          <p className={styles.emptyInline}>Suggerimenti non disponibili in questo momento.</p>
        ) : visibleSuggestions.length > 0 ? (
          <>
            <ul className={styles.suggestionList}>
              {visibleSuggestions.map((item) => {
                const id = item.service_dependency_id;
                const isChecked = checked[id] === true;
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
                          Da {item.upstream_service.name_it} - {dependencyTypeLabel(item.dependency_type)}
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
                  onClick={() => accept(selectedSuggestions)}
                >
                  Aggiungi selezionati ({selectedSuggestions.length})
                </Button>
                <Button size="sm" onClick={() => accept(visibleSuggestions)}>
                  Aggiungi tutti
                </Button>
              </div>
            ) : null}
          </>
        ) : (
          <p className={styles.emptyInline}>Nessun suggerimento per le voci in manutenzione.</p>
        )}
      </div>
    </section>
  );
}

interface RelationView {
  key: string;
  from: string;
  to: string;
  origin: string;
  severity: string;
  reason: string;
  suggested: boolean;
}

function buildRelations(
  operated: ImpactSelectionView[],
  dependent: ImpactSelectionView[],
  dependencies: ServiceDependency[],
): RelationView[] {
  if (operated.length === 0 || dependent.length === 0) return [];
  const operatedIds = new Set(operated.map((item) => item.reference.id));
  const operatedFallback =
    operated.length === 1 ? operated[0]?.reference.name_it ?? 'In manutenzione' : 'In manutenzione';
  return dependent.map((item) => {
    const dependency = dependencies.find(
      (candidate) =>
        candidate.downstream_service_id === item.reference.id &&
        operatedIds.has(candidate.upstream_service_id),
    );
    const from = dependency?.upstream_service.name_it ?? operatedFallback;
    const suggested = item.source === 'dependency_graph' || Boolean(dependency);
    return {
      key: `${from}-${item.reference.id}`,
      from,
      to: item.reference.name_it,
      origin: suggested ? 'Suggerito dalle dipendenze' : impactSourceLabel(item.source),
      severity: dependency
        ? `Proposta ${severityLabel(dependency.default_severity)}`
        : severityLabel(item.expectedSeverity),
      reason: dependency ? dependencyTypeLabel(dependency.dependency_type) : 'Effetto dichiarato',
      suggested,
    };
  });
}
