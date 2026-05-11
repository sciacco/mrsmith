import { type ReactNode } from 'react';
import {
  NotificationBell,
  type NotificationBellVisibility,
} from '../NotificationBell/NotificationBell';
import { SupportMenu, type AppShellSupportConfig } from '../SupportMenu/SupportMenu';
import { UserMenu } from '../UserMenu/UserMenu';
import styles from './AppShell.module.css';

type AppShellNotificationsConfig = {
  enabled?: boolean;
  visibility?: NotificationBellVisibility;
};

interface AppShellProps {
  userName?: string;
  appName?: string;
  onLogout?: () => void;
  support?: AppShellSupportConfig;
  notifications?: boolean | AppShellNotificationsConfig;
  children: ReactNode;
}

function Nav({ children }: { children: ReactNode }) {
  return <>{children}</>;
}

function Content({ children }: { children: ReactNode }) {
  return <>{children}</>;
}

export function AppShell({
  userName,
  appName,
  onLogout,
  support,
  notifications,
  children,
}: AppShellProps) {
  let nav: ReactNode = null;
  let content: ReactNode = null;
  const notificationSettings =
    typeof notifications === 'object' ? notifications : undefined;
  const notificationsEnabled =
    notifications !== false &&
    notificationSettings?.enabled !== false &&
    Boolean(support?.getAccessToken) &&
    support?.authenticated !== false;
  const notificationVisibility = notificationSettings?.visibility ?? 'unread';

  const childArray = Array.isArray(children) ? children : [children];
  for (const child of childArray) {
    if (child && typeof child === 'object' && 'type' in child) {
      if (child.type === Nav) nav = child.props.children;
      else if (child.type === Content) content = child.props.children;
    }
  }

  return (
    <div className={styles.shell}>
      <header className={styles.header}>
        <a href="/" className={styles.logo}>
          <span className={styles.logoIcon}>S</span>
          <span className={styles.logoText}>MrSmith</span>
        </a>
        {appName && (
          <>
            <span className={styles.separator} aria-hidden="true">╱</span>
            <span className={styles.appName}>{appName}</span>
          </>
        )}
        <nav className={styles.nav}>{nav}</nav>
        <div className={styles.actions}>
          {notificationsEnabled ? (
            <NotificationBell
              auth={support}
              status="unread"
              variant="clean"
              visibility={notificationVisibility}
            />
          ) : null}
          {support && <SupportMenu appName={appName} support={support} />}
          {userName && <UserMenu userName={userName} onLogout={onLogout} />}
        </div>
      </header>
      <main className={styles.main}>{content}</main>
    </div>
  );
}

AppShell.Nav = Nav;
AppShell.Content = Content;
