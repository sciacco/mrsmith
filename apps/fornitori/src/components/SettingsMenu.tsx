import { Link, useLocation } from 'react-router-dom';
import { Icon } from '@mrsmith/ui';
import styles from './SettingsMenu.module.css';

export function SettingsMenu() {
  const location = useLocation();
  const active = location.pathname === '/impostazioni' || location.pathname.startsWith('/impostazioni/');

  return (
    <Link
      to="/impostazioni/qualifica"
      aria-label="Impostazioni"
      aria-current={active ? 'page' : undefined}
      className={`${styles.trigger} ${active ? styles.active : ''}`}
      title="Impostazioni"
    >
      <Icon name="settings" size={16} aria-hidden />
      <span>Impostazioni</span>
    </Link>
  );
}
