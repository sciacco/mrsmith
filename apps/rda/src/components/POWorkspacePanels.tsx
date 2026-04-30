import { Icon } from '@mrsmith/ui';
import { useEffect, useId, useState } from 'react';
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
  const blockedItems = items.filter((item) => !item.ready);
  const allReady = blockedItems.length === 0;
  const panelId = useId();
  const [collapsed, setCollapsed] = useState(allReady);
  const summary = readinessSummary(blockedItems);

  useEffect(() => {
    setCollapsed(allReady);
  }, [allReady]);

  return (
    <section className={`surface poSidePanel ${collapsed ? 'collapsed' : ''}`}>
      <div className="surfaceHeader compactPanelHeader">
        <div>
          <h2>Controllo invio</h2>
          {summary ? <p className="muted">{summary}</p> : null}
        </div>
        <div className="readinessPanelActions">
          <span className={`badge ${allReady ? 'success' : 'warning'}`}>{readyCount}/{items.length}</span>
          <button
            className="iconButton readinessToggle"
            type="button"
            aria-label={collapsed ? 'Mostra controlli invio' : 'Nascondi controlli invio'}
            aria-controls={panelId}
            aria-expanded={!collapsed}
            title={collapsed ? 'Mostra controlli' : 'Nascondi controlli'}
            onClick={() => setCollapsed((value) => !value)}
          >
            <Icon name={collapsed ? 'chevron-down' : 'chevron-up'} size={16} />
          </button>
        </div>
      </div>
      <div id={panelId} className="poSidePanelBody" hidden={collapsed}>
        <ReadinessChecklist items={items} />
      </div>
    </section>
  );
}

function readinessSummary(blockedItems: ReadinessItem[]): string | null {
  if (blockedItems.length === 0) return null;
  if (blockedItems.length === 1) {
    const [item] = blockedItems;
    if (!item) return null;
    if (item.id === 'payment' && item.detail.toLowerCase().includes('approvazione')) {
      return 'Pagamento in attesa di approvazione.';
    }
    return `Da risolvere: ${item.label}.`;
  }

  const labels = blockedItems.slice(0, 2).map((item) => item.label);
  const remaining = blockedItems.length - labels.length;
  if (remaining === 0) return `Da risolvere: ${labels[0]} e ${labels[1]}.`;

  return `Da risolvere: ${labels.join(', ')} e ${remaining === 1 ? '1 altro' : `altri ${remaining}`}.`;
}
