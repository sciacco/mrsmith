import { Icon, Skeleton } from '@mrsmith/ui';
import type { MaintenanceCockpit as MaintenanceCockpitData, MaintenanceDetail } from '../api/types';
import {
  audienceLabel,
  dependencyTypeLabel,
  errorMessage,
  formatDateTime,
  sendStatusLabel,
  severityLabel,
  windowStatusLabel,
} from '../lib/format';
import { MAINTENANCE_EVENT_LABELS } from '../lib/labels';
import shared from '../pages/shared.module.css';
import styles from './MaintenanceCockpit.module.css';

interface Props {
  detail: MaintenanceDetail;
  cockpit?: MaintenanceCockpitData;
  isLoading: boolean;
  error: unknown;
  onTabChange: (tab: string) => void;
}

export function MaintenanceCockpit({
  detail,
  cockpit,
  isLoading,
  error,
  onTabChange,
}: Props) {
  if (isLoading) {
    return (
      <div className={shared.panel}>
        <Skeleton rows={8} />
      </div>
    );
  }

  if (error || !cockpit) {
    return (
      <div className={shared.emptyCard}>
        <div className={shared.emptyIconDanger}>
          <Icon name="triangle-alert" />
        </div>
        <h3>Cruscotto non disponibile</h3>
        <p>{errorMessage(error, 'Impossibile caricare la vista operativa.')}</p>
      </div>
    );
  }

  const nextAction = cockpit.next_action;
  const blockingCount = nextAction?.blocked_by.length ?? cockpit.readiness.filter((item) => item.blocking).length;
  const readinessOpenCount = cockpit.readiness.filter((item) => item.status === 'blocking').length;

  return (
    <section className={styles.cockpit} aria-label="Cruscotto manutenzione">
      <div className={styles.heroGrid}>
        <div className={styles.runwayPanel}>
          <div className={styles.panelHeader}>
            <div>
              <h3>Runway operativa</h3>
              <p>{nextActionSummary(nextAction, blockingCount)}</p>
            </div>
          </div>
          <div className={styles.runway}>
            {cockpit.lifecycle.map((step, index) => (
              <div key={step.key} className={styles.runwayStep} data-state={step.state}>
                <span className={styles.stepDot}>
                  {step.state === 'complete' ? <Icon name="check" size={13} /> : index + 1}
                </span>
                <span>{step.label}</span>
              </div>
            ))}
          </div>
        </div>

        <div className={styles.nextPanel} data-blocked={blockingCount > 0}>
          <span className={styles.nextEyebrow}>Prossima azione</span>
          <strong>{nextAction ? nextAction.label : finalStateLabel(detail.status)}</strong>
          <p>{nextActionDetail(nextAction, blockingCount)}</p>
          {nextAction && nextAction.blocked_by.length > 0 ? (
            <button
              type="button"
              className={styles.panelLink}
              onClick={() => onTabChange(firstBlockingTab(cockpit))}
            >
              Risolvi i blocchi
              <Icon name="chevron-right" size={14} />
            </button>
          ) : null}
        </div>
      </div>

      <div className={styles.workspaceGrid}>
        <section className={styles.readinessPanel}>
          <div className={styles.panelHeader}>
            <div>
              <h3>Readiness</h3>
              <p>
                {readinessOpenCount === 0
                  ? 'La manutenzione è pronta per il prossimo passaggio.'
                  : `${readinessOpenCount} punti da completare`}
              </p>
            </div>
          </div>
          <div className={styles.readinessList}>
            {cockpit.readiness.map((item) => (
              <button
                key={item.key}
                type="button"
                className={styles.readinessRow}
                data-status={item.status}
                onClick={() => onTabChange(item.target_tab)}
              >
                <span className={styles.readinessIcon}>
                  <Icon name={readinessIcon(item.status)} size={16} />
                </span>
                <span className={styles.readinessText}>
                  <strong>{item.label}</strong>
                  <span>{item.summary}</span>
                </span>
                <Icon name="chevron-right" size={15} />
              </button>
            ))}
          </div>
        </section>

        <section className={styles.impactPanel}>
          <div className={styles.panelHeader}>
            <div>
              <h3>Mappa impatti</h3>
              <p>
                {cockpit.impact.summary.services} servizi · {cockpit.impact.summary.targets} target ·{' '}
                {cockpit.impact.summary.customers} clienti
              </p>
            </div>
            <button type="button" className={styles.panelLink} onClick={() => onTabChange('impatto')}>
              Apri impatto
              <Icon name="chevron-right" size={14} />
            </button>
          </div>
          <ImpactMap cockpit={cockpit} onTabChange={onTabChange} />
        </section>

        <section className={styles.timelinePanel}>
          <div className={styles.panelHeader}>
            <div>
              <h3>Timeline</h3>
            </div>
          </div>
          <div className={styles.timeline}>
            {cockpit.timeline.length === 0 ? (
              <div className={styles.emptyInline}>Aggiungi una finestra o una comunicazione per costruire la timeline.</div>
            ) : (
              cockpit.timeline.slice(0, 7).map((item) => (
                <button
                  key={item.id}
                  type="button"
                  className={styles.timelineItem}
                  data-kind={item.kind}
                  data-status={item.status}
                  onClick={() => onTabChange(item.target_tab)}
                >
                  <span className={styles.timelineDot} data-kind={item.kind} data-status={item.status} />
                  <span className={styles.timelineText}>
                    <strong>{timelineLabel(item.kind, item.label)}</strong>
                    <small>{timelineMeta(item, timelineLabel(item.kind, item.label))}</small>
                  </span>
                </button>
              ))
            )}
          </div>
        </section>
      </div>
    </section>
  );
}

function ImpactMap({ cockpit, onTabChange }: { cockpit: MaintenanceCockpitData; onTabChange: (tab: string) => void }) {
  const operated = cockpit.impact.operated_services.slice(0, 4);
  const dependent = cockpit.impact.dependent_services.slice(0, 5);
  const dependencies = cockpit.impact.dependencies.slice(0, 5);

  return (
    <div className={styles.impactMap}>
      <div className={styles.mapColumn}>
        <span className={styles.mapLabel}>Servizi operati</span>
        {operated.length === 0 ? (
          <div className={styles.emptyInline}>Nessun servizio operato.</div>
        ) : (
          operated.map((item) => (
            <button key={item.id} type="button" className={styles.serviceNode} onClick={() => onTabChange('impatto')}>
              <strong>{item.reference.name_it}</strong>
              <span>{item.expected_audience ? audienceLabel(item.expected_audience) : audienceLabel(item.reference.audience ?? '')}</span>
            </button>
          ))
        )}
      </div>
      <div className={styles.mapBridge}>
        <span />
        <Icon name="arrow-right" size={18} />
      </div>
      <div className={styles.mapColumn}>
        <span className={styles.mapLabel}>Impatti attesi</span>
        {[...dependent.map((item) => ({ key: `dep-${item.id}`, label: item.reference.name_it, meta: severityLabel(item.expected_severity) })),
          ...dependencies.map((item) => ({
            key: `graph-${item.service_dependency_id}`,
            label: item.downstream_service.name_it,
            meta: `${dependencyTypeLabel(item.dependency_type)} · ${severityLabel(item.default_severity)}`,
          }))].slice(0, 6).map((node) => (
          <button key={node.key} type="button" className={styles.impactNode} onClick={() => onTabChange('impatto')}>
            <strong>{node.label}</strong>
            <span>{node.meta}</span>
          </button>
        ))}
        {dependent.length === 0 && dependencies.length === 0 ? (
          <div className={styles.emptyInline}>Nessun servizio dipendente evidenziato.</div>
        ) : null}
      </div>
    </div>
  );
}

function readinessIcon(status: string): 'check-circle' | 'triangle-alert' | 'info' {
  if (status === 'ready') return 'check-circle';
  if (status === 'blocking') return 'triangle-alert';
  return 'info';
}

function nextActionSummary(nextAction: MaintenanceCockpitData['next_action'], blockingCount: number) {
  if (!nextAction) return 'Ciclo operativo chiuso.';
  if (blockingCount > 0) return 'Completa i blocchi prima di avanzare.';
  return `${nextAction.label} disponibile.`;
}

function nextActionDetail(nextAction: MaintenanceCockpitData['next_action'], blockingCount: number) {
  if (!nextAction) return 'Non sono previste altre azioni di ciclo vita.';
  if (blockingCount > 0) return 'La readiness indica cosa completare prima di procedere.';
  return 'Tutti i controlli richiesti sono soddisfatti. Usa le azioni nella testata per avanzare.';
}

function firstBlockingTab(cockpit: MaintenanceCockpitData) {
  return cockpit.readiness.find((item) => item.blocking)?.target_tab ?? 'cockpit';
}

function finalStateLabel(status: string) {
  if (status === 'completed') return 'Completata';
  if (status === 'cancelled') return 'Annullata';
  if (status === 'superseded') return 'Superata';
  return 'Nessuna azione';
}

function timelineLabel(kind: string, label: string) {
  if (kind === 'window') return label;
  if (kind === 'notice') return `Comunicazione ${label}`;
  if (kind === 'event') return MAINTENANCE_EVENT_LABELS[label] ?? label;
  return label;
}

function timelineMeta(item: MaintenanceCockpitData['timeline'][number], label: string) {
  if (item.kind === 'window') {
    return `${formatDateTime(item.start_at)} · ${windowStatusLabel(item.status)}`;
  }
  if (item.kind === 'notice') {
    return `${formatDateTime(item.event_at)} · ${sendStatusLabel(item.status)}`;
  }
  return `${formatDateTime(item.event_at)}${item.summary && item.summary !== label ? ` · ${item.summary}` : ''}`;
}
