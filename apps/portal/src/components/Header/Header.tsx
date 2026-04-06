import { UserMenu } from '@mrsmith/ui';
import styles from './Header.module.css';

type HeaderProps = {
  appName?: string;
  userName?: string;
  onLogout?: () => void;
};

export function Header({
  appName = 'MrSmith',
  userName = 'Agent J. Doe',
  onLogout,
}: HeaderProps) {
  return (
    <header className={styles.header}>
      <div className={styles.logo}>
        {appName}
        <span className={styles.glasses} aria-hidden="true">
          <svg
            viewBox="0 0 48 20"
            fill="none"
            stroke="currentColor"
            strokeWidth={1.5}
            strokeLinecap="round"
            strokeLinejoin="round"
          >
            <rect
              x="2"
              y="5"
              width="16"
              height="11"
              rx="2"
              fill="rgba(0,255,65,0.08)"
            />
            <rect
              x="30"
              y="5"
              width="16"
              height="11"
              rx="2"
              fill="rgba(0,255,65,0.08)"
            />
            <path d="M18 10 Q24 4 30 10" />
            <line x1="2" y1="8" x2="0" y2="5" />
            <line x1="46" y1="8" x2="48" y2="5" />
          </svg>
        </span>
      </div>
      <UserMenu userName={userName} onLogout={onLogout} />
    </header>
  );
}
