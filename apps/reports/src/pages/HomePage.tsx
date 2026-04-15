import { Link } from 'react-router-dom';
import { Icon, type IconName } from '@mrsmith/ui';
import { reportNavSections } from '../navigation';
import shared from './shared.module.css';
import styles from './HomePage.module.css';

interface CardProps {
  icon: IconName;
  title: string;
  desc: string;
}

function Card({ icon, title, desc }: CardProps) {
  return (
    <div className={styles.card}>
      <div className={styles.iconWrap}>
        <Icon name={icon} size={20} />
      </div>
      <div className={styles.cardTitle}>{title}</div>
      <div className={styles.cardDesc}>{desc}</div>
    </div>
  );
}

export default function HomePage() {
  return (
    <div className={shared.page}>
      <h1 className={shared.title}>Reports</h1>
      {reportNavSections.map((section) => (
        <section key={section.label} className={styles.section}>
          <h2 className={styles.sectionTitle}>{section.label}</h2>
          <div className={styles.grid}>
            {section.items.map((item) => (
              <Link key={item.path} to={item.path} className={styles.cardLink}>
                <Card icon={item.icon} title={item.label} desc={item.desc} />
              </Link>
            ))}
          </div>
        </section>
      ))}
    </div>
  );
}
