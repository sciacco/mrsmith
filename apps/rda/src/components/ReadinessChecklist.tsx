import { Icon } from '@mrsmith/ui';

export interface ReadinessItem {
  id: string;
  label: string;
  detail: string;
  ready: boolean;
}

export function ReadinessChecklist({ items }: { items: ReadinessItem[] }) {
  return (
    <div className="readinessList">
      {items.map((item) => (
        <div key={item.id} className={`readinessItem ${item.ready ? 'ready' : 'blocked'}`}>
          <span className="readinessIcon">
            <Icon name={item.ready ? 'check' : 'triangle-alert'} size={16} />
          </span>
          <div>
            <strong>{item.label}</strong>
            <p>{item.detail}</p>
          </div>
        </div>
      ))}
    </div>
  );
}
