import styles from './TrialSlider.module.css';

interface TrialSliderProps {
  value: number;
  onChange: (value: number) => void;
  min?: number;
  max?: number;
  step?: number;
  tickEvery?: number;
  'aria-label'?: string;
}

export function TrialSlider({
  value,
  onChange,
  min = 0,
  max = 200,
  step = 10,
  tickEvery = 50,
  'aria-label': ariaLabel,
}: TrialSliderProps) {
  const progress = max === min ? 0 : ((value - min) / (max - min)) * 100;
  const ticks: number[] = [];
  for (let t = min; t <= max; t += tickEvery) ticks.push(t);

  return (
    <div className={styles.root}>
      <div className={styles.trackWrap}>
        <div className={styles.track} aria-hidden="true">
          <div className={styles.fill} style={{ width: `${progress}%` }} />
          {ticks.map(t => {
            const left = max === min ? 0 : ((t - min) / (max - min)) * 100;
            return (
              <span
                key={t}
                className={styles.tick}
                style={{ left: `${left}%` }}
              />
            );
          })}
        </div>
        <div className={styles.thumb} style={{ left: `${progress}%` }} aria-hidden="true">
          <span className={styles.bubble}>{value}€</span>
        </div>
        <input
          className={styles.input}
          type="range"
          min={min}
          max={max}
          step={step}
          value={value}
          onChange={e => onChange(Number(e.target.value))}
          aria-label={ariaLabel}
          aria-valuemin={min}
          aria-valuemax={max}
          aria-valuenow={value}
        />
      </div>
      <div className={styles.endpoints} aria-hidden="true">
        <span>{min}€</span>
        <span>{max}€</span>
      </div>
    </div>
  );
}
