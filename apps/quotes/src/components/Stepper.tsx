import { Icon } from '@mrsmith/ui';
import styles from './Stepper.module.css';

interface StepperProps {
  steps: string[];
  current: number;
  onStepClick: (step: number) => void;
}

export function Stepper({ steps, current, onStepClick }: StepperProps) {
  return (
    <div className={styles.stepper} role="list">
      {steps.map((label, i) => {
        const isCompleted = i < current;
        const isCurrent = i === current;
        const clickable = isCompleted;
        return (
          <div
            key={i}
            role="listitem"
            aria-current={isCurrent ? 'step' : undefined}
            className={`${styles.step} ${isCompleted ? styles.stepCompleted : ''} ${
              isCurrent ? styles.stepCurrent : ''
            } ${clickable ? styles.stepClickable : ''}`}
            onClick={() => clickable && onStepClick(i)}
          >
            <div
              className={`${styles.circle} ${
                isCompleted ? styles.circleCompleted : isCurrent ? styles.circleCurrent : ''
              }`}
            >
              {isCompleted ? (
                <Icon name="check" size={14} strokeWidth={2.5} />
              ) : (
                i + 1
              )}
            </div>
            <span className={`${styles.label} ${isCurrent || isCompleted ? styles.labelActive : ''}`}>
              {label}
            </span>
          </div>
        );
      })}
    </div>
  );
}
