import { Icon } from '@mrsmith/ui';
import type { ReadinessItem } from './ReadinessChecklist';
import { ReadinessChecklist } from './ReadinessChecklist';
import { workflowStages } from '../lib/po-detail-view-model';

export function POWorkflowRail({ stage }: { stage?: string | null }) {
  const currentIndex = Math.max(0, workflowStages.findIndex((item) => item.id === stage));

  return (
    <section className="surface poSidePanel">
      <div className="surfaceHeader compactPanelHeader">
        <div>
          <h2>Percorso PO</h2>
          <p className="muted">Stato operativo della richiesta.</p>
        </div>
      </div>
      <div className="workflowRail">
        {workflowStages.map((item, index) => {
          const current = index === currentIndex;
          const complete = index < currentIndex;
          return (
            <div key={item.id} className={`workflowStep ${current ? 'current' : ''} ${complete ? 'complete' : ''}`}>
              <span className="workflowMarker">{complete ? <Icon name="check" size={13} /> : index + 1}</span>
              <span>{item.label}</span>
            </div>
          );
        })}
      </div>
    </section>
  );
}

export function POReadinessPanel({ items }: { items: ReadinessItem[] }) {
  const readyCount = items.filter((item) => item.ready).length;
  return (
    <section className="surface poSidePanel">
      <div className="surfaceHeader compactPanelHeader">
        <div>
          <h2>Controllo invio</h2>
          <p className="muted">{readyCount}/{items.length} pronti.</p>
        </div>
        <span className={`badge ${readyCount === items.length ? 'success' : 'warning'}`}>{readyCount}/{items.length}</span>
      </div>
      <div className="poSidePanelBody">
        <ReadinessChecklist items={items} />
      </div>
    </section>
  );
}
