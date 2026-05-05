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
import { EffectRow } from './EffectRow';
import { SeverityDropdown } from './SeverityDropdown';
import { audienceLabel, dependencyTypeLabel, severityLabel } from '../lib/format';
import type { ImpactSelectionView } from './impactTypes';
import styles from './ImpactWorkbench.module.css';

export interface DeclaredEffect {
  selection: ImpactSelectionView;
  dependency?: ServiceDependency;
  targets: MaintenanceTarget[];
}

export interface OrphanEffect {
  selection: ImpactSelectionView;
  targets: MaintenanceTarget[];
}

interface Props {
  selection: ImpactSelectionView;
  targets: MaintenanceTarget[];
  maintenanceDomainId: number;
  suggestions: ServiceDependency[];
  suggestionsLoading: boolean;
  suggestionsUnavailable: boolean;
  ignoredCount: number;
  declaredEffects: DeclaredEffect[];
  orphanEffects: OrphanEffect[];
  manualEffectCatalog: ReferenceItem[];
  excludedEffectIds: Set<number>;
  canOperate: boolean;
  busy: boolean;
  onSelectionAudienceChange: (audience: AudienceOverride | null) => void;
  onSelectionSeverityChange: (severity: SeverityValue | null) => void;
  onRemoveOperated: () => void;
  onCreateTarget: (displayName: string) => void;
  onRequestRemoveTarget: (target: MaintenanceTarget) => void;
  onAcceptSuggestions: (items: ServiceDependency[]) => void;
  onIgnoreSuggestion: (dependencyId: number) => void;
  onResetIgnored: () => void;
  onAddManualEffect: (item: ReferenceItem) => void;
  onCreateManualEffectRequest: (initialName: string) => void;
  onEffectSeverityChange: (serviceId: number, severity: SeverityValue | null) => void;
  onEffectAudienceChange: (serviceId: number, audience: AudienceOverride | null) => void;
  onEffectCreateTarget: (service: ReferenceItem, displayName: string) => void;
  onEffectRemove: (serviceId: number) => void;
}

export function OperatedDetailPanel({
  selection,
  targets,
  maintenanceDomainId,
  suggestions,
  suggestionsLoading,
  suggestionsUnavailable,
  ignoredCount,
  declaredEffects,
  orphanEffects,
  manualEffectCatalog,
  excludedEffectIds,
  canOperate,
  busy,
  onSelectionAudienceChange,
  onSelectionSeverityChange,
  onRemoveOperated,
  onCreateTarget,
  onRequestRemoveTarget,
  onAcceptSuggestions,
  onIgnoreSuggestion,
  onResetIgnored,
  onAddManualEffect,
  onCreateManualEffectRequest,
  onEffectSeverityChange,
  onEffectAudienceChange,
  onEffectCreateTarget,
  onEffectRemove,
}: Props) {
  const service = selection.reference;
  const [targetDraft, setTargetDraft] = useState('');
  const [checkedSuggestions, setCheckedSuggestions] = useState<Record<number, boolean>>({});
  const [manualPickerOpen, setManualPickerOpen] = useState(false);
  const showAudienceSelector = service.audience === 'maintenance';
  const canCreateTarget = Boolean(service.target_type_id);
  const crossDomain =
    service.technical_domain_id != null && service.technical_domain_id !== maintenanceDomainId;
  const targetTypeName = service.target_type_name ?? null;
  const audienceCurrent = selection.expectedAudience ?? null;

  const selectedSuggestions = useMemo(
    () => suggestions.filter((item) => checkedSuggestions[item.service_dependency_id]),
    [checkedSuggestions, suggestions],
  );

  function submitTarget() {
    const displayName = targetDraft.trim();
    if (!displayName || !canCreateTarget) return;
    onCreateTarget(displayName);
    setTargetDraft('');
  }

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
    <div className={`${styles.detailInner} ${crossDomain ? styles.detailWarning : ''}`}>
      <header className={styles.detailHeader}>
        <div className={styles.detailIdentity}>
          <h3>{service.name_it}</h3>
          <p>
            {service.technical_domain_name ?? 'Dominio non indicato'}
            {targetTypeName ? <> · {targetTypeName}</> : null}
          </p>
        </div>
        {canOperate ? (
          <button
            type="button"
            className={styles.iconAction}
            onClick={onRemoveOperated}
            disabled={busy}
            aria-label="Rimuovi dalla manutenzione"
            title="Rimuovi dalla manutenzione"
          >
            <Icon name="trash" size={14} />
          </button>
        ) : null}
      </header>

      {crossDomain ? (
        <p className={styles.warningLine}>
          Il dominio non coincide con la manutenzione. Sposta la voce negli effetti o correggi il
          catalogo.
        </p>
      ) : null}

      <section className={styles.detailBlock}>
        <div className={styles.blockHeader}>
          <span>Istanze</span>
          <span className={styles.blockCount}>{targets.length}</span>
        </div>
        {targets.length > 0 ? (
          <ul className={styles.targetList}>
            {targets.map((target) => (
              <li key={target.maintenance_target_id} className={styles.targetItem}>
                <span className={styles.targetMain}>
                  <strong>{target.display_name}</strong>
                  <span>{target.target_type.name_it}</span>
                </span>
                {canOperate ? (
                  <button
                    type="button"
                    className={styles.iconAction}
                    onClick={() => onRequestRemoveTarget(target)}
                    disabled={busy}
                    aria-label="Rimuovi istanza"
                    title="Rimuovi istanza"
                  >
                    <Icon name="trash" size={13} />
                  </button>
                ) : null}
              </li>
            ))}
          </ul>
        ) : (
          <p className={styles.emptyInline}>Nessuna istanza collegata.</p>
        )}
        {canOperate ? (
          canCreateTarget ? (
            <div className={styles.targetCreate}>
              <input
                className={styles.targetInput}
                value={targetDraft}
                onChange={(event) => setTargetDraft(event.target.value)}
                onKeyDown={(event) => {
                  if (event.key === 'Enter') {
                    event.preventDefault();
                    submitTarget();
                  }
                }}
                placeholder="Aggiungi istanza"
                disabled={busy}
              />
              <Button
                size="sm"
                variant="secondary"
                onClick={submitTarget}
                disabled={!targetDraft.trim() || busy}
                leftIcon={<Icon name="plus" size={14} />}
              >
                Aggiungi
              </Button>
            </div>
          ) : (
            <p className={styles.warningLine}>
              Tipo target mancante: aggiunta istanza non disponibile.
            </p>
          )
        ) : null}
      </section>

      {showAudienceSelector ? (
        <section className={styles.detailBlock}>
          <div className={styles.blockHeader}>
            <span>Destinatari</span>
          </div>
          {canOperate ? (
            <div className={styles.controlRow}>
              <select
                className={styles.compactSelect}
                value={audienceCurrent ?? ''}
                onChange={(event) =>
                  onSelectionAudienceChange((event.target.value || null) as AudienceOverride | null)
                }
                disabled={busy}
              >
                <option value="">Da definire</option>
                <option value="internal">Interna</option>
                <option value="external">Esterna</option>
                <option value="both">Interna ed esterna</option>
              </select>
              <span className={styles.controlHint}>
                {audienceCurrent
                  ? 'Override impostato per questa manutenzione.'
                  : 'Default catalogo: ' + audienceLabel(service.audience ?? '')}
              </span>
            </div>
          ) : (
            <p className={styles.emptyInline}>
              {audienceCurrent ? audienceLabel(audienceCurrent) : audienceLabel(service.audience ?? '')}
            </p>
          )}
        </section>
      ) : null}

      <section className={styles.detailBlock}>
        <div className={styles.blockHeader}>
          <span>Severità del servizio in manutenzione</span>
        </div>
        {canOperate ? (
          <SeverityDropdown
            value={selection.expectedSeverity}
            onChange={onSelectionSeverityChange}
            disabled={busy}
          />
        ) : (
          <p className={styles.emptyInline}>{severityLabel(selection.expectedSeverity)}</p>
        )}
      </section>

      <div className={styles.sectionDivider}>
        <span>Effetti propagati</span>
      </div>

      <section className={styles.detailBlock}>
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
                        <small>{dependencyTypeLabel(item.dependency_type)}</small>
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
        ) : (
          <p className={styles.emptyInline}>
            Nessun suggerimento dal dependency graph per questo servizio.
          </p>
        )}
        {ignoredCount > 0 ? (
          <button
            type="button"
            className={styles.linkAction}
            onClick={onResetIgnored}
          >
            {ignoredCount} suggerimenti ignorati · ripristina
          </button>
        ) : null}
      </section>

      <section className={styles.detailBlock}>
        <div className={styles.blockHeader}>
          <span>Dichiarati per questo servizio</span>
          <span className={styles.blockCount}>{declaredEffects.length}</span>
        </div>
        {declaredEffects.length > 0 ? (
          <ul className={styles.effectList}>
            {declaredEffects.map((entry) => (
              <EffectRow
                key={entry.selection.reference.id}
                selection={entry.selection}
                targets={entry.targets}
                dependency={entry.dependency}
                canOperate={canOperate}
                busy={busy}
                onSeverityChange={(severity) =>
                  onEffectSeverityChange(entry.selection.reference.id, severity)
                }
                onAudienceChange={(audience) =>
                  onEffectAudienceChange(entry.selection.reference.id, audience)
                }
                onCreateTarget={(displayName) =>
                  onEffectCreateTarget(entry.selection.reference, displayName)
                }
                onRequestRemoveTarget={onRequestRemoveTarget}
                onRemove={() => onEffectRemove(entry.selection.reference.id)}
              />
            ))}
          </ul>
        ) : (
          <p className={styles.emptyInline}>
            Nessun effetto dichiarato per questo servizio. Accetta un suggerimento o aggiungi
            manualmente.
          </p>
        )}
        {canOperate ? (
          manualPickerOpen ? (
            <div className={styles.manualPicker}>
              <CatalogCombobox
                options={manualEffectCatalog}
                excludedIds={excludedEffectIds}
                domainHintId={maintenanceDomainId}
                placeholder="Aggiungi effetto manuale"
                onSelect={(item) => {
                  onAddManualEffect(item);
                  setManualPickerOpen(false);
                }}
                onCreateRequest={(name) => {
                  onCreateManualEffectRequest(name);
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
          ) : (
            <button
              type="button"
              className={styles.addManual}
              onClick={() => setManualPickerOpen(true)}
            >
              <Icon name="plus" size={14} />
              Aggiungi effetto manuale
            </button>
          )
        ) : null}
      </section>

      {orphanEffects.length > 0 ? (
        <section className={styles.detailBlock}>
          <div className={styles.blockHeader}>
            <span>Senza relazione con i servizi in manutenzione</span>
            <span className={styles.blockCount}>{orphanEffects.length}</span>
          </div>
          <p className={styles.helperLine}>
            Questi effetti non hanno una propagazione nota dal dependency graph. Visibili a livello
            di manutenzione.
          </p>
          <ul className={styles.effectList}>
            {orphanEffects.map((entry) => (
              <EffectRow
                key={entry.selection.reference.id}
                selection={entry.selection}
                targets={entry.targets}
                canOperate={canOperate}
                busy={busy}
                onSeverityChange={(severity) =>
                  onEffectSeverityChange(entry.selection.reference.id, severity)
                }
                onAudienceChange={(audience) =>
                  onEffectAudienceChange(entry.selection.reference.id, audience)
                }
                onCreateTarget={(displayName) =>
                  onEffectCreateTarget(entry.selection.reference, displayName)
                }
                onRequestRemoveTarget={onRequestRemoveTarget}
                onRemove={() => onEffectRemove(entry.selection.reference.id)}
              />
            ))}
          </ul>
        </section>
      ) : null}
    </div>
  );
}
