import styles from './SuccessCheckmark.module.css';

interface SuccessCheckmarkProps {
  size?: number;
}

export function SuccessCheckmark({ size = 64 }: SuccessCheckmarkProps) {
  return (
    <div
      className={styles.wrap}
      style={{ width: size, height: size }}
      role="img"
      aria-label="Operazione completata"
    >
      <svg viewBox="0 0 64 64" fill="none" xmlns="http://www.w3.org/2000/svg">
        <circle
          className={styles.circle}
          cx="32"
          cy="32"
          r="28"
          stroke="currentColor"
          strokeWidth="3"
        />
        <path
          className={styles.check}
          d="M20 33L28 41L44 24"
          stroke="currentColor"
          strokeWidth="4"
          strokeLinecap="round"
          strokeLinejoin="round"
        />
      </svg>
    </div>
  );
}
