import styles from './WizardNav.module.css';

interface WizardNavProps {
  step: number;
  totalSteps: number;
  canAdvance: boolean;
  isLastStep: boolean;
  onBack: () => void;
  onNext: () => void;
  isPending?: boolean;
}

export function WizardNav({ step, totalSteps, canAdvance, isLastStep, onBack, onNext, isPending }: WizardNavProps) {
  return (
    <div className={styles.nav}>
      <button
        className={`${styles.btnBack} ${step === 0 ? styles.hidden : ''}`}
        onClick={onBack}
      >
        Indietro
      </button>
      <span className={styles.stepInfo}>Passo {step + 1} di {totalSteps}</span>
      <button
        className={styles.btnNext}
        disabled={!canAdvance || isPending}
        onClick={onNext}
      >
        {isPending ? 'Creazione...' : isLastStep ? 'Crea proposta' : 'Avanti'}
      </button>
    </div>
  );
}
