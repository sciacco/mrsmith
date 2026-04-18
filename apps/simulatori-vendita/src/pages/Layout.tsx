import { NavLink, Outlet } from 'react-router-dom';
import styles from './Layout.module.css';

export function Layout() {
  return (
    <div className={styles.root}>
      <nav className={styles.tabs} aria-label="Versioni calcolatore">
        <NavLink
          to="/"
          end
          className={({ isActive }) =>
            `${styles.tab} ${isActive ? styles.tabActive : ''}`
          }
        >
          Calcolatore IaaS
        </NavLink>
        <NavLink
          to="/lab"
          className={({ isActive }) =>
            `${styles.tab} ${isActive ? styles.tabActive : ''}`
          }
        >
          Lab
          <span className={styles.tabBadge}>esperimenti</span>
        </NavLink>
      </nav>
      <Outlet />
    </div>
  );
}
