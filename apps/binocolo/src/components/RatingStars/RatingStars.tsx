import { Icon } from '@mrsmith/ui';
import styles from './RatingStars.module.css';

export function RatingStars({ rating, onRate }: { rating: number; onRate: (rating: number) => void }) {
  const excluded = rating === -1;
  return (
    <span className={styles.ratingControl} onClick={(event) => event.stopPropagation()}>
      <span className={styles.stars}>
        {[1, 2, 3].map((value) => (
          <button
            key={value}
            type="button"
            className={`${styles.starButton} ${!excluded && rating >= value ? styles.starOn : ''}`}
            onClick={() => onRate(value)}
            aria-label={`${value} stelle`}
          >
            ★
          </button>
        ))}
      </span>
      <button
        type="button"
        className={`${styles.excludeButton} ${excluded ? styles.excludeOn : ''}`}
        onClick={() => onRate(excluded ? 0 : -1)}
        title={excluded ? 'Rimuovi esclusione' : 'Escludi'}
        aria-label="Escludi"
      >
        <Icon name="x-circle" size={15} />
      </button>
    </span>
  );
}
