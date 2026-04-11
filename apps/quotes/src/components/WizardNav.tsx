import { useEffect, useState } from 'react';
import { Button } from '@mrsmith/ui';
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

export function WizardNav({
  step,
  totalSteps,
  canAdvance,
  isLastStep,
  onBack,
  onNext,
  isPending,
}: WizardNavProps) {
  const [shake, setShake] = useState(false);

  useEffect(() => {
    if (!shake) return;
    const id = window.setTimeout(() => setShake(false), 400);
    return () => window.clearTimeout(id);
  }, [shake]);

  const handleNextClick = () => {
    if (!canAdvance) {
      setShake(true);
      return;
    }
    onNext();
  };

  return (
    <div className={styles.nav}>
      <div className={styles.left}>
        {step > 0 && (
          <Button variant="ghost" onClick={onBack}>
            Indietro
          </Button>
        )}
      </div>
      <span className={styles.stepInfo}>
        Passo {step + 1} di {totalSteps}
      </span>
      <div className={`${styles.right} ${shake ? styles.shake : ''}`}>
        <Button
          variant="primary"
          onClick={handleNextClick}
          loading={isPending}
          disabled={isPending}
        >
          {isLastStep ? 'Crea proposta' : 'Avanti'}
        </Button>
      </div>
    </div>
  );
}
