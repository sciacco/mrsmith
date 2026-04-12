import { useEffect, useState } from 'react';
import { Button, useToast } from '@mrsmith/ui';
import styles from './WizardNav.module.css';

interface WizardNavProps {
  step: number;
  totalSteps: number;
  canAdvance: boolean;
  validationMessage?: string;
  isLastStep: boolean;
  onBack: () => void;
  onNext: () => void;
  isPending?: boolean;
}

export function WizardNav({
  step,
  totalSteps,
  canAdvance,
  validationMessage,
  isLastStep,
  onBack,
  onNext,
  isPending,
}: WizardNavProps) {
  const { toast } = useToast();
  const [shake, setShake] = useState(false);

  useEffect(() => {
    if (!shake) return;
    const id = window.setTimeout(() => setShake(false), 400);
    return () => window.clearTimeout(id);
  }, [shake]);

  const handleNextClick = () => {
    if (!canAdvance) {
      setShake(true);
      if (validationMessage) toast(validationMessage, 'warning');
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
