import { useState } from 'react';
import { Button, Icon } from '@mrsmith/ui';
import type { AudienceOverride, MaintenanceTarget, SeverityValue } from '../api/types';
import { SeverityDropdown } from './SeverityDropdown';
import { audienceLabel, severityLabel } from '../lib/format';
import type { ImpactSelectionView } from './impactTypes';
import styles from './ImpactWorkbench.module.css';

interface Props {
  selection: ImpactSelectionView;
  targets: MaintenanceTarget[];
  maintenanceDomainId: number;
  canOperate: boolean;
  busy: boolean;
  onSelectionAudienceChange: (audience: AudienceOverride | null) => void;
  onSelectionSeverityChange: (severity: SeverityValue | null) => void;
  onRemoveOperated: () => void;
  onCreateTarget: (displayName: string) => void;
  onRequestRemoveTarget: (target: MaintenanceTarget) => void;
}

export function OperatedDetailPanel({
  selection,
  targets,
  maintenanceDomainId,
  canOperate,
  busy,
  onSelectionAudienceChange,
  onSelectionSeverityChange,
  onRemoveOperated,
  onCreateTarget,
  onRequestRemoveTarget,
}: Props) {
  const service = selection.reference;
  const [targetDraft, setTargetDraft] = useState('');
  const showAudienceSelector = service.audience === 'maintenance';
  const canCreateTarget = Boolean(service.target_type_id);
  const crossDomain =
    service.technical_domain_id != null && service.technical_domain_id !== maintenanceDomainId;
  const targetTypeName = service.target_type_name ?? null;
  const audienceCurrent = selection.expectedAudience ?? null;

  function submitTarget() {
    const displayName = targetDraft.trim();
    if (!displayName || !canCreateTarget) return;
    onCreateTarget(displayName);
    setTargetDraft('');
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
        <div className={styles.blockHeaderInline}>
          <div className={styles.blockHeaderTitle}>
            <span>Istanze</span>
            <span className={styles.blockCount}>{targets.length}</span>
          </div>
          {canOperate && canCreateTarget ? (
            <div className={styles.targetCreateInline}>
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
          ) : null}
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
        {canOperate && !canCreateTarget ? (
          <p className={styles.warningLine}>
            Tipo target mancante: aggiunta istanza non disponibile.
          </p>
        ) : null}
      </section>

      <section className={styles.detailBlock}>
        <div className={styles.detailControls}>
          <div className={styles.detailControl}>
            <div className={styles.blockHeader}>
              <span>Severità</span>
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
          </div>
          {showAudienceSelector ? (
            <div className={styles.detailControl}>
              <div className={styles.blockHeader}>
                <span>Destinatari</span>
              </div>
              {canOperate ? (
                <>
                  <select
                    className={styles.compactSelect}
                    value={audienceCurrent ?? ''}
                    onChange={(event) =>
                      onSelectionAudienceChange((event.target.value || null) as AudienceOverride | null)
                    }
                    disabled={busy}
                  >
                    <option value="">Da definire</option>
                    <option value="internal">Solo utenti interni</option>
                    <option value="external">Clienti ed altre entità esterne</option>
                    <option value="both">Utenti interni e Clienti</option>
                  </select>
                  <span className={styles.controlHint}>
                    {audienceCurrent
                      ? 'Override impostato per questa manutenzione.'
                      : 'Default catalogo: ' + audienceLabel(service.audience ?? '')}
                  </span>
                </>
              ) : (
                <p className={styles.emptyInline}>
                  {audienceCurrent ? audienceLabel(audienceCurrent) : audienceLabel(service.audience ?? '')}
                </p>
              )}
            </div>
          ) : null}
        </div>
      </section>
    </div>
  );
}
