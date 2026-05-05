import { useState } from 'react';
import { Button, Icon } from '@mrsmith/ui';
import type {
  AudienceOverride,
  MaintenanceTarget,
  ReferenceItem,
  SeverityValue,
} from '../api/types';
import { SeverityDropdown } from './SeverityDropdown';
import { audienceLabel, severityLabel } from '../lib/format';
import styles from './ImpactWorkbench.module.css';

export interface ImpactSelectionView {
  reference: ReferenceItem;
  source: string;
  confidence?: number | null;
  isPrimary: boolean;
  role: 'operated' | 'dependent';
  expectedSeverity: SeverityValue | null;
  expectedAudience: AudienceOverride | null;
}

interface Props {
  selection: ImpactSelectionView;
  targets: MaintenanceTarget[];
  maintenanceDomainId: number;
  canOperate: boolean;
  busy: boolean;
  onRoleChange: (serviceId: number, role: ImpactSelectionView['role']) => void;
  onSeverityChange: (serviceId: number, severity: SeverityValue | null) => void;
  onAudienceChange: (serviceId: number, audience: AudienceOverride | null) => void;
  onCreateTarget: (service: ReferenceItem, displayName: string) => void;
  onRequestRemoveTarget: (target: MaintenanceTarget) => void;
  onRemoveService: (serviceId: number) => void;
}

export function ImpactServiceCard({
  selection,
  targets,
  maintenanceDomainId,
  canOperate,
  busy,
  onRoleChange,
  onSeverityChange,
  onAudienceChange,
  onCreateTarget,
  onRequestRemoveTarget,
  onRemoveService,
}: Props) {
  const [targetDraft, setTargetDraft] = useState('');
  const service = selection.reference;
  const canCreateTarget = Boolean(service.target_type_id);
  const crossDomainOperated =
    selection.role === 'operated' &&
    service.technical_domain_id != null &&
    service.technical_domain_id !== maintenanceDomainId;
  const showAudienceSelector = service.audience === 'maintenance';

  function submitTarget() {
    const displayName = targetDraft.trim();
    if (!displayName || !canCreateTarget) return;
    onCreateTarget(service, displayName);
    setTargetDraft('');
  }

  function handleRoleChange(value: string) {
    if (value === 'operated' || value === 'dependent') {
      onRoleChange(service.id, value);
    }
  }

  return (
    <article className={`${styles.serviceCard} ${crossDomainOperated ? styles.serviceCardWarning : ''}`}>
      <div className={styles.serviceCardTop}>
        <div className={styles.serviceIdentity}>
          <span className={styles.serviceName}>{service.name_it}</span>
          <span className={styles.serviceDomain}>{service.technical_domain_name ?? 'Dominio non indicato'}</span>
        </div>
        {canOperate ? (
          <button
            type="button"
            className={styles.iconAction}
            onClick={() => onRemoveService(service.id)}
            disabled={busy}
            aria-label="Rimuovi servizio"
            title="Rimuovi servizio"
          >
            <Icon name="x" size={14} />
          </button>
        ) : null}
      </div>

      <div className={styles.serviceMetaGrid}>
        <ServiceFact label="Tipo target" value={service.target_type_name ?? 'Da definire'} />
        <ServiceFact label="Origine" value={impactSourceLabel(selection.source)} />
        <ServiceFact
          label="Audience"
          value={
            selection.expectedAudience
              ? audienceLabel(selection.expectedAudience)
              : audienceLabel(service.audience ?? '')
          }
        />
      </div>

      {crossDomainOperated ? (
        <p className={styles.warningLine}>
          Il dominio non coincide con la manutenzione. Sposta la voce negli effetti o correggi il catalogo.
        </p>
      ) : null}

      <div className={styles.serviceControls}>
        {canOperate ? (
          <label className={styles.compactField}>
            <span>Ruolo</span>
            <select
              className={styles.compactSelect}
              value={selection.role}
              onChange={(event) => handleRoleChange(event.target.value)}
              disabled={busy}
            >
              <option value="operated">In manutenzione</option>
              <option value="dependent">Effetto su altri sistemi</option>
            </select>
          </label>
        ) : (
          <ServiceFact
            label="Ruolo"
            value={selection.role === 'operated' ? 'In manutenzione' : 'Effetto su altri sistemi'}
          />
        )}
        <div className={styles.severityControl}>
          <span className={styles.fieldCaption}>Severità</span>
          {canOperate ? (
            <SeverityDropdown
              value={selection.expectedSeverity}
              onChange={(value) => onSeverityChange(service.id, value)}
            />
          ) : (
            <strong>{severityLabel(selection.expectedSeverity)}</strong>
          )}
        </div>
      </div>

      {showAudienceSelector && canOperate ? (
        <label className={styles.compactField}>
          <span>Audience da risolvere</span>
          <select
            className={styles.compactSelect}
            value={selection.expectedAudience ?? ''}
            onChange={(event) =>
              onAudienceChange(
                service.id,
                (event.target.value || null) as AudienceOverride | null,
              )
            }
            disabled={busy}
          >
            <option value="">Da definire</option>
            <option value="internal">Interna</option>
            <option value="external">Esterna</option>
            <option value="both">Interna ed esterna</option>
          </select>
        </label>
      ) : null}

      <div className={styles.targetBlock}>
        <div className={styles.targetBlockHeader}>
          <span>Target e istanze</span>
          <span>{targets.length}</span>
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
                    aria-label="Rimuovi target"
                    title="Rimuovi target"
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
    </article>
  );
}

function ServiceFact({ label, value }: { label: string; value: string }) {
  return (
    <span className={styles.serviceFact}>
      <span>{label}</span>
      <strong>{value || '-'}</strong>
    </span>
  );
}

export function impactSourceLabel(source: string): string {
  const labels: Record<string, string> = {
    manual: 'Manuale',
    import: 'Importazione',
    rule: 'Regola',
    ai_extracted: 'AI',
    ai: 'AI',
    catalog_mapping: 'Catalogo',
    dependency_graph: 'Suggerito dalle dipendenze',
    hybrid: 'Ibrido',
  };
  return labels[source] ?? source;
}
