import { useState } from 'react';
import { Button, Icon } from '@mrsmith/ui';
import type {
  AudienceOverride,
  MaintenanceTarget,
  ServiceDependency,
  SeverityValue,
} from '../api/types';
import { SeverityDropdown } from './SeverityDropdown';
import { audienceLabel, dependencyTypeLabel, severityLabel } from '../lib/format';
import { impactSourceLabel, type ImpactSelectionView } from './impactTypes';
import styles from './ImpactWorkbench.module.css';

export interface EffectAttribution {
  operatedName: string;
  dependencyType: ServiceDependency['dependency_type'];
}

interface Props {
  selection: ImpactSelectionView;
  targets: MaintenanceTarget[];
  attribution?: EffectAttribution;
  canOperate: boolean;
  busy: boolean;
  onSeverityChange: (severity: SeverityValue | null) => void;
  onAudienceChange: (audience: AudienceOverride | null) => void;
  onCreateTarget: (displayName: string) => void;
  onRequestRemoveTarget: (target: MaintenanceTarget) => void;
  onRemove: () => void;
}

export function EffectRow({
  selection,
  targets,
  attribution,
  canOperate,
  busy,
  onSeverityChange,
  onAudienceChange,
  onCreateTarget,
  onRequestRemoveTarget,
  onRemove,
}: Props) {
  const [expanded, setExpanded] = useState(false);
  const [targetDraft, setTargetDraft] = useState('');
  const service = selection.reference;
  const canCreateTarget = Boolean(service.target_type_id);
  const showAudienceSelector = service.audience === 'maintenance';
  const showSourceBadge = selection.source !== 'manual';

  function submitTarget() {
    const displayName = targetDraft.trim();
    if (!displayName || !canCreateTarget) return;
    onCreateTarget(displayName);
    setTargetDraft('');
  }

  return (
    <li className={styles.effectRow}>
      <div className={styles.effectSummary}>
        <button
          type="button"
          className={styles.effectToggle}
          onClick={() => setExpanded((current) => !current)}
          aria-expanded={expanded}
          aria-label={expanded ? 'Comprimi effetto' : 'Espandi effetto'}
        >
          <Icon name={expanded ? 'chevron-down' : 'chevron-right'} size={14} />
        </button>
        <div className={styles.effectIdentity}>
          <strong>{service.name_it}</strong>
          <span>{service.technical_domain_name ?? 'Dominio non indicato'}</span>
        </div>
        <div className={styles.effectMeta}>
          {canOperate ? (
            <SeverityDropdown
              value={selection.expectedSeverity}
              onChange={onSeverityChange}
              disabled={busy}
            />
          ) : (
            <span className={styles.suggestionSeverity}>
              {severityLabel(selection.expectedSeverity)}
            </span>
          )}
          {targets.length > 0 ? (
            <span className={styles.effectInstanceCount}>{targets.length} ist.</span>
          ) : null}
          {attribution ? (
            <span className={styles.badgeAttribution}>
              via {attribution.operatedName} · {dependencyTypeLabel(attribution.dependencyType)}
            </span>
          ) : null}
          {showSourceBadge ? (
            <span className={styles.badgeSuggested}>{impactSourceLabel(selection.source)}</span>
          ) : null}
        </div>
        {canOperate ? (
          <button
            type="button"
            className={styles.iconAction}
            onClick={onRemove}
            disabled={busy}
            aria-label="Rimuovi effetto"
            title="Rimuovi effetto"
          >
            <Icon name="x" size={14} />
          </button>
        ) : null}
      </div>

      {expanded ? (
        <div className={styles.effectExpanded}>
          {showAudienceSelector ? (
            <label className={styles.compactField}>
              <span>Destinatari</span>
              <select
                className={styles.compactSelect}
                value={selection.expectedAudience ?? ''}
                onChange={(event) =>
                  onAudienceChange((event.target.value || null) as AudienceOverride | null)
                }
                disabled={busy || !canOperate}
              >
                <option value="">Da definire</option>
                <option value="internal">Solo utenti interni</option>
                <option value="external">Clienti ed altre entità esterne</option>
                <option value="both">Utenti interni e Clienti</option>
              </select>
              {selection.expectedAudience ? null : (
                <small className={styles.controlHint}>
                  Default catalogo: {audienceLabel(service.audience ?? '')}
                </small>
              )}
            </label>
          ) : null}

          <div className={styles.effectTargets}>
            <span className={styles.miniLabel}>Istanze ({targets.length})</span>
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
          </div>
        </div>
      ) : null}
    </li>
  );
}
