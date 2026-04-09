import styles from './Stepper.module.css';

interface StepperProps {
  steps: string[];
  current: number;
  onStepClick: (step: number) => void;
}

export function Stepper({ steps, current, onStepClick }: StepperProps) {
  return (
    <div className={styles.stepper}>
      {steps.map((label, i) => {
        const isCompleted = i < current;
        const isCurrent = i === current;
        return (
          <div
            key={i}
            className={`${styles.step} ${isCompleted ? styles.stepCompleted : ''}`}
            onClick={() => isCompleted && onStepClick(i)}
          >
            <div className={`${styles.circle} ${
              isCompleted ? styles.circleCompleted :
              isCurrent ? styles.circleCurrent : ''
            }`}>
              {isCompleted ? '\u2713' : i + 1}
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
