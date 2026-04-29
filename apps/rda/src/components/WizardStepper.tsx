import { Icon } from '@mrsmith/ui';

export function WizardStepper({
  steps,
  current,
  maxAvailable,
  onStepClick,
}: {
  steps: string[];
  current: number;
  maxAvailable: number;
  onStepClick: (step: number) => void;
}) {
  return (
    <div className="wizardStepper" role="list" aria-label="Avanzamento richiesta">
      {steps.map((label, index) => {
        const completed = index < current;
        const active = index === current;
        const enabled = index <= maxAvailable;
        return (
          <button
            key={label}
            className={`wizardStep ${completed ? 'completed' : ''} ${active ? 'active' : ''}`}
            type="button"
            role="listitem"
            aria-current={active ? 'step' : undefined}
            disabled={!enabled}
            onClick={() => onStepClick(index)}
          >
            <span className="wizardStepMarker">
              {completed ? <Icon name="check" size={14} strokeWidth={2.5} /> : index + 1}
            </span>
            <span className="wizardStepLabel">{label}</span>
          </button>
        );
      })}
    </div>
  );
}
