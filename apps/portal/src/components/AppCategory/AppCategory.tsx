import type { App } from '../../types';
import { AppCard } from '../AppCard';
import styles from './AppCategory.module.css';

type AppCategoryProps = {
  title: string;
  apps: App[];
};

export function AppCategory({ title, apps }: AppCategoryProps) {
  return (
    <section className={styles.category}>
      <h2 className={styles.title}>{title}</h2>
      <div className={styles.grid}>
        {apps.map((app) => (
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
    </section>
  );
}
