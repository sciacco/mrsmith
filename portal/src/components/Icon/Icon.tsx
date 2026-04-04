import { icons } from './icons';
import styles from './Icon.module.css';

type IconProps = {
  name: string;
  className?: string;
};

export function Icon({ name, className }: IconProps) {
  const icon = icons[name];
  if (!icon) return null;

  return (
    <span className={`${styles.icon} ${className ?? ''}`} aria-hidden="true">
      {icon}
    </span>
  );
}
