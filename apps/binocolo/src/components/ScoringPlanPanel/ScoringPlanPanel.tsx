import type { MAScoringPlan, MAScoringPlanSignal } from '../../api/types';
import styles from './ScoringPlanPanel.module.css';

const FAMILY_ORDER = ['aderenza', 'opportunita', 'economico'] as const;

const FAMILY_LABELS: Record<string, string> = {
  aderenza: 'Aderenza',
  opportunita: 'Opportunità',
  economico: 'Economico',
};

const weightFormat = new Intl.NumberFormat('it-IT', {
  minimumFractionDigits: 1,
  maximumFractionDigits: 1,
});

function familyLabel(family: string): string {
  return FAMILY_LABELS[family] ?? family;
}

function groupSignals(signals: MAScoringPlanSignal[]): Array<[string, MAScoringPlanSignal[]]> {
  const groups = new Map<string, MAScoringPlanSignal[]>();
  signals.forEach((signal) => {
    const familySignals = groups.get(signal.family) ?? [];
    familySignals.push(signal);
    groups.set(signal.family, familySignals);
  });

  return [...groups.entries()].sort(([left], [right]) => {
    const leftIndex = FAMILY_ORDER.indexOf(left as (typeof FAMILY_ORDER)[number]);
    const rightIndex = FAMILY_ORDER.indexOf(right as (typeof FAMILY_ORDER)[number]);
    if (leftIndex === -1 && rightIndex === -1) return left.localeCompare(right, 'it');
    if (leftIndex === -1) return 1;
    if (rightIndex === -1) return -1;
    return leftIndex - rightIndex;
  });
}

export function ScoringPlanPanel({ plan }: { plan: MAScoringPlan }) {
  const signalGroups = groupSignals(plan.evaluated);

  return (
    <div className={styles.plan}>
      <section className={styles.section} aria-labelledby="scoring-plan-evaluated">
        <div className={styles.sectionHeader}>
          <h3 id="scoring-plan-evaluated">Valutato</h3>
          <p>Pesi effettivi di questa ricerca: il budget della tesi è ripartito sui segnali attivi.</p>
        </div>
        {signalGroups.length > 0 ? (
          <div className={styles.families}>
            {signalGroups.map(([family, signals]) => (
              <div key={family} className={styles.family}>
                <h4>{familyLabel(family)}</h4>
                <dl className={styles.signals}>
                  {signals.map((signal) => (
                    <div key={signal.id} className={styles.signal}>
                      <dt>{signal.label}</dt>
                      <dd>{weightFormat.format(signal.weight)}%</dd>
                    </div>
                  ))}
                </dl>
              </div>
            ))}
          </div>
        ) : (
          <p className={styles.empty}>Nessun segnale attivo per questa ricerca.</p>
        )}
        <p className={styles.note}>Su una singola azienda i segnali senza dato non pesano: il peso si ridistribuisce tra quelli misurabili.</p>
      </section>

      <section className={styles.section} aria-labelledby="scoring-plan-filtered">
        <div className={styles.sectionHeader}>
          <h3 id="scoring-plan-filtered">Filtrato a monte</h3>
          <p>Vincoli già applicati alla superficie: ogni azienda valutata li rispetta.</p>
        </div>
        {plan.filtered.length > 0 ? (
          <ul className={styles.list}>
            {plan.filtered.map((item, index) => <li key={`${index}-${item}`}>{item}</li>)}
          </ul>
        ) : (
          <p className={styles.empty}>Nessun vincolo applicato a monte.</p>
        )}
      </section>

      {plan.ignored.length > 0 ? (
        <section className={styles.section} aria-labelledby="scoring-plan-ignored">
          <div className={styles.sectionHeader}>
            <h3 id="scoring-plan-ignored">Ignorato</h3>
            <p>Criteri della richiesta non supportati dallo score.</p>
          </div>
          <ul className={styles.list}>
            {plan.ignored.map((item, index) => <li key={`${index}-${item}`}>{item}</li>)}
          </ul>
        </section>
      ) : null}
    </div>
  );
}
