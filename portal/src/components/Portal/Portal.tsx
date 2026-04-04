import type { Category } from '../../types';
import { MatrixBackground } from '../MatrixBackground';
import { ScanlineOverlay } from '../ScanlineOverlay';
import { Header } from '../Header';
import { AppCategory } from '../AppCategory';
import styles from './Portal.module.css';

type PortalProps = {
  categories: Category[];
  appName?: string;
  userName?: string;
};

export function Portal({
  categories,
  appName = 'MrSmith',
  userName = 'Agent J. Doe',
}: PortalProps) {
  return (
    <>
      <MatrixBackground />
      <ScanlineOverlay />
      <div className={styles.wrapper}>
        <Header appName={appName} userName={userName} />
        <main className={styles.main}>
          <div className={styles.grid}>
            {categories.map((cat) => (
              <AppCategory key={cat.id} title={cat.title} apps={cat.apps} />
            ))}
          </div>
        </main>
      </div>
    </>
  );
}
