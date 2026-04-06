import { type ReactNode } from 'react';
import { UserMenu } from '../UserMenu/UserMenu';
import styles from './AppShell.module.css';

interface AppShellProps {
  userName?: string;
  onLogout?: () => void;
  children: ReactNode;
}

function Nav({ children }: { children: ReactNode }) {
  return <>{children}</>;
}

function Content({ children }: { children: ReactNode }) {
  return <>{children}</>;
}

export function AppShell({ userName, onLogout, children }: AppShellProps) {
  let nav: ReactNode = null;
  let content: ReactNode = null;

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
        <nav className={styles.nav}>{nav}</nav>
        {userName && <UserMenu userName={userName} onLogout={onLogout} />}
      </header>
      <main className={styles.main}>{content}</main>
    </div>
  );
}

AppShell.Nav = Nav;
AppShell.Content = Content;
