import type { Category } from '../../types';
import { MatrixBackground } from '../MatrixBackground';
import { ScanlineOverlay } from '../ScanlineOverlay';
import { Header } from '../Header';
import { AppCard } from '../AppCard';
import styles from './Portal.module.css';

type PortalProps = {
  categories: Category[];
  appName?: string;
  userName?: string;
  statusTitle?: string;
  statusMessage?: string;
  statusTone?: 'default' | 'error';
  onLogout?: () => void;
};

export function Portal({
  categories,
  appName = 'MrSmith',
  userName = 'Agent J. Doe',
  statusTitle,
  statusMessage,
  statusTone = 'default',
  onLogout,
}: PortalProps) {
  return (
    <>
      <MatrixBackground />
      <ScanlineOverlay />
      <div className={styles.wrapper}>
        <Header appName={appName} userName={userName} onLogout={onLogout} />
        <main className={styles.main}>
          {statusTitle ? (
            <section
              className={statusTone === 'error' ? styles.statusPanelError : styles.statusPanel}
            >
              <p className={styles.statusEyebrow}>SYSTEM STATUS</p>
              <h2 className={styles.statusTitle}>{statusTitle}</h2>
              {statusMessage ? <p className={styles.statusMessage}>{statusMessage}</p> : null}
            </section>
          ) : (
            <div className={styles.grid}>
              {categories.map((cat) => (
                <div key={cat.id} className={styles.section}>
                  <h2 className={styles.sectionTitle}>
                    <span className={styles.prompt}>&gt;</span> {cat.title}
                    <span className={styles.cursor}>_</span>
                  </h2>
                  <div className={styles.cards}>
                    {cat.apps.map((app) => (
                      <AppCard
                        key={app.id}
                        icon={app.icon}
                        name={app.name}
                        description={app.description}
                        href={app.href}
                        status={app.status}
                      />
                    ))}
                  </div>
                </div>
              ))}
            </div>
          )}
        </main>
      </div>
    </>
  );
}
