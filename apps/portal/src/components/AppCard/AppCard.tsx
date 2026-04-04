import { Icon } from '../Icon';
import styles from './AppCard.module.css';

type AppCardProps = {
  icon: string;
  name: string;
  description: string;
  href?: string;
  onClick?: () => void;
};

export function AppCard({ icon, name, description, href, onClick }: AppCardProps) {
  const handleClick = () => {
    if (onClick) {
      onClick();
    } else if (href) {
      window.location.href = href;
    }
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' || e.key === ' ') {
      e.preventDefault();
      handleClick();
    }
  };

  return (
    <article
      className={styles.card}
      tabIndex={0}
      role="button"
      aria-label={`Launch ${name}`}
      onClick={handleClick}
      onKeyDown={handleKeyDown}
    >
      <Icon name={icon} />
      <div className={styles.name}>{name}</div>
      <div className={styles.desc}>{description}</div>
    </article>
  );
}
